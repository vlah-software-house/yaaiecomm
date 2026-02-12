package vat

import (
	"testing"

	"github.com/shopspring/decimal"
)

// newTestCache creates a RateCache pre-loaded with realistic EU VAT rates
// for testing purposes.
func newTestCache() *RateCache {
	cache := NewRateCache()
	cache.Load([]VATRate{
		// Germany: standard 19%, reduced 7%
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "DE", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(7.0)},
		// Spain: standard 21%, reduced 10%, super_reduced 4%
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "ES", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(10.0)},
		{CountryCode: "ES", RateType: RateTypeSuperReduced, Rate: decimal.NewFromFloat(4.0)},
		// France: standard 20%, reduced 5.5%, reduced_alt 10%, super_reduced 2.1%
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
		{CountryCode: "FR", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(5.5)},
		{CountryCode: "FR", RateType: RateTypeReducedAlt, Rate: decimal.NewFromFloat(10.0)},
		{CountryCode: "FR", RateType: RateTypeSuperReduced, Rate: decimal.NewFromFloat(2.1)},
		// Denmark: only standard 25%
		{CountryCode: "DK", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(25.0)},
		// Hungary: standard 27% (highest in EU)
		{CountryCode: "HU", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(27.0)},
		{CountryCode: "HU", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(5.0)},
		// Belgium: standard 21%, reduced 6%, reduced_alt 12%, parking 12%
		{CountryCode: "BE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "BE", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(6.0)},
		{CountryCode: "BE", RateType: RateTypeReducedAlt, Rate: decimal.NewFromFloat(12.0)},
		{CountryCode: "BE", RateType: RateTypeParking, Rate: decimal.NewFromFloat(12.0)},
	})
	return cache
}

func TestEngine_Calculate_VATDisabled(t *testing.T) {
	engine := NewEngine(newTestCache())

	result := engine.Calculate(VATCalculationInput{
		ProductPrice:          decimal.NewFromFloat(100.00),
		VATCategoryRateType:   RateTypeStandard,
		DestinationCountry:    "DE",
		StoreVATEnabled:       false,
		StoreCountryCode:      "ES",
		StorePricesIncludeVAT: true,
	})

	if result.ExemptReason != ExemptReasonDisabled {
		t.Errorf("expected exempt reason %q, got %q", ExemptReasonDisabled, result.ExemptReason)
	}
	if !result.Amount.IsZero() {
		t.Errorf("expected zero VAT amount, got %s", result.Amount.String())
	}
	if !result.Rate.IsZero() {
		t.Errorf("expected zero rate, got %s", result.Rate.String())
	}
	if !result.NetPrice.Equal(decimal.NewFromFloat(100.00)) {
		t.Errorf("expected net price 100.00, got %s", result.NetPrice.String())
	}
	if !result.GrossPrice.Equal(decimal.NewFromFloat(100.00)) {
		t.Errorf("expected gross price 100.00, got %s", result.GrossPrice.String())
	}
}

func TestEngine_Calculate_B2BReverseCharge(t *testing.T) {
	engine := NewEngine(newTestCache())

	tests := []struct {
		name             string
		input            VATCalculationInput
		wantExemptReason string
		wantAmountZero   bool
		wantNetPrice     decimal.Decimal
	}{
		{
			name: "cross-border B2B with valid VAT number",
			input: VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(121.00), // Price includes 21% Spanish VAT
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    "DE",
				CustomerVATNumber:     "DE123456789",
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: true,
				B2BReverseCharge:      true,
			},
			wantExemptReason: ExemptReasonReverseCharge,
			wantAmountZero:   true,
			wantNetPrice:     decimal.NewFromFloat(100.00),
		},
		{
			name: "same country B2B should charge VAT normally",
			input: VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(121.00),
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    "ES", // Same as store country
				CustomerVATNumber:     "ESB12345678",
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: true,
				B2BReverseCharge:      true,
			},
			wantExemptReason: "", // No exemption for domestic B2B
			wantAmountZero:   false,
		},
		{
			name: "B2B without VAT number should charge VAT",
			input: VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(121.00),
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    "DE",
				CustomerVATNumber:     "", // No VAT number
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: true,
				B2BReverseCharge:      true,
			},
			wantExemptReason: "",
			wantAmountZero:   false,
		},
		{
			name: "B2B reverse charge disabled in store",
			input: VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(121.00),
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    "DE",
				CustomerVATNumber:     "DE123456789",
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: true,
				B2BReverseCharge:      false, // Disabled
			},
			wantExemptReason: "",
			wantAmountZero:   false,
		},
		{
			name: "cross-border B2B with net prices",
			input: VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(100.00), // Net price
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    "FR",
				CustomerVATNumber:     "FR12345678901",
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: false,
				B2BReverseCharge:      true,
			},
			wantExemptReason: ExemptReasonReverseCharge,
			wantAmountZero:   true,
			wantNetPrice:     decimal.NewFromFloat(100.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Calculate(tt.input)

			if result.ExemptReason != tt.wantExemptReason {
				t.Errorf("expected exempt reason %q, got %q", tt.wantExemptReason, result.ExemptReason)
			}
			if tt.wantAmountZero && !result.Amount.IsZero() {
				t.Errorf("expected zero VAT amount, got %s", result.Amount.String())
			}
			if !tt.wantAmountZero && result.Amount.IsZero() {
				t.Errorf("expected non-zero VAT amount")
			}
			if !tt.wantNetPrice.IsZero() && !result.NetPrice.Equal(tt.wantNetPrice) {
				t.Errorf("expected net price %s, got %s", tt.wantNetPrice.String(), result.NetPrice.String())
			}
		})
	}
}

func TestEngine_Calculate_PricesIncludeVAT(t *testing.T) {
	engine := NewEngine(newTestCache())

	tests := []struct {
		name           string
		productPrice   decimal.Decimal
		rateType       string
		destCountry    string
		storeCountry   string
		wantRate       decimal.Decimal
		wantNetPrice   decimal.Decimal
		wantVATAmount  decimal.Decimal
		wantGrossPrice decimal.Decimal
	}{
		{
			name:           "Spain standard 21% - extract VAT from 121",
			productPrice:   decimal.NewFromFloat(121.00),
			rateType:       RateTypeStandard,
			destCountry:    "ES",
			storeCountry:   "ES",
			wantRate:       decimal.NewFromFloat(21.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(21.00),
			wantGrossPrice: decimal.NewFromFloat(121.00),
		},
		{
			name:           "Germany standard 19% - extract VAT from 119",
			productPrice:   decimal.NewFromFloat(119.00),
			rateType:       RateTypeStandard,
			destCountry:    "DE",
			storeCountry:   "DE",
			wantRate:       decimal.NewFromFloat(19.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(19.00),
			wantGrossPrice: decimal.NewFromFloat(119.00),
		},
		{
			name:           "France reduced 5.5% - extract VAT from 105.50",
			productPrice:   decimal.NewFromFloat(105.50),
			rateType:       RateTypeReduced,
			destCountry:    "FR",
			storeCountry:   "FR",
			wantRate:       decimal.NewFromFloat(5.5),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(5.50),
			wantGrossPrice: decimal.NewFromFloat(105.50),
		},
		{
			name:           "Hungary standard 27% (highest EU rate)",
			productPrice:   decimal.NewFromFloat(127.00),
			rateType:       RateTypeStandard,
			destCountry:    "HU",
			storeCountry:   "HU",
			wantRate:       decimal.NewFromFloat(27.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(27.00),
			wantGrossPrice: decimal.NewFromFloat(127.00),
		},
		{
			name:           "Spain super reduced 4%",
			productPrice:   decimal.NewFromFloat(104.00),
			rateType:       RateTypeSuperReduced,
			destCountry:    "ES",
			storeCountry:   "ES",
			wantRate:       decimal.NewFromFloat(4.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(4.00),
			wantGrossPrice: decimal.NewFromFloat(104.00),
		},
		{
			name:           "Belgium parking rate 12%",
			productPrice:   decimal.NewFromFloat(112.00),
			rateType:       RateTypeParking,
			destCountry:    "BE",
			storeCountry:   "BE",
			wantRate:       decimal.NewFromFloat(12.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(12.00),
			wantGrossPrice: decimal.NewFromFloat(112.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Calculate(VATCalculationInput{
				ProductPrice:          tt.productPrice,
				VATCategoryRateType:   tt.rateType,
				DestinationCountry:    tt.destCountry,
				StoreCountryCode:      tt.storeCountry,
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: true,
			})

			if !result.Rate.Equal(tt.wantRate) {
				t.Errorf("rate: want %s, got %s", tt.wantRate.String(), result.Rate.String())
			}
			if !result.NetPrice.Equal(tt.wantNetPrice) {
				t.Errorf("net price: want %s, got %s", tt.wantNetPrice.String(), result.NetPrice.String())
			}
			if !result.Amount.Equal(tt.wantVATAmount) {
				t.Errorf("VAT amount: want %s, got %s", tt.wantVATAmount.String(), result.Amount.String())
			}
			if !result.GrossPrice.Equal(tt.wantGrossPrice) {
				t.Errorf("gross price: want %s, got %s", tt.wantGrossPrice.String(), result.GrossPrice.String())
			}
			if result.ExemptReason != "" {
				t.Errorf("expected no exempt reason, got %q", result.ExemptReason)
			}
		})
	}
}

func TestEngine_Calculate_PricesExcludeVAT(t *testing.T) {
	engine := NewEngine(newTestCache())

	tests := []struct {
		name           string
		productPrice   decimal.Decimal
		rateType       string
		destCountry    string
		wantRate       decimal.Decimal
		wantNetPrice   decimal.Decimal
		wantVATAmount  decimal.Decimal
		wantGrossPrice decimal.Decimal
	}{
		{
			name:           "Spain standard 21% - add VAT to 100",
			productPrice:   decimal.NewFromFloat(100.00),
			rateType:       RateTypeStandard,
			destCountry:    "ES",
			wantRate:       decimal.NewFromFloat(21.0),
			wantNetPrice:   decimal.NewFromFloat(100.00),
			wantVATAmount:  decimal.NewFromFloat(21.00),
			wantGrossPrice: decimal.NewFromFloat(121.00),
		},
		{
			name:           "Germany reduced 7% - add VAT to 50",
			productPrice:   decimal.NewFromFloat(50.00),
			rateType:       RateTypeReduced,
			destCountry:    "DE",
			wantRate:       decimal.NewFromFloat(7.0),
			wantNetPrice:   decimal.NewFromFloat(50.00),
			wantVATAmount:  decimal.NewFromFloat(3.50),
			wantGrossPrice: decimal.NewFromFloat(53.50),
		},
		{
			name:           "Denmark standard 25% - add VAT to 200",
			productPrice:   decimal.NewFromFloat(200.00),
			rateType:       RateTypeStandard,
			destCountry:    "DK",
			wantRate:       decimal.NewFromFloat(25.0),
			wantNetPrice:   decimal.NewFromFloat(200.00),
			wantVATAmount:  decimal.NewFromFloat(50.00),
			wantGrossPrice: decimal.NewFromFloat(250.00),
		},
		{
			name:           "France super reduced 2.1% - small amount",
			productPrice:   decimal.NewFromFloat(10.00),
			rateType:       RateTypeSuperReduced,
			destCountry:    "FR",
			wantRate:       decimal.NewFromFloat(2.1),
			wantNetPrice:   decimal.NewFromFloat(10.00),
			wantVATAmount:  decimal.NewFromFloat(0.21),
			wantGrossPrice: decimal.NewFromFloat(10.21),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Calculate(VATCalculationInput{
				ProductPrice:          tt.productPrice,
				VATCategoryRateType:   tt.rateType,
				DestinationCountry:    tt.destCountry,
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: false,
			})

			if !result.Rate.Equal(tt.wantRate) {
				t.Errorf("rate: want %s, got %s", tt.wantRate.String(), result.Rate.String())
			}
			if !result.NetPrice.Equal(tt.wantNetPrice) {
				t.Errorf("net price: want %s, got %s", tt.wantNetPrice.String(), result.NetPrice.String())
			}
			if !result.Amount.Equal(tt.wantVATAmount) {
				t.Errorf("VAT amount: want %s, got %s", tt.wantVATAmount.String(), result.Amount.String())
			}
			if !result.GrossPrice.Equal(tt.wantGrossPrice) {
				t.Errorf("gross price: want %s, got %s", tt.wantGrossPrice.String(), result.GrossPrice.String())
			}
		})
	}
}

func TestEngine_Calculate_FallbackToStandard(t *testing.T) {
	engine := NewEngine(newTestCache())

	tests := []struct {
		name         string
		rateType     string
		destCountry  string
		wantRate     decimal.Decimal
		wantRateType string
	}{
		{
			name:         "Denmark has no reduced rate, falls back to standard 25%",
			rateType:     RateTypeReduced,
			destCountry:  "DK",
			wantRate:     decimal.NewFromFloat(25.0),
			wantRateType: RateTypeStandard,
		},
		{
			name:         "Denmark has no super_reduced rate, falls back to standard 25%",
			rateType:     RateTypeSuperReduced,
			destCountry:  "DK",
			wantRate:     decimal.NewFromFloat(25.0),
			wantRateType: RateTypeStandard,
		},
		{
			name:         "Germany has no parking rate, falls back to standard 19%",
			rateType:     RateTypeParking,
			destCountry:  "DE",
			wantRate:     decimal.NewFromFloat(19.0),
			wantRateType: RateTypeStandard,
		},
		{
			name:         "Germany has no super_reduced, falls back to standard 19%",
			rateType:     RateTypeSuperReduced,
			destCountry:  "DE",
			wantRate:     decimal.NewFromFloat(19.0),
			wantRateType: RateTypeStandard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Calculate(VATCalculationInput{
				ProductPrice:          decimal.NewFromFloat(100.00),
				VATCategoryRateType:   tt.rateType,
				DestinationCountry:    tt.destCountry,
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: false,
			})

			if !result.Rate.Equal(tt.wantRate) {
				t.Errorf("rate: want %s, got %s", tt.wantRate.String(), result.Rate.String())
			}
			if result.RateType != tt.wantRateType {
				t.Errorf("rate type: want %q, got %q", tt.wantRateType, result.RateType)
			}
		})
	}
}

func TestEngine_Calculate_MultipleCountries(t *testing.T) {
	engine := NewEngine(newTestCache())

	// Same product, same rate type, different destination countries.
	// Prices are net (exclude VAT).
	price := decimal.NewFromFloat(100.00)

	tests := []struct {
		country       string
		wantRate      decimal.Decimal
		wantVATAmount decimal.Decimal
		wantGross     decimal.Decimal
	}{
		{"DE", decimal.NewFromFloat(19.0), decimal.NewFromFloat(19.00), decimal.NewFromFloat(119.00)},
		{"ES", decimal.NewFromFloat(21.0), decimal.NewFromFloat(21.00), decimal.NewFromFloat(121.00)},
		{"FR", decimal.NewFromFloat(20.0), decimal.NewFromFloat(20.00), decimal.NewFromFloat(120.00)},
		{"DK", decimal.NewFromFloat(25.0), decimal.NewFromFloat(25.00), decimal.NewFromFloat(125.00)},
		{"HU", decimal.NewFromFloat(27.0), decimal.NewFromFloat(27.00), decimal.NewFromFloat(127.00)},
		{"BE", decimal.NewFromFloat(21.0), decimal.NewFromFloat(21.00), decimal.NewFromFloat(121.00)},
	}

	for _, tt := range tests {
		t.Run(tt.country, func(t *testing.T) {
			result := engine.Calculate(VATCalculationInput{
				ProductPrice:          price,
				VATCategoryRateType:   RateTypeStandard,
				DestinationCountry:    tt.country,
				StoreCountryCode:      "ES",
				StoreVATEnabled:       true,
				StorePricesIncludeVAT: false,
			})

			if !result.Rate.Equal(tt.wantRate) {
				t.Errorf("rate: want %s, got %s", tt.wantRate.String(), result.Rate.String())
			}
			if !result.Amount.Equal(tt.wantVATAmount) {
				t.Errorf("VAT amount: want %s, got %s", tt.wantVATAmount.String(), result.Amount.String())
			}
			if !result.GrossPrice.Equal(tt.wantGross) {
				t.Errorf("gross price: want %s, got %s", tt.wantGross.String(), result.GrossPrice.String())
			}
			if result.CountryCode != tt.country {
				t.Errorf("country code: want %q, got %q", tt.country, result.CountryCode)
			}
		})
	}
}

func TestEngine_Calculate_UnknownCountry(t *testing.T) {
	engine := NewEngine(newTestCache())

	// Country not in cache should return zero VAT.
	result := engine.Calculate(VATCalculationInput{
		ProductPrice:          decimal.NewFromFloat(100.00),
		VATCategoryRateType:   RateTypeStandard,
		DestinationCountry:    "XX", // Not a real country
		StoreCountryCode:      "ES",
		StoreVATEnabled:       true,
		StorePricesIncludeVAT: false,
	})

	if !result.Rate.IsZero() {
		t.Errorf("expected zero rate for unknown country, got %s", result.Rate.String())
	}
	if !result.Amount.IsZero() {
		t.Errorf("expected zero VAT amount for unknown country, got %s", result.Amount.String())
	}
}

func TestEngine_Calculate_EmptyRateTypeDefaultsToStandard(t *testing.T) {
	engine := NewEngine(newTestCache())

	result := engine.Calculate(VATCalculationInput{
		ProductPrice:          decimal.NewFromFloat(100.00),
		VATCategoryRateType:   "", // Empty defaults to standard
		DestinationCountry:    "ES",
		StoreCountryCode:      "ES",
		StoreVATEnabled:       true,
		StorePricesIncludeVAT: false,
	})

	if !result.Rate.Equal(decimal.NewFromFloat(21.0)) {
		t.Errorf("expected standard rate 21.0 for empty rate type, got %s", result.Rate.String())
	}
	if result.RateType != RateTypeStandard {
		t.Errorf("expected rate type %q, got %q", RateTypeStandard, result.RateType)
	}
}

func TestEngine_Calculate_RoundingPrecision(t *testing.T) {
	engine := NewEngine(newTestCache())

	// Test rounding with amounts that produce repeating decimals.
	// France reduced rate 5.5% on a price of 99.99 (net).
	result := engine.Calculate(VATCalculationInput{
		ProductPrice:          decimal.NewFromFloat(99.99),
		VATCategoryRateType:   RateTypeReduced,
		DestinationCountry:    "FR",
		StoreCountryCode:      "FR",
		StoreVATEnabled:       true,
		StorePricesIncludeVAT: false,
	})

	// 99.99 * 5.5 / 100 = 5.49945, rounded to 5.50
	expectedVAT := decimal.NewFromFloat(5.50)
	if !result.Amount.Equal(expectedVAT) {
		t.Errorf("VAT amount: want %s, got %s", expectedVAT.String(), result.Amount.String())
	}

	// Test VAT-inclusive with an awkward price.
	// Hungary 27% on a gross price of 99.99.
	// net = 99.99 / 1.27 = 78.7322... -> 78.73
	// vat = 99.99 - 78.73 = 21.26
	result2 := engine.Calculate(VATCalculationInput{
		ProductPrice:          decimal.NewFromFloat(99.99),
		VATCategoryRateType:   RateTypeStandard,
		DestinationCountry:    "HU",
		StoreCountryCode:      "HU",
		StoreVATEnabled:       true,
		StorePricesIncludeVAT: true,
	})

	expectedNet := decimal.NewFromFloat(78.73)
	expectedVAT2 := decimal.NewFromFloat(21.26)
	if !result2.NetPrice.Equal(expectedNet) {
		t.Errorf("net price: want %s, got %s", expectedNet.String(), result2.NetPrice.String())
	}
	if !result2.Amount.Equal(expectedVAT2) {
		t.Errorf("VAT amount: want %s, got %s", expectedVAT2.String(), result2.Amount.String())
	}
}

func TestEngine_LookupRate(t *testing.T) {
	engine := NewEngine(newTestCache())

	tests := []struct {
		country   string
		rateType  string
		wantRate  decimal.Decimal
		wantFound bool
	}{
		{"DE", RateTypeStandard, decimal.NewFromFloat(19.0), true},
		{"DE", RateTypeReduced, decimal.NewFromFloat(7.0), true},
		{"DE", RateTypeSuperReduced, decimal.Zero, false}, // Germany has no super_reduced
		{"ES", RateTypeStandard, decimal.NewFromFloat(21.0), true},
		{"ES", RateTypeSuperReduced, decimal.NewFromFloat(4.0), true},
		{"DK", RateTypeStandard, decimal.NewFromFloat(25.0), true},
		{"DK", RateTypeReduced, decimal.Zero, false}, // Denmark has only standard
		{"XX", RateTypeStandard, decimal.Zero, false}, // Unknown country
	}

	for _, tt := range tests {
		name := tt.country + "/" + tt.rateType
		t.Run(name, func(t *testing.T) {
			rate, found := engine.LookupRate(tt.country, tt.rateType)
			if found != tt.wantFound {
				t.Errorf("found: want %v, got %v", tt.wantFound, found)
			}
			if !rate.Equal(tt.wantRate) {
				t.Errorf("rate: want %s, got %s", tt.wantRate.String(), rate.String())
			}
		})
	}
}

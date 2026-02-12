package vat

import (
	"github.com/shopspring/decimal"
)

var (
	hundred = decimal.NewFromInt(100)
	one     = decimal.NewFromInt(1)
)

// Engine performs VAT calculations using the in-memory rate cache.
// It implements the algorithm defined in the project specification:
//
//  1. Check if VAT is enabled
//  2. Check B2B reverse charge eligibility
//  3. Look up rate from cache for destination country + rate type
//  4. Fallback to standard rate if specific rate type not found
//  5. Calculate VAT amount based on whether prices include VAT or not
type Engine struct {
	cache *RateCache
}

// NewEngine creates a new VAT calculation engine with the given rate cache.
func NewEngine(cache *RateCache) *Engine {
	return &Engine{cache: cache}
}

// Calculate performs the VAT calculation for the given input and returns the result.
//
// The calculation follows EU VAT rules:
//   - If VAT is disabled, returns zero VAT.
//   - If B2B reverse charge applies (valid intra-EU VAT number, cross-border),
//     returns zero VAT with reverse_charge reason.
//   - Otherwise, looks up the appropriate rate and calculates VAT.
//   - If the requested rate type is not available for the destination country,
//     falls back to the standard rate.
//   - Uses decimal arithmetic throughout, rounding to 2 decimal places at the end.
func (e *Engine) Calculate(input VATCalculationInput) VATCalculationResult {
	// Step 0: Check if VAT is enabled.
	if !input.StoreVATEnabled {
		return VATCalculationResult{
			Rate:         decimal.Zero,
			RateType:     "",
			Amount:       decimal.Zero,
			NetPrice:     input.ProductPrice,
			GrossPrice:   input.ProductPrice,
			CountryCode:  input.DestinationCountry,
			ExemptReason: ExemptReasonDisabled,
		}
	}

	// Step 1: Check B2B reverse charge.
	// Reverse charge applies when:
	//   - B2B reverse charge is enabled in store settings
	//   - Customer has provided a VAT number
	//   - The destination country is different from the store country (intra-EU)
	// Note: VIES validation is assumed to have already been performed upstream.
	if input.B2BReverseCharge &&
		input.CustomerVATNumber != "" &&
		input.DestinationCountry != input.StoreCountryCode {

		netPrice := input.ProductPrice
		if input.StorePricesIncludeVAT {
			// Need to extract the net price using the store country's standard rate,
			// since the stored price includes VAT at the store's rate.
			storeRate, ok := e.cache.Get(input.StoreCountryCode, RateTypeStandard)
			if ok && storeRate.GreaterThan(decimal.Zero) {
				divisor := one.Add(storeRate.Div(hundred))
				netPrice = input.ProductPrice.Div(divisor).Round(2)
			}
		}

		return VATCalculationResult{
			Rate:           decimal.Zero,
			RateType:       "",
			Amount:         decimal.Zero,
			NetPrice:       netPrice,
			GrossPrice:     netPrice, // No VAT, gross = net.
			CountryCode:    input.DestinationCountry,
			ExemptReason:   ExemptReasonReverseCharge,
			CustomerVATNum: input.CustomerVATNumber,
		}
	}

	// Step 2: Look up the VAT rate for the destination country and rate type.
	rateType := input.VATCategoryRateType
	if rateType == "" {
		rateType = RateTypeStandard
	}

	rate, found := e.LookupRate(input.DestinationCountry, rateType)

	// Step 3: Fallback to standard rate if specific rate type not found.
	if !found && rateType != RateTypeStandard {
		rate, found = e.LookupRate(input.DestinationCountry, RateTypeStandard)
		if found {
			rateType = RateTypeStandard
		}
	}

	// If still no rate found (country not in cache), return zero VAT.
	if !found {
		return VATCalculationResult{
			Rate:        decimal.Zero,
			RateType:    rateType,
			Amount:      decimal.Zero,
			NetPrice:    input.ProductPrice,
			GrossPrice:  input.ProductPrice,
			CountryCode: input.DestinationCountry,
		}
	}

	// Step 4: Calculate VAT amount.
	var netPrice, grossPrice, vatAmount decimal.Decimal

	if input.StorePricesIncludeVAT {
		// Price includes VAT: extract VAT from the price.
		// net = price / (1 + rate/100)
		// vat = price - net
		divisor := one.Add(rate.Div(hundred))
		netPrice = input.ProductPrice.Div(divisor).Round(2)
		vatAmount = input.ProductPrice.Sub(netPrice).Round(2)
		grossPrice = input.ProductPrice
	} else {
		// Price is net: add VAT on top.
		// vat = price * (rate/100)
		// gross = price + vat
		netPrice = input.ProductPrice
		vatAmount = input.ProductPrice.Mul(rate).Div(hundred).Round(2)
		grossPrice = netPrice.Add(vatAmount)
	}

	return VATCalculationResult{
		Rate:        rate,
		RateType:    rateType,
		Amount:      vatAmount,
		NetPrice:    netPrice,
		GrossPrice:  grossPrice,
		CountryCode: input.DestinationCountry,
	}
}

// LookupRate retrieves the VAT rate for a given country and rate type from the cache.
// Returns the rate and true if found, or zero and false if not.
func (e *Engine) LookupRate(countryCode, rateType string) (decimal.Decimal, bool) {
	return e.cache.Get(countryCode, rateType)
}

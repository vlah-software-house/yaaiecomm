package vat

import (
	"time"

	"github.com/shopspring/decimal"
)

// CountryVATRates holds all current rates for a single EU country.
type CountryVATRates struct {
	CountryCode string
	Rates       map[string]decimal.Decimal // rate_type -> rate percentage (e.g., "standard" -> 21.00)
}

// VATRate represents a single rate row from the database.
type VATRate struct {
	ID          string
	CountryCode string
	RateType    string
	Rate        decimal.Decimal
	Description string
	ValidFrom   time.Time
	ValidTo     *time.Time
	Source      string
	SyncedAt    time.Time
}

// SyncResult holds the outcome of a VAT rate sync operation.
type SyncResult struct {
	Source       string // "ec_tedb", "euvatrates_json", "cache"
	RatesLoaded  int
	RatesChanged int
	SyncedAt     time.Time
	Error        error
}

// RateChange describes a single rate change detected during sync.
type RateChange struct {
	CountryCode string
	RateType    string
	OldRate     decimal.Decimal
	NewRate     decimal.Decimal
}

// VIESResult holds the VIES validation response.
type VIESResult struct {
	Valid              bool
	CompanyName        string
	CompanyAddress     string
	ConsultationNumber string
	CountryCode        string
	VATNumber          string
}

// VATCalculationInput holds the inputs for VAT calculation.
type VATCalculationInput struct {
	ProductPrice          decimal.Decimal
	VATCategoryRateType   string // "standard", "reduced", "reduced_alt", "super_reduced", "parking", "zero"
	DestinationCountry    string
	CustomerVATNumber     string // empty if B2C
	StorePricesIncludeVAT bool
	StoreCountryCode      string
	StoreVATEnabled       bool
	B2BReverseCharge      bool
}

// VATCalculationResult holds the output of VAT calculation.
type VATCalculationResult struct {
	Rate           decimal.Decimal
	RateType       string
	Amount         decimal.Decimal
	NetPrice       decimal.Decimal
	GrossPrice     decimal.Decimal
	CountryCode    string
	ExemptReason   string // "vat_disabled", "reverse_charge", ""
	CustomerVATNum string
	CompanyName    string
}

// Known EU VAT rate types.
const (
	RateTypeStandard     = "standard"
	RateTypeReduced      = "reduced"
	RateTypeReducedAlt   = "reduced_alt"
	RateTypeSuperReduced = "super_reduced"
	RateTypeParking      = "parking"
	RateTypeZero         = "zero"
)

// Sync source identifiers.
const (
	SourceECTEDB         = "ec_tedb"
	SourceEUVATRatesJSON = "euvatrates_json"
	SourceManual         = "manual"
	SourceSeed           = "seed"
	SourceCache          = "cache"
)

// Exempt reason constants.
const (
	ExemptReasonDisabled      = "vat_disabled"
	ExemptReasonReverseCharge = "reverse_charge"
)

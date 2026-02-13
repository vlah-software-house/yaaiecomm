package vat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	db "github.com/forgecommerce/api/internal/database/gen"
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

// ---------------------------------------------------------------------------
// VATService — database-integrated VAT calculation
// ---------------------------------------------------------------------------

// VATInput holds the high-level inputs for a VAT calculation that requires
// database lookups to resolve the product's VAT category and VIES status.
type VATInput struct {
	ProductID            uuid.UUID       // the product being purchased
	ProductVATCategoryID *uuid.UUID      // from product.vat_category_id (nullable)
	Price                decimal.Decimal // the unit price to calculate VAT for
	DestinationCountry   string          // ISO 3166-1 alpha-2
	CustomerVATNumber    string          // optional, for B2B reverse charge
	Quantity             int32           // item quantity, for line total calculation
}

// VATResult holds the full output of a database-integrated VAT calculation,
// including per-unit and line-level totals.
type VATResult struct {
	Rate          decimal.Decimal // e.g., 21.00
	RateType      string          // "standard", "reduced", etc.
	Amount        decimal.Decimal // VAT amount per unit
	NetPrice      decimal.Decimal // unit price without VAT
	GrossPrice    decimal.Decimal // unit price with VAT
	CountryCode   string
	ExemptReason  string // "vat_disabled", "reverse_charge", or ""
	ReverseCharge bool

	// B2B fields, populated when reverse charge applies.
	CustomerVATNumber string
	CompanyName       string

	// Line-level totals (unit values * quantity).
	LineNetTotal   decimal.Decimal
	LineVATTotal   decimal.Decimal
	LineGrossTotal decimal.Decimal
}

// VATService is the database-integrated VAT calculation service. It resolves
// store settings, product VAT category overrides, and VIES cache entries from
// the database, then delegates the pure arithmetic to the underlying Engine.
//
// This is the primary entry point for VAT calculations in handlers and
// services that have a database connection.
type VATService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	engine  *Engine
	logger  *slog.Logger
}

// NewVATService creates a new database-integrated VAT calculation service.
//
// Parameters:
//   - pool: PostgreSQL connection pool for direct queries (VIES cache lookup)
//   - rateCache: in-memory cache of current VAT rates (shared with RateSyncer)
//   - logger: structured logger for operational logging
func NewVATService(pool *pgxpool.Pool, rateCache *RateCache, logger *slog.Logger) *VATService {
	return &VATService{
		pool:    pool,
		queries: db.New(pool),
		engine:  NewEngine(rateCache),
		logger:  logger,
	}
}

// CalculateForProduct performs a full VAT calculation for a product being sold
// to a destination country, with optional B2B reverse charge via VIES
// validation cache lookup.
//
// The algorithm:
//  1. Load store settings to check VAT enabled, store country, pricing mode,
//     B2B reverse charge, and default VAT category.
//  2. If VAT is disabled, return zero result immediately.
//  3. If B2B reverse charge is enabled and customer provided a VAT number on a
//     cross-border sale, look up the VIES validation cache. If the number is
//     valid (and not expired), apply reverse charge (0% VAT).
//  4. Resolve the VAT category for this product + destination country:
//     a. Check for a ProductVATOverride (product_id, country_code).
//     b. Fall back to the product's own vat_category_id.
//     c. Fall back to the store's default VAT category name.
//  5. Map the resolved VAT category to a rate_type (e.g., "standard", "reduced").
//  6. Delegate to Engine.Calculate for the pure arithmetic.
//  7. Multiply per-unit results by quantity for line totals.
func (s *VATService) CalculateForProduct(ctx context.Context, input VATInput) (VATResult, error) {
	// Step 1: Load store settings.
	settings, err := s.queries.GetStoreSettings(ctx)
	if err != nil {
		return VATResult{}, fmt.Errorf("loading store settings for VAT calculation: %w", err)
	}

	// Step 2: Check if VAT is enabled.
	if !settings.VatEnabled {
		return s.buildResult(VATCalculationResult{
			Rate:         decimal.Zero,
			Amount:       decimal.Zero,
			NetPrice:     input.Price,
			GrossPrice:   input.Price,
			CountryCode:  input.DestinationCountry,
			ExemptReason: ExemptReasonDisabled,
		}, input.Quantity, false, "", ""), nil
	}

	storeCountry := ""
	if settings.VatCountryCode != nil {
		storeCountry = *settings.VatCountryCode
	}

	// Step 3: Check B2B reverse charge.
	customerVATNum := sanitizeVATNumber(input.CustomerVATNumber)
	reverseCharge := false
	companyName := ""

	if settings.VatB2bReverseChargeEnabled &&
		customerVATNum != "" &&
		input.DestinationCountry != storeCountry {

		valid, viesCompanyName, viesErr := s.lookupVIESCache(ctx, customerVATNum)
		if viesErr != nil {
			s.logger.Warn("VIES cache lookup failed, proceeding without reverse charge",
				"vat_number", customerVATNum,
				"error", viesErr,
			)
		} else if valid {
			reverseCharge = true
			companyName = viesCompanyName
		}
	}

	// Step 4: Resolve VAT category rate type.
	rateType, err := s.resolveRateType(ctx, input.ProductID, input.ProductVATCategoryID, input.DestinationCountry, settings.VatDefaultCategory)
	if err != nil {
		s.logger.Warn("failed to resolve VAT category, using standard rate",
			"product_id", input.ProductID,
			"destination", input.DestinationCountry,
			"error", err,
		)
		rateType = RateTypeStandard
	}

	// Step 5: Build calculation input and delegate to the pure engine.
	calcInput := VATCalculationInput{
		ProductPrice:          input.Price,
		VATCategoryRateType:   rateType,
		DestinationCountry:    input.DestinationCountry,
		CustomerVATNumber:     customerVATNum,
		StorePricesIncludeVAT: settings.VatPricesIncludeVat,
		StoreCountryCode:      storeCountry,
		StoreVATEnabled:       true, // Already checked above.
		B2BReverseCharge:      reverseCharge,
	}

	calcResult := s.engine.Calculate(calcInput)

	return s.buildResult(calcResult, input.Quantity, reverseCharge, customerVATNum, companyName), nil
}

// CalculateForCart is a convenience method that calculates VAT for multiple
// items and returns a per-item result slice plus aggregate totals.
func (s *VATService) CalculateForCart(ctx context.Context, items []VATInput) ([]VATResult, CartVATSummary, error) {
	if len(items) == 0 {
		return nil, CartVATSummary{}, nil
	}

	results := make([]VATResult, 0, len(items))
	totalNet := decimal.Zero
	totalVAT := decimal.Zero
	totalGross := decimal.Zero

	for _, item := range items {
		result, err := s.CalculateForProduct(ctx, item)
		if err != nil {
			return nil, CartVATSummary{}, fmt.Errorf("calculating VAT for product %s: %w", item.ProductID, err)
		}
		results = append(results, result)

		totalNet = totalNet.Add(result.LineNetTotal)
		totalVAT = totalVAT.Add(result.LineVATTotal)
		totalGross = totalGross.Add(result.LineGrossTotal)
	}

	summary := CartVATSummary{
		TotalNet:   totalNet,
		TotalVAT:   totalVAT,
		TotalGross: totalGross,
	}

	return results, summary, nil
}

// CartVATSummary holds the aggregate VAT totals for a cart.
type CartVATSummary struct {
	TotalNet   decimal.Decimal
	TotalVAT   decimal.Decimal
	TotalGross decimal.Decimal
}

// resolveRateType determines the VAT rate type for a product being sold to a
// specific country. It follows the priority chain:
//  1. ProductVATOverride (product_id, country_code) -> category -> maps_to_rate_type
//  2. Product-level vat_category_id -> category -> maps_to_rate_type
//  3. Store default VAT category name -> category -> maps_to_rate_type
func (s *VATService) resolveRateType(
	ctx context.Context,
	productID uuid.UUID,
	productVATCategoryID *uuid.UUID,
	countryCode string,
	storeDefaultCategoryName string,
) (string, error) {
	// Priority 1: Check for a per-product per-country override.
	overrideCategoryID, err := s.getProductCountryOverrideCategoryID(ctx, productID, countryCode)
	if err == nil && overrideCategoryID != uuid.Nil {
		rateType, catErr := s.getCategoryRateType(ctx, overrideCategoryID)
		if catErr == nil {
			return rateType, nil
		}
		s.logger.Warn("override category lookup failed, falling through",
			"category_id", overrideCategoryID,
			"error", catErr,
		)
	}

	// Priority 2: Use the product's own VAT category.
	if productVATCategoryID != nil && *productVATCategoryID != uuid.Nil {
		rateType, catErr := s.getCategoryRateType(ctx, *productVATCategoryID)
		if catErr == nil {
			return rateType, nil
		}
		s.logger.Warn("product category lookup failed, falling through",
			"category_id", *productVATCategoryID,
			"error", catErr,
		)
	}

	// Priority 3: Use the store default VAT category.
	if storeDefaultCategoryName != "" {
		cat, catErr := s.queries.GetVATCategoryByName(ctx, storeDefaultCategoryName)
		if catErr == nil {
			return cat.MapsToRateType, nil
		}
		s.logger.Warn("store default category lookup failed",
			"category_name", storeDefaultCategoryName,
			"error", catErr,
		)
	}

	// Ultimate fallback: standard rate.
	return RateTypeStandard, nil
}

// getProductCountryOverrideCategoryID looks up a per-product per-country VAT
// override and returns the override's VAT category ID.
// Returns uuid.Nil if no override exists.
func (s *VATService) getProductCountryOverrideCategoryID(ctx context.Context, productID uuid.UUID, countryCode string) (uuid.UUID, error) {
	// The sqlc-generated queries don't have a single GetProductVATOverride(product_id, country_code)
	// query, so we use ListProductVATOverrides and filter. For a single product this is efficient
	// because the number of country overrides per product is small (typically 0-5).
	overrides, err := s.queries.ListProductVATOverrides(ctx, productID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("listing product VAT overrides: %w", err)
	}

	for _, o := range overrides {
		if o.CountryCode == countryCode {
			return o.VatCategoryID, nil
		}
	}

	return uuid.Nil, fmt.Errorf("no override for country %s", countryCode)
}

// getCategoryRateType fetches a VATCategory by ID and returns its maps_to_rate_type.
func (s *VATService) getCategoryRateType(ctx context.Context, categoryID uuid.UUID) (string, error) {
	cat, err := s.queries.GetVATCategory(ctx, categoryID)
	if err != nil {
		return "", fmt.Errorf("getting VAT category %s: %w", categoryID, err)
	}
	return cat.MapsToRateType, nil
}

// lookupVIESCache checks the VIES validation cache in the database for a
// previously validated VAT number. Returns (isValid, companyName, error).
//
// A cached entry is considered valid only if:
//   - it exists in the database
//   - is_valid is true
//   - expires_at is in the future
//
// If the entry is missing or expired, it returns (false, "", nil) — the caller
// should treat this as "not validated" and proceed without reverse charge.
// A live VIES SOAP call would be performed separately (e.g., at checkout
// validation time) and the result cached.
func (s *VATService) lookupVIESCache(ctx context.Context, vatNumber string) (bool, string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT is_valid, company_name, expires_at
		FROM vies_validation_cache
		WHERE vat_number = $1
		LIMIT 1
	`, vatNumber)

	var isValid bool
	var companyName *string
	var expiresAt time.Time

	err := row.Scan(&isValid, &companyName, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			// No cached validation — not an error, just not validated.
			return false, "", nil
		}
		return false, "", fmt.Errorf("querying VIES cache for %s: %w", vatNumber, err)
	}

	// Check expiry.
	if time.Now().UTC().After(expiresAt) {
		s.logger.Debug("VIES cache entry expired",
			"vat_number", vatNumber,
			"expired_at", expiresAt,
		)
		return false, "", nil
	}

	name := ""
	if companyName != nil {
		name = *companyName
	}

	return isValid, name, nil
}

// buildResult converts an Engine-level VATCalculationResult into a full
// VATResult with line-level totals and B2B fields.
func (s *VATService) buildResult(
	calc VATCalculationResult,
	quantity int32,
	reverseCharge bool,
	customerVATNum string,
	companyName string,
) VATResult {
	qty := decimal.NewFromInt32(quantity)
	if qty.IsZero() {
		qty = one
	}

	result := VATResult{
		Rate:         calc.Rate,
		RateType:     calc.RateType,
		Amount:       calc.Amount,
		NetPrice:     calc.NetPrice,
		GrossPrice:   calc.GrossPrice,
		CountryCode:  calc.CountryCode,
		ExemptReason: calc.ExemptReason,
	}

	// Line-level totals.
	result.LineNetTotal = calc.NetPrice.Mul(qty).Round(2)
	result.LineVATTotal = calc.Amount.Mul(qty).Round(2)
	result.LineGrossTotal = calc.GrossPrice.Mul(qty).Round(2)

	// B2B fields.
	if reverseCharge || calc.ExemptReason == ExemptReasonReverseCharge {
		result.ReverseCharge = true
		result.CustomerVATNumber = customerVATNum
		result.CompanyName = companyName
	}

	// Carry forward from calc result if set there.
	if calc.CustomerVATNum != "" && result.CustomerVATNumber == "" {
		result.CustomerVATNumber = calc.CustomerVATNum
	}
	if calc.CompanyName != "" && result.CompanyName == "" {
		result.CompanyName = calc.CompanyName
	}

	return result
}

// sanitizeVATNumber normalizes a VAT number by stripping whitespace and
// converting to uppercase. This is important for consistent lookups against
// the VIES cache and for comparison with stored values.
func sanitizeVATNumber(vatNumber string) string {
	if vatNumber == "" {
		return ""
	}
	// Remove all whitespace (spaces, tabs, etc.) and dots.
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '.' {
			return -1 // Drop the character.
		}
		return r
	}, vatNumber)
	return strings.ToUpper(cleaned)
}

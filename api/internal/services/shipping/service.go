package shipping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrShippingDisabled is returned when shipping is not enabled in the store config.
	ErrShippingDisabled = errors.New("shipping is not enabled")

	// ErrCountryNotEnabled is returned when the destination country is not in the
	// store's list of enabled shipping countries.
	ErrCountryNotEnabled = errors.New("shipping to this country is not enabled")

	// ErrConfigNotFound is returned when no shipping config row exists.
	ErrConfigNotFound = errors.New("shipping config not found")

	// ErrZoneNotFound is returned when a shipping zone does not exist.
	ErrZoneNotFound = errors.New("shipping zone not found")

	// ErrNoMatchingBracket is returned when weight-based calculation finds no
	// bracket matching the shipment weight.
	ErrNoMatchingBracket = errors.New("no weight bracket matches the shipment weight")
)

// CalculateParams holds the inputs needed for a shipping fee calculation.
type CalculateParams struct {
	CountryCode  string
	TotalWeightG int // total weight in grams
	Items        []ShippingItem
	Subtotal     decimal.Decimal
}

// ShippingItem represents a line item's shipping-specific data.
type ShippingItem struct {
	ProductExtraFee decimal.Decimal
	Quantity        int
}

// ShippingResult contains the full breakdown of a shipping fee calculation.
type ShippingResult struct {
	BaseFee      decimal.Decimal
	ExtraFees    decimal.Decimal
	TotalFee     decimal.Decimal
	FreeShipping bool
	Method       string // "fixed", "weight_based", "size_based"
}

// WeightBracket defines a weight range and its corresponding shipping fee.
type WeightBracket struct {
	MinWeightG int             `json:"min_weight_g"`
	MaxWeightG int             `json:"max_weight_g"`
	Fee        decimal.Decimal `json:"fee"`
}

// SizeRate defines a volumetric/size-based shipping rate.
type SizeRate struct {
	BaseFee  decimal.Decimal `json:"base_fee"`
	PerKgFee decimal.Decimal `json:"per_kg_fee"`
	MinFee   decimal.Decimal `json:"min_fee"`
}

// UpdateConfigParams holds the mutable fields for updating the global shipping config.
type UpdateConfigParams struct {
	Enabled               bool
	CalculationMethod     string
	FixedFee              decimal.Decimal
	WeightRates           json.RawMessage
	SizeRates             json.RawMessage
	FreeShippingThreshold decimal.Decimal
	DefaultCurrency       string
}

// CreateZoneParams holds the fields required to create a shipping zone.
type CreateZoneParams struct {
	Name              string
	Countries         []string
	CalculationMethod string
	Rates             json.RawMessage
	Position          int32
}

// UpdateZoneParams holds the fields required to update a shipping zone.
type UpdateZoneParams struct {
	Name              string
	Countries         []string
	CalculationMethod string
	Rates             json.RawMessage
	Position          int32
}

// Service provides business logic for shipping calculation and configuration.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new shipping service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		pool:    pool,
		logger:  logger,
	}
}

// ---------------------------------------------------------------------------
// Shipping fee calculation
// ---------------------------------------------------------------------------

// Calculate computes the shipping fee for a given destination country and cart.
//
// The algorithm:
//  1. Load global config; if shipping is disabled, return a zero-fee result.
//  2. Verify the destination country is in the enabled shipping countries list.
//  3. Look up a shipping zone for the country (optional).
//  4. Determine calculation method and rates from the zone (preferred) or global config.
//  5. Compute the base fee (fixed / weight-based / size-based).
//  6. Sum per-item extra fees (product.shipping_extra_fee_per_unit * qty).
//  7. If the order subtotal meets or exceeds the free shipping threshold, waive the base fee.
//  8. Return the full breakdown.
func (s *Service) Calculate(ctx context.Context, params CalculateParams) (ShippingResult, error) {
	// Step 1: Load global shipping config.
	config, err := s.queries.GetShippingConfig(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ShippingResult{}, ErrConfigNotFound
		}
		return ShippingResult{}, fmt.Errorf("fetching shipping config: %w", err)
	}

	if !config.Enabled {
		return ShippingResult{
			BaseFee:      decimal.Zero,
			ExtraFees:    decimal.Zero,
			TotalFee:     decimal.Zero,
			FreeShipping: false,
			Method:       config.CalculationMethod,
		}, nil
	}

	// Step 2: Check that the destination country is enabled.
	enabledCountries, err := s.queries.ListEnabledShippingCountries(ctx)
	if err != nil {
		return ShippingResult{}, fmt.Errorf("listing enabled shipping countries: %w", err)
	}

	if !isCountryEnabled(params.CountryCode, enabledCountries) {
		return ShippingResult{}, ErrCountryNotEnabled
	}

	// Step 3: Look for a shipping zone that covers the destination country.
	calcMethod := config.CalculationMethod
	var rates json.RawMessage

	zone, err := s.queries.GetShippingZoneForCountry(ctx, params.CountryCode)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ShippingResult{}, fmt.Errorf("looking up shipping zone for country %s: %w", params.CountryCode, err)
	}

	// Step 4: Use zone overrides if a zone was found; otherwise fall back to global config.
	if err == nil {
		// Zone found — use its method and rates.
		calcMethod = zone.CalculationMethod
		rates = zone.Rates
	} else {
		// No zone — use global config rates depending on the method.
		switch calcMethod {
		case "weight_based":
			rates = config.WeightRates
		case "size_based":
			rates = config.SizeRates
		default:
			// "fixed" — rates are not needed; we read FixedFee directly.
		}
	}

	// Step 5: Calculate the base fee.
	baseFee, method, err := s.calculateBaseFee(calcMethod, rates, config, params)
	if err != nil {
		return ShippingResult{}, fmt.Errorf("calculating base fee: %w", err)
	}

	// Step 6: Sum per-item extra fees.
	extraFees := decimal.Zero
	for _, item := range params.Items {
		if item.Quantity > 0 && item.ProductExtraFee.IsPositive() {
			extraFees = extraFees.Add(
				item.ProductExtraFee.Mul(decimal.NewFromInt(int64(item.Quantity))),
			)
		}
	}

	// Step 7: Apply free shipping threshold (waives the base fee, not per-item extras).
	freeShipping := false
	threshold := numericToDecimal(config.FreeShippingThreshold)
	if threshold.IsPositive() && params.Subtotal.GreaterThanOrEqual(threshold) {
		freeShipping = true
		baseFee = decimal.Zero
	}

	// Step 8: Build result.
	totalFee := baseFee.Add(extraFees)
	if totalFee.IsNegative() {
		totalFee = decimal.Zero
	}

	result := ShippingResult{
		BaseFee:      baseFee.Round(2),
		ExtraFees:    extraFees.Round(2),
		TotalFee:     totalFee.Round(2),
		FreeShipping: freeShipping,
		Method:       method,
	}

	s.logger.Info("shipping fee calculated",
		slog.String("country", params.CountryCode),
		slog.String("method", method),
		slog.String("base_fee", baseFee.StringFixed(2)),
		slog.String("extra_fees", extraFees.StringFixed(2)),
		slog.String("total_fee", totalFee.StringFixed(2)),
		slog.Bool("free_shipping", freeShipping),
	)

	return result, nil
}

// calculateBaseFee dispatches to the correct calculation strategy.
func (s *Service) calculateBaseFee(
	method string,
	rates json.RawMessage,
	config db.ShippingConfig,
	params CalculateParams,
) (decimal.Decimal, string, error) {
	switch method {
	case "fixed":
		fee := numericToDecimal(config.FixedFee)
		// If rates came from a zone, parse the fixed fee from the zone rates JSON.
		if len(rates) > 0 {
			var zoneFixed struct {
				FixedFee decimal.Decimal `json:"fixed_fee"`
			}
			if err := json.Unmarshal(rates, &zoneFixed); err == nil && zoneFixed.FixedFee.IsPositive() {
				fee = zoneFixed.FixedFee
			}
		}
		return fee, "fixed", nil

	case "weight_based":
		fee, err := calculateWeightBasedFee(rates, params.TotalWeightG)
		if err != nil {
			return decimal.Zero, "weight_based", err
		}
		return fee, "weight_based", nil

	case "size_based":
		fee, err := calculateSizeBasedFee(rates, params.TotalWeightG)
		if err != nil {
			return decimal.Zero, "size_based", err
		}
		return fee, "size_based", nil

	default:
		return decimal.Zero, method, fmt.Errorf("unsupported calculation method: %s", method)
	}
}

// calculateWeightBasedFee finds the matching weight bracket and returns its fee.
func calculateWeightBasedFee(ratesJSON json.RawMessage, weightG int) (decimal.Decimal, error) {
	if len(ratesJSON) == 0 {
		return decimal.Zero, ErrNoMatchingBracket
	}

	var brackets []WeightBracket
	if err := json.Unmarshal(ratesJSON, &brackets); err != nil {
		return decimal.Zero, fmt.Errorf("parsing weight brackets: %w", err)
	}

	for _, b := range brackets {
		// MaxWeightG of 0 means "no upper limit" (the last open-ended bracket).
		if weightG >= b.MinWeightG && (b.MaxWeightG == 0 || weightG <= b.MaxWeightG) {
			return b.Fee, nil
		}
	}

	return decimal.Zero, ErrNoMatchingBracket
}

// calculateSizeBasedFee uses a per-kg model to compute the fee.
// The rates JSON is expected to be a SizeRate object with base_fee, per_kg_fee,
// and min_fee fields.
func calculateSizeBasedFee(ratesJSON json.RawMessage, weightG int) (decimal.Decimal, error) {
	if len(ratesJSON) == 0 {
		return decimal.Zero, fmt.Errorf("size rates configuration is empty")
	}

	var sizeRate SizeRate
	if err := json.Unmarshal(ratesJSON, &sizeRate); err != nil {
		return decimal.Zero, fmt.Errorf("parsing size rates: %w", err)
	}

	// Convert grams to kilograms (round up to nearest kg for billing).
	weightKg := decimal.NewFromInt(int64(weightG)).Div(decimal.NewFromInt(1000)).Ceil()

	fee := sizeRate.BaseFee.Add(sizeRate.PerKgFee.Mul(weightKg))

	// Enforce minimum fee.
	if fee.LessThan(sizeRate.MinFee) {
		fee = sizeRate.MinFee
	}

	return fee, nil
}

// ---------------------------------------------------------------------------
// Config CRUD
// ---------------------------------------------------------------------------

// GetConfig returns the global shipping configuration.
func (s *Service) GetConfig(ctx context.Context) (db.ShippingConfig, error) {
	config, err := s.queries.GetShippingConfig(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ShippingConfig{}, ErrConfigNotFound
		}
		return db.ShippingConfig{}, fmt.Errorf("fetching shipping config: %w", err)
	}
	return config, nil
}

// UpdateConfig updates the global shipping configuration.
func (s *Service) UpdateConfig(ctx context.Context, params UpdateConfigParams) (db.ShippingConfig, error) {
	config, err := s.queries.UpdateShippingConfig(ctx, db.UpdateShippingConfigParams{
		Enabled:               params.Enabled,
		CalculationMethod:     params.CalculationMethod,
		FixedFee:              decimalToNumeric(params.FixedFee),
		WeightRates:           params.WeightRates,
		SizeRates:             params.SizeRates,
		FreeShippingThreshold: decimalToNumeric(params.FreeShippingThreshold),
		UpdatedAt:             time.Now(),
	})
	if err != nil {
		return db.ShippingConfig{}, fmt.Errorf("updating shipping config: %w", err)
	}

	s.logger.Info("shipping config updated",
		slog.Bool("enabled", params.Enabled),
		slog.String("method", params.CalculationMethod),
		slog.String("currency", params.DefaultCurrency),
	)

	return config, nil
}

// ---------------------------------------------------------------------------
// Zone CRUD
// ---------------------------------------------------------------------------

// ListZones returns all shipping zones ordered by position.
func (s *Service) ListZones(ctx context.Context) ([]db.ShippingZone, error) {
	zones, err := s.queries.ListShippingZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing shipping zones: %w", err)
	}
	return zones, nil
}

// GetZone returns a single shipping zone by ID.
func (s *Service) GetZone(ctx context.Context, id uuid.UUID) (db.ShippingZone, error) {
	zone, err := s.queries.GetShippingZone(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ShippingZone{}, ErrZoneNotFound
		}
		return db.ShippingZone{}, fmt.Errorf("fetching shipping zone %s: %w", id, err)
	}
	return zone, nil
}

// CreateZone creates a new shipping zone.
func (s *Service) CreateZone(ctx context.Context, params CreateZoneParams) (db.ShippingZone, error) {
	zone, err := s.queries.CreateShippingZone(ctx, db.CreateShippingZoneParams{
		ID:                uuid.New(),
		Name:              params.Name,
		Countries:         params.Countries,
		CalculationMethod: params.CalculationMethod,
		Rates:             params.Rates,
		Position:          params.Position,
	})
	if err != nil {
		return db.ShippingZone{}, fmt.Errorf("creating shipping zone: %w", err)
	}

	s.logger.Info("shipping zone created",
		slog.String("zone_id", zone.ID.String()),
		slog.String("name", zone.Name),
		slog.Int("countries", len(params.Countries)),
	)

	return zone, nil
}

// UpdateZone updates an existing shipping zone.
func (s *Service) UpdateZone(ctx context.Context, id uuid.UUID, params UpdateZoneParams) (db.ShippingZone, error) {
	// Verify the zone exists.
	_, err := s.queries.GetShippingZone(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ShippingZone{}, ErrZoneNotFound
		}
		return db.ShippingZone{}, fmt.Errorf("fetching zone for update: %w", err)
	}

	zone, err := s.queries.UpdateShippingZone(ctx, db.UpdateShippingZoneParams{
		ID:                id,
		Name:              params.Name,
		Countries:         params.Countries,
		CalculationMethod: params.CalculationMethod,
		Rates:             params.Rates,
		Position:          params.Position,
	})
	if err != nil {
		return db.ShippingZone{}, fmt.Errorf("updating shipping zone %s: %w", id, err)
	}

	s.logger.Info("shipping zone updated",
		slog.String("zone_id", zone.ID.String()),
		slog.String("name", zone.Name),
	)

	return zone, nil
}

// DeleteZone deletes a shipping zone by ID.
func (s *Service) DeleteZone(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrZoneNotFound.
	_, err := s.queries.GetShippingZone(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrZoneNotFound
		}
		return fmt.Errorf("fetching zone for delete: %w", err)
	}

	if err := s.queries.DeleteShippingZone(ctx, id); err != nil {
		return fmt.Errorf("deleting shipping zone %s: %w", id, err)
	}

	s.logger.Info("shipping zone deleted", slog.String("zone_id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isCountryEnabled checks whether a country code exists in the list of enabled
// shipping countries. The enabledCountries slice contains StoreShippingCountry
// rows where is_enabled = true.
func isCountryEnabled(code string, enabledCountries []db.EuCountry) bool {
	for _, c := range enabledCountries {
		if c.CountryCode == code {
			return true
		}
	}
	return false
}

// numericToDecimal converts a pgtype.Numeric to a shopspring Decimal.
// Returns decimal.Zero if the Numeric is not valid.
func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(n.Int, n.Exp)
}

// decimalToNumeric converts a shopspring Decimal to a pgtype.Numeric.
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	coefficient := d.Coefficient()
	exp := int32(d.Exponent())
	return pgtype.Numeric{
		Int:   coefficient,
		Exp:   exp,
		Valid: true,
	}
}

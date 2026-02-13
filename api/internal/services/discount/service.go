package discount

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// Sentinel errors returned by the discount service.
var (
	// ErrNotFound is returned when a discount does not exist.
	ErrNotFound = errors.New("discount not found")

	// ErrCouponNotFound is returned when a coupon code does not exist.
	ErrCouponNotFound = errors.New("coupon not found")

	// ErrCouponExpired is returned when a coupon is inactive or outside its valid date range.
	ErrCouponExpired = errors.New("coupon is expired or inactive")

	// ErrCouponUsageLimitReached is returned when a coupon has reached its usage limit.
	ErrCouponUsageLimitReached = errors.New("coupon usage limit reached")

	// ErrMinimumNotMet is returned when the order amount does not meet the discount minimum.
	ErrMinimumNotMet = errors.New("minimum order amount not met")
)

// bigZero is a reusable zero value for big.Int comparisons.
var bigZero = big.NewInt(0)

// Service provides business logic for discounts and coupons.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new discount service with constructor injection.
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

// --------------------------------------------------------------------------
// Domain types
// --------------------------------------------------------------------------

// ApplyParams holds the inputs for calculating applicable discounts.
type ApplyParams struct {
	Subtotal    pgtype.Numeric
	ShippingFee pgtype.Numeric
	CouponCode  string    // optional — empty string means no coupon
	CustomerID  uuid.UUID // zero value for guest customers
}

// ApplyResult holds the output of the discount calculation.
type ApplyResult struct {
	TotalDiscount pgtype.Numeric
	DiscountID    pgtype.UUID // primary discount applied (first in breakdown)
	CouponID      pgtype.UUID // set when a coupon was used
	Breakdown     []DiscountApplication
}

// DiscountApplication describes a single discount that was applied.
type DiscountApplication struct {
	DiscountID   uuid.UUID
	DiscountName string
	Type         string         // "percentage" or "fixed_amount"
	Value        pgtype.Numeric // the configured value (e.g. 10 for 10% or 5.00 for fixed)
	Scope        string         // "subtotal", "shipping", or "total"
	Amount       pgtype.Numeric // actual monetary amount deducted
}

// CreateDiscountParams contains the input fields for creating a discount.
type CreateDiscountParams struct {
	Name            string
	Type            string // "percentage" or "fixed_amount"
	Value           pgtype.Numeric
	Scope           string // "subtotal", "shipping", or "total"
	MinimumAmount   pgtype.Numeric
	MaximumDiscount pgtype.Numeric
	StartsAt        pgtype.Timestamptz
	EndsAt          pgtype.Timestamptz
	IsActive        bool
	Priority        int32
	Stackable       bool
	Conditions      json.RawMessage
}

// UpdateDiscountParams contains the input fields for updating a discount.
type UpdateDiscountParams struct {
	Name            string
	Type            string
	Value           pgtype.Numeric
	Scope           string
	MinimumAmount   pgtype.Numeric
	MaximumDiscount pgtype.Numeric
	StartsAt        pgtype.Timestamptz
	EndsAt          pgtype.Timestamptz
	IsActive        bool
	Priority        int32
	Stackable       bool
	Conditions      json.RawMessage
}

// CreateCouponParams contains the input fields for creating a coupon.
type CreateCouponParams struct {
	Code                  string
	DiscountID            uuid.UUID
	UsageLimit            *int32
	UsageLimitPerCustomer *int32
	StartsAt              pgtype.Timestamptz
	EndsAt                pgtype.Timestamptz
	IsActive              bool
}

// --------------------------------------------------------------------------
// Discount CRUD
// --------------------------------------------------------------------------

// GetDiscount retrieves a discount by ID.
func (s *Service) GetDiscount(ctx context.Context, id uuid.UUID) (db.Discount, error) {
	d, err := s.queries.GetDiscount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Discount{}, ErrNotFound
		}
		return db.Discount{}, fmt.Errorf("getting discount: %w", err)
	}
	return d, nil
}

// ListDiscounts retrieves a paginated list of discounts ordered by creation date descending.
func (s *Service) ListDiscounts(ctx context.Context, limit, offset int32) ([]db.Discount, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	discounts, err := s.queries.ListDiscounts(ctx, db.ListDiscountsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("listing discounts: %w", err)
	}
	return discounts, nil
}

// CreateDiscount persists a new discount and returns it.
func (s *Service) CreateDiscount(ctx context.Context, p CreateDiscountParams) (db.Discount, error) {
	now := time.Now().UTC()
	conditions := p.Conditions
	if conditions == nil {
		conditions = json.RawMessage(`{}`)
	}

	d, err := s.queries.CreateDiscount(ctx, db.CreateDiscountParams{
		ID:              uuid.New(),
		Name:            p.Name,
		Type:            p.Type,
		Value:           p.Value,
		Scope:           p.Scope,
		MinimumAmount:   p.MinimumAmount,
		MaximumDiscount: p.MaximumDiscount,
		StartsAt:        p.StartsAt,
		EndsAt:          p.EndsAt,
		IsActive:        p.IsActive,
		Priority:        p.Priority,
		Stackable:       p.Stackable,
		Conditions:      conditions,
		CreatedAt:       now,
	})
	if err != nil {
		return db.Discount{}, fmt.Errorf("creating discount: %w", err)
	}

	s.logger.Info("discount created",
		slog.String("id", d.ID.String()),
		slog.String("name", d.Name),
		slog.String("type", d.Type),
	)
	return d, nil
}

// UpdateDiscount updates an existing discount and returns it.
func (s *Service) UpdateDiscount(ctx context.Context, id uuid.UUID, p UpdateDiscountParams) (db.Discount, error) {
	// Verify the discount exists first.
	if _, err := s.queries.GetDiscount(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Discount{}, ErrNotFound
		}
		return db.Discount{}, fmt.Errorf("checking discount existence: %w", err)
	}

	now := time.Now().UTC()
	conditions := p.Conditions
	if conditions == nil {
		conditions = json.RawMessage(`{}`)
	}

	d, err := s.queries.UpdateDiscount(ctx, db.UpdateDiscountParams{
		ID:              id,
		Name:            p.Name,
		Type:            p.Type,
		Value:           p.Value,
		Scope:           p.Scope,
		MinimumAmount:   p.MinimumAmount,
		MaximumDiscount: p.MaximumDiscount,
		StartsAt:        p.StartsAt,
		EndsAt:          p.EndsAt,
		IsActive:        p.IsActive,
		Priority:        p.Priority,
		Stackable:       p.Stackable,
		Conditions:      conditions,
		UpdatedAt:       now,
	})
	if err != nil {
		return db.Discount{}, fmt.Errorf("updating discount: %w", err)
	}

	s.logger.Info("discount updated",
		slog.String("id", d.ID.String()),
		slog.String("name", d.Name),
	)
	return d, nil
}

// --------------------------------------------------------------------------
// Coupon CRUD
// --------------------------------------------------------------------------

// GetCoupon retrieves a coupon by ID.
func (s *Service) GetCoupon(ctx context.Context, id uuid.UUID) (db.Coupon, error) {
	c, err := s.queries.GetCoupon(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Coupon{}, ErrCouponNotFound
		}
		return db.Coupon{}, fmt.Errorf("getting coupon: %w", err)
	}
	return c, nil
}

// GetCouponByCode retrieves a coupon by its unique code. The code is normalized
// to uppercase with whitespace trimmed before lookup.
func (s *Service) GetCouponByCode(ctx context.Context, code string) (db.Coupon, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	c, err := s.queries.GetCouponByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Coupon{}, ErrCouponNotFound
		}
		return db.Coupon{}, fmt.Errorf("getting coupon by code: %w", err)
	}
	return c, nil
}

// ListCoupons retrieves a paginated list of coupons with their associated discount name.
func (s *Service) ListCoupons(ctx context.Context, limit, offset int32) ([]db.ListCouponsRow, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	coupons, err := s.queries.ListCoupons(ctx, db.ListCouponsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("listing coupons: %w", err)
	}
	return coupons, nil
}

// CreateCoupon persists a new coupon linked to a discount and returns it.
func (s *Service) CreateCoupon(ctx context.Context, p CreateCouponParams) (db.Coupon, error) {
	// Verify the linked discount exists.
	if _, err := s.queries.GetDiscount(ctx, p.DiscountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Coupon{}, ErrNotFound
		}
		return db.Coupon{}, fmt.Errorf("verifying discount for coupon: %w", err)
	}

	now := time.Now().UTC()
	code := strings.TrimSpace(strings.ToUpper(p.Code))

	c, err := s.queries.CreateCoupon(ctx, db.CreateCouponParams{
		ID:                    uuid.New(),
		Code:                  code,
		DiscountID:            p.DiscountID,
		UsageLimit:            p.UsageLimit,
		UsageLimitPerCustomer: p.UsageLimitPerCustomer,
		StartsAt:              p.StartsAt,
		EndsAt:                p.EndsAt,
		IsActive:              p.IsActive,
		CreatedAt:             now,
	})
	if err != nil {
		return db.Coupon{}, fmt.Errorf("creating coupon: %w", err)
	}

	s.logger.Info("coupon created",
		slog.String("id", c.ID.String()),
		slog.String("code", c.Code),
		slog.String("discount_id", c.DiscountID.String()),
	)
	return c, nil
}

// IncrementCouponUsage increments the usage counter for a coupon after a successful order.
func (s *Service) IncrementCouponUsage(ctx context.Context, couponID uuid.UUID) error {
	err := s.queries.IncrementCouponUsage(ctx, db.IncrementCouponUsageParams{
		ID:        couponID,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("incrementing coupon usage: %w", err)
	}
	return nil
}

// DeleteCoupon permanently removes a coupon by ID.
func (s *Service) DeleteCoupon(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteCoupon(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting coupon: %w", err)
	}
	return nil
}

// DeleteDiscount permanently removes a discount by ID.
// Coupons linked to the discount are cascade-deleted by the DB constraint.
func (s *Service) DeleteDiscount(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteDiscount(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting discount: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Discount calculation engine
// --------------------------------------------------------------------------

// Apply evaluates all active discounts (and an optional coupon) against the given
// order parameters and returns the total discount with a per-discount breakdown.
//
// Evaluation order follows the CLAUDE.md spec:
//  1. Collect active discounts (date-valid, is_active=true).
//  2. If a coupon code is provided, validate and include its linked discount.
//  3. Evaluate by scope: subtotal -> shipping -> total.
//  4. Within each scope, apply stackable discounts in priority order (DESC);
//     stop at the first non-stackable discount.
//  5. Check minimum_amount and cap with maximum_discount.
func (s *Service) Apply(ctx context.Context, params ApplyParams) (ApplyResult, error) {
	result := ApplyResult{
		TotalDiscount: numericZero(),
		Breakdown:     []DiscountApplication{},
	}

	// Gather all active automatic discounts (no coupon needed).
	activeDiscounts, err := s.queries.ListActiveDiscounts(ctx)
	if err != nil {
		return result, fmt.Errorf("listing active discounts: %w", err)
	}

	// Track which discount IDs are already in the candidate set.
	seen := make(map[uuid.UUID]bool, len(activeDiscounts))
	for _, d := range activeDiscounts {
		seen[d.ID] = true
	}

	// If a coupon code was provided, validate and add its linked discount.
	if params.CouponCode != "" {
		coupon, couponDiscount, err := s.validateCoupon(ctx, params.CouponCode)
		if err != nil {
			return result, err
		}
		result.CouponID = pgtype.UUID{Bytes: coupon.ID, Valid: true}

		// Add the coupon's discount if it is not already in the active set.
		if !seen[couponDiscount.ID] {
			activeDiscounts = append(activeDiscounts, couponDiscount)
			seen[couponDiscount.ID] = true
		}
	}

	// Nothing to apply.
	if len(activeDiscounts) == 0 {
		return result, nil
	}

	// Sort by priority descending (higher priority first).
	sort.Slice(activeDiscounts, func(i, j int) bool {
		return activeDiscounts[i].Priority > activeDiscounts[j].Priority
	})

	// Normalize monetary inputs to cents (big.Int with implicit exponent -2).
	subtotalCents := numericToCents(params.Subtotal)
	shippingCents := numericToCents(params.ShippingFee)
	totalCents := new(big.Int).Add(
		new(big.Int).Set(subtotalCents),
		new(big.Int).Set(shippingCents),
	)

	// Track cumulative discount amounts per scope so we never discount more than
	// the scope's value.
	scopeConsumed := map[string]*big.Int{
		"subtotal": big.NewInt(0),
		"shipping": big.NewInt(0),
		"total":    big.NewInt(0),
	}
	scopeBase := map[string]*big.Int{
		"subtotal": subtotalCents,
		"shipping": shippingCents,
		"total":    totalCents,
	}

	// Track whether a non-stackable discount has been applied per scope.
	scopeStopped := map[string]bool{
		"subtotal": false,
		"shipping": false,
		"total":    false,
	}

	// Evaluate discounts in scope order: subtotal -> shipping -> total,
	// and within each scope in priority order (already sorted).
	for _, scope := range []string{"subtotal", "shipping", "total"} {
		for _, d := range activeDiscounts {
			if d.Scope != scope {
				continue
			}
			if scopeStopped[scope] {
				break
			}

			// Check minimum_amount against the subtotal.
			if d.MinimumAmount.Valid && d.MinimumAmount.Int != nil {
				minCents := numericToCents(d.MinimumAmount)
				if minCents.Cmp(bigZero) > 0 && subtotalCents.Cmp(minCents) < 0 {
					continue // order too small for this discount
				}
			}

			// Compute the raw discount amount in cents.
			base := scopeBase[scope]
			rawAmountCents := computeAmountCents(d, base)

			// Cap by maximum_discount if set.
			if d.MaximumDiscount.Valid && d.MaximumDiscount.Int != nil {
				maxCents := numericToCents(d.MaximumDiscount)
				if maxCents.Cmp(bigZero) > 0 && rawAmountCents.Cmp(maxCents) > 0 {
					rawAmountCents = maxCents
				}
			}

			// Ensure we do not discount more than the remaining scope value.
			remaining := new(big.Int).Sub(base, scopeConsumed[scope])
			if remaining.Cmp(bigZero) <= 0 {
				continue // scope fully consumed
			}
			if rawAmountCents.Cmp(remaining) > 0 {
				rawAmountCents.Set(remaining)
			}

			// Skip zero-value discounts.
			if rawAmountCents.Cmp(bigZero) <= 0 {
				continue
			}

			scopeConsumed[scope].Add(scopeConsumed[scope], rawAmountCents)

			application := DiscountApplication{
				DiscountID:   d.ID,
				DiscountName: d.Name,
				Type:         d.Type,
				Value:        d.Value,
				Scope:        d.Scope,
				Amount:       centsToNumeric(rawAmountCents),
			}
			result.Breakdown = append(result.Breakdown, application)

			// If this is non-stackable, stop processing further discounts in this scope.
			if !d.Stackable {
				scopeStopped[scope] = true
			}
		}
	}

	// Compute total discount from all applications.
	totalDiscountCents := big.NewInt(0)
	for _, app := range result.Breakdown {
		totalDiscountCents.Add(totalDiscountCents, numericToCents(app.Amount))
	}
	result.TotalDiscount = centsToNumeric(totalDiscountCents)

	// Set the primary discount ID from the first entry in the breakdown.
	if len(result.Breakdown) > 0 {
		result.DiscountID = pgtype.UUID{
			Bytes: result.Breakdown[0].DiscountID,
			Valid: true,
		}
	}

	s.logger.Debug("discount engine applied",
		slog.Int("discounts_applied", len(result.Breakdown)),
		slog.String("total_discount", formatNumeric(result.TotalDiscount)),
	)

	return result, nil
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// validateCoupon checks that a coupon is valid for use: it must be active,
// within its date range, and not have exceeded its usage limit.
func (s *Service) validateCoupon(ctx context.Context, code string) (db.Coupon, db.Discount, error) {
	code = strings.TrimSpace(strings.ToUpper(code))

	coupon, err := s.queries.GetCouponByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Coupon{}, db.Discount{}, ErrCouponNotFound
		}
		return db.Coupon{}, db.Discount{}, fmt.Errorf("looking up coupon: %w", err)
	}

	// Active check.
	if !coupon.IsActive {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}

	// Date range check.
	now := time.Now().UTC()
	if coupon.StartsAt.Valid && now.Before(coupon.StartsAt.Time) {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}
	if coupon.EndsAt.Valid && now.After(coupon.EndsAt.Time) {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}

	// Usage limit check.
	if coupon.UsageLimit != nil && coupon.UsageCount >= *coupon.UsageLimit {
		return db.Coupon{}, db.Discount{}, ErrCouponUsageLimitReached
	}

	// Fetch the linked discount.
	discount, err := s.queries.GetDiscount(ctx, coupon.DiscountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Coupon{}, db.Discount{}, ErrNotFound
		}
		return db.Coupon{}, db.Discount{}, fmt.Errorf("fetching coupon discount: %w", err)
	}

	// The discount itself must also be active and within date range.
	if !discount.IsActive {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}
	if discount.StartsAt.Valid && now.Before(discount.StartsAt.Time) {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}
	if discount.EndsAt.Valid && now.After(discount.EndsAt.Time) {
		return db.Coupon{}, db.Discount{}, ErrCouponExpired
	}

	return coupon, discount, nil
}

// computeAmountCents calculates the raw discount amount in cents for a single discount
// against a base value in cents.
//
// For "percentage": amount_cents = base_cents * percentage_value / 100
// For "fixed_amount": amount_cents = value_cents (capped at base_cents)
func computeAmountCents(d db.Discount, baseCents *big.Int) *big.Int {
	if baseCents.Cmp(bigZero) <= 0 {
		return big.NewInt(0)
	}

	switch d.Type {
	case "percentage":
		// d.Value is stored as e.g. {Int: 1000, Exp: -2} for 10.00%.
		// We need the percentage as a rational number: value / 100.
		// To preserve precision: amount = baseCents * valueInt / (10^(-valueExp) * 100)
		//
		// Example: 10% off 242.00 (24200 cents).
		//   d.Value = {Int: 1000, Exp: -2}  (i.e. 10.00)
		//   divisor = 10^2 * 100 = 10000
		//   amount = 24200 * 1000 / 10000 = 2420 cents = 24.20

		valueInt := d.Value.Int
		if valueInt == nil {
			return big.NewInt(0)
		}

		// Build the divisor: 10^(-Exp) * 100
		absExp := int64(-d.Value.Exp)
		if absExp < 0 {
			absExp = 0
		}
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(absExp), nil)
		divisor.Mul(divisor, big.NewInt(100))

		amount := new(big.Int).Mul(baseCents, valueInt)
		amount.Div(amount, divisor)

		// Clamp to base.
		if amount.Cmp(baseCents) > 0 {
			amount.Set(baseCents)
		}
		return amount

	case "fixed_amount":
		valueCents := numericToCents(d.Value)
		amount := new(big.Int).Set(valueCents)
		// Fixed amount cannot exceed the base value.
		if amount.Cmp(baseCents) > 0 {
			amount.Set(baseCents)
		}
		return amount

	default:
		return big.NewInt(0)
	}
}

// numericZero returns a pgtype.Numeric representing 0.00.
func numericZero() pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(0),
		Exp:   -2,
		Valid: true,
	}
}

// numericToCents converts a pgtype.Numeric to a big.Int in cents (exponent -2).
// For example, Numeric{Int: 2100, Exp: -2} returns 2100.
// Numeric{Int: 21, Exp: 0} returns 2100. If the numeric is not valid, returns 0.
func numericToCents(n pgtype.Numeric) *big.Int {
	if !n.Valid || n.Int == nil {
		return big.NewInt(0)
	}

	result := new(big.Int).Set(n.Int)

	// Normalize to exponent -2 (cents).
	// If current exponent is e, we need to multiply by 10^(e - (-2)) = 10^(e+2).
	shift := int64(n.Exp) + 2
	if shift > 0 {
		// Need to multiply (e.g., exponent=0, shift=2 -> multiply by 100).
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(shift), nil)
		result.Mul(result, factor)
	} else if shift < 0 {
		// Need to divide (e.g., exponent=-4, shift=-2 -> divide by 100).
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(-shift), nil)
		result.Div(result, factor)
	}

	return result
}

// centsToNumeric converts a big.Int representing cents back to a pgtype.Numeric
// with exponent -2.
func centsToNumeric(v *big.Int) pgtype.Numeric {
	if v == nil {
		return numericZero()
	}
	return pgtype.Numeric{
		Int:   new(big.Int).Set(v),
		Exp:   -2,
		Valid: true,
	}
}

// formatNumeric returns a human-readable string for a pgtype.Numeric for logging purposes.
func formatNumeric(n pgtype.Numeric) string {
	if !n.Valid || n.Int == nil {
		return "0.00"
	}

	s := n.Int.String()
	if n.Exp >= 0 {
		// No decimal places needed, but append .00 for consistency.
		if n.Exp > 0 {
			for i := int32(0); i < n.Exp; i++ {
				s += "0"
			}
		}
		return s + ".00"
	}

	absExp := int(-n.Exp)

	// Handle sign.
	negative := false
	digits := s
	if len(digits) > 0 && digits[0] == '-' {
		negative = true
		digits = digits[1:]
	}

	// Pad with leading zeros if the number has fewer digits than decimal places.
	for len(digits) <= absExp {
		digits = "0" + digits
	}

	intPart := digits[:len(digits)-absExp]
	fracPart := digits[len(digits)-absExp:]

	result := intPart + "." + fracPart
	if negative {
		result = "-" + result
	}
	return result
}

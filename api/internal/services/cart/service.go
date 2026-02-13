package cart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a cart does not exist.
	ErrNotFound = errors.New("cart not found")
	// ErrItemNotFound is returned when a cart item does not exist.
	ErrItemNotFound = errors.New("cart item not found")
	// ErrInvalidQuantity is returned when quantity is less than 1.
	ErrInvalidQuantity = errors.New("quantity must be at least 1")
	// ErrVariantUnavailable is returned when the variant is inactive or out of stock.
	ErrVariantUnavailable = errors.New("variant is unavailable")
)

// Service provides business logic for cart operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new cart service.
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

// defaultExpiry returns the default cart expiry time (7 days from now).
func defaultExpiry() time.Time {
	return time.Now().UTC().Add(7 * 24 * time.Hour)
}

// Create creates a new empty cart.
func (s *Service) Create(ctx context.Context) (db.Cart, error) {
	now := time.Now().UTC()
	cart, err := s.queries.CreateCart(ctx, db.CreateCartParams{
		ID:        uuid.New(),
		ExpiresAt: defaultExpiry(),
		CreatedAt: now,
	})
	if err != nil {
		return db.Cart{}, fmt.Errorf("creating cart: %w", err)
	}
	return cart, nil
}

// Get retrieves a cart by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Cart, error) {
	cart, err := s.queries.GetCart(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Cart{}, ErrNotFound
		}
		return db.Cart{}, fmt.Errorf("getting cart: %w", err)
	}
	return cart, nil
}

// UpdateParams contains the fields that can be updated on a cart.
type UpdateParams struct {
	Email       *string
	CountryCode *string
	VatNumber   *string
	CouponCode  *string
}

// Update updates cart metadata (email, country, VAT number, coupon).
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateParams) (db.Cart, error) {
	cart, err := s.queries.UpdateCart(ctx, db.UpdateCartParams{
		ID:          id,
		Email:       params.Email,
		CountryCode: params.CountryCode,
		VatNumber:   params.VatNumber,
		CouponCode:  params.CouponCode,
		UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Cart{}, ErrNotFound
		}
		return db.Cart{}, fmt.Errorf("updating cart: %w", err)
	}
	return cart, nil
}

// AddItem adds a variant to the cart. If the variant is already in the cart,
// the quantity is incremented (ON CONFLICT upsert in SQL).
func (s *Service) AddItem(ctx context.Context, cartID, variantID uuid.UUID, quantity int32) (db.CartItem, error) {
	if quantity < 1 {
		return db.CartItem{}, ErrInvalidQuantity
	}

	now := time.Now().UTC()
	item, err := s.queries.AddCartItem(ctx, db.AddCartItemParams{
		ID:        uuid.New(),
		CartID:    cartID,
		VariantID: variantID,
		Quantity:  quantity,
		CreatedAt: now,
	})
	if err != nil {
		return db.CartItem{}, fmt.Errorf("adding cart item: %w", err)
	}
	return item, nil
}

// ListItems retrieves all items in a cart with product and variant details.
func (s *Service) ListItems(ctx context.Context, cartID uuid.UUID) ([]db.GetCartItemsRow, error) {
	items, err := s.queries.GetCartItems(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("listing cart items: %w", err)
	}
	return items, nil
}

// UpdateItemQuantity updates the quantity of a specific cart item.
func (s *Service) UpdateItemQuantity(ctx context.Context, itemID uuid.UUID, quantity int32) (db.CartItem, error) {
	if quantity < 1 {
		return db.CartItem{}, ErrInvalidQuantity
	}

	item, err := s.queries.UpdateCartItemQuantity(ctx, db.UpdateCartItemQuantityParams{
		ID:        itemID,
		Quantity:  quantity,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CartItem{}, ErrItemNotFound
		}
		return db.CartItem{}, fmt.Errorf("updating cart item quantity: %w", err)
	}
	return item, nil
}

// RemoveItem removes a single item from the cart.
func (s *Service) RemoveItem(ctx context.Context, itemID uuid.UUID) error {
	if err := s.queries.RemoveCartItem(ctx, itemID); err != nil {
		return fmt.Errorf("removing cart item: %w", err)
	}
	return nil
}

// Clear removes all items from a cart.
func (s *Service) Clear(ctx context.Context, cartID uuid.UUID) error {
	if err := s.queries.ClearCart(ctx, cartID); err != nil {
		return fmt.Errorf("clearing cart: %w", err)
	}
	return nil
}

// DeleteExpired removes all carts past their expiry. Intended to be called
// by a background cleanup job.
func (s *Service) DeleteExpired(ctx context.Context) error {
	if err := s.queries.DeleteExpiredCarts(ctx); err != nil {
		return fmt.Errorf("deleting expired carts: %w", err)
	}
	return nil
}

// SetCustomer associates a customer with a cart (e.g., after login).
func (s *Service) SetCustomer(ctx context.Context, cartID uuid.UUID, customerID uuid.UUID) (db.Cart, error) {
	// Get the cart first so we can preserve existing fields.
	cart, err := s.queries.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Cart{}, ErrNotFound
		}
		return db.Cart{}, fmt.Errorf("getting cart for customer association: %w", err)
	}

	// We need a raw query since UpdateCart doesn't include customer_id.
	_, err = s.pool.Exec(ctx,
		`UPDATE carts SET customer_id = $1, updated_at = $2 WHERE id = $3`,
		pgtype.UUID{Bytes: customerID, Valid: true},
		time.Now().UTC(),
		cartID,
	)
	if err != nil {
		return db.Cart{}, fmt.Errorf("setting customer on cart: %w", err)
	}

	// Re-fetch to return updated cart.
	cart.CustomerID = pgtype.UUID{Bytes: customerID, Valid: true}
	return cart, nil
}

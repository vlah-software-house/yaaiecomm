package order

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

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when an order does not exist.
	ErrNotFound = errors.New("order not found")
)

// Service provides business logic for order operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new order service.
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

// CreateOrderItemInput contains the input fields for a single order item
// to be created as part of a new order.
type CreateOrderItemInput struct {
	ProductID        pgtype.UUID
	VariantID        pgtype.UUID
	ProductName      string
	VariantName      *string
	VariantOptions   []byte // JSON snapshot
	Sku              *string
	Quantity         int32
	UnitPrice        pgtype.Numeric
	TotalPrice       pgtype.Numeric
	VatRate          pgtype.Numeric
	VatRateType      *string
	VatAmount        pgtype.Numeric
	PriceIncludesVat bool
	NetUnitPrice     pgtype.Numeric
	GrossUnitPrice   pgtype.Numeric
	WeightGrams      *int32
	Metadata         json.RawMessage
}

// CreateOrderParams contains the input fields for creating an order,
// including all line items to be created atomically.
type CreateOrderParams struct {
	CustomerID              pgtype.UUID
	Status                  string
	Email                   string
	BillingAddress          json.RawMessage
	ShippingAddress         json.RawMessage
	Subtotal                pgtype.Numeric
	ShippingFee             pgtype.Numeric
	ShippingExtraFees       pgtype.Numeric
	DiscountAmount          pgtype.Numeric
	VatTotal                pgtype.Numeric
	Total                   pgtype.Numeric
	VatNumber               *string
	VatCompanyName          *string
	VatReverseCharge        bool
	VatCountryCode          *string
	StripePaymentIntentID   *string
	StripeCheckoutSessionID *string
	PaymentStatus           string
	DiscountID              pgtype.UUID
	CouponID                pgtype.UUID
	DiscountBreakdown       []byte
	ShippingMethod          *string
	Notes                   *string
	CustomerNotes           *string
	Metadata                json.RawMessage
	Items                   []CreateOrderItemInput
}

// Get returns a single order by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Order, error) {
	order, err := s.queries.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, ErrNotFound
		}
		return db.Order{}, fmt.Errorf("getting order %s: %w", id, err)
	}
	return order, nil
}

// GetByNumber returns a single order by its sequential order number.
func (s *Service) GetByNumber(ctx context.Context, orderNumber int64) (db.Order, error) {
	order, err := s.queries.GetOrderByNumber(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, ErrNotFound
		}
		return db.Order{}, fmt.Errorf("getting order by number %d: %w", orderNumber, err)
	}
	return order, nil
}

// List returns paginated orders with an optional status filter.
// It returns the order slice, total count, and any error.
func (s *Service) List(ctx context.Context, status *string, page, pageSize int) ([]db.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 250 {
		pageSize = 250
	}
	offset := (page - 1) * pageSize

	// CountOrders and ListOrders both accept a string where empty means "no filter"
	// and the SQL uses ($1::text IS NULL OR status = $1::text).
	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	total, err := s.queries.CountOrders(ctx, statusFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("counting orders: %w", err)
	}

	orders, err := s.queries.ListOrders(ctx, db.ListOrdersParams{
		Column1: statusFilter,
		Limit:   int32(pageSize),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing orders: %w", err)
	}

	return orders, total, nil
}

// Create creates a new order with its items and an initial "order_created" event,
// all within a single transaction. If any step fails, the entire operation is
// rolled back.
func (s *Service) Create(ctx context.Context, params CreateOrderParams) (db.Order, []db.OrderItem, error) {
	if params.Status == "" {
		params.Status = "pending"
	}
	if params.PaymentStatus == "" {
		params.PaymentStatus = "unpaid"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Order{}, nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	now := time.Now().UTC()
	orderID := uuid.New()

	order, err := qtx.CreateOrder(ctx, db.CreateOrderParams{
		ID:                      orderID,
		CustomerID:              params.CustomerID,
		Status:                  params.Status,
		Email:                   params.Email,
		BillingAddress:          params.BillingAddress,
		ShippingAddress:         params.ShippingAddress,
		Subtotal:                params.Subtotal,
		ShippingFee:             params.ShippingFee,
		ShippingExtraFees:       params.ShippingExtraFees,
		DiscountAmount:          params.DiscountAmount,
		VatTotal:                params.VatTotal,
		Total:                   params.Total,
		VatNumber:               params.VatNumber,
		VatCompanyName:          params.VatCompanyName,
		VatReverseCharge:        params.VatReverseCharge,
		VatCountryCode:          params.VatCountryCode,
		StripePaymentIntentID:   params.StripePaymentIntentID,
		StripeCheckoutSessionID: params.StripeCheckoutSessionID,
		PaymentStatus:           params.PaymentStatus,
		DiscountID:              params.DiscountID,
		CouponID:                params.CouponID,
		DiscountBreakdown:       params.DiscountBreakdown,
		ShippingMethod:          params.ShippingMethod,
		Notes:                   params.Notes,
		CustomerNotes:           params.CustomerNotes,
		Metadata:                params.Metadata,
		CreatedAt:               now,
	})
	if err != nil {
		return db.Order{}, nil, fmt.Errorf("creating order: %w", err)
	}

	// Create all order items.
	items := make([]db.OrderItem, 0, len(params.Items))
	for _, input := range params.Items {
		item, err := qtx.CreateOrderItem(ctx, db.CreateOrderItemParams{
			ID:               uuid.New(),
			OrderID:          order.ID,
			ProductID:        input.ProductID,
			VariantID:        input.VariantID,
			ProductName:      input.ProductName,
			VariantName:      input.VariantName,
			VariantOptions:   input.VariantOptions,
			Sku:              input.Sku,
			Quantity:         input.Quantity,
			UnitPrice:        input.UnitPrice,
			TotalPrice:       input.TotalPrice,
			VatRate:          input.VatRate,
			VatRateType:      input.VatRateType,
			VatAmount:        input.VatAmount,
			PriceIncludesVat: input.PriceIncludesVat,
			NetUnitPrice:     input.NetUnitPrice,
			GrossUnitPrice:   input.GrossUnitPrice,
			WeightGrams:      input.WeightGrams,
			Metadata:         input.Metadata,
		})
		if err != nil {
			return db.Order{}, nil, fmt.Errorf("creating order item %q: %w", input.ProductName, err)
		}
		items = append(items, item)
	}

	// Record the initial order event.
	toStatus := order.Status
	if err := qtx.CreateOrderEvent(ctx, db.CreateOrderEventParams{
		ID:        uuid.New(),
		OrderID:   order.ID,
		EventType: "order_created",
		ToStatus:  &toStatus,
		CreatedAt: now,
	}); err != nil {
		return db.Order{}, nil, fmt.Errorf("creating initial order event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, nil, fmt.Errorf("committing order creation: %w", err)
	}

	s.logger.Info("order created",
		slog.String("order_id", order.ID.String()),
		slog.Int64("order_number", order.OrderNumber),
		slog.String("email", order.Email),
		slog.Int("items", len(items)),
	)

	return order, items, nil
}

// UpdateStatus updates an order's status and records a status change event.
// It returns the updated order or ErrNotFound if the order does not exist.
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus string) (db.Order, error) {
	// Fetch the current order to capture the from_status for the event.
	existing, err := s.queries.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, ErrNotFound
		}
		return db.Order{}, fmt.Errorf("fetching order for status update: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Order{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)
	now := time.Now().UTC()

	order, err := qtx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:        id,
		Status:    newStatus,
		UpdatedAt: now,
	})
	if err != nil {
		return db.Order{}, fmt.Errorf("updating order status %s: %w", id, err)
	}

	// Record the status change event.
	fromStatus := existing.Status
	if err := qtx.CreateOrderEvent(ctx, db.CreateOrderEventParams{
		ID:         uuid.New(),
		OrderID:    id,
		EventType:  "status_changed",
		FromStatus: &fromStatus,
		ToStatus:   &newStatus,
		CreatedAt:  now,
	}); err != nil {
		return db.Order{}, fmt.Errorf("creating status change event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, fmt.Errorf("committing status update: %w", err)
	}

	s.logger.Info("order status updated",
		slog.String("order_id", id.String()),
		slog.String("from_status", fromStatus),
		slog.String("to_status", newStatus),
	)

	return order, nil
}

// UpdateTracking updates the tracking number and shipped-at timestamp for an order.
// It returns ErrNotFound if the order does not exist.
func (s *Service) UpdateTracking(ctx context.Context, id uuid.UUID, trackingNumber *string, shippedAt pgtype.Timestamptz) error {
	// Verify the order exists before attempting the update.
	_, err := s.queries.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching order for tracking update: %w", err)
	}

	now := time.Now().UTC()

	if err := s.queries.UpdateOrderTracking(ctx, db.UpdateOrderTrackingParams{
		ID:             id,
		TrackingNumber: trackingNumber,
		ShippedAt:      shippedAt,
		UpdatedAt:      now,
	}); err != nil {
		return fmt.Errorf("updating tracking for order %s: %w", id, err)
	}

	s.logger.Info("order tracking updated",
		slog.String("order_id", id.String()),
	)

	return nil
}

// ListItems returns all items for a given order.
func (s *Service) ListItems(ctx context.Context, orderID uuid.UUID) ([]db.OrderItem, error) {
	items, err := s.queries.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("listing items for order %s: %w", orderID, err)
	}
	return items, nil
}

// ListEvents returns all events for a given order, ordered by creation time descending.
func (s *Service) ListEvents(ctx context.Context, orderID uuid.UUID) ([]db.OrderEvent, error) {
	events, err := s.queries.ListOrderEvents(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("listing events for order %s: %w", orderID, err)
	}
	return events, nil
}

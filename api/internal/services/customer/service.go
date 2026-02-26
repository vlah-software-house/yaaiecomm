package customer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a customer does not exist.
	ErrNotFound = errors.New("customer not found")

	// ErrEmailTaken is returned when a customer with the given email already exists.
	ErrEmailTaken = errors.New("email address is already taken")
)

// Service provides business logic for customer operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new customer service.
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

// CreateCustomerParams contains the input fields for creating a customer.
type CreateCustomerParams struct {
	Email                  string
	FirstName              *string
	LastName               *string
	Phone                  *string
	PasswordHash           *string
	DefaultBillingAddress  json.RawMessage
	DefaultShippingAddress json.RawMessage
	AcceptsMarketing       bool
	StripeCustomerID       *string
	VatNumber              *string
	Notes                  *string
	Metadata               json.RawMessage
}

// UpdateCustomerParams contains the input fields for updating a customer.
type UpdateCustomerParams struct {
	FirstName              *string
	LastName               *string
	Phone                  *string
	DefaultBillingAddress  json.RawMessage
	DefaultShippingAddress json.RawMessage
	AcceptsMarketing       bool
	VatNumber              *string
	Notes                  *string
}

// Get returns a single customer by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Customer, error) {
	customer, err := s.queries.GetCustomer(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Customer{}, ErrNotFound
		}
		return db.Customer{}, fmt.Errorf("getting customer %s: %w", id, err)
	}
	return customer, nil
}

// GetByEmail returns a single customer by email address.
func (s *Service) GetByEmail(ctx context.Context, email string) (db.Customer, error) {
	customer, err := s.queries.GetCustomerByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Customer{}, ErrNotFound
		}
		return db.Customer{}, fmt.Errorf("getting customer by email %q: %w", email, err)
	}
	return customer, nil
}

// List returns paginated customers with a total count.
// It returns the customer slice, total count, and any error.
func (s *Service) List(ctx context.Context, page, pageSize int) ([]db.Customer, int64, error) {
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

	total, err := s.queries.CountCustomers(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting customers: %w", err)
	}

	customers, err := s.queries.ListCustomers(ctx, db.ListCustomersParams{
		Limit:  int32(pageSize),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing customers: %w", err)
	}

	return customers, total, nil
}

// Create creates a new customer. It returns ErrEmailTaken if the email address
// is already associated with an existing customer.
func (s *Service) Create(ctx context.Context, params CreateCustomerParams) (db.Customer, error) {
	// Check whether the email is already in use.
	_, err := s.queries.GetCustomerByEmail(ctx, params.Email)
	if err == nil {
		return db.Customer{}, ErrEmailTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Customer{}, fmt.Errorf("checking email availability: %w", err)
	}

	now := time.Now().UTC()

	// Default nil JSONB fields to empty objects to satisfy NOT NULL constraints.
	metadata := params.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}
	billingAddr := params.DefaultBillingAddress
	if billingAddr == nil {
		billingAddr = json.RawMessage(`{}`)
	}
	shippingAddr := params.DefaultShippingAddress
	if shippingAddr == nil {
		shippingAddr = json.RawMessage(`{}`)
	}

	customer, err := s.queries.CreateCustomer(ctx, db.CreateCustomerParams{
		ID:                     uuid.New(),
		Email:                  params.Email,
		FirstName:              params.FirstName,
		LastName:               params.LastName,
		Phone:                  params.Phone,
		PasswordHash:           params.PasswordHash,
		DefaultBillingAddress:  billingAddr,
		DefaultShippingAddress: shippingAddr,
		AcceptsMarketing:       params.AcceptsMarketing,
		StripeCustomerID:       params.StripeCustomerID,
		VatNumber:              params.VatNumber,
		Notes:                  params.Notes,
		Metadata:               metadata,
		CreatedAt:              now,
	})
	if err != nil {
		return db.Customer{}, fmt.Errorf("creating customer: %w", err)
	}

	s.logger.Info("customer created",
		slog.String("customer_id", customer.ID.String()),
		slog.String("email", customer.Email),
	)

	return customer, nil
}

// Update updates an existing customer. The caller must provide the full set of
// updatable fields; any nil optional fields will overwrite the existing values.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateCustomerParams) (db.Customer, error) {
	// Verify the customer exists before attempting the update.
	_, err := s.queries.GetCustomer(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Customer{}, ErrNotFound
		}
		return db.Customer{}, fmt.Errorf("fetching customer for update: %w", err)
	}

	now := time.Now().UTC()

	customer, err := s.queries.UpdateCustomer(ctx, db.UpdateCustomerParams{
		ID:                     id,
		FirstName:              params.FirstName,
		LastName:               params.LastName,
		Phone:                  params.Phone,
		DefaultBillingAddress:  params.DefaultBillingAddress,
		DefaultShippingAddress: params.DefaultShippingAddress,
		AcceptsMarketing:       params.AcceptsMarketing,
		VatNumber:              params.VatNumber,
		Notes:                  params.Notes,
		UpdatedAt:              now,
	})
	if err != nil {
		return db.Customer{}, fmt.Errorf("updating customer %s: %w", id, err)
	}

	s.logger.Info("customer updated",
		slog.String("customer_id", customer.ID.String()),
		slog.String("email", customer.Email),
	)

	return customer, nil
}

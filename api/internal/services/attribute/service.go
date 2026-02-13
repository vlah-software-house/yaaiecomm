package attribute

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when an attribute or option does not exist.
	ErrNotFound = errors.New("attribute not found")

	// ErrOptionNotFound is returned when an attribute option does not exist.
	ErrOptionNotFound = errors.New("attribute option not found")

	// ErrNameRequired is returned when an attribute name is empty.
	ErrNameRequired = errors.New("attribute name is required")

	// ErrValueRequired is returned when an option value is empty.
	ErrValueRequired = errors.New("option value is required")
)

// Service provides business logic for product attribute and option CRUD operations.
type Service struct {
	queries *db.Queries
	logger  *slog.Logger
}

// NewService creates a new attribute service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		logger:  logger,
	}
}

// CreateAttributeParams contains the input fields for creating a product attribute.
type CreateAttributeParams struct {
	ProductID       uuid.UUID
	Name            string
	DisplayName     string
	AttributeType   string // select, color_swatch, button_group, image_swatch
	Position        int32
	AffectsPricing  bool
	AffectsShipping bool
}

// UpdateAttributeParams contains the input fields for updating a product attribute.
type UpdateAttributeParams struct {
	Name            string
	DisplayName     string
	AttributeType   string
	Position        int32
	AffectsPricing  bool
	AffectsShipping bool
}

// CreateOptionParams contains the input fields for creating an attribute option.
type CreateOptionParams struct {
	AttributeID         uuid.UUID
	Value               string
	DisplayValue        string
	ColorHex            *string
	ImageURL            *string
	PriceModifier       pgtype.Numeric
	WeightModifierGrams *int32
	Position            int32
	IsActive            bool
}

// UpdateOptionParams contains the input fields for updating an attribute option.
type UpdateOptionParams struct {
	Value               string
	DisplayValue        string
	ColorHex            *string
	ImageURL            *string
	PriceModifier       pgtype.Numeric
	WeightModifierGrams *int32
	Position            int32
	IsActive            bool
}

// ListAttributes returns all attributes for a product, ordered by position.
func (s *Service) ListAttributes(ctx context.Context, productID uuid.UUID) ([]db.ProductAttribute, error) {
	attrs, err := s.queries.ListProductAttributes(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing attributes for product %s: %w", productID, err)
	}
	return attrs, nil
}

// GetAttribute returns a single product attribute by ID.
func (s *Service) GetAttribute(ctx context.Context, id uuid.UUID) (db.ProductAttribute, error) {
	attr, err := s.queries.GetProductAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductAttribute{}, ErrNotFound
		}
		return db.ProductAttribute{}, fmt.Errorf("getting attribute %s: %w", id, err)
	}
	return attr, nil
}

// CreateAttribute creates a new product attribute with an auto-generated UUID.
func (s *Service) CreateAttribute(ctx context.Context, params CreateAttributeParams) (db.ProductAttribute, error) {
	if params.Name == "" {
		return db.ProductAttribute{}, ErrNameRequired
	}

	if params.AttributeType == "" {
		params.AttributeType = "select"
	}

	attr, err := s.queries.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:              uuid.New(),
		ProductID:       params.ProductID,
		Name:            params.Name,
		DisplayName:     params.DisplayName,
		AttributeType:   params.AttributeType,
		Position:        params.Position,
		AffectsPricing:  params.AffectsPricing,
		AffectsShipping: params.AffectsShipping,
	})
	if err != nil {
		return db.ProductAttribute{}, fmt.Errorf("creating attribute: %w", err)
	}

	s.logger.Info("attribute created",
		slog.String("attribute_id", attr.ID.String()),
		slog.String("product_id", attr.ProductID.String()),
		slog.String("name", attr.Name),
	)

	return attr, nil
}

// UpdateAttribute updates an existing product attribute.
func (s *Service) UpdateAttribute(ctx context.Context, id uuid.UUID, params UpdateAttributeParams) (db.ProductAttribute, error) {
	if params.Name == "" {
		return db.ProductAttribute{}, ErrNameRequired
	}

	// Verify the attribute exists before updating.
	_, err := s.queries.GetProductAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductAttribute{}, ErrNotFound
		}
		return db.ProductAttribute{}, fmt.Errorf("fetching attribute for update: %w", err)
	}

	attr, err := s.queries.UpdateProductAttribute(ctx, db.UpdateProductAttributeParams{
		ID:              id,
		Name:            params.Name,
		DisplayName:     params.DisplayName,
		AttributeType:   params.AttributeType,
		Position:        params.Position,
		AffectsPricing:  params.AffectsPricing,
		AffectsShipping: params.AffectsShipping,
	})
	if err != nil {
		return db.ProductAttribute{}, fmt.Errorf("updating attribute %s: %w", id, err)
	}

	s.logger.Info("attribute updated",
		slog.String("attribute_id", attr.ID.String()),
		slog.String("name", attr.Name),
	)

	return attr, nil
}

// DeleteAttribute deletes a product attribute by ID. This cascades to its options.
func (s *Service) DeleteAttribute(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrNotFound.
	_, err := s.queries.GetProductAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching attribute for delete: %w", err)
	}

	if err := s.queries.DeleteProductAttribute(ctx, id); err != nil {
		return fmt.Errorf("deleting attribute %s: %w", id, err)
	}

	s.logger.Info("attribute deleted", slog.String("attribute_id", id.String()))
	return nil
}

// ListOptions returns all options for an attribute, ordered by position.
func (s *Service) ListOptions(ctx context.Context, attributeID uuid.UUID) ([]db.ProductAttributeOption, error) {
	options, err := s.queries.ListAttributeOptions(ctx, attributeID)
	if err != nil {
		return nil, fmt.Errorf("listing options for attribute %s: %w", attributeID, err)
	}
	return options, nil
}

// GetOption returns a single attribute option by ID.
func (s *Service) GetOption(ctx context.Context, id uuid.UUID) (db.ProductAttributeOption, error) {
	opt, err := s.queries.GetAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductAttributeOption{}, ErrOptionNotFound
		}
		return db.ProductAttributeOption{}, fmt.Errorf("getting option %s: %w", id, err)
	}
	return opt, nil
}

// CreateOption creates a new attribute option with an auto-generated UUID.
func (s *Service) CreateOption(ctx context.Context, params CreateOptionParams) (db.ProductAttributeOption, error) {
	if params.Value == "" {
		return db.ProductAttributeOption{}, ErrValueRequired
	}

	opt, err := s.queries.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:                  uuid.New(),
		AttributeID:         params.AttributeID,
		Value:               params.Value,
		DisplayValue:        params.DisplayValue,
		ColorHex:            params.ColorHex,
		ImageUrl:            params.ImageURL,
		PriceModifier:       params.PriceModifier,
		WeightModifierGrams: params.WeightModifierGrams,
		Position:            params.Position,
		IsActive:            params.IsActive,
	})
	if err != nil {
		return db.ProductAttributeOption{}, fmt.Errorf("creating option: %w", err)
	}

	s.logger.Info("attribute option created",
		slog.String("option_id", opt.ID.String()),
		slog.String("attribute_id", opt.AttributeID.String()),
		slog.String("value", opt.Value),
	)

	return opt, nil
}

// UpdateOption updates an existing attribute option.
func (s *Service) UpdateOption(ctx context.Context, id uuid.UUID, params UpdateOptionParams) (db.ProductAttributeOption, error) {
	if params.Value == "" {
		return db.ProductAttributeOption{}, ErrValueRequired
	}

	// Verify the option exists before updating.
	_, err := s.queries.GetAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductAttributeOption{}, ErrOptionNotFound
		}
		return db.ProductAttributeOption{}, fmt.Errorf("fetching option for update: %w", err)
	}

	opt, err := s.queries.UpdateAttributeOption(ctx, db.UpdateAttributeOptionParams{
		ID:                  id,
		Value:               params.Value,
		DisplayValue:        params.DisplayValue,
		ColorHex:            params.ColorHex,
		ImageUrl:            params.ImageURL,
		PriceModifier:       params.PriceModifier,
		WeightModifierGrams: params.WeightModifierGrams,
		Position:            params.Position,
		IsActive:            params.IsActive,
	})
	if err != nil {
		return db.ProductAttributeOption{}, fmt.Errorf("updating option %s: %w", id, err)
	}

	s.logger.Info("attribute option updated",
		slog.String("option_id", opt.ID.String()),
		slog.String("value", opt.Value),
	)

	return opt, nil
}

// DeleteOption deletes an attribute option by ID.
func (s *Service) DeleteOption(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrOptionNotFound.
	_, err := s.queries.GetAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOptionNotFound
		}
		return fmt.Errorf("fetching option for delete: %w", err)
	}

	if err := s.queries.DeleteAttributeOption(ctx, id); err != nil {
		return fmt.Errorf("deleting option %s: %w", id, err)
	}

	s.logger.Info("attribute option deleted", slog.String("option_id", id.String()))
	return nil
}

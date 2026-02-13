package bom

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// Service provides business logic for product BOM (Bill of Materials) operations.
//
// The BOM is structured in three layers:
//   - Layer 1: Product-level entries — materials common to ALL variants of a product.
//   - Layer 2a: Attribute option additional materials — extra materials for a specific option.
//   - Layer 2b: Attribute option modifiers — multiply/add/set quantity on Layer 1 entries.
//   - Layer 3: Variant BOM overrides — replace/add/remove/set_quantity per variant.
type Service struct {
	queries *db.Queries
	logger  *slog.Logger
}

// NewService creates a new BOM service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		logger:  logger,
	}
}

// ---------------------------------------------------------------------------
// Layer 1: Product BOM Entries
// ---------------------------------------------------------------------------

// CreateProductEntryParams holds the input for creating a product BOM entry.
type CreateProductEntryParams struct {
	ProductID     uuid.UUID
	RawMaterialID uuid.UUID
	Quantity      pgtype.Numeric
	UnitOfMeasure string
	IsRequired    bool
	Notes         *string
}

// UpdateProductEntryParams holds the input for updating a product BOM entry.
type UpdateProductEntryParams struct {
	Quantity      pgtype.Numeric
	UnitOfMeasure string
	IsRequired    bool
	Notes         *string
}

// ListProductEntries returns all BOM entries for a product with joined material info.
func (s *Service) ListProductEntries(ctx context.Context, productID uuid.UUID) ([]db.ListProductBOMEntriesRow, error) {
	entries, err := s.queries.ListProductBOMEntries(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing product BOM entries for product %s: %w", productID, err)
	}
	return entries, nil
}

// CreateProductEntry creates a new product-level BOM entry (Layer 1).
func (s *Service) CreateProductEntry(ctx context.Context, params CreateProductEntryParams) (db.ProductBomEntry, error) {
	entry, err := s.queries.CreateProductBOMEntry(ctx, db.CreateProductBOMEntryParams{
		ID:            uuid.New(),
		ProductID:     params.ProductID,
		RawMaterialID: params.RawMaterialID,
		Quantity:      params.Quantity,
		UnitOfMeasure: params.UnitOfMeasure,
		IsRequired:    params.IsRequired,
		Notes:         params.Notes,
	})
	if err != nil {
		return db.ProductBomEntry{}, fmt.Errorf("creating product BOM entry: %w", err)
	}

	s.logger.Info("product BOM entry created",
		slog.String("entry_id", entry.ID.String()),
		slog.String("product_id", entry.ProductID.String()),
		slog.String("material_id", entry.RawMaterialID.String()),
	)
	return entry, nil
}

// UpdateProductEntry updates a product BOM entry.
func (s *Service) UpdateProductEntry(ctx context.Context, id uuid.UUID, params UpdateProductEntryParams) (db.ProductBomEntry, error) {
	entry, err := s.queries.UpdateProductBOMEntry(ctx, db.UpdateProductBOMEntryParams{
		ID:            id,
		Quantity:      params.Quantity,
		UnitOfMeasure: params.UnitOfMeasure,
		IsRequired:    params.IsRequired,
		Notes:         params.Notes,
	})
	if err != nil {
		return db.ProductBomEntry{}, fmt.Errorf("updating product BOM entry %s: %w", id, err)
	}

	s.logger.Info("product BOM entry updated", slog.String("entry_id", id.String()))
	return entry, nil
}

// DeleteProductEntry deletes a product BOM entry by ID.
func (s *Service) DeleteProductEntry(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteProductBOMEntry(ctx, id); err != nil {
		return fmt.Errorf("deleting product BOM entry %s: %w", id, err)
	}
	s.logger.Info("product BOM entry deleted", slog.String("entry_id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Layer 2a: Attribute Option Additional Materials
// ---------------------------------------------------------------------------

// CreateOptionEntryParams holds the input for creating an option BOM entry.
type CreateOptionEntryParams struct {
	OptionID      uuid.UUID
	RawMaterialID uuid.UUID
	Quantity      pgtype.Numeric
	UnitOfMeasure string
	Notes         *string
}

// ListOptionEntries returns all additional material entries for an attribute option.
func (s *Service) ListOptionEntries(ctx context.Context, optionID uuid.UUID) ([]db.ListOptionBOMEntriesRow, error) {
	entries, err := s.queries.ListOptionBOMEntries(ctx, optionID)
	if err != nil {
		return nil, fmt.Errorf("listing option BOM entries for option %s: %w", optionID, err)
	}
	return entries, nil
}

// CreateOptionEntry creates a new option-level additional material entry (Layer 2a).
func (s *Service) CreateOptionEntry(ctx context.Context, params CreateOptionEntryParams) (db.AttributeOptionBomEntry, error) {
	entry, err := s.queries.CreateOptionBOMEntry(ctx, db.CreateOptionBOMEntryParams{
		ID:            uuid.New(),
		OptionID:      params.OptionID,
		RawMaterialID: params.RawMaterialID,
		Quantity:      params.Quantity,
		UnitOfMeasure: params.UnitOfMeasure,
		Notes:         params.Notes,
	})
	if err != nil {
		return db.AttributeOptionBomEntry{}, fmt.Errorf("creating option BOM entry: %w", err)
	}

	s.logger.Info("option BOM entry created",
		slog.String("entry_id", entry.ID.String()),
		slog.String("option_id", entry.OptionID.String()),
		slog.String("material_id", entry.RawMaterialID.String()),
	)
	return entry, nil
}

// DeleteOptionEntry deletes an option BOM entry by ID.
func (s *Service) DeleteOptionEntry(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteOptionBOMEntry(ctx, id); err != nil {
		return fmt.Errorf("deleting option BOM entry %s: %w", id, err)
	}
	s.logger.Info("option BOM entry deleted", slog.String("entry_id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Layer 2b: Attribute Option BOM Modifiers
// ---------------------------------------------------------------------------

// CreateOptionModifierParams holds the input for creating an option BOM modifier.
type CreateOptionModifierParams struct {
	OptionID          uuid.UUID
	ProductBomEntryID uuid.UUID
	ModifierType      string // "multiply", "add", "set"
	ModifierValue     pgtype.Numeric
	Notes             *string
}

// ListOptionModifiers returns all BOM modifiers for an attribute option.
func (s *Service) ListOptionModifiers(ctx context.Context, optionID uuid.UUID) ([]db.ListOptionBOMModifiersRow, error) {
	modifiers, err := s.queries.ListOptionBOMModifiers(ctx, optionID)
	if err != nil {
		return nil, fmt.Errorf("listing option BOM modifiers for option %s: %w", optionID, err)
	}
	return modifiers, nil
}

// CreateOptionModifier creates a new option BOM modifier (Layer 2b).
// Modifiers adjust the quantity of an existing product BOM entry (Layer 1)
// when a specific attribute option is selected.
func (s *Service) CreateOptionModifier(ctx context.Context, params CreateOptionModifierParams) (db.AttributeOptionBomModifier, error) {
	modifier, err := s.queries.CreateOptionBOMModifier(ctx, db.CreateOptionBOMModifierParams{
		ID:                uuid.New(),
		OptionID:          params.OptionID,
		ProductBomEntryID: params.ProductBomEntryID,
		ModifierType:      params.ModifierType,
		ModifierValue:     params.ModifierValue,
		Notes:             params.Notes,
	})
	if err != nil {
		return db.AttributeOptionBomModifier{}, fmt.Errorf("creating option BOM modifier: %w", err)
	}

	s.logger.Info("option BOM modifier created",
		slog.String("modifier_id", modifier.ID.String()),
		slog.String("option_id", modifier.OptionID.String()),
		slog.String("product_bom_entry_id", modifier.ProductBomEntryID.String()),
		slog.String("type", modifier.ModifierType),
	)
	return modifier, nil
}

// DeleteOptionModifier deletes an option BOM modifier by ID.
func (s *Service) DeleteOptionModifier(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteOptionBOMModifier(ctx, id); err != nil {
		return fmt.Errorf("deleting option BOM modifier %s: %w", id, err)
	}
	s.logger.Info("option BOM modifier deleted", slog.String("modifier_id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Layer 3: Variant BOM Overrides
// ---------------------------------------------------------------------------

// CreateVariantOverrideParams holds the input for creating a variant BOM override.
type CreateVariantOverrideParams struct {
	VariantID          uuid.UUID
	RawMaterialID      uuid.UUID
	OverrideType       string // "replace", "add", "remove", "set_quantity"
	ReplacesMaterialID pgtype.UUID
	Quantity           pgtype.Numeric
	UnitOfMeasure      *string
	Notes              *string
}

// ListVariantOverrides returns all BOM overrides for a variant.
func (s *Service) ListVariantOverrides(ctx context.Context, variantID uuid.UUID) ([]db.ListVariantBOMOverridesRow, error) {
	overrides, err := s.queries.ListVariantBOMOverrides(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("listing variant BOM overrides for variant %s: %w", variantID, err)
	}
	return overrides, nil
}

// CreateVariantOverride creates a new variant BOM override (Layer 3).
// Overrides allow per-variant adjustments: replacing one material with another,
// adding extra materials, removing materials, or setting exact quantities.
func (s *Service) CreateVariantOverride(ctx context.Context, params CreateVariantOverrideParams) (db.VariantBomOverride, error) {
	override, err := s.queries.CreateVariantBOMOverride(ctx, db.CreateVariantBOMOverrideParams{
		ID:                 uuid.New(),
		VariantID:          params.VariantID,
		RawMaterialID:      params.RawMaterialID,
		OverrideType:       params.OverrideType,
		ReplacesMaterialID: params.ReplacesMaterialID,
		Quantity:           params.Quantity,
		UnitOfMeasure:      params.UnitOfMeasure,
		Notes:              params.Notes,
	})
	if err != nil {
		return db.VariantBomOverride{}, fmt.Errorf("creating variant BOM override: %w", err)
	}

	s.logger.Info("variant BOM override created",
		slog.String("override_id", override.ID.String()),
		slog.String("variant_id", override.VariantID.String()),
		slog.String("material_id", override.RawMaterialID.String()),
		slog.String("type", override.OverrideType),
	)
	return override, nil
}

// DeleteVariantOverride deletes a variant BOM override by ID.
func (s *Service) DeleteVariantOverride(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteVariantBOMOverride(ctx, id); err != nil {
		return fmt.Errorf("deleting variant BOM override %s: %w", id, err)
	}
	s.logger.Info("variant BOM override deleted", slog.String("override_id", id.String()))
	return nil
}

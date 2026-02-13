package variant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a variant does not exist.
	ErrNotFound = errors.New("variant not found")

	// ErrSKURequired is returned when a variant SKU is empty.
	ErrSKURequired = errors.New("variant SKU is required")

	// ErrNoAttributes is returned when variant generation is attempted on a product
	// with no attributes or no active options.
	ErrNoAttributes = errors.New("product has no attributes or active options for variant generation")
)

// CreateVariantParams contains the input fields for creating a variant.
type CreateVariantParams struct {
	ProductID         uuid.UUID
	Sku               string
	Price             pgtype.Numeric
	CompareAtPrice    pgtype.Numeric
	WeightGrams       *int32
	DimensionsMm      []byte
	StockQuantity     int32
	LowStockThreshold int32
	Barcode           *string
	IsActive          bool
	Position          int32
}

// UpdateVariantParams contains the input fields for updating a variant.
type UpdateVariantParams struct {
	Sku               string
	Price             pgtype.Numeric
	CompareAtPrice    pgtype.Numeric
	WeightGrams       *int32
	DimensionsMm      []byte
	StockQuantity     int32
	LowStockThreshold int32
	Barcode           *string
	IsActive          bool
	Position          int32
}

// Service provides business logic for product variant operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new variant service.
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

// List returns all variants for a product, ordered by position.
func (s *Service) List(ctx context.Context, productID uuid.UUID) ([]db.ProductVariant, error) {
	variants, err := s.queries.ListProductVariants(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing variants for product %s: %w", productID, err)
	}
	return variants, nil
}

// Get returns a single variant by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.ProductVariant, error) {
	variant, err := s.queries.GetProductVariant(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductVariant{}, ErrNotFound
		}
		return db.ProductVariant{}, fmt.Errorf("getting variant %s: %w", id, err)
	}
	return variant, nil
}

// GetBySKU returns a variant by its SKU.
func (s *Service) GetBySKU(ctx context.Context, sku string) (db.ProductVariant, error) {
	variant, err := s.queries.GetProductVariantBySKU(ctx, sku)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductVariant{}, ErrNotFound
		}
		return db.ProductVariant{}, fmt.Errorf("getting variant by SKU %q: %w", sku, err)
	}
	return variant, nil
}

// Create creates a new product variant.
func (s *Service) Create(ctx context.Context, params CreateVariantParams) (db.ProductVariant, error) {
	if params.Sku == "" {
		return db.ProductVariant{}, ErrSKURequired
	}

	variant, err := s.queries.CreateProductVariant(ctx, db.CreateProductVariantParams{
		ID:                uuid.New(),
		ProductID:         params.ProductID,
		Sku:               params.Sku,
		Price:             params.Price,
		CompareAtPrice:    params.CompareAtPrice,
		WeightGrams:       params.WeightGrams,
		DimensionsMm:      params.DimensionsMm,
		StockQuantity:     params.StockQuantity,
		LowStockThreshold: params.LowStockThreshold,
		Barcode:           params.Barcode,
		IsActive:          params.IsActive,
		Position:          params.Position,
	})
	if err != nil {
		return db.ProductVariant{}, fmt.Errorf("creating variant: %w", err)
	}

	s.logger.Info("variant created",
		slog.String("variant_id", variant.ID.String()),
		slog.String("sku", variant.Sku),
		slog.String("product_id", variant.ProductID.String()),
	)

	return variant, nil
}

// Update updates an existing variant. The caller must provide the full set of
// mutable fields; any zero-valued optional fields will overwrite the existing values.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateVariantParams) (db.ProductVariant, error) {
	if params.Sku == "" {
		return db.ProductVariant{}, ErrSKURequired
	}

	// Verify the variant exists before attempting the update.
	_, err := s.queries.GetProductVariant(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductVariant{}, ErrNotFound
		}
		return db.ProductVariant{}, fmt.Errorf("fetching variant for update: %w", err)
	}

	variant, err := s.queries.UpdateProductVariant(ctx, db.UpdateProductVariantParams{
		ID:                id,
		Sku:               params.Sku,
		Price:             params.Price,
		CompareAtPrice:    params.CompareAtPrice,
		WeightGrams:       params.WeightGrams,
		DimensionsMm:      params.DimensionsMm,
		StockQuantity:     params.StockQuantity,
		LowStockThreshold: params.LowStockThreshold,
		Barcode:           params.Barcode,
		IsActive:          params.IsActive,
		Position:          params.Position,
	})
	if err != nil {
		return db.ProductVariant{}, fmt.Errorf("updating variant %s: %w", id, err)
	}

	s.logger.Info("variant updated",
		slog.String("variant_id", variant.ID.String()),
		slog.String("sku", variant.Sku),
	)

	return variant, nil
}

// Delete deletes a variant by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrNotFound.
	_, err := s.queries.GetProductVariant(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching variant for delete: %w", err)
	}

	if err := s.queries.DeleteProductVariant(ctx, id); err != nil {
		return fmt.Errorf("deleting variant %s: %w", id, err)
	}

	s.logger.Info("variant deleted", slog.String("variant_id", id.String()))
	return nil
}

// UpdateStock updates the stock quantity for a variant.
func (s *Service) UpdateStock(ctx context.Context, id uuid.UUID, quantity int32) error {
	// Verify existence first so callers get a clear ErrNotFound.
	_, err := s.queries.GetProductVariant(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching variant for stock update: %w", err)
	}

	if err := s.queries.UpdateVariantStock(ctx, db.UpdateVariantStockParams{
		ID:            id,
		StockQuantity: quantity,
	}); err != nil {
		return fmt.Errorf("updating stock for variant %s: %w", id, err)
	}

	s.logger.Info("variant stock updated",
		slog.String("variant_id", id.String()),
		slog.Int("stock_quantity", int(quantity)),
	)
	return nil
}

// ListOptions returns the attribute options linked to a variant.
func (s *Service) ListOptions(ctx context.Context, variantID uuid.UUID) ([]db.ListVariantOptionsRow, error) {
	options, err := s.queries.ListVariantOptions(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("listing options for variant %s: %w", variantID, err)
	}
	return options, nil
}

// SetOption links a variant to an attribute option. If the variant already has
// an option for the given attribute, it is replaced (upsert).
func (s *Service) SetOption(ctx context.Context, variantID, attributeID, optionID uuid.UUID) error {
	if err := s.queries.SetVariantOption(ctx, db.SetVariantOptionParams{
		VariantID:   variantID,
		AttributeID: attributeID,
		OptionID:    optionID,
	}); err != nil {
		return fmt.Errorf("setting option for variant %s: %w", variantID, err)
	}
	return nil
}

// GenerateVariants generates the Cartesian product of all active attribute
// options for a product. Existing variants whose option combinations already
// exist are preserved; only new combinations are created.
//
// The generated SKU for each variant follows the pattern:
//
//	{skuPrefix}-{OPT1}-{OPT2}-...
//
// where each option abbreviation is the first 3 characters of the option value,
// uppercased.
func (s *Service) GenerateVariants(ctx context.Context, productID uuid.UUID, skuPrefix string) ([]db.ProductVariant, error) {
	// Step 1: Load all attributes for the product, ordered by position.
	attributes, err := s.queries.ListProductAttributes(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing attributes for product %s: %w", productID, err)
	}
	if len(attributes) == 0 {
		return nil, ErrNoAttributes
	}

	// Step 2: For each attribute, load all active options ordered by position.
	// attrOpts preserves attribute order and holds only active options.
	attrOpts := make([]attributeWithOptions, 0, len(attributes))

	for _, attr := range attributes {
		allOptions, err := s.queries.ListAttributeOptions(ctx, attr.ID)
		if err != nil {
			return nil, fmt.Errorf("listing options for attribute %s: %w", attr.ID, err)
		}
		// Filter to only active options.
		activeOptions := make([]db.ProductAttributeOption, 0, len(allOptions))
		for _, opt := range allOptions {
			if opt.IsActive {
				activeOptions = append(activeOptions, opt)
			}
		}
		if len(activeOptions) == 0 {
			continue
		}
		attrOpts = append(attrOpts, attributeWithOptions{
			attribute: attr,
			options:   activeOptions,
		})
	}

	if len(attrOpts) == 0 {
		return nil, ErrNoAttributes
	}

	// Step 3: Build set of existing variant option combinations to avoid
	// recreating variants that already exist.
	existingVariants, err := s.queries.ListProductVariants(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing existing variants for product %s: %w", productID, err)
	}

	// Build a lookup of existing combinations. The key is a sorted, hyphen-joined
	// string of option UUIDs (deterministic regardless of attribute order).
	existingCombinations := make(map[string]bool, len(existingVariants))
	for _, v := range existingVariants {
		opts, err := s.queries.ListVariantOptions(ctx, v.ID)
		if err != nil {
			return nil, fmt.Errorf("listing options for existing variant %s: %w", v.ID, err)
		}
		key := optionSetKey(opts)
		existingCombinations[key] = true
	}

	// Step 4: Generate the Cartesian product.
	combinations := cartesianProduct(attrOpts)

	// Step 5: Create new variants inside a transaction.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Position counter starts after existing variants.
	nextPosition := int32(len(existingVariants))
	var created []db.ProductVariant

	for _, combo := range combinations {
		// Build the option set key for this combination.
		key := comboKey(combo)
		if existingCombinations[key] {
			// This combination already exists as a variant; skip.
			continue
		}

		// Build SKU from prefix and option abbreviations.
		sku := buildSKU(skuPrefix, combo)

		variant, err := qtx.CreateProductVariant(ctx, db.CreateProductVariantParams{
			ID:                uuid.New(),
			ProductID:         productID,
			Sku:               sku,
			Price:             pgtype.Numeric{}, // NULL -- calculated from base price + modifiers
			CompareAtPrice:    pgtype.Numeric{},
			WeightGrams:       nil,
			DimensionsMm:      nil,
			StockQuantity:     0,
			LowStockThreshold: 0,
			Barcode:           nil,
			IsActive:          true,
			Position:          nextPosition,
		})
		if err != nil {
			return nil, fmt.Errorf("creating variant with SKU %q: %w", sku, err)
		}

		// Link the variant to its options.
		for _, optEntry := range combo {
			if err := qtx.SetVariantOption(ctx, db.SetVariantOptionParams{
				VariantID:   variant.ID,
				AttributeID: optEntry.attributeID,
				OptionID:    optEntry.option.ID,
			}); err != nil {
				return nil, fmt.Errorf("linking variant %s to option %s: %w", variant.ID, optEntry.option.ID, err)
			}
		}

		created = append(created, variant)
		nextPosition++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing variant generation: %w", err)
	}

	s.logger.Info("variants generated",
		slog.String("product_id", productID.String()),
		slog.Int("new_variants", len(created)),
		slog.Int("existing_variants", len(existingVariants)),
	)

	return created, nil
}

// comboEntry represents a single attribute-option pair within a variant combination.
type comboEntry struct {
	attributeID uuid.UUID
	option      db.ProductAttributeOption
}

// attributeWithOptions pairs an attribute with its active options for variant generation.
type attributeWithOptions struct {
	attribute db.ProductAttribute
	options   []db.ProductAttributeOption
}

// cartesianProduct generates all combinations of active options across attributes.
// Each element in the outer slice is one full combination (one option per attribute).
func cartesianProduct(attrOpts []attributeWithOptions) [][]comboEntry {
	if len(attrOpts) == 0 {
		return nil
	}

	// Start with a single empty combination.
	result := [][]comboEntry{{}}

	for _, ao := range attrOpts {
		var expanded [][]comboEntry
		for _, existing := range result {
			for _, opt := range ao.options {
				// Copy the existing combination and append the new option.
				newCombo := make([]comboEntry, len(existing), len(existing)+1)
				copy(newCombo, existing)
				newCombo = append(newCombo, comboEntry{
					attributeID: ao.attribute.ID,
					option:      opt,
				})
				expanded = append(expanded, newCombo)
			}
		}
		result = expanded
	}

	return result
}

// optionSetKey creates a deterministic string key from a variant's option rows.
// Option IDs are sorted to ensure consistent keys regardless of query order.
func optionSetKey(opts []db.ListVariantOptionsRow) string {
	ids := make([]string, len(opts))
	for i, o := range opts {
		ids[i] = o.OptionID.String()
	}
	sort.Strings(ids)
	return strings.Join(ids, "-")
}

// comboKey creates a deterministic string key from a combination of option entries.
// Option IDs are sorted to ensure consistent keys regardless of attribute order.
func comboKey(combo []comboEntry) string {
	ids := make([]string, len(combo))
	for i, c := range combo {
		ids[i] = c.option.ID.String()
	}
	sort.Strings(ids)
	return strings.Join(ids, "-")
}

// buildSKU generates a SKU string from a prefix and option values.
// Each option is abbreviated to its first 3 characters, uppercased.
// Example: "BAG" prefix with options ["Black", "Large"] -> "BAG-BLA-LAR"
func buildSKU(prefix string, combo []comboEntry) string {
	parts := make([]string, 0, len(combo)+1)
	parts = append(parts, prefix)
	for _, c := range combo {
		abbr := c.option.Value
		if len(abbr) > 3 {
			abbr = abbr[:3]
		}
		parts = append(parts, strings.ToUpper(abbr))
	}
	return strings.Join(parts, "-")
}

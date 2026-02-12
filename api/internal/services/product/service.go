package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a product does not exist.
	ErrNotFound = errors.New("product not found")

	// ErrNameRequired is returned when a product name is empty.
	ErrNameRequired = errors.New("product name is required")
)

// Service provides business logic for product CRUD operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new product service.
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

// CreateProductParams contains the input fields for creating a product.
type CreateProductParams struct {
	Name                    string
	Slug                    string // optional; auto-generated from Name if empty
	Description             *string
	ShortDescription        *string
	Status                  string // defaults to "draft" if empty
	SkuPrefix               *string
	BasePrice               pgtype.Numeric
	CompareAtPrice          pgtype.Numeric
	VatCategoryID           pgtype.UUID
	BaseWeightGrams         int32
	BaseDimensionsMm        json.RawMessage
	ShippingExtraFeePerUnit pgtype.Numeric
	HasVariants             bool
	SeoTitle                *string
	SeoDescription          *string
	Metadata                json.RawMessage
}

// UpdateProductParams contains the input fields for updating a product.
type UpdateProductParams struct {
	Name                    string
	Slug                    string
	Description             *string
	ShortDescription        *string
	Status                  string
	SkuPrefix               *string
	BasePrice               pgtype.Numeric
	CompareAtPrice          pgtype.Numeric
	VatCategoryID           pgtype.UUID
	BaseWeightGrams         int32
	BaseDimensionsMm        json.RawMessage
	ShippingExtraFeePerUnit pgtype.Numeric
	HasVariants             bool
	SeoTitle                *string
	SeoDescription          *string
	Metadata                json.RawMessage
}

// List returns paginated products with an optional status filter.
// It returns the product slice, total count, and any error.
func (s *Service) List(ctx context.Context, status *string, page, pageSize int) ([]db.Product, int64, error) {
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

	// CountProducts and ListProducts both accept a string where empty means "no filter"
	// and the SQL uses ($1::text IS NULL OR status = $1::text).
	// pgx sends an empty string as a non-NULL text value, so we need to pass the
	// status directly or an empty string to get "all".
	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	total, err := s.queries.CountProducts(ctx, statusFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("counting products: %w", err)
	}

	products, err := s.queries.ListProducts(ctx, db.ListProductsParams{
		Column1: statusFilter,
		Limit:   int32(pageSize),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing products: %w", err)
	}

	return products, total, nil
}

// Get returns a single product by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Product, error) {
	product, err := s.queries.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, fmt.Errorf("getting product %s: %w", id, err)
	}
	return product, nil
}

// GetBySlug returns a product by its URL slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (db.Product, error) {
	product, err := s.queries.GetProductBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, fmt.Errorf("getting product by slug %q: %w", slug, err)
	}
	return product, nil
}

// Create creates a new product. It auto-generates a UUID and slug (from the
// product name) if they are not provided, and defaults the status to "draft".
func (s *Service) Create(ctx context.Context, params CreateProductParams) (db.Product, error) {
	if params.Name == "" {
		return db.Product{}, ErrNameRequired
	}

	if params.Status == "" {
		params.Status = "draft"
	}

	productSlug := params.Slug
	if productSlug == "" {
		productSlug = slugify(params.Name)
	}

	// Ensure slug uniqueness: if a product with this slug already exists,
	// append a short random suffix.
	productSlug, err := s.ensureUniqueSlug(ctx, productSlug)
	if err != nil {
		return db.Product{}, fmt.Errorf("ensuring unique slug: %w", err)
	}

	now := time.Now().UTC()

	product, err := s.queries.CreateProduct(ctx, db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    params.Name,
		Slug:                    productSlug,
		Description:             params.Description,
		ShortDescription:        params.ShortDescription,
		Status:                  params.Status,
		SkuPrefix:               params.SkuPrefix,
		BasePrice:               params.BasePrice,
		CompareAtPrice:          params.CompareAtPrice,
		VatCategoryID:           params.VatCategoryID,
		BaseWeightGrams:         params.BaseWeightGrams,
		BaseDimensionsMm:        params.BaseDimensionsMm,
		ShippingExtraFeePerUnit: params.ShippingExtraFeePerUnit,
		HasVariants:             params.HasVariants,
		SeoTitle:                params.SeoTitle,
		SeoDescription:          params.SeoDescription,
		Metadata:                params.Metadata,
		CreatedAt:               now,
	})
	if err != nil {
		return db.Product{}, fmt.Errorf("creating product: %w", err)
	}

	s.logger.Info("product created",
		slog.String("product_id", product.ID.String()),
		slog.String("name", product.Name),
		slog.String("slug", product.Slug),
	)

	return product, nil
}

// Update updates an existing product. The caller must provide the full set of
// fields; any zero-valued optional fields will overwrite the existing values.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateProductParams) (db.Product, error) {
	if params.Name == "" {
		return db.Product{}, ErrNameRequired
	}

	// Verify the product exists before attempting the update.
	existing, err := s.queries.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, fmt.Errorf("fetching product for update: %w", err)
	}

	productSlug := params.Slug
	if productSlug == "" {
		productSlug = existing.Slug
	}

	// If the slug changed, ensure uniqueness.
	if productSlug != existing.Slug {
		productSlug, err = s.ensureUniqueSlug(ctx, productSlug)
		if err != nil {
			return db.Product{}, fmt.Errorf("ensuring unique slug on update: %w", err)
		}
	}

	now := time.Now().UTC()

	product, err := s.queries.UpdateProduct(ctx, db.UpdateProductParams{
		ID:                      id,
		Name:                    params.Name,
		Slug:                    productSlug,
		Description:             params.Description,
		ShortDescription:        params.ShortDescription,
		Status:                  params.Status,
		SkuPrefix:               params.SkuPrefix,
		BasePrice:               params.BasePrice,
		CompareAtPrice:          params.CompareAtPrice,
		VatCategoryID:           params.VatCategoryID,
		BaseWeightGrams:         params.BaseWeightGrams,
		BaseDimensionsMm:        params.BaseDimensionsMm,
		ShippingExtraFeePerUnit: params.ShippingExtraFeePerUnit,
		HasVariants:             params.HasVariants,
		SeoTitle:                params.SeoTitle,
		SeoDescription:          params.SeoDescription,
		Metadata:                params.Metadata,
		UpdatedAt:               now,
	})
	if err != nil {
		return db.Product{}, fmt.Errorf("updating product %s: %w", id, err)
	}

	s.logger.Info("product updated",
		slog.String("product_id", product.ID.String()),
		slog.String("name", product.Name),
	)

	return product, nil
}

// Delete deletes a product by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrNotFound.
	_, err := s.queries.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching product for delete: %w", err)
	}

	if err := s.queries.DeleteProduct(ctx, id); err != nil {
		return fmt.Errorf("deleting product %s: %w", id, err)
	}

	s.logger.Info("product deleted", slog.String("product_id", id.String()))
	return nil
}

// Search searches products by name or SKU prefix, with an optional status
// filter and pagination.
func (s *Service) Search(ctx context.Context, query string, status *string, page, pageSize int) ([]db.Product, error) {
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

	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	products, err := s.queries.SearchProducts(ctx, db.SearchProductsParams{
		Column1: &query,
		Column2: statusFilter,
		Limit:   int32(pageSize),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("searching products: %w", err)
	}

	return products, nil
}

// SetCategories replaces all category associations for a product.
// It deletes existing associations and inserts the new set in order.
func (s *Service) SetCategories(ctx context.Context, productID uuid.UUID, categoryIDs []uuid.UUID) error {
	// Verify the product exists.
	_, err := s.queries.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching product for category assignment: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Delete all existing category associations.
	if err := qtx.SetProductCategories(ctx, productID); err != nil {
		return fmt.Errorf("clearing product categories: %w", err)
	}

	// Insert the new associations in order.
	for i, catID := range categoryIDs {
		if err := qtx.AddProductCategory(ctx, db.AddProductCategoryParams{
			ProductID:  productID,
			CategoryID: catID,
			Position:   int32(i),
		}); err != nil {
			return fmt.Errorf("adding category %s to product: %w", catID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing category update: %w", err)
	}

	s.logger.Info("product categories updated",
		slog.String("product_id", productID.String()),
		slog.Int("count", len(categoryIDs)),
	)

	return nil
}

// GetCategories returns the categories associated with a product.
func (s *Service) GetCategories(ctx context.Context, productID uuid.UUID) ([]db.Category, error) {
	categories, err := s.queries.ListProductCategories(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing categories for product %s: %w", productID, err)
	}
	return categories, nil
}

// ensureUniqueSlug checks whether the given slug already exists and, if so,
// appends a short random suffix to make it unique.
func (s *Service) ensureUniqueSlug(ctx context.Context, base string) (string, error) {
	candidate := base
	for attempts := 0; attempts < 10; attempts++ {
		_, err := s.queries.GetProductBySlug(ctx, candidate)
		if errors.Is(err, pgx.ErrNoRows) {
			// Slug is available.
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("checking slug availability: %w", err)
		}
		// Slug taken -- append a random suffix and retry.
		candidate = fmt.Sprintf("%s-%s", base, randomSuffix())
	}
	return "", fmt.Errorf("could not generate unique slug after 10 attempts for base %q", base)
}

// randomSuffix returns a short random alphanumeric string (5 chars).
func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

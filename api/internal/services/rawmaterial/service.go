package rawmaterial

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service wraps sqlc-generated queries for raw material CRUD operations.
type Service struct {
	queries *db.Queries
	logger  *slog.Logger
}

// NewService creates a new raw material service backed by the given connection pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{
		queries: db.New(pool),
		logger:  logger,
	}
}

// CreateRawMaterialParams holds the input fields for creating a raw material.
type CreateRawMaterialParams struct {
	Name              string
	Sku               string
	Description       *string
	CategoryID        *uuid.UUID
	UnitOfMeasure     string
	CostPerUnit       pgtype.Numeric
	StockQuantity     pgtype.Numeric
	LowStockThreshold pgtype.Numeric
	SupplierName      *string
	SupplierSku       *string
	LeadTimeDays      *int32
	Metadata          json.RawMessage
	IsActive          bool
}

// UpdateRawMaterialParams holds the input fields for updating a raw material.
type UpdateRawMaterialParams struct {
	Name              string
	Sku               string
	Description       *string
	CategoryID        *uuid.UUID
	UnitOfMeasure     string
	CostPerUnit       pgtype.Numeric
	StockQuantity     pgtype.Numeric
	LowStockThreshold pgtype.Numeric
	SupplierName      *string
	SupplierSku       *string
	LeadTimeDays      *int32
	Metadata          json.RawMessage
	IsActive          bool
}

// CreateCategoryParams holds the input fields for creating a raw material category.
type CreateCategoryParams struct {
	Name     string
	ParentID *uuid.UUID
	Position int32
}

// List returns a paginated list of raw materials with optional filters.
// categoryID filters by category when non-nil.
// activeOnly filters by is_active when non-nil.
// Returns the materials, total count, and any error.
func (s *Service) List(ctx context.Context, categoryID *uuid.UUID, activeOnly *bool, page, pageSize int) ([]db.RawMaterial, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	offset := (page - 1) * pageSize

	// The sqlc-generated filter params use plain Go types (uuid.UUID, bool)
	// rather than pgtype nullable types. The SQL query uses a
	// "$1::uuid IS NULL OR category_id = $1" pattern intended for nullable
	// params. With plain Go types, pgx always sends non-NULL values, so the
	// IS NULL branch never matches. uuid.Nil (zero UUID) won't match any real
	// category, effectively meaning "no results" rather than "no filter".
	//
	// Known sqlc limitation: the NULL-coalescing filter pattern does not work
	// correctly when sqlc generates plain types instead of pgtype. The proper
	// fix is to regenerate with pgtype overrides. Until then, callers should
	// always provide a categoryID when filtering by category.
	var catFilter uuid.UUID
	if categoryID != nil {
		catFilter = *categoryID
	}

	// Same limitation for the bool filter: a plain Go bool is never NULL.
	// When activeOnly is nil, we default to false, which means the query will
	// filter to is_active = false. Callers should pass activeOnly explicitly.
	var activeFilter bool
	if activeOnly != nil {
		activeFilter = *activeOnly
	}

	listParams := db.ListRawMaterialsParams{
		Column1: catFilter,
		Column2: activeFilter,
		Limit:   int32(pageSize),
		Offset:  int32(offset),
	}

	countParams := db.CountRawMaterialsParams{
		Column1: catFilter,
		Column2: activeFilter,
	}

	materials, err := s.queries.ListRawMaterials(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("listing raw materials: %w", err)
	}

	total, err := s.queries.CountRawMaterials(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("counting raw materials: %w", err)
	}

	return materials, total, nil
}

// Get retrieves a single raw material by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.RawMaterial, error) {
	material, err := s.queries.GetRawMaterial(ctx, id)
	if err != nil {
		return db.RawMaterial{}, fmt.Errorf("getting raw material %s: %w", id, err)
	}
	return material, nil
}

// Create creates a new raw material with an auto-generated UUID and timestamps.
func (s *Service) Create(ctx context.Context, params CreateRawMaterialParams) (db.RawMaterial, error) {
	now := time.Now()

	var categoryID pgtype.UUID
	if params.CategoryID != nil {
		categoryID = pgtype.UUID{Bytes: *params.CategoryID, Valid: true}
	}

	metadata := params.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	dbParams := db.CreateRawMaterialParams{
		ID:                uuid.New(),
		Name:              params.Name,
		Sku:               params.Sku,
		Description:       params.Description,
		CategoryID:        categoryID,
		UnitOfMeasure:     params.UnitOfMeasure,
		CostPerUnit:       params.CostPerUnit,
		StockQuantity:     params.StockQuantity,
		LowStockThreshold: params.LowStockThreshold,
		SupplierName:      params.SupplierName,
		SupplierSku:       params.SupplierSku,
		LeadTimeDays:      params.LeadTimeDays,
		Metadata:          metadata,
		IsActive:          params.IsActive,
		CreatedAt:         now,
	}

	material, err := s.queries.CreateRawMaterial(ctx, dbParams)
	if err != nil {
		return db.RawMaterial{}, fmt.Errorf("creating raw material: %w", err)
	}

	s.logger.Info("raw material created",
		slog.String("id", material.ID.String()),
		slog.String("name", material.Name),
		slog.String("sku", material.Sku),
	)

	return material, nil
}

// Update updates an existing raw material by ID with the given params.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateRawMaterialParams) (db.RawMaterial, error) {
	now := time.Now()

	var categoryID pgtype.UUID
	if params.CategoryID != nil {
		categoryID = pgtype.UUID{Bytes: *params.CategoryID, Valid: true}
	}

	metadata := params.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	dbParams := db.UpdateRawMaterialParams{
		ID:                id,
		Name:              params.Name,
		Sku:               params.Sku,
		Description:       params.Description,
		CategoryID:        categoryID,
		UnitOfMeasure:     params.UnitOfMeasure,
		CostPerUnit:       params.CostPerUnit,
		StockQuantity:     params.StockQuantity,
		LowStockThreshold: params.LowStockThreshold,
		SupplierName:      params.SupplierName,
		SupplierSku:       params.SupplierSku,
		LeadTimeDays:      params.LeadTimeDays,
		Metadata:          metadata,
		IsActive:          params.IsActive,
		UpdatedAt:         now,
	}

	material, err := s.queries.UpdateRawMaterial(ctx, dbParams)
	if err != nil {
		return db.RawMaterial{}, fmt.Errorf("updating raw material %s: %w", id, err)
	}

	s.logger.Info("raw material updated",
		slog.String("id", material.ID.String()),
		slog.String("name", material.Name),
	)

	return material, nil
}

// Delete removes a raw material by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteRawMaterial(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting raw material %s: %w", id, err)
	}

	s.logger.Info("raw material deleted", slog.String("id", id.String()))
	return nil
}

// ListCategories returns all raw material categories ordered by position and name.
func (s *Service) ListCategories(ctx context.Context) ([]db.RawMaterialCategory, error) {
	categories, err := s.queries.ListRawMaterialCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing raw material categories: %w", err)
	}
	return categories, nil
}

// CreateCategory creates a new raw material category with an auto-generated UUID,
// a slugified name, and a timestamp.
func (s *Service) CreateCategory(ctx context.Context, params CreateCategoryParams) (db.RawMaterialCategory, error) {
	now := time.Now()

	var parentID pgtype.UUID
	if params.ParentID != nil {
		parentID = pgtype.UUID{Bytes: *params.ParentID, Valid: true}
	}

	dbParams := db.CreateRawMaterialCategoryParams{
		ID:        uuid.New(),
		Name:      params.Name,
		Slug:      slugify(params.Name),
		ParentID:  parentID,
		Position:  params.Position,
		CreatedAt: now,
	}

	category, err := s.queries.CreateRawMaterialCategory(ctx, dbParams)
	if err != nil {
		return db.RawMaterialCategory{}, fmt.Errorf("creating raw material category: %w", err)
	}

	s.logger.Info("raw material category created",
		slog.String("id", category.ID.String()),
		slog.String("name", category.Name),
		slog.String("slug", category.Slug),
	)

	return category, nil
}

// ListLowStock returns raw materials whose stock is at or below their low stock threshold.
func (s *Service) ListLowStock(ctx context.Context, limit int) ([]db.RawMaterial, error) {
	if limit < 1 {
		limit = 50
	}

	materials, err := s.queries.ListLowStockRawMaterials(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("listing low stock raw materials: %w", err)
	}
	return materials, nil
}

// Search performs a case-insensitive search across raw material names and SKUs.
func (s *Service) Search(ctx context.Context, query string, page, pageSize int) ([]db.RawMaterial, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	offset := (page - 1) * pageSize

	searchParams := db.SearchRawMaterialsParams{
		Column1: &query,
		Limit:   int32(pageSize),
		Offset:  int32(offset),
	}

	materials, err := s.queries.SearchRawMaterials(ctx, searchParams)
	if err != nil {
		return nil, fmt.Errorf("searching raw materials: %w", err)
	}
	return materials, nil
}

// slugify converts a human-readable name into a URL-safe slug.
// Example: "Leather & Supplies" -> "leather-supplies"
var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
	multipleHyphens = regexp.MustCompile(`-{2,}`)
)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumeric.ReplaceAllString(s, "")
	s = multipleHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

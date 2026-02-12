package category

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateCategoryParams holds the input for creating a new category.
type CreateCategoryParams struct {
	Name           string
	Slug           string     // Auto-generated from Name if empty.
	Description    *string
	ParentID       *uuid.UUID // nil = top-level category.
	Position       int32
	ImageUrl       *string
	SeoTitle       *string
	SeoDescription *string
	IsActive       bool
}

// UpdateCategoryParams holds the input for updating an existing category.
type UpdateCategoryParams struct {
	Name           string
	Slug           string     // Auto-generated from Name if empty.
	Description    *string
	ParentID       *uuid.UUID // nil = top-level category.
	Position       int32
	ImageUrl       *string
	SeoTitle       *string
	SeoDescription *string
	IsActive       bool
}

// Service provides business logic for category CRUD operations.
type Service struct {
	queries *db.Queries
	logger  *slog.Logger
}

// NewService creates a new category service backed by the given connection pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{
		queries: db.New(pool),
		logger:  logger,
	}
}

// List returns categories ordered by position and name.
// When activeOnly is true, only active categories are returned.
func (s *Service) List(ctx context.Context, activeOnly bool) ([]db.Category, error) {
	if activeOnly {
		categories, err := s.queries.ListCategories(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing active categories: %w", err)
		}
		return categories, nil
	}

	categories, err := s.queries.ListAllCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing all categories: %w", err)
	}
	return categories, nil
}

// ListTop returns active top-level categories (parent_id IS NULL).
func (s *Service) ListTop(ctx context.Context) ([]db.Category, error) {
	categories, err := s.queries.ListTopCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing top categories: %w", err)
	}
	return categories, nil
}

// ListChildren returns active child categories for the given parent.
func (s *Service) ListChildren(ctx context.Context, parentID uuid.UUID) ([]db.Category, error) {
	pgParentID := uuidToPgtype(parentID)
	categories, err := s.queries.ListChildCategories(ctx, pgParentID)
	if err != nil {
		return nil, fmt.Errorf("listing children of category %s: %w", parentID, err)
	}
	return categories, nil
}

// Get returns a single category by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.Category, error) {
	cat, err := s.queries.GetCategory(ctx, id)
	if err != nil {
		return db.Category{}, fmt.Errorf("getting category %s: %w", id, err)
	}
	return cat, nil
}

// GetBySlug returns a single category by its URL slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (db.Category, error) {
	cat, err := s.queries.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return db.Category{}, fmt.Errorf("getting category by slug %q: %w", slug, err)
	}
	return cat, nil
}

// Create creates a new category. If Slug is empty it is auto-generated from Name.
func (s *Service) Create(ctx context.Context, params CreateCategoryParams) (db.Category, error) {
	slug := params.Slug
	if slug == "" {
		slug = slugify(params.Name)
	}

	now := time.Now().UTC()

	cat, err := s.queries.CreateCategory(ctx, db.CreateCategoryParams{
		ID:             uuid.New(),
		Name:           params.Name,
		Slug:           slug,
		Description:    params.Description,
		ParentID:       optionalUUIDToPgtype(params.ParentID),
		Position:       params.Position,
		ImageUrl:       params.ImageUrl,
		SeoTitle:       params.SeoTitle,
		SeoDescription: params.SeoDescription,
		IsActive:       params.IsActive,
		CreatedAt:      now,
	})
	if err != nil {
		return db.Category{}, fmt.Errorf("creating category %q: %w", params.Name, err)
	}

	s.logger.Info("category created",
		slog.String("id", cat.ID.String()),
		slog.String("name", cat.Name),
		slog.String("slug", cat.Slug),
	)

	return cat, nil
}

// Update updates an existing category. If Slug is empty it is auto-generated from Name.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateCategoryParams) (db.Category, error) {
	slug := params.Slug
	if slug == "" {
		slug = slugify(params.Name)
	}

	now := time.Now().UTC()

	cat, err := s.queries.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:             id,
		Name:           params.Name,
		Slug:           slug,
		Description:    params.Description,
		ParentID:       optionalUUIDToPgtype(params.ParentID),
		Position:       params.Position,
		ImageUrl:       params.ImageUrl,
		SeoTitle:       params.SeoTitle,
		SeoDescription: params.SeoDescription,
		IsActive:       params.IsActive,
		UpdatedAt:      now,
	})
	if err != nil {
		return db.Category{}, fmt.Errorf("updating category %s: %w", id, err)
	}

	s.logger.Info("category updated",
		slog.String("id", cat.ID.String()),
		slog.String("name", cat.Name),
		slog.String("slug", cat.Slug),
	)

	return cat, nil
}

// Delete removes a category by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteCategory(ctx, id); err != nil {
		return fmt.Errorf("deleting category %s: %w", id, err)
	}

	s.logger.Info("category deleted", slog.String("id", id.String()))

	return nil
}

// CountProducts returns the number of products associated with a category.
func (s *Service) CountProducts(ctx context.Context, id uuid.UUID) (int64, error) {
	count, err := s.queries.CountProductsInCategory(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("counting products in category %s: %w", id, err)
	}
	return count, nil
}

// uuidToPgtype converts a uuid.UUID to a pgtype.UUID with Valid=true.
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

// optionalUUIDToPgtype converts an optional *uuid.UUID to a pgtype.UUID.
// A nil pointer results in an invalid (NULL) pgtype.UUID.
func optionalUUIDToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{
		Bytes: *id,
		Valid: true,
	}
}

package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/services/category"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/variant"
)

// PublicHandler holds dependencies for public-facing API handlers.
type PublicHandler struct {
	productSvc  *product.Service
	categorySvc *category.Service
	variantSvc  *variant.Service
	queries     *db.Queries
	logger      *slog.Logger
}

// NewPublicHandler creates a new public API handler with all required dependencies.
func NewPublicHandler(
	productSvc *product.Service,
	categorySvc *category.Service,
	variantSvc *variant.Service,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) *PublicHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PublicHandler{
		productSvc:  productSvc,
		categorySvc: categorySvc,
		variantSvc:  variantSvc,
		queries:     db.New(pool),
		logger:      logger,
	}
}

// RegisterRoutes registers all public API routes on the given mux.
func (h *PublicHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/products", h.ListProducts)
	mux.HandleFunc("GET /api/v1/products/{slug}", h.GetProduct)
	mux.HandleFunc("GET /api/v1/products/{slug}/variants", h.ListProductVariants)
	mux.HandleFunc("GET /api/v1/categories", h.ListCategories)
	mux.HandleFunc("GET /api/v1/countries", h.ListCountries)
}

// --- JSON response types ---

// listResponse is the standard paginated list response wrapper.
type listResponse struct {
	Data       any   `json:"data"`
	Page       int   `json:"page"`
	TotalPages int   `json:"total_pages"`
	Total      int64 `json:"total"`
}

// imageJSON is the public-facing image representation.
type imageJSON struct {
	ID        uuid.UUID  `json:"id"`
	URL       string     `json:"url"`
	AltText   *string    `json:"alt_text"`
	Position  int32      `json:"position"`
	IsPrimary bool       `json:"is_primary"`
	VariantID *uuid.UUID `json:"variant_id,omitempty"`
}

// productSummary is the public-facing product representation for list endpoints.
type productSummary struct {
	ID               uuid.UUID      `json:"id"`
	Name             string         `json:"name"`
	Slug             string         `json:"slug"`
	SkuPrefix        *string        `json:"sku_prefix"`
	BasePrice        pgtype.Numeric `json:"base_price"`
	CompareAtPrice   pgtype.Numeric `json:"compare_at_price"`
	ShortDescription *string        `json:"short_description"`
	Status           string         `json:"status"`
	HasVariants      bool           `json:"has_variants"`
	FeaturedImage    *imageJSON     `json:"featured_image"`
	CreatedAt        time.Time      `json:"created_at"`
}

// productDetail is the full product representation for the single-product endpoint.
type productDetail struct {
	ID                      uuid.UUID       `json:"id"`
	Name                    string          `json:"name"`
	Slug                    string          `json:"slug"`
	Description             *string         `json:"description"`
	ShortDescription        *string         `json:"short_description"`
	Status                  string          `json:"status"`
	SkuPrefix               *string         `json:"sku_prefix"`
	BasePrice               pgtype.Numeric  `json:"base_price"`
	CompareAtPrice          pgtype.Numeric  `json:"compare_at_price"`
	BaseWeightGrams         int32           `json:"base_weight_grams"`
	HasVariants             bool            `json:"has_variants"`
	SeoTitle                *string         `json:"seo_title,omitempty"`
	SeoDescription          *string         `json:"seo_description,omitempty"`
	Metadata                json.RawMessage `json:"metadata,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
	Images                  []imageJSON     `json:"images"`
	Attributes              []attributeJSON `json:"attributes"`
	Variants                []variantJSON   `json:"variants"`
}

// attributeJSON represents a product attribute with its options.
type attributeJSON struct {
	ID            uuid.UUID    `json:"id"`
	Name          string       `json:"name"`
	DisplayName   string       `json:"display_name"`
	AttributeType string       `json:"attribute_type"`
	Position      int32        `json:"position"`
	Options       []optionJSON `json:"options"`
}

// optionJSON represents a single attribute option.
type optionJSON struct {
	ID                  uuid.UUID      `json:"id"`
	Value               string         `json:"value"`
	DisplayValue        string         `json:"display_value"`
	ColorHex            *string        `json:"color_hex,omitempty"`
	PriceModifier       pgtype.Numeric `json:"price_modifier"`
	WeightModifierGrams *int32         `json:"weight_modifier_grams,omitempty"`
	Position            int32          `json:"position"`
}

// variantJSON is the public-facing variant representation.
type variantJSON struct {
	ID             uuid.UUID         `json:"id"`
	Sku            string            `json:"sku"`
	Price          pgtype.Numeric    `json:"price"`
	CompareAtPrice pgtype.Numeric    `json:"compare_at_price"`
	StockQuantity  int32             `json:"stock_quantity"`
	WeightGrams    *int32            `json:"weight_grams,omitempty"`
	Barcode        *string           `json:"barcode,omitempty"`
	IsActive       bool              `json:"is_active"`
	Position       int32             `json:"position"`
	Options        []variantOptJSON  `json:"options"`
	Images         []imageJSON       `json:"images"`
}

// variantOptJSON represents the attribute-option pair for a variant.
type variantOptJSON struct {
	AttributeName      string `json:"attribute_name"`
	OptionValue        string `json:"option_value"`
	OptionDisplayValue string `json:"option_display_value"`
}

// categoryJSON is the public-facing category representation.
type categoryJSON struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Description *string     `json:"description,omitempty"`
	ParentID    *uuid.UUID  `json:"parent_id"`
	Position    int32       `json:"position"`
	ImageUrl    *string     `json:"image_url,omitempty"`
}

// countryJSON is the public-facing country representation.
type countryJSON struct {
	CountryCode string `json:"country_code"`
	Name        string `json:"name"`
}

// errorJSON is the error response format.
type errorJSON struct {
	Error string `json:"error"`
}

// --- Handlers ---

// ListProducts handles GET /api/v1/products
func (h *PublicHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	// Public API always filters to active products only.
	status := "active"

	products, total, err := h.productSvc.List(r.Context(), &status, page, limit)
	if err != nil {
		h.logger.Error("failed to list products", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	summaries := make([]productSummary, len(products))
	for i, p := range products {
		summaries[i] = productSummary{
			ID:               p.ID,
			Name:             p.Name,
			Slug:             p.Slug,
			SkuPrefix:        p.SkuPrefix,
			BasePrice:        p.BasePrice,
			CompareAtPrice:   p.CompareAtPrice,
			ShortDescription: p.ShortDescription,
			Status:           p.Status,
			HasVariants:      p.HasVariants,
			CreatedAt:        p.CreatedAt,
		}

		// Fetch featured image (primary image) for each product.
		if primary, err := h.queries.GetPrimaryImageByProduct(r.Context(), p.ID); err == nil {
			summaries[i].FeaturedImage = productImageToJSON(primary)
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	writeJSON(w, http.StatusOK, listResponse{
		Data:       summaries,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	})
}

// GetProduct handles GET /api/v1/products/{slug}
func (h *PublicHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "slug is required"})
		return
	}

	p, err := h.productSvc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "product not found"})
			return
		}
		h.logger.Error("failed to get product", "slug", slug, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Only expose active products via the public API.
	if p.Status != "active" {
		writeJSON(w, http.StatusNotFound, errorJSON{Error: "product not found"})
		return
	}

	// Load attributes and their options.
	attrs, err := h.queries.ListProductAttributes(r.Context(), p.ID)
	if err != nil {
		h.logger.Error("failed to list product attributes", "product_id", p.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	attrList := make([]attributeJSON, 0, len(attrs))
	for _, a := range attrs {
		opts, err := h.queries.ListAttributeOptions(r.Context(), a.ID)
		if err != nil {
			h.logger.Error("failed to list attribute options", "attribute_id", a.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
			return
		}

		// Only include active options.
		optList := make([]optionJSON, 0, len(opts))
		for _, o := range opts {
			if !o.IsActive {
				continue
			}
			optList = append(optList, optionJSON{
				ID:                  o.ID,
				Value:               o.Value,
				DisplayValue:        o.DisplayValue,
				ColorHex:            o.ColorHex,
				PriceModifier:       o.PriceModifier,
				WeightModifierGrams: o.WeightModifierGrams,
				Position:            o.Position,
			})
		}

		attrList = append(attrList, attributeJSON{
			ID:            a.ID,
			Name:          a.Name,
			DisplayName:   a.DisplayName,
			AttributeType: a.AttributeType,
			Position:      a.Position,
			Options:       optList,
		})
	}

	// Load all product images (product-level + variant-level, ordered by position).
	allImages, err := h.queries.ListProductImagesByProduct(r.Context(), p.ID)
	if err != nil {
		h.logger.Error("failed to list product images", "product_id", p.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Build image list and index variant images by variant ID.
	imageList := make([]imageJSON, 0, len(allImages))
	variantImages := make(map[uuid.UUID][]imageJSON)
	for _, img := range allImages {
		ij := productImageToJSON(img)
		imageList = append(imageList, *ij)

		if img.VariantID.Valid {
			vid := uuid.UUID(img.VariantID.Bytes)
			variantImages[vid] = append(variantImages[vid], *ij)
		}
	}

	// Load active variants with their options.
	variants, err := h.variantSvc.List(r.Context(), p.ID)
	if err != nil {
		h.logger.Error("failed to list variants", "product_id", p.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	variantList := make([]variantJSON, 0, len(variants))
	for _, v := range variants {
		if !v.IsActive {
			continue
		}

		vOpts, err := h.variantSvc.ListOptions(r.Context(), v.ID)
		if err != nil {
			h.logger.Error("failed to list variant options", "variant_id", v.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
			return
		}

		optEntries := make([]variantOptJSON, len(vOpts))
		for j, vo := range vOpts {
			optEntries[j] = variantOptJSON{
				AttributeName:      vo.AttributeName,
				OptionValue:        vo.OptionValue,
				OptionDisplayValue: vo.OptionDisplayValue,
			}
		}

		// Attach variant-specific images (empty array if none).
		vImages := variantImages[v.ID]
		if vImages == nil {
			vImages = []imageJSON{}
		}

		variantList = append(variantList, variantJSON{
			ID:             v.ID,
			Sku:            v.Sku,
			Price:          v.Price,
			CompareAtPrice: v.CompareAtPrice,
			StockQuantity:  v.StockQuantity,
			WeightGrams:    v.WeightGrams,
			Barcode:        v.Barcode,
			IsActive:       v.IsActive,
			Position:       v.Position,
			Options:        optEntries,
			Images:         vImages,
		})
	}

	detail := productDetail{
		ID:               p.ID,
		Name:             p.Name,
		Slug:             p.Slug,
		Description:      p.Description,
		ShortDescription: p.ShortDescription,
		Status:           p.Status,
		SkuPrefix:        p.SkuPrefix,
		BasePrice:        p.BasePrice,
		CompareAtPrice:   p.CompareAtPrice,
		BaseWeightGrams:  p.BaseWeightGrams,
		HasVariants:      p.HasVariants,
		SeoTitle:         p.SeoTitle,
		SeoDescription:   p.SeoDescription,
		Metadata:         p.Metadata,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		Images:           imageList,
		Attributes:       attrList,
		Variants:         variantList,
	}

	writeJSON(w, http.StatusOK, detail)
}

// ListProductVariants handles GET /api/v1/products/{slug}/variants
func (h *PublicHandler) ListProductVariants(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "slug is required"})
		return
	}

	p, err := h.productSvc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "product not found"})
			return
		}
		h.logger.Error("failed to get product for variants", "slug", slug, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	if p.Status != "active" {
		writeJSON(w, http.StatusNotFound, errorJSON{Error: "product not found"})
		return
	}

	variants, err := h.variantSvc.List(r.Context(), p.ID)
	if err != nil {
		h.logger.Error("failed to list variants", "product_id", p.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Pre-load all images for this product and index by variant ID.
	allImages, _ := h.queries.ListProductImagesByProduct(r.Context(), p.ID)
	variantImgs := make(map[uuid.UUID][]imageJSON)
	for _, img := range allImages {
		if img.VariantID.Valid {
			vid := uuid.UUID(img.VariantID.Bytes)
			variantImgs[vid] = append(variantImgs[vid], *productImageToJSON(img))
		}
	}

	result := make([]variantJSON, 0, len(variants))
	for _, v := range variants {
		if !v.IsActive {
			continue
		}

		vOpts, err := h.variantSvc.ListOptions(r.Context(), v.ID)
		if err != nil {
			h.logger.Error("failed to list variant options", "variant_id", v.ID, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
			return
		}

		optEntries := make([]variantOptJSON, len(vOpts))
		for j, vo := range vOpts {
			optEntries[j] = variantOptJSON{
				AttributeName:      vo.AttributeName,
				OptionValue:        vo.OptionValue,
				OptionDisplayValue: vo.OptionDisplayValue,
			}
		}

		vImages := variantImgs[v.ID]
		if vImages == nil {
			vImages = []imageJSON{}
		}

		result = append(result, variantJSON{
			ID:             v.ID,
			Sku:            v.Sku,
			Price:          v.Price,
			CompareAtPrice: v.CompareAtPrice,
			StockQuantity:  v.StockQuantity,
			WeightGrams:    v.WeightGrams,
			Barcode:        v.Barcode,
			IsActive:       v.IsActive,
			Position:       v.Position,
			Options:        optEntries,
			Images:         vImages,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// ListCategories handles GET /api/v1/categories
func (h *PublicHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	// Public API returns only active categories.
	categories, err := h.categorySvc.List(r.Context(), true)
	if err != nil {
		h.logger.Error("failed to list categories", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	result := make([]categoryJSON, len(categories))
	for i, c := range categories {
		result[i] = categoryJSON{
			ID:          c.ID,
			Name:        c.Name,
			Slug:        c.Slug,
			Description: c.Description,
			ParentID:    pgtypeUUIDToPtr(c.ParentID),
			Position:    c.Position,
			ImageUrl:    c.ImageUrl,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// ListCountries handles GET /api/v1/countries
func (h *PublicHandler) ListCountries(w http.ResponseWriter, r *http.Request) {
	countries, err := h.queries.ListEnabledShippingCountries(r.Context())
	if err != nil {
		h.logger.Error("failed to list enabled countries", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	result := make([]countryJSON, len(countries))
	for i, c := range countries {
		result[i] = countryJSON{
			CountryCode: c.CountryCode,
			Name:        c.Name,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Helpers ---

// writeJSON marshals v as JSON and writes it to the response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers are already sent; just log the error.
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// parsePagination extracts page and limit from query parameters with defaults.
func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20

	if v := r.URL.Query().Get("page"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
			if limit > 250 {
				limit = 250
			}
		}
	}

	return page, limit
}

// pgtypeUUIDToPtr converts a pgtype.UUID to a *uuid.UUID pointer.
// Returns nil when the pgtype.UUID is not valid (SQL NULL).
func pgtypeUUIDToPtr(pg pgtype.UUID) *uuid.UUID {
	if !pg.Valid {
		return nil
	}
	id := uuid.UUID(pg.Bytes)
	return &id
}

// productImageToJSON converts a database ProductImage to the public API imageJSON.
func productImageToJSON(img db.ProductImage) *imageJSON {
	return &imageJSON{
		ID:        img.ID,
		URL:       img.Url,
		AltText:   img.AltText,
		Position:  img.Position,
		IsPrimary: img.IsPrimary,
		VariantID: pgtypeUUIDToPtr(img.VariantID),
	}
}

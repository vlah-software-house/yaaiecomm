package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/category"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/templates/admin"
)

const defaultPageSize = 20

// ProductHandler handles admin product CRUD endpoints.
type ProductHandler struct {
	products   *product.Service
	categories *category.Service
	logger     *slog.Logger
}

// NewProductHandler creates a new product handler.
func NewProductHandler(products *product.Service, categories *category.Service, logger *slog.Logger) *ProductHandler {
	return &ProductHandler{
		products:   products,
		categories: categories,
		logger:     logger,
	}
}

// RegisterRoutes registers product admin routes on the given mux.
func (h *ProductHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products", h.ListProducts)
	mux.HandleFunc("GET /admin/products/new", h.ShowNewProduct)
	mux.HandleFunc("POST /admin/products", h.CreateProduct)
	mux.HandleFunc("GET /admin/products/{id}", h.ShowEditProduct)
	mux.HandleFunc("POST /admin/products/{id}", h.UpdateProduct)
	mux.HandleFunc("POST /admin/products/{id}/delete", h.DeleteProduct)
}

// ListProducts handles GET /admin/products.
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	var statusFilter *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = &s
	}

	products, total, err := h.products.List(r.Context(), statusFilter, page, defaultPageSize)
	if err != nil {
		h.logger.Error("failed to list products", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalPages := int((total + int64(defaultPageSize) - 1) / int64(defaultPageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	items := make([]admin.ProductListItem, 0, len(products))
	for _, p := range products {
		items = append(items, admin.ProductListItem{
			ID:        p.ID.String(),
			Name:      p.Name,
			SKU:       derefString(p.SkuPrefix),
			Status:    p.Status,
			Price:     formatNumeric(p.BasePrice),
			Stock:     0, // Variant stock aggregation not yet implemented
			CreatedAt: p.CreatedAt.Format("2006-01-02"),
		})
	}

	statusStr := ""
	if statusFilter != nil {
		statusStr = *statusFilter
	}

	data := admin.ProductListData{
		Products:   items,
		Page:       page,
		TotalPages: totalPages,
		Total:      int(total),
		Status:     statusStr,
	}

	admin.ProductListPage(data).Render(r.Context(), w)
}

// ShowNewProduct handles GET /admin/products/new.
func (h *ProductHandler) ShowNewProduct(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	data := admin.ProductFormData{
		Status:    "draft",
		IsNew:     true,
		CSRFToken: csrfToken,
	}
	admin.ProductFormPage(data).Render(r.Context(), w)
}

// CreateProduct handles POST /admin/products.
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		h.renderFormWithError(w, r, admin.ProductFormData{
			IsNew:     true,
			CSRFToken: csrfToken,
		}, "Invalid form data.")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderFormWithError(w, r, formDataFromRequest(r, csrfToken, true), "Product name is required.")
		return
	}

	params := product.CreateProductParams{
		Name:             name,
		Slug:             strings.TrimSpace(r.FormValue("slug")),
		Description:      strPtr(r.FormValue("description")),
		ShortDescription: strPtr(r.FormValue("short_description")),
		Status:           r.FormValue("status"),
		SkuPrefix:        strPtr(r.FormValue("sku_prefix")),
		BasePrice:        parseNumeric(r.FormValue("base_price")),
		CompareAtPrice:   parseNumeric(r.FormValue("compare_at_price")),
		BaseWeightGrams:  parseInt32(r.FormValue("weight_grams")),
		HasVariants:      r.FormValue("has_variants") == "on",
		SeoTitle:         strPtr(r.FormValue("seo_title")),
		SeoDescription:   strPtr(r.FormValue("seo_description")),
	}

	created, err := h.products.Create(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to create product", "error", err)
		msg := "Failed to create product."
		if errors.Is(err, product.ErrNameRequired) {
			msg = "Product name is required."
		}
		h.renderFormWithError(w, r, formDataFromRequest(r, csrfToken, true), msg)
		return
	}

	http.Redirect(w, r, "/admin/products/"+created.ID.String(), http.StatusSeeOther)
}

// ShowEditProduct handles GET /admin/products/{id}.
func (h *ProductHandler) ShowEditProduct(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	p, err := h.products.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get product", "error", err, "product_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := productToFormData(p, csrfToken)
	admin.ProductFormPage(data).Render(r.Context(), w)
}

// UpdateProduct handles POST /admin/products/{id}.
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		formData := formDataFromRequest(r, csrfToken, false)
		formData.ID = id.String()
		h.renderFormWithError(w, r, formData, "Product name is required.")
		return
	}

	params := product.UpdateProductParams{
		Name:             name,
		Slug:             strings.TrimSpace(r.FormValue("slug")),
		Description:      strPtr(r.FormValue("description")),
		ShortDescription: strPtr(r.FormValue("short_description")),
		Status:           r.FormValue("status"),
		SkuPrefix:        strPtr(r.FormValue("sku_prefix")),
		BasePrice:        parseNumeric(r.FormValue("base_price")),
		CompareAtPrice:   parseNumeric(r.FormValue("compare_at_price")),
		BaseWeightGrams:  parseInt32(r.FormValue("weight_grams")),
		HasVariants:      r.FormValue("has_variants") == "on",
		SeoTitle:         strPtr(r.FormValue("seo_title")),
		SeoDescription:   strPtr(r.FormValue("seo_description")),
	}

	updated, err := h.products.Update(r.Context(), id, params)
	if err != nil {
		h.logger.Error("failed to update product", "error", err, "product_id", id)
		msg := "Failed to update product."
		if errors.Is(err, product.ErrNotFound) {
			msg = "Product not found."
		} else if errors.Is(err, product.ErrNameRequired) {
			msg = "Product name is required."
		}
		formData := formDataFromRequest(r, csrfToken, false)
		formData.ID = id.String()
		h.renderFormWithError(w, r, formData, msg)
		return
	}

	data := productToFormData(updated, csrfToken)
	admin.ProductFormPage(data).Render(r.Context(), w)
}

// DeleteProduct handles POST /admin/products/{id}/delete.
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := h.products.Delete(r.Context(), id); err != nil {
		if errors.Is(err, product.ErrNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete product", "error", err, "product_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/products", http.StatusSeeOther)
}

// --- Helpers ---

// renderFormWithError sets a 422 status and renders the product form with an error message.
func (h *ProductHandler) renderFormWithError(w http.ResponseWriter, r *http.Request, data admin.ProductFormData, errMsg string) {
	data.Error = errMsg
	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.ProductFormPage(data).Render(r.Context(), w)
}

// productToFormData converts a db.Product into the template form data struct.
func productToFormData(p db.Product, csrfToken string) admin.ProductFormData {
	return admin.ProductFormData{
		ID:               p.ID.String(),
		Name:             p.Name,
		Slug:             p.Slug,
		Description:      derefString(p.Description),
		ShortDescription: derefString(p.ShortDescription),
		Status:           p.Status,
		SKUPrefix:        derefString(p.SkuPrefix),
		BasePrice:        formatNumeric(p.BasePrice),
		CompareAtPrice:   formatNumeric(p.CompareAtPrice),
		HasVariants:      p.HasVariants,
		WeightGrams:      formatInt32(p.BaseWeightGrams),
		SEOTitle:         derefString(p.SeoTitle),
		SEODescription:   derefString(p.SeoDescription),
		IsNew:            false,
		CSRFToken:        csrfToken,
	}
}

// formDataFromRequest reconstructs form data from the submitted request values
// so the form can be re-rendered with user input preserved on error.
func formDataFromRequest(r *http.Request, csrfToken string, isNew bool) admin.ProductFormData {
	return admin.ProductFormData{
		Name:             r.FormValue("name"),
		Slug:             r.FormValue("slug"),
		Description:      r.FormValue("description"),
		ShortDescription: r.FormValue("short_description"),
		Status:           r.FormValue("status"),
		SKUPrefix:        r.FormValue("sku_prefix"),
		BasePrice:        r.FormValue("base_price"),
		CompareAtPrice:   r.FormValue("compare_at_price"),
		HasVariants:      r.FormValue("has_variants") == "on",
		WeightGrams:      r.FormValue("weight_grams"),
		SEOTitle:         r.FormValue("seo_title"),
		SEODescription:   r.FormValue("seo_description"),
		IsNew:            isNew,
		CSRFToken:        csrfToken,
	}
}

// parseNumeric converts a decimal string (e.g., "19.99") to a pgtype.Numeric.
// Returns an invalid (NULL) Numeric if the string is empty.
func parseNumeric(s string) pgtype.Numeric {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// formatNumeric converts a pgtype.Numeric to its string representation.
// Returns an empty string for invalid (NULL) values.
func formatNumeric(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

// parseInt32 parses a string into an int32, returning 0 on failure.
func parseInt32(s string) int32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}

// formatInt32 formats an int32 as a string, returning "0" for zero values.
func formatInt32(v int32) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(int64(v), 10)
}

// strPtr returns a pointer to s, or nil if s is empty.
func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

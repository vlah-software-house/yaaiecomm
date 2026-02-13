package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/templates/admin"
)

// ProductVATHandler handles the VAT tab on the product edit page,
// including the product's default VAT category and per-country overrides.
type ProductVATHandler struct {
	products *product.Service
	queries  *db.Queries
	logger   *slog.Logger
}

// NewProductVATHandler creates a new product VAT handler.
func NewProductVATHandler(products *product.Service, pool *pgxpool.Pool, logger *slog.Logger) *ProductVATHandler {
	return &ProductVATHandler{
		products: products,
		queries:  db.New(pool),
		logger:   logger,
	}
}

// RegisterRoutes registers product VAT admin routes on the given mux.
func (h *ProductVATHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products/{id}/vat", h.ShowProductVAT)
	mux.HandleFunc("POST /admin/products/{id}/vat", h.UpdateProductVATCategory)
	mux.HandleFunc("POST /admin/products/{id}/vat/overrides", h.AddOverride)
	mux.HandleFunc("POST /admin/products/{id}/vat/overrides/{overrideId}/delete", h.DeleteOverride)
}

// ShowProductVAT handles GET /admin/products/{id}/vat.
// It loads the product, VAT categories, EU countries, and existing overrides,
// then renders the VAT tab page.
func (h *ProductVATHandler) ShowProductVAT(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	p, err := h.products.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get product", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	vatCategories, err := h.queries.ListVATCategories(ctx)
	if err != nil {
		h.logger.Error("failed to list VAT categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	euCountries, err := h.queries.ListEUCountries(ctx)
	if err != nil {
		h.logger.Error("failed to list EU countries", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	overrides, err := h.queries.ListProductVATOverrides(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list product VAT overrides", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Resolve the product's VatCategoryID (UUID) to its category name for the template.
	vatCategoryName := ""
	if p.VatCategoryID.Valid {
		cat, err := h.queries.GetVATCategory(ctx, p.VatCategoryID.Bytes)
		if err == nil {
			vatCategoryName = cat.Name
		}
	}

	data := admin.ProductVATData{
		ProductID:     productID.String(),
		ProductName:   p.Name,
		VATCategoryID: vatCategoryName,
		VATCategories: toVATCategoryItems(vatCategories),
		Overrides:     toOverrideItems(overrides),
		EUCountries:   toVATCountryItems(euCountries),
		CSRFToken:     csrfToken,
	}

	admin.ProductVATPage(data).Render(ctx, w)
}

// UpdateProductVATCategory handles POST /admin/products/{id}/vat.
// It updates the product's default VAT category and redirects back to the VAT tab.
func (h *ProductVATHandler) UpdateProductVATCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Get the existing product so we can build a full update params struct.
	p, err := h.products.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get product", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Resolve the VAT category name to a UUID. Empty string means use store default (NULL).
	vatCategoryName := strings.TrimSpace(r.FormValue("vat_category_id"))
	var vatCategoryID pgtype.UUID
	if vatCategoryName != "" {
		cat, err := h.queries.GetVATCategoryByName(ctx, vatCategoryName)
		if err != nil {
			h.logger.Error("failed to find VAT category", "error", err, "name", vatCategoryName)
			http.Error(w, "Invalid VAT category", http.StatusBadRequest)
			return
		}
		vatCategoryID = pgtype.UUID{Bytes: cat.ID, Valid: true}
	}

	// Build the full update params from the existing product, changing only VatCategoryID.
	params := product.UpdateProductParams{
		Name:                    p.Name,
		Slug:                    p.Slug,
		Description:             p.Description,
		ShortDescription:        p.ShortDescription,
		Status:                  p.Status,
		SkuPrefix:               p.SkuPrefix,
		BasePrice:               p.BasePrice,
		CompareAtPrice:          p.CompareAtPrice,
		VatCategoryID:           vatCategoryID,
		BaseWeightGrams:         p.BaseWeightGrams,
		BaseDimensionsMm:        p.BaseDimensionsMm,
		ShippingExtraFeePerUnit: p.ShippingExtraFeePerUnit,
		HasVariants:             p.HasVariants,
		SeoTitle:                p.SeoTitle,
		SeoDescription:          p.SeoDescription,
		Metadata:                p.Metadata,
	}

	if _, err := h.products.Update(ctx, productID, params); err != nil {
		h.logger.Error("failed to update product VAT category", "error", err, "product_id", productID)
		http.Error(w, "Failed to update VAT category", http.StatusInternalServerError)
		return
	}

	h.logger.Info("product VAT category updated",
		"product_id", productID,
		"vat_category", vatCategoryName,
	)

	http.Redirect(w, r, fmt.Sprintf("/admin/products/%s/vat", productID), http.StatusSeeOther)
}

// AddOverride handles POST /admin/products/{id}/vat/overrides.
// It creates (or upserts) a per-country VAT override and returns an HTML table row
// fragment for HTMX to append.
func (h *ProductVATHandler) AddOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	countryCode := strings.TrimSpace(r.FormValue("country_code"))
	if countryCode == "" {
		http.Error(w, "Country is required", http.StatusBadRequest)
		return
	}

	vatCategoryName := strings.TrimSpace(r.FormValue("vat_category"))
	if vatCategoryName == "" {
		http.Error(w, "VAT category is required", http.StatusBadRequest)
		return
	}

	notes := strPtr(r.FormValue("notes"))

	// Resolve the VAT category name to its UUID.
	cat, err := h.queries.GetVATCategoryByName(ctx, vatCategoryName)
	if err != nil {
		h.logger.Error("failed to find VAT category", "error", err, "name", vatCategoryName)
		http.Error(w, "Invalid VAT category", http.StatusBadRequest)
		return
	}

	// Look up the country name for the response.
	country, err := h.queries.GetEUCountry(ctx, countryCode)
	if err != nil {
		h.logger.Error("failed to find EU country", "error", err, "country_code", countryCode)
		http.Error(w, "Invalid country", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	override, err := h.queries.UpsertProductVATOverride(ctx, db.UpsertProductVATOverrideParams{
		ID:            uuid.New(),
		ProductID:     productID,
		CountryCode:   countryCode,
		VatCategoryID: cat.ID,
		Notes:         notes,
		CreatedAt:     now,
	})
	if err != nil {
		h.logger.Error("failed to upsert product VAT override", "error", err,
			"product_id", productID, "country_code", countryCode)
		http.Error(w, "Failed to add override", http.StatusInternalServerError)
		return
	}

	h.logger.Info("product VAT override added",
		"product_id", productID,
		"country_code", countryCode,
		"vat_category", vatCategoryName,
	)

	item := admin.ProductVATOverrideItem{
		ID:                  override.ID.String(),
		CountryCode:         countryCode,
		CountryName:         country.Name,
		CategoryName:        cat.Name,
		CategoryDisplayName: cat.DisplayName,
		Notes:               derefString(notes),
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ProductVATOverrideRow(productID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteOverride handles POST /admin/products/{id}/vat/overrides/{overrideId}/delete.
// It deletes a per-country VAT override and returns an empty response so HTMX
// removes the table row via outerHTML swap.
func (h *ProductVATHandler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	overrideID, err := uuid.Parse(r.PathValue("overrideId"))
	if err != nil {
		http.Error(w, "Invalid override ID", http.StatusBadRequest)
		return
	}

	if err := h.queries.DeleteProductVATOverride(ctx, overrideID); err != nil {
		h.logger.Error("failed to delete product VAT override", "error", err, "override_id", overrideID)
		http.Error(w, "Failed to delete override", http.StatusInternalServerError)
		return
	}

	h.logger.Info("product VAT override deleted", "override_id", overrideID)

	// Return empty response — HTMX outerHTML swap removes the row.
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

// toVATCategoryItems converts database VAT categories to template items.
func toVATCategoryItems(cats []db.VatCategory) []admin.VATCategoryItem {
	items := make([]admin.VATCategoryItem, 0, len(cats))
	for _, cat := range cats {
		items = append(items, admin.VATCategoryItem{
			Name:        cat.Name,
			DisplayName: cat.DisplayName,
		})
	}
	return items
}

// toOverrideItems converts database override rows to template items.
func toOverrideItems(rows []db.ListProductVATOverridesRow) []admin.ProductVATOverrideItem {
	items := make([]admin.ProductVATOverrideItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, admin.ProductVATOverrideItem{
			ID:                  row.ID.String(),
			CountryCode:         row.CountryCode,
			CountryName:         row.CountryName,
			CategoryName:        row.CategoryName,
			CategoryDisplayName: row.CategoryDisplayName,
			Notes:               derefString(row.Notes),
		})
	}
	return items
}

// toVATCountryItems converts database EU countries to template items.
func toVATCountryItems(countries []db.EuCountry) []admin.VATCountryItem {
	items := make([]admin.VATCountryItem, 0, len(countries))
	for _, c := range countries {
		items = append(items, admin.VATCountryItem{
			Code: c.CountryCode,
			Name: c.Name,
		})
	}
	return items
}

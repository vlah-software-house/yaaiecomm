package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/attribute"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/variant"
	"github.com/forgecommerce/api/templates/admin"
)

// VariantHandler handles admin product variant endpoints.
type VariantHandler struct {
	variants   *variant.Service
	attributes *attribute.Service
	products   *product.Service
	logger     *slog.Logger
}

// NewVariantHandler creates a new variant handler.
func NewVariantHandler(variants *variant.Service, attributes *attribute.Service, products *product.Service, logger *slog.Logger) *VariantHandler {
	return &VariantHandler{
		variants:   variants,
		attributes: attributes,
		products:   products,
		logger:     logger,
	}
}

// RegisterRoutes registers product variant admin routes on the given mux.
func (h *VariantHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products/{id}/variants", h.ShowVariants)
	mux.HandleFunc("POST /admin/products/{id}/variants/generate", h.GenerateVariants)
	mux.HandleFunc("GET /admin/products/{id}/variants/{variantId}", h.ShowEditVariant)
	mux.HandleFunc("POST /admin/products/{id}/variants/{variantId}", h.UpdateVariant)
	mux.HandleFunc("POST /admin/products/{id}/variants/{variantId}/delete", h.DeleteVariant)
}

// ShowVariants handles GET /admin/products/{id}/variants.
func (h *VariantHandler) ShowVariants(w http.ResponseWriter, r *http.Request) {
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

	attrs, err := h.attributes.ListAttributes(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list attributes", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	variants, err := h.variants.List(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list variants", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	variantItems := make([]admin.ProductVariantItem, 0, len(variants))
	for _, v := range variants {
		opts, err := h.variants.ListOptions(ctx, v.ID)
		if err != nil {
			h.logger.Error("failed to list variant options", "error", err, "variant_id", v.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		optionStr := buildOptionString(opts)

		variantItems = append(variantItems, admin.ProductVariantItem{
			ID:                v.ID.String(),
			SKU:               v.Sku,
			Options:           optionStr,
			Price:             formatNumeric(v.Price),
			CompareAtPrice:    formatNumeric(v.CompareAtPrice),
			WeightGrams:       formatInt32Ptr(v.WeightGrams),
			StockQuantity:     int(v.StockQuantity),
			LowStockThreshold: int(v.LowStockThreshold),
			Barcode:           derefString(v.Barcode),
			IsActive:          v.IsActive,
			Position:          int(v.Position),
		})
	}

	data := admin.ProductVariantsData{
		ProductID:     productID.String(),
		ProductName:   p.Name,
		HasAttributes: len(attrs) > 0,
		Variants:      variantItems,
		CSRFToken:     csrfToken,
	}

	admin.ProductVariantsPage(data).Render(ctx, w)
}

// GenerateVariants handles POST /admin/products/{id}/variants/generate.
// Triggers Cartesian product variant generation and returns the full
// variants table body for HTMX innerHTML swap.
func (h *VariantHandler) GenerateVariants(w http.ResponseWriter, r *http.Request) {
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

	skuPrefix := ""
	if p.SkuPrefix != nil {
		skuPrefix = *p.SkuPrefix
	}
	if skuPrefix == "" {
		// Fallback: use first 3 chars of product name
		skuPrefix = strings.ToUpper(p.Name)
		if len(skuPrefix) > 3 {
			skuPrefix = skuPrefix[:3]
		}
	}

	_, err = h.variants.GenerateVariants(ctx, productID, skuPrefix)
	if err != nil {
		if errors.Is(err, variant.ErrNoAttributes) {
			http.Error(w, "No attributes or active options defined", http.StatusBadRequest)
			return
		}
		h.logger.Error("failed to generate variants", "error", err, "product_id", productID)
		http.Error(w, "Failed to generate variants", http.StatusInternalServerError)
		return
	}

	// Reload all variants to return the full table body.
	allVariants, err := h.variants.List(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list variants after generation", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	for _, v := range allVariants {
		opts, _ := h.variants.ListOptions(ctx, v.ID)
		optionStr := buildOptionString(opts)

		item := admin.ProductVariantItem{
			ID:                v.ID.String(),
			SKU:               v.Sku,
			Options:           optionStr,
			Price:             formatNumeric(v.Price),
			CompareAtPrice:    formatNumeric(v.CompareAtPrice),
			WeightGrams:       formatInt32Ptr(v.WeightGrams),
			StockQuantity:     int(v.StockQuantity),
			LowStockThreshold: int(v.LowStockThreshold),
			Barcode:           derefString(v.Barcode),
			IsActive:          v.IsActive,
			Position:          int(v.Position),
		}
		admin.ProductVariantRow(productID.String(), item, csrfToken).Render(ctx, w)
	}
}

// ShowEditVariant handles GET /admin/products/{id}/variants/{variantId}.
func (h *VariantHandler) ShowEditVariant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	variantID, err := uuid.Parse(r.PathValue("variantId"))
	if err != nil {
		http.Error(w, "Invalid variant ID", http.StatusBadRequest)
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

	v, err := h.variants.Get(ctx, variantID)
	if err != nil {
		if errors.Is(err, variant.ErrNotFound) {
			http.Error(w, "Variant not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get variant", "error", err, "variant_id", variantID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	opts, _ := h.variants.ListOptions(ctx, variantID)
	optionStr := buildOptionString(opts)

	data := admin.ProductVariantEditData{
		ProductID:         productID.String(),
		ProductName:       p.Name,
		VariantID:         variantID.String(),
		SKU:               v.Sku,
		Options:           optionStr,
		Price:             formatNumeric(v.Price),
		CompareAtPrice:    formatNumeric(v.CompareAtPrice),
		WeightGrams:       formatInt32Ptr(v.WeightGrams),
		StockQuantity:     fmt.Sprintf("%d", v.StockQuantity),
		LowStockThreshold: fmt.Sprintf("%d", v.LowStockThreshold),
		Barcode:           derefString(v.Barcode),
		IsActive:          v.IsActive,
		CSRFToken:         csrfToken,
	}

	admin.ProductVariantEditPage(data).Render(ctx, w)
}

// UpdateVariant handles POST /admin/products/{id}/variants/{variantId}.
func (h *VariantHandler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	variantID, err := uuid.Parse(r.PathValue("variantId"))
	if err != nil {
		http.Error(w, "Invalid variant ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get the current variant to preserve its SKU.
	existing, err := h.variants.Get(ctx, variantID)
	if err != nil {
		if errors.Is(err, variant.ErrNotFound) {
			http.Error(w, "Variant not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get variant", "error", err, "variant_id", variantID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	price := parseNumeric(r.FormValue("price"))
	compareAtPrice := parseNumeric(r.FormValue("compare_at_price"))

	params := variant.UpdateVariantParams{
		Sku:               existing.Sku, // SKU is readonly in form
		Price:             price,
		CompareAtPrice:    compareAtPrice,
		WeightGrams:       parseInt32Ptr(r.FormValue("weight_grams")),
		StockQuantity:     parseInt32(r.FormValue("stock_quantity")),
		LowStockThreshold: parseInt32(r.FormValue("low_stock_threshold")),
		Barcode:           strPtr(r.FormValue("barcode")),
		IsActive:          r.FormValue("is_active") == "true",
		Position:          existing.Position,
	}

	_, err = h.variants.Update(ctx, variantID, params)
	if err != nil {
		h.logger.Error("failed to update variant", "error", err, "variant_id", variantID)
		p, _ := h.products.Get(ctx, productID)
		productName := ""
		if p.ID != uuid.Nil {
			productName = p.Name
		}
		opts, _ := h.variants.ListOptions(ctx, variantID)
		data := admin.ProductVariantEditData{
			ProductID:         productID.String(),
			ProductName:       productName,
			VariantID:         variantID.String(),
			SKU:               existing.Sku,
			Options:           buildOptionString(opts),
			Price:             r.FormValue("price"),
			CompareAtPrice:    r.FormValue("compare_at_price"),
			WeightGrams:       r.FormValue("weight_grams"),
			StockQuantity:     r.FormValue("stock_quantity"),
			LowStockThreshold: r.FormValue("low_stock_threshold"),
			Barcode:           r.FormValue("barcode"),
			IsActive:          r.FormValue("is_active") == "true",
			CSRFToken:         csrfToken,
			Error:             "Failed to update variant.",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.ProductVariantEditPage(data).Render(ctx, w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/products/%s/variants", productID), http.StatusSeeOther)
}

// DeleteVariant handles POST /admin/products/{id}/variants/{variantId}/delete.
// Returns empty response so HTMX outerHTML swap removes the row.
func (h *VariantHandler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	variantID, err := uuid.Parse(r.PathValue("variantId"))
	if err != nil {
		http.Error(w, "Invalid variant ID", http.StatusBadRequest)
		return
	}

	if err := h.variants.Delete(ctx, variantID); err != nil {
		if errors.Is(err, variant.ErrNotFound) {
			http.Error(w, "Variant not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete variant", "error", err, "variant_id", variantID)
		http.Error(w, "Failed to delete variant", http.StatusInternalServerError)
		return
	}

	h.logger.Info("variant deleted via admin", "variant_id", variantID)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

// buildOptionString creates a human-readable "Color / Size" style string
// from a variant's linked options.
func buildOptionString(opts []db.ListVariantOptionsRow) string {
	if len(opts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(opts))
	for _, o := range opts {
		display := o.OptionDisplayValue
		if display == "" {
			display = o.OptionValue
		}
		parts = append(parts, display)
	}
	return strings.Join(parts, " / ")
}

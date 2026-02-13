package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/bom"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/rawmaterial"
	"github.com/forgecommerce/api/internal/services/variant"
	"github.com/forgecommerce/api/templates/admin"
)

// BOMHandler handles admin product BOM (Bill of Materials) endpoints.
type BOMHandler struct {
	bom        *bom.Service
	products   *product.Service
	materials  *rawmaterial.Service
	variants   *variant.Service
	logger     *slog.Logger
}

// NewBOMHandler creates a new BOM handler.
func NewBOMHandler(bomSvc *bom.Service, products *product.Service, materials *rawmaterial.Service, variants *variant.Service, logger *slog.Logger) *BOMHandler {
	return &BOMHandler{
		bom:       bomSvc,
		products:  products,
		materials: materials,
		variants:  variants,
		logger:    logger,
	}
}

// RegisterRoutes registers BOM admin routes on the given mux.
func (h *BOMHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products/{id}/bom", h.ShowBOM)
	mux.HandleFunc("POST /admin/products/{id}/bom/entries", h.AddEntry)
	mux.HandleFunc("POST /admin/products/{id}/bom/entries/{entryId}/delete", h.DeleteEntry)
	mux.HandleFunc("POST /admin/products/{id}/bom/overrides", h.AddOverride)
	mux.HandleFunc("POST /admin/products/{id}/bom/overrides/{overrideId}/delete", h.DeleteOverride)
}

// ShowBOM handles GET /admin/products/{id}/bom.
func (h *BOMHandler) ShowBOM(w http.ResponseWriter, r *http.Request) {
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

	// Load BOM entries (Layer 1).
	entries, err := h.bom.ListProductEntries(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list BOM entries", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load all variants for the override dropdown.
	productVariants, err := h.variants.List(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list variants", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load variant overrides (Layer 3) — aggregate across all variants.
	var overrideItems []admin.VariantBOMOverrideItem
	variantSKUMap := make(map[string]string) // variantID -> SKU
	for _, v := range productVariants {
		variantSKUMap[v.ID.String()] = v.Sku
		overrides, err := h.bom.ListVariantOverrides(ctx, v.ID)
		if err != nil {
			h.logger.Error("failed to list variant overrides", "error", err, "variant_id", v.ID)
			continue
		}
		for _, o := range overrides {
			overrideItems = append(overrideItems, admin.VariantBOMOverrideItem{
				ID:                   o.ID.String(),
				VariantID:            o.VariantID.String(),
				VariantSKU:           v.Sku,
				MaterialName:         o.MaterialName,
				MaterialID:           o.RawMaterialID.String(),
				OverrideType:         o.OverrideType,
				ReplacesMaterialName: derefString(o.ReplacesMaterialName),
				ReplacesMaterialID:   formatPgUUID(o.ReplacesMaterialID),
				Quantity:             formatNumeric(o.Quantity),
				UnitOfMeasure:        derefString(o.UnitOfMeasure),
				Notes:                derefString(o.Notes),
			})
		}
	}

	// Load raw materials for dropdowns.
	materials, _, err := h.materials.List(ctx, nil, nil, 1, 1000)
	if err != nil {
		h.logger.Error("failed to list raw materials", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build template data.
	entryItems := make([]admin.ProductBOMEntryItem, 0, len(entries))
	for _, e := range entries {
		entryItems = append(entryItems, admin.ProductBOMEntryItem{
			ID:            e.ID.String(),
			MaterialName:  e.MaterialName,
			MaterialSKU:   e.MaterialSku,
			MaterialID:    e.RawMaterialID.String(),
			Quantity:      formatNumeric(e.Quantity),
			UnitOfMeasure: e.UnitOfMeasure,
			IsRequired:    e.IsRequired,
			Notes:         derefString(e.Notes),
		})
	}

	materialItems := make([]admin.BOMRawMaterialItem, 0, len(materials))
	for _, m := range materials {
		materialItems = append(materialItems, admin.BOMRawMaterialItem{
			ID:   m.ID.String(),
			Name: m.Name,
			SKU:  m.Sku,
			Unit: m.UnitOfMeasure,
		})
	}

	variantItems := make([]admin.BOMVariantItem, 0, len(productVariants))
	for _, v := range productVariants {
		variantItems = append(variantItems, admin.BOMVariantItem{
			ID:  v.ID.String(),
			SKU: v.Sku,
		})
	}

	data := admin.ProductBOMData{
		ProductID:   productID.String(),
		ProductName: p.Name,
		Entries:     entryItems,
		Overrides:   overrideItems,
		Materials:   materialItems,
		Variants:    variantItems,
		CSRFToken:   csrfToken,
	}

	admin.ProductBOMPage(data).Render(ctx, w)
}

// AddEntry handles POST /admin/products/{id}/bom/entries.
// Returns an HTMX fragment (table row) for appending.
func (h *BOMHandler) AddEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	materialID, err := uuid.Parse(r.FormValue("raw_material_id"))
	if err != nil {
		http.Error(w, "Invalid material ID", http.StatusBadRequest)
		return
	}

	quantity := parseNumeric(r.FormValue("quantity"))
	if !quantity.Valid {
		http.Error(w, "Quantity is required", http.StatusBadRequest)
		return
	}

	uom := strings.TrimSpace(r.FormValue("unit_of_measure"))
	if uom == "" {
		uom = "piece"
	}

	entry, err := h.bom.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     productID,
		RawMaterialID: materialID,
		Quantity:      quantity,
		UnitOfMeasure: uom,
		IsRequired:    r.FormValue("is_required") == "true",
		Notes:         strPtr(r.FormValue("notes")),
	})
	if err != nil {
		h.logger.Error("failed to create BOM entry", "error", err, "product_id", productID)
		http.Error(w, "Failed to add material", http.StatusInternalServerError)
		return
	}

	// Look up material name/SKU for the response row.
	material, err := h.materials.Get(ctx, materialID)
	if err != nil {
		h.logger.Error("failed to get material for response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	item := admin.ProductBOMEntryItem{
		ID:            entry.ID.String(),
		MaterialName:  material.Name,
		MaterialSKU:   material.Sku,
		MaterialID:    materialID.String(),
		Quantity:      formatNumeric(entry.Quantity),
		UnitOfMeasure: entry.UnitOfMeasure,
		IsRequired:    entry.IsRequired,
		Notes:         derefString(entry.Notes),
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ProductBOMEntryRow(productID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteEntry handles POST /admin/products/{id}/bom/entries/{entryId}/delete.
func (h *BOMHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entryID, err := uuid.Parse(r.PathValue("entryId"))
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	if err := h.bom.DeleteProductEntry(ctx, entryID); err != nil {
		h.logger.Error("failed to delete BOM entry", "error", err, "entry_id", entryID)
		http.Error(w, "Failed to delete entry", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// AddOverride handles POST /admin/products/{id}/bom/overrides.
// Returns an HTMX fragment (table row) for appending.
func (h *BOMHandler) AddOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	variantID, err := uuid.Parse(r.FormValue("variant_id"))
	if err != nil {
		http.Error(w, "Invalid variant ID", http.StatusBadRequest)
		return
	}

	materialID, err := uuid.Parse(r.FormValue("raw_material_id"))
	if err != nil {
		http.Error(w, "Invalid material ID", http.StatusBadRequest)
		return
	}

	overrideType := strings.TrimSpace(r.FormValue("override_type"))
	if overrideType == "" {
		http.Error(w, "Override type is required", http.StatusBadRequest)
		return
	}

	var replacesMaterialID pgtype.UUID
	if rmid := strings.TrimSpace(r.FormValue("replaces_material_id")); rmid != "" {
		parsed, err := uuid.Parse(rmid)
		if err != nil {
			http.Error(w, "Invalid replaces material ID", http.StatusBadRequest)
			return
		}
		replacesMaterialID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	override, err := h.bom.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:          variantID,
		RawMaterialID:      materialID,
		OverrideType:       overrideType,
		ReplacesMaterialID: replacesMaterialID,
		Quantity:           parseNumeric(r.FormValue("quantity")),
		UnitOfMeasure:      strPtr(r.FormValue("unit_of_measure")),
		Notes:              strPtr(r.FormValue("notes")),
	})
	if err != nil {
		h.logger.Error("failed to create BOM override", "error", err, "variant_id", variantID)
		http.Error(w, "Failed to add override", http.StatusInternalServerError)
		return
	}

	// Look up variant SKU and material name.
	v, _ := h.variants.Get(ctx, variantID)
	material, _ := h.materials.Get(ctx, materialID)

	replacesMaterialName := ""
	if replacesMaterialID.Valid {
		rmID, _ := uuid.FromBytes(replacesMaterialID.Bytes[:])
		if rm, err := h.materials.Get(ctx, rmID); err == nil {
			replacesMaterialName = rm.Name
		}
	}

	item := admin.VariantBOMOverrideItem{
		ID:                   override.ID.String(),
		VariantID:            variantID.String(),
		VariantSKU:           v.Sku,
		MaterialName:         material.Name,
		MaterialID:           materialID.String(),
		OverrideType:         override.OverrideType,
		ReplacesMaterialName: replacesMaterialName,
		ReplacesMaterialID:   formatPgUUID(replacesMaterialID),
		Quantity:             formatNumeric(override.Quantity),
		UnitOfMeasure:        derefString(override.UnitOfMeasure),
		Notes:                derefString(override.Notes),
	}

	w.Header().Set("Content-Type", "text/html")
	admin.VariantBOMOverrideRow(productID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteOverride handles POST /admin/products/{id}/bom/overrides/{overrideId}/delete.
func (h *BOMHandler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overrideID, err := uuid.Parse(r.PathValue("overrideId"))
	if err != nil {
		http.Error(w, "Invalid override ID", http.StatusBadRequest)
		return
	}

	if err := h.bom.DeleteVariantOverride(ctx, overrideID); err != nil {
		h.logger.Error("failed to delete BOM override", "error", err, "override_id", overrideID)
		http.Error(w, "Failed to delete override", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

// formatPgUUID formats a pgtype.UUID as a string, returning "" if invalid.
func formatPgUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	id, err := uuid.FromBytes(u.Bytes[:])
	if err != nil {
		return ""
	}
	return id.String()
}

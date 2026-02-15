package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/rawmaterial"
	"github.com/forgecommerce/api/templates/admin"
	"github.com/google/uuid"
)

const rawMaterialPageSize = 25

// RawMaterialHandler provides admin CRUD endpoints for raw materials.
type RawMaterialHandler struct {
	materials *rawmaterial.Service
	logger    *slog.Logger
}

// NewRawMaterialHandler creates a new RawMaterialHandler.
func NewRawMaterialHandler(materials *rawmaterial.Service, logger *slog.Logger) *RawMaterialHandler {
	return &RawMaterialHandler{
		materials: materials,
		logger:    logger,
	}
}

// RegisterRoutes registers all raw material admin routes on the given mux.
func (h *RawMaterialHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/inventory/raw-materials", h.ListRawMaterials)
	mux.HandleFunc("GET /admin/inventory/raw-materials/new", h.ShowNewRawMaterial)
	mux.HandleFunc("POST /admin/inventory/raw-materials", h.CreateRawMaterial)
	mux.HandleFunc("GET /admin/inventory/raw-materials/{id}", h.ShowEditRawMaterial)
	mux.HandleFunc("POST /admin/inventory/raw-materials/{id}", h.UpdateRawMaterial)
	mux.HandleFunc("POST /admin/inventory/raw-materials/{id}/delete", h.DeleteRawMaterial)
}

// ListRawMaterials handles GET /admin/inventory/raw-materials.
// It renders a paginated list of raw materials.
func (h *RawMaterialHandler) ListRawMaterials(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	// Show all materials by default (no category or active-only filter).
	materials, total, err := h.materials.List(r.Context(), nil, nil, page, rawMaterialPageSize)
	if err != nil {
		h.logger.Error("failed to list raw materials", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build a category map for resolving category names.
	categories, err := h.materials.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("failed to list categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	categoryMap := make(map[uuid.UUID]string, len(categories))
	for _, c := range categories {
		categoryMap[c.ID] = c.Name
	}

	// Convert db models to template list items.
	items := make([]admin.RawMaterialListItem, 0, len(materials))
	for _, m := range materials {
		categoryName := ""
		if m.CategoryID.Valid {
			categoryName = categoryMap[m.CategoryID.Bytes]
		}

		items = append(items, admin.RawMaterialListItem{
			ID:          m.ID.String(),
			Name:        m.Name,
			SKU:         m.Sku,
			Category:    categoryName,
			Stock:       formatNumeric(m.StockQuantity),
			Unit:        m.UnitOfMeasure,
			CostPerUnit: formatNumeric(m.CostPerUnit),
			IsActive:    m.IsActive,
			IsLowStock:  false, // Proper decimal comparison requires shopspring/decimal
		})
	}

	totalPages := int(total) / rawMaterialPageSize
	if int(total)%rawMaterialPageSize > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	data := admin.RawMaterialListData{
		Materials:  items,
		Page:       page,
		TotalPages: totalPages,
		Total:      int(total),
	}

	admin.RawMaterialListPage(data).Render(r.Context(), w)
}

// ShowNewRawMaterial handles GET /admin/inventory/raw-materials/new.
// It renders an empty form for creating a new raw material.
func (h *RawMaterialHandler) ShowNewRawMaterial(w http.ResponseWriter, r *http.Request) {
	categories, err := h.materials.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("failed to list categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := admin.RawMaterialFormData{
		IsNew:      true,
		IsActive:   true,
		CSRFToken:  middleware.CSRFToken(r),
		Categories: toTemplateCategories(categories),
	}

	admin.RawMaterialFormPage(data).Render(r.Context(), w)
}

// CreateRawMaterial handles POST /admin/inventory/raw-materials.
// It parses the form, creates the material, and redirects to the edit page.
func (h *RawMaterialHandler) CreateRawMaterial(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	params := rawmaterial.CreateRawMaterialParams{
		Name:              r.FormValue("name"),
		Sku:               r.FormValue("sku"),
		Description:       strPtr(r.FormValue("description")),
		CategoryID:        parseUUIDPtr(r.FormValue("category_id")),
		UnitOfMeasure:     r.FormValue("unit_of_measure"),
		CostPerUnit:       parseNumeric(r.FormValue("cost_per_unit")),
		StockQuantity:     parseNumeric(r.FormValue("stock_quantity")),
		LowStockThreshold: parseNumeric(r.FormValue("low_stock_threshold")),
		SupplierName:      strPtr(r.FormValue("supplier_name")),
		SupplierSku:       strPtr(r.FormValue("supplier_sku")),
		LeadTimeDays:      parseInt32Ptr(r.FormValue("lead_time_days")),
		IsActive:          r.FormValue("is_active") != "",
	}

	material, err := h.materials.Create(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to create raw material", "error", err)
		h.renderFormWithError(w, r, admin.RawMaterialFormData{
			IsNew:             true,
			Name:              params.Name,
			SKU:               params.Sku,
			Description:       derefString(params.Description),
			CategoryID:        derefUUID(params.CategoryID),
			UnitOfMeasure:     params.UnitOfMeasure,
			CostPerUnit:       formatNumeric(params.CostPerUnit),
			StockQuantity:     formatNumeric(params.StockQuantity),
			LowStockThreshold: formatNumeric(params.LowStockThreshold),
			SupplierName:      derefString(params.SupplierName),
			SupplierSku:       derefString(params.SupplierSku),
			LeadTimeDays:      formatInt32Ptr(params.LeadTimeDays),
			IsActive:          params.IsActive,
			Error:             fmt.Sprintf("Failed to create material: %v", err),
		})
		return
	}

	http.Redirect(w, r, "/admin/inventory/raw-materials/"+material.ID.String(), http.StatusSeeOther)
}

// ShowEditRawMaterial handles GET /admin/inventory/raw-materials/{id}.
// It loads the material and renders the edit form.
func (h *RawMaterialHandler) ShowEditRawMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	material, err := h.materials.Get(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get raw material", "error", err, "id", id.String())
		http.NotFound(w, r)
		return
	}

	categories, err := h.materials.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("failed to list categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	categoryID := ""
	if material.CategoryID.Valid {
		categoryID = uuid.UUID(material.CategoryID.Bytes).String()
	}

	data := admin.RawMaterialFormData{
		ID:                material.ID.String(),
		Name:              material.Name,
		SKU:               material.Sku,
		Description:       derefString(material.Description),
		CategoryID:        categoryID,
		UnitOfMeasure:     material.UnitOfMeasure,
		CostPerUnit:       formatNumeric(material.CostPerUnit),
		StockQuantity:     formatNumeric(material.StockQuantity),
		LowStockThreshold: formatNumeric(material.LowStockThreshold),
		SupplierName:      derefString(material.SupplierName),
		SupplierSku:       derefString(material.SupplierSku),
		LeadTimeDays:      formatInt32Ptr(material.LeadTimeDays),
		IsActive:          material.IsActive,
		IsNew:             false,
		CSRFToken:         middleware.CSRFToken(r),
		Categories:        toTemplateCategories(categories),
	}

	admin.RawMaterialFormPage(data).Render(r.Context(), w)
}

// UpdateRawMaterial handles POST /admin/inventory/raw-materials/{id}.
// It parses the form, updates the material, and re-renders the edit form.
func (h *RawMaterialHandler) UpdateRawMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	params := rawmaterial.UpdateRawMaterialParams{
		Name:              r.FormValue("name"),
		Sku:               r.FormValue("sku"),
		Description:       strPtr(r.FormValue("description")),
		CategoryID:        parseUUIDPtr(r.FormValue("category_id")),
		UnitOfMeasure:     r.FormValue("unit_of_measure"),
		CostPerUnit:       parseNumeric(r.FormValue("cost_per_unit")),
		StockQuantity:     parseNumeric(r.FormValue("stock_quantity")),
		LowStockThreshold: parseNumeric(r.FormValue("low_stock_threshold")),
		SupplierName:      strPtr(r.FormValue("supplier_name")),
		SupplierSku:       strPtr(r.FormValue("supplier_sku")),
		LeadTimeDays:      parseInt32Ptr(r.FormValue("lead_time_days")),
		IsActive:          r.FormValue("is_active") != "",
	}

	material, err := h.materials.Update(r.Context(), id, params)
	if err != nil {
		h.logger.Error("failed to update raw material", "error", err, "id", id.String())
		h.renderFormWithError(w, r, admin.RawMaterialFormData{
			ID:                id.String(),
			IsNew:             false,
			Name:              params.Name,
			SKU:               params.Sku,
			Description:       derefString(params.Description),
			CategoryID:        derefUUID(params.CategoryID),
			UnitOfMeasure:     params.UnitOfMeasure,
			CostPerUnit:       formatNumeric(params.CostPerUnit),
			StockQuantity:     formatNumeric(params.StockQuantity),
			LowStockThreshold: formatNumeric(params.LowStockThreshold),
			SupplierName:      derefString(params.SupplierName),
			SupplierSku:       derefString(params.SupplierSku),
			LeadTimeDays:      formatInt32Ptr(params.LeadTimeDays),
			IsActive:          params.IsActive,
			Error:             fmt.Sprintf("Failed to update material: %v", err),
		})
		return
	}

	// Redirect to the edit page to show updated state.
	http.Redirect(w, r, "/admin/inventory/raw-materials/"+material.ID.String(), http.StatusSeeOther)
}

// DeleteRawMaterial handles POST /admin/inventory/raw-materials/{id}/delete.
// It deletes the material and redirects to the list page.
func (h *RawMaterialHandler) DeleteRawMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.materials.Delete(r.Context(), id); err != nil {
		h.logger.Error("failed to delete raw material", "error", err, "id", id.String())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/inventory/raw-materials", http.StatusSeeOther)
}

// --- Helper methods ---

// renderFormWithError loads categories, attaches the CSRF token and error, then renders the form.
func (h *RawMaterialHandler) renderFormWithError(w http.ResponseWriter, r *http.Request, data admin.RawMaterialFormData) {
	categories, err := h.materials.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("failed to list categories for error form", "error", err)
	}
	data.Categories = toTemplateCategories(categories)
	data.CSRFToken = middleware.CSRFToken(r)

	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.RawMaterialFormPage(data).Render(r.Context(), w)
}

// --- Conversion helpers ---

// toTemplateCategories converts db categories to the template RawMaterialCategory type.
func toTemplateCategories(categories []db.RawMaterialCategory) []admin.RawMaterialCategory {
	result := make([]admin.RawMaterialCategory, 0, len(categories))
	for _, c := range categories {
		result = append(result, admin.RawMaterialCategory{
			ID:   c.ID.String(),
			Name: c.Name,
		})
	}
	return result
}

// parseUUIDPtr parses a string into a *uuid.UUID.
// Returns nil if the string is empty or not a valid UUID.
func parseUUIDPtr(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

// parseInt32Ptr parses a string into a *int32.
// Returns nil if the string is empty or not a valid integer.
func parseInt32Ptr(s string) *int32 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return nil
	}
	i := int32(v)
	return &i
}

// derefUUID returns the string representation of a *uuid.UUID, or "" if nil.
func derefUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// formatInt32Ptr returns the string representation of a *int32, or "" if nil.
func formatInt32Ptr(v *int32) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(int64(*v), 10)
}

package admin

import (
	"context"
	"encoding/json"
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
	"github.com/forgecommerce/api/internal/services/globalattr"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/templates/admin"
)

// GlobalAttributeHandler handles admin routes for global attribute management
// and product-global-attribute link management.
type GlobalAttributeHandler struct {
	globalAttrs *globalattr.Service
	products    *product.Service
	logger      *slog.Logger
}

// NewGlobalAttributeHandler creates a new global attribute handler.
func NewGlobalAttributeHandler(globalAttrs *globalattr.Service, products *product.Service, logger *slog.Logger) *GlobalAttributeHandler {
	return &GlobalAttributeHandler{
		globalAttrs: globalAttrs,
		products:    products,
		logger:      logger,
	}
}

// RegisterRoutes registers global attribute admin routes on the given mux.
func (h *GlobalAttributeHandler) RegisterRoutes(mux *http.ServeMux) {
	// Global Attribute CRUD
	mux.HandleFunc("GET /admin/global-attributes", h.List)
	mux.HandleFunc("GET /admin/global-attributes/new", h.NewForm)
	mux.HandleFunc("POST /admin/global-attributes", h.Create)
	mux.HandleFunc("GET /admin/global-attributes/{id}", h.Edit)
	mux.HandleFunc("POST /admin/global-attributes/{id}", h.Update)
	mux.HandleFunc("POST /admin/global-attributes/{id}/delete", h.Delete)

	// Metadata Fields
	mux.HandleFunc("POST /admin/global-attributes/{id}/fields", h.AddField)
	mux.HandleFunc("POST /admin/global-attributes/{id}/fields/{fieldId}/delete", h.DeleteField)

	// Options
	mux.HandleFunc("POST /admin/global-attributes/{id}/options", h.AddOption)
	mux.HandleFunc("POST /admin/global-attributes/{id}/options/{optId}/delete", h.DeleteOption)

	// Product Links
	mux.HandleFunc("GET /admin/products/{id}/global-attributes", h.ShowProductLinks)
	mux.HandleFunc("POST /admin/products/{id}/global-attributes", h.CreateProductLink)
	mux.HandleFunc("POST /admin/products/{id}/global-attributes/{linkId}/delete", h.DeleteProductLink)
	mux.HandleFunc("POST /admin/products/{id}/global-attributes/{linkId}/selections", h.SaveSelections)
}

// ---------------------------------------------------------------------------
// Part 1: Global Attribute Management (Settings > Global Attributes)
// ---------------------------------------------------------------------------

// List handles GET /admin/global-attributes.
func (h *GlobalAttributeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	attrs, err := h.globalAttrs.ListAll(ctx)
	if err != nil {
		h.logger.Error("failed to list global attributes", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	items := make([]admin.GlobalAttributeListItem, 0, len(attrs))
	for _, a := range attrs {
		options, _ := h.globalAttrs.ListOptions(ctx, a.ID)
		usageCount, _ := h.globalAttrs.CountUsage(ctx, a.ID)

		items = append(items, admin.GlobalAttributeListItem{
			ID:            a.ID.String(),
			Name:          a.Name,
			DisplayName:   a.DisplayName,
			Description:   derefString(a.Description),
			AttributeType: a.AttributeType,
			Category:      derefString(a.Category),
			OptionCount:   len(options),
			UsageCount:    usageCount,
			IsActive:      a.IsActive,
		})
	}

	data := admin.GlobalAttributeListData{
		Attributes: items,
		CSRFToken:  csrfToken,
	}

	admin.GlobalAttributesListPage(data).Render(ctx, w)
}

// NewForm handles GET /admin/global-attributes/new.
func (h *GlobalAttributeHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	data := admin.GlobalAttributeEditData{
		AttributeType: "select",
		IsActive:      true,
		IsNew:         true,
		CSRFToken:     csrfToken,
	}

	admin.GlobalAttributeEditPage(data).Render(r.Context(), w)
}

// Create handles POST /admin/global-attributes.
func (h *GlobalAttributeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		data := globalAttrFormFromRequest(r, csrfToken, true)
		data.Error = "Attribute name is required."
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.GlobalAttributeEditPage(data).Render(ctx, w)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		displayName = name
	}

	attr, err := h.globalAttrs.Create(ctx, globalattr.CreateParams{
		Name:          name,
		DisplayName:   displayName,
		Description:   strPtr(r.FormValue("description")),
		AttributeType: r.FormValue("attribute_type"),
		Category:      strPtr(r.FormValue("category")),
		IsActive:      r.FormValue("is_active") == "true",
	})
	if err != nil {
		h.logger.Error("failed to create global attribute", "error", err)
		data := globalAttrFormFromRequest(r, csrfToken, true)
		data.Error = "Failed to create attribute. " + err.Error()
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.GlobalAttributeEditPage(data).Render(ctx, w)
		return
	}

	http.Redirect(w, r, "/admin/global-attributes/"+attr.ID.String(), http.StatusSeeOther)
}

// Edit handles GET /admin/global-attributes/{id}.
func (h *GlobalAttributeHandler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	attr, err := h.globalAttrs.Get(ctx, id)
	if err != nil {
		if errors.Is(err, globalattr.ErrNotFound) {
			http.Error(w, "Global attribute not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get global attribute", "error", err, "id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	fields, err := h.globalAttrs.ListFields(ctx, id)
	if err != nil {
		h.logger.Error("failed to list metadata fields", "error", err, "attribute_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	fieldItems := make([]admin.MetadataFieldItem, 0, len(fields))
	for _, f := range fields {
		fieldItems = append(fieldItems, dbFieldToItem(f))
	}

	options, err := h.globalAttrs.ListOptions(ctx, id)
	if err != nil {
		h.logger.Error("failed to list options", "error", err, "attribute_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	optItems := make([]admin.GlobalOptionItem, 0, len(options))
	for _, o := range options {
		optItems = append(optItems, dbOptionToItem(o))
	}

	data := admin.GlobalAttributeEditData{
		ID:            attr.ID.String(),
		Name:          attr.Name,
		DisplayName:   attr.DisplayName,
		Description:   derefString(attr.Description),
		AttributeType: attr.AttributeType,
		Category:      derefString(attr.Category),
		IsActive:      attr.IsActive,
		IsNew:         false,
		Fields:        fieldItems,
		Options:       optItems,
		CSRFToken:     csrfToken,
	}

	admin.GlobalAttributeEditPage(data).Render(ctx, w)
}

// Update handles POST /admin/global-attributes/{id}.
func (h *GlobalAttributeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		data := globalAttrFormFromRequest(r, csrfToken, false)
		data.ID = id.String()
		data.Error = "Attribute name is required."
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.GlobalAttributeEditPage(data).Render(ctx, w)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		displayName = name
	}

	_, err = h.globalAttrs.Update(ctx, id, globalattr.UpdateParams{
		Name:          name,
		DisplayName:   displayName,
		Description:   strPtr(r.FormValue("description")),
		AttributeType: r.FormValue("attribute_type"),
		Category:      strPtr(r.FormValue("category")),
		IsActive:      r.FormValue("is_active") == "true",
	})
	if err != nil {
		h.logger.Error("failed to update global attribute", "error", err, "id", id)
		data := globalAttrFormFromRequest(r, csrfToken, false)
		data.ID = id.String()
		data.Error = "Failed to update attribute."
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.GlobalAttributeEditPage(data).Render(ctx, w)
		return
	}

	http.Redirect(w, r, "/admin/global-attributes/"+id.String(), http.StatusSeeOther)
}

// Delete handles POST /admin/global-attributes/{id}/delete.
func (h *GlobalAttributeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	if err := h.globalAttrs.Delete(r.Context(), id); err != nil {
		if errors.Is(err, globalattr.ErrNotFound) {
			http.Error(w, "Attribute not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, globalattr.ErrInUse) {
			http.Error(w, "Cannot delete attribute that is linked to products", http.StatusConflict)
			return
		}
		h.logger.Error("failed to delete global attribute", "error", err, "id", id)
		http.Error(w, "Failed to delete attribute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Metadata Fields
// ---------------------------------------------------------------------------

// AddField handles POST /admin/global-attributes/{id}/fields.
func (h *GlobalAttributeHandler) AddField(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	attrID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fieldName := strings.TrimSpace(r.FormValue("field_name"))
	if fieldName == "" {
		http.Error(w, "Field name is required", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		displayName = fieldName
	}

	existingFields, _ := h.globalAttrs.ListFields(ctx, attrID)
	position := int32(len(existingFields))

	field, err := h.globalAttrs.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID,
		FieldName:         fieldName,
		DisplayName:       displayName,
		FieldType:         r.FormValue("field_type"),
		IsRequired:        r.FormValue("is_required") == "true",
		DefaultValue:      strPtr(r.FormValue("default_value")),
		HelpText:          strPtr(r.FormValue("help_text")),
		Position:          position,
	})
	if err != nil {
		h.logger.Error("failed to create metadata field", "error", err, "attribute_id", attrID)
		http.Error(w, "Failed to create field", http.StatusInternalServerError)
		return
	}

	item := dbFieldToItem(field)

	w.Header().Set("Content-Type", "text/html")
	admin.MetadataFieldRow(attrID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteField handles POST /admin/global-attributes/{id}/fields/{fieldId}/delete.
func (h *GlobalAttributeHandler) DeleteField(w http.ResponseWriter, r *http.Request) {
	fieldID, err := uuid.Parse(r.PathValue("fieldId"))
	if err != nil {
		http.Error(w, "Invalid field ID", http.StatusBadRequest)
		return
	}

	if err := h.globalAttrs.DeleteField(r.Context(), fieldID); err != nil {
		if errors.Is(err, globalattr.ErrFieldNotFound) {
			http.Error(w, "Field not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete metadata field", "error", err, "field_id", fieldID)
		http.Error(w, "Failed to delete field", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

// AddOption handles POST /admin/global-attributes/{id}/options.
func (h *GlobalAttributeHandler) AddOption(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	attrID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	value := strings.TrimSpace(r.FormValue("value"))
	if value == "" {
		http.Error(w, "Option value is required", http.StatusBadRequest)
		return
	}

	displayValue := strings.TrimSpace(r.FormValue("display_value"))
	if displayValue == "" {
		displayValue = value
	}

	existingOptions, _ := h.globalAttrs.ListOptions(ctx, attrID)
	position := int32(len(existingOptions))

	fields, _ := h.globalAttrs.ListFields(ctx, attrID)
	metadataJSON := buildMetadataJSON(r, fields)

	opt, err := h.globalAttrs.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID,
		Value:             value,
		DisplayValue:      displayValue,
		ColorHex:          strPtr(r.FormValue("color_hex")),
		ImageURL:          strPtr(r.FormValue("image_url")),
		Position:          position,
		IsActive:          true,
		Metadata:          metadataJSON,
	})
	if err != nil {
		h.logger.Error("failed to create global option", "error", err, "attribute_id", attrID)
		http.Error(w, "Failed to create option", http.StatusInternalServerError)
		return
	}

	fieldItems := buildFieldItemsForTemplate(fields)
	item := dbOptionToItem(opt)

	w.Header().Set("Content-Type", "text/html")
	admin.GlobalOptionRow(attrID.String(), item, fieldItems, csrfToken).Render(ctx, w)
}

// DeleteOption handles POST /admin/global-attributes/{id}/options/{optId}/delete.
func (h *GlobalAttributeHandler) DeleteOption(w http.ResponseWriter, r *http.Request) {
	optID, err := uuid.Parse(r.PathValue("optId"))
	if err != nil {
		http.Error(w, "Invalid option ID", http.StatusBadRequest)
		return
	}

	if err := h.globalAttrs.DeleteOption(r.Context(), optID); err != nil {
		if errors.Is(err, globalattr.ErrOptionNotFound) {
			http.Error(w, "Option not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete global option", "error", err, "option_id", optID)
		http.Error(w, "Failed to delete option", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Part 2: Product Global Attribute Links (Products > {id} > Global Attributes)
// ---------------------------------------------------------------------------

// ShowProductLinks handles GET /admin/products/{id}/global-attributes.
func (h *GlobalAttributeHandler) ShowProductLinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	prod, err := h.products.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get product", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := h.buildProductLinksData(ctx, productID, prod.Name, csrfToken)
	admin.ProductGlobalLinksPage(data).Render(ctx, w)
}

// CreateProductLink handles POST /admin/products/{id}/global-attributes.
func (h *GlobalAttributeHandler) CreateProductLink(w http.ResponseWriter, r *http.Request) {
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

	globalAttrID, err := uuid.Parse(r.FormValue("global_attribute_id"))
	if err != nil {
		http.Error(w, "Invalid global attribute ID", http.StatusBadRequest)
		return
	}

	roleName := strings.TrimSpace(r.FormValue("role"))
	if roleName == "" {
		roleName = "variant_axis"
	}

	existingLinks, _ := h.globalAttrs.ListLinks(ctx, productID)
	position := int32(len(existingLinks))

	_, err = h.globalAttrs.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID:         productID,
		GlobalAttributeID: globalAttrID,
		RoleName:          roleName,
		RoleDisplayName:   roleName,
		Position:          position,
		AffectsPricing:    r.FormValue("affects_pricing") == "true",
		AffectsShipping:   r.FormValue("affects_shipping") == "true",
	})
	if err != nil {
		h.logger.Error("failed to create product global attribute link", "error", err,
			"product_id", productID, "global_attribute_id", globalAttrID)
		http.Error(w, "Failed to link attribute", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderLinksSection(w, r, productID, csrfToken)
		return
	}
	http.Redirect(w, r, "/admin/products/"+productID.String()+"/global-attributes", http.StatusSeeOther)
}

// DeleteProductLink handles POST /admin/products/{id}/global-attributes/{linkId}/delete.
func (h *GlobalAttributeHandler) DeleteProductLink(w http.ResponseWriter, r *http.Request) {
	linkID, err := uuid.Parse(r.PathValue("linkId"))
	if err != nil {
		http.Error(w, "Invalid link ID", http.StatusBadRequest)
		return
	}

	if err := h.globalAttrs.DeleteLink(r.Context(), linkID); err != nil {
		if errors.Is(err, globalattr.ErrLinkNotFound) {
			http.Error(w, "Link not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete product global attribute link", "error", err, "link_id", linkID)
		http.Error(w, "Failed to unlink attribute", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// SaveSelections handles POST /admin/products/{id}/global-attributes/{linkId}/selections.
func (h *GlobalAttributeHandler) SaveSelections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	linkID, err := uuid.Parse(r.PathValue("linkId"))
	if err != nil {
		http.Error(w, "Invalid link ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	optionIDStrs := r.Form["option_ids"]
	selections := make([]globalattr.SelectionInput, 0, len(optionIDStrs))

	for i, idStr := range optionIDStrs {
		optID, parseErr := uuid.Parse(idStr)
		if parseErr != nil {
			continue
		}

		var pm pgtype.Numeric
		pmStr := strings.TrimSpace(r.FormValue("price_modifier_" + idStr))
		if pmStr != "" {
			pm = parseNumeric(pmStr)
		}

		pos := int32(i)
		selections = append(selections, globalattr.SelectionInput{
			GlobalOptionID:   optID,
			PriceModifier:    pm,
			PositionOverride: &pos,
		})
	}

	if err := h.globalAttrs.SetSelections(ctx, linkID, selections); err != nil {
		h.logger.Error("failed to save option selections", "error", err, "link_id", linkID)
		http.Error(w, "Failed to save selections", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderLinkCard(w, r, productID, linkID, csrfToken)
		return
	}
	http.Redirect(w, r, "/admin/products/"+productID.String()+"/global-attributes", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Product link rendering helpers
// ---------------------------------------------------------------------------

func (h *GlobalAttributeHandler) renderLinksSection(w http.ResponseWriter, r *http.Request, productID uuid.UUID, csrfToken string) {
	ctx := r.Context()

	prod, err := h.products.Get(ctx, productID)
	if err != nil {
		h.logger.Error("failed to get product for links section", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := h.buildProductLinksData(ctx, productID, prod.Name, csrfToken)
	w.Header().Set("Content-Type", "text/html")
	admin.ProductGlobalLinksSection(data).Render(ctx, w)
}

func (h *GlobalAttributeHandler) renderLinkCard(w http.ResponseWriter, r *http.Request, productID, linkID uuid.UUID, csrfToken string) {
	ctx := r.Context()

	link, err := h.globalAttrs.GetLink(ctx, linkID)
	if err != nil {
		if errors.Is(err, globalattr.ErrLinkNotFound) {
			http.Error(w, "Link not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get product link for card", "error", err, "link_id", linkID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ga, err := h.globalAttrs.Get(ctx, link.GlobalAttributeID)
	if err != nil {
		h.logger.Error("failed to get global attribute for link card", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	linkItem := h.buildLinkItem(ctx, link, ga)

	w.Header().Set("Content-Type", "text/html")
	admin.ProductGlobalLinkCard(productID.String(), linkItem, csrfToken).Render(ctx, w)
}

func (h *GlobalAttributeHandler) buildProductLinksData(ctx context.Context, productID uuid.UUID, productName, csrfToken string) admin.ProductGlobalLinksData {
	links, _ := h.globalAttrs.ListLinks(ctx, productID)

	linkedAttrIDs := make(map[uuid.UUID]bool, len(links))
	linkItems := make([]admin.ProductGlobalLinkItem, 0, len(links))

	for _, link := range links {
		linkedAttrIDs[link.GlobalAttributeID] = true

		ga, err := h.globalAttrs.Get(ctx, link.GlobalAttributeID)
		if err != nil {
			h.logger.Error("failed to get global attribute for link", "error", err,
				"global_attribute_id", link.GlobalAttributeID)
			continue
		}

		linkItems = append(linkItems, h.buildLinkItem(ctx, link, ga))
	}

	allAttrs, _ := h.globalAttrs.ListAll(ctx)
	available := make([]admin.GlobalAttributeListItem, 0)
	for _, a := range allAttrs {
		if linkedAttrIDs[a.ID] || !a.IsActive {
			continue
		}
		opts, _ := h.globalAttrs.ListOptions(ctx, a.ID)
		available = append(available, admin.GlobalAttributeListItem{
			ID:            a.ID.String(),
			Name:          a.Name,
			DisplayName:   a.DisplayName,
			AttributeType: a.AttributeType,
			OptionCount:   len(opts),
			IsActive:      a.IsActive,
		})
	}

	return admin.ProductGlobalLinksData{
		ProductID:      productID.String(),
		ProductName:    productName,
		Links:          linkItems,
		AvailableAttrs: available,
		CSRFToken:      csrfToken,
	}
}

func (h *GlobalAttributeHandler) buildLinkItem(ctx context.Context, link db.ProductGlobalAttributeLink, ga db.GlobalAttribute) admin.ProductGlobalLinkItem {
	options, _ := h.globalAttrs.ListOptions(ctx, link.GlobalAttributeID)
	selections, _ := h.globalAttrs.ListSelections(ctx, link.ID)

	selectedMap := make(map[uuid.UUID]pgtype.Numeric, len(selections))
	for _, sel := range selections {
		selectedMap[sel.GlobalOptionID] = sel.PriceModifier
	}

	allOptions := make([]admin.GlobalOptionSelectionItem, 0, len(options))
	selectedCount := 0
	for _, opt := range options {
		pm, isSelected := selectedMap[opt.ID]
		if isSelected {
			selectedCount++
		}
		allOptions = append(allOptions, admin.GlobalOptionSelectionItem{
			OptionID:      opt.ID.String(),
			Value:         opt.Value,
			DisplayValue:  opt.DisplayValue,
			ColorHex:      derefString(opt.ColorHex),
			IsSelected:    isSelected,
			PriceModifier: formatNumeric(pm),
		})
	}

	return admin.ProductGlobalLinkItem{
		LinkID:               link.ID.String(),
		GlobalAttributeID:    link.GlobalAttributeID.String(),
		AttributeName:        ga.Name,
		AttributeDisplayName: ga.DisplayName,
		AttributeType:        ga.AttributeType,
		RoleName:             link.RoleName,
		RoleDisplayName:      link.RoleDisplayName,
		AffectsPricing:       link.AffectsPricing,
		AffectsShipping:      link.AffectsShipping,
		TotalOptions:         len(options),
		SelectedOptions:      selectedCount,
		AllOptions:           allOptions,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func globalAttrFormFromRequest(r *http.Request, csrfToken string, isNew bool) admin.GlobalAttributeEditData {
	return admin.GlobalAttributeEditData{
		Name:          r.FormValue("name"),
		DisplayName:   r.FormValue("display_name"),
		Description:   r.FormValue("description"),
		AttributeType: r.FormValue("attribute_type"),
		Category:      r.FormValue("category"),
		IsActive:      r.FormValue("is_active") == "true",
		IsNew:         isNew,
		CSRFToken:     csrfToken,
	}
}

func metadataToMap(raw json.RawMessage) map[string]string {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return map[string]string{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func buildMetadataJSON(r *http.Request, fields []db.GlobalAttributeMetadataField) json.RawMessage {
	meta := make(map[string]interface{})
	for _, f := range fields {
		val := strings.TrimSpace(r.FormValue("meta_" + f.FieldName))
		if val == "" {
			continue
		}
		switch f.FieldType {
		case "number":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				meta[f.FieldName] = n
			} else {
				meta[f.FieldName] = val
			}
		case "boolean":
			meta[f.FieldName] = val == "true"
		default:
			meta[f.FieldName] = val
		}
	}
	b, _ := json.Marshal(meta)
	return b
}

func buildFieldItemsForTemplate(fields []db.GlobalAttributeMetadataField) []admin.MetadataFieldItem {
	items := make([]admin.MetadataFieldItem, 0, len(fields))
	for _, f := range fields {
		items = append(items, admin.MetadataFieldItem{
			ID:            f.ID.String(),
			FieldName:     f.FieldName,
			DisplayName:   f.DisplayName,
			FieldType:     f.FieldType,
			IsRequired:    f.IsRequired,
			DefaultValue:  derefString(f.DefaultValue),
			SelectOptions: f.SelectOptions,
			HelpText:      derefString(f.HelpText),
			Position:      int(f.Position),
		})
	}
	return items
}

func dbFieldToItem(f db.GlobalAttributeMetadataField) admin.MetadataFieldItem {
	return admin.MetadataFieldItem{
		ID:            f.ID.String(),
		FieldName:     f.FieldName,
		DisplayName:   f.DisplayName,
		FieldType:     f.FieldType,
		IsRequired:    f.IsRequired,
		DefaultValue:  derefString(f.DefaultValue),
		SelectOptions: f.SelectOptions,
		HelpText:      derefString(f.HelpText),
		Position:      int(f.Position),
	}
}

func dbOptionToItem(opt db.GlobalAttributeOption) admin.GlobalOptionItem {
	return admin.GlobalOptionItem{
		ID:           opt.ID.String(),
		Value:        opt.Value,
		DisplayValue: opt.DisplayValue,
		ColorHex:     derefString(opt.ColorHex),
		ImageURL:     derefString(opt.ImageUrl),
		Metadata:     metadataToMap(opt.Metadata),
		Position:     int(opt.Position),
		IsActive:     opt.IsActive,
	}
}

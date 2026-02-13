package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/attribute"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/templates/admin"
)

// AttributeHandler handles admin product attribute CRUD endpoints.
type AttributeHandler struct {
	attributes *attribute.Service
	products   *product.Service
	logger     *slog.Logger
}

// NewAttributeHandler creates a new attribute handler.
func NewAttributeHandler(attributes *attribute.Service, products *product.Service, logger *slog.Logger) *AttributeHandler {
	return &AttributeHandler{
		attributes: attributes,
		products:   products,
		logger:     logger,
	}
}

// RegisterRoutes registers product attribute admin routes on the given mux.
func (h *AttributeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products/{id}/attributes", h.ShowAttributes)
	mux.HandleFunc("POST /admin/products/{id}/attributes", h.AddAttribute)
	mux.HandleFunc("POST /admin/products/{id}/attributes/{attrId}/delete", h.DeleteAttribute)
	mux.HandleFunc("POST /admin/products/{id}/attributes/{attrId}/options", h.AddOption)
	mux.HandleFunc("POST /admin/products/{id}/attributes/{attrId}/options/{optId}/delete", h.DeleteOption)
}

// ShowAttributes handles GET /admin/products/{id}/attributes.
func (h *AttributeHandler) ShowAttributes(w http.ResponseWriter, r *http.Request) {
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

	attrItems := make([]admin.ProductAttributeItem, 0, len(attrs))
	for _, a := range attrs {
		options, err := h.attributes.ListOptions(ctx, a.ID)
		if err != nil {
			h.logger.Error("failed to list options", "error", err, "attribute_id", a.ID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		optItems := make([]admin.ProductAttributeOptionItem, 0, len(options))
		for _, o := range options {
			optItems = append(optItems, admin.ProductAttributeOptionItem{
				ID:                  o.ID.String(),
				Value:               o.Value,
				DisplayValue:        o.DisplayValue,
				ColorHex:            derefString(o.ColorHex),
				PriceModifier:       formatNumeric(o.PriceModifier),
				WeightModifierGrams: formatInt32Ptr(o.WeightModifierGrams),
				Position:            int(o.Position),
				IsActive:            o.IsActive,
			})
		}

		attrItems = append(attrItems, admin.ProductAttributeItem{
			ID:              a.ID.String(),
			Name:            a.Name,
			DisplayName:     a.DisplayName,
			AttributeType:   a.AttributeType,
			Position:        int(a.Position),
			AffectsPricing:  a.AffectsPricing,
			AffectsShipping: a.AffectsShipping,
			Options:         optItems,
		})
	}

	data := admin.ProductAttributesData{
		ProductID:   productID.String(),
		ProductName: p.Name,
		Attributes:  attrItems,
		CSRFToken:   csrfToken,
	}

	admin.ProductAttributesPage(data).Render(ctx, w)
}

// AddAttribute handles POST /admin/products/{id}/attributes.
// Returns an HTMX fragment (attribute card) for appending.
func (h *AttributeHandler) AddAttribute(w http.ResponseWriter, r *http.Request) {
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

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Attribute name is required", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		displayName = name
	}

	// Count existing attributes to set position.
	existing, _ := h.attributes.ListAttributes(ctx, productID)
	position := int32(len(existing))

	attr, err := h.attributes.CreateAttribute(ctx, attribute.CreateAttributeParams{
		ProductID:       productID,
		Name:            name,
		DisplayName:     displayName,
		AttributeType:   r.FormValue("attribute_type"),
		Position:        position,
		AffectsPricing:  r.FormValue("affects_pricing") == "true",
		AffectsShipping: r.FormValue("affects_shipping") == "true",
	})
	if err != nil {
		h.logger.Error("failed to create attribute", "error", err, "product_id", productID)
		http.Error(w, "Failed to create attribute", http.StatusInternalServerError)
		return
	}

	item := admin.ProductAttributeItem{
		ID:              attr.ID.String(),
		Name:            attr.Name,
		DisplayName:     attr.DisplayName,
		AttributeType:   attr.AttributeType,
		Position:        int(attr.Position),
		AffectsPricing:  attr.AffectsPricing,
		AffectsShipping: attr.AffectsShipping,
		Options:         []admin.ProductAttributeOptionItem{},
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ProductAttributeCard(productID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteAttribute handles POST /admin/products/{id}/attributes/{attrId}/delete.
// Returns empty response so HTMX outerHTML swap removes the card.
func (h *AttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	attrID, err := uuid.Parse(r.PathValue("attrId"))
	if err != nil {
		http.Error(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	if err := h.attributes.DeleteAttribute(ctx, attrID); err != nil {
		if errors.Is(err, attribute.ErrNotFound) {
			http.Error(w, "Attribute not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete attribute", "error", err, "attribute_id", attrID)
		http.Error(w, "Failed to delete attribute", http.StatusInternalServerError)
		return
	}

	h.logger.Info("attribute deleted via admin", "attribute_id", attrID)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// AddOption handles POST /admin/products/{id}/attributes/{attrId}/options.
// Returns an HTMX fragment (option table row) for appending.
func (h *AttributeHandler) AddOption(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	attrID, err := uuid.Parse(r.PathValue("attrId"))
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

	// Count existing options to set position.
	existing, _ := h.attributes.ListOptions(ctx, attrID)
	position := int32(len(existing))

	opt, err := h.attributes.CreateOption(ctx, attribute.CreateOptionParams{
		AttributeID:         attrID,
		Value:               value,
		DisplayValue:        displayValue,
		ColorHex:            strPtr(r.FormValue("color_hex")),
		PriceModifier:       parseNumeric(r.FormValue("price_modifier")),
		WeightModifierGrams: parseInt32Ptr(r.FormValue("weight_modifier_grams")),
		Position:            position,
		IsActive:            true,
	})
	if err != nil {
		h.logger.Error("failed to create option", "error", err, "attribute_id", attrID)
		http.Error(w, "Failed to create option", http.StatusInternalServerError)
		return
	}

	item := admin.ProductAttributeOptionItem{
		ID:                  opt.ID.String(),
		Value:               opt.Value,
		DisplayValue:        opt.DisplayValue,
		ColorHex:            derefString(opt.ColorHex),
		PriceModifier:       formatNumeric(opt.PriceModifier),
		WeightModifierGrams: formatInt32Ptr(opt.WeightModifierGrams),
		Position:            int(opt.Position),
		IsActive:            opt.IsActive,
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ProductAttributeOptionRow(productID.String(), attrID.String(), item, csrfToken).Render(ctx, w)
}

// DeleteOption handles POST /admin/products/{id}/attributes/{attrId}/options/{optId}/delete.
// Returns empty response so HTMX outerHTML swap removes the row.
func (h *AttributeHandler) DeleteOption(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	optID, err := uuid.Parse(r.PathValue("optId"))
	if err != nil {
		http.Error(w, "Invalid option ID", http.StatusBadRequest)
		return
	}

	if err := h.attributes.DeleteOption(ctx, optID); err != nil {
		if errors.Is(err, attribute.ErrOptionNotFound) {
			http.Error(w, "Option not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete option", "error", err, "option_id", optID)
		http.Error(w, "Failed to delete option", http.StatusInternalServerError)
		return
	}

	h.logger.Info("attribute option deleted via admin", "option_id", optID)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}


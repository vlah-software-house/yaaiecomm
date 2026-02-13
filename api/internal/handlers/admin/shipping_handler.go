package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/shipping"
	"github.com/forgecommerce/api/templates/admin"
)

// ShippingHandler handles admin shipping configuration and zone management endpoints.
type ShippingHandler struct {
	shipping *shipping.Service
	logger   *slog.Logger
}

// NewShippingHandler creates a new shipping handler.
func NewShippingHandler(shippingSvc *shipping.Service, logger *slog.Logger) *ShippingHandler {
	return &ShippingHandler{
		shipping: shippingSvc,
		logger:   logger,
	}
}

// RegisterRoutes registers all shipping admin routes on the given mux.
func (h *ShippingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/settings/shipping", h.ShowShipping)
	mux.HandleFunc("POST /admin/settings/shipping", h.UpdateConfig)
	mux.HandleFunc("POST /admin/settings/shipping/zones", h.CreateZone)
	mux.HandleFunc("POST /admin/settings/shipping/zones/{id}/delete", h.DeleteZone)
}

// ShowShipping handles GET /admin/settings/shipping.
// It loads the global shipping configuration and all shipping zones,
// then renders the shipping settings page.
func (h *ShippingHandler) ShowShipping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	config, err := h.shipping.GetConfig(ctx)
	if err != nil {
		h.logger.Error("failed to load shipping config", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	zones, err := h.shipping.ListZones(ctx)
	if err != nil {
		h.logger.Error("failed to list shipping zones", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert zones to template items.
	zoneItems := make([]admin.ShippingZoneItem, 0, len(zones))
	for _, z := range zones {
		zoneItems = append(zoneItems, admin.ShippingZoneItem{
			ID:                z.ID.String(),
			Name:              z.Name,
			Countries:         strings.Join(z.Countries, ", "),
			CalculationMethod: z.CalculationMethod,
			Position:          int(z.Position),
		})
	}

	data := admin.ShippingSettingsData{
		CSRFToken: csrfToken,
		Config: admin.ShippingConfigItem{
			Enabled:               config.Enabled,
			CalculationMethod:     config.CalculationMethod,
			FixedFee:              formatNumeric(config.FixedFee),
			FreeShippingThreshold: formatNumeric(config.FreeShippingThreshold),
			DefaultCurrency:       config.DefaultCurrency,
		},
		Zones: zoneItems,
	}

	admin.ShippingSettingsPage(data).Render(ctx, w)
}

// UpdateConfig handles POST /admin/settings/shipping.
// It parses the form, updates the global shipping configuration, and redirects
// back to the shipping settings page.
func (h *ShippingHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse fixed fee.
	fixedFee := decimal.Zero
	if feeStr := strings.TrimSpace(r.FormValue("fixed_fee")); feeStr != "" {
		parsed, err := decimal.NewFromString(feeStr)
		if err != nil {
			h.showShippingWithError(w, r, "Invalid fixed fee value.")
			return
		}
		fixedFee = parsed
	}

	// Parse free shipping threshold.
	freeShippingThreshold := decimal.Zero
	if threshStr := strings.TrimSpace(r.FormValue("free_shipping_threshold")); threshStr != "" {
		parsed, err := decimal.NewFromString(threshStr)
		if err != nil {
			h.showShippingWithError(w, r, "Invalid free shipping threshold value.")
			return
		}
		freeShippingThreshold = parsed
	}

	params := shipping.UpdateConfigParams{
		Enabled:               r.FormValue("enabled") != "",
		CalculationMethod:     r.FormValue("calculation_method"),
		FixedFee:              fixedFee,
		FreeShippingThreshold: freeShippingThreshold,
		DefaultCurrency:       "EUR",
	}

	if _, err := h.shipping.UpdateConfig(ctx, params); err != nil {
		h.logger.Error("failed to update shipping config", "error", err)
		h.showShippingWithError(w, r, "Failed to save shipping configuration. Please try again.")
		return
	}

	h.logger.Info("shipping config updated",
		"enabled", params.Enabled,
		"method", params.CalculationMethod,
	)

	http.Redirect(w, r, "/admin/settings/shipping", http.StatusSeeOther)
}

// CreateZone handles POST /admin/settings/shipping/zones.
// This is an HTMX endpoint that creates a new shipping zone and returns
// an HTML table row fragment for the new zone.
func (h *ShippingHandler) CreateZone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Zone name is required", http.StatusUnprocessableEntity)
		return
	}

	// Parse comma-separated country codes into a slice.
	countriesStr := strings.TrimSpace(r.FormValue("countries"))
	if countriesStr == "" {
		http.Error(w, "At least one country code is required", http.StatusUnprocessableEntity)
		return
	}
	rawCountries := strings.Split(countriesStr, ",")
	countries := make([]string, 0, len(rawCountries))
	for _, c := range rawCountries {
		trimmed := strings.TrimSpace(strings.ToUpper(c))
		if trimmed != "" {
			countries = append(countries, trimmed)
		}
	}
	if len(countries) == 0 {
		http.Error(w, "At least one valid country code is required", http.StatusUnprocessableEntity)
		return
	}

	calcMethod := r.FormValue("calculation_method")
	position := parseInt32(r.FormValue("position"))

	// Build empty rates JSON — zones start with no specific rates configured.
	emptyRates, _ := json.Marshal(struct{}{})

	params := shipping.CreateZoneParams{
		Name:              name,
		Countries:         countries,
		CalculationMethod: calcMethod,
		Rates:             emptyRates,
		Position:          position,
	}

	zone, err := h.shipping.CreateZone(ctx, params)
	if err != nil {
		h.logger.Error("failed to create shipping zone", "error", err)
		http.Error(w, "Failed to create shipping zone", http.StatusInternalServerError)
		return
	}

	h.logger.Info("shipping zone created",
		"zone_id", zone.ID.String(),
		"name", zone.Name,
		"countries", countries,
	)

	// Return the HTMX fragment for the new zone row.
	zoneItem := admin.ShippingZoneItem{
		ID:                zone.ID.String(),
		Name:              zone.Name,
		Countries:         strings.Join(zone.Countries, ", "),
		CalculationMethod: zone.CalculationMethod,
		Position:          int(zone.Position),
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ShippingZoneRow(zoneItem, csrfToken).Render(ctx, w)
}

// DeleteZone handles POST /admin/settings/shipping/zones/{id}/delete.
// This is an HTMX endpoint that deletes a shipping zone. An empty 200 response
// is returned so the HTMX outerHTML swap removes the table row.
func (h *ShippingHandler) DeleteZone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid zone ID", http.StatusBadRequest)
		return
	}

	if err := h.shipping.DeleteZone(ctx, id); err != nil {
		h.logger.Error("failed to delete shipping zone", "error", err, "zone_id", id)
		if err == shipping.ErrZoneNotFound {
			http.Error(w, "Shipping zone not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete shipping zone", http.StatusInternalServerError)
		return
	}

	h.logger.Info("shipping zone deleted", "zone_id", id.String())

	// Return 200 with empty body — HTMX outerHTML swap removes the row.
	w.WriteHeader(http.StatusOK)
}

// showShippingWithError re-renders the shipping settings page with an error message.
// This is used when UpdateConfig fails and we need to show the form again with
// user input preserved.
func (h *ShippingHandler) showShippingWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	// Reload zones so the page renders completely.
	zones, _ := h.shipping.ListZones(ctx)
	zoneItems := make([]admin.ShippingZoneItem, 0, len(zones))
	for _, z := range zones {
		zoneItems = append(zoneItems, admin.ShippingZoneItem{
			ID:                z.ID.String(),
			Name:              z.Name,
			Countries:         strings.Join(z.Countries, ", "),
			CalculationMethod: z.CalculationMethod,
			Position:          int(z.Position),
		})
	}

	// Populate config from submitted form values so user input is preserved.
	data := admin.ShippingSettingsData{
		CSRFToken: csrfToken,
		Error:     errMsg,
		Config: admin.ShippingConfigItem{
			Enabled:               r.FormValue("enabled") != "",
			CalculationMethod:     r.FormValue("calculation_method"),
			FixedFee:              r.FormValue("fixed_fee"),
			FreeShippingThreshold: r.FormValue("free_shipping_threshold"),
			DefaultCurrency:       "EUR",
		},
		Zones: zoneItems,
	}

	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.ShippingSettingsPage(data).Render(ctx, w)
}

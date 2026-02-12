package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/vat"
	"github.com/forgecommerce/api/templates/admin"
)

// SettingsHandler handles admin settings endpoints for VAT configuration,
// selling countries, and VAT rate sync.
type SettingsHandler struct {
	queries *db.Queries
	syncer  *vat.RateSyncer
	logger  *slog.Logger
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(pool *pgxpool.Pool, syncer *vat.RateSyncer, logger *slog.Logger) *SettingsHandler {
	return &SettingsHandler{
		queries: db.New(pool),
		syncer:  syncer,
		logger:  logger,
	}
}

// RegisterRoutes registers all settings admin routes on the given mux.
func (h *SettingsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/settings/vat", h.ShowVATSettings)
	mux.HandleFunc("POST /admin/settings/vat", h.UpdateVATSettings)
	mux.HandleFunc("POST /admin/settings/countries", h.UpdateCountries)
	mux.HandleFunc("POST /admin/settings/vat/sync", h.SyncVATRates)
}

// ShowVATSettings handles GET /admin/settings/vat.
// It loads store settings, EU countries, VAT categories, shipping countries,
// and active VAT rates, then renders the VAT settings page.
func (h *SettingsHandler) ShowVATSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	// Load store settings.
	settings, err := h.queries.GetStoreSettings(ctx)
	if err != nil {
		h.logger.Error("failed to load store settings", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load EU countries.
	euCountries, err := h.queries.ListEUCountries(ctx)
	if err != nil {
		h.logger.Error("failed to list EU countries", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load VAT categories.
	vatCategories, err := h.queries.ListVATCategories(ctx)
	if err != nil {
		h.logger.Error("failed to list VAT categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load shipping countries.
	shippingCountries, err := h.queries.ListStoreShippingCountries(ctx)
	if err != nil {
		h.logger.Error("failed to list shipping countries", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load active VAT rates.
	activeRates, err := h.queries.ListActiveVATRates(ctx)
	if err != nil {
		h.logger.Error("failed to list active VAT rates", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build a set of enabled country codes for filtering rates.
	enabledCountries := make(map[string]bool, len(shippingCountries))
	for _, sc := range shippingCountries {
		if sc.IsEnabled {
			enabledCountries[sc.CountryCode] = true
		}
	}

	// Build a map of country code -> name from EU countries.
	countryNames := make(map[string]string, len(euCountries))
	for _, c := range euCountries {
		countryNames[c.CountryCode] = c.Name
	}

	// Group rates by country, only for enabled countries.
	type rateGroup struct {
		standard     string
		reduced      string
		reducedAlt   string
		superReduced string
		parking      string
	}
	ratesByCountry := make(map[string]*rateGroup)
	var lastSyncTime time.Time
	var lastSyncSource string

	for _, rate := range activeRates {
		// Track the most recent sync time/source.
		if rate.SyncedAt.After(lastSyncTime) {
			lastSyncTime = rate.SyncedAt
			lastSyncSource = rate.Source
		}

		// Only include rates for enabled shipping countries.
		if !enabledCountries[rate.CountryCode] {
			continue
		}

		group, ok := ratesByCountry[rate.CountryCode]
		if !ok {
			group = &rateGroup{}
			ratesByCountry[rate.CountryCode] = group
		}

		rateStr := formatNumeric(rate.Rate)
		if rateStr != "" {
			rateStr += "%"
		}

		switch rate.RateType {
		case "standard":
			group.standard = rateStr
		case "reduced":
			group.reduced = rateStr
		case "reduced_alt":
			group.reducedAlt = rateStr
		case "super_reduced":
			group.superReduced = rateStr
		case "parking":
			group.parking = rateStr
		}
	}

	// Convert grouped rates to template slice, ordered by country name.
	var countryRates []admin.VATCountryRates
	for _, sc := range shippingCountries {
		if !sc.IsEnabled {
			continue
		}
		group, ok := ratesByCountry[sc.CountryCode]
		if !ok {
			continue
		}
		countryRates = append(countryRates, admin.VATCountryRates{
			CountryCode:  sc.CountryCode,
			CountryName:  countryNames[sc.CountryCode],
			Standard:     group.standard,
			Reduced:      group.reduced,
			ReducedAlt:   group.reducedAlt,
			SuperReduced: group.superReduced,
			Parking:      group.parking,
		})
	}

	// Convert EU countries to template items.
	euCountryItems := make([]admin.VATCountryItem, 0, len(euCountries))
	for _, c := range euCountries {
		euCountryItems = append(euCountryItems, admin.VATCountryItem{
			Code: c.CountryCode,
			Name: c.Name,
		})
	}

	// Convert VAT categories to template items.
	vatCategoryItems := make([]admin.VATCategoryItem, 0, len(vatCategories))
	for _, cat := range vatCategories {
		vatCategoryItems = append(vatCategoryItems, admin.VATCategoryItem{
			Name:        cat.Name,
			DisplayName: cat.DisplayName,
		})
	}

	// Convert shipping countries to template items.
	shippingCountryItems := make([]admin.VATShippingCountryItem, 0, len(shippingCountries))
	for _, sc := range shippingCountries {
		shippingCountryItems = append(shippingCountryItems, admin.VATShippingCountryItem{
			Code:      sc.CountryCode,
			Name:      sc.CountryName,
			IsEnabled: sc.IsEnabled,
		})
	}

	// Format last sync time.
	lastSyncTimeStr := ""
	if !lastSyncTime.IsZero() {
		lastSyncTimeStr = lastSyncTime.Format("2006-01-02 15:04:05 MST")
	}

	data := admin.VATSettingsData{
		CSRFToken:           csrfToken,
		VATEnabled:          settings.VatEnabled,
		VATNumber:           derefString(settings.VatNumber),
		VATCountryCode:      derefString(settings.VatCountryCode),
		VATPricesIncludeVAT: settings.VatPricesIncludeVat,
		VATDefaultCategory:  settings.VatDefaultCategory,
		VATB2BReverseCharge: settings.VatB2bReverseChargeEnabled,
		EUCountries:         euCountryItems,
		VATCategories:       vatCategoryItems,
		ShippingCountries:   shippingCountryItems,
		RatesByCountry:      countryRates,
		LastSyncTime:        lastSyncTimeStr,
		LastSyncSource:      lastSyncSource,
	}

	admin.VATSettingsPage(data).Render(ctx, w)
}

// UpdateVATSettings handles POST /admin/settings/vat.
// It parses the form, updates VAT settings, and redirects back to the settings page.
func (h *SettingsHandler) UpdateVATSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	params := db.UpdateStoreVATSettingsParams{
		VatEnabled:                 r.FormValue("vat_enabled") != "",
		VatNumber:                  strPtr(r.FormValue("vat_number")),
		VatCountryCode:             strPtr(r.FormValue("vat_country_code")),
		VatPricesIncludeVat:        r.FormValue("vat_prices_include_vat") == "true",
		VatDefaultCategory:         r.FormValue("vat_default_category"),
		VatB2bReverseChargeEnabled: r.FormValue("vat_b2b_reverse_charge") != "",
		UpdatedAt:                  time.Now(),
	}

	if err := h.queries.UpdateStoreVATSettings(ctx, params); err != nil {
		h.logger.Error("failed to update VAT settings", "error", err)

		// Re-render the page with an error. Load all data needed for the form.
		h.showVATSettingsWithError(w, r, "Failed to save VAT settings. Please try again.")
		return
	}

	h.logger.Info("VAT settings updated",
		"vat_enabled", params.VatEnabled,
		"vat_country_code", derefString(params.VatCountryCode),
	)

	http.Redirect(w, r, "/admin/settings/vat", http.StatusSeeOther)
}

// UpdateCountries handles POST /admin/settings/countries.
// It updates which EU countries are enabled for selling/shipping.
func (h *SettingsHandler) UpdateCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Get the list of selected country codes from the form.
	selectedCountries := make(map[string]bool)
	for _, code := range r.Form["countries"] {
		selectedCountries[code] = true
	}

	// Load all shipping countries from the database.
	shippingCountries, err := h.queries.ListStoreShippingCountries(ctx)
	if err != nil {
		h.logger.Error("failed to list shipping countries", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update each country's enabled status.
	for _, sc := range shippingCountries {
		isEnabled := selectedCountries[sc.CountryCode]
		if err := h.queries.SetShippingCountryEnabled(ctx, db.SetShippingCountryEnabledParams{
			CountryCode: sc.CountryCode,
			IsEnabled:   isEnabled,
		}); err != nil {
			h.logger.Error("failed to update shipping country",
				"country_code", sc.CountryCode,
				"error", err,
			)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("shipping countries updated", "enabled_count", len(selectedCountries))

	http.Redirect(w, r, "/admin/settings/vat", http.StatusSeeOther)
}

// SyncVATRates handles POST /admin/settings/vat/sync.
// This is an HTMX endpoint that triggers a manual VAT rate sync and returns
// an HTML fragment with the result.
func (h *SettingsHandler) SyncVATRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("manual VAT rate sync triggered")

	result := h.syncer.Sync(ctx)

	w.Header().Set("Content-Type", "text/html")

	if result.Error != nil {
		h.logger.Error("manual VAT rate sync failed", "error", result.Error)
		fmt.Fprintf(w,
			`<div class="alert alert-error" style="margin-top: 8px;">Sync failed: %s</div>`,
			result.Error.Error(),
		)
		return
	}

	h.logger.Info("manual VAT rate sync completed",
		"source", result.Source,
		"rates_loaded", result.RatesLoaded,
		"rates_changed", result.RatesChanged,
	)

	fmt.Fprintf(w,
		`<div class="alert alert-success" style="margin-top: 8px;">Synced %d rates from %s. %d rates changed.</div>`,
		result.RatesLoaded, result.Source, result.RatesChanged,
	)
}

// showVATSettingsWithError re-renders the VAT settings page with an error message.
// This is used when UpdateVATSettings fails and we need to show the form again.
func (h *SettingsHandler) showVATSettingsWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	// Load all reference data needed for the form.
	euCountries, _ := h.queries.ListEUCountries(ctx)
	vatCategories, _ := h.queries.ListVATCategories(ctx)
	shippingCountries, _ := h.queries.ListStoreShippingCountries(ctx)

	euCountryItems := make([]admin.VATCountryItem, 0, len(euCountries))
	for _, c := range euCountries {
		euCountryItems = append(euCountryItems, admin.VATCountryItem{
			Code: c.CountryCode,
			Name: c.Name,
		})
	}

	vatCategoryItems := make([]admin.VATCategoryItem, 0, len(vatCategories))
	for _, cat := range vatCategories {
		vatCategoryItems = append(vatCategoryItems, admin.VATCategoryItem{
			Name:        cat.Name,
			DisplayName: cat.DisplayName,
		})
	}

	shippingCountryItems := make([]admin.VATShippingCountryItem, 0, len(shippingCountries))
	for _, sc := range shippingCountries {
		shippingCountryItems = append(shippingCountryItems, admin.VATShippingCountryItem{
			Code:      sc.CountryCode,
			Name:      sc.CountryName,
			IsEnabled: sc.IsEnabled,
		})
	}

	// Populate form data from the submitted form values so user input is preserved.
	data := admin.VATSettingsData{
		CSRFToken:           csrfToken,
		Error:               errMsg,
		VATEnabled:          r.FormValue("vat_enabled") != "",
		VATNumber:           r.FormValue("vat_number"),
		VATCountryCode:      r.FormValue("vat_country_code"),
		VATPricesIncludeVAT: r.FormValue("vat_prices_include_vat") == "true",
		VATDefaultCategory:  r.FormValue("vat_default_category"),
		VATB2BReverseCharge: r.FormValue("vat_b2b_reverse_charge") != "",
		EUCountries:         euCountryItems,
		VATCategories:       vatCategoryItems,
		ShippingCountries:   shippingCountryItems,
	}

	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.VATSettingsPage(data).Render(ctx, w)
}

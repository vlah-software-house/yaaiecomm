package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/discount"
	"github.com/forgecommerce/api/templates/admin"
)

// DiscountHandler handles admin discount and coupon management endpoints.
type DiscountHandler struct {
	discounts *discount.Service
	logger    *slog.Logger
}

// NewDiscountHandler creates a new discount handler.
func NewDiscountHandler(discounts *discount.Service, logger *slog.Logger) *DiscountHandler {
	return &DiscountHandler{
		discounts: discounts,
		logger:    logger,
	}
}

// RegisterRoutes registers discount and coupon admin routes on the given mux.
func (h *DiscountHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/discounts", h.ListDiscounts)
	mux.HandleFunc("GET /admin/discounts/new", h.NewDiscount)
	mux.HandleFunc("POST /admin/discounts", h.CreateDiscount)
	mux.HandleFunc("GET /admin/discounts/{id}", h.EditDiscount)
	mux.HandleFunc("POST /admin/discounts/{id}", h.UpdateDiscount)
	mux.HandleFunc("GET /admin/coupons", h.ListCoupons)
	mux.HandleFunc("POST /admin/coupons", h.CreateCoupon)
	mux.HandleFunc("POST /admin/coupons/{id}/delete", h.DeleteCoupon)
}

// ListDiscounts handles GET /admin/discounts.
func (h *DiscountHandler) ListDiscounts(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := int32(defaultPageSize)
	offset := int32((page - 1) * defaultPageSize)

	// Fetch one extra row to detect if there is a next page.
	discounts, err := h.discounts.ListDiscounts(r.Context(), limit+1, offset)
	if err != nil {
		h.logger.Error("failed to list discounts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hasNextPage := len(discounts) > int(limit)
	if hasNextPage {
		discounts = discounts[:limit]
	}

	totalPages := page
	if hasNextPage {
		totalPages = page + 1
	}

	items := make([]admin.DiscountListItem, 0, len(discounts))
	for _, d := range discounts {
		items = append(items, admin.DiscountListItem{
			ID:        d.ID.String(),
			Name:      d.Name,
			Type:      d.Type,
			Value:     formatNumeric(d.Value),
			Scope:     d.Scope,
			IsActive:  d.IsActive,
			Priority:  int(d.Priority),
			Stackable: d.Stackable,
			StartsAt:  formatTimestamptz(d.StartsAt),
			EndsAt:    formatTimestamptz(d.EndsAt),
		})
	}

	data := admin.DiscountListData{
		Discounts:   items,
		CurrentPage: page,
		TotalPages:  totalPages,
		CSRFToken:   csrfToken,
	}

	admin.DiscountListPage(data).Render(r.Context(), w)
}

// NewDiscount handles GET /admin/discounts/new.
func (h *DiscountHandler) NewDiscount(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	data := admin.DiscountFormData{
		Discount: admin.DiscountFormItem{
			Type:     "percentage",
			Scope:    "subtotal",
			IsActive: true,
			Priority: "0",
		},
		IsEdit:    false,
		CSRFToken: csrfToken,
	}

	admin.DiscountFormPage(data).Render(r.Context(), w)
}

// CreateDiscount handles POST /admin/discounts.
func (h *DiscountHandler) CreateDiscount(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		h.renderDiscountFormWithError(w, r, admin.DiscountFormData{
			CSRFToken: csrfToken,
		}, "Invalid form data.")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderDiscountFormWithError(w, r, discountFormDataFromRequest(r, csrfToken, false), "Discount name is required.")
		return
	}

	value := parseNumeric(r.FormValue("value"))
	if !value.Valid {
		h.renderDiscountFormWithError(w, r, discountFormDataFromRequest(r, csrfToken, false), "Value is required.")
		return
	}

	params := discount.CreateDiscountParams{
		Name:            name,
		Type:            r.FormValue("type"),
		Value:           value,
		Scope:           r.FormValue("scope"),
		MinimumAmount:   parseNumeric(r.FormValue("minimum_amount")),
		MaximumDiscount: parseNumeric(r.FormValue("maximum_discount")),
		StartsAt:        parseTimestamptzForm(r.FormValue("starts_at")),
		EndsAt:          parseTimestamptzForm(r.FormValue("ends_at")),
		IsActive:        r.FormValue("is_active") == "true",
		Priority:        parseInt32(r.FormValue("priority")),
		Stackable:       r.FormValue("stackable") == "true",
	}

	_, err := h.discounts.CreateDiscount(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to create discount", "error", err)
		h.renderDiscountFormWithError(w, r, discountFormDataFromRequest(r, csrfToken, false), "Failed to create discount.")
		return
	}

	http.Redirect(w, r, "/admin/discounts", http.StatusSeeOther)
}

// EditDiscount handles GET /admin/discounts/{id}.
func (h *DiscountHandler) EditDiscount(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid discount ID", http.StatusBadRequest)
		return
	}

	d, err := h.discounts.GetDiscount(r.Context(), id)
	if err != nil {
		if errors.Is(err, discount.ErrNotFound) {
			http.Error(w, "Discount not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get discount", "error", err, "discount_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := admin.DiscountFormData{
		Discount: admin.DiscountFormItem{
			ID:              d.ID.String(),
			Name:            d.Name,
			Type:            d.Type,
			Value:           formatNumeric(d.Value),
			Scope:           d.Scope,
			MinimumAmount:   formatNumeric(d.MinimumAmount),
			MaximumDiscount: formatNumeric(d.MaximumDiscount),
			StartsAt:        formatTimestamptzForm(d.StartsAt),
			EndsAt:          formatTimestamptzForm(d.EndsAt),
			IsActive:        d.IsActive,
			Priority:        fmt.Sprintf("%d", d.Priority),
			Stackable:       d.Stackable,
		},
		IsEdit:    true,
		CSRFToken: csrfToken,
	}

	admin.DiscountFormPage(data).Render(r.Context(), w)
}

// UpdateDiscount handles POST /admin/discounts/{id}.
func (h *DiscountHandler) UpdateDiscount(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid discount ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		formData := discountFormDataFromRequest(r, csrfToken, true)
		formData.Discount.ID = id.String()
		h.renderDiscountFormWithError(w, r, formData, "Discount name is required.")
		return
	}

	value := parseNumeric(r.FormValue("value"))
	if !value.Valid {
		formData := discountFormDataFromRequest(r, csrfToken, true)
		formData.Discount.ID = id.String()
		h.renderDiscountFormWithError(w, r, formData, "Value is required.")
		return
	}

	params := discount.UpdateDiscountParams{
		Name:            name,
		Type:            r.FormValue("type"),
		Value:           value,
		Scope:           r.FormValue("scope"),
		MinimumAmount:   parseNumeric(r.FormValue("minimum_amount")),
		MaximumDiscount: parseNumeric(r.FormValue("maximum_discount")),
		StartsAt:        parseTimestamptzForm(r.FormValue("starts_at")),
		EndsAt:          parseTimestamptzForm(r.FormValue("ends_at")),
		IsActive:        r.FormValue("is_active") == "true",
		Priority:        parseInt32(r.FormValue("priority")),
		Stackable:       r.FormValue("stackable") == "true",
	}

	_, err = h.discounts.UpdateDiscount(r.Context(), id, params)
	if err != nil {
		if errors.Is(err, discount.ErrNotFound) {
			http.Error(w, "Discount not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to update discount", "error", err, "discount_id", id)
		formData := discountFormDataFromRequest(r, csrfToken, true)
		formData.Discount.ID = id.String()
		h.renderDiscountFormWithError(w, r, formData, "Failed to update discount.")
		return
	}

	http.Redirect(w, r, "/admin/discounts", http.StatusSeeOther)
}

// ListCoupons handles GET /admin/coupons.
func (h *DiscountHandler) ListCoupons(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := int32(defaultPageSize)
	offset := int32((page - 1) * defaultPageSize)

	// Fetch one extra row to detect if there is a next page.
	coupons, err := h.discounts.ListCoupons(ctx, limit+1, offset)
	if err != nil {
		h.logger.Error("failed to list coupons", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hasNextPage := len(coupons) > int(limit)
	if hasNextPage {
		coupons = coupons[:limit]
	}

	totalPages := page
	if hasNextPage {
		totalPages = page + 1
	}

	// Build coupon list items.
	items := make([]admin.CouponListItem, 0, len(coupons))
	for _, c := range coupons {
		items = append(items, admin.CouponListItem{
			ID:                    c.ID.String(),
			Code:                  c.Code,
			DiscountName:          c.DiscountName,
			UsageCount:            int(c.UsageCount),
			UsageLimit:            formatOptionalInt32(c.UsageLimit),
			UsageLimitPerCustomer: formatOptionalInt32(c.UsageLimitPerCustomer),
			IsActive:              c.IsActive,
			StartsAt:              formatTimestamptz(c.StartsAt),
			EndsAt:                formatTimestamptz(c.EndsAt),
		})
	}

	// Fetch all discounts for the create coupon dropdown.
	// Use a large limit to get all discounts.
	allDiscounts, err := h.discounts.ListDiscounts(ctx, 1000, 0)
	if err != nil {
		h.logger.Error("failed to list discounts for coupon form", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	discountRefs := make([]admin.DiscountRefItem, 0, len(allDiscounts))
	for _, d := range allDiscounts {
		discountRefs = append(discountRefs, admin.DiscountRefItem{
			ID:   d.ID.String(),
			Name: d.Name,
		})
	}

	data := admin.CouponListData{
		Coupons:     items,
		Discounts:   discountRefs,
		CurrentPage: page,
		TotalPages:  totalPages,
		CSRFToken:   csrfToken,
	}

	admin.CouponListPage(data).Render(ctx, w)
}

// CreateCoupon handles POST /admin/coupons.
// Returns an HTMX fragment (table row) for appending to the coupon table.
func (h *DiscountHandler) CreateCoupon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse coupon form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		http.Error(w, "Coupon code is required", http.StatusBadRequest)
		return
	}

	discountID, err := uuid.Parse(r.FormValue("discount_id"))
	if err != nil {
		http.Error(w, "Invalid discount ID", http.StatusBadRequest)
		return
	}

	params := discount.CreateCouponParams{
		Code:                  code,
		DiscountID:            discountID,
		UsageLimit:            parseOptionalInt32(r.FormValue("usage_limit")),
		UsageLimitPerCustomer: parseOptionalInt32(r.FormValue("usage_limit_per_customer")),
		IsActive:              r.FormValue("is_active") == "true",
	}

	coupon, err := h.discounts.CreateCoupon(ctx, params)
	if err != nil {
		if errors.Is(err, discount.ErrNotFound) {
			http.Error(w, "Selected discount does not exist", http.StatusBadRequest)
			return
		}
		h.logger.Error("failed to create coupon", "error", err)
		http.Error(w, "Failed to create coupon", http.StatusInternalServerError)
		return
	}

	// Look up the discount name for the response row.
	d, err := h.discounts.GetDiscount(ctx, discountID)
	if err != nil {
		h.logger.Error("failed to get discount for coupon response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	item := admin.CouponListItem{
		ID:                    coupon.ID.String(),
		Code:                  coupon.Code,
		DiscountName:          d.Name,
		UsageCount:            int(coupon.UsageCount),
		UsageLimit:            formatOptionalInt32(coupon.UsageLimit),
		UsageLimitPerCustomer: formatOptionalInt32(coupon.UsageLimitPerCustomer),
		IsActive:              coupon.IsActive,
		StartsAt:              formatTimestamptz(coupon.StartsAt),
		EndsAt:                formatTimestamptz(coupon.EndsAt),
	}

	w.Header().Set("Content-Type", "text/html")
	admin.CouponRow(item, csrfToken).Render(ctx, w)
}

// DeleteCoupon handles POST /admin/coupons/{id}/delete.
// Permanently removes a coupon and returns an empty body so the
// HTMX hx-swap="outerHTML" removes the row from the table.
func (h *DiscountHandler) DeleteCoupon(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid coupon ID", http.StatusBadRequest)
		return
	}

	if err := h.discounts.DeleteCoupon(r.Context(), id); err != nil {
		h.logger.Error("failed to delete coupon", "coupon_id", id, "error", err)
		http.Error(w, "Failed to delete coupon", http.StatusInternalServerError)
		return
	}

	h.logger.Info("coupon deleted", "coupon_id", id)

	// Return empty 200 OK so the HTMX hx-swap="outerHTML" removes the row.
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

// renderDiscountFormWithError sets a 422 status and renders the discount form with an error.
func (h *DiscountHandler) renderDiscountFormWithError(w http.ResponseWriter, r *http.Request, data admin.DiscountFormData, errMsg string) {
	data.Error = errMsg
	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.DiscountFormPage(data).Render(r.Context(), w)
}

// discountFormDataFromRequest reconstructs form data from the submitted request values
// so the form can be re-rendered with user input preserved on error.
func discountFormDataFromRequest(r *http.Request, csrfToken string, isEdit bool) admin.DiscountFormData {
	return admin.DiscountFormData{
		Discount: admin.DiscountFormItem{
			Name:            r.FormValue("name"),
			Type:            r.FormValue("type"),
			Value:           r.FormValue("value"),
			Scope:           r.FormValue("scope"),
			MinimumAmount:   r.FormValue("minimum_amount"),
			MaximumDiscount: r.FormValue("maximum_discount"),
			StartsAt:        r.FormValue("starts_at"),
			EndsAt:          r.FormValue("ends_at"),
			IsActive:        r.FormValue("is_active") == "true",
			Priority:        r.FormValue("priority"),
			Stackable:       r.FormValue("stackable") == "true",
		},
		IsEdit:    isEdit,
		CSRFToken: csrfToken,
	}
}

// parseTimestamptzForm parses a datetime-local input value (2006-01-02T15:04)
// into a pgtype.Timestamptz. Returns an invalid Timestamptz if the string is
// empty or cannot be parsed.
func parseTimestamptzForm(s string) pgtype.Timestamptz {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Timestamptz{}
	}
	t, err := time.Parse("2006-01-02T15:04", s)
	if err != nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{
		Time:  t.UTC(),
		Valid: true,
	}
}

// formatTimestamptzForm formats a pgtype.Timestamptz into a datetime-local
// input value (2006-01-02T15:04). Returns an empty string if invalid.
func formatTimestamptzForm(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format("2006-01-02T15:04")
}

// parseOptionalInt32 parses a form string into an *int32. Returns nil if the
// string is empty or cannot be parsed.
func parseOptionalInt32(s string) *int32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return nil
	}
	val := int32(v)
	return &val
}

// formatOptionalInt32 formats an *int32 as a string. Returns "Unlimited" if nil.
func formatOptionalInt32(v *int32) string {
	if v == nil {
		return "Unlimited"
	}
	return fmt.Sprintf("%d", *v)
}

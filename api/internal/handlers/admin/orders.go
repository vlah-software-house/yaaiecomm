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
	"github.com/forgecommerce/api/internal/services/order"
	"github.com/forgecommerce/api/templates/admin"
)

// OrderHandler handles admin order management endpoints.
type OrderHandler struct {
	orders *order.Service
	logger *slog.Logger
}

// NewOrderHandler creates a new order handler.
func NewOrderHandler(orders *order.Service, logger *slog.Logger) *OrderHandler {
	return &OrderHandler{
		orders: orders,
		logger: logger,
	}
}

// RegisterRoutes registers order admin routes on the given mux.
func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/orders", h.ListOrders)
	mux.HandleFunc("GET /admin/orders/{id}", h.ShowOrder)
	mux.HandleFunc("POST /admin/orders/{id}/status", h.UpdateStatus)
	mux.HandleFunc("POST /admin/orders/{id}/tracking", h.UpdateTracking)
}

// ListOrders handles GET /admin/orders.
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
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

	orders, total, err := h.orders.List(r.Context(), statusFilter, page, defaultPageSize)
	if err != nil {
		h.logger.Error("failed to list orders", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalPages := int((total + int64(defaultPageSize) - 1) / int64(defaultPageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	items := make([]admin.OrderListItem, 0, len(orders))
	for _, o := range orders {
		items = append(items, admin.OrderListItem{
			ID:            o.ID.String(),
			OrderNumber:   fmt.Sprintf("#%d", o.OrderNumber),
			CustomerEmail: o.Email,
			Status:        o.Status,
			PaymentStatus: o.PaymentStatus,
			Total:         formatNumeric(o.Total),
			VatTotal:      formatNumeric(o.VatTotal),
			ItemCount:     0,
			CreatedAt:     o.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	statusStr := ""
	if statusFilter != nil {
		statusStr = *statusFilter
	}

	data := admin.OrderListData{
		Orders:      items,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalOrders: int(total),
		StatusFilter: statusStr,
	}

	admin.OrderListPage(data).Render(r.Context(), w)
}

// ShowOrder handles GET /admin/orders/{id}.
func (h *OrderHandler) ShowOrder(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	o, err := h.orders.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get order", "error", err, "order_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	items, err := h.orders.ListItems(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list order items", "error", err, "order_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	events, err := h.orders.ListEvents(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list order events", "error", err, "order_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build order items for the template.
	orderItems := make([]admin.OrderDetailItemRow, 0, len(items))
	for _, item := range items {
		orderItems = append(orderItems, admin.OrderDetailItemRow{
			ProductName:    item.ProductName,
			VariantName:    derefString(item.VariantName),
			SKU:            derefString(item.Sku),
			Quantity:       int(item.Quantity),
			UnitPrice:      formatNumeric(item.UnitPrice),
			TotalPrice:     formatNumeric(item.TotalPrice),
			VatRate:        formatNumeric(item.VatRate),
			VatRateType:    derefString(item.VatRateType),
			VatAmount:      formatNumeric(item.VatAmount),
			NetUnitPrice:   formatNumeric(item.NetUnitPrice),
			GrossUnitPrice: formatNumeric(item.GrossUnitPrice),
		})
	}

	// Build order events for the template.
	orderEvents := make([]admin.OrderEventItem, 0, len(events))
	for _, ev := range events {
		orderEvents = append(orderEvents, admin.OrderEventItem{
			EventType:  ev.EventType,
			FromStatus: derefString(ev.FromStatus),
			ToStatus:   derefString(ev.ToStatus),
			CreatedAt:  ev.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	data := admin.OrderDetailData{
		Order: admin.OrderDetailItem{
			ID:                o.ID.String(),
			OrderNumber:       fmt.Sprintf("#%d", o.OrderNumber),
			Email:             o.Email,
			Status:            o.Status,
			PaymentStatus:     o.PaymentStatus,
			Subtotal:          formatNumeric(o.Subtotal),
			ShippingFee:       formatNumeric(o.ShippingFee),
			ShippingExtraFees: formatNumeric(o.ShippingExtraFees),
			DiscountAmount:    formatNumeric(o.DiscountAmount),
			VatTotal:          formatNumeric(o.VatTotal),
			Total:             formatNumeric(o.Total),
			VatNumber:         derefString(o.VatNumber),
			VatCompanyName:    derefString(o.VatCompanyName),
			VatReverseCharge:  o.VatReverseCharge,
			VatCountryCode:    derefString(o.VatCountryCode),
			ShippingMethod:    derefString(o.ShippingMethod),
			TrackingNumber:    derefString(o.TrackingNumber),
			ShippedAt:         formatTimestamptz(o.ShippedAt),
			DeliveredAt:       formatTimestamptz(o.DeliveredAt),
			BillingAddress:    string(o.BillingAddress),
			ShippingAddress:   string(o.ShippingAddress),
			Notes:             derefString(o.Notes),
			CustomerNotes:     derefString(o.CustomerNotes),
			CreatedAt:         o.CreatedAt.Format("2006-01-02 15:04"),
		},
		Items:     orderItems,
		Events:    orderEvents,
		CSRFToken: csrfToken,
	}

	admin.OrderDetailPage(data).Render(r.Context(), w)
}

// UpdateStatus handles POST /admin/orders/{id}/status.
func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	newStatus := strings.TrimSpace(r.FormValue("status"))
	if newStatus == "" {
		http.Error(w, "Status is required", http.StatusBadRequest)
		return
	}

	_, err = h.orders.UpdateStatus(r.Context(), id, newStatus)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to update order status", "error", err, "order_id", id, "new_status", newStatus)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/orders/"+id.String(), http.StatusSeeOther)
}

// UpdateTracking handles POST /admin/orders/{id}/tracking.
func (h *OrderHandler) UpdateTracking(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	trackingNumber := strPtr(r.FormValue("tracking_number"))

	// Set shipped_at to now if a tracking number is provided.
	var shippedAt pgtype.Timestamptz
	if trackingNumber != nil {
		shippedAt = pgtype.Timestamptz{
			Time:  time.Now().UTC(),
			Valid: true,
		}
	}

	if err := h.orders.UpdateTracking(r.Context(), id, trackingNumber, shippedAt); err != nil {
		if errors.Is(err, order.ErrNotFound) {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to update order tracking", "error", err, "order_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/orders/"+id.String(), http.StatusSeeOther)
}

// formatTimestamptz formats a pgtype.Timestamptz into a readable string.
// Returns an empty string if the timestamp is not valid (NULL).
func formatTimestamptz(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format("2006-01-02 15:04")
}

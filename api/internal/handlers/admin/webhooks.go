package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/webhook"
	"github.com/forgecommerce/api/templates/admin"
)

// allWebhookEvents lists the available event types for webhook subscriptions.
var allWebhookEvents = []string{
	webhook.EventOrderCreated,
	webhook.EventOrderUpdated,
	webhook.EventOrderCompleted,
	webhook.EventProductCreated,
	webhook.EventProductUpdated,
	webhook.EventProductDeleted,
	webhook.EventStockLow,
}

// WebhookHandler handles admin webhook management endpoints.
type WebhookHandler struct {
	webhooks *webhook.Service
	logger   *slog.Logger
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(webhooks *webhook.Service, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		webhooks: webhooks,
		logger:   logger,
	}
}

// RegisterRoutes registers webhook admin routes on the given mux.
func (h *WebhookHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/webhooks", h.ListEndpoints)
	mux.HandleFunc("GET /admin/webhooks/new", h.NewEndpoint)
	mux.HandleFunc("POST /admin/webhooks", h.CreateEndpoint)
	mux.HandleFunc("GET /admin/webhooks/{id}/edit", h.EditEndpoint)
	mux.HandleFunc("POST /admin/webhooks/{id}", h.UpdateEndpoint)
	mux.HandleFunc("POST /admin/webhooks/{id}/delete", h.DeleteEndpoint)
	mux.HandleFunc("GET /admin/webhooks/{id}", h.ShowEndpoint)
}

// ListEndpoints handles GET /admin/webhooks.
func (h *WebhookHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	endpoints, err := h.webhooks.ListEndpoints(r.Context())
	if err != nil {
		h.logger.Error("failed to list webhook endpoints", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	items := make([]admin.WebhookEndpointItem, 0, len(endpoints))
	for _, ep := range endpoints {
		items = append(items, admin.WebhookEndpointItem{
			ID:          ep.ID.String(),
			Url:         ep.Url,
			Description: derefString(ep.Description),
			Events:      strings.Join(ep.Events, ", "),
			IsActive:    ep.IsActive,
			CreatedAt:   ep.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	data := admin.WebhookListData{
		Endpoints: items,
		CSRFToken: csrfToken,
	}

	admin.WebhookListPage(data).Render(r.Context(), w)
}

// NewEndpoint handles GET /admin/webhooks/new.
func (h *WebhookHandler) NewEndpoint(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	data := admin.WebhookFormData{
		IsEdit:    false,
		CSRFToken: csrfToken,
		AllEvents: allWebhookEvents,
	}

	admin.WebhookFormPage(data).Render(r.Context(), w)
}

// CreateEndpoint handles POST /admin/webhooks.
func (h *WebhookHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	secret := strings.TrimSpace(r.FormValue("secret"))
	description := strings.TrimSpace(r.FormValue("description"))
	events := r.Form["events"]
	isActive := r.FormValue("is_active") == "true"

	// Validation
	if url == "" || secret == "" {
		data := admin.WebhookFormData{
			IsEdit:    false,
			CSRFToken: csrfToken,
			AllEvents: allWebhookEvents,
			Error:     "URL and signing secret are required.",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	if len(events) == 0 {
		data := admin.WebhookFormData{
			IsEdit:    false,
			CSRFToken: csrfToken,
			AllEvents: allWebhookEvents,
			Error:     "At least one event must be selected.",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	_, err := h.webhooks.CreateEndpoint(r.Context(), url, secret, description, events, isActive)
	if err != nil {
		h.logger.Error("failed to create webhook endpoint", "error", err)
		data := admin.WebhookFormData{
			IsEdit:    false,
			CSRFToken: csrfToken,
			AllEvents: allWebhookEvents,
			Error:     "Failed to create webhook endpoint. Please try again.",
		}
		w.WriteHeader(http.StatusInternalServerError)
		admin.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

// EditEndpoint handles GET /admin/webhooks/{id}/edit.
func (h *WebhookHandler) EditEndpoint(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid endpoint ID", http.StatusBadRequest)
		return
	}

	ep, err := h.webhooks.GetEndpoint(r.Context(), id)
	if err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			http.Error(w, "Webhook endpoint not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get webhook endpoint", "error", err, "id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	item := &admin.WebhookEndpointItem{
		ID:          ep.ID.String(),
		Url:         ep.Url,
		Description: derefString(ep.Description),
		Events:      strings.Join(ep.Events, ", "),
		IsActive:    ep.IsActive,
		CreatedAt:   ep.CreatedAt.Format("2006-01-02 15:04"),
	}

	data := admin.WebhookFormData{
		Endpoint:  item,
		IsEdit:    true,
		CSRFToken: csrfToken,
		AllEvents: allWebhookEvents,
	}

	admin.WebhookFormPage(data).Render(r.Context(), w)
}

// UpdateEndpoint handles POST /admin/webhooks/{id}.
func (h *WebhookHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid endpoint ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	description := strings.TrimSpace(r.FormValue("description"))
	events := r.Form["events"]
	isActive := r.FormValue("is_active") == "true"

	if url == "" {
		item := &admin.WebhookEndpointItem{
			ID:  id.String(),
			Url: url,
		}
		data := admin.WebhookFormData{
			Endpoint:  item,
			IsEdit:    true,
			CSRFToken: csrfToken,
			AllEvents: allWebhookEvents,
			Error:     "URL is required.",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	if len(events) == 0 {
		item := &admin.WebhookEndpointItem{
			ID:          id.String(),
			Url:         url,
			Description: description,
		}
		data := admin.WebhookFormData{
			Endpoint:  item,
			IsEdit:    true,
			CSRFToken: csrfToken,
			AllEvents: allWebhookEvents,
			Error:     "At least one event must be selected.",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	_, err = h.webhooks.UpdateEndpoint(r.Context(), id, url, description, events, isActive)
	if err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			http.Error(w, "Webhook endpoint not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to update webhook endpoint", "error", err, "id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

// DeleteEndpoint handles POST /admin/webhooks/{id}/delete.
func (h *WebhookHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid endpoint ID", http.StatusBadRequest)
		return
	}

	if err := h.webhooks.DeleteEndpoint(r.Context(), id); err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			http.Error(w, "Webhook endpoint not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete webhook endpoint", "error", err, "id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

// ShowEndpoint handles GET /admin/webhooks/{id} — shows endpoint details and delivery history.
func (h *WebhookHandler) ShowEndpoint(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid endpoint ID", http.StatusBadRequest)
		return
	}

	ep, err := h.webhooks.GetEndpoint(r.Context(), id)
	if err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			http.Error(w, "Webhook endpoint not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get webhook endpoint", "error", err, "id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	offset := (page - 1) * pageSize

	deliveries, err := h.webhooks.ListDeliveries(r.Context(), id, pageSize, offset)
	if err != nil {
		h.logger.Error("failed to list webhook deliveries", "error", err, "endpoint_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalCount, err := h.webhooks.CountDeliveries(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to count webhook deliveries", "error", err, "endpoint_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	deliveryItems := make([]admin.WebhookDeliveryItem, 0, len(deliveries))
	for _, d := range deliveries {
		var responseStatus string
		if d.ResponseStatus != nil {
			responseStatus = fmt.Sprintf("%d", *d.ResponseStatus)
		}

		var deliveredAt string
		if d.DeliveredAt.Valid {
			deliveredAt = d.DeliveredAt.Time.Format("2006-01-02 15:04")
		}

		deliveryItems = append(deliveryItems, admin.WebhookDeliveryItem{
			ID:             d.ID.String(),
			EventType:      d.EventType,
			ResponseStatus: responseStatus,
			Attempt:        int(d.Attempt),
			DeliveredAt:    deliveredAt,
			CreatedAt:      d.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	data := admin.WebhookDetailData{
		Endpoint: admin.WebhookEndpointItem{
			ID:          ep.ID.String(),
			Url:         ep.Url,
			Description: derefString(ep.Description),
			Events:      strings.Join(ep.Events, ", "),
			IsActive:    ep.IsActive,
			CreatedAt:   ep.CreatedAt.Format("2006-01-02 15:04"),
		},
		Deliveries: deliveryItems,
		CSRFToken:  csrfToken,
		Page:       page,
		TotalPages: totalPages,
	}

	admin.WebhookDetailPage(data).Render(r.Context(), w)
}

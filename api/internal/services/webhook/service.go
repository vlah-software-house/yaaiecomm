package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a webhook endpoint does not exist.
	ErrNotFound = errors.New("webhook endpoint not found")
)

// Event type constants for webhook dispatch.
const (
	EventOrderCreated   = "order.created"
	EventOrderUpdated   = "order.updated"
	EventOrderCompleted = "order.completed"
	EventProductCreated = "product.created"
	EventProductUpdated = "product.updated"
	EventProductDeleted = "product.deleted"
	EventStockLow       = "stock.low"
)

// Service provides business logic for webhook operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
	client  *http.Client
}

// NewService creates a new webhook service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		pool:    pool,
		logger:  logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateEndpoint registers a new webhook endpoint.
func (s *Service) CreateEndpoint(ctx context.Context, url, secret, description string, events []string, isActive bool) (db.WebhookEndpoint, error) {
	var desc *string
	if description != "" {
		desc = &description
	}

	endpoint, err := s.queries.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
		Url:         url,
		Secret:      secret,
		Events:      events,
		Description: desc,
		IsActive:    isActive,
	})
	if err != nil {
		return db.WebhookEndpoint{}, fmt.Errorf("create webhook endpoint: %w", err)
	}
	return endpoint, nil
}

// GetEndpoint retrieves a webhook endpoint by ID.
func (s *Service) GetEndpoint(ctx context.Context, id uuid.UUID) (db.WebhookEndpoint, error) {
	endpoint, err := s.queries.GetWebhookEndpoint(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.WebhookEndpoint{}, ErrNotFound
		}
		return db.WebhookEndpoint{}, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return endpoint, nil
}

// ListEndpoints returns all webhook endpoints.
func (s *Service) ListEndpoints(ctx context.Context) ([]db.WebhookEndpoint, error) {
	endpoints, err := s.queries.ListWebhookEndpoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	return endpoints, nil
}

// UpdateEndpoint updates a webhook endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, id uuid.UUID, url, description string, events []string, isActive bool) (db.WebhookEndpoint, error) {
	// Verify existence first.
	_, err := s.queries.GetWebhookEndpoint(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.WebhookEndpoint{}, ErrNotFound
		}
		return db.WebhookEndpoint{}, fmt.Errorf("get webhook endpoint for update: %w", err)
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	endpoint, err := s.queries.UpdateWebhookEndpoint(ctx, db.UpdateWebhookEndpointParams{
		ID:          id,
		Url:         url,
		Events:      events,
		Description: desc,
		IsActive:    isActive,
	})
	if err != nil {
		return db.WebhookEndpoint{}, fmt.Errorf("update webhook endpoint: %w", err)
	}
	return endpoint, nil
}

// DeleteEndpoint removes a webhook endpoint.
func (s *Service) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	// Verify existence first.
	_, err := s.queries.GetWebhookEndpoint(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get webhook endpoint for delete: %w", err)
	}

	if err := s.queries.DeleteWebhookEndpoint(ctx, id); err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}

// Dispatch sends an event to all active endpoints subscribed to the event type.
// The delivery is performed asynchronously in a background goroutine.
func (s *Service) Dispatch(ctx context.Context, eventType string, payload interface{}) {
	go func() {
		bgCtx := context.Background()

		endpoints, err := s.queries.ListActiveWebhookEndpoints(bgCtx)
		if err != nil {
			s.logger.Error("list active webhook endpoints for dispatch", "error", err)
			return
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error("marshal webhook payload", "error", err, "event", eventType)
			return
		}

		for _, ep := range endpoints {
			if !containsEvent(ep.Events, eventType) {
				continue
			}

			delivery, err := s.queries.CreateWebhookDelivery(bgCtx, db.CreateWebhookDeliveryParams{
				EndpointID: ep.ID,
				EventType:  eventType,
				Payload:    payloadBytes,
			})
			if err != nil {
				s.logger.Error("create webhook delivery record",
					"error", err,
					"endpoint_id", ep.ID,
					"event", eventType,
				)
				continue
			}

			s.deliver(bgCtx, delivery.ID, ep.Url, ep.Secret, eventType, payloadBytes)
		}
	}()
}

// deliver performs the actual HTTP POST to the webhook endpoint.
func (s *Service) deliver(ctx context.Context, deliveryID uuid.UUID, url, secret, eventType string, payload []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		s.markFailed(ctx, deliveryID, err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-Delivery", deliveryID.String())

	if secret != "" {
		sig := signPayload(payload, secret)
		req.Header.Set("X-Webhook-Signature", sig)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.markFailed(ctx, deliveryID, err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := string(body)

	statusCode := int32(resp.StatusCode)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		if err := s.queries.UpdateWebhookDeliverySuccess(ctx, db.UpdateWebhookDeliverySuccessParams{
			ID:             deliveryID,
			ResponseStatus: &statusCode,
			ResponseBody:   &bodyStr,
		}); err != nil {
			s.logger.Error("update webhook delivery success",
				"error", err,
				"delivery_id", deliveryID,
			)
		}
	} else {
		// HTTP error, schedule retry
		if err := s.queries.UpdateWebhookDeliveryFailed(ctx, db.UpdateWebhookDeliveryFailedParams{
			ID:             deliveryID,
			ResponseStatus: &statusCode,
			ResponseBody:   &bodyStr,
			NextRetryAt:    pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
		}); err != nil {
			s.logger.Error("update webhook delivery failed",
				"error", err,
				"delivery_id", deliveryID,
			)
		}
	}
}

// markFailed records a delivery failure when no HTTP response was received (e.g., DNS error, timeout).
func (s *Service) markFailed(ctx context.Context, deliveryID uuid.UUID, errMsg string) {
	if err := s.queries.UpdateWebhookDeliveryFailed(ctx, db.UpdateWebhookDeliveryFailedParams{
		ID:           deliveryID,
		ResponseBody: &errMsg,
		NextRetryAt:  pgtype.Timestamptz{Time: time.Now().Add(5 * time.Minute), Valid: true},
	}); err != nil {
		s.logger.Error("mark webhook delivery failed",
			"error", err,
			"delivery_id", deliveryID,
		)
	}
}

// ListDeliveries returns delivery history for an endpoint.
func (s *Service) ListDeliveries(ctx context.Context, endpointID uuid.UUID, limit, offset int) ([]db.WebhookDelivery, error) {
	deliveries, err := s.queries.ListWebhookDeliveries(ctx, db.ListWebhookDeliveriesParams{
		EndpointID: endpointID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// CountDeliveries returns the total number of deliveries for an endpoint.
func (s *Service) CountDeliveries(ctx context.Context, endpointID uuid.UUID) (int64, error) {
	count, err := s.queries.CountWebhookDeliveries(ctx, endpointID)
	if err != nil {
		return 0, fmt.Errorf("count webhook deliveries: %w", err)
	}
	return count, nil
}

// ProcessPendingDeliveries retries failed/pending webhook deliveries.
// This should be called periodically (e.g., every minute via scheduler).
func (s *Service) ProcessPendingDeliveries(ctx context.Context) error {
	deliveries, err := s.queries.ListPendingWebhookDeliveries(ctx, 50)
	if err != nil {
		return fmt.Errorf("list pending deliveries: %w", err)
	}

	for _, d := range deliveries {
		s.deliver(ctx, d.ID, d.Url, d.Secret, d.EventType, d.Payload)
	}

	if len(deliveries) > 0 {
		s.logger.Info("processed pending webhook deliveries", "count", len(deliveries))
	}

	return nil
}

// signPayload creates an HMAC-SHA256 signature for the given payload and secret.
func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// containsEvent checks if the events list includes the given event or a wildcard.
func containsEvent(events []string, event string) bool {
	for _, e := range events {
		if e == event || e == "*" {
			return true
		}
		// Support prefix wildcards like "order.*"
		if strings.HasSuffix(e, ".*") {
			prefix := strings.TrimSuffix(e, ".*")
			if strings.HasPrefix(event, prefix+".") {
				return true
			}
		}
	}
	return false
}

package webhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/webhook"
	"github.com/forgecommerce/api/internal/testutil"
)

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	db, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer db.Close()
	testDB = db

	code = m.Run()
}

func newService() *webhook.Service {
	return webhook.NewService(testDB.Pool, nil)
}

// --------------------------------------------------------------------------
// CreateEndpoint
// --------------------------------------------------------------------------

func TestCreateEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/webhook",
		"my-secret",
		"",
		[]string{"order.created", "order.updated"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if ep.Url != "https://example.com/webhook" {
		t.Errorf("url: got %q, want %q", ep.Url, "https://example.com/webhook")
	}
	if ep.Secret != "my-secret" {
		t.Errorf("secret: got %q, want %q", ep.Secret, "my-secret")
	}
	if len(ep.Events) != 2 {
		t.Errorf("events count: got %d, want 2", len(ep.Events))
	}
	if !ep.IsActive {
		t.Error("expected is_active=true")
	}
	if ep.Description != nil {
		t.Errorf("description: got %v, want nil", ep.Description)
	}
	if ep.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	if ep.UpdatedAt.IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

func TestCreateEndpoint_WithDescription(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/hook",
		"secret-123",
		"Order notifications",
		[]string{"order.*"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.Description == nil {
		t.Fatal("expected non-nil description")
	}
	if *ep.Description != "Order notifications" {
		t.Errorf("description: got %q, want %q", *ep.Description, "Order notifications")
	}
}

func TestCreateEndpoint_Inactive(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/disabled",
		"sec",
		"",
		[]string{"*"},
		false,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.IsActive {
		t.Error("expected is_active=false")
	}
}

// --------------------------------------------------------------------------
// GetEndpoint
// --------------------------------------------------------------------------

func TestGetEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.CreateEndpoint(ctx,
		"https://example.com/get-test",
		"secret",
		"test endpoint",
		[]string{"product.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	got, err := svc.GetEndpoint(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("id: got %s, want %s", got.ID, created.ID)
	}
	if got.Url != "https://example.com/get-test" {
		t.Errorf("url: got %q, want %q", got.Url, "https://example.com/get-test")
	}
	if got.Secret != "secret" {
		t.Errorf("secret: got %q, want %q", got.Secret, "secret")
	}
	if len(got.Events) != 1 || got.Events[0] != "product.created" {
		t.Errorf("events: got %v, want [product.created]", got.Events)
	}
	if got.Description == nil || *got.Description != "test endpoint" {
		t.Errorf("description: got %v, want %q", got.Description, "test endpoint")
	}
}

func TestGetEndpoint_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetEndpoint(ctx, uuid.New())
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// ListEndpoints
// --------------------------------------------------------------------------

func TestListEndpoints(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create two endpoints.
	_, err := svc.CreateEndpoint(ctx, "https://example.com/a", "s1", "", []string{"order.created"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint a: %v", err)
	}
	_, err = svc.CreateEndpoint(ctx, "https://example.com/b", "s2", "second", []string{"*"}, false)
	if err != nil {
		t.Fatalf("CreateEndpoint b: %v", err)
	}

	endpoints, err := svc.ListEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(endpoints) != 2 {
		t.Errorf("count: got %d, want 2", len(endpoints))
	}

	// Verify both URLs are present (order is created_at DESC).
	urls := map[string]bool{}
	for _, ep := range endpoints {
		urls[ep.Url] = true
	}
	if !urls["https://example.com/a"] || !urls["https://example.com/b"] {
		t.Errorf("expected both URLs present, got %v", urls)
	}
}

func TestListEndpoints_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	endpoints, err := svc.ListEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("count: got %d, want 0", len(endpoints))
	}
}

// --------------------------------------------------------------------------
// UpdateEndpoint
// --------------------------------------------------------------------------

func TestUpdateEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.CreateEndpoint(ctx,
		"https://example.com/original",
		"secret",
		"original desc",
		[]string{"order.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	updated, err := svc.UpdateEndpoint(ctx, created.ID,
		"https://example.com/updated",
		"updated desc",
		[]string{"order.created", "product.created"},
		false,
	)
	if err != nil {
		t.Fatalf("UpdateEndpoint: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("id: got %s, want %s", updated.ID, created.ID)
	}
	if updated.Url != "https://example.com/updated" {
		t.Errorf("url: got %q, want %q", updated.Url, "https://example.com/updated")
	}
	if len(updated.Events) != 2 {
		t.Errorf("events count: got %d, want 2", len(updated.Events))
	}
	if updated.IsActive {
		t.Error("expected is_active=false after update")
	}
	if updated.Description == nil || *updated.Description != "updated desc" {
		t.Errorf("description: got %v, want %q", updated.Description, "updated desc")
	}
	// Secret should remain unchanged (UpdateEndpoint does not change secret).
	if updated.Secret != "secret" {
		t.Errorf("secret: got %q, want %q (unchanged)", updated.Secret, "secret")
	}
	// updated_at should be after created_at (or at least equal with fast execution).
	if updated.UpdatedAt.Before(created.CreatedAt) {
		t.Errorf("updated_at (%v) should not be before created_at (%v)", updated.UpdatedAt, created.CreatedAt)
	}
}

func TestUpdateEndpoint_ClearDescription(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.CreateEndpoint(ctx,
		"https://example.com/desc",
		"sec",
		"has a description",
		[]string{"*"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Pass empty description to clear it.
	updated, err := svc.UpdateEndpoint(ctx, created.ID,
		"https://example.com/desc",
		"",
		[]string{"*"},
		true,
	)
	if err != nil {
		t.Fatalf("UpdateEndpoint: %v", err)
	}
	if updated.Description != nil {
		t.Errorf("description: got %v, want nil", updated.Description)
	}
}

func TestUpdateEndpoint_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateEndpoint(ctx, uuid.New(),
		"https://example.com/nope",
		"desc",
		[]string{"order.created"},
		true,
	)
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// DeleteEndpoint
// --------------------------------------------------------------------------

func TestDeleteEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.CreateEndpoint(ctx,
		"https://example.com/delete-me",
		"secret",
		"",
		[]string{"*"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	err = svc.DeleteEndpoint(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}

	// Verify it is gone.
	_, err = svc.GetEndpoint(ctx, created.ID)
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// Verify list no longer contains it.
	endpoints, err := svc.ListEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(endpoints) != 0 {
		t.Errorf("count: got %d, want 0 after delete", len(endpoints))
	}
}

func TestDeleteEndpoint_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.DeleteEndpoint(ctx, uuid.New())
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// ListDeliveries
// --------------------------------------------------------------------------

func TestListDeliveries(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create an endpoint first.
	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/deliveries",
		"secret",
		"",
		[]string{"order.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Insert deliveries directly via SQL.
	payload1, _ := json.Marshal(map[string]string{"order_id": "aaa"})
	payload2, _ := json.Marshal(map[string]string{"order_id": "bbb"})
	payload3, _ := json.Marshal(map[string]string{"order_id": "ccc"})

	for _, p := range [][]byte{payload1, payload2, payload3} {
		_, err := testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep.ID, "order.created", p,
		)
		if err != nil {
			t.Fatalf("inserting delivery: %v", err)
		}
	}

	// List all 3 deliveries.
	deliveries, err := svc.ListDeliveries(ctx, ep.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 3 {
		t.Errorf("count: got %d, want 3", len(deliveries))
	}

	// Verify all belong to the correct endpoint.
	for i, d := range deliveries {
		if d.EndpointID != ep.ID {
			t.Errorf("delivery[%d].endpoint_id: got %s, want %s", i, d.EndpointID, ep.ID)
		}
		if d.EventType != "order.created" {
			t.Errorf("delivery[%d].event_type: got %q, want %q", i, d.EventType, "order.created")
		}
	}
}

func TestListDeliveries_WithLimitOffset(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/paged",
		"secret",
		"",
		[]string{"*"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Insert 5 deliveries.
	for i := 0; i < 5; i++ {
		payload, _ := json.Marshal(map[string]int{"idx": i})
		_, err := testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep.ID, "product.created", payload,
		)
		if err != nil {
			t.Fatalf("inserting delivery %d: %v", i, err)
		}
	}

	// Page 1: limit 2, offset 0.
	page1, err := svc.ListDeliveries(ctx, ep.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListDeliveries page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 count: got %d, want 2", len(page1))
	}

	// Page 2: limit 2, offset 2.
	page2, err := svc.ListDeliveries(ctx, ep.ID, 2, 2)
	if err != nil {
		t.Fatalf("ListDeliveries page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 count: got %d, want 2", len(page2))
	}

	// Page 3: limit 2, offset 4 -> only 1 remaining.
	page3, err := svc.ListDeliveries(ctx, ep.ID, 2, 4)
	if err != nil {
		t.Fatalf("ListDeliveries page3: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("page3 count: got %d, want 1", len(page3))
	}

	// Verify no overlap between pages (IDs should be unique).
	seen := map[uuid.UUID]bool{}
	for _, d := range page1 {
		seen[d.ID] = true
	}
	for _, d := range page2 {
		if seen[d.ID] {
			t.Errorf("page2 delivery %s also appeared in page1", d.ID)
		}
		seen[d.ID] = true
	}
	for _, d := range page3 {
		if seen[d.ID] {
			t.Errorf("page3 delivery %s also appeared in earlier pages", d.ID)
		}
	}
}

func TestListDeliveries_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/no-deliveries",
		"secret",
		"",
		[]string{"*"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	deliveries, err := svc.ListDeliveries(ctx, ep.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("count: got %d, want 0", len(deliveries))
	}
}

func TestListDeliveries_DifferentEndpoints(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep1, err := svc.CreateEndpoint(ctx, "https://example.com/ep1", "s1", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint ep1: %v", err)
	}
	ep2, err := svc.CreateEndpoint(ctx, "https://example.com/ep2", "s2", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint ep2: %v", err)
	}

	// Insert 2 deliveries for ep1, 3 for ep2.
	for i := 0; i < 2; i++ {
		payload, _ := json.Marshal(map[string]int{"idx": i})
		testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep1.ID, "order.created", payload,
		)
	}
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]int{"idx": i})
		testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep2.ID, "product.updated", payload,
		)
	}

	// ep1 should only see its 2 deliveries.
	d1, err := svc.ListDeliveries(ctx, ep1.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries ep1: %v", err)
	}
	if len(d1) != 2 {
		t.Errorf("ep1 count: got %d, want 2", len(d1))
	}

	// ep2 should only see its 3 deliveries.
	d2, err := svc.ListDeliveries(ctx, ep2.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries ep2: %v", err)
	}
	if len(d2) != 3 {
		t.Errorf("ep2 count: got %d, want 3", len(d2))
	}
}

// --------------------------------------------------------------------------
// CountDeliveries
// --------------------------------------------------------------------------

func TestCountDeliveries(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep, err := svc.CreateEndpoint(ctx,
		"https://example.com/count",
		"secret",
		"",
		[]string{"order.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Initially zero.
	count, err := svc.CountDeliveries(ctx, ep.ID)
	if err != nil {
		t.Fatalf("CountDeliveries: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}

	// Insert 3 deliveries.
	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]string{"event": "test"})
		_, err := testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep.ID, "order.created", payload,
		)
		if err != nil {
			t.Fatalf("inserting delivery %d: %v", i, err)
		}
	}

	count, err = svc.CountDeliveries(ctx, ep.ID)
	if err != nil {
		t.Fatalf("CountDeliveries: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}
}

func TestCountDeliveries_OnlyCountsOwnEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	ep1, err := svc.CreateEndpoint(ctx, "https://example.com/c1", "s1", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint ep1: %v", err)
	}
	ep2, err := svc.CreateEndpoint(ctx, "https://example.com/c2", "s2", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint ep2: %v", err)
	}

	// 2 deliveries for ep1, 5 for ep2.
	for i := 0; i < 2; i++ {
		payload, _ := json.Marshal(map[string]int{"i": i})
		testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep1.ID, "order.created", payload,
		)
	}
	for i := 0; i < 5; i++ {
		payload, _ := json.Marshal(map[string]int{"i": i})
		testDB.Pool.Exec(ctx,
			`INSERT INTO webhook_deliveries (endpoint_id, event_type, payload) VALUES ($1, $2, $3)`,
			ep2.ID, "product.updated", payload,
		)
	}

	count1, err := svc.CountDeliveries(ctx, ep1.ID)
	if err != nil {
		t.Fatalf("CountDeliveries ep1: %v", err)
	}
	if count1 != 2 {
		t.Errorf("ep1 count: got %d, want 2", count1)
	}

	count2, err := svc.CountDeliveries(ctx, ep2.ID)
	if err != nil {
		t.Fatalf("CountDeliveries ep2: %v", err)
	}
	if count2 != 5 {
		t.Errorf("ep2 count: got %d, want 5", count2)
	}
}

func TestCountDeliveries_NonexistentEndpoint(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Counting for a UUID with no deliveries should return 0, not error.
	count, err := svc.CountDeliveries(ctx, uuid.New())
	if err != nil {
		t.Fatalf("CountDeliveries: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

// --------------------------------------------------------------------------
// Dispatch
// --------------------------------------------------------------------------

func TestDispatch_DeliversToMatchingEndpoint(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	// Set up a mock webhook receiver.
	var received atomic.Int32
	var receivedEvent string
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		receivedEvent = r.Header.Get("X-Webhook-Event")
		receivedSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	// Create an endpoint pointing to our mock server.
	_, err := svc.CreateEndpoint(ctx,
		server.URL,
		"test-secret",
		"",
		[]string{"order.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Dispatch an event.
	svc.Dispatch(ctx, "order.created", map[string]string{"order_id": "123"})

	// Wait for the async delivery (Dispatch runs in a goroutine).
	deadline := time.After(5 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook delivery")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if receivedEvent != "order.created" {
		t.Errorf("X-Webhook-Event: got %q, want %q", receivedEvent, "order.created")
	}
	if receivedSig == "" {
		t.Error("expected X-Webhook-Signature header (secret was set)")
	}
}

func TestDispatch_SkipsNonMatchingEndpoints(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	// Create an endpoint that only subscribes to "product.created".
	_, err := svc.CreateEndpoint(ctx,
		server.URL,
		"secret",
		"",
		[]string{"product.created"},
		true,
	)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Dispatch an "order.created" event (should NOT match).
	svc.Dispatch(ctx, "order.created", map[string]string{"order_id": "456"})

	// Wait a short time to verify no delivery happens.
	time.Sleep(500 * time.Millisecond)

	if received.Load() > 0 {
		t.Errorf("endpoint should not receive non-matching event, got %d deliveries", received.Load())
	}
}

func TestDispatch_WildcardEndpoint(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	// Create an endpoint with wildcard "*".
	_, err := svc.CreateEndpoint(ctx, server.URL, "secret", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	svc.Dispatch(ctx, "order.created", map[string]string{"test": "wildcard"})

	deadline := time.After(5 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for wildcard webhook delivery")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if received.Load() != 1 {
		t.Errorf("expected 1 delivery for wildcard, got %d", received.Load())
	}
}

func TestDispatch_InactiveEndpointSkipped(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	// Create an inactive endpoint.
	_, err := svc.CreateEndpoint(ctx, server.URL, "secret", "", []string{"*"}, false)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	svc.Dispatch(ctx, "order.created", map[string]string{"test": "inactive"})

	time.Sleep(500 * time.Millisecond)
	if received.Load() > 0 {
		t.Errorf("inactive endpoint should not receive events, got %d", received.Load())
	}
}

// --------------------------------------------------------------------------
// ProcessPendingDeliveries
// --------------------------------------------------------------------------

func TestProcessPendingDeliveries_RetriesPending(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	// Create an endpoint.
	ep, err := svc.CreateEndpoint(ctx, server.URL, "secret", "", []string{"order.created"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Insert a pending delivery directly (delivered_at NULL = pending, next_retry_at in the past).
	payload, _ := json.Marshal(map[string]string{"order_id": "retry-me"})
	_, err = testDB.Pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (endpoint_id, event_type, payload, next_retry_at)
		VALUES ($1, $2, $3, NOW() - INTERVAL '1 minute')
	`, ep.ID, "order.created", payload)
	if err != nil {
		t.Fatalf("inserting pending delivery: %v", err)
	}

	err = svc.ProcessPendingDeliveries(ctx)
	if err != nil {
		t.Fatalf("ProcessPendingDeliveries: %v", err)
	}

	// Wait briefly for the delivery to complete.
	deadline := time.After(5 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pending delivery retry")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if received.Load() != 1 {
		t.Errorf("expected 1 retry delivery, got %d", received.Load())
	}
}

func TestProcessPendingDeliveries_NoPending(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.ProcessPendingDeliveries(ctx)
	if err != nil {
		t.Fatalf("ProcessPendingDeliveries with nothing pending: %v", err)
	}
}

// --------------------------------------------------------------------------
// Dispatch — delivery HTTP error triggers retry scheduling
// --------------------------------------------------------------------------

func TestDispatch_HTTPErrorSchedulesRetry(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server down"}`))
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)

	ep, err := svc.CreateEndpoint(ctx, server.URL, "secret-key", "Error-test endpoint", []string{"order.created"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	payload := json.RawMessage(`{"order_id":"fail-test"}`)
	svc.Dispatch(ctx, "order.created", payload)

	// Wait for the delivery attempt.
	deadline := time.After(5 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for delivery attempt")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// The delivery should have been recorded with the error status.
	deliveries, err := svc.ListDeliveries(ctx, ep.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	d := deliveries[0]
	if d.DeliveredAt.Valid {
		t.Error("delivery should NOT be marked as delivered for HTTP 500")
	}
	if d.ResponseStatus == nil || *d.ResponseStatus != 500 {
		t.Errorf("response_status: want 500, got %v", d.ResponseStatus)
	}
	if !d.NextRetryAt.Valid {
		t.Error("next_retry_at should be set for failed delivery")
	}
}

// --------------------------------------------------------------------------
// Dispatch — unreachable URL triggers markFailed
// --------------------------------------------------------------------------

func TestDispatch_UnreachableURLMarksDeliveryFailed(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	// Use a URL that will refuse connections immediately.
	svc := webhook.NewService(testDB.Pool, nil)

	ep, err := svc.CreateEndpoint(ctx, "http://127.0.0.1:1", "secret", "Unreachable", []string{"order.created"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	payload := json.RawMessage(`{"order_id":"unreachable-test"}`)
	svc.Dispatch(ctx, "order.created", payload)

	// Wait a bit for the goroutine to attempt delivery and fail.
	time.Sleep(2 * time.Second)

	deliveries, err := svc.ListDeliveries(ctx, ep.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	d := deliveries[0]
	if d.DeliveredAt.Valid {
		t.Error("delivery should NOT be marked as delivered for unreachable URL")
	}
	// markFailed stores the error message in response_body
	if d.ResponseBody == nil || *d.ResponseBody == "" {
		t.Error("response_body should contain the error message")
	}
	if !d.NextRetryAt.Valid {
		t.Error("next_retry_at should be set for failed delivery")
	}
}

// --------------------------------------------------------------------------
// Dispatch — delivery sets correct headers
// --------------------------------------------------------------------------

func TestDispatch_SetsCorrectHeaders(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var capturedHeaders http.Header
	var capturedBody []byte
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		capturedBody, _ = json.Marshal(r.Header)
		_ = capturedBody
		body, _ := json.Marshal(map[string]string{"ok": "true"})
		_ = body
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)
	_, err := svc.CreateEndpoint(ctx, server.URL, "my-secret-123", "", []string{"product.updated"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	payload := json.RawMessage(`{"product_id":"p-123"}`)
	svc.Dispatch(ctx, "product.updated", payload)

	deadline := time.After(5 * time.Second)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for delivery")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if capturedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", capturedHeaders.Get("Content-Type"))
	}
	if capturedHeaders.Get("X-Webhook-Event") != "product.updated" {
		t.Errorf("X-Webhook-Event: got %q, want product.updated", capturedHeaders.Get("X-Webhook-Event"))
	}
	if capturedHeaders.Get("X-Webhook-Delivery") == "" {
		t.Error("X-Webhook-Delivery header should be set")
	}
	if capturedHeaders.Get("X-Webhook-Signature") == "" {
		t.Error("X-Webhook-Signature should be set when secret is configured")
	}
}

// --------------------------------------------------------------------------
// ListDeliveries and CountDeliveries
// --------------------------------------------------------------------------

func TestCountDeliveries_Empty(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()
	svc := newService()

	ep, err := svc.CreateEndpoint(ctx, "https://example.com/webhook", "secret", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	count, err := svc.CountDeliveries(ctx, ep.ID)
	if err != nil {
		t.Fatalf("CountDeliveries: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestCountDeliveries_WithDeliveries(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := webhook.NewService(testDB.Pool, nil)
	ep, err := svc.CreateEndpoint(ctx, server.URL, "sec", "", []string{"*"}, true)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	// Dispatch two events.
	svc.Dispatch(ctx, "order.created", json.RawMessage(`{"id":"1"}`))
	svc.Dispatch(ctx, "order.updated", json.RawMessage(`{"id":"2"}`))

	deadline := time.After(5 * time.Second)
	for received.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for deliveries, got %d", received.Load())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	count, err := svc.CountDeliveries(ctx, ep.ID)
	if err != nil {
		t.Fatalf("CountDeliveries: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deliveries, got %d", count)
	}
}

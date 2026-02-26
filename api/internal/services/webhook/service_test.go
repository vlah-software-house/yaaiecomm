package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// --------------------------------------------------------------------------
// Tests for containsEvent
// --------------------------------------------------------------------------

func TestContainsEvent(t *testing.T) {
	tests := []struct {
		name   string
		events []string
		event  string
		want   bool
	}{
		{
			name:   "exact match",
			events: []string{"order.created", "order.updated"},
			event:  "order.created",
			want:   true,
		},
		{
			name:   "no match",
			events: []string{"order.created", "order.updated"},
			event:  "product.created",
			want:   false,
		},
		{
			name:   "wildcard * matches everything",
			events: []string{"*"},
			event:  "order.created",
			want:   true,
		},
		{
			name:   "wildcard * matches product events too",
			events: []string{"*"},
			event:  "product.deleted",
			want:   true,
		},
		{
			name:   "prefix wildcard order.* matches order.created",
			events: []string{"order.*"},
			event:  "order.created",
			want:   true,
		},
		{
			name:   "prefix wildcard order.* matches order.updated",
			events: []string{"order.*"},
			event:  "order.updated",
			want:   true,
		},
		{
			name:   "prefix wildcard order.* does not match product.created",
			events: []string{"order.*"},
			event:  "product.created",
			want:   false,
		},
		{
			name:   "prefix wildcard product.* matches product.deleted",
			events: []string{"product.*"},
			event:  "product.deleted",
			want:   true,
		},
		{
			name:   "empty events list",
			events: nil,
			event:  "order.created",
			want:   false,
		},
		{
			name:   "empty event string",
			events: []string{"order.created"},
			event:  "",
			want:   false,
		},
		{
			name:   "multiple wildcards with exact",
			events: []string{"order.*", "product.created"},
			event:  "order.completed",
			want:   true,
		},
		{
			name:   "stock.low exact match",
			events: []string{"stock.low"},
			event:  EventStockLow,
			want:   true,
		},
		{
			name:   "prefix wildcard order.* does not match orderfoo (no dot)",
			events: []string{"order.*"},
			event:  "orderfoo",
			want:   false,
		},
		{
			name:   "prefix wildcard order.* does not match orderfoo.bar",
			events: []string{"order.*"},
			event:  "orderfoo.bar",
			want:   false,
		},
		{
			name:   "prefix wildcard order.* does not match bare order",
			events: []string{"order.*"},
			event:  "order",
			want:   false,
		},
		{
			name:   "multiple events no match",
			events: []string{"order.created", "product.updated", "stock.low"},
			event:  "order.updated",
			want:   false,
		},
		{
			name:   "nil events list",
			events: nil,
			event:  "anything",
			want:   false,
		},
		{
			name:   "empty slice events list",
			events: []string{},
			event:  "anything",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsEvent(tt.events, tt.event)
			if got != tt.want {
				t.Errorf("containsEvent(%v, %q) = %v, want %v", tt.events, tt.event, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for signPayload
// --------------------------------------------------------------------------

func TestSignPayload_Correctness(t *testing.T) {
	payload := []byte(`{"order_id":"123","total":42.50}`)
	secret := "webhook-secret-key"

	// Compute expected signature manually.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	got := signPayload(payload, secret)
	if got != expected {
		t.Errorf("signPayload() = %q, want %q", got, expected)
	}
}

func TestSignPayload_HasPrefix(t *testing.T) {
	sig := signPayload([]byte("test"), "secret")
	if len(sig) < 7 || sig[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix, got %q", sig)
	}
}

func TestSignPayload_DifferentPayloads(t *testing.T) {
	secret := "same-secret"
	sig1 := signPayload([]byte("payload1"), secret)
	sig2 := signPayload([]byte("payload2"), secret)

	if sig1 == sig2 {
		t.Error("different payloads should produce different signatures")
	}
}

func TestSignPayload_DifferentSecrets(t *testing.T) {
	payload := []byte("same-payload")
	sig1 := signPayload(payload, "secret1")
	sig2 := signPayload(payload, "secret2")

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestSignPayload_EmptyPayload(t *testing.T) {
	sig := signPayload([]byte{}, "secret")
	if len(sig) < 7 {
		t.Errorf("expected valid signature even for empty payload, got %q", sig)
	}
}

func TestSignPayload_Deterministic(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "my-secret"

	sig1 := signPayload(payload, secret)
	sig2 := signPayload(payload, secret)

	if sig1 != sig2 {
		t.Error("same inputs should produce identical signatures")
	}
}

func TestSignPayload_HexLength(t *testing.T) {
	sig := signPayload([]byte(`{"test":true}`), "secret")

	// "sha256=" prefix (7 chars) + 64 hex chars = 71 total.
	if len(sig) != 71 {
		t.Errorf("signature total length: got %d, want 71", len(sig))
	}

	hexPart := sig[7:]
	if _, err := hex.DecodeString(hexPart); err != nil {
		t.Errorf("hex portion is not valid hex: %v", err)
	}
}

func TestSignPayload_EmptySecret(t *testing.T) {
	sig := signPayload([]byte("payload"), "")

	if len(sig) < 7 || sig[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix even with empty secret, got %q", sig)
	}

	// Verify correctness with empty key.
	mac := hmac.New(sha256.New, []byte(""))
	mac.Write([]byte("payload"))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != expected {
		t.Errorf("empty secret signature mismatch:\n  got:  %s\n  want: %s", sig, expected)
	}
}

func TestSignPayload_KnownVector(t *testing.T) {
	// Known test vector: HMAC-SHA256 of "hello" with key "key".
	payload := []byte("hello")
	secret := "key"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	got := signPayload(payload, secret)
	want := "sha256=" + expectedHex

	if got != want {
		t.Errorf("signPayload(hello, key) = %q, want %q", got, want)
	}
}

// --------------------------------------------------------------------------
// Tests for event constants
// --------------------------------------------------------------------------

func TestEventConstants(t *testing.T) {
	events := map[string]string{
		"EventOrderCreated":   EventOrderCreated,
		"EventOrderUpdated":   EventOrderUpdated,
		"EventOrderCompleted": EventOrderCompleted,
		"EventProductCreated": EventProductCreated,
		"EventProductUpdated": EventProductUpdated,
		"EventProductDeleted": EventProductDeleted,
		"EventStockLow":       EventStockLow,
	}

	for name, value := range events {
		if value == "" {
			t.Errorf("event constant %s should not be empty", name)
		}
	}

	// Verify specific values match the expected format "domain.action".
	if EventOrderCreated != "order.created" {
		t.Errorf("EventOrderCreated = %q, want %q", EventOrderCreated, "order.created")
	}
	if EventOrderUpdated != "order.updated" {
		t.Errorf("EventOrderUpdated = %q, want %q", EventOrderUpdated, "order.updated")
	}
	if EventOrderCompleted != "order.completed" {
		t.Errorf("EventOrderCompleted = %q, want %q", EventOrderCompleted, "order.completed")
	}
	if EventProductCreated != "product.created" {
		t.Errorf("EventProductCreated = %q, want %q", EventProductCreated, "product.created")
	}
	if EventProductUpdated != "product.updated" {
		t.Errorf("EventProductUpdated = %q, want %q", EventProductUpdated, "product.updated")
	}
	if EventProductDeleted != "product.deleted" {
		t.Errorf("EventProductDeleted = %q, want %q", EventProductDeleted, "product.deleted")
	}
	if EventStockLow != "stock.low" {
		t.Errorf("EventStockLow = %q, want %q", EventStockLow, "stock.low")
	}
}

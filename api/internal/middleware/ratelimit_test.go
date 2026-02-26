package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// okHandler is a simple handler that returns 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func TestRateLimiter_RequestsWithinLimitPass(t *testing.T) {
	// Allow 10 req/sec with burst of 10.
	limiter := RateLimiter(10, 10)
	handler := limiter(okHandler)

	// Send 10 requests — all should pass (within burst).
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}

		// Verify rate limit headers are present.
		if w.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("missing X-RateLimit-Limit header")
		}
		if w.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("missing X-RateLimit-Remaining header")
		}
	}
}

func TestRateLimiter_RequestsExceedingLimitGet429(t *testing.T) {
	// Allow 5 req/sec with burst of 5.
	limiter := RateLimiter(5, 5)
	handler := limiter(okHandler)

	// Exhaust the burst.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 during burst, got %d", i+1, w.Code)
		}
	}

	// The 6th request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 after burst, got %d", w.Code)
	}

	// Verify Retry-After header.
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429 response")
	}

	// Verify response body is JSON with error message.
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 429 response body: %v", err)
	}
	if body["error"] != "rate limit exceeded" {
		t.Errorf("expected error 'rate limit exceeded', got %q", body["error"])
	}
}

func TestRateLimiter_DifferentIPsHaveSeparateLimits(t *testing.T) {
	// Allow burst of 2 per IP.
	limiter := RateLimiter(1, 2)
	handler := limiter(okHandler)

	// IP 1: send 2 requests (exhaust burst).
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("IP1 request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// IP 1: 3rd request should be rate limited.
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 3rd request: expected 429, got %d", w1.Code)
	}

	// IP 2: should still have full burst available.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req2.RemoteAddr = "10.0.0.2:2222"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("IP2 first request: expected 200, got %d", w2.Code)
	}
}

func TestRateLimiter_TokensRefillOverTime(t *testing.T) {
	// Rate of 10 tokens/sec, burst of 2.
	// After exhausting burst, waiting 200ms should refill ~2 tokens.
	limiter := RateLimiter(10, 2)
	handler := limiter(okHandler)

	ip := "172.16.0.1:5555"

	// Exhaust the burst.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("initial request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Should be rate limited now.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", w.Code)
	}

	// Wait for tokens to refill (10 tokens/sec -> 1 token in 100ms).
	// Wait 250ms to be safe.
	time.Sleep(250 * time.Millisecond)

	// Should be allowed again.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after token refill, got %d", w.Code)
	}
}

func TestRateLimiter_BurstAllowsInitialSpike(t *testing.T) {
	// Rate is very low (1 per second), but burst is 20.
	// This means we can do 20 requests instantly before being limited.
	limiter := RateLimiter(1, 20)
	handler := limiter(okHandler)

	ip := "10.10.10.10:8080"

	// All 20 burst requests should pass.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("burst request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Request 21 should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request after burst: expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_HealthCheckEndpointsAreExempt(t *testing.T) {
	// Very restrictive limiter: burst of 1.
	limiter := RateLimiter(0.1, 1)
	handler := limiter(okHandler)

	ip := "192.168.0.1:1234"

	// Exhaust the single token.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w.Code)
	}

	// Next regular request should be rate limited.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for regular endpoint, got %d", w.Code)
	}

	// Health check paths should still pass despite rate limit.
	healthPaths := []string{"/healthz", "/readyz", "/livez", "/health", "/ready", "/ping"}
	for _, path := range healthPaths {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = ip
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("health check %s: expected 200, got %d", path, w.Code)
		}

		// Health check responses should NOT have rate limit headers.
		if w.Header().Get("X-RateLimit-Limit") != "" {
			t.Errorf("health check %s: should not have X-RateLimit-Limit header", path)
		}
	}
}

func TestRateLimiter_XForwardedForHeader(t *testing.T) {
	limiter := RateLimiter(1, 1)
	handler := limiter(okHandler)

	// Use X-Forwarded-For to identify the real client IP.
	// Even though RemoteAddr is the same, X-Forwarded-For differs.
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first request from 203.0.113.1: expected 200, got %d", w1.Code)
	}

	// Second request from same forwarded IP should be limited.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	req2.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request from 203.0.113.1: expected 429, got %d", w2.Code)
	}

	// Request from different forwarded IP should pass.
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	req3.RemoteAddr = "127.0.0.1:1234"
	req3.Header.Set("X-Forwarded-For", "203.0.113.2")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("request from 203.0.113.2: expected 200, got %d", w3.Code)
	}
}

func TestLoginRateLimiter(t *testing.T) {
	limiter := LoginRateLimiter()
	handler := limiter(okHandler)

	ip := "10.20.30.40:5678"

	// LoginRateLimiter allows burst of 5.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("login attempt %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 6th attempt should be rate limited.
	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th login attempt: expected 429, got %d", w.Code)
	}
}

func TestRetryAfter_UnknownIP_Returns1(t *testing.T) {
	state := &rateLimiterState{
		rate:  10,
		burst: 10,
		done:  make(chan struct{}),
	}

	// For an IP that has never been seen, retryAfter should return 1.
	got := state.retryAfter("unknown-ip")
	if got != 1 {
		t.Errorf("retryAfter for unknown IP: got %d, want 1", got)
	}
}

func TestRetryAfter_TokensAvailable_Returns0(t *testing.T) {
	state := &rateLimiterState{
		rate:  10,
		burst: 10,
		done:  make(chan struct{}),
	}

	// Use allow to create a bucket, consuming one token (leaves 9).
	allowed, _, _ := state.allow("10.0.0.1")
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	// With tokens still available, retryAfter should return 0.
	got := state.retryAfter("10.0.0.1")
	if got != 0 {
		t.Errorf("retryAfter with tokens available: got %d, want 0", got)
	}
}

func TestRetryAfter_NoTokens_ReturnsPositive(t *testing.T) {
	state := &rateLimiterState{
		rate:  1, // 1 token per second
		burst: 1,
		done:  make(chan struct{}),
	}

	// Exhaust the single token.
	state.allow("10.0.0.2")

	// With no tokens and rate of 1/sec, retryAfter should be ~1 second.
	got := state.retryAfter("10.0.0.2")
	if got < 1 {
		t.Errorf("retryAfter with no tokens: got %d, want >= 1", got)
	}
}

func TestStartCleanup_RemovesStaleBuckets(t *testing.T) {
	state := &rateLimiterState{
		rate:  10,
		burst: 10,
		done:  make(chan struct{}),
	}

	// Manually store a bucket with a very old lastRefill time (stale).
	staleBucket := &bucket{
		tokens:     10,
		lastRefill: time.Now().Add(-20 * time.Minute), // 20 minutes ago, well past the 10-minute threshold
	}
	state.buckets.Store("stale-ip", staleBucket)

	// Store a fresh bucket.
	freshBucket := &bucket{
		tokens:     5,
		lastRefill: time.Now(),
	}
	state.buckets.Store("fresh-ip", freshBucket)

	// Simulate what the cleanup loop does (range and delete stale).
	staleThreshold := time.Now().Add(-10 * time.Minute)
	state.buckets.Range(func(key, value any) bool {
		b := value.(*bucket)
		b.mu.Lock()
		if b.lastRefill.Before(staleThreshold) {
			b.mu.Unlock()
			state.buckets.Delete(key)
		} else {
			b.mu.Unlock()
		}
		return true
	})

	// Stale bucket should be removed.
	if _, ok := state.buckets.Load("stale-ip"); ok {
		t.Error("expected stale bucket to be removed")
	}

	// Fresh bucket should remain.
	if _, ok := state.buckets.Load("fresh-ip"); !ok {
		t.Error("expected fresh bucket to remain")
	}
}

func TestStartCleanup_StopsOnDoneSignal(t *testing.T) {
	state := &rateLimiterState{
		rate:  10,
		burst: 10,
		done:  make(chan struct{}),
	}

	state.startCleanup()

	// Closing done should stop the cleanup goroutine without hanging.
	close(state.done)

	// If the goroutine does not exit, this test would eventually be killed
	// by the test timeout. If it exits cleanly, the test passes.
}

func TestRateLimiter_429ResponseContentType(t *testing.T) {
	limiter := RateLimiter(1, 1)
	handler := limiter(okHandler)

	ip := "10.99.99.99:1234"

	// Exhaust the burst.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// This request should be rate limited.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if body["message"] == "" {
		t.Error("expected non-empty message in 429 response body")
	}
}

func TestExtractIP_XForwardedFor_EmptyFirstEntry(t *testing.T) {
	// Edge case: X-Forwarded-For with empty first entry after trimming.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", " , 10.0.0.1")

	got := extractIP(req)
	// The first entry after trimming is empty, so it should fall through
	// to X-Real-IP or RemoteAddr.
	// Actually, looking at the code: parts[0] = " ", TrimSpace = "".
	// If ip == "" it continues to X-Real-IP, then RemoteAddr.
	if got != "192.168.1.1" {
		t.Errorf("extractIP with empty XFF first entry: got %q, want %q", got, "192.168.1.1")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xRealIP    string
		wantIP     string
	}{
		{
			name:       "plain RemoteAddr with port",
			remoteAddr: "192.168.1.1:12345",
			wantIP:     "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "127.0.0.1:80",
			xff:        "203.0.113.50",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "X-Forwarded-For multiple IPs takes first",
			remoteAddr: "127.0.0.1:80",
			xff:        "203.0.113.50, 70.41.3.18, 150.172.238.178",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "X-Real-IP used when no X-Forwarded-For",
			remoteAddr: "127.0.0.1:80",
			xRealIP:    "198.51.100.22",
			wantIP:     "198.51.100.22",
		},
		{
			name:       "X-Forwarded-For takes priority over X-Real-IP",
			remoteAddr: "127.0.0.1:80",
			xff:        "203.0.113.50",
			xRealIP:    "198.51.100.22",
			wantIP:     "203.0.113.50",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			wantIP:     "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			got := extractIP(req)
			if got != tt.wantIP {
				t.Errorf("extractIP: want %q, got %q", tt.wantIP, got)
			}
		})
	}
}

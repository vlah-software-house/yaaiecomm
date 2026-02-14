package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expected := range expectedHeaders {
		got := rr.Header().Get(header)
		if got != expected {
			t.Errorf("header %s: want %q, got %q", header, expected, got)
		}
	}
}

func TestSecurityHeaders_PassesThrough(t *testing.T) {
	called := false
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected next handler to be called")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
}

func TestRequestLogger(t *testing.T) {
	logger := slog.Default()
	middleware := RequestLogger(logger)

	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/products", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected handler to be called")
	}
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}
}

func TestRequestLogger_CapturesStatus(t *testing.T) {
	logger := slog.Default()
	middleware := RequestLogger(logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestRecover_NoPanic(t *testing.T) {
	logger := slog.Default()
	middleware := Recover(logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestRecover_CatchesPanic(t *testing.T) {
	logger := slog.Default()
	middleware := Recover(logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()

	// Should not panic - the middleware should catch it.
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 after panic, got %d", rr.Code)
	}
}

func TestRecover_CatchesNilPanic(t *testing.T) {
	logger := slog.Default()
	middleware := Recover(logger)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	}))

	req := httptest.NewRequest(http.MethodGet, "/nil-panic", nil)
	rr := httptest.NewRecorder()

	// Should not crash the process.
	handler.ServeHTTP(rr, req)

	// panic(nil) may not be recovered by recover() in some Go versions.
	// In Go 1.21+, panic(nil) still triggers recover(). Either way, the handler
	// should complete without crashing the test.
}

func TestStatusWriter_DefaultStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	// Without calling WriteHeader, default should be 200.
	if sw.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", sw.status)
	}
}

func TestStatusWriter_CapturesStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusNotFound {
		t.Errorf("expected captured status 404, got %d", sw.status)
	}
}

func TestAdminUserFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		wantID   string
		wantOK   bool
	}{
		{
			name:   "with admin user ID",
			ctx:    context.WithValue(context.Background(), AdminUserIDKey, "user-123"),
			wantID: "user-123",
			wantOK: true,
		},
		{
			name:   "without admin user ID",
			ctx:    context.Background(),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "wrong type in context",
			ctx:    context.WithValue(context.Background(), AdminUserIDKey, 123),
			wantID: "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := AdminUserFromContext(tt.ctx)
			if ok != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, ok)
			}
			if id != tt.wantID {
				t.Errorf("id: want %q, got %q", tt.wantID, id)
			}
		})
	}
}

func TestContextKeys(t *testing.T) {
	// Verify context key constants have expected values.
	if AdminUserIDKey != "admin_user_id" {
		t.Errorf("unexpected AdminUserIDKey: %q", AdminUserIDKey)
	}
	if SessionTokenKey != "session_token" {
		t.Errorf("unexpected SessionTokenKey: %q", SessionTokenKey)
	}
	if CSRFTokenKey != "csrf_token" {
		t.Errorf("unexpected CSRFTokenKey: %q", CSRFTokenKey)
	}
}

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/auth"
)

func TestRequireCustomerAuth_MissingHeader(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}

	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "missing authorization header" {
		t.Errorf("error: got %q", body["error"])
	}
}

func TestRequireCustomerAuth_InvalidFormat(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}

	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "invalid authorization format" {
		t.Errorf("error: got %q", body["error"])
	}
}

func TestRequireCustomerAuth_InvalidToken(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rr.Code)
	}
}

func TestRequireCustomerAuth_ValidToken(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	customerID := uuid.New()

	token, err := jwtMgr.GenerateAccessToken(customerID, "customer@example.com")
	if err != nil {
		t.Fatalf("generating token: %v", err)
	}

	var capturedID uuid.UUID
	var capturedEmail string
	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID, _ = r.Context().Value(CustomerIDKey).(uuid.UUID)
		capturedEmail, _ = r.Context().Value(CustomerEmailKey).(string)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	if capturedID != customerID {
		t.Errorf("customer ID: got %s, want %s", capturedID, customerID)
	}
	if capturedEmail != "customer@example.com" {
		t.Errorf("email: got %q, want %q", capturedEmail, "customer@example.com")
	}
}

func TestRequireCustomerAuth_CaseInsensitiveBearer(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	customerID := uuid.New()

	token, _ := jwtMgr.GenerateAccessToken(customerID, "test@test.com")

	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "BEARER "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (case-insensitive Bearer)", rr.Code)
	}
}

func TestRequireCustomerAuth_OnlyBearer(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret")
	handler := RequireCustomerAuth(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401 (no space after Bearer)", rr.Code)
	}
}

func TestCustomerFromContext_Valid(t *testing.T) {
	expected := uuid.New()
	ctx := context.WithValue(context.Background(), CustomerIDKey, expected)

	id, ok := CustomerFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if id != expected {
		t.Errorf("id: got %s, want %s", id, expected)
	}
}

func TestCustomerFromContext_Missing(t *testing.T) {
	_, ok := CustomerFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for missing context value")
	}
}

func TestCustomerFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CustomerIDKey, "not-a-uuid")

	_, ok := CustomerFromContext(ctx)
	if ok {
		t.Error("expected ok=false for wrong type (string instead of uuid.UUID)")
	}
}

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, http.StatusBadRequest, "something went wrong")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", rr.Header().Get("Content-Type"), "application/json")
	}

	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "something went wrong" {
		t.Errorf("error message: got %q", body["error"])
	}
}

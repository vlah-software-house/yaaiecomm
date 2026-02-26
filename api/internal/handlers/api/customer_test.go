package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/customer"
)

const testJWTSecret = "test-secret-key-for-handler-tests-minimum-length"

func newCustomerHandler() *api.CustomerHandler {
	customerSvc := customer.NewService(testDB.Pool, nil)
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	return api.NewCustomerHandler(customerSvc, jwtMgr, slog.Default())
}

func customerMux() *http.ServeMux {
	mux := http.NewServeMux()
	h := newCustomerHandler()
	h.RegisterPublicRoutes(mux)

	// Protected routes need auth middleware wrapping.
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	protected := http.NewServeMux()
	h.RegisterProtectedRoutes(protected)
	mux.Handle("/api/v1/customers/me", middleware.RequireCustomerAuth(jwtMgr)(protected))
	mux.Handle("/api/v1/customers/me/", middleware.RequireCustomerAuth(jwtMgr)(protected))

	return mux
}

// --------------------------------------------------------------------------
// Register
// --------------------------------------------------------------------------

func TestRegister(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"email":    "new@example.com",
		"password": "securepassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Customer     struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"customer"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
	if resp.Customer.Email != "new@example.com" {
		t.Errorf("email: got %q, want %q", resp.Customer.Email, "new@example.com")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"email":    "dup@example.com",
		"password": "securepassword123",
	})

	// First registration.
	rr1 := httptest.NewRecorder()
	mux.ServeHTTP(rr1, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first register: got %d", rr1.Code)
	}

	// Second registration with same email.
	body2, _ := json.Marshal(map[string]string{
		"email":    "dup@example.com",
		"password": "anotherpassword123",
	})
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body2)))

	if rr2.Code != http.StatusConflict {
		t.Errorf("duplicate register: got %d, want %d", rr2.Code, http.StatusConflict)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing email", map[string]string{"password": "securepassword123"}},
		{"missing password", map[string]string{"email": "test@example.com"}},
		{"empty email", map[string]string{"email": "", "password": "securepassword123"}},
		{"empty password", map[string]string{"email": "test@example.com", "password": ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"email":    "short@example.com",
		"password": "short",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// Login
// --------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register first.
	regBody, _ := json.Marshal(map[string]string{
		"email":    "login@example.com",
		"password": "securepassword123",
	})
	regRR := httptest.NewRecorder()
	mux.ServeHTTP(regRR, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(regBody)))
	if regRR.Code != http.StatusCreated {
		t.Fatalf("register: got %d", regRR.Code)
	}

	// Login.
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "login@example.com",
		"password": "securepassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(loginBody)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Customer     struct {
			Email string `json:"email"`
		} `json:"customer"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.Customer.Email != "login@example.com" {
		t.Errorf("email: got %q, want %q", resp.Customer.Email, "login@example.com")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register.
	regBody, _ := json.Marshal(map[string]string{
		"email":    "wrong@example.com",
		"password": "correctpassword123",
	})
	regRR := httptest.NewRecorder()
	mux.ServeHTTP(regRR, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(regBody)))

	// Login with wrong password.
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "wrong@example.com",
		"password": "wrongpassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(loginBody)))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogin_NonexistentEmail(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"email":    "nobody@example.com",
		"password": "somepassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(body)))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// --------------------------------------------------------------------------
// RefreshToken
// --------------------------------------------------------------------------

func TestRefreshToken(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register to get tokens.
	regBody, _ := json.Marshal(map[string]string{
		"email":    "refresh@example.com",
		"password": "securepassword123",
	})
	regRR := httptest.NewRecorder()
	mux.ServeHTTP(regRR, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(regBody)))
	var regResp struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(regRR.Body).Decode(&regResp)

	// Refresh.
	refreshBody, _ := json.Marshal(map[string]string{
		"refresh_token": regResp.RefreshToken,
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(refreshBody)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"refresh_token": "invalid-token",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(body)))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRefreshToken_MissingToken(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(body)))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// GetProfile (protected)
// --------------------------------------------------------------------------

func TestGetProfile(t *testing.T) {
	testDB.Truncate(t)

	customerSvc := customer.NewService(testDB.Pool, nil)
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	h := api.NewCustomerHandler(customerSvc, jwtMgr, slog.Default())

	// Register a customer directly via handler to get a real customer in DB.
	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)

	regBody, _ := json.Marshal(map[string]string{
		"email":    "profile@example.com",
		"password": "securepassword123",
	})
	regRR := httptest.NewRecorder()
	regMux.ServeHTTP(regRR, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(regBody)))
	var regResp struct {
		Customer struct {
			ID string `json:"id"`
		} `json:"customer"`
	}
	json.NewDecoder(regRR.Body).Decode(&regResp)
	customerID, _ := uuid.Parse(regResp.Customer.ID)

	// Build a mux with the protected route and inject customer context manually.
	profileMux := http.NewServeMux()
	h.RegisterProtectedRoutes(profileMux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me", nil)
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, customerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	profileMux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Email string `json:"email"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Email != "profile@example.com" {
		t.Errorf("email: got %q, want %q", resp.Email, "profile@example.com")
	}
}

func TestGetProfile_Unauthenticated(t *testing.T) {
	testDB.Truncate(t)

	customerSvc := customer.NewService(testDB.Pool, nil)
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	h := api.NewCustomerHandler(customerSvc, jwtMgr, slog.Default())

	mux := http.NewServeMux()
	h.RegisterProtectedRoutes(mux)

	// Request without customer context.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// --------------------------------------------------------------------------
// UpdateProfile (protected)
// --------------------------------------------------------------------------

func TestUpdateProfile(t *testing.T) {
	testDB.Truncate(t)

	customerSvc := customer.NewService(testDB.Pool, nil)
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	h := api.NewCustomerHandler(customerSvc, jwtMgr, slog.Default())

	// Register a customer.
	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)

	regBody, _ := json.Marshal(map[string]string{
		"email":    "update@example.com",
		"password": "securepassword123",
	})
	regRR := httptest.NewRecorder()
	regMux.ServeHTTP(regRR, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(regBody)))
	var regResp struct {
		Customer struct {
			ID string `json:"id"`
		} `json:"customer"`
	}
	json.NewDecoder(regRR.Body).Decode(&regResp)
	customerID, _ := uuid.Parse(regResp.Customer.ID)

	// Update profile.
	profileMux := http.NewServeMux()
	h.RegisterProtectedRoutes(profileMux)

	updateBody, _ := json.Marshal(map[string]string{
		"first_name": "Jane",
		"last_name":  "Doe",
		"phone":      "+34612345678",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, customerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	profileMux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		Phone     *string `json:"phone"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.FirstName == nil || *resp.FirstName != "Jane" {
		t.Errorf("first_name: got %v, want %q", resp.FirstName, "Jane")
	}
	if resp.LastName == nil || *resp.LastName != "Doe" {
		t.Errorf("last_name: got %v, want %q", resp.LastName, "Doe")
	}
	if resp.Phone == nil || *resp.Phone != "+34612345678" {
		t.Errorf("phone: got %v, want %q", resp.Phone, "+34612345678")
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// registerCustomerResult holds the response of a successful customer registration.
type registerCustomerResult struct {
	CustomerID   uuid.UUID
	Email        string
	AccessToken  string
	RefreshToken string
}

// registerCustomerViaHandler registers a customer through the HTTP handler and
// returns the parsed response. It calls t.Fatal on failure.
func registerCustomerViaHandler(t *testing.T, mux *http.ServeMux, email, password string) registerCustomerResult {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("registerCustomerViaHandler: status %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Customer     struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"customer"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	id, _ := uuid.Parse(resp.Customer.ID)
	return registerCustomerResult{
		CustomerID:   id,
		Email:        resp.Customer.Email,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}
}

// protectedMux returns a handler mux and the customer handler where
// protected routes are registered WITHOUT middleware -- the caller injects
// the customer context directly. Use for direct handler testing.
func protectedMux(t *testing.T) (*http.ServeMux, *api.CustomerHandler) {
	t.Helper()
	customerSvc := customer.NewService(testDB.Pool, nil)
	jwtMgr := auth.NewJWTManager(testJWTSecret)
	h := api.NewCustomerHandler(customerSvc, jwtMgr, slog.Default())

	mux := http.NewServeMux()
	h.RegisterProtectedRoutes(mux)
	return mux, h
}

// --------------------------------------------------------------------------
// Register -- additional coverage
// --------------------------------------------------------------------------

func TestRegister_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers/register",
		strings.NewReader(`{invalid json`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRegister_WithOptionalNames(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	body, _ := json.Marshal(map[string]string{
		"email":      "named@example.com",
		"password":   "securepassword123",
		"first_name": "Alice",
		"last_name":  "Smith",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		Customer struct {
			Email     string  `json:"email"`
			FirstName *string `json:"first_name"`
			LastName  *string `json:"last_name"`
		} `json:"customer"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Customer.FirstName == nil || *resp.Customer.FirstName != "Alice" {
		t.Errorf("first_name: got %v, want %q", resp.Customer.FirstName, "Alice")
	}
	if resp.Customer.LastName == nil || *resp.Customer.LastName != "Smith" {
		t.Errorf("last_name: got %v, want %q", resp.Customer.LastName, "Smith")
	}
}

func TestRegister_EmailNormalization(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Email with uppercase and leading/trailing whitespace.
	body, _ := json.Marshal(map[string]string{
		"email":    "  TEST@Example.COM  ",
		"password": "securepassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/register", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		Customer struct {
			Email string `json:"email"`
		} `json:"customer"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Customer.Email != "test@example.com" {
		t.Errorf("email: got %q, want %q (should be normalized)", resp.Customer.Email, "test@example.com")
	}
}

// --------------------------------------------------------------------------
// Login -- additional coverage
// --------------------------------------------------------------------------

func TestLogin_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers/login",
		strings.NewReader(`not json`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing email", map[string]string{"password": "securepassword123"}},
		{"missing password", map[string]string{"email": "test@example.com"}},
		{"empty email", map[string]string{"email": "", "password": "securepassword123"}},
		{"empty password", map[string]string{"email": "test@example.com", "password": ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(body)))

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestLogin_ReturnsRefreshToken(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register.
	registerCustomerViaHandler(t, mux, "loginrt@example.com", "securepassword123")

	// Login.
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "loginrt@example.com",
		"password": "securepassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(loginBody)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh_token from login")
	}
	if resp.AccessToken == "" {
		t.Error("expected non-empty access_token from login")
	}
}

func TestLogin_NilPasswordHash(t *testing.T) {
	testDB.Truncate(t)

	// Create a customer directly without a password (simulates social login or guest).
	customerSvc := customer.NewService(testDB.Pool, nil)
	_, err := customerSvc.Create(t.Context(), customer.CreateCustomerParams{
		Email: "nopassword@example.com",
		// PasswordHash intentionally left nil.
	})
	if err != nil {
		t.Fatalf("creating customer without password: %v", err)
	}

	mux := customerMux()
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "nopassword@example.com",
		"password": "anypassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(loginBody)))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d (customer has no password)", rr.Code, http.StatusUnauthorized)
	}
}

func TestLogin_EmailNormalization(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register with lowercase email.
	registerCustomerViaHandler(t, mux, "normalize@example.com", "securepassword123")

	// Login with uppercase email -- should still match.
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "  NORMALIZE@Example.COM ",
		"password": "securepassword123",
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/login", bytes.NewReader(loginBody)))

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d (email normalization should match)", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// RefreshToken -- additional coverage
// --------------------------------------------------------------------------

func TestRefreshToken_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh",
		strings.NewReader(`broken`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRefreshToken_RotatedTokenIsValid(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register to get an initial refresh token.
	reg := registerCustomerViaHandler(t, mux, "refresh2@example.com", "securepassword123")

	// First refresh.
	refreshBody, _ := json.Marshal(map[string]string{
		"refresh_token": reg.RefreshToken,
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(refreshBody)))

	if rr.Code != http.StatusOK {
		t.Fatalf("first refresh: status %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	// The rotated refresh token should itself be usable for another refresh.
	refreshBody2, _ := json.Marshal(map[string]string{
		"refresh_token": resp.RefreshToken,
	})
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(refreshBody2)))

	if rr2.Code != http.StatusOK {
		t.Errorf("second refresh with rotated token: status %d, want %d", rr2.Code, http.StatusOK)
	}
}

func TestRefreshToken_WrongSecret(t *testing.T) {
	testDB.Truncate(t)

	// Generate a token signed with a different secret.
	otherJWT := auth.NewJWTManager("completely-different-secret-key-long-enough")
	badToken, err := otherJWT.GenerateRefreshToken(uuid.New(), "bad@example.com")
	if err != nil {
		t.Fatalf("generating token: %v", err)
	}

	mux := customerMux()
	body, _ := json.Marshal(map[string]string{
		"refresh_token": badToken,
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/customers/refresh", bytes.NewReader(body)))

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// --------------------------------------------------------------------------
// GetProfile -- additional coverage
// --------------------------------------------------------------------------

func TestGetProfile_CustomerNotFound(t *testing.T) {
	testDB.Truncate(t)

	mux, _ := protectedMux(t)

	// Use a UUID that does not exist in the database.
	nonexistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me", nil)
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, nonexistentID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestGetProfile_ViaBearer(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	// Register to get tokens.
	reg := registerCustomerViaHandler(t, mux, "bearer@example.com", "securepassword123")

	// Use the access token in the Authorization header through the full middleware mux.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me", nil)
	req.Header.Set("Authorization", "Bearer "+reg.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Email != "bearer@example.com" {
		t.Errorf("email: got %q, want %q", resp.Email, "bearer@example.com")
	}
	if resp.ID != reg.CustomerID.String() {
		t.Errorf("id: got %q, want %q", resp.ID, reg.CustomerID.String())
	}
}

func TestGetProfile_AllFields(t *testing.T) {
	testDB.Truncate(t)

	// Register, then update the profile to populate all fields, then GET.
	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "allfields@example.com", "securepassword123")

	// Update the profile with all fields.
	updateBody, _ := json.Marshal(map[string]string{
		"first_name": "Bob",
		"last_name":  "Builder",
		"phone":      "+49123456",
		"vat_number": "DE123456789",
	})
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	ctx := context.WithValue(updateReq.Context(), middleware.CustomerIDKey, reg.CustomerID)
	updateReq = updateReq.WithContext(ctx)
	updateRR := httptest.NewRecorder()
	mux.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update: status %d, body: %s", updateRR.Code, updateRR.Body.String())
	}

	// GET the profile and verify all fields.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me", nil)
	ctx = context.WithValue(getReq.Context(), middleware.CustomerIDKey, reg.CustomerID)
	getReq = getReq.WithContext(ctx)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("get: status %d, body: %s", getRR.Code, getRR.Body.String())
	}

	var resp struct {
		ID        string  `json:"id"`
		Email     string  `json:"email"`
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		Phone     *string `json:"phone"`
		VatNumber *string `json:"vat_number"`
	}
	json.NewDecoder(getRR.Body).Decode(&resp)

	if resp.Email != "allfields@example.com" {
		t.Errorf("email: got %q, want %q", resp.Email, "allfields@example.com")
	}
	if resp.FirstName == nil || *resp.FirstName != "Bob" {
		t.Errorf("first_name: got %v, want %q", resp.FirstName, "Bob")
	}
	if resp.LastName == nil || *resp.LastName != "Builder" {
		t.Errorf("last_name: got %v, want %q", resp.LastName, "Builder")
	}
	if resp.Phone == nil || *resp.Phone != "+49123456" {
		t.Errorf("phone: got %v, want %q", resp.Phone, "+49123456")
	}
	if resp.VatNumber == nil || *resp.VatNumber != "DE123456789" {
		t.Errorf("vat_number: got %v, want %q", resp.VatNumber, "DE123456789")
	}
}

// --------------------------------------------------------------------------
// UpdateProfile -- additional coverage
// --------------------------------------------------------------------------

func TestUpdateProfile_Unauthenticated(t *testing.T) {
	testDB.Truncate(t)

	mux, _ := protectedMux(t)

	updateBody, _ := json.Marshal(map[string]string{"first_name": "Jane"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	// No customer context injected.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestUpdateProfile_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)

	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "badjson@example.com", "securepassword123")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me",
		strings.NewReader(`{not valid json`))
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	testDB.Truncate(t)

	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "partial@example.com", "securepassword123")

	// First update: set first_name and last_name.
	body1, _ := json.Marshal(map[string]string{
		"first_name": "Alice",
		"last_name":  "Wonderland",
	})
	req1 := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(body1))
	ctx := context.WithValue(req1.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req1 = req1.WithContext(ctx)
	rr1 := httptest.NewRecorder()
	mux.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first update: status %d, body: %s", rr1.Code, rr1.Body.String())
	}

	// Second update: only set phone (partial update).
	body2, _ := json.Marshal(map[string]string{
		"phone": "+1234567890",
	})
	req2 := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(body2))
	ctx2 := context.WithValue(req2.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req2 = req2.WithContext(ctx2)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("second update: status %d, body: %s", rr2.Code, rr2.Body.String())
	}

	var resp struct {
		Phone *string `json:"phone"`
	}
	json.NewDecoder(rr2.Body).Decode(&resp)

	if resp.Phone == nil || *resp.Phone != "+1234567890" {
		t.Errorf("phone: got %v, want %q", resp.Phone, "+1234567890")
	}
}

func TestUpdateProfile_CustomerNotFound(t *testing.T) {
	testDB.Truncate(t)

	mux, _ := protectedMux(t)

	nonexistentID := uuid.New()
	updateBody, _ := json.Marshal(map[string]string{"first_name": "Ghost"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, nonexistentID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestUpdateProfile_VatNumber(t *testing.T) {
	testDB.Truncate(t)

	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "vat@example.com", "securepassword123")

	updateBody, _ := json.Marshal(map[string]string{
		"vat_number": "ES12345678A",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		VatNumber *string `json:"vat_number"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.VatNumber == nil || *resp.VatNumber != "ES12345678A" {
		t.Errorf("vat_number: got %v, want %q", resp.VatNumber, "ES12345678A")
	}
}

func TestUpdateProfile_ViaBearer(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	reg := registerCustomerViaHandler(t, mux, "updatebearer@example.com", "securepassword123")

	updateBody, _ := json.Marshal(map[string]string{
		"first_name": "Bearer",
		"last_name":  "User",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+reg.AccessToken)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.FirstName == nil || *resp.FirstName != "Bearer" {
		t.Errorf("first_name: got %v, want %q", resp.FirstName, "Bearer")
	}
	if resp.LastName == nil || *resp.LastName != "User" {
		t.Errorf("last_name: got %v, want %q", resp.LastName, "User")
	}
}

func TestUpdateProfile_EmptyBody(t *testing.T) {
	testDB.Truncate(t)

	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "empty@example.com", "securepassword123")

	// Sending an empty JSON object should succeed (no fields to update).
	updateBody, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/customers/me", bytes.NewReader(updateBody))
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// ListOrders (protected)
// --------------------------------------------------------------------------

func TestListOrders_Unauthenticated(t *testing.T) {
	testDB.Truncate(t)

	mux, _ := protectedMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me/orders", nil)
	// No customer context.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestListOrders_Empty(t *testing.T) {
	testDB.Truncate(t)

	mux, h := protectedMux(t)

	regMux := http.NewServeMux()
	h.RegisterPublicRoutes(regMux)
	reg := registerCustomerViaHandler(t, regMux, "orders@example.com", "securepassword123")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me/orders", nil)
	ctx := context.WithValue(req.Context(), middleware.CustomerIDKey, reg.CustomerID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp []json.RawMessage
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp) != 0 {
		t.Errorf("expected 0 orders, got %d", len(resp))
	}
}

func TestListOrders_ViaBearer(t *testing.T) {
	testDB.Truncate(t)
	mux := customerMux()

	reg := registerCustomerViaHandler(t, mux, "ordersbearer@example.com", "securepassword123")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers/me/orders", nil)
	req.Header.Set("Authorization", "Bearer "+reg.AccessToken)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp []json.RawMessage
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp == nil {
		t.Error("expected non-nil response array")
	}
	if len(resp) != 0 {
		t.Errorf("expected 0 orders, got %d", len(resp))
	}
}

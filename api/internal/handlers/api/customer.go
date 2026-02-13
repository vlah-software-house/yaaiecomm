package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/customer"
)

// CustomerHandler handles customer authentication and profile endpoints.
type CustomerHandler struct {
	customerSvc *customer.Service
	jwtMgr      *auth.JWTManager
	logger      *slog.Logger
}

// NewCustomerHandler creates a new customer handler.
func NewCustomerHandler(
	customerSvc *customer.Service,
	jwtMgr *auth.JWTManager,
	logger *slog.Logger,
) *CustomerHandler {
	return &CustomerHandler{
		customerSvc: customerSvc,
		jwtMgr:      jwtMgr,
		logger:      logger,
	}
}

// RegisterPublicRoutes registers unauthenticated customer routes (login, register).
func (h *CustomerHandler) RegisterPublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/customers/register", h.Register)
	mux.HandleFunc("POST /api/v1/customers/login", h.Login)
	mux.HandleFunc("POST /api/v1/customers/refresh", h.RefreshToken)
}

// RegisterProtectedRoutes registers authenticated customer routes (profile, orders).
func (h *CustomerHandler) RegisterProtectedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/customers/me", h.GetProfile)
	mux.HandleFunc("PATCH /api/v1/customers/me", h.UpdateProfile)
	mux.HandleFunc("GET /api/v1/customers/me/orders", h.ListOrders)
}

// --- Request/Response types ---

type registerRequest struct {
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	Customer     customerJSON `json:"customer"`
}

type customerJSON struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
	Phone     *string   `json:"phone"`
	VatNumber *string   `json:"vat_number,omitempty"`
}

type updateProfileRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Phone     *string `json:"phone"`
	VatNumber *string `json:"vat_number"`
}

// --- Handlers ---

// Register handles POST /api/v1/customers/register
func (h *CustomerHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "email and password are required"})
		return
	}

	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "password must be at least 8 characters"})
		return
	}

	// Hash the password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}
	hashStr := string(hash)

	// Create the customer.
	cust, err := h.customerSvc.Create(r.Context(), customer.CreateCustomerParams{
		Email:        req.Email,
		PasswordHash: &hashStr,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
	})
	if err != nil {
		if errors.Is(err, customer.ErrEmailTaken) {
			writeJSON(w, http.StatusConflict, errorJSON{Error: "email address is already registered"})
			return
		}
		h.logger.Error("failed to create customer", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Generate tokens.
	accessToken, err := h.jwtMgr.GenerateAccessToken(cust.ID, cust.Email)
	if err != nil {
		h.logger.Error("failed to generate access token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	refreshToken, err := h.jwtMgr.GenerateRefreshToken(cust.ID, cust.Email)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Customer: customerJSON{
			ID:        cust.ID,
			Email:     cust.Email,
			FirstName: cust.FirstName,
			LastName:  cust.LastName,
			Phone:     cust.Phone,
			VatNumber: cust.VatNumber,
		},
	})
}

// Login handles POST /api/v1/customers/login
func (h *CustomerHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "email and password are required"})
		return
	}

	// Look up customer by email.
	cust, err := h.customerSvc.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, customer.ErrNotFound) {
			writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "invalid email or password"})
			return
		}
		h.logger.Error("failed to look up customer", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Verify password.
	if cust.PasswordHash == nil {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "invalid email or password"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*cust.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "invalid email or password"})
		return
	}

	// Generate tokens.
	accessToken, err := h.jwtMgr.GenerateAccessToken(cust.ID, cust.Email)
	if err != nil {
		h.logger.Error("failed to generate access token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	refreshToken, err := h.jwtMgr.GenerateRefreshToken(cust.ID, cust.Email)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Customer: customerJSON{
			ID:        cust.ID,
			Email:     cust.Email,
			FirstName: cust.FirstName,
			LastName:  cust.LastName,
			Phone:     cust.Phone,
			VatNumber: cust.VatNumber,
		},
	})
}

// RefreshToken handles POST /api/v1/customers/refresh
func (h *CustomerHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "refresh_token is required"})
		return
	}

	// Validate the refresh token.
	claims, err := h.jwtMgr.ValidateToken(req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "invalid or expired refresh token"})
		return
	}

	// Generate a new access token.
	accessToken, err := h.jwtMgr.GenerateAccessToken(claims.CustomerID, claims.Email)
	if err != nil {
		h.logger.Error("failed to generate access token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Generate a new refresh token (rotate).
	refreshToken, err := h.jwtMgr.GenerateRefreshToken(claims.CustomerID, claims.Email)
	if err != nil {
		h.logger.Error("failed to generate refresh token", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// GetProfile handles GET /api/v1/customers/me
func (h *CustomerHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	customerID, ok := middleware.CustomerFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "not authenticated"})
		return
	}

	cust, err := h.customerSvc.Get(r.Context(), customerID)
	if err != nil {
		if errors.Is(err, customer.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "customer not found"})
			return
		}
		h.logger.Error("failed to get customer profile", "error", err, "customer_id", customerID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, customerJSON{
		ID:        cust.ID,
		Email:     cust.Email,
		FirstName: cust.FirstName,
		LastName:  cust.LastName,
		Phone:     cust.Phone,
		VatNumber: cust.VatNumber,
	})
}

// UpdateProfile handles PATCH /api/v1/customers/me
func (h *CustomerHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	customerID, ok := middleware.CustomerFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "not authenticated"})
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	cust, err := h.customerSvc.Update(r.Context(), customerID, customer.UpdateCustomerParams{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		VatNumber: req.VatNumber,
	})
	if err != nil {
		if errors.Is(err, customer.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "customer not found"})
			return
		}
		h.logger.Error("failed to update customer profile", "error", err, "customer_id", customerID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, customerJSON{
		ID:        cust.ID,
		Email:     cust.Email,
		FirstName: cust.FirstName,
		LastName:  cust.LastName,
		Phone:     cust.Phone,
		VatNumber: cust.VatNumber,
	})
}

// ListOrders handles GET /api/v1/customers/me/orders
// Returns the authenticated customer's order history.
func (h *CustomerHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.CustomerFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorJSON{Error: "not authenticated"})
		return
	}

	// TODO: Implement customer order listing.
	// The order service needs a ListByCustomer method.
	writeJSON(w, http.StatusOK, []any{})
}

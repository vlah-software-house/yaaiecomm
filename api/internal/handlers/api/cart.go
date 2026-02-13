package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/cart"
)

// CartHandler holds dependencies for cart API endpoints.
type CartHandler struct {
	cartSvc *cart.Service
	logger  *slog.Logger
}

// NewCartHandler creates a new cart handler.
func NewCartHandler(cartSvc *cart.Service, logger *slog.Logger) *CartHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CartHandler{
		cartSvc: cartSvc,
		logger:  logger,
	}
}

// RegisterRoutes registers all cart API routes on the given mux.
func (h *CartHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/cart", h.CreateCart)
	mux.HandleFunc("GET /api/v1/cart/{id}", h.GetCart)
	mux.HandleFunc("PATCH /api/v1/cart/{id}", h.UpdateCart)
	mux.HandleFunc("POST /api/v1/cart/{id}/items", h.AddItem)
	mux.HandleFunc("PATCH /api/v1/cart/{id}/items/{itemId}", h.UpdateItem)
	mux.HandleFunc("DELETE /api/v1/cart/{id}/items/{itemId}", h.RemoveItem)
}

// --- JSON request/response types ---

type createCartResponse struct {
	ID        uuid.UUID  `json:"id"`
	ExpiresAt string     `json:"expires_at"`
	CreatedAt string     `json:"created_at"`
}

type cartResponse struct {
	ID          uuid.UUID       `json:"id"`
	CustomerID  *uuid.UUID      `json:"customer_id"`
	Email       *string         `json:"email"`
	CountryCode *string         `json:"country_code"`
	VatNumber   *string         `json:"vat_number"`
	CouponCode  *string         `json:"coupon_code"`
	ExpiresAt   string          `json:"expires_at"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Items       []cartItemJSON  `json:"items"`
}

type cartItemJSON struct {
	ID                   uuid.UUID      `json:"id"`
	VariantID            uuid.UUID      `json:"variant_id"`
	Quantity             int32          `json:"quantity"`
	VariantSku           string         `json:"variant_sku"`
	VariantPrice         pgtype.Numeric `json:"variant_price"`
	VariantStock         int32          `json:"variant_stock"`
	VariantWeightGrams   *int32         `json:"variant_weight_grams,omitempty"`
	VariantIsActive      bool           `json:"variant_is_active"`
	ProductID            uuid.UUID      `json:"product_id"`
	ProductName          string         `json:"product_name"`
	ProductSlug          string         `json:"product_slug"`
	ProductBasePrice     pgtype.Numeric `json:"product_base_price"`
	ProductVatCategoryID *uuid.UUID     `json:"product_vat_category_id,omitempty"`
}

type updateCartRequest struct {
	Email       *string `json:"email"`
	CountryCode *string `json:"country_code"`
	VatNumber   *string `json:"vat_number"`
	CouponCode  *string `json:"coupon_code"`
}

type addItemRequest struct {
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int32     `json:"quantity"`
}

type updateItemRequest struct {
	Quantity int32 `json:"quantity"`
}

type cartItemResponse struct {
	ID        uuid.UUID `json:"id"`
	VariantID uuid.UUID `json:"variant_id"`
	Quantity  int32     `json:"quantity"`
}

// --- Handlers ---

// CreateCart handles POST /api/v1/cart
func (h *CartHandler) CreateCart(w http.ResponseWriter, r *http.Request) {
	c, err := h.cartSvc.Create(r.Context())
	if err != nil {
		h.logger.Error("failed to create cart", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, createCartResponse{
		ID:        c.ID,
		ExpiresAt: c.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// GetCart handles GET /api/v1/cart/{id}
func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	cartID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	c, err := h.cartSvc.Get(r.Context(), cartID)
	if err != nil {
		if errors.Is(err, cart.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "cart not found"})
			return
		}
		h.logger.Error("failed to get cart", "error", err, "cart_id", cartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	items, err := h.cartSvc.ListItems(r.Context(), cartID)
	if err != nil {
		h.logger.Error("failed to list cart items", "error", err, "cart_id", cartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	cartItems := make([]cartItemJSON, len(items))
	for i, item := range items {
		cartItems[i] = cartItemJSON{
			ID:                 item.ID,
			VariantID:          item.VariantID,
			Quantity:           item.Quantity,
			VariantSku:         item.VariantSku,
			VariantPrice:       item.VariantPrice,
			VariantStock:       item.VariantStock,
			VariantWeightGrams: item.VariantWeightGrams,
			VariantIsActive:    item.VariantIsActive,
			ProductID:          item.ProductID,
			ProductName:        item.ProductName,
			ProductSlug:        item.ProductSlug,
			ProductBasePrice:   item.ProductBasePrice,
			ProductVatCategoryID: pgtypeUUIDToPtr(item.ProductVatCategoryID),
		}
	}

	resp := cartResponse{
		ID:          c.ID,
		CustomerID:  pgtypeUUIDToPtr(c.CustomerID),
		Email:       c.Email,
		CountryCode: c.CountryCode,
		VatNumber:   c.VatNumber,
		CouponCode:  c.CouponCode,
		ExpiresAt:   c.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		CreatedAt:   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Items:       cartItems,
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateCart handles PATCH /api/v1/cart/{id}
func (h *CartHandler) UpdateCart(w http.ResponseWriter, r *http.Request) {
	cartID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	var req updateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	c, err := h.cartSvc.Update(r.Context(), cartID, cart.UpdateParams{
		Email:       req.Email,
		CountryCode: req.CountryCode,
		VatNumber:   req.VatNumber,
		CouponCode:  req.CouponCode,
	})
	if err != nil {
		if errors.Is(err, cart.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "cart not found"})
			return
		}
		h.logger.Error("failed to update cart", "error", err, "cart_id", cartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         c.ID,
		"updated_at": c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// AddItem handles POST /api/v1/cart/{id}/items
func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	cartID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	if req.Quantity < 1 {
		req.Quantity = 1
	}

	item, err := h.cartSvc.AddItem(r.Context(), cartID, req.VariantID, req.Quantity)
	if err != nil {
		if errors.Is(err, cart.ErrInvalidQuantity) {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "quantity must be at least 1"})
			return
		}
		h.logger.Error("failed to add cart item", "error", err, "cart_id", cartID, "variant_id", req.VariantID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, cartItemResponse{
		ID:        item.ID,
		VariantID: item.VariantID,
		Quantity:  item.Quantity,
	})
}

// UpdateItem handles PATCH /api/v1/cart/{id}/items/{itemId}
func (h *CartHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid item ID"})
		return
	}

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	item, err := h.cartSvc.UpdateItemQuantity(r.Context(), itemID, req.Quantity)
	if err != nil {
		if errors.Is(err, cart.ErrItemNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "cart item not found"})
			return
		}
		if errors.Is(err, cart.ErrInvalidQuantity) {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "quantity must be at least 1"})
			return
		}
		h.logger.Error("failed to update cart item", "error", err, "item_id", itemID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, cartItemResponse{
		ID:        item.ID,
		VariantID: item.VariantID,
		Quantity:  item.Quantity,
	})
}

// RemoveItem handles DELETE /api/v1/cart/{id}/items/{itemId}
func (h *CartHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid item ID"})
		return
	}

	if err := h.cartSvc.RemoveItem(r.Context(), itemID); err != nil {
		h.logger.Error("failed to remove cart item", "error", err, "item_id", itemID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/cart"
	"github.com/forgecommerce/api/internal/vat"
)

// VATNumberHandler handles VAT number validation and cart association.
type VATNumberHandler struct {
	cartSvc    *cart.Service
	viesClient *vat.VIESClient
	logger     *slog.Logger
}

// NewVATNumberHandler creates a new VAT number handler.
func NewVATNumberHandler(cartSvc *cart.Service, viesClient *vat.VIESClient, logger *slog.Logger) *VATNumberHandler {
	return &VATNumberHandler{
		cartSvc:    cartSvc,
		viesClient: viesClient,
		logger:     logger,
	}
}

// RegisterRoutes registers the VAT number validation route.
func (h *VATNumberHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/cart/{id}/vat-number", h.ValidateAndApply)
}

type vatNumberRequest struct {
	VATNumber string `json:"vat_number"`
}

type vatNumberResponse struct {
	Valid       bool   `json:"valid"`
	CompanyName string `json:"company_name,omitempty"`
	Address     string `json:"address,omitempty"`
	VATNumber   string `json:"vat_number"`
	Message     string `json:"message,omitempty"`
}

// ValidateAndApply handles POST /api/v1/cart/{id}/vat-number
// Validates the VAT number via VIES, then stores it on the cart.
func (h *VATNumberHandler) ValidateAndApply(w http.ResponseWriter, r *http.Request) {
	cartID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid cart ID"})
		return
	}

	var req vatNumberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	vatNumber := strings.TrimSpace(strings.ToUpper(req.VATNumber))
	if vatNumber == "" {
		// Clear the VAT number from the cart.
		if _, err := h.cartSvc.Update(r.Context(), cartID, cart.UpdateParams{}); err != nil {
			h.logger.Error("failed to clear VAT number", "error", err, "cart_id", cartID)
			writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
			return
		}
		writeJSON(w, http.StatusOK, vatNumberResponse{
			Valid:   false,
			Message: "VAT number cleared",
		})
		return
	}

	// Validate via VIES.
	result, err := h.viesClient.Validate(r.Context(), vatNumber)
	if err != nil {
		h.logger.Error("VIES validation error", "error", err, "vat_number", vatNumber)
		writeJSON(w, http.StatusServiceUnavailable, errorJSON{Error: "VAT number validation service is currently unavailable"})
		return
	}

	if !result.Valid {
		writeJSON(w, http.StatusOK, vatNumberResponse{
			Valid:     false,
			VATNumber: vatNumber,
			Message:   "VAT number is not valid",
		})
		return
	}

	// Store the validated VAT number on the cart.
	if _, err := h.cartSvc.Update(r.Context(), cartID, cart.UpdateParams{
		VatNumber: &vatNumber,
	}); err != nil {
		h.logger.Error("failed to update cart VAT number", "error", err, "cart_id", cartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, vatNumberResponse{
		Valid:       true,
		CompanyName: result.CompanyName,
		Address:     result.CompanyAddress,
		VATNumber:   vatNumber,
		Message:     "VAT number is valid. Reverse charge may apply at checkout.",
	})
}

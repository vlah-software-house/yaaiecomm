package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	stripe "github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/services/cart"
	"github.com/forgecommerce/api/internal/services/order"
	"github.com/forgecommerce/api/internal/services/shipping"
	"github.com/forgecommerce/api/internal/vat"
)

// CheckoutHandler holds dependencies for checkout API endpoints.
type CheckoutHandler struct {
	cartSvc     *cart.Service
	orderSvc    *order.Service
	vatSvc      *vat.VATService
	shippingSvc *shipping.Service
	queries     *db.Queries
	logger      *slog.Logger
	successURL  string
	cancelURL   string
}

// NewCheckoutHandler creates a new checkout handler with all required dependencies.
func NewCheckoutHandler(
	cartSvc *cart.Service,
	orderSvc *order.Service,
	vatSvc *vat.VATService,
	shippingSvc *shipping.Service,
	queries *db.Queries,
	logger *slog.Logger,
	successURL string,
	cancelURL string,
) *CheckoutHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CheckoutHandler{
		cartSvc:     cartSvc,
		orderSvc:    orderSvc,
		vatSvc:      vatSvc,
		shippingSvc: shippingSvc,
		queries:     queries,
		logger:      logger,
		successURL:  successURL,
		cancelURL:   cancelURL,
	}
}

// RegisterRoutes registers all checkout API routes on the given mux.
func (h *CheckoutHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/checkout", h.CreateCheckout)
	mux.HandleFunc("POST /api/v1/checkout/calculate", h.Calculate)
}

// --- JSON request/response types ---

type createCheckoutRequest struct {
	CartID          uuid.UUID       `json:"cart_id"`
	Email           string          `json:"email"`
	CountryCode     string          `json:"country_code"`
	VatNumber       string          `json:"vat_number"`
	BillingAddress  json.RawMessage `json:"billing_address"`
	ShippingAddress json.RawMessage `json:"shipping_address"`
}

type createCheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

type calculateRequest struct {
	CartID      uuid.UUID `json:"cart_id"`
	CountryCode string    `json:"country_code"`
	VatNumber   string    `json:"vat_number"`
}

type calculateResponse struct {
	Subtotal       string             `json:"subtotal"`
	VatTotal       string             `json:"vat_total"`
	ShippingFee    string             `json:"shipping_fee"`
	DiscountAmount string             `json:"discount_amount"`
	Total          string             `json:"total"`
	VatBreakdown   []vatBreakdownItem `json:"vat_breakdown"`
	ReverseCharge  bool               `json:"reverse_charge"`
}

type vatBreakdownItem struct {
	ProductName string `json:"product_name"`
	Rate        string `json:"rate"`
	RateType    string `json:"rate_type"`
	Amount      string `json:"amount"`
}

// --- Handlers ---

// CreateCheckout handles POST /api/v1/checkout.
// It validates the cart, calculates VAT and shipping, then creates a Stripe
// Checkout Session and returns the checkout URL.
func (h *CheckoutHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	var req createCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	if req.CartID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "cart_id is required"})
		return
	}
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "email is required"})
		return
	}
	if req.CountryCode == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "country_code is required"})
		return
	}

	ctx := r.Context()

	// Step 1: Load the cart.
	c, err := h.cartSvc.Get(ctx, req.CartID)
	if err != nil {
		if errors.Is(err, cart.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "cart not found"})
			return
		}
		h.logger.Error("failed to load cart for checkout", "error", err, "cart_id", req.CartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Step 2: Load cart items.
	items, err := h.cartSvc.ListItems(ctx, c.ID)
	if err != nil {
		h.logger.Error("failed to list cart items for checkout", "error", err, "cart_id", c.ID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Step 3: Validate cart is not empty.
	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "cart is empty"})
		return
	}

	// Step 4: Validate destination country is enabled for shipping.
	if err := h.validateCountryEnabled(ctx, req.CountryCode); err != nil {
		if errors.Is(err, errCountryNotEnabled) {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "shipping to this country is not enabled"})
			return
		}
		h.logger.Error("failed to validate country", "error", err, "country_code", req.CountryCode)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Step 5: Calculate VAT for each item.
	vatInputs := buildVATInputs(items, req.CountryCode, req.VatNumber)
	vatResults, vatSummary, err := h.vatSvc.CalculateForCart(ctx, vatInputs)
	if err != nil {
		h.logger.Error("VAT calculation failed during checkout", "error", err, "cart_id", c.ID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "failed to calculate VAT"})
		return
	}

	// Step 6: Calculate shipping fee.
	shippingResult, err := h.calculateShipping(ctx, items, req.CountryCode, vatSummary.TotalNet)
	if err != nil {
		if errors.Is(err, shipping.ErrCountryNotEnabled) {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "shipping to this country is not enabled"})
			return
		}
		h.logger.Error("shipping calculation failed during checkout", "error", err, "cart_id", c.ID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "failed to calculate shipping"})
		return
	}

	// Step 7: Build Stripe line items.
	// Each item's gross price (including VAT) is sent to Stripe as the unit amount.
	stripeLineItems := make([]*stripe.CheckoutSessionLineItemParams, 0, len(items)+1)
	for i, item := range items {
		vatRes := vatResults[i]
		stripeLineItems = append(stripeLineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("eur"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name:        stripe.String(item.ProductName),
					Description: stripe.String(item.VariantSku),
				},
				UnitAmount: stripe.Int64(decimalToCents(vatRes.GrossPrice)),
			},
			Quantity: stripe.Int64(int64(item.Quantity)),
		})
	}

	// Add shipping as a line item if there is a fee.
	if shippingResult.TotalFee.IsPositive() {
		stripeLineItems = append(stripeLineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("eur"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name:        stripe.String("Shipping"),
					Description: stripe.String(shippingResult.Method),
				},
				UnitAmount: stripe.Int64(decimalToCents(shippingResult.TotalFee)),
			},
			Quantity: stripe.Int64(1),
		})
	}

	// Step 8: Create the Stripe Checkout Session.
	// Metadata carries all info needed to reconstruct the order in the webhook.
	metadata := map[string]string{
		"cart_id":      c.ID.String(),
		"country_code": req.CountryCode,
		"vat_number":   req.VatNumber,
	}
	if len(req.BillingAddress) > 0 {
		metadata["billing_address"] = string(req.BillingAddress)
	}
	if len(req.ShippingAddress) > 0 {
		metadata["shipping_address"] = string(req.ShippingAddress)
	}

	sessionParams := &stripe.CheckoutSessionParams{
		Mode:          stripe.String(string(stripe.CheckoutSessionModePayment)),
		CustomerEmail: stripe.String(req.Email),
		SuccessURL:    stripe.String(h.successURL),
		CancelURL:     stripe.String(h.cancelURL),
		LineItems:     stripeLineItems,
		Metadata:      metadata,
	}

	session, err := checkoutsession.New(sessionParams)
	if err != nil {
		h.logger.Error("failed to create Stripe checkout session",
			"error", err,
			"cart_id", c.ID,
			"email", req.Email,
		)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "failed to create checkout session"})
		return
	}

	h.logger.Info("checkout session created",
		slog.String("cart_id", c.ID.String()),
		slog.String("stripe_session_id", session.ID),
		slog.String("email", req.Email),
		slog.String("country_code", req.CountryCode),
		slog.Int("items", len(items)),
		slog.String("subtotal", vatSummary.TotalNet.StringFixed(2)),
		slog.String("vat_total", vatSummary.TotalVAT.StringFixed(2)),
		slog.String("shipping", shippingResult.TotalFee.StringFixed(2)),
	)

	writeJSON(w, http.StatusOK, createCheckoutResponse{
		CheckoutURL: session.URL,
	})
}

// Calculate handles POST /api/v1/checkout/calculate.
// It previews checkout totals including VAT breakdown, shipping, and discounts
// without creating any Stripe session or order.
func (h *CheckoutHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	var req calculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "invalid request body"})
		return
	}

	if req.CartID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "cart_id is required"})
		return
	}
	if req.CountryCode == "" {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "country_code is required"})
		return
	}

	ctx := r.Context()

	// Load cart.
	c, err := h.cartSvc.Get(ctx, req.CartID)
	if err != nil {
		if errors.Is(err, cart.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorJSON{Error: "cart not found"})
			return
		}
		h.logger.Error("failed to load cart for calculate", "error", err, "cart_id", req.CartID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	// Load cart items.
	items, err := h.cartSvc.ListItems(ctx, c.ID)
	if err != nil {
		h.logger.Error("failed to list cart items for calculate", "error", err, "cart_id", c.ID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "internal server error"})
		return
	}

	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, errorJSON{Error: "cart is empty"})
		return
	}

	// Calculate VAT for each item.
	vatInputs := buildVATInputs(items, req.CountryCode, req.VatNumber)
	vatResults, vatSummary, err := h.vatSvc.CalculateForCart(ctx, vatInputs)
	if err != nil {
		h.logger.Error("VAT calculation failed during calculate preview", "error", err, "cart_id", c.ID)
		writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "failed to calculate VAT"})
		return
	}

	// Calculate shipping fee.
	shippingResult, err := h.calculateShipping(ctx, items, req.CountryCode, vatSummary.TotalNet)
	if err != nil {
		// If shipping is disabled or unconfigured, default to zero fee for preview.
		if errors.Is(err, shipping.ErrCountryNotEnabled) {
			writeJSON(w, http.StatusBadRequest, errorJSON{Error: "shipping to this country is not enabled"})
			return
		}
		if errors.Is(err, shipping.ErrConfigNotFound) || errors.Is(err, shipping.ErrShippingDisabled) {
			shippingResult = shipping.ShippingResult{}
		} else {
			h.logger.Error("shipping calculation failed during calculate preview", "error", err, "cart_id", c.ID)
			writeJSON(w, http.StatusInternalServerError, errorJSON{Error: "failed to calculate shipping"})
			return
		}
	}

	// Build per-item VAT breakdown.
	breakdown := make([]vatBreakdownItem, len(vatResults))
	for i, res := range vatResults {
		breakdown[i] = vatBreakdownItem{
			ProductName: items[i].ProductName,
			Rate:        res.Rate.StringFixed(2),
			RateType:    res.RateType,
			Amount:      res.LineVATTotal.StringFixed(2),
		}
	}

	// Determine reverse charge status from the first item's result.
	reverseCharge := false
	if len(vatResults) > 0 && vatResults[0].ReverseCharge {
		reverseCharge = true
	}

	// Compute grand total: gross (net + VAT) + shipping - discounts.
	discountAmount := decimal.Zero
	total := vatSummary.TotalGross.Add(shippingResult.TotalFee).Sub(discountAmount)

	writeJSON(w, http.StatusOK, calculateResponse{
		Subtotal:       vatSummary.TotalNet.StringFixed(2),
		VatTotal:       vatSummary.TotalVAT.StringFixed(2),
		ShippingFee:    shippingResult.TotalFee.StringFixed(2),
		DiscountAmount: discountAmount.StringFixed(2),
		Total:          total.StringFixed(2),
		VatBreakdown:   breakdown,
		ReverseCharge:  reverseCharge,
	})
}

// --- Internal helpers ---

var errCountryNotEnabled = errors.New("country not enabled for shipping")

// validateCountryEnabled checks that the given country code exists in the
// store's enabled shipping countries.
func (h *CheckoutHandler) validateCountryEnabled(ctx context.Context, countryCode string) error {
	countries, err := h.queries.ListEnabledShippingCountries(ctx)
	if err != nil {
		return fmt.Errorf("listing enabled shipping countries: %w", err)
	}
	for _, c := range countries {
		if c.CountryCode == countryCode {
			return nil
		}
	}
	return errCountryNotEnabled
}

// buildVATInputs converts cart items into VATInput structs for the VAT service.
func buildVATInputs(items []db.GetCartItemsRow, countryCode, vatNumber string) []vat.VATInput {
	inputs := make([]vat.VATInput, len(items))
	for i, item := range items {
		// Effective price: variant price if set, otherwise product base price.
		price := numericToDecimal(item.VariantPrice)
		if price.IsZero() {
			price = numericToDecimal(item.ProductBasePrice)
		}

		var vatCategoryID *uuid.UUID
		if item.ProductVatCategoryID.Valid {
			id := uuid.UUID(item.ProductVatCategoryID.Bytes)
			vatCategoryID = &id
		}

		inputs[i] = vat.VATInput{
			ProductID:            item.ProductID,
			ProductVATCategoryID: vatCategoryID,
			Price:                price,
			DestinationCountry:   countryCode,
			CustomerVATNumber:    vatNumber,
			Quantity:             item.Quantity,
		}
	}
	return inputs
}

// calculateShipping delegates to the shipping service to compute the shipping
// fee for the given cart items and destination country.
func (h *CheckoutHandler) calculateShipping(
	ctx context.Context,
	items []db.GetCartItemsRow,
	countryCode string,
	subtotal decimal.Decimal,
) (shipping.ShippingResult, error) {
	totalWeight := 0
	shippingItems := make([]shipping.ShippingItem, len(items))
	for i, item := range items {
		weight := 0
		if item.VariantWeightGrams != nil {
			weight = int(*item.VariantWeightGrams)
		}
		totalWeight += weight * int(item.Quantity)

		// Per-product shipping extra fees are not available directly on the
		// cart item row; this would require an additional product query.
		// Using zero here as a simplification.
		shippingItems[i] = shipping.ShippingItem{
			ProductExtraFee: decimal.Zero,
			Quantity:        int(item.Quantity),
		}
	}

	return h.shippingSvc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  countryCode,
		TotalWeightG: totalWeight,
		Items:        shippingItems,
		Subtotal:     subtotal,
	})
}

// decimalToCents converts a shopspring Decimal amount (e.g., 21.50) to integer
// cents (e.g., 2150) for Stripe. Uses IntPart after multiplying by 100.
func decimalToCents(d decimal.Decimal) int64 {
	return d.Mul(decimal.NewFromInt(100)).IntPart()
}

// decimalToNumeric converts a shopspring Decimal to a pgtype.Numeric suitable
// for sqlc-generated database parameters.
func decimalToNumeric(d decimal.Decimal) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(d.String())
	return n
}

// numericToDecimal converts a pgtype.Numeric from the database to a shopspring
// Decimal. Returns decimal.Zero if the Numeric is not valid.
func numericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(n.Int, n.Exp)
}

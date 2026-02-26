package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	stripe "github.com/stripe/stripe-go/v82"

	"github.com/forgecommerce/api/internal/services/order"
	forgestripe "github.com/forgecommerce/api/internal/stripe"
)

// WebhookHandler handles incoming Stripe webhook events.
type WebhookHandler struct {
	stripeSvc *forgestripe.Service
	orderSvc  *order.Service
	logger    *slog.Logger
	secret    string // webhook signing secret
}

// NewWebhookHandler creates a new Stripe webhook handler.
func NewWebhookHandler(
	stripeSvc *forgestripe.Service,
	orderSvc *order.Service,
	logger *slog.Logger,
	webhookSecret string,
) *WebhookHandler {
	return &WebhookHandler{
		stripeSvc: stripeSvc,
		orderSvc:  orderSvc,
		logger:    logger,
		secret:    webhookSecret,
	}
}

// RegisterRoutes registers the webhook endpoint.
func (h *WebhookHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/webhooks/stripe", h.HandleStripeWebhook)
}

// HandleStripeWebhook handles POST /api/v1/webhooks/stripe.
// It verifies the Stripe signature, then dispatches based on event type.
func (h *WebhookHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Read the body (Stripe requires raw body for signature verification).
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		h.logger.Error("failed to read webhook body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verify signature.
	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := h.stripeSvc.VerifyWebhookSignature(body, sigHeader, h.secret)
	if err != nil {
		h.logger.Warn("webhook signature verification failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.logger.Info("stripe webhook received",
		slog.String("event_id", event.ID),
		slog.String("event_type", string(event.Type)),
	)

	// Dispatch based on event type.
	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutSessionCompleted(r, event)
	case "payment_intent.succeeded":
		h.handlePaymentIntentSucceeded(r, event)
	case "payment_intent.payment_failed":
		h.handlePaymentIntentFailed(r, event)
	default:
		h.logger.Debug("unhandled webhook event type", "type", string(event.Type))
	}

	// Always return 200 to Stripe to acknowledge receipt.
	w.WriteHeader(http.StatusOK)
}

// handleCheckoutSessionCompleted processes a completed checkout session.
// This is the primary event for creating orders — it fires when the customer
// successfully completes payment on the Stripe Checkout page.
func (h *WebhookHandler) handleCheckoutSessionCompleted(r *http.Request, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		h.logger.Error("failed to unmarshal checkout session", "error", err, "event_id", event.ID)
		return
	}

	cartIDStr, ok := session.Metadata["cart_id"]
	if !ok || cartIDStr == "" {
		h.logger.Error("checkout session missing cart_id metadata", "session_id", session.ID)
		return
	}

	cartID, err := uuid.Parse(cartIDStr)
	if err != nil {
		h.logger.Error("invalid cart_id in checkout metadata", "cart_id", cartIDStr, "error", err)
		return
	}

	// Extract metadata fields.
	countryCode := session.Metadata["country_code"]
	vatNumber := session.Metadata["vat_number"]

	// Build the order from the checkout session metadata.
	email := ""
	if session.CustomerEmail != "" {
		email = session.CustomerEmail
	}
	if session.CustomerDetails != nil && session.CustomerDetails.Email != "" {
		email = session.CustomerDetails.Email
	}

	paymentIntentID := ""
	if session.PaymentIntent != nil {
		paymentIntentID = session.PaymentIntent.ID
	}
	sessionID := session.ID

	// Parse addresses from metadata (stored as JSON strings).
	billingAddress := parseMaybeJSON(session.Metadata["billing_address"])
	shippingAddress := parseMaybeJSON(session.Metadata["shipping_address"])

	h.logger.Info("creating order from checkout session",
		slog.String("session_id", session.ID),
		slog.String("cart_id", cartID.String()),
		slog.String("email", email),
		slog.String("country_code", countryCode),
		slog.String("payment_intent_id", paymentIntentID),
	)

	// Build order params — amounts will be populated from the session totals.
	// Note: In a production system, we'd recalculate from cart items for accuracy.
	// Here we use the Stripe session amounts as the source of truth since
	// the customer has already paid.
	total := numericFromStripeAmount(session.AmountTotal)
	subtotal := numericFromStripeAmount(session.AmountSubtotal)

	params := order.CreateOrderParams{
		Status:                  "pending",
		Email:                   email,
		BillingAddress:          billingAddress,
		ShippingAddress:         shippingAddress,
		Subtotal:                subtotal,
		ShippingFee:             zeroNumeric(),
		ShippingExtraFees:       zeroNumeric(),
		DiscountAmount:          zeroNumeric(),
		VatTotal:                zeroNumeric(),
		Total:                   total,
		VatReverseCharge:        vatNumber != "",
		StripePaymentIntentID:   strPtrOrNil(paymentIntentID),
		StripeCheckoutSessionID: &sessionID,
		PaymentStatus:           "paid",
		Items:                   []order.CreateOrderItemInput{},
		Metadata:                json.RawMessage(`{}`),
	}

	if countryCode != "" {
		params.VatCountryCode = &countryCode
	}
	if vatNumber != "" {
		params.VatNumber = &vatNumber
	}

	_, _, err = h.orderSvc.Create(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to create order from checkout",
			"error", err,
			"session_id", session.ID,
			"cart_id", cartID.String(),
		)
		return
	}

	h.logger.Info("order created from checkout session",
		slog.String("session_id", session.ID),
		slog.String("cart_id", cartID.String()),
	)
}

// handlePaymentIntentSucceeded handles a payment_intent.succeeded event.
// This can be used to update an existing order's payment status.
func (h *WebhookHandler) handlePaymentIntentSucceeded(r *http.Request, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		h.logger.Error("failed to unmarshal payment intent", "error", err, "event_id", event.ID)
		return
	}

	h.logger.Info("payment intent succeeded",
		slog.String("payment_intent_id", pi.ID),
		slog.Int64("amount", pi.Amount),
		slog.String("currency", string(pi.Currency)),
	)
}

// handlePaymentIntentFailed handles a payment_intent.payment_failed event.
func (h *WebhookHandler) handlePaymentIntentFailed(r *http.Request, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		h.logger.Error("failed to unmarshal payment intent", "error", err, "event_id", event.ID)
		return
	}

	failMsg := ""
	if pi.LastPaymentError != nil {
		failMsg = pi.LastPaymentError.Msg
	}

	h.logger.Warn("payment intent failed",
		slog.String("payment_intent_id", pi.ID),
		slog.String("failure_message", failMsg),
	)
}

// --- Helpers ---

// parseMaybeJSON returns the raw JSON bytes if the string is valid JSON,
// otherwise returns an empty JSON object.
func parseMaybeJSON(s string) []byte {
	if s == "" {
		return []byte("{}")
	}
	// Validate it's actually JSON.
	var js json.RawMessage
	if err := json.Unmarshal([]byte(s), &js); err != nil {
		return []byte("{}")
	}
	return []byte(s)
}

// numericFromStripeAmount converts a Stripe amount (in cents) to pgtype.Numeric
// representing the value in the base currency unit (e.g., 4250 -> 42.50).
func numericFromStripeAmount(cents int64) pgtype.Numeric {
	// Convert cents to a decimal string: 4250 -> "42.50"
	whole := cents / 100
	frac := cents % 100
	s := fmt.Sprintf("%d.%02d", whole, frac)
	var n pgtype.Numeric
	_ = n.Scan(s)
	return n
}

// zeroNumeric returns a pgtype.Numeric representing zero.
func zeroNumeric() pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan("0.00")
	return n
}

// strPtrOrNil returns a pointer to the string if non-empty, nil otherwise.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

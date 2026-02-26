package api_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	gostripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/services/order"
	forgestripe "github.com/forgecommerce/api/internal/stripe"
)

const testWebhookSecret = "whsec_test_webhook_handler_secret"

// newWebhookHandler creates a WebhookHandler wired to real services backed by testDB.
func newWebhookHandler() *api.WebhookHandler {
	logger := slog.Default()
	stripeSvc := forgestripe.NewService("sk_test_webhook_handler", logger)
	orderSvc := order.NewService(testDB.Pool, logger)
	return api.NewWebhookHandler(stripeSvc, orderSvc, logger, testWebhookSecret)
}

// webhookMux registers the webhook handler on a fresh ServeMux.
func webhookMux() *http.ServeMux {
	mux := http.NewServeMux()
	newWebhookHandler().RegisterRoutes(mux)
	return mux
}

// signPayload creates a properly signed Stripe webhook payload and returns the
// body bytes and the Stripe-Signature header value.
func signPayload(t *testing.T, payload []byte) (body []byte, sigHeader string) {
	t.Helper()
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    testWebhookSecret,
		Timestamp: time.Now(),
	})
	return signed.Payload, signed.Header
}

// --------------------------------------------------------------------------
// TestWebhookHandler_MissingSignature
// --------------------------------------------------------------------------

func TestWebhookHandler_MissingSignature(t *testing.T) {
	mux := webhookMux()

	body := []byte(`{"id":"evt_test","type":"checkout.session.completed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	// No Stripe-Signature header set.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_InvalidSignature
// --------------------------------------------------------------------------

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	mux := webhookMux()

	body := []byte(`{"id":"evt_test","type":"checkout.session.completed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", "t=1234567890,v1=invalidsignaturevalue")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_EmptyBody
// --------------------------------------------------------------------------

func TestWebhookHandler_EmptyBody(t *testing.T) {
	mux := webhookMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader([]byte{}))
	req.Header.Set("Stripe-Signature", "t=1234567890,v1=abc")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Empty body cannot produce a valid signature match, so verification fails -> 400.
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_checkout_ok",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_session_123",
				"customer_email": "buyer@example.com",
				"customer_details": {"email": "buyer@example.com"},
				"payment_intent": {"id": "pi_test_intent_456"},
				"amount_total": 12500,
				"amount_subtotal": 10000,
				"metadata": {
					"cart_id": %q,
					"country_code": "ES",
					"billing_address": "{\"city\":\"Madrid\",\"country\":\"ES\"}",
					"shipping_address": "{\"city\":\"Madrid\",\"country\":\"ES\"}"
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Webhook handler always returns 200 to Stripe after successful signature verification.
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify that an order was created in the database.
	ctx := context.Background()
	var orderCount int64
	err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}
	if orderCount != 1 {
		t.Fatalf("expected 1 order, got %d", orderCount)
	}

	// Verify order fields.
	var (
		email                   string
		status                  string
		paymentStatus           string
		vatReverseCharge        bool
		stripeCheckoutSessionID *string
		stripePaymentIntentID   *string
		vatCountryCode          *string
	)
	err = testDB.Pool.QueryRow(ctx,
		`SELECT email, status, payment_status, vat_reverse_charge,
		        stripe_checkout_session_id, stripe_payment_intent_id, vat_country_code
		 FROM orders LIMIT 1`,
	).Scan(&email, &status, &paymentStatus, &vatReverseCharge,
		&stripeCheckoutSessionID, &stripePaymentIntentID, &vatCountryCode)
	if err != nil {
		t.Fatalf("scanning order: %v", err)
	}

	if email != "buyer@example.com" {
		t.Errorf("email: got %q, want %q", email, "buyer@example.com")
	}
	if status != "pending" {
		t.Errorf("status: got %q, want %q", status, "pending")
	}
	if paymentStatus != "paid" {
		t.Errorf("payment_status: got %q, want %q", paymentStatus, "paid")
	}
	if vatReverseCharge {
		t.Error("vat_reverse_charge: got true, want false (no VAT number provided)")
	}
	if stripeCheckoutSessionID == nil || *stripeCheckoutSessionID != "cs_test_session_123" {
		t.Errorf("stripe_checkout_session_id: got %v, want %q", stripeCheckoutSessionID, "cs_test_session_123")
	}
	if stripePaymentIntentID == nil || *stripePaymentIntentID != "pi_test_intent_456" {
		t.Errorf("stripe_payment_intent_id: got %v, want %q", stripePaymentIntentID, "pi_test_intent_456")
	}
	if vatCountryCode == nil || *vatCountryCode != "ES" {
		t.Errorf("vat_country_code: got %v, want %q", vatCountryCode, "ES")
	}

	// Verify an order_created event was recorded.
	var eventCount int64
	err = testDB.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM order_events WHERE event_type = 'order_created'",
	).Scan(&eventCount)
	if err != nil {
		t.Fatalf("counting order events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("expected 1 order_created event, got %d", eventCount)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_WithVATNumber
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_WithVATNumber(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_checkout_b2b",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_b2b_session",
				"customer_email": "business@example.de",
				"customer_details": {"email": "business@example.de"},
				"payment_intent": {"id": "pi_test_b2b_intent"},
				"amount_total": 20000,
				"amount_subtotal": 20000,
				"metadata": {
					"cart_id": %q,
					"country_code": "DE",
					"vat_number": "DE123456789",
					"billing_address": "{\"city\":\"Berlin\",\"country\":\"DE\"}",
					"shipping_address": "{\"city\":\"Berlin\",\"country\":\"DE\"}"
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify order was created with reverse charge and VAT number.
	ctx := context.Background()
	var (
		vatReverseCharge bool
		vatNumber        *string
		vatCountryCode   *string
	)
	err := testDB.Pool.QueryRow(ctx,
		`SELECT vat_reverse_charge, vat_number, vat_country_code FROM orders LIMIT 1`,
	).Scan(&vatReverseCharge, &vatNumber, &vatCountryCode)
	if err != nil {
		t.Fatalf("scanning order: %v", err)
	}

	if !vatReverseCharge {
		t.Error("vat_reverse_charge: got false, want true (VAT number was provided)")
	}
	if vatNumber == nil || *vatNumber != "DE123456789" {
		t.Errorf("vat_number: got %v, want %q", vatNumber, "DE123456789")
	}
	if vatCountryCode == nil || *vatCountryCode != "DE" {
		t.Errorf("vat_country_code: got %v, want %q", vatCountryCode, "DE")
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_MissingCartID
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_MissingCartID(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	// Valid signed event, but metadata has no cart_id.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_no_cart",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_no_cart",
				"customer_email": "someone@example.com",
				"payment_intent": {"id": "pi_test_no_cart"},
				"amount_total": 5000,
				"amount_subtotal": 5000,
				"metadata": {
					"country_code": "FR"
				}
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Always return 200 to Stripe to acknowledge receipt.
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d (must always return 200 to Stripe)", rr.Code, http.StatusOK)
	}

	// No order should have been created.
	ctx := context.Background()
	var orderCount int64
	err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}
	if orderCount != 0 {
		t.Errorf("expected 0 orders (missing cart_id), got %d", orderCount)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_InvalidCartID
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_InvalidCartID(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	// Valid signed event, but cart_id is not a valid UUID.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_bad_cart",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_bad_cart",
				"customer_email": "someone@example.com",
				"payment_intent": {"id": "pi_test_bad_cart"},
				"amount_total": 5000,
				"amount_subtotal": 5000,
				"metadata": {
					"cart_id": "not-a-valid-uuid",
					"country_code": "FR"
				}
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Always return 200 to Stripe.
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// No order should have been created.
	ctx := context.Background()
	var orderCount int64
	err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}
	if orderCount != 0 {
		t.Errorf("expected 0 orders (invalid cart_id), got %d", orderCount)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_EmptyCartID
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_EmptyCartID(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	// Valid signed event, but cart_id is empty string.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_empty_cart",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_empty_cart",
				"customer_email": "someone@example.com",
				"payment_intent": {"id": "pi_test_empty_cart"},
				"amount_total": 5000,
				"amount_subtotal": 5000,
				"metadata": {
					"cart_id": "",
					"country_code": "FR"
				}
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// No order should have been created because cart_id is empty.
	ctx := context.Background()
	var orderCount int64
	err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}
	if orderCount != 0 {
		t.Errorf("expected 0 orders (empty cart_id), got %d", orderCount)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_PaymentIntentSucceeded
// --------------------------------------------------------------------------

func TestWebhookHandler_PaymentIntentSucceeded(t *testing.T) {
	mux := webhookMux()

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_pi_succeeded",
		"type": "payment_intent.succeeded",
		"api_version": %q,
		"data": {
			"object": {
				"id": "pi_test_succeeded_789",
				"amount": 12500,
				"currency": "eur",
				"status": "succeeded"
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Handler logs the event and returns 200.
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_PaymentIntentFailed
// --------------------------------------------------------------------------

func TestWebhookHandler_PaymentIntentFailed(t *testing.T) {
	mux := webhookMux()

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_pi_failed",
		"type": "payment_intent.payment_failed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "pi_test_failed_456",
				"amount": 7500,
				"currency": "eur",
				"status": "requires_payment_method",
				"last_payment_error": {
					"message": "Your card was declined."
				}
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Handler logs the failure and returns 200.
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_PaymentIntentFailed_NoError
// --------------------------------------------------------------------------

func TestWebhookHandler_PaymentIntentFailed_NoError(t *testing.T) {
	mux := webhookMux()

	// Payment intent failed without a last_payment_error (tests the nil check).
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_pi_failed_no_err",
		"type": "payment_intent.payment_failed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "pi_test_failed_no_err",
				"amount": 3000,
				"currency": "eur",
				"status": "requires_payment_method"
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_UnknownEventType
// --------------------------------------------------------------------------

func TestWebhookHandler_UnknownEventType(t *testing.T) {
	mux := webhookMux()

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_unknown",
		"type": "customer.subscription.updated",
		"api_version": %q,
		"data": {
			"object": {
				"id": "sub_test_123"
			}
		}
	}`, gostripe.APIVersion))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Unknown events are acknowledged with 200 (no error to Stripe).
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_WrongSecret
// --------------------------------------------------------------------------

func TestWebhookHandler_WrongSecret(t *testing.T) {
	mux := webhookMux()

	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_wrong_secret",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {"object": {}}
	}`, gostripe.APIVersion))

	// Sign with a DIFFERENT secret than the handler expects.
	wrongSecret := "whsec_this_is_the_wrong_secret"
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    wrongSecret,
		Timestamp: time.Now(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(signed.Payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d (wrong secret should fail verification)", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_CustomerDetailsEmail
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_CustomerDetailsEmail(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	// Test that customer_details.email takes precedence over customer_email.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_email_precedence",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_email_pref",
				"customer_email": "first@example.com",
				"customer_details": {"email": "preferred@example.com"},
				"payment_intent": {"id": "pi_test_email"},
				"amount_total": 5000,
				"amount_subtotal": 5000,
				"metadata": {
					"cart_id": %q,
					"country_code": "FR",
					"billing_address": "{}",
					"shipping_address": "{}"
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify the email used is from customer_details (the later assignment wins).
	ctx := context.Background()
	var email string
	err := testDB.Pool.QueryRow(ctx, "SELECT email FROM orders LIMIT 1").Scan(&email)
	if err != nil {
		t.Fatalf("scanning order email: %v", err)
	}
	if email != "preferred@example.com" {
		t.Errorf("email: got %q, want %q (customer_details.email should take precedence)", email, "preferred@example.com")
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_NoPaymentIntent
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_NoPaymentIntent(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	// Session without a payment_intent object (e.g., setup mode or edge case).
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_no_pi",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_no_pi",
				"customer_email": "nopi@example.com",
				"amount_total": 8000,
				"amount_subtotal": 8000,
				"metadata": {
					"cart_id": %q,
					"country_code": "DE",
					"billing_address": "{}",
					"shipping_address": "{}"
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Order should still be created; stripe_payment_intent_id will be NULL.
	ctx := context.Background()
	var piID *string
	err := testDB.Pool.QueryRow(ctx, "SELECT stripe_payment_intent_id FROM orders LIMIT 1").Scan(&piID)
	if err != nil {
		t.Fatalf("scanning order: %v", err)
	}
	if piID != nil {
		t.Errorf("stripe_payment_intent_id: got %q, want nil (no payment_intent in event)", *piID)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_InvalidAddressJSON
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_InvalidAddressJSON(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	// Send invalid JSON for billing_address; parseMaybeJSON should return "{}".
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_bad_addr",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_bad_addr",
				"customer_email": "addr@example.com",
				"payment_intent": {"id": "pi_test_addr"},
				"amount_total": 3000,
				"amount_subtotal": 3000,
				"metadata": {
					"cart_id": %q,
					"country_code": "FR",
					"billing_address": "not valid json {{",
					"shipping_address": ""
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Order should still be created with empty JSON objects for addresses.
	ctx := context.Background()
	var orderCount int64
	err := testDB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("counting orders: %v", err)
	}
	if orderCount != 1 {
		t.Errorf("expected 1 order (bad address JSON should fall back to {}), got %d", orderCount)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_MethodNotAllowed
// --------------------------------------------------------------------------

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	mux := webhookMux()

	// The route is registered for POST only. GET should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/stripe", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Go 1.22+ ServeMux returns 405 for method mismatch on explicit method routes.
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d for GET on POST-only route", rr.Code, http.StatusMethodNotAllowed)
	}
}

// --------------------------------------------------------------------------
// TestWebhookHandler_CheckoutSessionCompleted_NoCountryCode
// --------------------------------------------------------------------------

func TestWebhookHandler_CheckoutSessionCompleted_NoCountryCode(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	mux := webhookMux()

	cartID := uuid.New()
	// No country_code in metadata.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_no_country",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {
			"object": {
				"id": "cs_test_no_country",
				"customer_email": "nocountry@example.com",
				"payment_intent": {"id": "pi_test_no_country"},
				"amount_total": 4000,
				"amount_subtotal": 4000,
				"metadata": {
					"cart_id": %q,
					"billing_address": "{}",
					"shipping_address": "{}"
				}
			}
		}
	}`, gostripe.APIVersion, cartID.String()))

	body, sigHeader := signPayload(t, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", sigHeader)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	// Order should still be created; vat_country_code will be NULL.
	ctx := context.Background()
	var vatCountryCode *string
	err := testDB.Pool.QueryRow(ctx, "SELECT vat_country_code FROM orders LIMIT 1").Scan(&vatCountryCode)
	if err != nil {
		t.Fatalf("scanning order: %v", err)
	}
	if vatCountryCode != nil {
		t.Errorf("vat_country_code: got %q, want nil (no country_code in metadata)", *vatCountryCode)
	}
}

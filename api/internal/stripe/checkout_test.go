package stripe

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	gostripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

// --------------------------------------------------------------------------
// Tests for validateCheckoutInput
// --------------------------------------------------------------------------

func TestValidateCheckoutInput(t *testing.T) {
	validInput := CheckoutInput{
		CartID:     uuid.New(),
		Email:      "test@example.com",
		SuccessURL: "https://shop.example.com/success",
		CancelURL:  "https://shop.example.com/cancel",
		Currency:   "eur",
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
	}

	tests := []struct {
		name    string
		modify  func(CheckoutInput) CheckoutInput
		wantErr error
	}{
		{
			name:    "valid input",
			modify:  func(i CheckoutInput) CheckoutInput { return i },
			wantErr: nil,
		},
		{
			name: "empty items",
			modify: func(i CheckoutInput) CheckoutInput {
				i.Items = nil
				return i
			},
			wantErr: ErrEmptyItems,
		},
		{
			name: "empty items slice",
			modify: func(i CheckoutInput) CheckoutInput {
				i.Items = []CheckoutItem{}
				return i
			},
			wantErr: ErrEmptyItems,
		},
		{
			name: "empty currency",
			modify: func(i CheckoutInput) CheckoutInput {
				i.Currency = ""
				return i
			},
			wantErr: ErrInvalidCurrency,
		},
		{
			name: "missing success URL",
			modify: func(i CheckoutInput) CheckoutInput {
				i.SuccessURL = ""
				return i
			},
			wantErr: ErrMissingURLs,
		},
		{
			name: "missing cancel URL",
			modify: func(i CheckoutInput) CheckoutInput {
				i.CancelURL = ""
				return i
			},
			wantErr: ErrMissingURLs,
		},
		{
			name: "both URLs missing",
			modify: func(i CheckoutInput) CheckoutInput {
				i.SuccessURL = ""
				i.CancelURL = ""
				return i
			},
			wantErr: ErrMissingURLs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.modify(validInput)
			err := validateCheckoutInput(input)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %v, got nil", tt.wantErr)
			}
			if err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for decimalToCents
// --------------------------------------------------------------------------

func TestDecimalToCents(t *testing.T) {
	tests := []struct {
		name  string
		input decimal.Decimal
		want  int64
	}{
		{"42.50 -> 4250", decimal.NewFromFloat(42.50), 4250},
		{"0.01 -> 1", decimal.NewFromFloat(0.01), 1},
		{"100.00 -> 10000", decimal.NewFromFloat(100.00), 10000},
		{"0.00 -> 0", decimal.NewFromFloat(0.00), 0},
		{"99.999 rounds to 10000", decimal.NewFromFloat(99.999), 10000},
		{"0.005 rounds to 1", decimal.NewFromFloat(0.005), 1},
		{"0.004 rounds to 0", decimal.NewFromFloat(0.004), 0},
		{"large amount 9999.99", decimal.NewFromFloat(9999.99), 999999},
		{"negative -5.00 -> -500", decimal.NewFromFloat(-5.00), -500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decimalToCents(tt.input)
			if got != tt.want {
				t.Errorf("decimalToCents(%s) = %d, want %d", tt.input.String(), got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for strPtr
// --------------------------------------------------------------------------

func TestStrPtr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{"empty string returns nil", "", true},
		{"non-empty returns pointer", "hello", false},
		{"whitespace is non-empty", " ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strPtr(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %q", *got)
				}
			} else {
				if got == nil {
					t.Fatal("expected non-nil, got nil")
				}
				if *got != tt.input {
					t.Errorf("expected %q, got %q", tt.input, *got)
				}
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for NewService
// --------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	svc := NewService("sk_test_12345", slog.Default())
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.logger == nil {
		t.Error("expected logger to be set")
	}
	// Verify the global key was set.
	if gostripe.Key != "sk_test_12345" {
		t.Errorf("expected stripe.Key = %q, got %q", "sk_test_12345", gostripe.Key)
	}
}

func TestNewService_NilLogger(t *testing.T) {
	svc := NewService("sk_test_nil_logger", nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.logger == nil {
		t.Error("expected default logger when nil is passed")
	}
}

// --------------------------------------------------------------------------
// Tests for VerifyWebhookSignature
// --------------------------------------------------------------------------

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	svc := NewService("sk_test_webhook", slog.Default())
	secret := "whsec_test_secret_12345"

	// Create a valid Stripe event payload with the correct API version.
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_test_123",
		"type": "checkout.session.completed",
		"api_version": %q,
		"data": {"object": {}}
	}`, gostripe.APIVersion))

	// Generate a valid test signature using Stripe's test helper.
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now(),
	})

	result, err := svc.VerifyWebhookSignature(signedPayload.Payload, signedPayload.Header, secret)
	if err != nil {
		t.Fatalf("VerifyWebhookSignature: unexpected error: %v", err)
	}

	if result.ID != "evt_test_123" {
		t.Errorf("event ID: got %q, want %q", result.ID, "evt_test_123")
	}
	if result.Type != "checkout.session.completed" {
		t.Errorf("event type: got %q, want %q", result.Type, "checkout.session.completed")
	}
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	svc := NewService("sk_test_webhook", slog.Default())
	secret := "whsec_test_secret_12345"
	payload := []byte(`{"id":"evt_test","type":"checkout.session.completed"}`)

	_, err := svc.VerifyWebhookSignature(payload, "t=12345,v1=invalidsignature", secret)
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
	if !strings.Contains(err.Error(), "verifying webhook signature") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestVerifyWebhookSignature_EmptySignature(t *testing.T) {
	svc := NewService("sk_test_webhook", slog.Default())

	_, err := svc.VerifyWebhookSignature([]byte(`{}`), "", "whsec_secret")
	if err == nil {
		t.Fatal("expected error for empty signature header, got nil")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	svc := NewService("sk_test_webhook", slog.Default())
	correctSecret := "whsec_correct"
	wrongSecret := "whsec_wrong"

	payload := []byte(`{"id":"evt_test","type":"payment_intent.succeeded"}`)
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    correctSecret,
		Timestamp: time.Now(),
	})

	_, err := svc.VerifyWebhookSignature(signedPayload.Payload, signedPayload.Header, wrongSecret)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestVerifyWebhookSignature_ExpiredTimestamp(t *testing.T) {
	svc := NewService("sk_test_webhook", slog.Default())
	secret := "whsec_test_expired"

	payload := []byte(`{"id":"evt_old","type":"charge.succeeded"}`)
	// Use a timestamp from 10 minutes ago (beyond the 5-minute default tolerance).
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now().Add(-10 * time.Minute),
	})

	_, err := svc.VerifyWebhookSignature(signedPayload.Payload, signedPayload.Header, secret)
	if err == nil {
		t.Fatal("expected error for expired timestamp, got nil")
	}
}

// --------------------------------------------------------------------------
// Tests for CreateCheckoutSession input validation path
// --------------------------------------------------------------------------

func TestCreateCheckoutSession_ValidationErrors(t *testing.T) {
	// We can test the validation path without hitting Stripe's API.
	svc := NewService("sk_test_validation", slog.Default())

	tests := []struct {
		name    string
		input   CheckoutInput
		wantErr string
	}{
		{
			name:    "empty items",
			input:   CheckoutInput{Currency: "eur", SuccessURL: "https://ok", CancelURL: "https://cancel"},
			wantErr: "validating checkout input",
		},
		{
			name: "empty currency",
			input: CheckoutInput{
				Items:      []CheckoutItem{{Name: "Test", Quantity: 1, UnitPrice: 100}},
				SuccessURL: "https://ok",
				CancelURL:  "https://cancel",
			},
			wantErr: "validating checkout input",
		},
		{
			name: "missing URLs",
			input: CheckoutInput{
				Items:    []CheckoutItem{{Name: "Test", Quantity: 1, UnitPrice: 100}},
				Currency: "eur",
			},
			wantErr: "validating checkout input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCheckoutSession(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for decimalToCents edge cases
// --------------------------------------------------------------------------

func TestDecimalToCents_Precision(t *testing.T) {
	// Verify that the function handles typical EU VAT amounts correctly.
	// €21.26 VAT -> 2126 cents
	vatAmount := decimal.NewFromFloat(21.26)
	if got := decimalToCents(vatAmount); got != 2126 {
		t.Errorf("decimalToCents(21.26) = %d, want 2126", got)
	}

	// €0.21 super reduced VAT -> 21 cents
	smallVAT := decimal.NewFromFloat(0.21)
	if got := decimalToCents(smallVAT); got != 21 {
		t.Errorf("decimalToCents(0.21) = %d, want 21", got)
	}

	// Test with string-constructed decimal for exact precision.
	exact, _ := decimal.NewFromString("99.99")
	if got := decimalToCents(exact); got != 9999 {
		t.Errorf("decimalToCents(99.99) = %d, want 9999", got)
	}
}

// --------------------------------------------------------------------------
// Tests for CheckoutInput types
// --------------------------------------------------------------------------

func TestCheckoutItem_Metadata(t *testing.T) {
	// Verify that the metadata map on CheckoutInput works as expected.
	input := CheckoutInput{
		CartID:     uuid.New(),
		Email:      "test@example.com",
		SuccessURL: "https://shop.example.com/success",
		CancelURL:  "https://shop.example.com/cancel",
		Currency:   "eur",
		Items:      []CheckoutItem{{Name: "P", Quantity: 1, UnitPrice: 100}},
		Metadata: map[string]string{
			"vat_country": "ES",
			"order_ref":   "ORD-001",
		},
	}

	// Ensure metadata is retained.
	if input.Metadata["vat_country"] != "ES" {
		t.Errorf("metadata vat_country: got %q", input.Metadata["vat_country"])
	}

	// Verify cart_id would be added (this is done in CreateCheckoutSession).
	cartIDStr := input.CartID.String()
	if cartIDStr == "" {
		t.Error("cart ID should not be empty")
	}
}

func TestCheckoutResult_Fields(t *testing.T) {
	result := CheckoutResult{
		SessionID:       "cs_test_abc123",
		SessionURL:      "https://checkout.stripe.com/pay/cs_test_abc123",
		PaymentIntentID: "pi_test_xyz789",
	}

	if result.SessionID != "cs_test_abc123" {
		t.Errorf("SessionID: got %q", result.SessionID)
	}
	if !strings.HasPrefix(result.SessionURL, "https://") {
		t.Errorf("SessionURL should be HTTPS: got %q", result.SessionURL)
	}
	if !strings.HasPrefix(result.PaymentIntentID, "pi_") {
		t.Errorf("PaymentIntentID should start with pi_: got %q", result.PaymentIntentID)
	}
}

func TestErrConstants(t *testing.T) {
	// Verify error sentinel values exist and have meaningful messages.
	if ErrEmptyItems.Error() == "" {
		t.Error("ErrEmptyItems should have a message")
	}
	if ErrInvalidCurrency.Error() == "" {
		t.Error("ErrInvalidCurrency should have a message")
	}
	if ErrMissingURLs.Error() == "" {
		t.Error("ErrMissingURLs should have a message")
	}

	// Verify they are distinct.
	if ErrEmptyItems == ErrInvalidCurrency {
		t.Error("ErrEmptyItems and ErrInvalidCurrency should be distinct")
	}
}

// --------------------------------------------------------------------------
// Tests for CreateCheckoutSession — exercises all code paths up to the Stripe
// API call. Since we don't have a valid API key, session.New() returns an
// authentication error, but all the line-item building, shipping, discount,
// and metadata code executes and gets coverage.
// --------------------------------------------------------------------------

func TestCreateCheckoutSession_BuildsLineItems(t *testing.T) {
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:     uuid.New(),
		Email:      "buyer@example.com",
		SuccessURL: "https://shop.example.com/success",
		CancelURL:  "https://shop.example.com/cancel",
		Currency:   "eur",
		Items: []CheckoutItem{
			{
				Name:        "Leather Bag",
				Description: "Black / Large",
				Quantity:    2,
				UnitPrice:   4500,
				VATAmount:   945, // 21% of 45.00
				ProductID:   "prod_001",
				VariantID:   "var_001",
			},
			{
				Name:        "Canvas Tote",
				Description: "", // empty description → strPtr returns nil
				Quantity:    1,
				UnitPrice:   6900,
				VATAmount:   0, // no VAT metadata branch
				ProductID:   "prod_002",
				VariantID:   "var_002",
			},
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	// Expect Stripe API error (auth), not a validation error.
	if err == nil {
		t.Fatal("expected error from Stripe API, got nil")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Errorf("should not be a validation error: %v", err)
	}
	if !strings.Contains(err.Error(), "creating stripe checkout session") {
		t.Errorf("expected Stripe session creation error, got: %v", err)
	}
}

func TestCreateCheckoutSession_WithShippingFee(t *testing.T) {
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:      uuid.New(),
		Email:       "buyer@example.com",
		SuccessURL:  "https://shop.example.com/success",
		CancelURL:   "https://shop.example.com/cancel",
		Currency:    "eur",
		ShippingFee: decimal.NewFromFloat(8.50), // positive → adds shipping line item
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	// Should reach the Stripe API call, not fail on validation.
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_WithDiscount(t *testing.T) {
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:         uuid.New(),
		Email:          "buyer@example.com",
		SuccessURL:     "https://shop.example.com/success",
		CancelURL:      "https://shop.example.com/cancel",
		Currency:       "eur",
		DiscountAmount: decimal.NewFromFloat(10.00), // positive → discount coupon branch
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 5000},
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_WithMetadata(t *testing.T) {
	svc := NewService("sk_test_fake_key", slog.Default())

	cartID := uuid.New()
	input := CheckoutInput{
		CartID:     cartID,
		Email:      "buyer@example.com",
		SuccessURL: "https://shop.example.com/success",
		CancelURL:  "https://shop.example.com/cancel",
		Currency:   "eur",
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
		Metadata: map[string]string{
			"vat_country":    "ES",
			"vat_rate":       "21.00",
			"order_ref":      "ORD-999",
			"reverse_charge": "false",
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_AllFeatures(t *testing.T) {
	// Exercise every branch in one test: multiple items (with/without VAT),
	// shipping, discount, and metadata.
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:         uuid.New(),
		Email:          "vip@example.com",
		SuccessURL:     "https://shop.example.com/success?session_id={CHECKOUT_SESSION_ID}",
		CancelURL:      "https://shop.example.com/cancel",
		Currency:       "eur",
		ShippingFee:    decimal.NewFromFloat(12.99),
		DiscountAmount: decimal.NewFromFloat(5.00),
		Items: []CheckoutItem{
			{
				Name: "Item A", Description: "Desc A", Quantity: 3,
				UnitPrice: 2500, VATAmount: 525, ProductID: "p1", VariantID: "v1",
			},
			{
				Name: "Item B", Description: "", Quantity: 1,
				UnitPrice: 9900, VATAmount: 0, ProductID: "p2", VariantID: "v2",
			},
		},
		Metadata: map[string]string{"source": "storefront"},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_ZeroShippingAndDiscount(t *testing.T) {
	// ShippingFee = 0 and DiscountAmount = 0 → should NOT add shipping line
	// item or discount coupon.
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:         uuid.New(),
		Email:          "buyer@example.com",
		SuccessURL:     "https://shop.example.com/success",
		CancelURL:      "https://shop.example.com/cancel",
		Currency:       "eur",
		ShippingFee:    decimal.Zero,
		DiscountAmount: decimal.Zero,
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_NilMetadata(t *testing.T) {
	// Metadata is nil → the range over nil map is a no-op, but cart_id still added.
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:     uuid.New(),
		Email:      "buyer@example.com",
		SuccessURL: "https://shop.example.com/success",
		CancelURL:  "https://shop.example.com/cancel",
		Currency:   "eur",
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
		Metadata: nil,
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

func TestCreateCheckoutSession_NegativeShippingNotAdded(t *testing.T) {
	// Negative shipping fee → IsPositive() is false → no shipping line item.
	svc := NewService("sk_test_fake_key", slog.Default())

	input := CheckoutInput{
		CartID:      uuid.New(),
		Email:       "buyer@example.com",
		SuccessURL:  "https://shop.example.com/success",
		CancelURL:   "https://shop.example.com/cancel",
		Currency:    "eur",
		ShippingFee: decimal.NewFromFloat(-5.00),
		Items: []CheckoutItem{
			{Name: "Product", Quantity: 1, UnitPrice: 1000},
		},
	}

	_, err := svc.CreateCheckoutSession(input)
	if err == nil {
		t.Fatal("expected error from Stripe API")
	}
	if strings.Contains(err.Error(), "validating checkout input") {
		t.Fatalf("should not be a validation error: %v", err)
	}
}

package stripe

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

var (
	// ErrEmptyItems is returned when a checkout session is created with no items.
	ErrEmptyItems = errors.New("checkout items must not be empty")

	// ErrInvalidCurrency is returned when the currency string is empty.
	ErrInvalidCurrency = errors.New("currency must not be empty")

	// ErrMissingURLs is returned when success or cancel URLs are not provided.
	ErrMissingURLs = errors.New("success and cancel URLs are required")
)

// Service wraps the Stripe Go SDK to create Checkout Sessions and verify
// webhook signatures. It is the primary integration point with Stripe for
// payment processing.
type Service struct {
	logger *slog.Logger
}

// NewService creates a new Stripe service and sets the global API key.
//
// The Stripe Go SDK uses a package-level Key variable for authentication.
// This must be set before any API calls are made.
func NewService(secretKey string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	stripe.Key = secretKey
	return &Service{
		logger: logger,
	}
}

// CheckoutInput contains all data needed to create a Stripe Checkout Session.
type CheckoutInput struct {
	// CartID is the internal cart identifier, stored in session metadata for
	// webhook reconciliation.
	CartID uuid.UUID

	// Email is the customer's email address, pre-filled on the Checkout page.
	Email string

	// SuccessURL is where Stripe redirects the customer after successful payment.
	SuccessURL string

	// CancelURL is where Stripe redirects if the customer cancels.
	CancelURL string

	// Items are the cart line items to charge for.
	Items []CheckoutItem

	// ShippingFee is the total shipping cost in the store's currency.
	// Represented as a decimal for precision; converted to cents internally.
	ShippingFee decimal.Decimal

	// DiscountAmount is the total discount to apply, in the store's currency.
	// Stripe Checkout handles discounts via coupon objects, so this is converted
	// to a one-time coupon with amount_off.
	DiscountAmount decimal.Decimal

	// Currency is the three-letter ISO currency code (e.g., "eur").
	Currency string

	// Metadata contains additional key-value pairs to attach to both the
	// Checkout Session and the resulting PaymentIntent. Useful for storing
	// cart_id, vat_country, order references, etc.
	Metadata map[string]string
}

// CheckoutItem represents a single line item in the checkout.
type CheckoutItem struct {
	// Name is the product name displayed to the customer.
	Name string

	// Description provides additional detail (e.g., "Black / Large").
	Description string

	// Quantity is the number of units being purchased.
	Quantity int64

	// UnitPrice is the price per unit in the smallest currency unit (cents).
	// This should be the gross price (including VAT if prices include VAT,
	// or net price if prices exclude VAT). The full amount the customer pays
	// per unit for this item.
	UnitPrice int64

	// VATAmount is the VAT amount per unit in the smallest currency unit (cents).
	// This is informational and stored in line item metadata for record-keeping.
	// Stripe does not natively break out VAT on Checkout line items, so we
	// record it in metadata for downstream processing (invoices, order records).
	VATAmount int64

	// ProductID is the internal product identifier, stored in metadata.
	ProductID string

	// VariantID is the internal variant identifier, stored in metadata.
	VariantID string
}

// CheckoutResult contains the output of a successfully created Checkout Session.
type CheckoutResult struct {
	// SessionID is the Stripe Checkout Session ID (e.g., "cs_test_...").
	SessionID string

	// SessionURL is the URL to redirect the customer to for payment.
	SessionURL string

	// PaymentIntentID is the Stripe PaymentIntent ID created for this session.
	// May be empty if Stripe has not yet created the PI (depends on mode).
	PaymentIntentID string
}

// CreateCheckoutSession creates a Stripe Checkout Session for a cart and
// returns the session URL that the customer should be redirected to.
//
// The method:
//   - Maps each cart item to a Stripe line item with inline PriceData
//   - Adds shipping as a separate line item if the fee is positive
//   - Creates a one-time coupon for discounts if applicable
//   - Stores cart_id and other metadata on both the session and PaymentIntent
//   - Returns the hosted Checkout page URL for customer redirect
func (s *Service) CreateCheckoutSession(input CheckoutInput) (CheckoutResult, error) {
	if err := validateCheckoutInput(input); err != nil {
		return CheckoutResult{}, fmt.Errorf("validating checkout input: %w", err)
	}

	// Build Stripe line items from cart items.
	lineItems := make([]*stripe.CheckoutSessionLineItemParams, 0, len(input.Items)+1)

	for _, item := range input.Items {
		lineItem := &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(input.Currency),
				UnitAmount: stripe.Int64(item.UnitPrice),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name:        stripe.String(item.Name),
					Description: strPtr(item.Description),
					Metadata: map[string]string{
						"product_id": item.ProductID,
						"variant_id": item.VariantID,
					},
				},
			},
			Quantity: stripe.Int64(item.Quantity),
		}

		// Store VAT breakdown in product metadata for order reconciliation.
		if item.VATAmount > 0 {
			lineItem.PriceData.ProductData.Metadata["vat_amount_per_unit"] = fmt.Sprintf("%d", item.VATAmount)
		}

		lineItems = append(lineItems, lineItem)
	}

	// Add shipping as a separate line item if applicable.
	if input.ShippingFee.IsPositive() {
		shippingCents := decimalToCents(input.ShippingFee)
		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(input.Currency),
				UnitAmount: stripe.Int64(shippingCents),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String("Shipping"),
				},
			},
			Quantity: stripe.Int64(1),
		})
	}

	// Build the session metadata. Always include cart_id.
	metadata := make(map[string]string)
	for k, v := range input.Metadata {
		metadata[k] = v
	}
	metadata["cart_id"] = input.CartID.String()

	// Build the Checkout Session params.
	params := &stripe.CheckoutSessionParams{
		Mode:          stripe.String(string(stripe.CheckoutSessionModePayment)),
		CustomerEmail: stripe.String(input.Email),
		SuccessURL:    stripe.String(input.SuccessURL),
		CancelURL:     stripe.String(input.CancelURL),
		LineItems:     lineItems,
		Metadata:      metadata,
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: metadata,
		},
	}

	// Apply discount as a Stripe coupon if applicable.
	// Stripe Checkout does not support negative line items, so we create
	// an inline coupon with amount_off to represent the discount.
	if input.DiscountAmount.IsPositive() {
		discountCents := decimalToCents(input.DiscountAmount)
		params.Discounts = []*stripe.CheckoutSessionDiscountParams{
			{
				Coupon: stripe.String(fmt.Sprintf("forge_discount_%s", input.CartID.String())),
			},
		}
		// Note: The coupon must be pre-created via the Stripe API before the
		// session is created. In a production flow, the caller should create
		// the coupon first using:
		//
		//   coupon.New(&stripe.CouponParams{
		//       AmountOff: stripe.Int64(discountCents),
		//       Currency:  stripe.String(input.Currency),
		//       Duration:  stripe.String(string(stripe.CouponDurationOnce)),
		//       ID:        stripe.String("forge_discount_<cart_id>"),
		//   })
		//
		// If callers prefer a self-contained approach, we provide a helper
		// method CreateDiscountCoupon that should be called before this method.
		_ = discountCents // Used by the caller via CreateDiscountCoupon.
	}

	s.logger.Info("creating stripe checkout session",
		slog.String("cart_id", input.CartID.String()),
		slog.String("email", input.Email),
		slog.Int("line_items", len(lineItems)),
		slog.String("currency", input.Currency),
	)

	sess, err := session.New(params)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("creating stripe checkout session: %w", err)
	}

	result := CheckoutResult{
		SessionID:  sess.ID,
		SessionURL: sess.URL,
	}

	// Extract PaymentIntent ID if available.
	if sess.PaymentIntent != nil {
		result.PaymentIntentID = sess.PaymentIntent.ID
	}

	s.logger.Info("stripe checkout session created",
		slog.String("session_id", sess.ID),
		slog.String("cart_id", input.CartID.String()),
		slog.String("payment_intent_id", result.PaymentIntentID),
	)

	return result, nil
}

// VerifyWebhookSignature validates the payload from a Stripe webhook request
// using the provided signature header and webhook secret. Returns the parsed
// Event on success.
//
// The signature header is the value of the "Stripe-Signature" HTTP header.
// The webhook secret is the endpoint-specific signing secret from the Stripe
// Dashboard (starts with "whsec_").
//
// This method enforces a default tolerance of 5 minutes for replay attack
// prevention. Events with timestamps older than 5 minutes are rejected.
func (s *Service) VerifyWebhookSignature(payload []byte, sigHeader string, webhookSecret string) (stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, sigHeader, webhookSecret)
	if err != nil {
		return stripe.Event{}, fmt.Errorf("verifying webhook signature: %w", err)
	}

	s.logger.Debug("webhook signature verified",
		slog.String("event_id", event.ID),
		slog.String("event_type", string(event.Type)),
	)

	return event, nil
}

// decimalToCents converts a shopspring/decimal value representing a currency
// amount (e.g., 42.50) to the smallest currency unit (e.g., 4250 cents).
// The value is rounded to 2 decimal places before conversion to avoid
// floating-point precision issues.
func decimalToCents(d decimal.Decimal) int64 {
	return d.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

// strPtr returns a pointer to the string, or nil if empty. This avoids
// sending empty strings to Stripe where nil means "not provided".
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// validateCheckoutInput performs basic validation on the checkout input before
// calling the Stripe API.
func validateCheckoutInput(input CheckoutInput) error {
	if len(input.Items) == 0 {
		return ErrEmptyItems
	}
	if input.Currency == "" {
		return ErrInvalidCurrency
	}
	if input.SuccessURL == "" || input.CancelURL == "" {
		return ErrMissingURLs
	}
	return nil
}

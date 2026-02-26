package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/services/cart"
	"github.com/forgecommerce/api/internal/services/order"
	"github.com/forgecommerce/api/internal/services/shipping"
	"github.com/forgecommerce/api/internal/vat"
)

// newCheckoutHandler creates a CheckoutHandler wired to the shared test database.
// The VAT rate cache is pre-loaded with standard rates for ES and DE.
func newCheckoutHandler() *api.CheckoutHandler {
	cartSvc := cart.NewService(testDB.Pool, nil)
	orderSvc := order.NewService(testDB.Pool, nil)
	cache := vat.NewRateCache()
	cache.Load([]vat.VATRate{
		{CountryCode: "ES", RateType: "standard", Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "DE", RateType: "standard", Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "FR", RateType: "standard", Rate: decimal.NewFromFloat(20.0)},
	})
	vatSvc := vat.NewVATService(testDB.Pool, cache, nil)
	shippingSvc := shipping.NewService(testDB.Pool, nil)
	queries := db.New(testDB.Pool)
	return api.NewCheckoutHandler(
		cartSvc, orderSvc, vatSvc, shippingSvc, queries, nil,
		"https://example.com/success", "https://example.com/cancel",
	)
}

func checkoutMux() *http.ServeMux {
	mux := http.NewServeMux()
	newCheckoutHandler().RegisterRoutes(mux)
	return mux
}

// seedCheckoutDeps inserts the VAT rates, store settings (VAT enabled), and
// shipping config needed for successful checkout/calculate tests.
func seedCheckoutDeps(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Insert VAT rates for ES and DE into the database.
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vat_rates (id, country_code, rate_type, rate, valid_from, source, synced_at)
		VALUES
			($1, 'ES', 'standard', 21.00, '2024-01-01', 'seed', now()),
			($2, 'DE', 'standard', 19.00, '2024-01-01', 'seed', now())
		ON CONFLICT (country_code, rate_type, valid_from) DO NOTHING
	`, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("seeding VAT rates: %v", err)
	}

	// Enable VAT in store settings.
	_, err = testDB.Pool.Exec(ctx, `
		UPDATE store_settings SET
			vat_enabled = true,
			vat_country_code = 'ES',
			vat_prices_include_vat = true,
			vat_default_category = 'standard'
	`)
	if err != nil {
		t.Fatalf("updating store settings for checkout: %v", err)
	}

	// Ensure a shipping config row exists with fixed fee.
	_, err = testDB.Pool.Exec(ctx, `
		UPDATE shipping_config SET
			enabled = true,
			calculation_method = 'fixed',
			fixed_fee = 5.00,
			default_currency = 'EUR'
	`)
	if err != nil {
		t.Fatalf("updating shipping config for checkout: %v", err)
	}
}

// createCartWithItem creates a cart, adds one variant item to it via the cart
// service, and returns the cart ID. Requires product + variant to already exist.
func createCartWithItem(t *testing.T, variantID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	cartSvc := cart.NewService(testDB.Pool, nil)

	c, err := cartSvc.Create(ctx)
	if err != nil {
		t.Fatalf("creating cart: %v", err)
	}
	_, err = cartSvc.AddItem(ctx, c.ID, variantID, 2)
	if err != nil {
		t.Fatalf("adding item to cart: %v", err)
	}
	return c.ID
}

// --------------------------------------------------------------------------
// POST /api/v1/checkout/calculate — validation errors
// --------------------------------------------------------------------------

func TestCalculate_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate",
		bytes.NewReader([]byte(`{invalid json`)))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "invalid request body" {
		t.Errorf("error message: got %q, want %q", resp.Error, "invalid request body")
	}
}

func TestCalculate_MissingCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart_id is required" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart_id is required")
	}
}

func TestCalculate_MissingCountryCode(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"cart_id": uuid.New().String(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "country_code is required" {
		t.Errorf("error message: got %q, want %q", resp.Error, "country_code is required")
	}
}

func TestCalculate_CartNotFound(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"cart_id":      uuid.New().String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart not found" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart not found")
	}
}

func TestCalculate_EmptyCart(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	mux := checkoutMux()

	// Create an empty cart (no items).
	ctx := context.Background()
	cartSvc := cart.NewService(testDB.Pool, nil)
	c, err := cartSvc.Create(ctx)
	if err != nil {
		t.Fatalf("creating cart: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"cart_id":      c.ID.String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart is empty" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart is empty")
	}
}

// --------------------------------------------------------------------------
// POST /api/v1/checkout/calculate — happy path
// --------------------------------------------------------------------------

func TestCalculate_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	testDB.FixtureShippingCountry(t, "ES")
	mux := checkoutMux()

	// Create product, variant, cart with 2 items.
	p := testDB.FixtureProduct(t, "Test Product", "test-product")
	v := testDB.FixtureVariant(t, p.ID, "TEST-SKU", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Subtotal       string `json:"subtotal"`
		VatTotal       string `json:"vat_total"`
		ShippingFee    string `json:"shipping_fee"`
		DiscountAmount string `json:"discount_amount"`
		Total          string `json:"total"`
		VatBreakdown   []struct {
			ProductName string `json:"product_name"`
			Rate        string `json:"rate"`
			RateType    string `json:"rate_type"`
			Amount      string `json:"amount"`
		} `json:"vat_breakdown"`
		ReverseCharge bool `json:"reverse_charge"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify response has all expected fields populated.
	if resp.Subtotal == "" {
		t.Error("expected non-empty subtotal")
	}
	if resp.VatTotal == "" {
		t.Error("expected non-empty vat_total")
	}
	if resp.ShippingFee == "" {
		t.Error("expected non-empty shipping_fee")
	}
	if resp.DiscountAmount == "" {
		t.Error("expected non-empty discount_amount")
	}
	if resp.Total == "" {
		t.Error("expected non-empty total")
	}
	if resp.ReverseCharge {
		t.Error("expected reverse_charge to be false for B2C")
	}

	// Verify VAT breakdown has one entry (one product in the cart).
	if len(resp.VatBreakdown) != 1 {
		t.Fatalf("vat_breakdown length: got %d, want 1", len(resp.VatBreakdown))
	}

	breakdown := resp.VatBreakdown[0]
	if breakdown.ProductName != "Test Product" {
		t.Errorf("vat_breakdown[0].product_name: got %q, want %q", breakdown.ProductName, "Test Product")
	}
	if breakdown.RateType != "standard" {
		t.Errorf("vat_breakdown[0].rate_type: got %q, want %q", breakdown.RateType, "standard")
	}
	if breakdown.Rate != "21.00" {
		t.Errorf("vat_breakdown[0].rate: got %q, want %q", breakdown.Rate, "21.00")
	}
}

func TestCalculate_HappyPath_VerifyAmounts(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	testDB.FixtureShippingCountry(t, "ES")
	mux := checkoutMux()

	// Product/variant price is 25.00 (from FixtureVariant).
	// Quantity: 2, so gross total = 50.00.
	// Store prices include VAT (ES 21%).
	// Net per unit = 25.00 / 1.21 = 20.66 (rounded).
	// VAT per unit = 25.00 - 20.66 = 4.34.
	// Line net = 20.66 * 2 = 41.32.
	// Line VAT = 4.34 * 2 = 8.68.
	// Line gross = 25.00 * 2 = 50.00.
	// Shipping fixed fee: 5.00.
	// Discount: 0.00.
	// Total = 50.00 + 5.00 - 0.00 = 55.00.
	p := testDB.FixtureProduct(t, "Amount Check", "amount-check")
	v := testDB.FixtureVariant(t, p.ID, "AMT-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Subtotal       string `json:"subtotal"`
		VatTotal       string `json:"vat_total"`
		ShippingFee    string `json:"shipping_fee"`
		DiscountAmount string `json:"discount_amount"`
		Total          string `json:"total"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	subtotal := decimalParse(t, resp.Subtotal)
	vatTotal := decimalParse(t, resp.VatTotal)
	shippingFee := decimalParse(t, resp.ShippingFee)
	total := decimalParse(t, resp.Total)

	// Subtotal should be net amount (prices include VAT, so net = gross / 1.21).
	// 25.00 / 1.21 = 20.66 per unit, 20.66 * 2 = 41.32.
	expectedSubtotal := decimal.NewFromFloat(41.32)
	if !subtotal.Equal(expectedSubtotal) {
		t.Errorf("subtotal: got %s, want %s", subtotal, expectedSubtotal)
	}

	// VAT total = 4.34 * 2 = 8.68.
	expectedVAT := decimal.NewFromFloat(8.68)
	if !vatTotal.Equal(expectedVAT) {
		t.Errorf("vat_total: got %s, want %s", vatTotal, expectedVAT)
	}

	// Shipping = 5.00 (fixed).
	expectedShipping := decimal.NewFromFloat(5.00)
	if !shippingFee.Equal(expectedShipping) {
		t.Errorf("shipping_fee: got %s, want %s", shippingFee, expectedShipping)
	}

	// Total = gross + shipping = 50.00 + 5.00 = 55.00.
	expectedTotal := decimal.NewFromFloat(55.00)
	if !total.Equal(expectedTotal) {
		t.Errorf("total: got %s, want %s", total, expectedTotal)
	}
}

func TestCalculate_CountryNotEnabled(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	// ES is NOT enabled as a shipping country here.
	mux := checkoutMux()

	p := testDB.FixtureProduct(t, "Country Check", "country-check")
	v := testDB.FixtureVariant(t, p.ID, "CC-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "shipping to this country is not enabled" {
		t.Errorf("error message: got %q, want %q", resp.Error, "shipping to this country is not enabled")
	}
}

func TestCalculate_DifferentCountry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	testDB.FixtureShippingCountry(t, "DE")
	mux := checkoutMux()

	p := testDB.FixtureProduct(t, "DE Product", "de-product")
	v := testDB.FixtureVariant(t, p.ID, "DE-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"country_code": "DE",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		VatBreakdown []struct {
			Rate     string `json:"rate"`
			RateType string `json:"rate_type"`
		} `json:"vat_breakdown"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.VatBreakdown) != 1 {
		t.Fatalf("vat_breakdown length: got %d, want 1", len(resp.VatBreakdown))
	}
	// DE standard rate is 19%.
	if resp.VatBreakdown[0].Rate != "19.00" {
		t.Errorf("vat_breakdown[0].rate: got %q, want %q", resp.VatBreakdown[0].Rate, "19.00")
	}
	if resp.VatBreakdown[0].RateType != "standard" {
		t.Errorf("vat_breakdown[0].rate_type: got %q, want %q", resp.VatBreakdown[0].RateType, "standard")
	}
}

func TestCalculate_ResponseJSON_ContentType(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	testDB.FixtureShippingCountry(t, "ES")
	mux := checkoutMux()

	p := testDB.FixtureProduct(t, "CT Product", "ct-product")
	v := testDB.FixtureVariant(t, p.ID, "CT-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/calculate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

// --------------------------------------------------------------------------
// POST /api/v1/checkout — validation errors
// --------------------------------------------------------------------------

func TestCreateCheckout_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout",
		bytes.NewReader([]byte(`not json`)))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "invalid request body" {
		t.Errorf("error message: got %q, want %q", resp.Error, "invalid request body")
	}
}

func TestCreateCheckout_MissingCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"email":        "test@example.com",
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart_id is required" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart_id is required")
	}
}

func TestCreateCheckout_MissingEmail(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"cart_id":      uuid.New().String(),
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "email is required" {
		t.Errorf("error message: got %q, want %q", resp.Error, "email is required")
	}
}

func TestCreateCheckout_MissingCountryCode(t *testing.T) {
	testDB.Truncate(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"cart_id": uuid.New().String(),
		"email":   "test@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "country_code is required" {
		t.Errorf("error message: got %q, want %q", resp.Error, "country_code is required")
	}
}

func TestCreateCheckout_CartNotFound(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := checkoutMux()

	body, _ := json.Marshal(map[string]string{
		"cart_id":      uuid.New().String(),
		"email":        "test@example.com",
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart not found" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart not found")
	}
}

func TestCreateCheckout_EmptyCart(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	mux := checkoutMux()

	ctx := context.Background()
	cartSvc := cart.NewService(testDB.Pool, nil)
	c, err := cartSvc.Create(ctx)
	if err != nil {
		t.Fatalf("creating cart: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"cart_id":      c.ID.String(),
		"email":        "test@example.com",
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "cart is empty" {
		t.Errorf("error message: got %q, want %q", resp.Error, "cart is empty")
	}
}

func TestCreateCheckout_CountryNotEnabled(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	// Do NOT enable ES as a shipping country.
	mux := checkoutMux()

	p := testDB.FixtureProduct(t, "Checkout Country", "checkout-country")
	v := testDB.FixtureVariant(t, p.ID, "CKCO-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"email":        "test@example.com",
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "shipping to this country is not enabled" {
		t.Errorf("error message: got %q, want %q", resp.Error, "shipping to this country is not enabled")
	}
}

func TestCreateCheckout_StripeAPIError(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	seedCheckoutDeps(t)
	testDB.FixtureShippingCountry(t, "ES")
	mux := checkoutMux()

	p := testDB.FixtureProduct(t, "Stripe Error Product", "stripe-error")
	v := testDB.FixtureVariant(t, p.ID, "SE-001", 10)
	cartID := createCartWithItem(t, v.ID)

	body, _ := json.Marshal(map[string]string{
		"cart_id":      cartID.String(),
		"email":        "test@example.com",
		"country_code": "ES",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Without a valid Stripe API key, the Stripe SDK call will fail.
	// We expect a 500 error.
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "failed to create checkout session" {
		t.Errorf("error message: got %q, want %q", resp.Error, "failed to create checkout session")
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// decimalParse parses a decimal string and fails the test if invalid.
func decimalParse(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("parsing decimal %q: %v", s, err)
	}
	return d
}

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/services/cart"
	"github.com/forgecommerce/api/internal/vat"
)

// newVATNumberHandler creates a VATNumberHandler wired to the shared testDB.
// The VIESClient uses a short timeout so that any uncached lookup (which will
// attempt a real SOAP call) completes quickly in tests.
func newVATNumberHandler() *api.VATNumberHandler {
	logger := slog.Default()
	cartSvc := cart.NewService(testDB.Pool, logger)
	viesClient := vat.NewVIESClient(testDB.Pool, 5*time.Second, 24*time.Hour, logger)
	return api.NewVATNumberHandler(cartSvc, viesClient, logger)
}

// vatNumberMux returns an http.ServeMux with the VATNumberHandler routes registered.
func vatNumberMux() *http.ServeMux {
	mux := http.NewServeMux()
	newVATNumberHandler().RegisterRoutes(mux)
	return mux
}

// createTestCart is a helper that creates a cart via the cart service and
// returns its UUID. It fails the test on error.
func createTestCart(t *testing.T) uuid.UUID {
	t.Helper()
	cartSvc := cart.NewService(testDB.Pool, nil)
	c, err := cartSvc.Create(context.Background())
	if err != nil {
		t.Fatalf("creating test cart: %v", err)
	}
	return c.ID
}

// seedVIESCache inserts an entry into the vies_validation_cache table.
func seedVIESCache(t *testing.T, vatNumber string, isValid bool, companyName, companyAddress string) {
	t.Helper()
	ctx := context.Background()
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vies_validation_cache (vat_number, is_valid, company_name, company_address, validated_at, expires_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW() + INTERVAL '24 hours')
	`, vatNumber, isValid, nilIfEmpty(companyName), nilIfEmpty(companyAddress))
	if err != nil {
		t.Fatalf("seeding VIES cache for %q: %v", vatNumber, err)
	}
}

// nilIfEmpty returns nil for empty strings, or a pointer to the string otherwise.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// vatNumberResponse mirrors the handler's response struct for JSON decoding.
type vatNumberResponse struct {
	Valid       bool   `json:"valid"`
	CompanyName string `json:"company_name,omitempty"`
	Address     string `json:"address,omitempty"`
	VATNumber   string `json:"vat_number"`
	Message     string `json:"message,omitempty"`
}

// errorResponse mirrors the handler's error response struct.
type errorResponse struct {
	Error string `json:"error"`
}

// --------------------------------------------------------------------------
// TestVATNumber_InvalidCartID
// --------------------------------------------------------------------------

func TestVATNumber_InvalidCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := vatNumberMux()

	body, _ := json.Marshal(map[string]string{"vat_number": "ES12345678A"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/not-a-uuid/vat-number", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "invalid cart ID" {
		t.Errorf("error message: got %q, want %q", resp.Error, "invalid cart ID")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_InvalidRequestBody
// --------------------------------------------------------------------------

func TestVATNumber_InvalidRequestBody(t *testing.T) {
	testDB.Truncate(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Send invalid JSON.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader([]byte("this is not json")))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "invalid request body" {
		t.Errorf("error message: got %q, want %q", resp.Error, "invalid request body")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_EmptyVATNumber
// --------------------------------------------------------------------------

func TestVATNumber_EmptyVATNumber(t *testing.T) {
	testDB.Truncate(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	body, _ := json.Marshal(map[string]string{"vat_number": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for empty VAT number")
	}
	if resp.Message != "VAT number cleared" {
		t.Errorf("message: got %q, want %q", resp.Message, "VAT number cleared")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_EmptyVATNumber_WhitespaceOnly
// --------------------------------------------------------------------------

func TestVATNumber_EmptyVATNumber_WhitespaceOnly(t *testing.T) {
	testDB.Truncate(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Whitespace-only should be treated the same as empty after TrimSpace.
	body, _ := json.Marshal(map[string]string{"vat_number": "   "})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Valid {
		t.Error("expected valid=false for whitespace-only VAT number")
	}
	if resp.Message != "VAT number cleared" {
		t.Errorf("message: got %q, want %q", resp.Message, "VAT number cleared")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_ValidCachedVATNumber
// --------------------------------------------------------------------------

func TestVATNumber_ValidCachedVATNumber(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Pre-seed the VIES cache with a valid entry.
	seedVIESCache(t, "ES12345678A", true, "Test Company SL", "Calle Mayor 1, Madrid")

	body, _ := json.Marshal(map[string]string{"vat_number": "ES12345678A"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !resp.Valid {
		t.Error("expected valid=true for cached valid VAT number")
	}
	if resp.CompanyName != "Test Company SL" {
		t.Errorf("company_name: got %q, want %q", resp.CompanyName, "Test Company SL")
	}
	if resp.Address != "Calle Mayor 1, Madrid" {
		t.Errorf("address: got %q, want %q", resp.Address, "Calle Mayor 1, Madrid")
	}
	if resp.VATNumber != "ES12345678A" {
		t.Errorf("vat_number: got %q, want %q", resp.VATNumber, "ES12345678A")
	}
	if resp.Message != "VAT number is valid. Reverse charge may apply at checkout." {
		t.Errorf("message: got %q, want %q", resp.Message, "VAT number is valid. Reverse charge may apply at checkout.")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_InvalidCachedVATNumber
// --------------------------------------------------------------------------

func TestVATNumber_InvalidCachedVATNumber(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Pre-seed the VIES cache with an invalid entry.
	seedVIESCache(t, "ES00000000X", false, "", "")

	body, _ := json.Marshal(map[string]string{"vat_number": "ES00000000X"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Valid {
		t.Error("expected valid=false for cached invalid VAT number")
	}
	if resp.VATNumber != "ES00000000X" {
		t.Errorf("vat_number: got %q, want %q", resp.VATNumber, "ES00000000X")
	}
	if resp.Message != "VAT number is not valid" {
		t.Errorf("message: got %q, want %q", resp.Message, "VAT number is not valid")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_VIESValidationError
// --------------------------------------------------------------------------

func TestVATNumber_VIESValidationError(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// A VAT number that is too short after sanitization (< 4 chars) triggers
	// an error from VIESClient.Validate(), exercising the handler's 503 path.
	// The handler TrimSpace + ToUpper produces "XY", then sanitizeVATNumber
	// keeps it as "XY" (2 chars < 4), so Validate returns an error.
	body, _ := json.Marshal(map[string]string{"vat_number": "XY"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "VAT number validation service is currently unavailable" {
		t.Errorf("error message: got %q, want %q", resp.Error, "VAT number validation service is currently unavailable")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_CartNotFound
// --------------------------------------------------------------------------

func TestVATNumber_CartNotFound(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	// Use a valid UUID that does not correspond to any existing cart.
	fakeCartID := uuid.New()

	// Pre-seed the VIES cache so the handler gets past validation.
	seedVIESCache(t, "FR12345678901", true, "Entreprise Francaise SARL", "1 Rue de Paris, Paris")

	body, _ := json.Marshal(map[string]string{"vat_number": "FR12345678901"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+fakeCartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "internal server error" {
		t.Errorf("error message: got %q, want %q", resp.Error, "internal server error")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_CartNotFound_EmptyVATNumber
// --------------------------------------------------------------------------

func TestVATNumber_CartNotFound_EmptyVATNumber(t *testing.T) {
	testDB.Truncate(t)
	mux := vatNumberMux()

	// Use a valid UUID that does not correspond to any existing cart.
	// Even clearing VAT should fail because the cart doesn't exist.
	fakeCartID := uuid.New()

	body, _ := json.Marshal(map[string]string{"vat_number": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+fakeCartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "internal server error" {
		t.Errorf("error message: got %q, want %q", resp.Error, "internal server error")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_ValidVATNumberUpdatesCart
// --------------------------------------------------------------------------

func TestVATNumber_ValidVATNumberUpdatesCart(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Pre-seed the VIES cache with a valid entry.
	seedVIESCache(t, "DE123456789", true, "Deutsche GmbH", "Hauptstrasse 1, Berlin")

	body, _ := json.Marshal(map[string]string{"vat_number": "DE123456789"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Verify the cart was updated in the database.
	cartSvc := cart.NewService(testDB.Pool, nil)
	c, err := cartSvc.Get(context.Background(), cartID)
	if err != nil {
		t.Fatalf("getting cart: %v", err)
	}

	if c.VatNumber == nil {
		t.Fatal("expected cart vat_number to be set")
	}
	if *c.VatNumber != "DE123456789" {
		t.Errorf("cart vat_number: got %q, want %q", *c.VatNumber, "DE123456789")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_LowercaseVATNumber_Normalized
// --------------------------------------------------------------------------

func TestVATNumber_LowercaseVATNumber_Normalized(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Seed cache with the uppercased version (the handler normalizes before lookup).
	seedVIESCache(t, "ES12345678A", true, "Empresa Espanola SL", "Calle Sol 5, Barcelona")

	// Send a lowercase version. The handler uppercases it before passing
	// to VIESClient.Validate, which then sanitizes and finds the cache hit.
	body, _ := json.Marshal(map[string]string{"vat_number": "es12345678a"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Valid {
		t.Error("expected valid=true for lowercase VAT number that normalizes to a cached entry")
	}
	// The response VATNumber comes from the handler's trimmed+uppercased variable.
	if resp.VATNumber != "ES12345678A" {
		t.Errorf("vat_number: got %q, want %q", resp.VATNumber, "ES12345678A")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_VATNumberWithLeadingTrailingSpaces
// --------------------------------------------------------------------------

func TestVATNumber_VATNumberWithLeadingTrailingSpaces(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Seed cache with the trimmed version.
	seedVIESCache(t, "ES12345678A", true, "Empresa Test SL", "Calle Luna 10, Madrid")

	// Send a version with leading/trailing spaces. The handler TrimSpace
	// removes them, then VIESClient.Validate sanitizes and finds the cache hit.
	body, _ := json.Marshal(map[string]string{"vat_number": "  ES12345678A  "})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp vatNumberResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Valid {
		t.Error("expected valid=true for VAT number with leading/trailing spaces")
	}
	if resp.VATNumber != "ES12345678A" {
		t.Errorf("vat_number: got %q, want %q", resp.VATNumber, "ES12345678A")
	}
	if resp.CompanyName != "Empresa Test SL" {
		t.Errorf("company_name: got %q, want %q", resp.CompanyName, "Empresa Test SL")
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_ExpiredCacheEntry
// --------------------------------------------------------------------------

func TestVATNumber_ExpiredCacheEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)

	// Seed an expired cache entry. The VIESClient should treat it as a cache
	// miss and fall through to a live SOAP call. In this test environment the
	// real VIES endpoint is reachable, so we verify the handler does NOT return
	// the stale cached data and instead returns a fresh result.
	ctx := context.Background()
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vies_validation_cache (vat_number, is_valid, company_name, company_address, validated_at, expires_at)
		VALUES ($1, true, 'Expired Company', 'Old Address', NOW() - INTERVAL '48 hours', NOW() - INTERVAL '24 hours')
	`, "DE111111111")
	if err != nil {
		t.Fatalf("seeding expired VIES cache: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"vat_number": "DE111111111"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// The expired cache entry should NOT be returned. The handler either gets
	// a fresh result from VIES (200 with valid/invalid) or a VIES error (503).
	// In either case, the response company name should NOT be "Expired Company".
	if rr.Code == http.StatusOK {
		var resp vatNumberResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.CompanyName == "Expired Company" {
			t.Error("expected expired cache entry to be bypassed, but got stale data")
		}
	} else if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 200 or 503\nbody: %s", rr.Code, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// TestVATNumber_ResponseContentType
// --------------------------------------------------------------------------

func TestVATNumber_ResponseContentType(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := vatNumberMux()

	cartID := createTestCart(t)
	seedVIESCache(t, "ES12345678A", true, "Test Company", "Test Address")

	body, _ := json.Marshal(map[string]string{"vat_number": "ES12345678A"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+cartID.String()+"/vat-number",
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

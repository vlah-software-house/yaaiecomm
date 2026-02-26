package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/services/cart"
)

func newCartHandler() *api.CartHandler {
	cartSvc := cart.NewService(testDB.Pool, nil)
	return api.NewCartHandler(cartSvc, nil)
}

func cartMux() *http.ServeMux {
	mux := http.NewServeMux()
	newCartHandler().RegisterRoutes(mux)
	return mux
}

// --------------------------------------------------------------------------
// CreateCart
// --------------------------------------------------------------------------

func TestCreateCart(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		ID        string `json:"id"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, err := uuid.Parse(resp.ID); err != nil {
		t.Errorf("invalid cart ID: %q", resp.ID)
	}
	if resp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

// --------------------------------------------------------------------------
// GetCart
// --------------------------------------------------------------------------

func TestGetCart(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	// Create a cart first.
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil)
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, createReq)

	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Get the cart.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		ID    string        `json:"id"`
		Items []interface{} `json:"items"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ID != created.ID {
		t.Errorf("id: got %q, want %q", resp.ID, created.ID)
	}
	if resp.Items == nil {
		t.Error("expected items array (even if empty)")
	}
}

func TestGetCart_NotFound(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetCart_InvalidID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// UpdateCart
// --------------------------------------------------------------------------

func TestUpdateCart(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	// Create a cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))

	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Update email and country.
	email := "test@example.com"
	body, _ := json.Marshal(map[string]string{
		"email":        email,
		"country_code": "DE",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/"+created.ID, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		ID        string `json:"id"`
		UpdatedAt string `json:"updated_at"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ID != created.ID {
		t.Errorf("id: got %q, want %q", resp.ID, created.ID)
	}
}

func TestUpdateCart_NotFound(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]string{"email": "x@y.com"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/"+uuid.New().String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// --------------------------------------------------------------------------
// AddItem
// --------------------------------------------------------------------------

func TestAddItem(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	// Need a product + variant.
	p := testDB.FixtureProduct(t, "Cart Product", "cart-product")
	v := testDB.FixtureVariant(t, p.ID, "CART-001", 50)

	// Create a cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Add an item.
	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   2,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var item struct {
		ID        string `json:"id"`
		VariantID string `json:"variant_id"`
		Quantity  int32  `json:"quantity"`
	}
	json.NewDecoder(rr.Body).Decode(&item)

	if item.VariantID != v.ID.String() {
		t.Errorf("variant_id: got %q, want %q", item.VariantID, v.ID.String())
	}
	if item.Quantity != 2 {
		t.Errorf("quantity: got %d, want 2", item.Quantity)
	}
}

func TestAddItem_CartWithItems(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Multi Item Product", "multi-item")
	v1 := testDB.FixtureVariant(t, p.ID, "MI-001", 50)
	v2 := testDB.FixtureVariant(t, p.ID, "MI-002", 30)

	// Create cart and add two different items.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	for _, vid := range []uuid.UUID{v1.ID, v2.ID} {
		body, _ := json.Marshal(map[string]interface{}{
			"variant_id": vid.String(),
			"quantity":   1,
		})
		addRR := httptest.NewRecorder()
		mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
			"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body)))
		if addRR.Code != http.StatusCreated {
			t.Fatalf("add item: got %d\nbody: %s", addRR.Code, addRR.Body.String())
		}
	}

	// Get cart and verify 2 items.
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil))

	var cartResp struct {
		Items []json.RawMessage `json:"items"`
	}
	json.NewDecoder(getRR.Body).Decode(&cartResp)

	if len(cartResp.Items) != 2 {
		t.Errorf("items: got %d, want 2", len(cartResp.Items))
	}
}

// --------------------------------------------------------------------------
// UpdateItem
// --------------------------------------------------------------------------

func TestUpdateItem(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Update Item Product", "update-item")
	v := testDB.FixtureVariant(t, p.ID, "UI-001", 50)

	// Create cart + add item.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	addBody, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   1,
	})
	addRR := httptest.NewRecorder()
	mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
		"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(addBody)))
	var addedItem struct {
		ID string `json:"id"`
	}
	json.NewDecoder(addRR.Body).Decode(&addedItem)

	// Update quantity to 5.
	updateBody, _ := json.Marshal(map[string]int{"quantity": 5})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", created.ID, addedItem.ID),
		bytes.NewReader(updateBody))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var updated struct {
		Quantity int32 `json:"quantity"`
	}
	json.NewDecoder(rr.Body).Decode(&updated)

	if updated.Quantity != 5 {
		t.Errorf("quantity: got %d, want 5", updated.Quantity)
	}
}

// --------------------------------------------------------------------------
// RemoveItem
// --------------------------------------------------------------------------

func TestRemoveItem(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Remove Item Product", "remove-item")
	v := testDB.FixtureVariant(t, p.ID, "RI-001", 50)

	// Create cart + add item.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	addBody, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   1,
	})
	addRR := httptest.NewRecorder()
	mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
		"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(addBody)))
	var addedItem struct {
		ID string `json:"id"`
	}
	json.NewDecoder(addRR.Body).Decode(&addedItem)

	// Remove the item.
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", created.ID, addedItem.ID), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	// Verify cart is empty.
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil))

	var cartResp struct {
		Items []json.RawMessage `json:"items"`
	}
	json.NewDecoder(getRR.Body).Decode(&cartResp)

	if len(cartResp.Items) != 0 {
		t.Errorf("items after removal: got %d, want 0", len(cartResp.Items))
	}
}

// ==========================================================================
// Additional tests for branch/error-path coverage
// ==========================================================================

// --------------------------------------------------------------------------
// CreateCart -- verify full response and roundtrip
// --------------------------------------------------------------------------

func TestCreateCart_ResponseFields(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp struct {
		ID        string `json:"id"`
		ExpiresAt string `json:"expires_at"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.CreatedAt == "" {
		t.Error("expected non-empty created_at")
	}
	// Verify the cart is actually retrievable.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+resp.ID, nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Errorf("GET after create: got %d, want %d", getRR.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// GetCart -- with items (covers item mapping loop + field mapping)
// --------------------------------------------------------------------------

func TestGetCart_WithItems(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Get Items Product", "get-items-product")
	v := testDB.FixtureVariant(t, p.ID, "GI-001", 100)

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Add item.
	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   3,
	})
	addRR := httptest.NewRecorder()
	mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
		"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body)))
	if addRR.Code != http.StatusCreated {
		t.Fatalf("add item: status %d, body: %s", addRR.Code, addRR.Body.String())
	}

	// Get cart and verify full response structure with items.
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil))

	if getRR.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", getRR.Code, http.StatusOK, getRR.Body.String())
	}

	var resp struct {
		ID          string  `json:"id"`
		CustomerID  *string `json:"customer_id"`
		Email       *string `json:"email"`
		CountryCode *string `json:"country_code"`
		VatNumber   *string `json:"vat_number"`
		CouponCode  *string `json:"coupon_code"`
		ExpiresAt   string  `json:"expires_at"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
		Items       []struct {
			ID           string `json:"id"`
			VariantID    string `json:"variant_id"`
			Quantity     int32  `json:"quantity"`
			VariantSku   string `json:"variant_sku"`
			VariantStock int32  `json:"variant_stock"`
			ProductID    string `json:"product_id"`
			ProductName  string `json:"product_name"`
			ProductSlug  string `json:"product_slug"`
		} `json:"items"`
	}
	if err := json.NewDecoder(getRR.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("items: got %d, want 1", len(resp.Items))
	}
	item := resp.Items[0]
	if item.VariantID != v.ID.String() {
		t.Errorf("variant_id: got %q, want %q", item.VariantID, v.ID.String())
	}
	if item.Quantity != 3 {
		t.Errorf("quantity: got %d, want 3", item.Quantity)
	}
	if item.VariantSku != "GI-001" {
		t.Errorf("variant_sku: got %q, want %q", item.VariantSku, "GI-001")
	}
	if item.ProductName != "Get Items Product" {
		t.Errorf("product_name: got %q, want %q", item.ProductName, "Get Items Product")
	}
	if item.ProductSlug != "get-items-product" {
		t.Errorf("product_slug: got %q, want %q", item.ProductSlug, "get-items-product")
	}
}

// --------------------------------------------------------------------------
// UpdateCart -- invalid cart ID (not a UUID)
// --------------------------------------------------------------------------

func TestUpdateCart_InvalidID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]string{"email": "a@b.com"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/not-a-uuid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// UpdateCart -- invalid JSON body
// --------------------------------------------------------------------------

func TestUpdateCart_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	// Create a cart first so the ID is valid.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/"+created.ID,
		strings.NewReader("{this is not valid json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// AddItem -- invalid cart ID (not a UUID)
// --------------------------------------------------------------------------

func TestAddItem_InvalidCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": uuid.New().String(),
		"quantity":   1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/not-a-uuid/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// AddItem -- invalid JSON body
// --------------------------------------------------------------------------

func TestAddItem_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	// Create a cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+created.ID+"/items",
		strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// AddItem -- quantity zero (gets clamped to 1 by handler)
// --------------------------------------------------------------------------

func TestAddItem_QuantityZero(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Zero Qty Product", "zero-qty")
	v := testDB.FixtureVariant(t, p.ID, "ZQ-001", 50)

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Add item with quantity 0 -- handler clamps to 1.
	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var item struct {
		Quantity int32 `json:"quantity"`
	}
	json.NewDecoder(rr.Body).Decode(&item)

	if item.Quantity != 1 {
		t.Errorf("quantity: got %d, want 1 (clamped from 0)", item.Quantity)
	}
}

// --------------------------------------------------------------------------
// AddItem -- negative quantity (gets clamped to 1 by handler)
// --------------------------------------------------------------------------

func TestAddItem_QuantityNegative(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Neg Qty Product", "neg-qty")
	v := testDB.FixtureVariant(t, p.ID, "NQ-001", 50)

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Add item with negative quantity -- handler clamps to 1.
	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   -5,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var item struct {
		Quantity int32 `json:"quantity"`
	}
	json.NewDecoder(rr.Body).Decode(&item)

	if item.Quantity != 1 {
		t.Errorf("quantity: got %d, want 1 (clamped from -5)", item.Quantity)
	}
}

// --------------------------------------------------------------------------
// AddItem -- nonexistent cart (valid UUID but no cart row) triggers FK error
// --------------------------------------------------------------------------

func TestAddItem_CartNotFound(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": uuid.New().String(),
		"quantity":   1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+uuid.New().String()+"/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// FK constraint violation means the service returns a generic error -> 500.
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- invalid cart ID (not a UUID)
// --------------------------------------------------------------------------

func TestUpdateItem_InvalidCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 3})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/not-a-uuid/items/"+uuid.New().String(),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- invalid item ID (not a UUID)
// --------------------------------------------------------------------------

func TestUpdateItem_InvalidItemID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 3})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/not-a-uuid", uuid.New().String()),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- invalid JSON body
// --------------------------------------------------------------------------

func TestUpdateItem_InvalidJSON(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", uuid.New().String(), uuid.New().String()),
		strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- item not found (valid UUIDs, no matching row)
// --------------------------------------------------------------------------

func TestUpdateItem_ItemNotFound(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 3})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", uuid.New().String(), uuid.New().String()),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- quantity zero triggers ErrInvalidQuantity from service
// --------------------------------------------------------------------------

func TestUpdateItem_QuantityZero(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "UpdateItem Zero Product", "ui-zero")
	v := testDB.FixtureVariant(t, p.ID, "UIZ-001", 50)

	// Create cart + add item.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	addBody, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   2,
	})
	addRR := httptest.NewRecorder()
	mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
		"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(addBody)))
	var addedItem struct {
		ID string `json:"id"`
	}
	json.NewDecoder(addRR.Body).Decode(&addedItem)

	// Update with quantity 0 -- should return 400.
	body, _ := json.Marshal(map[string]int{"quantity": 0})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", created.ID, addedItem.ID),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// UpdateItem -- negative quantity triggers ErrInvalidQuantity
// --------------------------------------------------------------------------

func TestUpdateItem_QuantityNegative(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "UpdateItem Neg Product", "ui-neg")
	v := testDB.FixtureVariant(t, p.ID, "UIN-001", 50)

	// Create cart + add item.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	addBody, _ := json.Marshal(map[string]interface{}{
		"variant_id": v.ID.String(),
		"quantity":   2,
	})
	addRR := httptest.NewRecorder()
	mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
		"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(addBody)))
	var addedItem struct {
		ID string `json:"id"`
	}
	json.NewDecoder(addRR.Body).Decode(&addedItem)

	// Update with negative quantity -- should return 400.
	body, _ := json.Marshal(map[string]int{"quantity": -3})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", created.ID, addedItem.ID),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// RemoveItem -- invalid cart ID (not a UUID)
// --------------------------------------------------------------------------

func TestRemoveItem_InvalidCartID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/cart/not-a-uuid/items/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// RemoveItem -- invalid item ID (not a UUID)
// --------------------------------------------------------------------------

func TestRemoveItem_InvalidItemID(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/cart/%s/items/not-a-uuid", uuid.New().String()), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --------------------------------------------------------------------------
// RemoveItem -- nonexistent item (valid UUID, no row) -- idempotent delete
// --------------------------------------------------------------------------

func TestRemoveItem_NonexistentItem(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	// Create a cart so the cart ID is valid.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Remove a nonexistent item -- should be 204 (idempotent delete).
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", created.ID, uuid.New().String()), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}
}

// --------------------------------------------------------------------------
// GetCart -- with updated nullable fields round-trip
// --------------------------------------------------------------------------

func TestGetCart_WithUpdatedFields(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Update with email and country.
	updateBody, _ := json.Marshal(map[string]string{
		"email":        "vat@example.com",
		"country_code": "FR",
	})
	updateRR := httptest.NewRecorder()
	mux.ServeHTTP(updateRR, httptest.NewRequest(http.MethodPatch,
		"/api/v1/cart/"+created.ID, bytes.NewReader(updateBody)))
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update: status %d, body: %s", updateRR.Code, updateRR.Body.String())
	}

	// Get cart and verify updated fields.
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil))

	if getRR.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", getRR.Code, http.StatusOK)
	}

	var resp struct {
		ID          string  `json:"id"`
		Email       *string `json:"email"`
		CountryCode *string `json:"country_code"`
		UpdatedAt   string  `json:"updated_at"`
	}
	json.NewDecoder(getRR.Body).Decode(&resp)

	if resp.Email == nil || *resp.Email != "vat@example.com" {
		t.Errorf("email: got %v, want %q", resp.Email, "vat@example.com")
	}
	if resp.CountryCode == nil || *resp.CountryCode != "FR" {
		t.Errorf("country_code: got %v, want %q", resp.CountryCode, "FR")
	}
	if resp.UpdatedAt == "" {
		t.Error("expected non-empty updated_at")
	}
}

// --------------------------------------------------------------------------
// AddItem -- duplicate variant (upsert behavior)
// --------------------------------------------------------------------------

func TestAddItem_DuplicateVariantUpsert(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	p := testDB.FixtureProduct(t, "Upsert Product", "upsert-product")
	v := testDB.FixtureVariant(t, p.ID, "UP-001", 100)

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	// Add the same variant twice.
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(map[string]interface{}{
			"variant_id": v.ID.String(),
			"quantity":   3,
		})
		addRR := httptest.NewRecorder()
		mux.ServeHTTP(addRR, httptest.NewRequest(http.MethodPost,
			"/api/v1/cart/"+created.ID+"/items", bytes.NewReader(body)))
		if addRR.Code != http.StatusCreated {
			t.Fatalf("add item attempt %d: status %d, body: %s", i+1, addRR.Code, addRR.Body.String())
		}
	}

	// Get cart -- upsert should have merged to 1 item with quantity 6.
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+created.ID, nil))

	var resp struct {
		Items []struct {
			Quantity int32 `json:"quantity"`
		} `json:"items"`
	}
	json.NewDecoder(getRR.Body).Decode(&resp)

	if len(resp.Items) != 1 {
		t.Errorf("items: got %d, want 1 (upsert should merge)", len(resp.Items))
	}
	if len(resp.Items) == 1 && resp.Items[0].Quantity != 6 {
		t.Errorf("quantity: got %d, want 6 (3+3 upsert)", resp.Items[0].Quantity)
	}
}

// --------------------------------------------------------------------------
// UpdateCart -- all four fields at once
// --------------------------------------------------------------------------

func TestUpdateCart_AllFields(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := cartMux()

	// Create cart.
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	body, _ := json.Marshal(map[string]string{
		"email":        "full@test.com",
		"country_code": "ES",
		"vat_number":   "ESB12345678",
		"coupon_code":  "SAVE10",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/"+created.ID, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		ID        string `json:"id"`
		UpdatedAt string `json:"updated_at"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.ID != created.ID {
		t.Errorf("id: got %q, want %q", resp.ID, created.ID)
	}
	if resp.UpdatedAt == "" {
		t.Error("expected non-empty updated_at")
	}
}

// --------------------------------------------------------------------------
// Error response body validation
// --------------------------------------------------------------------------

func TestGetCart_InvalidID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart/xyz", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid cart ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid cart ID")
	}
}

func TestGetCart_NotFound_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cart/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "cart not found" {
		t.Errorf("error: got %q, want %q", errResp.Error, "cart not found")
	}
}

func TestUpdateItem_ItemNotFound_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 3})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", uuid.New().String(), uuid.New().String()),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "cart item not found" {
		t.Errorf("error: got %q, want %q", errResp.Error, "cart item not found")
	}
}

func TestUpdateCart_InvalidID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]string{"email": "x@y.com"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/zzz", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid cart ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid cart ID")
	}
}

func TestAddItem_InvalidCartID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]interface{}{
		"variant_id": uuid.New().String(),
		"quantity":   1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/bad-id/items", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid cart ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid cart ID")
	}
}

func TestRemoveItem_InvalidCartID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/cart/bad/items/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid cart ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid cart ID")
	}
}

func TestRemoveItem_InvalidItemID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/cart/%s/items/bad", uuid.New().String()), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid item ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid item ID")
	}
}

func TestUpdateItem_InvalidCartID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 1})
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/cart/bad/items/"+uuid.New().String(),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid cart ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid cart ID")
	}
}

func TestUpdateItem_InvalidItemID_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	body, _ := json.Marshal(map[string]int{"quantity": 1})
	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/bad", uuid.New().String()),
		bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid item ID" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid item ID")
	}
}

func TestUpdateItem_InvalidJSON_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	req := httptest.NewRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/cart/%s/items/%s", uuid.New().String(), uuid.New().String()),
		strings.NewReader("{broken"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid request body" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid request body")
	}
}

func TestAddItem_InvalidJSON_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/"+created.ID+"/items",
		strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid request body" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid request body")
	}
}

func TestUpdateCart_InvalidJSON_ErrorBody(t *testing.T) {
	testDB.Truncate(t)
	mux := cartMux()

	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/api/v1/cart", nil))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(createRR.Body).Decode(&created)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/cart/"+created.ID,
		strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "invalid request body" {
		t.Errorf("error: got %q, want %q", errResp.Error, "invalid request body")
	}
}

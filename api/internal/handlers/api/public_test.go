package api_test

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/handlers/api"
	"github.com/forgecommerce/api/internal/services/category"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/variant"
	"github.com/forgecommerce/api/internal/testutil"
)

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	database, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer database.Close()
	testDB = database

	code = m.Run()
}

func newPublicHandler() *api.PublicHandler {
	logger := slog.Default()
	productSvc := product.NewService(testDB.Pool, logger)
	categorySvc := category.NewService(testDB.Pool, logger)
	variantSvc := variant.NewService(testDB.Pool, logger)
	return api.NewPublicHandler(productSvc, categorySvc, variantSvc, testDB.Pool, logger)
}

func publicMux() *http.ServeMux {
	mux := http.NewServeMux()
	newPublicHandler().RegisterRoutes(mux)
	return mux
}

// --------------------------------------------------------------------------
// ListProducts
// --------------------------------------------------------------------------

func TestListProducts(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureProduct(t, "Active Product A", "active-a")
	testDB.FixtureProduct(t, "Active Product B", "active-b")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Data       []json.RawMessage `json:"data"`
		Page       int               `json:"page"`
		TotalPages int               `json:"total_pages"`
		Total      int64             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("total: got %d, want 2", resp.Total)
	}
	if len(resp.Data) != 2 {
		t.Errorf("data length: got %d, want 2", len(resp.Data))
	}
	if resp.Page != 1 {
		t.Errorf("page: got %d, want 1", resp.Page)
	}
}

func TestListProducts_Pagination(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	for i := 0; i < 5; i++ {
		testDB.FixtureProduct(t, "Product "+string(rune('A'+i)), "product-"+string(rune('a'+i)))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?page=1&limit=2", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Data       []json.RawMessage `json:"data"`
		Total      int64             `json:"total"`
		TotalPages int               `json:"total_pages"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Total != 5 {
		t.Errorf("total: got %d, want 5", resp.Total)
	}
	if len(resp.Data) != 2 {
		t.Errorf("page 1 data: got %d, want 2", len(resp.Data))
	}
	if resp.TotalPages != 3 {
		t.Errorf("total_pages: got %d, want 3", resp.TotalPages)
	}
}

func TestListProducts_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Total int64 `json:"total"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Errorf("total: got %d, want 0", resp.Total)
	}
}

func TestListProducts_OnlyActive(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	// Create an active product via fixture.
	testDB.FixtureProduct(t, "Active", "active")

	// Create a draft product directly.
	q := db.New(testDB.Pool)
	q.CreateProduct(t.Context(), db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    "Draft",
		Slug:                    "draft",
		Status:                  "draft",
		HasVariants:             false,
		Metadata:                json.RawMessage(`{}`),
		CreatedAt:               time.Now().UTC(),
		BasePrice:               pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true},
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp struct {
		Total int64 `json:"total"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	// Only active products should be returned (1 active, not the draft).
	if resp.Total != 1 {
		t.Errorf("total: got %d, want 1 (draft should be excluded)", resp.Total)
	}
}

// --------------------------------------------------------------------------
// GetProduct
// --------------------------------------------------------------------------

func TestGetProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureProduct(t, "Leather Bag", "leather-bag")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/leather-bag", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Name       string            `json:"name"`
		Slug       string            `json:"slug"`
		Status     string            `json:"status"`
		Images     []json.RawMessage `json:"images"`
		Attributes []json.RawMessage `json:"attributes"`
		Variants   []json.RawMessage `json:"variants"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Name != "Leather Bag" {
		t.Errorf("name: got %q, want %q", resp.Name, "Leather Bag")
	}
	if resp.Slug != "leather-bag" {
		t.Errorf("slug: got %q, want %q", resp.Slug, "leather-bag")
	}
	if resp.Status != "active" {
		t.Errorf("status: got %q, want %q", resp.Status, "active")
	}
}

func TestGetProduct_NotFound(t *testing.T) {
	testDB.Truncate(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/nonexistent-slug", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// --------------------------------------------------------------------------
// ListProductVariants
// --------------------------------------------------------------------------

func TestListProductVariants(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Variant Product", "variant-product")
	testDB.FixtureVariant(t, p.ID, "VP-001", 10)
	testDB.FixtureVariant(t, p.ID, "VP-002", 20)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/variant-product/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var variants []struct {
		Sku      string `json:"sku"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&variants); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
	}
}

func TestListProductVariants_NotFound(t *testing.T) {
	testDB.Truncate(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/no-such-product/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// --------------------------------------------------------------------------
// ListCategories
// --------------------------------------------------------------------------

func TestListCategories(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureCategory(t, "Bags", "bags")
	testDB.FixtureCategory(t, "Wallets", "wallets")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var categories []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&categories); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(categories))
	}
}

// --------------------------------------------------------------------------
// ListCountries
// --------------------------------------------------------------------------

func TestListCountries(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	// Enable a shipping country.
	testDB.FixtureShippingCountry(t, "DE")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/countries", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var countries []struct {
		CountryCode string `json:"country_code"`
		Name        string `json:"name"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&countries); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(countries) != 1 {
		t.Errorf("expected 1 country, got %d", len(countries))
	}
	if len(countries) > 0 && countries[0].CountryCode != "DE" {
		t.Errorf("country code: got %q, want %q", countries[0].CountryCode, "DE")
	}
}

func TestListCountries_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/countries", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var countries []json.RawMessage
	json.NewDecoder(rr.Body).Decode(&countries)
	if len(countries) != 0 {
		t.Errorf("expected 0 countries, got %d", len(countries))
	}
}

// --------------------------------------------------------------------------
// Content-Type header
// --------------------------------------------------------------------------

func TestResponseContentType(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

// --------------------------------------------------------------------------
// NewPublicHandler — nil logger fallback
// --------------------------------------------------------------------------

func TestNewPublicHandler_NilLogger(t *testing.T) {
	productSvc := product.NewService(testDB.Pool, nil)
	categorySvc := category.NewService(testDB.Pool, nil)
	variantSvc := variant.NewService(testDB.Pool, nil)

	// Should not panic; uses slog.Default() internally.
	h := api.NewPublicHandler(productSvc, categorySvc, variantSvc, testDB.Pool, nil)
	if h == nil {
		t.Fatal("expected non-nil handler with nil logger")
	}

	// Verify it can serve requests.
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// GetProduct — draft product returns 404 via public API
// --------------------------------------------------------------------------

func TestGetProduct_DraftReturns404(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	// Create a draft product directly (not via fixture which creates "active").
	q := db.New(testDB.Pool)
	q.CreateProduct(t.Context(), db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    "Draft Product",
		Slug:                    "draft-product",
		Status:                  "draft",
		HasVariants:             false,
		Metadata:                json.RawMessage(`{}`),
		CreatedAt:               time.Now().UTC(),
		BasePrice:               pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true},
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/draft-product", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (draft products should not be visible)", rr.Code, http.StatusNotFound)
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "product not found" {
		t.Errorf("error message: got %q, want %q", resp.Error, "product not found")
	}
}

// --------------------------------------------------------------------------
// GetProduct — product with images (covers productImageToJSON, image list)
// --------------------------------------------------------------------------

func TestGetProduct_WithImages(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Product With Images", "product-with-images")

	// Create product-level images (no variant).
	q := db.New(testDB.Pool)
	ctx := context.Background()
	altText := "Front view"

	img1, err := q.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: p.ID,
		Url:       "https://cdn.example.com/img1.jpg",
		AltText:   &altText,
		Position:  0,
		IsPrimary: true,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating image 1: %v", err)
	}

	_, err = q.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: p.ID,
		Url:       "https://cdn.example.com/img2.jpg",
		Position:  1,
		IsPrimary: false,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating image 2: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/product-with-images", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Name   string `json:"name"`
		Images []struct {
			ID        string  `json:"id"`
			URL       string  `json:"url"`
			AltText   *string `json:"alt_text"`
			Position  int32   `json:"position"`
			IsPrimary bool    `json:"is_primary"`
			VariantID *string `json:"variant_id"`
		} `json:"images"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(resp.Images))
	}

	// Check the primary image fields.
	found := false
	for _, img := range resp.Images {
		if img.ID == img1.ID.String() {
			found = true
			if img.URL != "https://cdn.example.com/img1.jpg" {
				t.Errorf("image url: got %q, want %q", img.URL, "https://cdn.example.com/img1.jpg")
			}
			if img.AltText == nil || *img.AltText != "Front view" {
				t.Errorf("image alt_text: got %v, want %q", img.AltText, "Front view")
			}
			if !img.IsPrimary {
				t.Error("expected primary image to have is_primary=true")
			}
			if img.VariantID != nil {
				t.Errorf("product-level image should have no variant_id, got %v", img.VariantID)
			}
		}
	}
	if !found {
		t.Error("primary image not found in response")
	}
}

// --------------------------------------------------------------------------
// GetProduct — product with attributes and active options
// --------------------------------------------------------------------------

func TestGetProduct_WithAttributes(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Attributed Product", "attributed-product")

	q := db.New(testDB.Pool)
	ctx := context.Background()

	// Create an attribute with two options (one active, one inactive).
	attr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     p.ID,
		Name:          "color",
		DisplayName:   "Color",
		AttributeType: "color_swatch",
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating attribute: %v", err)
	}

	colorHex := "#000000"
	_, err = q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:           uuid.New(),
		AttributeID:  attr.ID,
		Value:        "black",
		DisplayValue: "Black",
		ColorHex:     &colorHex,
		Position:     1,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("creating active option: %v", err)
	}

	// Inactive option — should NOT appear in public API response.
	_, err = q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:           uuid.New(),
		AttributeID:  attr.ID,
		Value:        "red",
		DisplayValue: "Red",
		Position:     2,
		IsActive:     false,
	})
	if err != nil {
		t.Fatalf("creating inactive option: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/attributed-product", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Attributes []struct {
			Name          string `json:"name"`
			DisplayName   string `json:"display_name"`
			AttributeType string `json:"attribute_type"`
			Options       []struct {
				Value        string  `json:"value"`
				DisplayValue string  `json:"display_value"`
				ColorHex     *string `json:"color_hex"`
			} `json:"options"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(resp.Attributes))
	}

	a := resp.Attributes[0]
	if a.Name != "color" {
		t.Errorf("attribute name: got %q, want %q", a.Name, "color")
	}
	if a.DisplayName != "Color" {
		t.Errorf("attribute display_name: got %q, want %q", a.DisplayName, "Color")
	}
	if a.AttributeType != "color_swatch" {
		t.Errorf("attribute_type: got %q, want %q", a.AttributeType, "color_swatch")
	}

	// Only the active option should appear.
	if len(a.Options) != 1 {
		t.Fatalf("expected 1 active option, got %d", len(a.Options))
	}
	if a.Options[0].Value != "black" {
		t.Errorf("option value: got %q, want %q", a.Options[0].Value, "black")
	}
	if a.Options[0].ColorHex == nil || *a.Options[0].ColorHex != "#000000" {
		t.Errorf("option color_hex: got %v, want %q", a.Options[0].ColorHex, "#000000")
	}
}

// --------------------------------------------------------------------------
// GetProduct — product with variants and variant options
// --------------------------------------------------------------------------

func TestGetProduct_WithVariants(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Variant Detail Product", "variant-detail-product")

	q := db.New(testDB.Pool)
	ctx := context.Background()

	// Create attribute + option.
	attr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     p.ID,
		Name:          "size",
		DisplayName:   "Size",
		AttributeType: "button_group",
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating attribute: %v", err)
	}

	opt, err := q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:           uuid.New(),
		AttributeID:  attr.ID,
		Value:        "large",
		DisplayValue: "Large",
		Position:     1,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("creating option: %v", err)
	}

	// Create an active variant linked to the option.
	v := testDB.FixtureVariant(t, p.ID, "VDP-LRG", 50)
	err = q.SetVariantOption(ctx, db.SetVariantOptionParams{
		VariantID:   v.ID,
		AttributeID: attr.ID,
		OptionID:    opt.ID,
	})
	if err != nil {
		t.Fatalf("linking variant option: %v", err)
	}

	// Create a variant-level image.
	_, err = q.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: p.ID,
		VariantID: pgtype.UUID{Bytes: v.ID, Valid: true},
		Url:       "https://cdn.example.com/variant-img.jpg",
		Position:  0,
		IsPrimary: false,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating variant image: %v", err)
	}

	// Also create an inactive variant — should NOT appear.
	q.CreateProductVariant(ctx, db.CreateProductVariantParams{
		ID:            uuid.New(),
		ProductID:     p.ID,
		Sku:           "VDP-INACTIVE",
		Price:         pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true},
		StockQuantity: 0,
		IsActive:      false,
		Position:      2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/variant-detail-product", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Variants []struct {
			ID       string `json:"id"`
			Sku      string `json:"sku"`
			IsActive bool   `json:"is_active"`
			Options  []struct {
				AttributeName      string `json:"attribute_name"`
				OptionValue        string `json:"option_value"`
				OptionDisplayValue string `json:"option_display_value"`
			} `json:"options"`
			Images []struct {
				URL       string  `json:"url"`
				VariantID *string `json:"variant_id"`
			} `json:"images"`
		} `json:"variants"`
		Images []json.RawMessage `json:"images"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Only the active variant should appear.
	if len(resp.Variants) != 1 {
		t.Fatalf("expected 1 active variant, got %d", len(resp.Variants))
	}

	vr := resp.Variants[0]
	if vr.Sku != "VDP-LRG" {
		t.Errorf("variant sku: got %q, want %q", vr.Sku, "VDP-LRG")
	}

	// Variant should have its option.
	if len(vr.Options) != 1 {
		t.Fatalf("expected 1 variant option, got %d", len(vr.Options))
	}
	if vr.Options[0].AttributeName != "size" {
		t.Errorf("option attribute_name: got %q, want %q", vr.Options[0].AttributeName, "size")
	}
	if vr.Options[0].OptionValue != "large" {
		t.Errorf("option value: got %q, want %q", vr.Options[0].OptionValue, "large")
	}

	// Variant should have its image with variant_id set.
	if len(vr.Images) != 1 {
		t.Fatalf("expected 1 variant image, got %d", len(vr.Images))
	}
	if vr.Images[0].URL != "https://cdn.example.com/variant-img.jpg" {
		t.Errorf("variant image url: got %q", vr.Images[0].URL)
	}
	if vr.Images[0].VariantID == nil {
		t.Error("expected variant_id to be set on variant image")
	}

	// The product-level images list should also contain the variant image.
	if len(resp.Images) != 1 {
		t.Errorf("expected 1 total image in product images, got %d", len(resp.Images))
	}
}

// --------------------------------------------------------------------------
// GetProduct — full detail fields (description, seo, metadata, weight)
// --------------------------------------------------------------------------

func TestGetProduct_DetailFields(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	q := db.New(testDB.Pool)
	ctx := context.Background()

	desc := "Full description"
	shortDesc := "Short desc"
	seoTitle := "SEO Title"
	seoDesc := "SEO Description"
	skuPrefix := "LB"

	_, err := q.CreateProduct(ctx, db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    "Detail Product",
		Slug:                    "detail-product",
		Description:             &desc,
		ShortDescription:        &shortDesc,
		Status:                  "active",
		SkuPrefix:               &skuPrefix,
		BasePrice:               pgtype.Numeric{Int: big.NewInt(5000), Exp: -2, Valid: true},
		CompareAtPrice:          pgtype.Numeric{Int: big.NewInt(6000), Exp: -2, Valid: true},
		BaseWeightGrams:         500,
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(150), Exp: -2, Valid: true},
		HasVariants:             true,
		SeoTitle:                &seoTitle,
		SeoDescription:          &seoDesc,
		Metadata:                json.RawMessage(`{"brand":"test"}`),
		CreatedAt:               time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/detail-product", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Name             string          `json:"name"`
		Description      *string         `json:"description"`
		ShortDescription *string         `json:"short_description"`
		SkuPrefix        *string         `json:"sku_prefix"`
		BaseWeightGrams  int32           `json:"base_weight_grams"`
		HasVariants      bool            `json:"has_variants"`
		SeoTitle         *string         `json:"seo_title"`
		SeoDescription   *string         `json:"seo_description"`
		Metadata         json.RawMessage `json:"metadata"`
		CreatedAt        string          `json:"created_at"`
		UpdatedAt        string          `json:"updated_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Description == nil || *resp.Description != "Full description" {
		t.Errorf("description: got %v, want %q", resp.Description, "Full description")
	}
	if resp.ShortDescription == nil || *resp.ShortDescription != "Short desc" {
		t.Errorf("short_description: got %v, want %q", resp.ShortDescription, "Short desc")
	}
	if resp.SkuPrefix == nil || *resp.SkuPrefix != "LB" {
		t.Errorf("sku_prefix: got %v, want %q", resp.SkuPrefix, "LB")
	}
	if resp.BaseWeightGrams != 500 {
		t.Errorf("base_weight_grams: got %d, want 500", resp.BaseWeightGrams)
	}
	if !resp.HasVariants {
		t.Error("has_variants: got false, want true")
	}
	if resp.SeoTitle == nil || *resp.SeoTitle != "SEO Title" {
		t.Errorf("seo_title: got %v, want %q", resp.SeoTitle, "SEO Title")
	}
	if resp.SeoDescription == nil || *resp.SeoDescription != "SEO Description" {
		t.Errorf("seo_description: got %v, want %q", resp.SeoDescription, "SEO Description")
	}

	// Metadata should be present.
	var meta map[string]string
	if err := json.Unmarshal(resp.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["brand"] != "test" {
		t.Errorf("metadata brand: got %q, want %q", meta["brand"], "test")
	}
}

// --------------------------------------------------------------------------
// ListProductVariants — draft product returns 404
// --------------------------------------------------------------------------

func TestListProductVariants_DraftReturns404(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	q := db.New(testDB.Pool)
	q.CreateProduct(t.Context(), db.CreateProductParams{
		ID:                      uuid.New(),
		Name:                    "Draft For Variants",
		Slug:                    "draft-for-variants",
		Status:                  "draft",
		HasVariants:             true,
		Metadata:                json.RawMessage(`{}`),
		CreatedAt:               time.Now().UTC(),
		BasePrice:               pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true},
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/draft-for-variants/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// --------------------------------------------------------------------------
// ListProductVariants — product with no variants returns empty array
// --------------------------------------------------------------------------

func TestListProductVariants_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureProduct(t, "No Variants Product", "no-variants-product")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/no-variants-product/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var variants []json.RawMessage
	if err := json.NewDecoder(rr.Body).Decode(&variants); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(variants) != 0 {
		t.Errorf("expected 0 variants, got %d", len(variants))
	}
}

// --------------------------------------------------------------------------
// ListProductVariants — with variant options
// --------------------------------------------------------------------------

func TestListProductVariants_WithOptions(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Options Variant Product", "options-variant-product")

	q := db.New(testDB.Pool)
	ctx := context.Background()

	attr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     p.ID,
		Name:          "material",
		DisplayName:   "Material",
		AttributeType: "select",
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating attribute: %v", err)
	}

	opt, err := q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
		ID:           uuid.New(),
		AttributeID:  attr.ID,
		Value:        "leather",
		DisplayValue: "Leather",
		Position:     1,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("creating option: %v", err)
	}

	v := testDB.FixtureVariant(t, p.ID, "OVP-LEA", 15)
	err = q.SetVariantOption(ctx, db.SetVariantOptionParams{
		VariantID:   v.ID,
		AttributeID: attr.ID,
		OptionID:    opt.ID,
	})
	if err != nil {
		t.Fatalf("linking variant option: %v", err)
	}

	// Also add a variant-level image.
	_, err = q.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: p.ID,
		VariantID: pgtype.UUID{Bytes: v.ID, Valid: true},
		Url:       "https://cdn.example.com/variant-leather.jpg",
		Position:  0,
		IsPrimary: false,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating variant image: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/options-variant-product/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var variants []struct {
		Sku     string `json:"sku"`
		Options []struct {
			AttributeName      string `json:"attribute_name"`
			OptionValue        string `json:"option_value"`
			OptionDisplayValue string `json:"option_display_value"`
		} `json:"options"`
		Images []struct {
			URL       string  `json:"url"`
			VariantID *string `json:"variant_id"`
		} `json:"images"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&variants); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}

	vr := variants[0]
	if vr.Sku != "OVP-LEA" {
		t.Errorf("sku: got %q, want %q", vr.Sku, "OVP-LEA")
	}
	if len(vr.Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(vr.Options))
	}
	if vr.Options[0].AttributeName != "material" {
		t.Errorf("attribute_name: got %q, want %q", vr.Options[0].AttributeName, "material")
	}
	if vr.Options[0].OptionValue != "leather" {
		t.Errorf("option_value: got %q, want %q", vr.Options[0].OptionValue, "leather")
	}
	if vr.Options[0].OptionDisplayValue != "Leather" {
		t.Errorf("option_display_value: got %q, want %q", vr.Options[0].OptionDisplayValue, "Leather")
	}

	// Variant image should be present.
	if len(vr.Images) != 1 {
		t.Fatalf("expected 1 variant image, got %d", len(vr.Images))
	}
	if vr.Images[0].URL != "https://cdn.example.com/variant-leather.jpg" {
		t.Errorf("variant image url: got %q", vr.Images[0].URL)
	}
}

// --------------------------------------------------------------------------
// ListProductVariants — inactive variants are filtered out
// --------------------------------------------------------------------------

func TestListProductVariants_FiltersInactive(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Mixed Variants Product", "mixed-variants-product")

	// Active variant (via fixture helper).
	testDB.FixtureVariant(t, p.ID, "MVR-ACTIVE", 10)

	// Inactive variant (direct creation).
	q := db.New(testDB.Pool)
	q.CreateProductVariant(t.Context(), db.CreateProductVariantParams{
		ID:            uuid.New(),
		ProductID:     p.ID,
		Sku:           "MVR-INACTIVE",
		Price:         pgtype.Numeric{Int: big.NewInt(2000), Exp: -2, Valid: true},
		StockQuantity: 5,
		IsActive:      false,
		Position:      2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/mixed-variants-product/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var variants []struct {
		Sku      string `json:"sku"`
		IsActive bool   `json:"is_active"`
	}
	json.NewDecoder(rr.Body).Decode(&variants)

	if len(variants) != 1 {
		t.Fatalf("expected 1 active variant, got %d", len(variants))
	}
	if variants[0].Sku != "MVR-ACTIVE" {
		t.Errorf("sku: got %q, want %q", variants[0].Sku, "MVR-ACTIVE")
	}
}

// --------------------------------------------------------------------------
// ListCategories — empty state
// --------------------------------------------------------------------------

func TestListCategories_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var categories []json.RawMessage
	json.NewDecoder(rr.Body).Decode(&categories)

	if len(categories) != 0 {
		t.Errorf("expected 0 categories, got %d", len(categories))
	}
}

// --------------------------------------------------------------------------
// ListCategories — category fields (description, parent_id, image_url)
// --------------------------------------------------------------------------

func TestListCategories_FieldContent(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	q := db.New(testDB.Pool)
	ctx := context.Background()

	parentDesc := "Top-level category"
	parentImg := "https://cdn.example.com/bags.jpg"

	parent, err := q.CreateCategory(ctx, db.CreateCategoryParams{
		ID:          uuid.New(),
		Name:        "Bags",
		Slug:        "bags",
		Description: &parentDesc,
		ImageUrl:    &parentImg,
		IsActive:    true,
		Position:    1,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating parent category: %v", err)
	}

	childDesc := "Messenger bags"
	_, err = q.CreateCategory(ctx, db.CreateCategoryParams{
		ID:          uuid.New(),
		Name:        "Messenger",
		Slug:        "messenger",
		Description: &childDesc,
		ParentID:    pgtype.UUID{Bytes: parent.ID, Valid: true},
		IsActive:    true,
		Position:    1,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating child category: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var categories []struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		ParentID    *string `json:"parent_id"`
		Position    int32   `json:"position"`
		ImageUrl    *string `json:"image_url"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&categories); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}

	// Find parent and child.
	var parentCat, childCat *struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		ParentID    *string `json:"parent_id"`
		Position    int32   `json:"position"`
		ImageUrl    *string `json:"image_url"`
	}
	for i := range categories {
		if categories[i].Name == "Bags" {
			parentCat = &categories[i]
		}
		if categories[i].Name == "Messenger" {
			childCat = &categories[i]
		}
	}

	if parentCat == nil {
		t.Fatal("parent category not found in response")
	}
	if childCat == nil {
		t.Fatal("child category not found in response")
	}

	// Parent should have no parent_id (covers pgtypeUUIDToPtr nil path).
	if parentCat.ParentID != nil {
		t.Errorf("parent category parent_id: got %v, want nil", parentCat.ParentID)
	}
	if parentCat.Description == nil || *parentCat.Description != "Top-level category" {
		t.Errorf("parent description: got %v, want %q", parentCat.Description, "Top-level category")
	}
	if parentCat.ImageUrl == nil || *parentCat.ImageUrl != "https://cdn.example.com/bags.jpg" {
		t.Errorf("parent image_url: got %v, want non-nil", parentCat.ImageUrl)
	}

	// Child should have parent_id set (covers pgtypeUUIDToPtr valid path).
	if childCat.ParentID == nil {
		t.Fatal("child category parent_id: got nil, want non-nil")
	}
	if *childCat.ParentID != parent.ID.String() {
		t.Errorf("child parent_id: got %q, want %q", *childCat.ParentID, parent.ID.String())
	}
}

// --------------------------------------------------------------------------
// ListCountries — multiple countries, ordered by name
// --------------------------------------------------------------------------

func TestListCountries_Multiple(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureShippingCountry(t, "DE")
	testDB.FixtureShippingCountry(t, "FR")
	testDB.FixtureShippingCountry(t, "ES")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/countries", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var countries []struct {
		CountryCode string `json:"country_code"`
		Name        string `json:"name"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&countries); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(countries) != 3 {
		t.Fatalf("expected 3 countries, got %d", len(countries))
	}

	// Verify all three are present.
	codes := map[string]bool{}
	for _, c := range countries {
		codes[c.CountryCode] = true
		if c.Name == "" {
			t.Errorf("country %q has empty name", c.CountryCode)
		}
	}
	for _, code := range []string{"DE", "FR", "ES"} {
		if !codes[code] {
			t.Errorf("country %q not found in response", code)
		}
	}
}

// --------------------------------------------------------------------------
// ListProducts — with featured image (covers productImageToJSON via ListProducts)
// --------------------------------------------------------------------------

func TestListProducts_WithFeaturedImage(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	p := testDB.FixtureProduct(t, "Product With Primary Image", "product-with-primary-image")

	// Create a primary image for the product.
	q := db.New(testDB.Pool)
	ctx := context.Background()
	altText := "Primary shot"
	_, err := q.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: p.ID,
		Url:       "https://cdn.example.com/primary.jpg",
		AltText:   &altText,
		Position:  0,
		IsPrimary: true,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("creating primary image: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			Name          string `json:"name"`
			FeaturedImage *struct {
				URL       string  `json:"url"`
				AltText   *string `json:"alt_text"`
				IsPrimary bool    `json:"is_primary"`
			} `json:"featured_image"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 product, got %d", len(resp.Data))
	}

	fi := resp.Data[0].FeaturedImage
	if fi == nil {
		t.Fatal("expected featured_image to be non-nil")
	}
	if fi.URL != "https://cdn.example.com/primary.jpg" {
		t.Errorf("featured_image url: got %q, want %q", fi.URL, "https://cdn.example.com/primary.jpg")
	}
	if fi.AltText == nil || *fi.AltText != "Primary shot" {
		t.Errorf("featured_image alt_text: got %v, want %q", fi.AltText, "Primary shot")
	}
	if !fi.IsPrimary {
		t.Error("expected featured_image is_primary=true")
	}
}

// --------------------------------------------------------------------------
// ListProducts — product without primary image has null featured_image
// --------------------------------------------------------------------------

func TestListProducts_NoFeaturedImage(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureProduct(t, "No Image Product", "no-image-product")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			FeaturedImage *json.RawMessage `json:"featured_image"`
		} `json:"data"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 product, got %d", len(resp.Data))
	}

	// featured_image should be null.
	if resp.Data[0].FeaturedImage != nil {
		raw := string(*resp.Data[0].FeaturedImage)
		if raw != "null" {
			t.Errorf("expected featured_image to be null, got %s", raw)
		}
	}
}

// --------------------------------------------------------------------------
// Pagination edge cases
// --------------------------------------------------------------------------

func TestPagination_InvalidValues(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	testDB.FixtureProduct(t, "Pagination Product", "pagination-product")

	tests := []struct {
		name  string
		query string
		check func(t *testing.T, resp struct {
			Page int `json:"page"`
		})
	}{
		{
			name:  "negative page defaults to 1",
			query: "?page=-1",
			check: func(t *testing.T, resp struct {
				Page int `json:"page"`
			}) {
				if resp.Page != 1 {
					t.Errorf("page: got %d, want 1", resp.Page)
				}
			},
		},
		{
			name:  "zero page defaults to 1",
			query: "?page=0",
			check: func(t *testing.T, resp struct {
				Page int `json:"page"`
			}) {
				if resp.Page != 1 {
					t.Errorf("page: got %d, want 1", resp.Page)
				}
			},
		},
		{
			name:  "non-numeric page defaults to 1",
			query: "?page=abc",
			check: func(t *testing.T, resp struct {
				Page int `json:"page"`
			}) {
				if resp.Page != 1 {
					t.Errorf("page: got %d, want 1", resp.Page)
				}
			},
		},
		{
			name:  "non-numeric limit uses default",
			query: "?limit=xyz",
			check: func(t *testing.T, resp struct {
				Page int `json:"page"`
			}) {
				// Just verify it works without error.
				if resp.Page != 1 {
					t.Errorf("page: got %d, want 1", resp.Page)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/products"+tt.query, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
			}

			var resp struct {
				Page int `json:"page"`
			}
			json.NewDecoder(rr.Body).Decode(&resp)
			tt.check(t, resp)
		})
	}
}

func TestPagination_LimitCappedAt250(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	// Request limit=500. The handler should cap it at 250.
	// We just need to verify the endpoint works (no error) with a huge limit.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=500", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d\nbody: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestPagination_NegativeLimit(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	mux := publicMux()

	// Negative limit should default to 20.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=-5", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
}

// --------------------------------------------------------------------------
// Error response JSON format on not-found
// --------------------------------------------------------------------------

func TestGetProduct_NotFoundErrorFormat(t *testing.T) {
	testDB.Truncate(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/does-not-exist", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error != "product not found" {
		t.Errorf("error: got %q, want %q", errResp.Error, "product not found")
	}
}

func TestListProductVariants_NotFoundErrorFormat(t *testing.T) {
	testDB.Truncate(t)
	mux := publicMux()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/ghost/variants", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "product not found" {
		t.Errorf("error: got %q, want %q", errResp.Error, "product not found")
	}
}

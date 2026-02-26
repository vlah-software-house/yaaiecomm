package variant_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"

	db "github.com/forgecommerce/api/internal/database/gen"
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

func newService() *variant.Service {
	return variant.NewService(testDB.Pool, nil)
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")

	v, err := svc.Create(ctx, variant.CreateVariantParams{
		ProductID:     product.ID,
		Sku:           "TST-BLK-LG",
		StockQuantity: 10,
		IsActive:      true,
		Position:      1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if v.ID == uuid.Nil {
		t.Error("expected non-nil variant ID")
	}
	if v.Sku != "TST-BLK-LG" {
		t.Errorf("SKU: got %q, want %q", v.Sku, "TST-BLK-LG")
	}
	if v.StockQuantity != 10 {
		t.Errorf("stock: got %d, want 10", v.StockQuantity)
	}
}

func TestCreate_EmptySKU(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.Create(ctx, variant.CreateVariantParams{
		ProductID: uuid.New(),
		Sku:       "",
	})
	if err != variant.ErrSKURequired {
		t.Errorf("expected ErrSKURequired, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Get
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	created := testDB.FixtureVariant(t, product.ID, "TST-001", 5)

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Error("Get: ID mismatch")
	}
	if got.Sku != "TST-001" {
		t.Errorf("SKU: got %q, want %q", got.Sku, "TST-001")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// GetBySKU
// --------------------------------------------------------------------------

func TestGetBySKU(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	testDB.FixtureVariant(t, product.ID, "UNIQUE-SKU-42", 3)

	got, err := svc.GetBySKU(ctx, "UNIQUE-SKU-42")
	if err != nil {
		t.Fatalf("GetBySKU: %v", err)
	}
	if got.Sku != "UNIQUE-SKU-42" {
		t.Errorf("SKU: got %q, want %q", got.Sku, "UNIQUE-SKU-42")
	}
}

func TestGetBySKU_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetBySKU(ctx, "NONEXISTENT")
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// List
// --------------------------------------------------------------------------

func TestList(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	testDB.FixtureVariant(t, product.ID, "V-001", 10)
	testDB.FixtureVariant(t, product.ID, "V-002", 20)

	variants, err := svc.List(ctx, product.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
	}
}

func TestList_EmptyProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Empty Product", "empty-product")

	variants, err := svc.List(ctx, product.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(variants) != 0 {
		t.Errorf("expected 0 variants, got %d", len(variants))
	}
}

// --------------------------------------------------------------------------
// Update
// --------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	created := testDB.FixtureVariant(t, product.ID, "TST-001", 5)

	updated, err := svc.Update(ctx, created.ID, variant.UpdateVariantParams{
		Sku:           "TST-001-UPDATED",
		StockQuantity: 42,
		IsActive:      true,
		Position:      2,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Sku != "TST-001-UPDATED" {
		t.Errorf("SKU: got %q, want %q", updated.Sku, "TST-001-UPDATED")
	}
	if updated.StockQuantity != 42 {
		t.Errorf("stock: got %d, want 42", updated.StockQuantity)
	}
}

func TestUpdate_EmptySKU(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), variant.UpdateVariantParams{Sku: ""})
	if err != variant.ErrSKURequired {
		t.Errorf("expected ErrSKURequired, got %v", err)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), variant.UpdateVariantParams{
		Sku:      "ANY",
		IsActive: true,
	})
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Delete
// --------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	created := testDB.FixtureVariant(t, product.ID, "TST-DEL", 5)

	err := svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone.
	_, err = svc.Get(ctx, created.ID)
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.Delete(ctx, uuid.New())
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// UpdateStock
// --------------------------------------------------------------------------

func TestUpdateStock(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	created := testDB.FixtureVariant(t, product.ID, "TST-STK", 5)

	err := svc.UpdateStock(ctx, created.ID, 100)
	if err != nil {
		t.Fatalf("UpdateStock: %v", err)
	}

	got, _ := svc.Get(ctx, created.ID)
	if got.StockQuantity != 100 {
		t.Errorf("stock: got %d, want 100", got.StockQuantity)
	}
}

func TestUpdateStock_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.UpdateStock(ctx, uuid.New(), 10)
	if err != variant.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// GenerateVariants
// --------------------------------------------------------------------------

func TestGenerateVariants(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()
	q := db.New(testDB.Pool)

	product := testDB.FixtureProduct(t, "Variant Product", "variant-product")

	// Create 2 attributes: Color (2 options) and Size (3 options).
	colorAttr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     product.ID,
		Name:          "color",
		DisplayName:   "Color",
		AttributeType: "select",
		Position:      1,
	})
	if err != nil {
		t.Fatalf("creating color attribute: %v", err)
	}

	sizeAttr, err := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     product.ID,
		Name:          "size",
		DisplayName:   "Size",
		AttributeType: "select",
		Position:      2,
	})
	if err != nil {
		t.Fatalf("creating size attribute: %v", err)
	}

	// Create color options.
	for i, val := range []string{"black", "white"} {
		_, err := q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
			ID:           uuid.New(),
			AttributeID:  colorAttr.ID,
			Value:        val,
			DisplayValue: val,
			Position:     int32(i + 1),
			IsActive:     true,
		})
		if err != nil {
			t.Fatalf("creating color option %q: %v", val, err)
		}
	}

	// Create size options.
	for i, val := range []string{"small", "medium", "large"} {
		_, err := q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
			ID:           uuid.New(),
			AttributeID:  sizeAttr.ID,
			Value:        val,
			DisplayValue: val,
			Position:     int32(i + 1),
			IsActive:     true,
		})
		if err != nil {
			t.Fatalf("creating size option %q: %v", val, err)
		}
	}

	// Generate variants: should create 2 × 3 = 6 variants.
	variants, err := svc.GenerateVariants(ctx, product.ID, "VP")
	if err != nil {
		t.Fatalf("GenerateVariants: %v", err)
	}
	if len(variants) != 6 {
		t.Fatalf("expected 6 variants (2×3), got %d", len(variants))
	}

	// Verify all SKUs are unique.
	skus := map[string]bool{}
	for _, v := range variants {
		if skus[v.Sku] {
			t.Errorf("duplicate SKU: %q", v.Sku)
		}
		skus[v.Sku] = true
	}
}

func TestGenerateVariants_NoAttributes(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "No Attrs Product", "no-attrs")

	_, err := svc.GenerateVariants(ctx, product.ID, "NA")
	if err != variant.ErrNoAttributes {
		t.Errorf("expected ErrNoAttributes, got %v", err)
	}
}

func TestGenerateVariants_Idempotent(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()
	q := db.New(testDB.Pool)

	product := testDB.FixtureProduct(t, "Idempotent Product", "idempotent-product")

	// Create 1 attribute with 2 options.
	attr, _ := q.CreateProductAttribute(ctx, db.CreateProductAttributeParams{
		ID:            uuid.New(),
		ProductID:     product.ID,
		Name:          "color",
		DisplayName:   "Color",
		AttributeType: "select",
		Position:      1,
	})
	for i, val := range []string{"red", "blue"} {
		q.CreateAttributeOption(ctx, db.CreateAttributeOptionParams{
			ID:           uuid.New(),
			AttributeID:  attr.ID,
			Value:        val,
			DisplayValue: val,
			Position:     int32(i + 1),
			IsActive:     true,
		})
	}

	// First generation: 2 variants.
	v1, _ := svc.GenerateVariants(ctx, product.ID, "IDM")
	if len(v1) != 2 {
		t.Fatalf("first gen: expected 2, got %d", len(v1))
	}

	// Second generation: should return 0 new variants (existing preserved).
	v2, err := svc.GenerateVariants(ctx, product.ID, "IDM")
	if err != nil {
		t.Fatalf("second gen: %v", err)
	}
	if len(v2) != 0 {
		t.Errorf("second gen: expected 0 new variants (all existed), got %d", len(v2))
	}

	// Verify the original 2 variants still exist.
	all, _ := svc.List(ctx, product.ID)
	if len(all) != 2 {
		t.Errorf("expected 2 total variants after re-gen, got %d", len(all))
	}
}

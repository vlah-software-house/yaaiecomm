package rawmaterial_test

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/rawmaterial"
	"github.com/forgecommerce/api/internal/testutil"
)

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	db, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer db.Close()
	testDB = db

	code = m.Run()
}

func newService() *rawmaterial.Service {
	return rawmaterial.NewService(testDB.Pool, slog.Default())
}

func numericFromInt(n int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(n), Exp: 0, Valid: true}
}

func numericFromCents(cents int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(cents), Exp: -2, Valid: true}
}

func minimalMaterialParams(name, sku string) rawmaterial.CreateRawMaterialParams {
	return rawmaterial.CreateRawMaterialParams{
		Name:              name,
		Sku:               sku,
		UnitOfMeasure:     "unit",
		CostPerUnit:       numericFromCents(500),
		StockQuantity:     numericFromInt(100),
		LowStockThreshold: numericFromInt(10),
		Metadata:          json.RawMessage(`{}`),
		IsActive:          true,
	}
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	m, err := svc.Create(ctx, minimalMaterialParams("Black Leather", "BLK-LTH-001"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if m.Name != "Black Leather" {
		t.Errorf("name: got %q, want %q", m.Name, "Black Leather")
	}
	if m.Sku != "BLK-LTH-001" {
		t.Errorf("sku: got %q, want %q", m.Sku, "BLK-LTH-001")
	}
	if !m.IsActive {
		t.Error("expected is_active=true")
	}
}

func TestCreate_WithCategory(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, err := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
		Name:     "Fabrics",
		Position: 1,
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	params := minimalMaterialParams("Silk Thread", "SLK-001")
	params.CategoryID = &cat.ID

	m, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !m.CategoryID.Valid || m.CategoryID.Bytes != cat.ID {
		t.Errorf("category ID: got %v, want %s", m.CategoryID, cat.ID)
	}
}

func TestCreate_WithOptionalFields(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	supplier := "LeatherCo"
	supplierSku := "LC-BLK-001"
	desc := "Premium full-grain leather"
	leadTime := int32(14)

	params := minimalMaterialParams("Full-Grain Leather", "FGL-001")
	params.SupplierName = &supplier
	params.SupplierSku = &supplierSku
	params.Description = &desc
	params.LeadTimeDays = &leadTime

	m, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.SupplierName == nil || *m.SupplierName != supplier {
		t.Errorf("supplier name: got %v, want %q", m.SupplierName, supplier)
	}
	if m.SupplierSku == nil || *m.SupplierSku != supplierSku {
		t.Errorf("supplier sku: got %v, want %q", m.SupplierSku, supplierSku)
	}
	if m.Description == nil || *m.Description != desc {
		t.Errorf("description: got %v, want %q", m.Description, desc)
	}
	if m.LeadTimeDays == nil || *m.LeadTimeDays != leadTime {
		t.Errorf("lead time: got %v, want %d", m.LeadTimeDays, leadTime)
	}
}

// --------------------------------------------------------------------------
// Get
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.Create(ctx, minimalMaterialParams("Thread", "THR-001"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Error("Get: ID mismatch")
	}
	if got.Sku != "THR-001" {
		t.Errorf("SKU: got %q, want %q", got.Sku, "THR-001")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

// --------------------------------------------------------------------------
// Update
// --------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.Create(ctx, minimalMaterialParams("Old Name", "OLD-001"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.ID, rawmaterial.UpdateRawMaterialParams{
		Name:              "New Name",
		Sku:               "NEW-001",
		UnitOfMeasure:     "kg",
		CostPerUnit:       numericFromCents(750),
		StockQuantity:     numericFromInt(200),
		LowStockThreshold: numericFromInt(20),
		Metadata:          json.RawMessage(`{}`),
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("name: got %q, want %q", updated.Name, "New Name")
	}
	if updated.Sku != "NEW-001" {
		t.Errorf("sku: got %q, want %q", updated.Sku, "NEW-001")
	}
	if updated.UnitOfMeasure != "kg" {
		t.Errorf("uom: got %q, want %q", updated.UnitOfMeasure, "kg")
	}
}

// --------------------------------------------------------------------------
// Delete
// --------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.Create(ctx, minimalMaterialParams("To Delete", "DEL-001"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone.
	_, err = svc.Get(ctx, created.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --------------------------------------------------------------------------
// List
// --------------------------------------------------------------------------

func TestList(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalMaterialParams("Material A", "MAT-A"))
	svc.Create(ctx, minimalMaterialParams("Material B", "MAT-B"))
	svc.Create(ctx, minimalMaterialParams("Material C", "MAT-C"))

	materials, total, err := svc.List(ctx, nil, nil, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total: got %d, want 3", total)
	}
	if len(materials) != 3 {
		t.Errorf("materials: got %d, want 3", len(materials))
	}
}

func TestList_CategoryFilter(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, _ := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
		Name:     "Metals",
		Position: 1,
	})

	p1 := minimalMaterialParams("Brass Buckle", "BRS-001")
	p1.CategoryID = &cat.ID
	svc.Create(ctx, p1)

	p2 := minimalMaterialParams("Steel Ring", "STL-001")
	p2.CategoryID = &cat.ID
	svc.Create(ctx, p2)

	// Uncategorized material.
	svc.Create(ctx, minimalMaterialParams("Loose Thread", "THR-001"))

	materials, total, err := svc.List(ctx, &cat.ID, nil, 1, 10)
	if err != nil {
		t.Fatalf("List with category: %v", err)
	}
	if total != 2 {
		t.Errorf("total: got %d, want 2", total)
	}
	if len(materials) != 2 {
		t.Errorf("materials: got %d, want 2", len(materials))
	}
}

func TestList_ActiveFilter(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalMaterialParams("Active Material", "ACT-001"))

	inactive := minimalMaterialParams("Inactive Material", "INA-001")
	inactive.IsActive = false
	svc.Create(ctx, inactive)

	activeOnly := true
	materials, total, err := svc.List(ctx, nil, &activeOnly, 1, 10)
	if err != nil {
		t.Fatalf("List active only: %v", err)
	}
	if total != 1 {
		t.Errorf("total: got %d, want 1", total)
	}
	if len(materials) != 1 {
		t.Errorf("materials: got %d, want 1", len(materials))
	}
}

func TestList_Pagination(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Create(ctx, minimalMaterialParams(
			"Material "+string(rune('A'+i)),
			"MAT-"+string(rune('A'+i)),
		))
	}

	page1, total, _ := svc.List(ctx, nil, nil, 1, 2)
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(page1) != 2 {
		t.Errorf("page 1: got %d, want 2", len(page1))
	}

	page3, _, _ := svc.List(ctx, nil, nil, 3, 2)
	if len(page3) != 1 {
		t.Errorf("page 3: got %d, want 1", len(page3))
	}
}

func TestList_BoundsProtection(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// page < 1 and pageSize < 1 should default safely.
	_, _, err := svc.List(ctx, nil, nil, 0, 0)
	if err != nil {
		t.Errorf("List with zero bounds should not error: %v", err)
	}
}

// --------------------------------------------------------------------------
// Categories
// --------------------------------------------------------------------------

func TestCreateCategory(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, err := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
		Name:     "Leather & Supplies",
		Position: 1,
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	if cat.ID == uuid.Nil {
		t.Error("expected non-nil category ID")
	}
	if cat.Name != "Leather & Supplies" {
		t.Errorf("name: got %q, want %q", cat.Name, "Leather & Supplies")
	}
	if cat.Slug != "leather-supplies" {
		t.Errorf("slug: got %q, want %q", cat.Slug, "leather-supplies")
	}
}

func TestCreateCategory_WithParent(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	parent, _ := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
		Name:     "Hardware",
		Position: 1,
	})

	child, err := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
		Name:     "Buckles",
		ParentID: &parent.ID,
		Position: 1,
	})
	if err != nil {
		t.Fatalf("CreateCategory with parent: %v", err)
	}

	if !child.ParentID.Valid || child.ParentID.Bytes != parent.ID {
		t.Errorf("parent ID: got %v, want %s", child.ParentID, parent.ID)
	}
}

func TestListCategories(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{Name: "Cat A", Position: 1})
	svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{Name: "Cat B", Position: 2})

	cats, err := svc.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

// --------------------------------------------------------------------------
// LowStock
// --------------------------------------------------------------------------

func TestListLowStock(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Material at threshold (stock == threshold).
	atThreshold := minimalMaterialParams("At Threshold", "AT-001")
	atThreshold.StockQuantity = numericFromInt(10)
	atThreshold.LowStockThreshold = numericFromInt(10)
	svc.Create(ctx, atThreshold)

	// Material below threshold.
	belowThreshold := minimalMaterialParams("Below Threshold", "BT-001")
	belowThreshold.StockQuantity = numericFromInt(3)
	belowThreshold.LowStockThreshold = numericFromInt(10)
	svc.Create(ctx, belowThreshold)

	// Material above threshold — should NOT appear.
	aboveThreshold := minimalMaterialParams("Above Threshold", "ABV-001")
	aboveThreshold.StockQuantity = numericFromInt(100)
	aboveThreshold.LowStockThreshold = numericFromInt(10)
	svc.Create(ctx, aboveThreshold)

	materials, err := svc.ListLowStock(ctx, 50)
	if err != nil {
		t.Fatalf("ListLowStock: %v", err)
	}

	if len(materials) != 2 {
		t.Errorf("expected 2 low-stock materials, got %d", len(materials))
	}
}

func TestListLowStock_DefaultLimit(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// limit < 1 should default to 50, not error.
	_, err := svc.ListLowStock(ctx, 0)
	if err != nil {
		t.Errorf("ListLowStock with limit=0 should not error: %v", err)
	}
}

// --------------------------------------------------------------------------
// Search
// --------------------------------------------------------------------------

func TestSearch(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalMaterialParams("Black Leather", "BLK-LTH"))
	svc.Create(ctx, minimalMaterialParams("Tan Leather", "TAN-LTH"))
	svc.Create(ctx, minimalMaterialParams("Brass Buckle", "BRS-BCK"))

	// Search by name.
	results, err := svc.Search(ctx, "leather", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 leather results, got %d", len(results))
	}

	// Search by SKU.
	results, err = svc.Search(ctx, "BRS", 1, 10)
	if err != nil {
		t.Fatalf("Search by SKU: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 brass result, got %d", len(results))
	}
}

func TestSearch_NoResults(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	results, err := svc.Search(ctx, "nonexistent", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --------------------------------------------------------------------------
// slugify (pure logic, tested via CreateCategory)
// --------------------------------------------------------------------------

func TestSlugify_ViaCategory(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	tests := []struct {
		name     string
		wantSlug string
	}{
		{"Simple Name", "simple-name"},
		{"Leather & Supplies", "leather-supplies"},
		{"  Extra   Spaces  ", "extra-spaces"},
		{"UPPERCASE", "uppercase"},
		{"special@chars!here", "specialcharshere"},
	}

	for i, tt := range tests {
		cat, err := svc.CreateCategory(ctx, rawmaterial.CreateCategoryParams{
			Name:     tt.name,
			Position: int32(i + 1),
		})
		if err != nil {
			t.Fatalf("CreateCategory %q: %v", tt.name, err)
		}
		if cat.Slug != tt.wantSlug {
			t.Errorf("slugify(%q): got %q, want %q", tt.name, cat.Slug, tt.wantSlug)
		}
	}
}

package product_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/product"
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

func newService() *product.Service {
	return product.NewService(testDB.Pool, nil)
}

func minimalCreateParams(name string) product.CreateProductParams {
	return product.CreateProductParams{
		Name:                    name,
		Status:                  "active",
		BasePrice:               pgtype.Numeric{Int: big.NewInt(2500), Exp: -2, Valid: true},
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
		Metadata:                json.RawMessage(`{}`),
	}
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	p, err := svc.Create(ctx, minimalCreateParams("Leather Bag"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if p.Name != "Leather Bag" {
		t.Errorf("name: got %q, want %q", p.Name, "Leather Bag")
	}
	if p.Slug != "leather-bag" {
		t.Errorf("slug: got %q, want %q", p.Slug, "leather-bag")
	}
	if p.Status != "active" {
		t.Errorf("status: got %q, want %q", p.Status, "active")
	}
}

func TestCreate_DefaultDraftStatus(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	params := minimalCreateParams("Draft Product")
	params.Status = "" // should default to "draft"

	p, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.Status != "draft" {
		t.Errorf("status: got %q, want %q", p.Status, "draft")
	}
}

func TestCreate_AutoSlug(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	p, err := svc.Create(ctx, minimalCreateParams("Waxed Canvas Tote"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.Slug != "waxed-canvas-tote" {
		t.Errorf("slug: got %q, want %q", p.Slug, "waxed-canvas-tote")
	}
}

func TestCreate_CustomSlug(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	params := minimalCreateParams("My Product")
	params.Slug = "custom-slug"

	p, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.Slug != "custom-slug" {
		t.Errorf("slug: got %q, want %q", p.Slug, "custom-slug")
	}
}

func TestCreate_DuplicateSlugAutoSuffix(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	// Create first product.
	_, err := svc.Create(ctx, minimalCreateParams("Duplicate Name"))
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// Create second product with same name — slug should get a suffix.
	p2, err := svc.Create(ctx, minimalCreateParams("Duplicate Name"))
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if p2.Slug == "duplicate-name" {
		t.Error("expected second product to have a different slug")
	}
	if len(p2.Slug) <= len("duplicate-name") {
		t.Errorf("expected slug with suffix, got %q", p2.Slug)
	}
}

func TestCreate_NameRequired(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	params := minimalCreateParams("")
	_, err := svc.Create(ctx, params)
	if err != product.ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestCreate_WithOptionalFields(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	desc := "A fine bag"
	shortDesc := "Fine bag"
	seoTitle := "Buy Leather Bag"
	params := minimalCreateParams("Full Product")
	params.Description = &desc
	params.ShortDescription = &shortDesc
	params.SeoTitle = &seoTitle
	params.HasVariants = true
	params.BaseWeightGrams = 500

	p, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.Description == nil || *p.Description != desc {
		t.Errorf("description: got %v, want %q", p.Description, desc)
	}
	if !p.HasVariants {
		t.Error("expected has_variants=true")
	}
	if p.BaseWeightGrams != 500 {
		t.Errorf("weight: got %d, want 500", p.BaseWeightGrams)
	}
}

// --------------------------------------------------------------------------
// Get / GetBySlug
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, minimalCreateParams("Get Test"))

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Get Test" {
		t.Errorf("name: got %q, want %q", got.Name, "Get Test")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != product.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBySlug(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Slug Lookup"))

	got, err := svc.GetBySlug(ctx, "slug-lookup")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Name != "Slug Lookup" {
		t.Errorf("name: got %q, want %q", got.Name, "Slug Lookup")
	}
}

func TestGetBySlug_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetBySlug(ctx, "nonexistent")
	if err != product.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
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

	created, _ := svc.Create(ctx, minimalCreateParams("Original Name"))

	updated, err := svc.Update(ctx, created.ID, product.UpdateProductParams{
		Name:                    "Updated Name",
		Slug:                    created.Slug,
		Status:                  "active",
		BasePrice:               pgtype.Numeric{Int: big.NewInt(3000), Exp: -2, Valid: true},
		ShippingExtraFeePerUnit: pgtype.Numeric{Int: big.NewInt(0), Exp: -2, Valid: true},
		Metadata:                json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("name: got %q, want %q", updated.Name, "Updated Name")
	}
}

func TestUpdate_NameRequired(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, minimalCreateParams("To Update"))

	_, err := svc.Update(ctx, created.ID, product.UpdateProductParams{
		Name: "",
	})
	if err != product.ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), product.UpdateProductParams{
		Name:     "Nope",
		Metadata: json.RawMessage(`{}`),
	})
	if err != product.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdate_SlugPreserved(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, minimalCreateParams("Keep Slug"))

	updated, err := svc.Update(ctx, created.ID, product.UpdateProductParams{
		Name:                    "Changed Name But Keep Slug",
		Slug:                    "", // empty slug = keep existing
		Status:                  "active",
		BasePrice:               created.BasePrice,
		ShippingExtraFeePerUnit: created.ShippingExtraFeePerUnit,
		Metadata:                json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Slug != created.Slug {
		t.Errorf("slug changed: got %q, want %q", updated.Slug, created.Slug)
	}
}

func TestUpdate_NewSlugUniqueness(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Existing Slug Product"))
	p2, _ := svc.Create(ctx, minimalCreateParams("Another Product"))

	// Try to update p2 with a slug that matches p1's.
	updated, err := svc.Update(ctx, p2.ID, product.UpdateProductParams{
		Name:                    "Another Product",
		Slug:                    "existing-slug-product",
		Status:                  "active",
		BasePrice:               p2.BasePrice,
		ShippingExtraFeePerUnit: p2.ShippingExtraFeePerUnit,
		Metadata:                json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Should get a different slug (with suffix).
	if updated.Slug == "existing-slug-product" {
		t.Error("expected slug to be modified due to conflict")
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

	created, _ := svc.Create(ctx, minimalCreateParams("To Delete"))

	err := svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone.
	_, err = svc.Get(ctx, created.ID)
	if err != product.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.Delete(ctx, uuid.New())
	if err != product.ErrNotFound {
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

	svc.Create(ctx, minimalCreateParams("Product A"))
	svc.Create(ctx, minimalCreateParams("Product B"))
	svc.Create(ctx, minimalCreateParams("Product C"))

	products, total, err := svc.List(ctx, nil, 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total: got %d, want 3", total)
	}
	if len(products) != 3 {
		t.Errorf("products: got %d, want 3", len(products))
	}
}

func TestList_StatusFilter(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Active One"))

	draftParams := minimalCreateParams("Draft One")
	draftParams.Status = "draft"
	svc.Create(ctx, draftParams)

	active := "active"
	products, total, err := svc.List(ctx, &active, 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total: got %d, want 1", total)
	}
	if len(products) != 1 {
		t.Errorf("products: got %d, want 1", len(products))
	}
}

func TestList_Pagination(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Create(ctx, minimalCreateParams(fmt.Sprintf("Paginated %d", i)))
	}

	// Page 1, size 2.
	p1, total, _ := svc.List(ctx, nil, 1, 2)
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(p1) != 2 {
		t.Errorf("page 1: got %d, want 2", len(p1))
	}

	// Page 3, size 2 = last page with 1 item.
	p3, _, _ := svc.List(ctx, nil, 3, 2)
	if len(p3) != 1 {
		t.Errorf("page 3: got %d, want 1", len(p3))
	}
}

func TestList_BoundsProtection(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Bounds Test"))

	// Negative page/pageSize should be clamped.
	products, _, err := svc.List(ctx, nil, -1, -5)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("products: got %d, want 1", len(products))
	}

	// Oversized page size clamped to 250.
	products, _, err = svc.List(ctx, nil, 1, 500)
	if err != nil {
		t.Fatalf("List large: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("products: got %d, want 1", len(products))
	}
}

// --------------------------------------------------------------------------
// Search
// --------------------------------------------------------------------------

func TestSearch(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Leather Bag"))
	svc.Create(ctx, minimalCreateParams("Canvas Tote"))
	svc.Create(ctx, minimalCreateParams("Leather Wallet"))

	results, err := svc.Search(ctx, "Leather", nil, 1, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("search results: got %d, want 2", len(results))
	}
}

func TestSearch_NoResults(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Something"))

	results, err := svc.Search(ctx, "nonexistent", nil, 1, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search results: got %d, want 0", len(results))
	}
}

func TestSearch_WithStatusFilter(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, minimalCreateParams("Active Leather"))
	draft := minimalCreateParams("Draft Leather")
	draft.Status = "draft"
	svc.Create(ctx, draft)

	active := "active"
	results, err := svc.Search(ctx, "Leather", &active, 1, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("search results: got %d, want 1", len(results))
	}
}

// --------------------------------------------------------------------------
// SetCategories / GetCategories
// --------------------------------------------------------------------------

func TestSetCategories(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	p, _ := svc.Create(ctx, minimalCreateParams("Categorized Product"))
	cat1 := testDB.FixtureCategory(t, "Bags", "bags")
	cat2 := testDB.FixtureCategory(t, "Leather", "leather")

	err := svc.SetCategories(ctx, p.ID, []uuid.UUID{cat1.ID, cat2.ID})
	if err != nil {
		t.Fatalf("SetCategories: %v", err)
	}

	categories, err := svc.GetCategories(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if len(categories) != 2 {
		t.Errorf("categories: got %d, want 2", len(categories))
	}
}

func TestSetCategories_Replace(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	p, _ := svc.Create(ctx, minimalCreateParams("Replace Cats"))
	cat1 := testDB.FixtureCategory(t, "Old Cat", "old-cat")
	cat2 := testDB.FixtureCategory(t, "New Cat", "new-cat")

	// Set to cat1.
	svc.SetCategories(ctx, p.ID, []uuid.UUID{cat1.ID})

	// Replace with cat2.
	err := svc.SetCategories(ctx, p.ID, []uuid.UUID{cat2.ID})
	if err != nil {
		t.Fatalf("SetCategories replace: %v", err)
	}

	categories, _ := svc.GetCategories(ctx, p.ID)
	if len(categories) != 1 {
		t.Fatalf("categories: got %d, want 1", len(categories))
	}
	if categories[0].ID != cat2.ID {
		t.Errorf("category: got %s, want %s", categories[0].ID, cat2.ID)
	}
}

func TestSetCategories_ProductNotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.SetCategories(ctx, uuid.New(), []uuid.UUID{uuid.New()})
	if err != product.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCategories_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	p, _ := svc.Create(ctx, minimalCreateParams("No Cats"))

	categories, err := svc.GetCategories(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if len(categories) != 0 {
		t.Errorf("categories: got %d, want 0", len(categories))
	}
}

package category_test

import (
	"context"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/category"
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

func newService() *category.Service {
	return category.NewService(testDB.Pool, slog.Default())
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, err := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Bags",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if cat.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if cat.Name != "Bags" {
		t.Errorf("name: got %q, want %q", cat.Name, "Bags")
	}
	if cat.Slug != "bags" {
		t.Errorf("slug: got %q, want %q", cat.Slug, "bags")
	}
	if !cat.IsActive {
		t.Error("expected is_active=true")
	}
}

func TestCreate_AutoSlug(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, err := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Leather Goods",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cat.Slug != "leather-goods" {
		t.Errorf("slug: got %q, want %q", cat.Slug, "leather-goods")
	}
}

func TestCreate_CustomSlug(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat, err := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "My Category",
		Slug:     "custom-cat-slug",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cat.Slug != "custom-cat-slug" {
		t.Errorf("slug: got %q, want %q", cat.Slug, "custom-cat-slug")
	}
}

func TestCreate_WithParent(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	parent, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Parent",
		IsActive: true,
	})

	child, err := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Child",
		ParentID: &parent.ID,
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}
	if !child.ParentID.Valid {
		t.Error("expected child to have parent_id set")
	}
}

func TestCreate_WithOptionalFields(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	desc := "Fine leather goods"
	seoTitle := "Buy Leather"
	cat, err := svc.Create(ctx, category.CreateCategoryParams{
		Name:        "Full Category",
		Description: &desc,
		SeoTitle:    &seoTitle,
		Position:    5,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cat.Description == nil || *cat.Description != desc {
		t.Errorf("description: got %v, want %q", cat.Description, desc)
	}
	if cat.Position != 5 {
		t.Errorf("position: got %d, want 5", cat.Position)
	}
}

// --------------------------------------------------------------------------
// Get / GetBySlug
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Get Test",
		IsActive: true,
	})

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Get Test" {
		t.Errorf("name: got %q, want %q", got.Name, "Get Test")
	}
}

func TestGetBySlug(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Slug Lookup",
		IsActive: true,
	})

	got, err := svc.GetBySlug(ctx, "slug-lookup")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Name != "Slug Lookup" {
		t.Errorf("name: got %q, want %q", got.Name, "Slug Lookup")
	}
}

// --------------------------------------------------------------------------
// List
// --------------------------------------------------------------------------

func TestList_ActiveOnly(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, category.CreateCategoryParams{Name: "Active", IsActive: true})
	svc.Create(ctx, category.CreateCategoryParams{Name: "Inactive", IsActive: false})

	active, err := svc.List(ctx, true)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("active: got %d, want 1", len(active))
	}
}

func TestList_All(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, category.CreateCategoryParams{Name: "Active", Slug: "active", IsActive: true})
	svc.Create(ctx, category.CreateCategoryParams{Name: "Inactive", Slug: "inactive", IsActive: false})

	all, err := svc.List(ctx, false)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all: got %d, want 2", len(all))
	}
}

func TestList_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cats, err := svc.List(ctx, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cats) != 0 {
		t.Errorf("expected 0, got %d", len(cats))
	}
}

// --------------------------------------------------------------------------
// ListTop / ListChildren
// --------------------------------------------------------------------------

func TestListTop(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	parent, _ := svc.Create(ctx, category.CreateCategoryParams{Name: "Top Level", IsActive: true})
	svc.Create(ctx, category.CreateCategoryParams{Name: "Child", ParentID: &parent.ID, Slug: "child", IsActive: true})

	top, err := svc.ListTop(ctx)
	if err != nil {
		t.Fatalf("ListTop: %v", err)
	}
	if len(top) != 1 {
		t.Errorf("top-level: got %d, want 1", len(top))
	}
	if top[0].Name != "Top Level" {
		t.Errorf("name: got %q, want %q", top[0].Name, "Top Level")
	}
}

func TestListChildren(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	parent, _ := svc.Create(ctx, category.CreateCategoryParams{Name: "Parent", IsActive: true})
	svc.Create(ctx, category.CreateCategoryParams{Name: "Child 1", ParentID: &parent.ID, Slug: "child-1", IsActive: true})
	svc.Create(ctx, category.CreateCategoryParams{Name: "Child 2", ParentID: &parent.ID, Slug: "child-2", IsActive: true})

	children, err := svc.ListChildren(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("children: got %d, want 2", len(children))
	}
}

func TestListChildren_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	parent, _ := svc.Create(ctx, category.CreateCategoryParams{Name: "Childless", IsActive: true})

	children, err := svc.ListChildren(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("children: got %d, want 0", len(children))
	}
}

// --------------------------------------------------------------------------
// Update
// --------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Original",
		IsActive: true,
	})

	updated, err := svc.Update(ctx, created.ID, category.UpdateCategoryParams{
		Name:     "Renamed",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("name: got %q, want %q", updated.Name, "Renamed")
	}
	if updated.Slug != "renamed" {
		t.Errorf("slug: got %q, want %q", updated.Slug, "renamed")
	}
}

func TestUpdate_KeepSlug(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Original",
		Slug:     "original-slug",
		IsActive: true,
	})

	updated, err := svc.Update(ctx, created.ID, category.UpdateCategoryParams{
		Name:     "New Name",
		Slug:     "original-slug",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Slug != "original-slug" {
		t.Errorf("slug: got %q, want %q", updated.Slug, "original-slug")
	}
}

// --------------------------------------------------------------------------
// Delete
// --------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "To Delete",
		IsActive: true,
	})

	err := svc.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// --------------------------------------------------------------------------
// CountProducts
// --------------------------------------------------------------------------

func TestCountProducts(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	cat, _ := svc.Create(ctx, category.CreateCategoryParams{
		Name:     "Count Test",
		IsActive: true,
	})

	count, err := svc.CountProducts(ctx, cat.ID)
	if err != nil {
		t.Fatalf("CountProducts: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

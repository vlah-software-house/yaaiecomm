package cart_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/cart"
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

func newService() *cart.Service {
	return cart.NewService(testDB.Pool, nil)
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	c, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if c.ID == uuid.Nil {
		t.Error("expected non-nil cart ID")
	}
	if c.ExpiresAt.Before(c.CreatedAt) {
		t.Error("expiry should be after creation time")
	}
}

// --------------------------------------------------------------------------
// Get
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Get: got ID %s, want %s", got.ID, created.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != cart.ErrNotFound {
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

	c, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	email := "test@example.com"
	country := "DE"
	updated, err := svc.Update(ctx, c.ID, cart.UpdateParams{
		Email:       &email,
		CountryCode: &country,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Email == nil || *updated.Email != email {
		t.Errorf("email: got %v, want %q", updated.Email, email)
	}
	if updated.CountryCode == nil || *updated.CountryCode != country {
		t.Errorf("country: got %v, want %q", updated.CountryCode, country)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	email := "test@example.com"
	_, err := svc.Update(ctx, uuid.New(), cart.UpdateParams{Email: &email})
	if err != cart.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// AddItem
// --------------------------------------------------------------------------

func TestAddItem(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	variant := testDB.FixtureVariant(t, product.ID, "TST-001", 10)

	c, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	item, err := svc.AddItem(ctx, c.ID, variant.ID, 2)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if item.CartID != c.ID {
		t.Errorf("item cart ID: got %s, want %s", item.CartID, c.ID)
	}
	if item.VariantID != variant.ID {
		t.Errorf("item variant ID: got %s, want %s", item.VariantID, variant.ID)
	}
	if item.Quantity != 2 {
		t.Errorf("item quantity: got %d, want 2", item.Quantity)
	}
}

func TestAddItem_UpsertIncrementsQuantity(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	variant := testDB.FixtureVariant(t, product.ID, "TST-001", 50)

	c, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Add 2 first.
	_, err = svc.AddItem(ctx, c.ID, variant.ID, 2)
	if err != nil {
		t.Fatalf("AddItem first: %v", err)
	}

	// Add 3 more of the same variant — should increment to 5.
	item, err := svc.AddItem(ctx, c.ID, variant.ID, 3)
	if err != nil {
		t.Fatalf("AddItem second: %v", err)
	}

	if item.Quantity != 5 {
		t.Errorf("upsert quantity: got %d, want 5", item.Quantity)
	}
}

func TestAddItem_InvalidQuantity(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.AddItem(ctx, uuid.New(), uuid.New(), 0)
	if err != cart.ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity for qty=0, got %v", err)
	}

	_, err = svc.AddItem(ctx, uuid.New(), uuid.New(), -1)
	if err != cart.ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity for qty=-1, got %v", err)
	}
}

// --------------------------------------------------------------------------
// UpdateItemQuantity
// --------------------------------------------------------------------------

func TestUpdateItemQuantity(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	variant := testDB.FixtureVariant(t, product.ID, "TST-001", 20)

	c, _ := svc.Create(ctx)
	item, _ := svc.AddItem(ctx, c.ID, variant.ID, 1)

	updated, err := svc.UpdateItemQuantity(ctx, item.ID, 5)
	if err != nil {
		t.Fatalf("UpdateItemQuantity: %v", err)
	}
	if updated.Quantity != 5 {
		t.Errorf("updated quantity: got %d, want 5", updated.Quantity)
	}
}

func TestUpdateItemQuantity_InvalidQuantity(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateItemQuantity(ctx, uuid.New(), 0)
	if err != cart.ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestUpdateItemQuantity_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateItemQuantity(ctx, uuid.New(), 1)
	if err != cart.ErrItemNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// RemoveItem
// --------------------------------------------------------------------------

func TestRemoveItem(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	variant := testDB.FixtureVariant(t, product.ID, "TST-001", 10)

	c, _ := svc.Create(ctx)
	item, _ := svc.AddItem(ctx, c.ID, variant.ID, 2)

	err := svc.RemoveItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("RemoveItem: %v", err)
	}

	// Verify item is gone.
	items, err := svc.ListItems(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListItems after remove: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items after removal, got %d", len(items))
	}
}

// --------------------------------------------------------------------------
// Clear
// --------------------------------------------------------------------------

func TestClear(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	v1 := testDB.FixtureVariant(t, product.ID, "TST-001", 10)
	v2 := testDB.FixtureVariant(t, product.ID, "TST-002", 10)

	c, _ := svc.Create(ctx)
	svc.AddItem(ctx, c.ID, v1.ID, 1)
	svc.AddItem(ctx, c.ID, v2.ID, 3)

	err := svc.Clear(ctx, c.ID)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}

	items, _ := svc.ListItems(ctx, c.ID)
	if len(items) != 0 {
		t.Errorf("expected 0 items after clear, got %d", len(items))
	}

	// Cart itself should still exist.
	_, err = svc.Get(ctx, c.ID)
	if err != nil {
		t.Errorf("cart should still exist after clear: %v", err)
	}
}

// --------------------------------------------------------------------------
// ListItems
// --------------------------------------------------------------------------

func TestListItems(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Test Product", "test-product")
	v1 := testDB.FixtureVariant(t, product.ID, "TST-001", 10)
	v2 := testDB.FixtureVariant(t, product.ID, "TST-002", 20)

	c, _ := svc.Create(ctx)
	svc.AddItem(ctx, c.ID, v1.ID, 1)
	svc.AddItem(ctx, c.ID, v2.ID, 2)

	items, err := svc.ListItems(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Verify items have product info joined.
	for _, item := range items {
		if item.VariantSku == "" {
			t.Error("expected variant SKU to be populated from join")
		}
		if item.ProductName == "" {
			t.Error("expected product name to be populated from join")
		}
	}
}

func TestListItems_EmptyCart(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	c, _ := svc.Create(ctx)
	items, err := svc.ListItems(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListItems empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty cart, got %d", len(items))
	}
}

// --------------------------------------------------------------------------
// SetCustomer
// --------------------------------------------------------------------------

func TestSetCustomer(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	customer := testDB.FixtureCustomer(t, "customer@example.com")
	c, _ := svc.Create(ctx)

	updated, err := svc.SetCustomer(ctx, c.ID, customer.ID)
	if err != nil {
		t.Fatalf("SetCustomer: %v", err)
	}

	if !updated.CustomerID.Valid || updated.CustomerID.Bytes != customer.ID {
		t.Errorf("customer ID: got %v, want %s", updated.CustomerID, customer.ID)
	}
}

func TestSetCustomer_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.SetCustomer(ctx, uuid.New(), uuid.New())
	if err != cart.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// DeleteExpired
// --------------------------------------------------------------------------

func TestDeleteExpired(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create a cart with already-expired timestamp.
	c, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Manually expire it.
	_, err = testDB.Pool.Exec(ctx,
		`UPDATE carts SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`,
		c.ID,
	)
	if err != nil {
		t.Fatalf("expiring cart: %v", err)
	}

	// Create a fresh (non-expired) cart.
	fresh, _ := svc.Create(ctx)

	// Run cleanup.
	err = svc.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	// Expired cart should be gone.
	_, err = svc.Get(ctx, c.ID)
	if err != cart.ErrNotFound {
		t.Errorf("expired cart should be deleted, got %v", err)
	}

	// Fresh cart should still exist.
	_, err = svc.Get(ctx, fresh.ID)
	if err != nil {
		t.Errorf("fresh cart should still exist: %v", err)
	}
}

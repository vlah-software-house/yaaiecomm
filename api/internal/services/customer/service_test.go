package customer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/customer"
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

func newService() *customer.Service {
	return customer.NewService(testDB.Pool, nil)
}

func strPtr(s string) *string { return &s }

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cust, err := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "alice@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if cust.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if cust.Email != "alice@example.com" {
		t.Errorf("email: got %q, want %q", cust.Email, "alice@example.com")
	}
}

func TestCreate_WithOptionalFields(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	first := "Bob"
	last := "Smith"
	phone := "+34612345678"
	vat := "ES12345678A"

	cust, err := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "bob@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
		FirstName:    &first,
		LastName:     &last,
		Phone:        &phone,
		VatNumber:    &vat,
		Metadata:     json.RawMessage(`{"source":"test"}`),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if cust.FirstName == nil || *cust.FirstName != "Bob" {
		t.Errorf("first_name: got %v, want %q", cust.FirstName, "Bob")
	}
	if cust.LastName == nil || *cust.LastName != "Smith" {
		t.Errorf("last_name: got %v, want %q", cust.LastName, "Smith")
	}
	if cust.Phone == nil || *cust.Phone != "+34612345678" {
		t.Errorf("phone: got %v, want %q", cust.Phone, "+34612345678")
	}
	if cust.VatNumber == nil || *cust.VatNumber != "ES12345678A" {
		t.Errorf("vat_number: got %v, want %q", cust.VatNumber, "ES12345678A")
	}
}

func TestCreate_NilMetadataDefaults(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// No metadata/address fields set — should default to {}.
	cust, err := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "defaults@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})
	if err != nil {
		t.Fatalf("Create with nil JSONB: %v", err)
	}
	if cust.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "dup@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})

	_, err := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "dup@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})
	if err != customer.ErrEmailTaken {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Get / GetByEmail
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "get@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Email != "get@example.com" {
		t.Errorf("email: got %q, want %q", got.Email, "get@example.com")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != customer.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByEmail(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "lookup@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})

	got, err := svc.GetByEmail(ctx, "lookup@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Email != "lookup@example.com" {
		t.Errorf("email: got %q, want %q", got.Email, "lookup@example.com")
	}
}

func TestGetByEmail_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetByEmail(ctx, "nobody@example.com")
	if err != customer.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// List
// --------------------------------------------------------------------------

func TestList(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, customer.CreateCustomerParams{Email: "a@example.com", PasswordHash: strPtr("$2a$12$hash1hashhashhashhashhashhashhashhashhashhashhashhashhash")})
	svc.Create(ctx, customer.CreateCustomerParams{Email: "b@example.com", PasswordHash: strPtr("$2a$12$hash2hashhashhashhashhashhashhashhashhashhashhashhashhash")})
	svc.Create(ctx, customer.CreateCustomerParams{Email: "c@example.com", PasswordHash: strPtr("$2a$12$hash3hashhashhashhashhashhashhashhashhashhashhashhashhash")})

	customers, total, err := svc.List(ctx, 1, 20)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total: got %d, want 3", total)
	}
	if len(customers) != 3 {
		t.Errorf("customers: got %d, want 3", len(customers))
	}
}

func TestList_Pagination(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Create(ctx, customer.CreateCustomerParams{
			Email:        fmt.Sprintf("page%d@example.com", i),
			PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
		})
	}

	p1, total, _ := svc.List(ctx, 1, 2)
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(p1) != 2 {
		t.Errorf("page 1: got %d, want 2", len(p1))
	}

	p3, _, _ := svc.List(ctx, 3, 2)
	if len(p3) != 1 {
		t.Errorf("page 3: got %d, want 1", len(p3))
	}
}

func TestList_BoundsProtection(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "bounds@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})

	customers, _, err := svc.List(ctx, -1, -5)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(customers) != 1 {
		t.Errorf("customers: got %d, want 1", len(customers))
	}
}

// --------------------------------------------------------------------------
// Update
// --------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.Create(ctx, customer.CreateCustomerParams{
		Email:        "update@example.com",
		PasswordHash: strPtr("$2a$12$hashhashhashhashhashhashhashhashhashhashhashhashhashhash"),
	})

	first := "Updated"
	last := "Name"
	phone := "+34999999999"
	updated, err := svc.Update(ctx, created.ID, customer.UpdateCustomerParams{
		FirstName: &first,
		LastName:  &last,
		Phone:     &phone,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.FirstName == nil || *updated.FirstName != "Updated" {
		t.Errorf("first_name: got %v, want %q", updated.FirstName, "Updated")
	}
	if updated.LastName == nil || *updated.LastName != "Name" {
		t.Errorf("last_name: got %v, want %q", updated.LastName, "Name")
	}
	if updated.Phone == nil || *updated.Phone != "+34999999999" {
		t.Errorf("phone: got %v, want %q", updated.Phone, "+34999999999")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), customer.UpdateCustomerParams{})
	if err != customer.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

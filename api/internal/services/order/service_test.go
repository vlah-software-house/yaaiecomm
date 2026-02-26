package order_test

import (
	"context"
	"encoding/json"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/order"
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

func newService() *order.Service {
	return order.NewService(testDB.Pool, nil)
}

func numericFromCents(cents int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(cents), Exp: -2, Valid: true}
}

func minimalOrderParams() order.CreateOrderParams {
	zero := numericFromCents(0)
	return order.CreateOrderParams{
		Status:            "pending",
		Email:             "buyer@example.com",
		PaymentStatus:     "unpaid",
		BillingAddress:    json.RawMessage(`{"city":"Madrid"}`),
		ShippingAddress:   json.RawMessage(`{"city":"Madrid"}`),
		Subtotal:          numericFromCents(5000),
		ShippingFee:       zero,
		ShippingExtraFees: zero,
		DiscountAmount:    zero,
		VatTotal:          zero,
		Total:             numericFromCents(5000),
		Metadata:          json.RawMessage(`{}`),
		Items: []order.CreateOrderItemInput{
			{
				ProductName:    "Test Product",
				Quantity:       1,
				UnitPrice:      numericFromCents(5000),
				TotalPrice:     numericFromCents(5000),
				VatRate:        zero,
				VatAmount:      zero,
				NetUnitPrice:   numericFromCents(5000),
				GrossUnitPrice: numericFromCents(5000),
				Metadata:       json.RawMessage(`{}`),
			},
		},
	}
}

// --------------------------------------------------------------------------
// Create
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	params := minimalOrderParams()
	o, items, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if o.ID == uuid.Nil {
		t.Error("expected non-nil order ID")
	}
	if o.OrderNumber <= 0 {
		t.Errorf("expected positive order number, got %d", o.OrderNumber)
	}
	if o.Email != "buyer@example.com" {
		t.Errorf("email: got %q, want %q", o.Email, "buyer@example.com")
	}
	if o.Status != "pending" {
		t.Errorf("status: got %q, want %q", o.Status, "pending")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ProductName != "Test Product" {
		t.Errorf("item name: got %q, want %q", items[0].ProductName, "Test Product")
	}
}

func TestCreate_DefaultStatuses(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	params := minimalOrderParams()
	params.Status = ""
	params.PaymentStatus = ""

	o, _, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if o.Status != "pending" {
		t.Errorf("expected default status 'pending', got %q", o.Status)
	}
	if o.PaymentStatus != "unpaid" {
		t.Errorf("expected default payment status 'unpaid', got %q", o.PaymentStatus)
	}
}

func TestCreate_AutoIncrementsOrderNumber(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	o1, _, _ := svc.Create(ctx, minimalOrderParams())
	o2, _, _ := svc.Create(ctx, minimalOrderParams())

	if o2.OrderNumber <= o1.OrderNumber {
		t.Errorf("order numbers should auto-increment: %d <= %d",
			o2.OrderNumber, o1.OrderNumber)
	}
}

func TestCreate_RecordsInitialEvent(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	o, _, err := svc.Create(ctx, minimalOrderParams())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	events, err := svc.ListEvents(ctx, o.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "order_created" {
		t.Errorf("event type: got %q, want %q", events[0].EventType, "order_created")
	}
}

func TestCreate_MultipleItems(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	params := minimalOrderParams()
	params.Items = append(params.Items, order.CreateOrderItemInput{
		ProductName:    "Second Product",
		Quantity:       3,
		UnitPrice:      numericFromCents(1500),
		TotalPrice:     numericFromCents(4500),
		VatRate:        numericFromCents(0),
		VatAmount:      numericFromCents(0),
		NetUnitPrice:   numericFromCents(1500),
		GrossUnitPrice: numericFromCents(1500),
		Metadata:       json.RawMessage(`{}`),
	})

	o, items, err := svc.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Verify items are retrievable.
	fetched, err := svc.ListItems(ctx, o.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(fetched) != 2 {
		t.Errorf("ListItems: expected 2, got %d", len(fetched))
	}
}

// --------------------------------------------------------------------------
// Get
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _, _ := svc.Create(ctx, minimalOrderParams())

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Get: ID mismatch")
	}
	if got.OrderNumber != created.OrderNumber {
		t.Errorf("Get: order number mismatch")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != order.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// GetByNumber
// --------------------------------------------------------------------------

func TestGetByNumber(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _, _ := svc.Create(ctx, minimalOrderParams())

	got, err := svc.GetByNumber(ctx, created.OrderNumber)
	if err != nil {
		t.Fatalf("GetByNumber: %v", err)
	}
	if got.ID != created.ID {
		t.Error("GetByNumber: ID mismatch")
	}
}

func TestGetByNumber_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetByNumber(ctx, 999999)
	if err != order.ErrNotFound {
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

	// Create 3 orders.
	svc.Create(ctx, minimalOrderParams())
	svc.Create(ctx, minimalOrderParams())
	svc.Create(ctx, minimalOrderParams())

	orders, total, err := svc.List(ctx, nil, 1, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total: got %d, want 3", total)
	}
	if len(orders) != 3 {
		t.Errorf("orders: got %d, want 3", len(orders))
	}
}

func TestList_StatusFilter(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create 2 pending orders.
	svc.Create(ctx, minimalOrderParams())
	svc.Create(ctx, minimalOrderParams())

	// Create 1 and update to "shipped".
	o3, _, _ := svc.Create(ctx, minimalOrderParams())
	svc.UpdateStatus(ctx, o3.ID, "shipped")

	shipped := "shipped"
	orders, total, err := svc.List(ctx, &shipped, 1, 10)
	if err != nil {
		t.Fatalf("List with filter: %v", err)
	}
	if total != 1 {
		t.Errorf("total: got %d, want 1", total)
	}
	if len(orders) != 1 {
		t.Errorf("orders: got %d, want 1", len(orders))
	}
}

func TestList_Pagination(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Create(ctx, minimalOrderParams())
	}

	// Page 1: 2 items.
	page1, total, _ := svc.List(ctx, nil, 1, 2)
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(page1) != 2 {
		t.Errorf("page 1: got %d, want 2", len(page1))
	}

	// Page 2: 2 items.
	page2, _, _ := svc.List(ctx, nil, 2, 2)
	if len(page2) != 2 {
		t.Errorf("page 2: got %d, want 2", len(page2))
	}

	// Page 3: 1 item.
	page3, _, _ := svc.List(ctx, nil, 3, 2)
	if len(page3) != 1 {
		t.Errorf("page 3: got %d, want 1", len(page3))
	}
}

func TestList_BoundsProtection(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// page < 1 should default to 1, pageSize < 1 should default to 20.
	_, _, err := svc.List(ctx, nil, 0, 0)
	if err != nil {
		t.Errorf("List with zero bounds should not error: %v", err)
	}
}

// --------------------------------------------------------------------------
// UpdateStatus
// --------------------------------------------------------------------------

func TestUpdateStatus(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	o, _, _ := svc.Create(ctx, minimalOrderParams())

	updated, err := svc.UpdateStatus(ctx, o.ID, "shipped")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != "shipped" {
		t.Errorf("status: got %q, want %q", updated.Status, "shipped")
	}

	// Verify status_changed event was recorded.
	events, _ := svc.ListEvents(ctx, o.ID)
	if len(events) != 2 {
		t.Fatalf("expected 2 events (created + status_changed), got %d", len(events))
	}

	// Find the status_changed event.
	found := false
	for _, e := range events {
		if e.EventType == "status_changed" {
			found = true
			if e.FromStatus == nil || *e.FromStatus != "pending" {
				t.Errorf("from_status: got %v, want 'pending'", e.FromStatus)
			}
			if e.ToStatus == nil || *e.ToStatus != "shipped" {
				t.Errorf("to_status: got %v, want 'shipped'", e.ToStatus)
			}
		}
	}
	if !found {
		t.Error("expected status_changed event")
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateStatus(ctx, uuid.New(), "shipped")
	if err != order.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// UpdateTracking
// --------------------------------------------------------------------------

func TestUpdateTracking(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	o, _, _ := svc.Create(ctx, minimalOrderParams())

	tracking := "TRACK-12345"
	err := svc.UpdateTracking(ctx, o.ID, &tracking, pgtype.Timestamptz{})
	if err != nil {
		t.Fatalf("UpdateTracking: %v", err)
	}

	// Re-fetch and verify.
	got, _ := svc.Get(ctx, o.ID)
	if got.TrackingNumber == nil || *got.TrackingNumber != tracking {
		t.Errorf("tracking: got %v, want %q", got.TrackingNumber, tracking)
	}
}

func TestUpdateTracking_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	tracking := "TRACK-X"
	err := svc.UpdateTracking(ctx, uuid.New(), &tracking, pgtype.Timestamptz{})
	if err != order.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// ListItems
// --------------------------------------------------------------------------

func TestListItems_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Order with no items (create manually to bypass the normal Create flow).
	// Just use a non-existent ID — ListItems returns empty, not error.
	items, err := svc.ListItems(ctx, uuid.New())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// --------------------------------------------------------------------------
// ListEvents
// --------------------------------------------------------------------------

func TestListEvents_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	events, err := svc.ListEvents(ctx, uuid.New())
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

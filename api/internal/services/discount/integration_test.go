package discount_test

import (
	"context"
	"log"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/discount"
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

func newService() *discount.Service {
	return discount.NewService(testDB.Pool, nil)
}

// num builds a pgtype.Numeric from an int64 mantissa and exponent.
// Example: num(2500, -2) = 25.00
func num(intVal int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(intVal), Exp: exp, Valid: true}
}

// ts builds a pgtype.Timestamptz from a time.Time.
func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// nullTS returns a null pgtype.Timestamptz.
func nullTS() pgtype.Timestamptz {
	return pgtype.Timestamptz{}
}

// assertCentsEqual compares a pgtype.Numeric against an expected cents value.
func assertCentsEqual(t *testing.T, label string, got pgtype.Numeric, wantCents int64) {
	t.Helper()
	if !got.Valid || got.Int == nil {
		if wantCents == 0 {
			return
		}
		t.Errorf("%s: got invalid/nil numeric, want %d cents", label, wantCents)
		return
	}
	// Normalize to cents (exp -2).
	gotCents := new(big.Int).Set(got.Int)
	shift := int64(got.Exp) + 2
	if shift > 0 {
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(shift), nil)
		gotCents.Mul(gotCents, factor)
	} else if shift < 0 {
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(-shift), nil)
		gotCents.Div(gotCents, factor)
	}
	if gotCents.Cmp(big.NewInt(wantCents)) != 0 {
		t.Errorf("%s: got %s cents, want %d cents", label, gotCents.String(), wantCents)
	}
}

// createActiveDiscount is a helper that creates and returns an active discount.
func createActiveDiscount(t *testing.T, svc *discount.Service, name, typ, scope string, value pgtype.Numeric, priority int32, stackable bool) uuid.UUID {
	t.Helper()
	d, err := svc.CreateDiscount(context.Background(), discount.CreateDiscountParams{
		Name:      name,
		Type:      typ,
		Value:     value,
		Scope:     scope,
		IsActive:  true,
		Priority:  priority,
		Stackable: stackable,
	})
	if err != nil {
		t.Fatalf("creating discount %q: %v", name, err)
	}
	return d.ID
}

// --------------------------------------------------------------------------
// Discount CRUD
// --------------------------------------------------------------------------

func TestCreateDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, err := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Summer Sale",
		Type:     "percentage",
		Value:    num(1000, -2), // 10%
		Scope:    "subtotal",
		IsActive: true,
		Priority: 10,
	})
	if err != nil {
		t.Fatalf("CreateDiscount: %v", err)
	}

	if d.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if d.Name != "Summer Sale" {
		t.Errorf("name: got %q, want %q", d.Name, "Summer Sale")
	}
	if d.Type != "percentage" {
		t.Errorf("type: got %q, want %q", d.Type, "percentage")
	}
	if d.Scope != "subtotal" {
		t.Errorf("scope: got %q, want %q", d.Scope, "subtotal")
	}
	if !d.IsActive {
		t.Error("expected is_active=true")
	}
	if d.Priority != 10 {
		t.Errorf("priority: got %d, want 10", d.Priority)
	}
}

func TestCreateDiscount_NilConditionsDefault(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, err := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "No Conditions",
		Type:     "fixed_amount",
		Value:    num(500, -2),
		Scope:    "subtotal",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateDiscount: %v", err)
	}
	if d.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestGetDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Get Test",
		Type:     "percentage",
		Value:    num(500, -2),
		Scope:    "subtotal",
		IsActive: true,
	})

	got, err := svc.GetDiscount(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetDiscount: %v", err)
	}
	if got.Name != "Get Test" {
		t.Errorf("name: got %q, want %q", got.Name, "Get Test")
	}
}

func TestGetDiscount_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetDiscount(ctx, uuid.New())
	if err != discount.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListDiscounts(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.CreateDiscount(ctx, discount.CreateDiscountParams{Name: "D1", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true})
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{Name: "D2", Type: "fixed_amount", Value: num(300, -2), Scope: "shipping", IsActive: true})
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{Name: "D3", Type: "percentage", Value: num(1500, -2), Scope: "total", IsActive: false})

	all, err := svc.ListDiscounts(ctx, 20, 0)
	if err != nil {
		t.Fatalf("ListDiscounts: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("count: got %d, want 3", len(all))
	}
}

func TestListDiscounts_BoundsProtection(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.CreateDiscount(ctx, discount.CreateDiscountParams{Name: "D1", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true})

	// Negative limit/offset should default to safe values.
	all, err := svc.ListDiscounts(ctx, -1, -5)
	if err != nil {
		t.Fatalf("ListDiscounts: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("count: got %d, want 1", len(all))
	}
}

func TestUpdateDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Original",
		Type:     "percentage",
		Value:    num(500, -2),
		Scope:    "subtotal",
		IsActive: true,
		Priority: 1,
	})

	updated, err := svc.UpdateDiscount(ctx, created.ID, discount.UpdateDiscountParams{
		Name:     "Updated",
		Type:     "fixed_amount",
		Value:    num(1000, -2),
		Scope:    "total",
		IsActive: false,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("UpdateDiscount: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("name: got %q, want %q", updated.Name, "Updated")
	}
	if updated.Type != "fixed_amount" {
		t.Errorf("type: got %q, want %q", updated.Type, "fixed_amount")
	}
	if updated.Scope != "total" {
		t.Errorf("scope: got %q, want %q", updated.Scope, "total")
	}
	if updated.IsActive {
		t.Error("expected is_active=false")
	}
	if updated.Priority != 5 {
		t.Errorf("priority: got %d, want 5", updated.Priority)
	}
}

func TestUpdateDiscount_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateDiscount(ctx, uuid.New(), discount.UpdateDiscountParams{
		Name: "Nope", Type: "percentage", Value: num(500, -2), Scope: "subtotal",
	})
	if err != discount.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "To Delete", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})

	err := svc.DeleteDiscount(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteDiscount: %v", err)
	}

	// Verify it's gone.
	_, err = svc.GetDiscount(ctx, created.ID)
	if err != discount.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Coupon CRUD
// --------------------------------------------------------------------------

func TestCreateCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "Base Discount", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: true,
	})

	c, err := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "SUMMER10",
		DiscountID: d.ID,
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("CreateCoupon: %v", err)
	}
	if c.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if c.Code != "SUMMER10" {
		t.Errorf("code: got %q, want %q", c.Code, "SUMMER10")
	}
	if c.DiscountID != d.ID {
		t.Error("discount_id mismatch")
	}
	if c.UsageCount != 0 {
		t.Errorf("usage_count: got %d, want 0", c.UsageCount)
	}
}

func TestCreateCoupon_CodeNormalized(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "Norm Discount", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})

	c, err := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "  spring5  ",
		DiscountID: d.ID,
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("CreateCoupon: %v", err)
	}
	if c.Code != "SPRING5" {
		t.Errorf("code: got %q, want %q (trimmed + uppercased)", c.Code, "SPRING5")
	}
}

func TestCreateCoupon_DiscountNotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "ORPHAN",
		DiscountID: uuid.New(),
		IsActive:   true,
	})
	if err != discount.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	created, _ := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "GET1", DiscountID: d.ID, IsActive: true,
	})

	got, err := svc.GetCoupon(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCoupon: %v", err)
	}
	if got.Code != "GET1" {
		t.Errorf("code: got %q, want %q", got.Code, "GET1")
	}
}

func TestGetCoupon_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetCoupon(ctx, uuid.New())
	if err != discount.ErrCouponNotFound {
		t.Errorf("expected ErrCouponNotFound, got %v", err)
	}
}

func TestGetCouponByCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "BYLOOKUP", DiscountID: d.ID, IsActive: true,
	})

	got, err := svc.GetCouponByCode(ctx, "BYLOOKUP")
	if err != nil {
		t.Fatalf("GetCouponByCode: %v", err)
	}
	if got.Code != "BYLOOKUP" {
		t.Errorf("code: got %q, want %q", got.Code, "BYLOOKUP")
	}
}

func TestGetCouponByCode_CaseInsensitive(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "MIXEDCASE", DiscountID: d.ID, IsActive: true,
	})

	// The service normalizes input to uppercase before lookup.
	got, err := svc.GetCouponByCode(ctx, "  mixedCase  ")
	if err != nil {
		t.Fatalf("GetCouponByCode: %v", err)
	}
	if got.Code != "MIXEDCASE" {
		t.Errorf("code: got %q, want %q", got.Code, "MIXEDCASE")
	}
}

func TestGetCouponByCode_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetCouponByCode(ctx, "NOSUCH")
	if err != discount.ErrCouponNotFound {
		t.Errorf("expected ErrCouponNotFound, got %v", err)
	}
}

func TestListCoupons(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "Parent Discount", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{Code: "C1", DiscountID: d.ID, IsActive: true})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{Code: "C2", DiscountID: d.ID, IsActive: true})

	rows, err := svc.ListCoupons(ctx, 20, 0)
	if err != nil {
		t.Fatalf("ListCoupons: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("count: got %d, want 2", len(rows))
	}
	// ListCoupons JOIN returns discount_name.
	if rows[0].DiscountName != "Parent Discount" {
		t.Errorf("discount_name: got %q, want %q", rows[0].DiscountName, "Parent Discount")
	}
}

func TestIncrementCouponUsage(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	c, _ := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "INCR", DiscountID: d.ID, IsActive: true,
	})
	if c.UsageCount != 0 {
		t.Fatalf("initial usage_count: got %d, want 0", c.UsageCount)
	}

	if err := svc.IncrementCouponUsage(ctx, c.ID); err != nil {
		t.Fatalf("IncrementCouponUsage: %v", err)
	}
	if err := svc.IncrementCouponUsage(ctx, c.ID); err != nil {
		t.Fatalf("IncrementCouponUsage second: %v", err)
	}

	got, _ := svc.GetCoupon(ctx, c.ID)
	if got.UsageCount != 2 {
		t.Errorf("usage_count: got %d, want 2", got.UsageCount)
	}
}

func TestDeleteCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	c, _ := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "DEL", DiscountID: d.ID, IsActive: true,
	})

	err := svc.DeleteCoupon(ctx, c.ID)
	if err != nil {
		t.Fatalf("DeleteCoupon: %v", err)
	}

	_, err = svc.GetCoupon(ctx, c.ID)
	if err != discount.ErrCouponNotFound {
		t.Errorf("expected ErrCouponNotFound after delete, got %v", err)
	}
}

func TestDeleteDiscount_CascadesCoupons(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "Cascade D", Type: "percentage", Value: num(500, -2), Scope: "subtotal", IsActive: true,
	})
	c, _ := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code: "CASCADE", DiscountID: d.ID, IsActive: true,
	})

	// Delete the discount — coupons should be cascade-deleted.
	err := svc.DeleteDiscount(ctx, d.ID)
	if err != nil {
		t.Fatalf("DeleteDiscount: %v", err)
	}

	_, err = svc.GetCoupon(ctx, c.ID)
	if err != discount.ErrCouponNotFound {
		t.Errorf("expected coupon cascade-deleted, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Apply engine
// --------------------------------------------------------------------------

func TestApply_NoDiscounts(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(24200, -2), // 242.00
		ShippingFee: num(850, -2),   // 8.50
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 0)
	if len(result.Breakdown) != 0 {
		t.Errorf("breakdown: got %d entries, want 0", len(result.Breakdown))
	}
}

func TestApply_SinglePercentageSubtotal(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 10% off subtotal.
	createActiveDiscount(t, svc, "10% Off", "percentage", "subtotal", num(1000, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(24200, -2), // 242.00
		ShippingFee: num(850, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 10% of 242.00 = 24.20 = 2420 cents
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 2420)
	if len(result.Breakdown) != 1 {
		t.Fatalf("breakdown: got %d, want 1", len(result.Breakdown))
	}
	if result.Breakdown[0].Scope != "subtotal" {
		t.Errorf("scope: got %q, want %q", result.Breakdown[0].Scope, "subtotal")
	}
	if result.Breakdown[0].Type != "percentage" {
		t.Errorf("type: got %q, want %q", result.Breakdown[0].Type, "percentage")
	}
}

func TestApply_SingleFixedSubtotal(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 5.00 off subtotal.
	createActiveDiscount(t, svc, "5 Off", "fixed_amount", "subtotal", num(500, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00
		ShippingFee: num(500, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 500)
}

func TestApply_ShippingScope(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Free shipping (100% off shipping).
	createActiveDiscount(t, svc, "Free Ship", "percentage", "shipping", num(10000, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(850, -2), // 8.50
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 100% of 8.50 = 850 cents
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 850)
	if len(result.Breakdown) != 1 {
		t.Fatalf("breakdown: got %d, want 1", len(result.Breakdown))
	}
	if result.Breakdown[0].Scope != "shipping" {
		t.Errorf("scope: got %q, want %q", result.Breakdown[0].Scope, "shipping")
	}
}

func TestApply_TotalScope(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 5.00 off total (subtotal + shipping).
	createActiveDiscount(t, svc, "5 Total", "fixed_amount", "total", num(500, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00
		ShippingFee: num(850, -2),   // 8.50
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 500)
	if result.Breakdown[0].Scope != "total" {
		t.Errorf("scope: got %q, want %q", result.Breakdown[0].Scope, "total")
	}
}

func TestApply_MinimumNotMet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 10% off but requires minimum 100.00.
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:          "Needs 100",
		Type:          "percentage",
		Value:         num(1000, -2),
		Scope:         "subtotal",
		MinimumAmount: num(10000, -2), // 100.00
		IsActive:      true,
		Priority:      10,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(5000, -2), // 50.00 — below minimum
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 0)
	if len(result.Breakdown) != 0 {
		t.Errorf("breakdown: got %d, want 0 (minimum not met)", len(result.Breakdown))
	}
}

func TestApply_MinimumMet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:          "Needs 50",
		Type:          "percentage",
		Value:         num(1000, -2), // 10%
		Scope:         "subtotal",
		MinimumAmount: num(5000, -2), // 50.00
		IsActive:      true,
		Priority:      10,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00 >= 50.00
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 10% of 100.00 = 10.00
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1000)
}

func TestApply_MaximumDiscountCap(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 50% off but max 15.00.
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:            "50% Max 15",
		Type:            "percentage",
		Value:           num(5000, -2), // 50%
		Scope:           "subtotal",
		MaximumDiscount: num(1500, -2), // 15.00
		IsActive:        true,
		Priority:        10,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00 -> 50% = 50.00, capped to 15.00
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1500)
}

func TestApply_NonStackableStops(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Two discounts on subtotal. First one (higher priority) is non-stackable.
	createActiveDiscount(t, svc, "High Prio (non-stack)", "percentage", "subtotal", num(1000, -2), 20, false)
	createActiveDiscount(t, svc, "Low Prio (stack)", "fixed_amount", "subtotal", num(500, -2), 10, true)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Only the non-stackable discount should apply: 10% of 100.00 = 10.00.
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1000)
	if len(result.Breakdown) != 1 {
		t.Fatalf("breakdown: got %d, want 1 (non-stackable stops others)", len(result.Breakdown))
	}
	if result.Breakdown[0].DiscountName != "High Prio (non-stack)" {
		t.Errorf("name: got %q, want %q", result.Breakdown[0].DiscountName, "High Prio (non-stack)")
	}
}

func TestApply_StackableAccumulate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Two stackable subtotal discounts.
	createActiveDiscount(t, svc, "10% Stack", "percentage", "subtotal", num(1000, -2), 20, true)
	createActiveDiscount(t, svc, "5 Off Stack", "fixed_amount", "subtotal", num(500, -2), 10, true)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 10% of 100.00 = 10.00 + 5.00 fixed = 15.00
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1500)
	if len(result.Breakdown) != 2 {
		t.Fatalf("breakdown: got %d, want 2", len(result.Breakdown))
	}
}

func TestApply_ScopeOrdering(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Discounts across all three scopes.
	createActiveDiscount(t, svc, "Subtotal 10%", "percentage", "subtotal", num(1000, -2), 10, true)
	createActiveDiscount(t, svc, "Shipping 50%", "percentage", "shipping", num(5000, -2), 10, true)
	createActiveDiscount(t, svc, "Total 2 Off", "fixed_amount", "total", num(200, -2), 10, true)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2), // 100.00
		ShippingFee: num(1000, -2),  // 10.00
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 10% of 100.00 = 10.00 + 50% of 10.00 = 5.00 + 2.00 fixed = 17.00
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1700)
	if len(result.Breakdown) != 3 {
		t.Fatalf("breakdown: got %d, want 3", len(result.Breakdown))
	}
	// Verify scope ordering.
	if result.Breakdown[0].Scope != "subtotal" {
		t.Errorf("breakdown[0] scope: got %q, want %q", result.Breakdown[0].Scope, "subtotal")
	}
	if result.Breakdown[1].Scope != "shipping" {
		t.Errorf("breakdown[1] scope: got %q, want %q", result.Breakdown[1].Scope, "shipping")
	}
	if result.Breakdown[2].Scope != "total" {
		t.Errorf("breakdown[2] scope: got %q, want %q", result.Breakdown[2].Scope, "total")
	}
}

func TestApply_FixedExceedsScope_Capped(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// 50.00 off shipping (but shipping is only 8.50).
	createActiveDiscount(t, svc, "Big Ship Disc", "fixed_amount", "shipping", num(5000, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(850, -2), // 8.50
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Capped at the shipping fee: 8.50 = 850 cents.
	assertCentsEqual(t, "total_discount", result.TotalDiscount, 850)
}

func TestApply_WithCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create a discount that is NOT automatically active (it's only usable via coupon).
	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Coupon-Only",
		Type:     "fixed_amount",
		Value:    num(1000, -2), // 10.00
		Scope:    "subtotal",
		IsActive: true,
		Priority: 5,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "SAVE10",
		DiscountID: d.ID,
		IsActive:   true,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "save10", // lowercase — should be normalized
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 1000)
	if !result.CouponID.Valid {
		t.Error("expected CouponID to be set")
	}
}

func TestApply_InvalidCouponCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "NOSUCH",
	})
	if err != discount.ErrCouponNotFound {
		t.Errorf("expected ErrCouponNotFound, got %v", err)
	}
}

func TestApply_ExpiredCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: true,
	})

	past := time.Now().UTC().Add(-48 * time.Hour)
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "EXPIRED",
		DiscountID: d.ID,
		IsActive:   true,
		EndsAt:     ts(past), // ended 2 days ago
	})

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "EXPIRED",
	})
	if err != discount.ErrCouponExpired {
		t.Errorf("expected ErrCouponExpired, got %v", err)
	}
}

func TestApply_CouponNotYetStarted(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: true,
	})

	future := time.Now().UTC().Add(48 * time.Hour)
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "FUTURE",
		DiscountID: d.ID,
		IsActive:   true,
		StartsAt:   ts(future), // starts in 2 days
	})

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "FUTURE",
	})
	if err != discount.ErrCouponExpired {
		t.Errorf("expected ErrCouponExpired, got %v", err)
	}
}

func TestApply_InactiveCoupon(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: true,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "INACTIVE",
		DiscountID: d.ID,
		IsActive:   false,
	})

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "INACTIVE",
	})
	if err != discount.ErrCouponExpired {
		t.Errorf("expected ErrCouponExpired, got %v", err)
	}
}

func TestApply_CouponUsageLimitReached(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "D", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: true,
	})

	limit := int32(1)
	c, _ := svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "LIMITED",
		DiscountID: d.ID,
		UsageLimit: &limit,
		IsActive:   true,
	})

	// Use it once.
	svc.IncrementCouponUsage(ctx, c.ID)

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "LIMITED",
	})
	if err != discount.ErrCouponUsageLimitReached {
		t.Errorf("expected ErrCouponUsageLimitReached, got %v", err)
	}
}

func TestApply_CouponWithInactiveDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Discount is inactive.
	d, _ := svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name: "Inactive D", Type: "percentage", Value: num(1000, -2), Scope: "subtotal", IsActive: false,
	})
	svc.CreateCoupon(ctx, discount.CreateCouponParams{
		Code:       "DEADCODE",
		DiscountID: d.ID,
		IsActive:   true,
	})

	_, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
		CouponCode:  "DEADCODE",
	})
	if err != discount.ErrCouponExpired {
		t.Errorf("expected ErrCouponExpired (inactive discount), got %v", err)
	}
}

func TestApply_PrimaryDiscountID(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createActiveDiscount(t, svc, "Primary", "fixed_amount", "subtotal", num(100, -2), 10, false)

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !result.DiscountID.Valid {
		t.Fatal("expected DiscountID to be set")
	}
	if result.DiscountID.Bytes != id {
		t.Errorf("DiscountID: got %s, want %s", result.DiscountID.Bytes, id)
	}
}

func TestApply_InactiveDiscountNotApplied(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Create an inactive discount — should not be picked up by ListActiveDiscounts.
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Inactive",
		Type:     "percentage",
		Value:    num(5000, -2),
		Scope:    "subtotal",
		IsActive: false,
		Priority: 10,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 0)
}

func TestApply_DateFilteredDiscount(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	past := time.Now().UTC().Add(-48 * time.Hour)
	// Discount that ended 2 days ago.
	svc.CreateDiscount(ctx, discount.CreateDiscountParams{
		Name:     "Ended",
		Type:     "percentage",
		Value:    num(5000, -2),
		Scope:    "subtotal",
		EndsAt:   ts(past),
		IsActive: true,
		Priority: 10,
	})

	result, err := svc.Apply(ctx, discount.ApplyParams{
		Subtotal:    num(10000, -2),
		ShippingFee: num(0, -2),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertCentsEqual(t, "total_discount", result.TotalDiscount, 0)
}

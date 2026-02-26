package report_test

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/services/report"
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

func newService() *report.Service {
	return report.NewService(testDB.Pool, nil)
}

// insertPaidOrder inserts an order with payment_status='paid' at the given time
// and returns its ID. The amounts are subtotal, vatTotal, discountAmount, total.
func insertPaidOrder(t *testing.T, createdAt time.Time, subtotal, vatTotal, discountAmount, total float64, vatCountryCode *string, vatReverseCharge bool, vatNumber, vatCompanyName *string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	ctx := context.Background()

	var countryCode interface{} = nil
	if vatCountryCode != nil {
		countryCode = *vatCountryCode
	}

	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO orders (
			id, email, billing_address, shipping_address,
			subtotal, vat_total, discount_amount, total,
			payment_status, vat_country_code, vat_reverse_charge,
			vat_number, vat_company_name,
			created_at, updated_at
		) VALUES (
			$1, 'test@example.com', '{}', '{}',
			$2, $3, $4, $5,
			'paid', $6, $7,
			$8, $9,
			$10, $10
		)`,
		id, subtotal, vatTotal, discountAmount, total,
		countryCode, vatReverseCharge,
		vatNumber, vatCompanyName,
		createdAt,
	)
	if err != nil {
		t.Fatalf("inserting paid order: %v", err)
	}
	return id
}

// insertOrderItem inserts an order item for the given order.
func insertOrderItem(t *testing.T, orderID uuid.UUID, productName string, qty int, unitPrice, totalPrice, vatRate, vatAmount, netUnit, grossUnit float64, vatRateType string) {
	t.Helper()
	ctx := context.Background()

	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO order_items (
			id, order_id, product_name, quantity,
			unit_price, total_price,
			vat_rate, vat_rate_type, vat_amount,
			price_includes_vat, net_unit_price, gross_unit_price
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9,
			true, $10, $11
		)`,
		uuid.New(), orderID, productName, qty,
		unitPrice, totalPrice,
		vatRate, vatRateType, vatAmount,
		netUnit, grossUnit,
	)
	if err != nil {
		t.Fatalf("inserting order item: %v", err)
	}
}


// --------------------------------------------------------------------------
// Sales Report
// --------------------------------------------------------------------------

func TestGetSalesReport_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetSalesReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetSalesReport: %v", err)
	}
	if rpt.Summary.OrderCount != 0 {
		t.Errorf("order count: got %d, want 0", rpt.Summary.OrderCount)
	}
	if len(rpt.DailyData) != 0 {
		t.Errorf("daily data: got %d rows, want 0", len(rpt.DailyData))
	}
}

func TestGetSalesReport_WithOrders(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day1 := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 2, 11, 14, 0, 0, 0, time.UTC)

	es := strPtr("ES")

	// Day 1: two orders.
	insertPaidOrder(t, day1, 100.00, 21.00, 0, 121.00, es, false, nil, nil)
	insertPaidOrder(t, day1, 50.00, 10.50, 5.00, 55.50, es, false, nil, nil)

	// Day 2: one order.
	insertPaidOrder(t, day2, 200.00, 42.00, 10.00, 232.00, es, false, nil, nil)

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetSalesReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetSalesReport: %v", err)
	}

	// Summary assertions.
	if rpt.Summary.OrderCount != 3 {
		t.Errorf("order count: got %d, want 3", rpt.Summary.OrderCount)
	}

	// Net revenue: 100 + 50 + 200 = 350.
	f, _ := rpt.Summary.NetRevenue.Float64Value()
	if math.Abs(f.Float64-350.00) > 0.01 {
		t.Errorf("net revenue: got %.2f, want 350.00", f.Float64)
	}

	// Gross revenue: 121 + 55.50 + 232 = 408.50.
	g, _ := rpt.Summary.GrossRevenue.Float64Value()
	if math.Abs(g.Float64-408.50) > 0.01 {
		t.Errorf("gross revenue: got %.2f, want 408.50", g.Float64)
	}

	// VAT collected: 21 + 10.50 + 42 = 73.50.
	v, _ := rpt.Summary.VATCollected.Float64Value()
	if math.Abs(v.Float64-73.50) > 0.01 {
		t.Errorf("vat collected: got %.2f, want 73.50", v.Float64)
	}

	// Total discounts: 0 + 5 + 10 = 15.
	d, _ := rpt.Summary.TotalDiscounts.Float64Value()
	if math.Abs(d.Float64-15.00) > 0.01 {
		t.Errorf("total discounts: got %.2f, want 15.00", d.Float64)
	}

	// Daily data: 2 rows (two distinct dates).
	if len(rpt.DailyData) != 2 {
		t.Errorf("daily data rows: got %d, want 2", len(rpt.DailyData))
	}
}

func TestGetSalesReport_OnlyPaidOrders(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	es := strPtr("ES")

	// Insert a paid order.
	insertPaidOrder(t, day, 100.00, 21.00, 0, 121.00, es, false, nil, nil)

	// Insert an unpaid order directly.
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO orders (
			id, email, billing_address, shipping_address,
			subtotal, vat_total, discount_amount, total,
			payment_status, vat_country_code, vat_reverse_charge,
			created_at, updated_at
		) VALUES (
			$1, 'unpaid@test.com', '{}', '{}',
			999.00, 99.00, 0, 1098.00,
			'unpaid', $2, false,
			$3, $3
		)`, uuid.New(), "ES", day)
	if err != nil {
		t.Fatalf("inserting unpaid order: %v", err)
	}

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetSalesReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetSalesReport: %v", err)
	}

	// Only the paid order should be counted.
	if rpt.Summary.OrderCount != 1 {
		t.Errorf("order count: got %d, want 1 (only paid)", rpt.Summary.OrderCount)
	}
}

func TestGetSalesReport_DateFiltering(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()

	es := strPtr("ES")

	// Order inside the range.
	insertPaidOrder(t, time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC),
		100.00, 21.00, 0, 121.00, es, false, nil, nil)

	// Order outside the range (January).
	insertPaidOrder(t, time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		500.00, 50.00, 0, 550.00, es, false, nil, nil)

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetSalesReport(context.Background(), from, to)
	if err != nil {
		t.Fatalf("GetSalesReport: %v", err)
	}

	if rpt.Summary.OrderCount != 1 {
		t.Errorf("order count: got %d, want 1 (only Feb)", rpt.Summary.OrderCount)
	}
}

// --------------------------------------------------------------------------
// VAT Report
// --------------------------------------------------------------------------

func TestGetVATReport_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetVATReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetVATReport: %v", err)
	}
	if len(rpt.CountryBreakdown) != 0 {
		t.Errorf("country breakdown: got %d rows, want 0", len(rpt.CountryBreakdown))
	}
	if rpt.ReverseCharge.OrderCount != 0 {
		t.Errorf("reverse charge count: got %d, want 0", rpt.ReverseCharge.OrderCount)
	}
}

func TestGetVATReport_CountryBreakdown(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	es := strPtr("ES")
	de := strPtr("DE")

	// Spanish order with standard VAT.
	orderES := insertPaidOrder(t, day, 100.00, 21.00, 0, 121.00, es, false, nil, nil)
	insertOrderItem(t, orderES, "Widget", 2, 50.00, 100.00, 21.00, 21.00, 41.32, 50.00, "standard")

	// German order with reduced VAT.
	orderDE := insertPaidOrder(t, day, 80.00, 5.60, 0, 85.60, de, false, nil, nil)
	insertOrderItem(t, orderDE, "Book", 1, 80.00, 80.00, 7.00, 5.60, 74.77, 80.00, "reduced")

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetVATReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetVATReport: %v", err)
	}

	// Should have two breakdown rows (DE reduced + ES standard).
	if len(rpt.CountryBreakdown) != 2 {
		t.Fatalf("country breakdown rows: got %d, want 2", len(rpt.CountryBreakdown))
	}

	// Find the DE entry.
	var deLine, esLine *report.VATCountryLine
	for i, line := range rpt.CountryBreakdown {
		switch line.CountryCode {
		case "DE":
			deLine = &rpt.CountryBreakdown[i]
		case "ES":
			esLine = &rpt.CountryBreakdown[i]
		}
	}

	if deLine == nil {
		t.Fatal("missing DE line")
	}
	if deLine.RateType != "reduced" {
		t.Errorf("DE rate type: got %q, want %q", deLine.RateType, "reduced")
	}

	if esLine == nil {
		t.Fatal("missing ES line")
	}
	if esLine.RateType != "standard" {
		t.Errorf("ES rate type: got %q, want %q", esLine.RateType, "standard")
	}
}

func TestGetVATReport_ReverseCharge(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	de := strPtr("DE")
	vatNum := strPtr("DE123456789")
	company := strPtr("Acme GmbH")

	// B2B reverse charge order.
	insertPaidOrder(t, day, 500.00, 0, 0, 500.00, de, true, vatNum, company)

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetVATReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetVATReport: %v", err)
	}

	// Reverse charge summary.
	if rpt.ReverseCharge.OrderCount != 1 {
		t.Errorf("reverse charge count: got %d, want 1", rpt.ReverseCharge.OrderCount)
	}
	rcNet, _ := rpt.ReverseCharge.TotalNet.Float64Value()
	if math.Abs(rcNet.Float64-500.00) > 0.01 {
		t.Errorf("reverse charge net: got %.2f, want 500.00", rcNet.Float64)
	}

	// Reverse charge orders list.
	if len(rpt.ReverseChargeOrders) != 1 {
		t.Fatalf("reverse charge orders: got %d, want 1", len(rpt.ReverseChargeOrders))
	}
	rco := rpt.ReverseChargeOrders[0]
	if rco.VATNumber == nil || *rco.VATNumber != "DE123456789" {
		t.Errorf("VAT number: got %v, want DE123456789", rco.VATNumber)
	}
	if rco.CompanyName == nil || *rco.CompanyName != "Acme GmbH" {
		t.Errorf("company: got %v, want Acme GmbH", rco.CompanyName)
	}
}

func TestGetVATReport_ReverseChargeExcludedFromBreakdown(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	de := strPtr("DE")
	vatNum := strPtr("DE123456789")

	// Only a reverse-charge order (no regular B2C orders).
	orderRC := insertPaidOrder(t, day, 500.00, 0, 0, 500.00, de, true, vatNum, nil)
	insertOrderItem(t, orderRC, "Machine", 1, 500.00, 500.00, 0, 0, 500.00, 500.00, "standard")

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	rpt, err := svc.GetVATReport(ctx, from, to)
	if err != nil {
		t.Fatalf("GetVATReport: %v", err)
	}

	// Country breakdown should be empty — reverse charge orders are excluded.
	if len(rpt.CountryBreakdown) != 0 {
		t.Errorf("country breakdown should exclude RC orders, got %d rows", len(rpt.CountryBreakdown))
	}
}

// --------------------------------------------------------------------------
// Top Products
// --------------------------------------------------------------------------

func TestGetTopProducts_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	products, err := svc.GetTopProducts(ctx, from, to, 10)
	if err != nil {
		t.Fatalf("GetTopProducts: %v", err)
	}
	if len(products) != 0 {
		t.Errorf("count: got %d, want 0", len(products))
	}
}

func TestGetTopProducts_Ranked(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	es := strPtr("ES")

	order := insertPaidOrder(t, day, 300.00, 63.00, 0, 363.00, es, false, nil, nil)

	// "Bag" has higher total revenue than "Belt".
	insertOrderItem(t, order, "Leather Bag", 2, 100.00, 200.00, 21.00, 42.00, 82.64, 100.00, "standard")
	insertOrderItem(t, order, "Belt", 1, 100.00, 100.00, 21.00, 21.00, 82.64, 100.00, "standard")

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	products, err := svc.GetTopProducts(ctx, from, to, 10)
	if err != nil {
		t.Fatalf("GetTopProducts: %v", err)
	}

	if len(products) != 2 {
		t.Fatalf("count: got %d, want 2", len(products))
	}

	// First product should be "Leather Bag" (higher revenue).
	if products[0].ProductName != "Leather Bag" {
		t.Errorf("top product: got %q, want %q", products[0].ProductName, "Leather Bag")
	}
	if products[0].TotalQuantity != 2 {
		t.Errorf("top product qty: got %d, want 2", products[0].TotalQuantity)
	}
}

func TestGetTopProducts_LimitRespected(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	day := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	es := strPtr("ES")

	order := insertPaidOrder(t, day, 300.00, 0, 0, 300.00, es, false, nil, nil)

	for i := 0; i < 5; i++ {
		insertOrderItem(t, order, fmt.Sprintf("Product %d", i), 1, 60.00, 60.00, 0, 0, 60.00, 60.00, "standard")
	}

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	products, err := svc.GetTopProducts(ctx, from, to, 3)
	if err != nil {
		t.Fatalf("GetTopProducts: %v", err)
	}

	if len(products) != 3 {
		t.Errorf("count: got %d, want 3 (limited)", len(products))
	}
}

// --------------------------------------------------------------------------
// Prediction
// --------------------------------------------------------------------------

func TestPredictSales_NoData(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	results, err := svc.PredictSales(ctx, 7)
	if err != nil {
		t.Fatalf("PredictSales: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("count: got %d, want 7", len(results))
	}

	// With no data, predictions should be zero.
	for _, r := range results {
		if r.PredictedGross != 0 {
			t.Errorf("predicted gross: got %.2f, want 0 (no data)", r.PredictedGross)
		}
		if r.Method != "wma_only" {
			t.Errorf("method: got %q, want %q (no previous year)", r.Method, "wma_only")
		}
	}
}

func TestPredictSales_InvalidDays(t *testing.T) {
	svc := newService()
	ctx := context.Background()

	_, err := svc.PredictSales(ctx, 0)
	if err == nil {
		t.Error("expected error for numDays=0")
	}

	_, err = svc.PredictSales(ctx, -1)
	if err == nil {
		t.Error("expected error for numDays=-1")
	}
}

func TestPredictSales_WithRecentData(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	es := strPtr("ES")

	// Insert orders for the last 7 days so there's recent data.
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for i := 1; i <= 7; i++ {
		orderDate := today.AddDate(0, 0, -i).Add(10 * time.Hour)
		insertPaidOrder(t, orderDate, 100.00, 21.00, 0, 121.00, es, false, nil, nil)
	}

	results, err := svc.PredictSales(ctx, 3)
	if err != nil {
		t.Fatalf("PredictSales: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("count: got %d, want 3", len(results))
	}

	// With recent data, predictions should be positive.
	for _, r := range results {
		if r.PredictedGross <= 0 {
			t.Errorf("date %s: predicted gross should be positive, got %.2f", r.Date.Format("2006-01-02"), r.PredictedGross)
		}
		// No previous year data, so method should be wma_only.
		if r.Method != "wma_only" {
			t.Errorf("method: got %q, want %q", r.Method, "wma_only")
		}
		if r.Confidence <= 0 {
			t.Errorf("confidence should be > 0, got %.2f", r.Confidence)
		}
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func strPtr(s string) *string {
	return &s
}

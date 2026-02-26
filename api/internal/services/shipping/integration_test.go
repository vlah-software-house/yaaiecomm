package shipping_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/forgecommerce/api/internal/services/shipping"
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

func newService() *shipping.Service {
	return shipping.NewService(testDB.Pool, nil)
}

// setupCountry seeds essentials and enables a shipping country.
func setupCountry(t *testing.T, code string) {
	t.Helper()
	testDB.SeedEssentials(t)
	testDB.FixtureShippingCountry(t, code)
}

// resetConfig restores shipping config to a known default state.
func resetConfig(t *testing.T, svc *shipping.Service) {
	t.Helper()
	_, err := svc.UpdateConfig(context.Background(), shipping.UpdateConfigParams{
		Enabled:               true,
		CalculationMethod:     "fixed",
		FixedFee:              decimal.NewFromFloat(5.00),
		WeightRates:           json.RawMessage(`[]`),
		SizeRates:             json.RawMessage(`[]`),
		FreeShippingThreshold: decimal.Zero,
		DefaultCurrency:       "EUR",
	})
	if err != nil {
		t.Fatalf("resetting shipping config: %v", err)
	}
}

// --------------------------------------------------------------------------
// Config CRUD
// --------------------------------------------------------------------------

func TestGetConfig(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// The migration seeds a default row.
	config, err := svc.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if config.CalculationMethod != "fixed" {
		t.Errorf("method: got %q, want %q", config.CalculationMethod, "fixed")
	}
}

func TestUpdateConfig(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	updated, err := svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:               true,
		CalculationMethod:     "weight_based",
		FixedFee:              decimal.NewFromFloat(0),
		WeightRates:           json.RawMessage(`[{"min_weight_g":0,"max_weight_g":1000,"fee":"5.00"}]`),
		SizeRates:             json.RawMessage(`[]`),
		FreeShippingThreshold: decimal.NewFromFloat(100.00),
		DefaultCurrency:       "EUR",
	})
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	if updated.CalculationMethod != "weight_based" {
		t.Errorf("method: got %q, want %q", updated.CalculationMethod, "weight_based")
	}
	if !updated.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestUpdateConfig_Disable(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	updated, err := svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:           false,
		CalculationMethod: "fixed",
		FixedFee:          decimal.Zero,
		WeightRates:       json.RawMessage(`[]`),
		SizeRates:         json.RawMessage(`[]`),
		DefaultCurrency:   "EUR",
	})
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if updated.Enabled {
		t.Error("expected enabled=false")
	}
}

// --------------------------------------------------------------------------
// Zone CRUD
// --------------------------------------------------------------------------

func TestCreateZone(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	zone, err := svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name:              "Iberian Peninsula",
		Countries:         []string{"ES", "PT"},
		CalculationMethod: "fixed",
		Rates:             json.RawMessage(`{"fixed_fee":"3.50"}`),
		Position:          1,
	})
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}
	if zone.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if zone.Name != "Iberian Peninsula" {
		t.Errorf("name: got %q, want %q", zone.Name, "Iberian Peninsula")
	}
	if len(zone.Countries) != 2 {
		t.Errorf("countries: got %d, want 2", len(zone.Countries))
	}
}

func TestGetZone(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name:              "Central Europe",
		Countries:         []string{"DE", "FR"},
		CalculationMethod: "weight_based",
		Rates:             json.RawMessage(`[{"min_weight_g":0,"max_weight_g":0,"fee":"10.00"}]`),
		Position:          1,
	})

	got, err := svc.GetZone(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetZone: %v", err)
	}
	if got.Name != "Central Europe" {
		t.Errorf("name: got %q, want %q", got.Name, "Central Europe")
	}
}

func TestGetZone_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetZone(ctx, uuid.New())
	if err != shipping.ErrZoneNotFound {
		t.Errorf("expected ErrZoneNotFound, got %v", err)
	}
}

func TestListZones(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name: "Z1", Countries: []string{"ES"}, CalculationMethod: "fixed",
		Rates: json.RawMessage(`{}`), Position: 2,
	})
	svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name: "Z2", Countries: []string{"DE"}, CalculationMethod: "fixed",
		Rates: json.RawMessage(`{}`), Position: 1,
	})

	zones, err := svc.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) != 2 {
		t.Errorf("count: got %d, want 2", len(zones))
	}
	// Ordered by position.
	if zones[0].Name != "Z2" {
		t.Errorf("first zone: got %q, want %q (lower position)", zones[0].Name, "Z2")
	}
}

func TestListZones_Empty(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	zones, err := svc.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) != 0 {
		t.Errorf("count: got %d, want 0", len(zones))
	}
}

func TestUpdateZone(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name: "Original", Countries: []string{"ES"}, CalculationMethod: "fixed",
		Rates: json.RawMessage(`{}`), Position: 1,
	})

	updated, err := svc.UpdateZone(ctx, created.ID, shipping.UpdateZoneParams{
		Name:              "Renamed",
		Countries:         []string{"ES", "PT"},
		CalculationMethod: "weight_based",
		Rates:             json.RawMessage(`[{"min_weight_g":0,"max_weight_g":0,"fee":"8.00"}]`),
		Position:          2,
	})
	if err != nil {
		t.Fatalf("UpdateZone: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("name: got %q, want %q", updated.Name, "Renamed")
	}
	if len(updated.Countries) != 2 {
		t.Errorf("countries: got %d, want 2", len(updated.Countries))
	}
	if updated.CalculationMethod != "weight_based" {
		t.Errorf("method: got %q, want %q", updated.CalculationMethod, "weight_based")
	}
}

func TestUpdateZone_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateZone(ctx, uuid.New(), shipping.UpdateZoneParams{
		Name: "Nope", Countries: []string{"ES"}, CalculationMethod: "fixed",
		Rates: json.RawMessage(`{}`),
	})
	if err != shipping.ErrZoneNotFound {
		t.Errorf("expected ErrZoneNotFound, got %v", err)
	}
}

func TestDeleteZone(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created, _ := svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name: "To Delete", Countries: []string{"ES"}, CalculationMethod: "fixed",
		Rates: json.RawMessage(`{}`), Position: 1,
	})

	err := svc.DeleteZone(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteZone: %v", err)
	}

	_, err = svc.GetZone(ctx, created.ID)
	if err != shipping.ErrZoneNotFound {
		t.Errorf("expected ErrZoneNotFound after delete, got %v", err)
	}
}

func TestDeleteZone_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.DeleteZone(ctx, uuid.New())
	if err != shipping.ErrZoneNotFound {
		t.Errorf("expected ErrZoneNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Calculate
// --------------------------------------------------------------------------

func TestCalculate_Fixed(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "ES")
	resetConfig(t, svc) // fixed fee = 5.00

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if !result.BaseFee.Equal(decimal.NewFromFloat(5.00)) {
		t.Errorf("base_fee: got %s, want 5.00", result.BaseFee.String())
	}
	if result.Method != "fixed" {
		t.Errorf("method: got %q, want %q", result.Method, "fixed")
	}
	if !result.TotalFee.Equal(decimal.NewFromFloat(5.00)) {
		t.Errorf("total_fee: got %s, want 5.00", result.TotalFee.String())
	}
}

func TestCalculate_ShippingDisabled(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// Disable shipping.
	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:           false,
		CalculationMethod: "fixed",
		FixedFee:          decimal.NewFromFloat(5.00),
		WeightRates:       json.RawMessage(`[]`),
		SizeRates:         json.RawMessage(`[]`),
		DefaultCurrency:   "EUR",
	})

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	// When shipping is disabled, zero fee is returned (no country check).
	if !result.TotalFee.Equal(decimal.Zero) {
		t.Errorf("total_fee: got %s, want 0", result.TotalFee.String())
	}
}

func TestCalculate_CountryNotEnabled(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	// No countries enabled (Truncate clears store_shipping_countries).
	// But shipping_config is enabled by default from migration.
	resetConfig(t, svc)

	_, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != shipping.ErrCountryNotEnabled {
		t.Errorf("expected ErrCountryNotEnabled, got %v", err)
	}
}

func TestCalculate_WithExtraFees(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "ES")
	resetConfig(t, svc) // fixed 5.00

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(50.00),
		Items: []shipping.ShippingItem{
			{ProductExtraFee: decimal.NewFromFloat(2.00), Quantity: 2}, // 4.00
			{ProductExtraFee: decimal.NewFromFloat(1.50), Quantity: 1}, // 1.50
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if !result.BaseFee.Equal(decimal.NewFromFloat(5.00)) {
		t.Errorf("base_fee: got %s, want 5.00", result.BaseFee.String())
	}
	if !result.ExtraFees.Equal(decimal.NewFromFloat(5.50)) {
		t.Errorf("extra_fees: got %s, want 5.50", result.ExtraFees.String())
	}
	if !result.TotalFee.Equal(decimal.NewFromFloat(10.50)) {
		t.Errorf("total_fee: got %s, want 10.50", result.TotalFee.String())
	}
}

func TestCalculate_FreeShippingThreshold(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "ES")
	// Set free shipping threshold to 50.00.
	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:               true,
		CalculationMethod:     "fixed",
		FixedFee:              decimal.NewFromFloat(8.00),
		WeightRates:           json.RawMessage(`[]`),
		SizeRates:             json.RawMessage(`[]`),
		FreeShippingThreshold: decimal.NewFromFloat(50.00),
		DefaultCurrency:       "EUR",
	})

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(75.00), // above threshold
		Items: []shipping.ShippingItem{
			{ProductExtraFee: decimal.NewFromFloat(1.00), Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if !result.FreeShipping {
		t.Error("expected FreeShipping=true")
	}
	if !result.BaseFee.Equal(decimal.Zero) {
		t.Errorf("base_fee: got %s, want 0 (free shipping)", result.BaseFee.String())
	}
	// Extra fees still apply even with free shipping.
	if !result.ExtraFees.Equal(decimal.NewFromFloat(1.00)) {
		t.Errorf("extra_fees: got %s, want 1.00", result.ExtraFees.String())
	}
	if !result.TotalFee.Equal(decimal.NewFromFloat(1.00)) {
		t.Errorf("total_fee: got %s, want 1.00 (extras only)", result.TotalFee.String())
	}
}

func TestCalculate_FreeShippingThreshold_BelowThreshold(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "ES")
	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:               true,
		CalculationMethod:     "fixed",
		FixedFee:              decimal.NewFromFloat(8.00),
		WeightRates:           json.RawMessage(`[]`),
		SizeRates:             json.RawMessage(`[]`),
		FreeShippingThreshold: decimal.NewFromFloat(100.00),
		DefaultCurrency:       "EUR",
	})

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 500,
		Subtotal:     decimal.NewFromFloat(75.00), // below 100 threshold
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if result.FreeShipping {
		t.Error("expected FreeShipping=false (below threshold)")
	}
	if !result.BaseFee.Equal(decimal.NewFromFloat(8.00)) {
		t.Errorf("base_fee: got %s, want 8.00", result.BaseFee.String())
	}
}

func TestCalculate_WeightBased(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "DE")

	brackets := []shipping.WeightBracket{
		{MinWeightG: 0, MaxWeightG: 500, Fee: decimal.NewFromFloat(5.00)},
		{MinWeightG: 501, MaxWeightG: 2000, Fee: decimal.NewFromFloat(8.50)},
		{MinWeightG: 2001, MaxWeightG: 0, Fee: decimal.NewFromFloat(15.00)},
	}
	bracketsJSON, _ := json.Marshal(brackets)

	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:           true,
		CalculationMethod: "weight_based",
		WeightRates:       bracketsJSON,
		SizeRates:         json.RawMessage(`[]`),
		DefaultCurrency:   "EUR",
	})

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "DE",
		TotalWeightG: 750,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if result.Method != "weight_based" {
		t.Errorf("method: got %q, want %q", result.Method, "weight_based")
	}
	if !result.BaseFee.Equal(decimal.NewFromFloat(8.50)) {
		t.Errorf("base_fee: got %s, want 8.50", result.BaseFee.String())
	}
}

func TestCalculate_SizeBased(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "FR")

	sizeRate := shipping.SizeRate{
		BaseFee:  decimal.NewFromFloat(3.00),
		PerKgFee: decimal.NewFromFloat(1.50),
		MinFee:   decimal.NewFromFloat(5.00),
	}
	sizeJSON, _ := json.Marshal(sizeRate)

	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:           true,
		CalculationMethod: "size_based",
		WeightRates:       json.RawMessage(`[]`),
		SizeRates:         sizeJSON,
		DefaultCurrency:   "EUR",
	})

	// 2500g rounds up to 3kg: 3.00 + 1.50 * 3 = 7.50
	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "FR",
		TotalWeightG: 2500,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if result.Method != "size_based" {
		t.Errorf("method: got %q, want %q", result.Method, "size_based")
	}
	if !result.BaseFee.Equal(decimal.NewFromFloat(7.50)) {
		t.Errorf("base_fee: got %s, want 7.50", result.BaseFee.String())
	}
}

func TestCalculate_WithZoneOverride(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	setupCountry(t, "ES")

	// Global config is weight_based, but zone overrides to fixed.
	svc.UpdateConfig(ctx, shipping.UpdateConfigParams{
		Enabled:           true,
		CalculationMethod: "weight_based",
		FixedFee:          decimal.NewFromFloat(10.00),
		WeightRates:       json.RawMessage(`[{"min_weight_g":0,"max_weight_g":0,"fee":"20.00"}]`),
		SizeRates:         json.RawMessage(`[]`),
		DefaultCurrency:   "EUR",
	})

	// Create a zone for ES with fixed rate override.
	zone, err := svc.CreateZone(ctx, shipping.CreateZoneParams{
		Name:              "Iberian",
		Countries:         []string{"ES"},
		CalculationMethod: "fixed",
		Rates:             json.RawMessage(`{"fixed_fee":"3.50"}`),
		Position:          1,
	})
	if err != nil {
		t.Fatalf("CreateZone: %v", err)
	}

	// Link the zone to the country. The FixtureShippingCountry only enables;
	// we need to set shipping_zone_id on the store_shipping_countries row.
	_, err = testDB.Pool.Exec(context.Background(),
		`UPDATE store_shipping_countries SET shipping_zone_id = $1 WHERE country_code = $2`,
		zone.ID, "ES")
	if err != nil {
		t.Fatalf("linking zone to country: %v", err)
	}

	result, err := svc.Calculate(ctx, shipping.CalculateParams{
		CountryCode:  "ES",
		TotalWeightG: 5000,
		Subtotal:     decimal.NewFromFloat(50.00),
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	// Zone overrides: should use the zone's fixed fee (3.50), not global weight_based (20.00).
	if result.Method != "fixed" {
		t.Errorf("method: got %q, want %q (zone override)", result.Method, "fixed")
	}
	if !result.BaseFee.Equal(decimal.NewFromFloat(3.50)) {
		t.Errorf("base_fee: got %s, want 3.50 (zone override)", result.BaseFee.String())
	}
}

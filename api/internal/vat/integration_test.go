package vat

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/forgecommerce/api/internal/config"
	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/testutil"
)

// ---------------------------------------------------------------------------
// TestMain -- shared PostgreSQL container for all tests in this package
// ---------------------------------------------------------------------------

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	tdb, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer tdb.Close()
	testDB = tdb

	code = m.Run()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

func setupStoreSettings(t *testing.T, vatEnabled bool, countryCode string, pricesIncludeVAT bool, defaultCategory string, b2bReverseCharge bool) {
	t.Helper()
	ctx := context.Background()
	q := db.New(testDB.Pool)
	err := q.UpdateStoreVATSettings(ctx, db.UpdateStoreVATSettingsParams{
		VatEnabled:                 vatEnabled,
		VatNumber:                  strPtr("ES12345678A"),
		VatCountryCode:             strPtr(countryCode),
		VatPricesIncludeVat:        pricesIncludeVAT,
		VatDefaultCategory:         defaultCategory,
		VatB2bReverseChargeEnabled: b2bReverseCharge,
		UpdatedAt:                  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("updating store VAT settings: %v", err)
	}
}

// lookupVATCategoryID returns the UUID of a VAT category by name.
// The categories are seeded by migration 004 so they always exist.
func lookupVATCategoryID(t *testing.T, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	q := db.New(testDB.Pool)
	cat, err := q.GetVATCategoryByName(ctx, name)
	if err != nil {
		t.Fatalf("looking up VAT category %q: %v", name, err)
	}
	return cat.ID
}

func insertVATRate(t *testing.T, countryCode, rateType string, rate float64, source string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, source, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), countryCode, rateType, rate,
		fmt.Sprintf("%s rate for %s", rateType, countryCode),
		today, source, now)
	if err != nil {
		t.Fatalf("inserting VAT rate %s/%s=%f: %v", countryCode, rateType, rate, err)
	}
}

// insertVATRateWithDate inserts a rate with an explicit valid_from date.
func insertVATRateWithDate(t *testing.T, countryCode, rateType string, rate float64, source string, validFrom time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, source, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, uuid.New(), countryCode, rateType, rate,
		fmt.Sprintf("%s rate for %s", rateType, countryCode),
		validFrom, source, time.Now().UTC())
	if err != nil {
		t.Fatalf("inserting VAT rate %s/%s=%f: %v", countryCode, rateType, rate, err)
	}
}

func insertProduct(t *testing.T) uuid.UUID {
	t.Helper()
	product := testDB.FixtureProduct(t, "Test Product", "test-product-"+uuid.New().String()[:8])
	return product.ID
}

func insertProductVATOverride(t *testing.T, productID uuid.UUID, countryCode string, categoryID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	q := db.New(testDB.Pool)
	_, err := q.UpsertProductVATOverride(ctx, db.UpsertProductVATOverrideParams{
		ID:            uuid.New(),
		ProductID:     productID,
		CountryCode:   countryCode,
		VatCategoryID: categoryID,
		Notes:         strPtr("test override"),
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("upserting product VAT override: %v", err)
	}
}

func insertVIESCache(t *testing.T, vatNumber string, isValid bool, companyName string, expiresAt time.Time) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := testDB.Pool.Exec(ctx, `
		INSERT INTO vies_validation_cache (vat_number, is_valid, company_name, consultation_number, validated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (vat_number) DO UPDATE SET
			is_valid = EXCLUDED.is_valid, company_name = EXCLUDED.company_name,
			validated_at = EXCLUDED.validated_at, expires_at = EXCLUDED.expires_at
	`, vatNumber, isValid, companyName, "FC-TEST0001", now, expiresAt)
	if err != nil {
		t.Fatalf("inserting VIES cache for %s: %v", vatNumber, err)
	}
}

// newIntegrationRateCache creates a rate cache loaded with standard test rates
// for DE (19% std, 7% reduced), ES (21% std, 10% reduced), FR (20% std, 5.5% reduced).
func newIntegrationRateCache() *RateCache {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "DE", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(7.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "ES", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(10.0)},
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
		{CountryCode: "FR", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(5.5)},
	})
	return cache
}

// callVIESWithURL is a test helper that mirrors VIESClient.callVIES but
// allows overriding the endpoint URL for mock server testing.
func callVIESWithURL(ctx context.Context, c *VIESClient, url, countryCode, number string) (VIESResult, error) {
	soapBody := fmt.Sprintf(viesSOAPEnvelope, countryCode, number)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(soapBody))
	if err != nil {
		return VIESResult{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "")

	resp, err := c.client.Do(req)
	if err != nil {
		return VIESResult{}, fmt.Errorf("calling VIES: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VIESResult{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return VIESResult{}, fmt.Errorf("VIES HTTP %d: %s", resp.StatusCode, string(body))
	}

	var soapResp viesSOAPResponse
	if err := xml.Unmarshal(body, &soapResp); err != nil {
		return VIESResult{}, fmt.Errorf("parsing XML: %w", err)
	}

	data := soapResp.Body.CheckVatResponse
	return VIESResult{
		Valid:          data.Valid,
		CompanyName:    strings.TrimSpace(data.Name),
		CompanyAddress: strings.TrimSpace(data.Address),
		CountryCode:    data.CountryCode,
		VATNumber:      countryCode + data.VATNumber,
	}, nil
}

// Note: VAT categories are seeded by migration 004_vat_categories.up.sql.
// Use lookupVATCategoryID(t, "standard") etc. to get the actual UUIDs.

// ===========================================================================
// VATService integration tests
// ===========================================================================

func TestVATService_CalculateForProduct_VATDisabled(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, false, "ES", true, "standard", true)

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(121.00),
		DestinationCountry: "DE",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExemptReason != ExemptReasonDisabled {
		t.Errorf("expected exempt reason %q, got %q", ExemptReasonDisabled, result.ExemptReason)
	}
	if !result.Amount.IsZero() {
		t.Errorf("expected zero VAT, got %s", result.Amount)
	}
	if !result.Rate.IsZero() {
		t.Errorf("expected zero rate, got %s", result.Rate)
	}
}

func TestVATService_CalculateForProduct_StandardRate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(100.00),
		DestinationCountry: "DE",
		Quantity:           2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DE standard rate = 19%, net price = 100, VAT = 19.
	if !result.Rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("rate: want 19.0, got %s", result.Rate)
	}
	if result.RateType != RateTypeStandard {
		t.Errorf("rate type: want %q, got %q", RateTypeStandard, result.RateType)
	}
	if !result.Amount.Equal(decimal.NewFromFloat(19.00)) {
		t.Errorf("unit VAT: want 19.00, got %s", result.Amount)
	}
	if !result.NetPrice.Equal(decimal.NewFromFloat(100.00)) {
		t.Errorf("net: want 100.00, got %s", result.NetPrice)
	}
	if !result.GrossPrice.Equal(decimal.NewFromFloat(119.00)) {
		t.Errorf("gross: want 119.00, got %s", result.GrossPrice)
	}
	// Line totals for qty=2.
	if !result.LineNetTotal.Equal(decimal.NewFromFloat(200.00)) {
		t.Errorf("line net: want 200.00, got %s", result.LineNetTotal)
	}
	if !result.LineVATTotal.Equal(decimal.NewFromFloat(38.00)) {
		t.Errorf("line VAT: want 38.00, got %s", result.LineVATTotal)
	}
	if !result.LineGrossTotal.Equal(decimal.NewFromFloat(238.00)) {
		t.Errorf("line gross: want 238.00, got %s", result.LineGrossTotal)
	}
}

func TestVATService_CalculateForProduct_ReducedRate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	// Product-level category set to reduced (seeded by migration).
	catID := lookupVATCategoryID(t, "reduced")
	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:            productID,
		ProductVATCategoryID: &catID,
		Price:                decimal.NewFromFloat(100.00),
		DestinationCountry:   "ES",
		Quantity:             1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ES reduced rate = 10%.
	if !result.Rate.Equal(decimal.NewFromFloat(10.0)) {
		t.Errorf("rate: want 10.0, got %s", result.Rate)
	}
	if result.RateType != RateTypeReduced {
		t.Errorf("rate type: want %q, got %q", RateTypeReduced, result.RateType)
	}
	if !result.Amount.Equal(decimal.NewFromFloat(10.00)) {
		t.Errorf("VAT: want 10.00, got %s", result.Amount)
	}
}

func TestVATService_CalculateForProduct_CountryOverride(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	reducedCatID := lookupVATCategoryID(t, "reduced")

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	// Override: this product uses reduced rate specifically in France.
	insertProductVATOverride(t, productID, "FR", reducedCatID)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(100.00),
		DestinationCountry: "FR",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FR reduced rate = 5.5%.
	if !result.Rate.Equal(decimal.NewFromFloat(5.5)) {
		t.Errorf("rate: want 5.5, got %s", result.Rate)
	}
	if result.RateType != RateTypeReduced {
		t.Errorf("rate type: want %q, got %q", RateTypeReduced, result.RateType)
	}
	if !result.Amount.Equal(decimal.NewFromFloat(5.50)) {
		t.Errorf("VAT: want 5.50, got %s", result.Amount)
	}
}

func TestVATService_CalculateForProduct_B2BReverseCharge(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", true, "standard", true)

	// Insert valid, non-expired VIES cache entry.
	insertVIESCache(t, "DE123456789", true, "Acme GmbH", time.Now().Add(24*time.Hour))

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(121.00),
		DestinationCountry: "DE",
		CustomerVATNumber:  "DE123456789",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.ReverseCharge {
		t.Error("expected ReverseCharge=true")
	}
	if result.ExemptReason != ExemptReasonReverseCharge {
		t.Errorf("exempt reason: want %q, got %q", ExemptReasonReverseCharge, result.ExemptReason)
	}
	if !result.Amount.IsZero() {
		t.Errorf("VAT amount: want 0, got %s", result.Amount)
	}
	if result.CustomerVATNumber != "DE123456789" {
		t.Errorf("customer VAT: want DE123456789, got %q", result.CustomerVATNumber)
	}
	if result.CompanyName != "Acme GmbH" {
		t.Errorf("company name: want Acme GmbH, got %q", result.CompanyName)
	}
	// Price includes VAT (ES 21%), net should be 121/1.21 = 100.00.
	if !result.NetPrice.Equal(decimal.NewFromFloat(100.00)) {
		t.Errorf("net: want 100.00, got %s", result.NetPrice)
	}
}

func TestVATService_CalculateForProduct_B2BReverseCharge_SameCountry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", true, "standard", true)

	// Even though this VAT number is valid and cached, reverse charge should NOT
	// apply when destination == store country (domestic B2B).
	insertVIESCache(t, "ESB12345678", true, "Empresa S.L.", time.Now().Add(24*time.Hour))

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(121.00),
		DestinationCountry: "ES", // same as store country
		CustomerVATNumber:  "ESB12345678",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ReverseCharge {
		t.Error("expected ReverseCharge=false for same-country")
	}
	if !result.Rate.Equal(decimal.NewFromFloat(21.0)) {
		t.Errorf("rate: want 21.0 (domestic), got %s", result.Rate)
	}
	if result.Amount.IsZero() {
		t.Error("expected non-zero VAT for domestic B2B")
	}
}

func TestVATService_CalculateForProduct_B2BReverseCharge_ExpiredCache(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", true)

	// Expired VIES cache entry -- should fall through to normal VAT.
	insertVIESCache(t, "DE123456789", true, "Acme GmbH", time.Now().Add(-1*time.Hour))

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(100.00),
		DestinationCountry: "DE",
		CustomerVATNumber:  "DE123456789",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ReverseCharge {
		t.Error("expected ReverseCharge=false for expired cache")
	}
	if !result.Rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("rate: want 19.0, got %s", result.Rate)
	}
	if result.Amount.IsZero() {
		t.Error("expected non-zero VAT when cache expired")
	}
}

func TestVATService_CalculateForProduct_B2BReverseCharge_NoCacheEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", true)

	// No VIES cache entry at all -- should fall through to normal VAT.
	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:          productID,
		Price:              decimal.NewFromFloat(100.00),
		DestinationCountry: "FR",
		CustomerVATNumber:  "FR12345678901",
		Quantity:           1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ReverseCharge {
		t.Error("expected ReverseCharge=false when no cache entry")
	}
	// FR standard = 20%.
	if !result.Rate.Equal(decimal.NewFromFloat(20.0)) {
		t.Errorf("rate: want 20.0, got %s", result.Rate)
	}
}

func TestVATService_CalculateForProduct_FallbackStandardRate(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	// super_reduced is seeded by migration; DE has no super_reduced rate in cache.
	superReducedCatID := lookupVATCategoryID(t, "super_reduced")

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	productID := insertProduct(t)

	result, err := svc.CalculateForProduct(context.Background(), VATInput{
		ProductID:            productID,
		ProductVATCategoryID: &superReducedCatID,
		Price:                decimal.NewFromFloat(100.00),
		DestinationCountry:   "DE", // DE has no super_reduced rate
		Quantity:             1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to DE standard (19%).
	if !result.Rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("rate: want 19.0 (fallback standard), got %s", result.Rate)
	}
	if result.RateType != RateTypeStandard {
		t.Errorf("rate type: want %q (fallback), got %q", RateTypeStandard, result.RateType)
	}
}

func TestVATService_CalculateForCart_MultipleItems(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	reducedID := lookupVATCategoryID(t, "reduced")

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())
	product1 := insertProduct(t)
	product2 := insertProduct(t)

	items := []VATInput{
		{
			ProductID:          product1,
			Price:              decimal.NewFromFloat(100.00),
			DestinationCountry: "DE",
			Quantity:           2,
		},
		{
			ProductID:            product2,
			ProductVATCategoryID: &reducedID,
			Price:                decimal.NewFromFloat(50.00),
			DestinationCountry:   "DE",
			Quantity:             3,
		},
	}

	results, summary, err := svc.CalculateForCart(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Item 1: DE standard 19%, 100*2=200 net, 38 VAT, 238 gross.
	r1 := results[0]
	if !r1.Rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("item 1 rate: want 19.0, got %s", r1.Rate)
	}
	if !r1.LineNetTotal.Equal(decimal.NewFromFloat(200.00)) {
		t.Errorf("item 1 line net: want 200.00, got %s", r1.LineNetTotal)
	}
	if !r1.LineVATTotal.Equal(decimal.NewFromFloat(38.00)) {
		t.Errorf("item 1 line VAT: want 38.00, got %s", r1.LineVATTotal)
	}

	// Item 2: DE reduced 7%, 50*3=150 net, 10.50 VAT, 160.50 gross.
	r2 := results[1]
	if !r2.Rate.Equal(decimal.NewFromFloat(7.0)) {
		t.Errorf("item 2 rate: want 7.0, got %s", r2.Rate)
	}
	if !r2.LineNetTotal.Equal(decimal.NewFromFloat(150.00)) {
		t.Errorf("item 2 line net: want 150.00, got %s", r2.LineNetTotal)
	}
	if !r2.LineVATTotal.Equal(decimal.NewFromFloat(10.50)) {
		t.Errorf("item 2 line VAT: want 10.50, got %s", r2.LineVATTotal)
	}

	// Summary: 200+150=350 net, 38+10.50=48.50 VAT, 238+160.50=398.50 gross.
	if !summary.TotalNet.Equal(decimal.NewFromFloat(350.00)) {
		t.Errorf("summary net: want 350.00, got %s", summary.TotalNet)
	}
	if !summary.TotalVAT.Equal(decimal.NewFromFloat(48.50)) {
		t.Errorf("summary VAT: want 48.50, got %s", summary.TotalVAT)
	}
	if !summary.TotalGross.Equal(decimal.NewFromFloat(398.50)) {
		t.Errorf("summary gross: want 398.50, got %s", summary.TotalGross)
	}
}

func TestVATService_CalculateForCart_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	setupStoreSettings(t, true, "ES", false, "standard", false)

	svc := NewVATService(testDB.Pool, newIntegrationRateCache(), slog.Default())

	results, summary, err := svc.CalculateForCart(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %d items", len(results))
	}
	if !summary.TotalNet.IsZero() || !summary.TotalVAT.IsZero() || !summary.TotalGross.IsZero() {
		t.Errorf("expected zero summary, got net=%s vat=%s gross=%s",
			summary.TotalNet, summary.TotalVAT, summary.TotalGross)
	}
}

func TestVATService_BuildResult_ZeroQuantity(t *testing.T) {
	svc := &VATService{logger: slog.Default()}

	calc := VATCalculationResult{
		Rate:        decimal.NewFromFloat(21.0),
		RateType:    RateTypeStandard,
		Amount:      decimal.NewFromFloat(21.00),
		NetPrice:    decimal.NewFromFloat(100.00),
		GrossPrice:  decimal.NewFromFloat(121.00),
		CountryCode: "ES",
	}

	// Zero quantity should be treated as 1.
	result := svc.buildResult(calc, 0, false, "", "")

	if !result.LineNetTotal.Equal(decimal.NewFromFloat(100.00)) {
		t.Errorf("line net: want 100.00 (qty treated as 1), got %s", result.LineNetTotal)
	}
	if !result.LineVATTotal.Equal(decimal.NewFromFloat(21.00)) {
		t.Errorf("line VAT: want 21.00, got %s", result.LineVATTotal)
	}
	if !result.LineGrossTotal.Equal(decimal.NewFromFloat(121.00)) {
		t.Errorf("line gross: want 121.00, got %s", result.LineGrossTotal)
	}
}

func TestVATService_BuildResult_LineTotal(t *testing.T) {
	svc := &VATService{logger: slog.Default()}

	calc := VATCalculationResult{
		Rate:        decimal.NewFromFloat(19.0),
		RateType:    RateTypeStandard,
		Amount:      decimal.NewFromFloat(4.75),
		NetPrice:    decimal.NewFromFloat(25.00),
		GrossPrice:  decimal.NewFromFloat(29.75),
		CountryCode: "DE",
	}

	result := svc.buildResult(calc, 5, false, "", "")

	// 25*5=125, 4.75*5=23.75, 29.75*5=148.75.
	if !result.LineNetTotal.Equal(decimal.NewFromFloat(125.00)) {
		t.Errorf("line net: want 125.00, got %s", result.LineNetTotal)
	}
	if !result.LineVATTotal.Equal(decimal.NewFromFloat(23.75)) {
		t.Errorf("line VAT: want 23.75, got %s", result.LineVATTotal)
	}
	if !result.LineGrossTotal.Equal(decimal.NewFromFloat(148.75)) {
		t.Errorf("line gross: want 148.75, got %s", result.LineGrossTotal)
	}
}

// ===========================================================================
// RateSyncer DB integration tests
// ===========================================================================

func TestRateSyncer_LoadFromDB_NoRates(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	syncer := &RateSyncer{
		db:     testDB.Pool,
		logger: slog.Default(),
		cache:  NewRateCache(),
	}

	rates, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rates) != 0 {
		t.Errorf("expected 0 rates from empty DB, got %d", len(rates))
	}
}

func TestRateSyncer_LoadFromDB_WithRates(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	insertVATRate(t, "DE", RateTypeStandard, 19.0, SourceSeed)
	insertVATRate(t, "ES", RateTypeStandard, 21.0, SourceSeed)
	insertVATRate(t, "FR", RateTypeReduced, 5.5, SourceSeed)

	syncer := &RateSyncer{
		db:     testDB.Pool,
		logger: slog.Default(),
		cache:  NewRateCache(),
	}

	rates, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rates) != 3 {
		t.Errorf("expected 3 rates, got %d", len(rates))
	}

	rateMap := make(map[string]decimal.Decimal)
	for _, r := range rates {
		rateMap[r.CountryCode+":"+r.RateType] = r.Rate
	}
	if !rateMap["DE:"+RateTypeStandard].Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("DE standard: want 19.0, got %s", rateMap["DE:"+RateTypeStandard])
	}
	if !rateMap["ES:"+RateTypeStandard].Equal(decimal.NewFromFloat(21.0)) {
		t.Errorf("ES standard: want 21.0, got %s", rateMap["ES:"+RateTypeStandard])
	}
	if !rateMap["FR:"+RateTypeReduced].Equal(decimal.NewFromFloat(5.5)) {
		t.Errorf("FR reduced: want 5.5, got %s", rateMap["FR:"+RateTypeReduced])
	}
}

func TestRateSyncer_SaveRates_NewRates(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	syncer := &RateSyncer{
		db:     testDB.Pool,
		logger: slog.Default(),
		cache:  NewRateCache(),
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	rates := []VATRate{
		{ID: uuid.New().String(), CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0), ValidFrom: today, Source: SourceECTEDB},
		{ID: uuid.New().String(), CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0), ValidFrom: today, Source: SourceECTEDB},
	}

	err := syncer.saveRates(context.Background(), rates, SourceECTEDB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("loading rates: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 rates in DB, got %d", len(loaded))
	}
}

func TestRateSyncer_SaveRates_UnchangedRates(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Pre-insert a rate with yesterday's date.
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	yesterdayDate := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
	insertVATRateWithDate(t, "DE", RateTypeStandard, 19.0, SourceSeed, yesterdayDate)

	syncer := &RateSyncer{
		db:     testDB.Pool,
		logger: slog.Default(),
		cache:  NewRateCache(),
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Save the same rate value (19.0). Should only update synced_at, not create new row.
	rates := []VATRate{
		{ID: uuid.New().String(), CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0), ValidFrom: today, Source: SourceECTEDB},
	}

	err := syncer.saveRates(context.Background(), rates, SourceECTEDB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have exactly 1 active rate.
	loaded, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("loading rates: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 rate (unchanged), got %d", len(loaded))
	}
	if !loaded[0].Rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("rate: want 19.0, got %s", loaded[0].Rate)
	}
}

func TestRateSyncer_SaveRates_ChangedRates(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Pre-insert a rate with yesterday's date.
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	yesterdayDate := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
	insertVATRateWithDate(t, "DE", RateTypeStandard, 19.0, SourceSeed, yesterdayDate)

	syncer := &RateSyncer{
		db:     testDB.Pool,
		logger: slog.Default(),
		cache:  NewRateCache(),
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Save a changed rate (19 -> 20). Old rate should be expired, new one inserted.
	rates := []VATRate{
		{ID: uuid.New().String(), CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0), ValidFrom: today, Source: SourceECTEDB},
	}

	err := syncer.saveRates(context.Background(), rates, SourceECTEDB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// loadFromDB only returns active rates (valid_to IS NULL).
	loaded, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("loading rates: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 active rate, got %d", len(loaded))
	}
	if !loaded[0].Rate.Equal(decimal.NewFromFloat(20.0)) {
		t.Errorf("active rate: want 20.0, got %s", loaded[0].Rate)
	}

	// Verify the old rate was expired (valid_to IS NOT NULL).
	var expiredCount int
	err = testDB.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM vat_rates WHERE country_code = 'DE' AND rate_type = 'standard' AND valid_to IS NOT NULL`,
	).Scan(&expiredCount)
	if err != nil {
		t.Fatalf("counting expired rates: %v", err)
	}
	if expiredCount != 1 {
		t.Errorf("expected 1 expired rate, got %d", expiredCount)
	}
}

func TestRateSyncer_Sync_WithMockHTTP(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Mock TEDB server that returns an error (so we fall through to euvatrates.com).
	tedbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tedbServer.Close()

	// Mock euvatrates.com server with valid JSON.
	euvatResponse := map[string]interface{}{
		"rates": map[string]interface{}{
			"DE": map[string]interface{}{
				"country":            "Germany",
				"standard_rate":      19.0,
				"reduced_rate":       7.0,
				"reduced_rate_alt":   false,
				"super_reduced_rate": false,
				"parking_rate":       false,
			},
			"ES": map[string]interface{}{
				"country":            "Spain",
				"standard_rate":      21.0,
				"reduced_rate":       10.0,
				"reduced_rate_alt":   false,
				"super_reduced_rate": 4.0,
				"parking_rate":       false,
			},
			"FR": map[string]interface{}{
				"country":            "France",
				"standard_rate":      20.0,
				"reduced_rate":       5.5,
				"reduced_rate_alt":   10.0,
				"super_reduced_rate": 2.1,
				"parking_rate":       false,
			},
		},
	}
	euvatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(euvatResponse)
	}))
	defer euvatServer.Close()

	cache := NewRateCache()
	syncer := &RateSyncer{
		db: testDB.Pool,
		cfg: config.VATConfig{
			TEDBTimeout: 5 * time.Second,
			FallbackURL: euvatServer.URL,
		},
		logger: slog.Default(),
		cache:  cache,
		client: euvatServer.Client(),
	}

	result := syncer.Sync(context.Background())

	if result.Error != nil {
		t.Fatalf("sync error: %v", result.Error)
	}
	if result.Source != SourceEUVATRatesJSON {
		t.Errorf("source: want %q, got %q", SourceEUVATRatesJSON, result.Source)
	}
	if result.RatesLoaded == 0 {
		t.Error("expected >0 rates loaded")
	}

	// Verify cache was populated.
	deStd, ok := cache.Get("DE", RateTypeStandard)
	if !ok {
		t.Error("DE standard not in cache")
	} else if !deStd.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("DE standard: want 19.0, got %s", deStd)
	}

	esStd, ok := cache.Get("ES", RateTypeStandard)
	if !ok {
		t.Error("ES standard not in cache")
	} else if !esStd.Equal(decimal.NewFromFloat(21.0)) {
		t.Errorf("ES standard: want 21.0, got %s", esStd)
	}

	// Verify rates were persisted to DB.
	loaded, err := syncer.loadFromDB(context.Background())
	if err != nil {
		t.Fatalf("loading from DB: %v", err)
	}
	if len(loaded) == 0 {
		t.Error("expected rates in DB after sync")
	}
}

// ===========================================================================
// VIESClient integration tests
// ===========================================================================

func TestVIESClient_GetFromCache_NoEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	_, err := client.getFromCache(context.Background(), "DE999999999")
	if err == nil {
		t.Error("expected error for missing cache entry, got nil")
	}
}

func TestVIESClient_GetFromCache_ValidEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	insertVIESCache(t, "DE123456789", true, "Acme GmbH", time.Now().Add(24*time.Hour))

	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	result, err := client.getFromCache(context.Background(), "DE123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.CompanyName != "Acme GmbH" {
		t.Errorf("company: want 'Acme GmbH', got %q", result.CompanyName)
	}
	if result.CountryCode != "DE" {
		t.Errorf("country: want DE, got %q", result.CountryCode)
	}
	if result.VATNumber != "DE123456789" {
		t.Errorf("vat number: want DE123456789, got %q", result.VATNumber)
	}
}

func TestVIESClient_GetFromCache_ExpiredEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Expired 1 hour ago.
	insertVIESCache(t, "FR12345678901", true, "SAS Test", time.Now().Add(-1*time.Hour))

	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	_, err := client.getFromCache(context.Background(), "FR12345678901")
	if err == nil {
		t.Error("expected error for expired entry, got nil")
	}
}

func TestVIESClient_SaveToCache_NewEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	result := VIESResult{
		Valid:              true,
		CompanyName:        "Test Company",
		CompanyAddress:     "123 Test St",
		ConsultationNumber: "FC-12345678",
		CountryCode:        "DE",
		VATNumber:          "DE111222333",
	}
	err := client.saveToCache(context.Background(), result)
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify it was saved by reading it back.
	got, err := client.getFromCache(context.Background(), "DE111222333")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !got.Valid {
		t.Error("expected Valid=true")
	}
	if got.CompanyName != "Test Company" {
		t.Errorf("company: want 'Test Company', got %q", got.CompanyName)
	}
}

func TestVIESClient_SaveToCache_UpsertExisting(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	// Insert initial entry.
	result1 := VIESResult{
		Valid:       true,
		CompanyName: "Old Name",
		CountryCode: "ES",
		VATNumber:   "ESB12345678",
	}
	if err := client.saveToCache(context.Background(), result1); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Upsert with updated company name.
	result2 := VIESResult{
		Valid:       true,
		CompanyName: "New Name",
		CountryCode: "ES",
		VATNumber:   "ESB12345678",
	}
	if err := client.saveToCache(context.Background(), result2); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Read back to verify upsert.
	got, err := client.getFromCache(context.Background(), "ESB12345678")
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if got.CompanyName != "New Name" {
		t.Errorf("company: want 'New Name', got %q", got.CompanyName)
	}
}

func TestVIESClient_Validate_CacheHit(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Pre-populate cache.
	insertVIESCache(t, "DE123456789", true, "Cached GmbH", time.Now().Add(24*time.Hour))

	// VIES server that should NOT be called (because of cache hit).
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &VIESClient{
		pool:     testDB.Pool,
		client:   server.Client(),
		logger:   slog.Default(),
		cacheTTL: 24 * time.Hour,
	}

	result, err := client.Validate(context.Background(), "DE123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("VIES server was called despite cache hit")
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.CompanyName != "Cached GmbH" {
		t.Errorf("company: want 'Cached GmbH', got %q", result.CompanyName)
	}
}

func TestVIESClient_Validate_CacheMiss_LiveCall(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Mock VIES SOAP server.
	viesResponse := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <checkVatResponse xmlns="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
      <countryCode>FR</countryCode>
      <vatNumber>12345678901</vatNumber>
      <requestDate>%s</requestDate>
      <valid>true</valid>
      <name>Societe Test SARL</name>
      <address>1 Rue de Test, Paris</address>
    </checkVatResponse>
  </soap:Body>
</soap:Envelope>`, time.Now().Format("2006-01-02"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "text/xml; charset=utf-8" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(viesResponse))
	}))
	defer server.Close()

	client := &VIESClient{
		pool:     testDB.Pool,
		client:   server.Client(),
		logger:   slog.Default(),
		cacheTTL: 24 * time.Hour,
	}

	// Since callVIES uses the package-level const viesEndpoint, we test via
	// the helper that mirrors callVIES but allows URL override.
	result, err := callVIESWithURL(context.Background(), client, server.URL, "FR", "12345678901")
	if err != nil {
		t.Fatalf("callVIES error: %v", err)
	}

	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.CompanyName != "Societe Test SARL" {
		t.Errorf("company: want 'Societe Test SARL', got %q", result.CompanyName)
	}

	// Save to cache and verify round-trip.
	result.VATNumber = "FR12345678901"
	result.CountryCode = "FR"
	if err := client.saveToCache(context.Background(), result); err != nil {
		t.Fatalf("saving to cache: %v", err)
	}

	cached, err := client.getFromCache(context.Background(), "FR12345678901")
	if err != nil {
		t.Fatalf("reading from cache: %v", err)
	}
	if !cached.Valid {
		t.Error("cached result should be valid")
	}
	if cached.CompanyName != "Societe Test SARL" {
		t.Errorf("cached company: want 'Societe Test SARL', got %q", cached.CompanyName)
	}
}

func TestVIESClient_Validate_TooShort(t *testing.T) {
	client := NewVIESClient(testDB.Pool, 10*time.Second, 24*time.Hour, slog.Default())

	_, err := client.Validate(context.Background(), "DE1")
	if err == nil {
		t.Error("expected error for too-short VAT number, got nil")
	}
}

// ===========================================================================
// Scheduler tests
// ===========================================================================

func TestDurationUntilNextMidnightUTC(t *testing.T) {
	d := durationUntilNextMidnightUTC()

	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 24*time.Hour {
		t.Errorf("expected duration <= 24h, got %v", d)
	}
}

func TestScheduler_StopImmediately(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	// Create mock servers that fail, so the initial sync hits the DB fallback path.
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	cache := NewRateCache()
	syncer := &RateSyncer{
		db: testDB.Pool,
		cfg: config.VATConfig{
			TEDBTimeout: 1 * time.Second,
			FallbackURL: failServer.URL,
		},
		logger: slog.Default(),
		cache:  cache,
		client: failServer.Client(),
	}

	scheduler := NewScheduler(syncer, slog.Default())

	// Start and immediately stop. Should not panic or hang.
	scheduler.Start(context.Background())
	scheduler.Stop()

	// Call Stop again to verify it is safe to call multiple times.
	scheduler.Stop()
}

// ===========================================================================
// flexibleFloat tests
// ===========================================================================

func TestFlexibleFloat_UnmarshalJSON_Number(t *testing.T) {
	var f flexibleFloat
	err := f.UnmarshalJSON([]byte("19.5"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if float64(f) != 19.5 {
		t.Errorf("want 19.5, got %f", f)
	}
}

func TestFlexibleFloat_UnmarshalJSON_False(t *testing.T) {
	var f flexibleFloat
	err := f.UnmarshalJSON([]byte("false"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if float64(f) != 0 {
		t.Errorf("want 0 for false, got %f", f)
	}
}

func TestFlexibleFloat_UnmarshalJSON_Invalid(t *testing.T) {
	var f flexibleFloat
	err := f.UnmarshalJSON([]byte(`"not a number"`))
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

package vat

import (
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestRateCache_EmptyCache(t *testing.T) {
	cache := NewRateCache()

	if cache.CountryCount() != 0 {
		t.Errorf("expected 0 countries, got %d", cache.CountryCount())
	}
	if cache.RateCount() != 0 {
		t.Errorf("expected 0 rates, got %d", cache.RateCount())
	}

	rate, ok := cache.Get("DE", RateTypeStandard)
	if ok {
		t.Error("expected Get to return false for empty cache")
	}
	if !rate.IsZero() {
		t.Errorf("expected zero rate, got %s", rate.String())
	}
}

func TestRateCache_LoadSingleCountry(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "DE", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(7.0)},
	})

	if cache.CountryCount() != 1 {
		t.Errorf("expected 1 country, got %d", cache.CountryCount())
	}
	if cache.RateCount() != 2 {
		t.Errorf("expected 2 rates, got %d", cache.RateCount())
	}

	rate, ok := cache.Get("DE", RateTypeStandard)
	if !ok {
		t.Error("expected to find DE standard rate")
	}
	if !rate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("expected 19.0, got %s", rate.String())
	}

	rate, ok = cache.Get("DE", RateTypeReduced)
	if !ok {
		t.Error("expected to find DE reduced rate")
	}
	if !rate.Equal(decimal.NewFromFloat(7.0)) {
		t.Errorf("expected 7.0, got %s", rate.String())
	}
}

func TestRateCache_LoadMultipleCountries(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
	})

	if cache.CountryCount() != 3 {
		t.Errorf("expected 3 countries, got %d", cache.CountryCount())
	}
	if cache.RateCount() != 3 {
		t.Errorf("expected 3 rates, got %d", cache.RateCount())
	}
}

func TestRateCache_GetRate_MissingRateType(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DK", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(25.0)},
	})

	_, ok := cache.Get("DK", RateTypeReduced)
	if ok {
		t.Error("expected DK reduced rate to not be found")
	}
}

func TestRateCache_GetRate_MissingCountry(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
	})

	_, ok := cache.Get("XX", RateTypeStandard)
	if ok {
		t.Error("expected unknown country to not be found")
	}
}

func TestRateCache_GetCountryRates(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
		{CountryCode: "ES", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(10.0)},
		{CountryCode: "ES", RateType: RateTypeSuperReduced, Rate: decimal.NewFromFloat(4.0)},
	})

	countryRates, ok := cache.GetCountryRates("ES")
	if !ok {
		t.Fatal("expected to find ES country rates")
	}
	if countryRates.CountryCode != "ES" {
		t.Errorf("expected country code ES, got %s", countryRates.CountryCode)
	}
	if len(countryRates.Rates) != 3 {
		t.Errorf("expected 3 rate types, got %d", len(countryRates.Rates))
	}

	// Verify the returned copy is independent (mutation test).
	countryRates.Rates["extra"] = decimal.NewFromFloat(99.0)
	_, ok = cache.Get("ES", "extra")
	if ok {
		t.Error("modifying returned CountryVATRates should not affect cache")
	}
}

func TestRateCache_GetCountryRates_NotFound(t *testing.T) {
	cache := NewRateCache()
	_, ok := cache.GetCountryRates("XX")
	if ok {
		t.Error("expected GetCountryRates to return false for unknown country")
	}
}

func TestRateCache_GetAll(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	})

	all := cache.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 countries, got %d", len(all))
	}

	// Verify mutation safety.
	all["XX"] = CountryVATRates{CountryCode: "XX"}
	if cache.CountryCount() != 2 {
		t.Error("modifying GetAll result should not affect the cache")
	}
}

func TestRateCache_LoadReplacesExistingData(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
	})

	if cache.CountryCount() != 2 {
		t.Fatalf("expected 2 countries before reload, got %d", cache.CountryCount())
	}

	cache.Load([]VATRate{
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	})

	if cache.CountryCount() != 1 {
		t.Errorf("expected 1 country after reload, got %d", cache.CountryCount())
	}

	_, ok := cache.Get("DE", RateTypeStandard)
	if ok {
		t.Error("DE should have been removed after reload")
	}

	rate, ok := cache.Get("ES", RateTypeStandard)
	if !ok {
		t.Fatal("ES should exist after reload")
	}
	if !rate.Equal(decimal.NewFromFloat(21.0)) {
		t.Errorf("expected ES standard 21.0, got %s", rate.String())
	}
}

func TestRateCache_LoadSkipsExpiredRates(t *testing.T) {
	cache := NewRateCache()
	past := time.Now().Add(-24 * time.Hour)
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "DE", RateType: RateTypeReduced, Rate: decimal.NewFromFloat(7.0), ValidTo: &past},
	})

	if cache.RateCount() != 1 {
		t.Errorf("expected 1 rate (expired skipped), got %d", cache.RateCount())
	}

	_, ok := cache.Get("DE", RateTypeReduced)
	if ok {
		t.Error("expired rate should not be in cache")
	}
}

func TestRateCache_ConcurrentAccess(t *testing.T) {
	cache := NewRateCache()
	cache.Load([]VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	})

	var wg sync.WaitGroup
	const goroutines = 100

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Get("DE", RateTypeStandard)
			cache.Get("ES", RateTypeStandard)
			cache.CountryCount()
			cache.RateCount()
			cache.GetCountryRates("DE")
			cache.GetAll()
		}()
	}

	// Concurrent writes interspersed.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Load([]VATRate{
				{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
				{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
			})
		}()
	}

	wg.Wait()
	// If we get here without a race condition panic, the test passes.
}

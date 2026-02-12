package vat

import (
	"sync"

	"github.com/shopspring/decimal"
)

// RateCache is a thread-safe in-memory cache for VAT rates.
// It stores rates indexed by country code, with each country holding
// a map of rate_type -> rate percentage.
// Uses sync.RWMutex to allow concurrent reads while serializing writes.
type RateCache struct {
	mu    sync.RWMutex
	rates map[string]CountryVATRates // country_code -> CountryVATRates
}

// NewRateCache creates a new empty RateCache.
func NewRateCache() *RateCache {
	return &RateCache{
		rates: make(map[string]CountryVATRates),
	}
}

// Get retrieves a specific VAT rate for a country and rate type.
// Returns the rate percentage and true if found, or zero and false if not.
func (c *RateCache) Get(countryCode, rateType string) (decimal.Decimal, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	country, ok := c.rates[countryCode]
	if !ok {
		return decimal.Zero, false
	}

	rate, ok := country.Rates[rateType]
	if !ok {
		return decimal.Zero, false
	}

	return rate, true
}

// GetCountryRates retrieves all VAT rates for a given country.
// Returns the CountryVATRates and true if found, or an empty struct and false if not.
func (c *RateCache) GetCountryRates(countryCode string) (CountryVATRates, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	country, ok := c.rates[countryCode]
	if !ok {
		return CountryVATRates{}, false
	}

	// Return a copy to prevent external mutation.
	copied := CountryVATRates{
		CountryCode: country.CountryCode,
		Rates:       make(map[string]decimal.Decimal, len(country.Rates)),
	}
	for k, v := range country.Rates {
		copied.Rates[k] = v
	}

	return copied, true
}

// GetAll returns a copy of all cached rates.
func (c *RateCache) GetAll() map[string]CountryVATRates {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]CountryVATRates, len(c.rates))
	for code, country := range c.rates {
		copied := CountryVATRates{
			CountryCode: country.CountryCode,
			Rates:       make(map[string]decimal.Decimal, len(country.Rates)),
		}
		for k, v := range country.Rates {
			copied.Rates[k] = v
		}
		result[code] = copied
	}

	return result
}

// Load replaces the entire cache with rates parsed from a slice of VATRate.
// Only loads rates that are currently active (ValidTo is nil).
func (c *RateCache) Load(rates []VATRate) {
	newRates := make(map[string]CountryVATRates, 30) // 27 EU members + buffer

	for _, r := range rates {
		// Only cache currently active rates.
		if r.ValidTo != nil {
			continue
		}

		country, ok := newRates[r.CountryCode]
		if !ok {
			country = CountryVATRates{
				CountryCode: r.CountryCode,
				Rates:       make(map[string]decimal.Decimal),
			}
		}
		country.Rates[r.RateType] = r.Rate
		newRates[r.CountryCode] = country
	}

	c.mu.Lock()
	c.rates = newRates
	c.mu.Unlock()
}

// CountryCount returns the number of countries in the cache.
func (c *RateCache) CountryCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rates)
}

// RateCount returns the total number of individual rates in the cache.
func (c *RateCache) RateCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, country := range c.rates {
		count += len(country.Rates)
	}
	return count
}

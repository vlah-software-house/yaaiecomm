package vat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgecommerce/api/internal/config"
	"github.com/shopspring/decimal"
)

func TestFetchFromEUVATRates_ValidJSON(t *testing.T) {
	mockResponse := map[string]interface{}{
		"rates": map[string]interface{}{
			"DE": map[string]interface{}{
				"country":            "Germany",
				"standard_rate":      19.0,
				"reduced_rate":       7.0,
				"reduced_rate_alt":   0.0,
				"super_reduced_rate": 0.0,
				"parking_rate":       0.0,
			},
			"ES": map[string]interface{}{
				"country":            "Spain",
				"standard_rate":      21.0,
				"reduced_rate":       10.0,
				"reduced_rate_alt":   0.0,
				"super_reduced_rate": 4.0,
				"parking_rate":       0.0,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg:    config.VATConfig{FallbackURL: server.URL},
		logger: slog.Default(),
		client: server.Client(),
	}

	rates, err := syncer.fetchFromEUVATRates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("expected non-empty rates")
	}

	// Germany: standard + reduced = 2. Spain: standard + reduced + super_reduced = 3. Total = 5.
	if len(rates) != 5 {
		t.Errorf("expected 5 rates, got %d", len(rates))
	}

	// Verify DE standard rate.
	found := false
	for _, r := range rates {
		if r.CountryCode == "DE" && r.RateType == RateTypeStandard {
			if !r.Rate.Equal(decimal.NewFromFloat(19.0)) {
				t.Errorf("expected DE standard rate 19.0, got %s", r.Rate.String())
			}
			if r.Source != SourceEUVATRatesJSON {
				t.Errorf("expected source %q, got %q", SourceEUVATRatesJSON, r.Source)
			}
			found = true
		}
	}
	if !found {
		t.Error("DE standard rate not found in parsed rates")
	}
}

func TestFetchFromEUVATRates_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg:    config.VATConfig{FallbackURL: server.URL},
		logger: slog.Default(),
		client: server.Client(),
	}

	_, err := syncer.fetchFromEUVATRates(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestFetchFromEUVATRates_EmptyRates(t *testing.T) {
	mockResponse := map[string]interface{}{
		"rates": map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg:    config.VATConfig{FallbackURL: server.URL},
		logger: slog.Default(),
		client: server.Client(),
	}

	_, err := syncer.fetchFromEUVATRates(context.Background())
	if err == nil {
		t.Fatal("expected error for empty rates, got nil")
	}
}

func TestFetchFromEUVATRates_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg:    config.VATConfig{FallbackURL: server.URL},
		logger: slog.Default(),
		client: server.Client(),
	}

	_, err := syncer.fetchFromEUVATRates(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestFetchFromEUVATRates_SkipsNon2LetterCodes(t *testing.T) {
	mockResponse := map[string]interface{}{
		"rates": map[string]interface{}{
			"DE": map[string]interface{}{
				"country":            "Germany",
				"standard_rate":      19.0,
				"reduced_rate":       0.0,
				"reduced_rate_alt":   0.0,
				"super_reduced_rate": 0.0,
				"parking_rate":       0.0,
			},
			"INVALID": map[string]interface{}{
				"country":            "Invalid Country",
				"standard_rate":      15.0,
				"reduced_rate":       0.0,
				"reduced_rate_alt":   0.0,
				"super_reduced_rate": 0.0,
				"parking_rate":       0.0,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg:    config.VATConfig{FallbackURL: server.URL},
		logger: slog.Default(),
		client: server.Client(),
	}

	rates, err := syncer.fetchFromEUVATRates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only DE standard rate should be included (1 rate).
	if len(rates) != 1 {
		t.Errorf("expected 1 rate (skipping INVALID), got %d", len(rates))
	}
}

func TestDetectChanges_NoChanges(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	old := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	}
	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	}

	changes := syncer.detectChanges(old, newRates)
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}

func TestDetectChanges_RateChanged(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	old := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
	}
	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
	}

	changes := syncer.detectChanges(old, newRates)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	ch := changes[0]
	if ch.CountryCode != "DE" {
		t.Errorf("expected country DE, got %s", ch.CountryCode)
	}
	if !ch.OldRate.Equal(decimal.NewFromFloat(19.0)) {
		t.Errorf("expected old rate 19.0, got %s", ch.OldRate.String())
	}
	if !ch.NewRate.Equal(decimal.NewFromFloat(20.0)) {
		t.Errorf("expected new rate 20.0, got %s", ch.NewRate.String())
	}
}

func TestDetectChanges_NewRate(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	old := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
	}
	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)},
	}

	changes := syncer.detectChanges(old, newRates)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change (new rate), got %d", len(changes))
	}

	ch := changes[0]
	if ch.CountryCode != "FR" {
		t.Errorf("expected country FR, got %s", ch.CountryCode)
	}
	if !ch.OldRate.IsZero() {
		t.Errorf("expected old rate 0 (new), got %s", ch.OldRate.String())
	}
	if !ch.NewRate.Equal(decimal.NewFromFloat(20.0)) {
		t.Errorf("expected new rate 20.0, got %s", ch.NewRate.String())
	}
}

func TestDetectChanges_IgnoresExpiredOldRates(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	past := time.Now().Add(-24 * time.Hour)
	old := []VATRate{
		// This rate has ValidTo set (expired), should be ignored in the old map.
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0), ValidTo: &past},
	}
	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
	}

	changes := syncer.detectChanges(old, newRates)
	// DE standard appears as "new" because the old rate was expired (has ValidTo).
	if len(changes) != 1 {
		t.Fatalf("expected 1 change (expired old treated as absent), got %d", len(changes))
	}
	if !changes[0].OldRate.IsZero() {
		t.Errorf("expected old rate 0 (expired), got %s", changes[0].OldRate.String())
	}
}

func TestDetectChanges_MultipleChanges(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	old := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	}
	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)}, // changed
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(22.0)}, // changed
		{CountryCode: "FR", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(20.0)}, // new
	}

	changes := syncer.detectChanges(old, newRates)
	if len(changes) != 3 {
		t.Errorf("expected 3 changes, got %d", len(changes))
	}
}

func TestDetectChanges_EmptyOld(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	newRates := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
		{CountryCode: "ES", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(21.0)},
	}

	changes := syncer.detectChanges(nil, newRates)
	if len(changes) != 2 {
		t.Errorf("expected 2 changes (all new), got %d", len(changes))
	}
}

func TestDetectChanges_EmptyNew(t *testing.T) {
	syncer := &RateSyncer{logger: slog.Default()}

	old := []VATRate{
		{CountryCode: "DE", RateType: RateTypeStandard, Rate: decimal.NewFromFloat(19.0)},
	}

	// If new is empty, no changes are detected (removed rates are not flagged by this implementation).
	changes := syncer.detectChanges(old, nil)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for empty new, got %d", len(changes))
	}
}

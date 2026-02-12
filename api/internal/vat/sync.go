package vat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/forgecommerce/api/internal/config"
)

// RateSyncer fetches EU VAT rates from external sources and keeps the
// database and in-memory cache up to date.
type RateSyncer struct {
	db     *pgxpool.Pool
	cfg    config.VATConfig
	logger *slog.Logger
	cache  *RateCache
	client *http.Client
}

// NewRateSyncer creates a new RateSyncer with the given dependencies.
func NewRateSyncer(db *pgxpool.Pool, cfg config.VATConfig, logger *slog.Logger, cache *RateCache) *RateSyncer {
	return &RateSyncer{
		db:     db,
		cfg:    cfg,
		logger: logger,
		cache:  cache,
		client: &http.Client{
			Timeout: cfg.TEDBTimeout,
		},
	}
}

// Sync performs a full VAT rate sync operation. It tries the EC TEDB SOAP
// service first, falls back to the euvatrates.com JSON endpoint, and if
// both fail, loads cached rates from the database.
//
// The result is always persisted to the database (if new) and loaded into
// the in-memory cache.
func (s *RateSyncer) Sync(ctx context.Context) SyncResult {
	now := time.Now().UTC()

	s.logger.Info("starting VAT rate sync")

	// Try primary source: EC TEDB SOAP service.
	rates, err := s.fetchFromTEDB(ctx)
	if err == nil {
		s.logger.Info("fetched VAT rates from EC TEDB", "count", len(rates))
		return s.processFetchedRates(ctx, rates, SourceECTEDB, now)
	}
	s.logger.Warn("EC TEDB sync failed, falling back to euvatrates.com",
		"error", err,
	)

	// Try fallback source: euvatrates.com JSON.
	rates, err = s.fetchFromEUVATRates(ctx)
	if err == nil {
		s.logger.Info("fetched VAT rates from euvatrates.com", "count", len(rates))
		return s.processFetchedRates(ctx, rates, SourceEUVATRatesJSON, now)
	}
	s.logger.Warn("euvatrates.com sync failed, loading from database cache",
		"error", err,
	)

	// Both external sources failed. Load from database.
	dbRates, dbErr := s.loadFromDB(ctx)
	if dbErr != nil {
		s.logger.Error("failed to load VAT rates from database",
			"error", dbErr,
		)
		return SyncResult{
			Source:  SourceCache,
			SyncedAt: now,
			Error:   fmt.Errorf("all VAT rate sources failed: tedb: %w, euvatrates: %v, db: %v", err, err, dbErr),
		}
	}

	s.cache.Load(dbRates)
	s.logger.Info("loaded VAT rates from database cache",
		"countries", s.cache.CountryCount(),
		"rates", s.cache.RateCount(),
	)

	return SyncResult{
		Source:      SourceCache,
		RatesLoaded: len(dbRates),
		SyncedAt:    now,
	}
}

// processFetchedRates detects changes, saves new rates to the database,
// and refreshes the in-memory cache.
func (s *RateSyncer) processFetchedRates(ctx context.Context, rates []VATRate, source string, now time.Time) SyncResult {
	// Load existing rates from the database for change detection.
	existingRates, err := s.loadFromDB(ctx)
	if err != nil {
		s.logger.Warn("could not load existing rates for change detection",
			"error", err,
		)
	}

	changes := s.detectChanges(existingRates, rates)
	if len(changes) > 0 {
		s.logger.Info("VAT rate changes detected",
			"changes", len(changes),
		)
		for _, ch := range changes {
			s.logger.Info("VAT rate changed",
				"country", ch.CountryCode,
				"rate_type", ch.RateType,
				"old_rate", ch.OldRate.String(),
				"new_rate", ch.NewRate.String(),
			)
		}
	} else {
		s.logger.Info("no VAT rate changes detected")
	}

	// Persist to database.
	if err := s.saveRates(ctx, rates, source); err != nil {
		s.logger.Error("failed to save VAT rates to database",
			"error", err,
		)
		// Even if save fails, still load into cache from what we fetched.
	}

	// Refresh in-memory cache.
	s.cache.Load(rates)

	return SyncResult{
		Source:       source,
		RatesLoaded:  len(rates),
		RatesChanged: len(changes),
		SyncedAt:     now,
	}
}

// fetchFromTEDB attempts to fetch VAT rates from the EC TEDB SOAP service.
// This is a stub that returns an error. The full SOAP/XML parsing
// implementation will be added once the WSDL contract is finalized.
func (s *RateSyncer) fetchFromTEDB(_ context.Context) ([]VATRate, error) {
	// TODO: Implement EC TEDB SOAP client.
	// The SOAP endpoint is: https://ec.europa.eu/taxation_customs/tedb/ws/VatRetrievalService.wsdl
	// Action: RetrieveVatRates - returns rates per member state, filterable by date.
	// This requires XML envelope construction and response parsing.
	return nil, fmt.Errorf("EC TEDB SOAP client not yet implemented")
}

// euVATRatesResponse represents the JSON structure returned by euvatrates.com.
type euVATRatesResponse struct {
	Rates map[string]euVATCountryRates `json:"rates"`
}

// euVATCountryRates represents a single country's rates in the euvatrates.com JSON.
type euVATCountryRates struct {
	Country          string  `json:"country"`
	StandardRate     float64 `json:"standard_rate"`
	ReducedRate      float64 `json:"reduced_rate"`
	ReducedRateAlt   float64 `json:"reduced_rate_alt"`
	SuperReducedRate float64 `json:"super_reduced_rate"`
	ParkingRate      float64 `json:"parking_rate"`
}

// fetchFromEUVATRates fetches VAT rates from the euvatrates.com JSON API.
// The JSON format is: {"rates": {"AT": {"country": "Austria", "standard_rate": 20, ...}, ...}}
func (s *RateSyncer) fetchFromEUVATRates(ctx context.Context) ([]VATRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.FallbackURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for euvatrates.com: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ForgeCommerce/1.0 (VAT rate sync)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching from euvatrates.com: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("euvatrates.com returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading euvatrates.com response body: %w", err)
	}

	var data euVATRatesResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing euvatrates.com JSON: %w", err)
	}

	if len(data.Rates) == 0 {
		return nil, fmt.Errorf("euvatrates.com returned empty rates")
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var rates []VATRate

	for countryCode, countryRates := range data.Rates {
		// Only include EU member state codes (2-letter ISO).
		if len(countryCode) != 2 {
			continue
		}

		// Map each non-zero rate to a VATRate entry.
		rateMap := map[string]float64{
			RateTypeStandard:     countryRates.StandardRate,
			RateTypeReduced:      countryRates.ReducedRate,
			RateTypeReducedAlt:   countryRates.ReducedRateAlt,
			RateTypeSuperReduced: countryRates.SuperReducedRate,
			RateTypeParking:      countryRates.ParkingRate,
		}

		for rateType, rateVal := range rateMap {
			if rateVal <= 0 {
				continue // Country does not have this rate type.
			}

			rates = append(rates, VATRate{
				ID:          uuid.New().String(),
				CountryCode: countryCode,
				RateType:    rateType,
				Rate:        decimal.NewFromFloat(rateVal),
				Description: fmt.Sprintf("%s rate for %s", rateType, countryRates.Country),
				ValidFrom:   today,
				ValidTo:     nil,
				Source:       SourceEUVATRatesJSON,
				SyncedAt:    now,
			})
		}
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("no valid VAT rates parsed from euvatrates.com")
	}

	s.logger.Info("parsed euvatrates.com rates",
		"countries", len(data.Rates),
		"total_rates", len(rates),
	)

	return rates, nil
}

// loadFromDB loads currently active VAT rates from the database.
// Active rates are those where valid_to IS NULL.
func (s *RateSyncer) loadFromDB(ctx context.Context) ([]VATRate, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, country_code, rate_type, rate, description,
		       valid_from, valid_to, source, synced_at
		FROM vat_rates
		WHERE valid_to IS NULL
		ORDER BY country_code, rate_type
	`)
	if err != nil {
		return nil, fmt.Errorf("querying vat_rates: %w", err)
	}
	defer rows.Close()

	var rates []VATRate
	for rows.Next() {
		var r VATRate
		if err := rows.Scan(
			&r.ID, &r.CountryCode, &r.RateType, &r.Rate, &r.Description,
			&r.ValidFrom, &r.ValidTo, &r.Source, &r.SyncedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning vat_rate row: %w", err)
		}
		rates = append(rates, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating vat_rate rows: %w", err)
	}

	return rates, nil
}

// saveRates persists fetched rates to the database. It expires old rates
// (sets valid_to) for any country+rate_type combination that has a new value,
// then inserts the new rates.
func (s *RateSyncer) saveRates(ctx context.Context, rates []VATRate, source string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()

	for _, r := range rates {
		// Check if there is an existing active rate for this country + rate_type.
		var existingID string
		var existingRate decimal.Decimal
		err := tx.QueryRow(ctx, `
			SELECT id, rate FROM vat_rates
			WHERE country_code = $1 AND rate_type = $2 AND valid_to IS NULL
			LIMIT 1
		`, r.CountryCode, r.RateType).Scan(&existingID, &existingRate)

		if err == nil {
			// Existing rate found.
			if existingRate.Equal(r.Rate) {
				// Rate unchanged, just update synced_at.
				_, err = tx.Exec(ctx, `
					UPDATE vat_rates SET synced_at = $1 WHERE id = $2
				`, now, existingID)
				if err != nil {
					return fmt.Errorf("updating synced_at for %s/%s: %w", r.CountryCode, r.RateType, err)
				}
				continue
			}

			// Rate changed: expire the old rate.
			_, err = tx.Exec(ctx, `
				UPDATE vat_rates SET valid_to = $1 WHERE id = $2
			`, now, existingID)
			if err != nil {
				return fmt.Errorf("expiring old rate for %s/%s: %w", r.CountryCode, r.RateType, err)
			}
		} else if err != pgx.ErrNoRows {
			return fmt.Errorf("checking existing rate for %s/%s: %w", r.CountryCode, r.RateType, err)
		}

		// Insert the new rate.
		rateID := r.ID
		if rateID == "" {
			rateID = uuid.New().String()
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO vat_rates (id, country_code, rate_type, rate, description, valid_from, valid_to, source, synced_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, rateID, r.CountryCode, r.RateType, r.Rate, r.Description, r.ValidFrom, r.ValidTo, source, now)
		if err != nil {
			return fmt.Errorf("inserting rate for %s/%s: %w", r.CountryCode, r.RateType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing rate sync transaction: %w", err)
	}

	s.logger.Info("saved VAT rates to database",
		"source", source,
		"count", len(rates),
	)

	return nil
}

// detectChanges compares old (from DB) and new (fetched) rates and returns
// a list of changes. Only compares currently active rates.
func (s *RateSyncer) detectChanges(old, new []VATRate) []RateChange {
	// Build a lookup from old rates: country_code + rate_type -> rate.
	oldMap := make(map[string]decimal.Decimal, len(old))
	for _, r := range old {
		if r.ValidTo != nil {
			continue
		}
		key := r.CountryCode + ":" + r.RateType
		oldMap[key] = r.Rate
	}

	var changes []RateChange
	for _, r := range new {
		key := r.CountryCode + ":" + r.RateType
		oldRate, existed := oldMap[key]

		if !existed {
			// New rate that did not exist before.
			changes = append(changes, RateChange{
				CountryCode: r.CountryCode,
				RateType:    r.RateType,
				OldRate:     decimal.Zero,
				NewRate:     r.Rate,
			})
			continue
		}

		if !oldRate.Equal(r.Rate) {
			changes = append(changes, RateChange{
				CountryCode: r.CountryCode,
				RateType:    r.RateType,
				OldRate:     oldRate,
				NewRate:     r.Rate,
			})
		}
	}

	return changes
}

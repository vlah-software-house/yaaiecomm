package vat

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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

// tedbEndpoint is the EC TEDB SOAP service endpoint for VAT rate retrieval.
const tedbEndpoint = "https://ec.europa.eu/taxation_customs/tedb/ws/VatRetrievalService"

// tedbSOAPAction is the SOAP action header for the RetrieveVatRates operation.
const tedbSOAPAction = "urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService/retrieveVatRates"

// euMemberStates is the complete list of 27 EU member state codes (ISO 3166-1 alpha-2).
var euMemberStates = []string{
	"AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR",
	"DE", "GR", "HU", "IE", "IT", "LV", "LT", "LU", "MT", "NL",
	"PL", "PT", "RO", "SK", "SI", "ES", "SE",
}

// buildTEDBRequest constructs the SOAP XML envelope for the RetrieveVatRates request.
// The envelope requests rates for all 27 EU member states on the given date.
func buildTEDBRequest(date time.Time) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"`)
	buf.WriteString(` xmlns:urn="urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService">`)
	buf.WriteString(`<soapenv:Header/>`)
	buf.WriteString(`<soapenv:Body>`)
	buf.WriteString(`<urn:retrieveVatRatesReqMsg>`)
	buf.WriteString(`<urn:memberStates>`)
	for _, code := range euMemberStates {
		buf.WriteString(`<urn:memberState>`)
		buf.WriteString(code)
		buf.WriteString(`</urn:memberState>`)
	}
	buf.WriteString(`</urn:memberStates>`)
	buf.WriteString(`<urn:dateOfApplication>`)
	buf.WriteString(date.Format("2006-01-02"))
	buf.WriteString(`</urn:dateOfApplication>`)
	buf.WriteString(`</urn:retrieveVatRatesReqMsg>`)
	buf.WriteString(`</soapenv:Body>`)
	buf.WriteString(`</soapenv:Envelope>`)
	return buf.Bytes()
}

// tedbEnvelope represents the top-level SOAP envelope of the TEDB response.
type tedbEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    tedbBody `xml:"Body"`
}

// tedbBody represents the SOAP body containing the response message.
type tedbBody struct {
	Response tedbResponse `xml:"retrieveVatRatesRespMsg"`
	Fault    *tedbFault   `xml:"Fault"`
}

// tedbFault represents a SOAP fault returned by the service.
type tedbFault struct {
	FaultCode   string `xml:"faultcode"`
	FaultString string `xml:"faultstring"`
}

// tedbResponse represents the RetrieveVatRates response message.
type tedbResponse struct {
	MemberStates []tedbMemberState `xml:"vatRateResults>memberState"`
}

// tedbMemberState represents a single member state's VAT rate data.
type tedbMemberState struct {
	Code string      `xml:"code"`
	Name string      `xml:"name"`
	Rate []tedbRate  `xml:"rate"`
}

// tedbRate represents a single VAT rate entry from the TEDB response.
type tedbRate struct {
	Type  string  `xml:"type"`
	Value float64 `xml:"value"`
}

// mapTEDBRateType maps the EC TEDB rate type names to the internal rate type constants.
// TEDB uses uppercase names like "STANDARD", "REDUCED", "SUPER_REDUCED", etc.
func mapTEDBRateType(tedbType string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(tedbType)) {
	case "STANDARD":
		return RateTypeStandard, true
	case "REDUCED":
		return RateTypeReduced, true
	case "REDUCED_RATE", "REDUCED RATE":
		return RateTypeReduced, true
	case "REDUCED_ALT", "REDUCED ALT", "SECOND_REDUCED", "SECOND REDUCED":
		return RateTypeReducedAlt, true
	case "SUPER_REDUCED", "SUPER REDUCED", "SUPER-REDUCED":
		return RateTypeSuperReduced, true
	case "PARKING":
		return RateTypeParking, true
	case "ZERO":
		return RateTypeZero, true
	default:
		return "", false
	}
}

// parseTEDBResponse parses the SOAP XML response from the EC TEDB service
// and returns a slice of VATRate entries.
func parseTEDBResponse(data []byte) ([]VATRate, error) {
	var envelope tedbEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing TEDB XML response: %w", err)
	}

	// Check for SOAP fault.
	if envelope.Body.Fault != nil {
		return nil, fmt.Errorf("TEDB SOAP fault: %s: %s",
			envelope.Body.Fault.FaultCode,
			envelope.Body.Fault.FaultString,
		)
	}

	if len(envelope.Body.Response.MemberStates) == 0 {
		return nil, fmt.Errorf("TEDB response contains no member state data")
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var rates []VATRate

	for _, ms := range envelope.Body.Response.MemberStates {
		countryCode := strings.TrimSpace(ms.Code)
		if len(countryCode) != 2 {
			continue
		}
		countryCode = strings.ToUpper(countryCode)

		for _, rate := range ms.Rate {
			rateType, ok := mapTEDBRateType(rate.Type)
			if !ok {
				continue // Skip unknown rate types.
			}

			if rate.Value <= 0 {
				continue // Skip zero or negative rates.
			}

			rates = append(rates, VATRate{
				ID:          uuid.New().String(),
				CountryCode: countryCode,
				RateType:    rateType,
				Rate:        decimal.NewFromFloat(rate.Value),
				Description: fmt.Sprintf("%s rate for %s", rateType, ms.Name),
				ValidFrom:   today,
				ValidTo:     nil,
				Source:      SourceECTEDB,
				SyncedAt:    now,
			})
		}
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("no valid VAT rates parsed from TEDB response")
	}

	return rates, nil
}

// fetchFromTEDB fetches VAT rates from the EC TEDB SOAP service.
//
// The TEDB (Taxes in Europe Database) is maintained by the European Commission
// and provides the official VAT rates for all EU member states. The service uses
// SOAP/XML for communication.
//
// The request asks for rates applicable on the current date for all 27 EU member
// states. The response is parsed into internal VATRate structs.
//
// If the service is unavailable or returns an error, the existing fallback chain
// in Sync() will try euvatrates.com JSON and then the database cache.
func (s *RateSyncer) fetchFromTEDB(ctx context.Context) ([]VATRate, error) {
	// Build the SOAP envelope.
	envelope := buildTEDBRequest(time.Now().UTC())

	// Create the HTTP request with SOAP-specific headers.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tedbEndpoint, bytes.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("creating TEDB SOAP request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", tedbSOAPAction)
	req.Header.Set("User-Agent", "ForgeCommerce/1.0 (VAT rate sync)")

	// Execute the request using the configured HTTP client (respects TEDBTimeout).
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling TEDB SOAP service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TEDB SOAP service returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading TEDB response body: %w", err)
	}

	rates, err := parseTEDBResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parsing TEDB response: %w", err)
	}

	s.logger.Info("parsed TEDB SOAP response",
		"member_states", len(euMemberStates),
		"total_rates", len(rates),
	)

	return rates, nil
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

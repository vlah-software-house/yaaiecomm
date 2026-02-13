package vat

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VIESClient validates EU VAT numbers against the VIES SOAP service.
// Results are cached in the database with a configurable TTL.
type VIESClient struct {
	pool     *pgxpool.Pool
	client   *http.Client
	logger   *slog.Logger
	cacheTTL time.Duration
}

// NewVIESClient creates a new VIES validation client.
func NewVIESClient(pool *pgxpool.Pool, timeout, cacheTTL time.Duration, logger *slog.Logger) *VIESClient {
	return &VIESClient{
		pool: pool,
		client: &http.Client{
			Timeout: timeout,
		},
		logger:   logger,
		cacheTTL: cacheTTL,
	}
}

const viesEndpoint = "https://ec.europa.eu/taxation_customs/vies/services/checkVatService"

// viesSOAPEnvelope is the SOAP request template for VIES VAT number validation.
const viesSOAPEnvelope = `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
                  xmlns:urn="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
  <soapenv:Body>
    <urn:checkVat>
      <urn:countryCode>%s</urn:countryCode>
      <urn:vatNumber>%s</urn:vatNumber>
    </urn:checkVat>
  </soapenv:Body>
</soapenv:Envelope>`

// viesSOAPResponse represents the XML structure of the VIES SOAP response.
type viesSOAPResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		CheckVatResponse struct {
			CountryCode string `xml:"countryCode"`
			VATNumber   string `xml:"vatNumber"`
			RequestDate string `xml:"requestDate"`
			Valid       bool   `xml:"valid"`
			Name        string `xml:"name"`
			Address     string `xml:"address"`
		} `xml:"checkVatResponse"`
	} `xml:"Body"`
}

// Validate checks a VAT number against VIES. It first checks the database
// cache; if missing or expired, it makes a live SOAP call and caches the result.
//
// The vatNumber should include the country prefix (e.g., "ES12345678A").
func (c *VIESClient) Validate(ctx context.Context, vatNumber string) (VIESResult, error) {
	cleaned := sanitizeVATNumber(vatNumber)
	if len(cleaned) < 4 {
		return VIESResult{Valid: false}, fmt.Errorf("VAT number too short: %q", vatNumber)
	}

	countryCode := cleaned[:2]
	number := cleaned[2:]

	// Check cache first.
	cached, err := c.getFromCache(ctx, cleaned)
	if err == nil {
		c.logger.Debug("VIES cache hit", "vat_number", cleaned, "valid", cached.Valid)
		return cached, nil
	}

	// Cache miss or expired — make live SOAP call.
	c.logger.Info("VIES live validation", "country", countryCode, "number", number)

	result, err := c.callVIES(ctx, countryCode, number)
	if err != nil {
		return VIESResult{}, fmt.Errorf("VIES validation failed for %s: %w", cleaned, err)
	}

	result.CountryCode = countryCode
	result.VATNumber = cleaned

	// Cache the result.
	if cacheErr := c.saveToCache(ctx, result); cacheErr != nil {
		c.logger.Warn("failed to cache VIES result", "error", cacheErr, "vat_number", cleaned)
	}

	return result, nil
}

// callVIES performs the actual SOAP call to the EC VIES service.
func (c *VIESClient) callVIES(ctx context.Context, countryCode, number string) (VIESResult, error) {
	soapBody := fmt.Sprintf(viesSOAPEnvelope, countryCode, number)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, viesEndpoint, strings.NewReader(soapBody))
	if err != nil {
		return VIESResult{}, fmt.Errorf("creating VIES request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "")

	resp, err := c.client.Do(req)
	if err != nil {
		return VIESResult{}, fmt.Errorf("calling VIES service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VIESResult{}, fmt.Errorf("reading VIES response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return VIESResult{}, fmt.Errorf("VIES returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var soapResp viesSOAPResponse
	if err := xml.Unmarshal(body, &soapResp); err != nil {
		return VIESResult{}, fmt.Errorf("parsing VIES response XML: %w", err)
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

// getFromCache looks up a VIES result from the database cache.
// Returns an error if no valid (non-expired) cache entry exists.
func (c *VIESClient) getFromCache(ctx context.Context, vatNumber string) (VIESResult, error) {
	var isValid bool
	var companyName, companyAddress, consultationNumber *string
	var expiresAt time.Time

	err := c.pool.QueryRow(ctx, `
		SELECT is_valid, company_name, company_address, consultation_number, expires_at
		FROM vies_validation_cache
		WHERE vat_number = $1
		LIMIT 1
	`, vatNumber).Scan(&isValid, &companyName, &companyAddress, &consultationNumber, &expiresAt)
	if err != nil {
		return VIESResult{}, err
	}

	if time.Now().UTC().After(expiresAt) {
		return VIESResult{}, fmt.Errorf("cache entry expired")
	}

	name := ""
	if companyName != nil {
		name = *companyName
	}
	address := ""
	if companyAddress != nil {
		address = *companyAddress
	}
	consNum := ""
	if consultationNumber != nil {
		consNum = *consultationNumber
	}

	return VIESResult{
		Valid:              isValid,
		CompanyName:        name,
		CompanyAddress:     address,
		ConsultationNumber: consNum,
		CountryCode:        vatNumber[:2],
		VATNumber:          vatNumber,
	}, nil
}

// saveToCache persists a VIES result to the database cache with the configured TTL.
func (c *VIESClient) saveToCache(ctx context.Context, result VIESResult) error {
	now := time.Now().UTC()
	expiresAt := now.Add(c.cacheTTL)

	_, err := c.pool.Exec(ctx, `
		INSERT INTO vies_validation_cache (vat_number, is_valid, company_name, company_address, consultation_number, validated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (vat_number) DO UPDATE SET
			is_valid = EXCLUDED.is_valid,
			company_name = EXCLUDED.company_name,
			company_address = EXCLUDED.company_address,
			consultation_number = EXCLUDED.consultation_number,
			validated_at = EXCLUDED.validated_at,
			expires_at = EXCLUDED.expires_at
	`, result.VATNumber, result.Valid, nilIfEmpty(result.CompanyName), nilIfEmpty(result.CompanyAddress),
		nilIfEmpty(result.ConsultationNumber), now, expiresAt)
	if err != nil {
		return fmt.Errorf("upserting VIES cache: %w", err)
	}

	return nil
}

// nilIfEmpty returns nil for empty strings, or a pointer to the string otherwise.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// generateConsultationNumber generates a unique consultation reference.
// In production, this comes from the VIES response's requestIdentifier field.
func generateConsultationNumber() string {
	return "FC-" + uuid.New().String()[:8]
}

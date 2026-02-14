package vat

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgecommerce/api/internal/config"
	"github.com/shopspring/decimal"
)

// sampleTEDBResponse is a realistic SOAP response from the EC TEDB service
// containing VAT rates for a few member states.
const sampleTEDBResponse = `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <retrieveVatRatesRespMsg xmlns="urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService">
      <vatRateResults>
        <memberState>
          <code>DE</code>
          <name>Germany</name>
          <rate>
            <type>STANDARD</type>
            <value>19</value>
          </rate>
          <rate>
            <type>REDUCED</type>
            <value>7</value>
          </rate>
        </memberState>
        <memberState>
          <code>FR</code>
          <name>France</name>
          <rate>
            <type>STANDARD</type>
            <value>20</value>
          </rate>
          <rate>
            <type>REDUCED</type>
            <value>5.5</value>
          </rate>
          <rate>
            <type>SUPER_REDUCED</type>
            <value>2.1</value>
          </rate>
          <rate>
            <type>REDUCED_ALT</type>
            <value>10</value>
          </rate>
        </memberState>
        <memberState>
          <code>ES</code>
          <name>Spain</name>
          <rate>
            <type>STANDARD</type>
            <value>21</value>
          </rate>
          <rate>
            <type>REDUCED</type>
            <value>10</value>
          </rate>
          <rate>
            <type>SUPER_REDUCED</type>
            <value>4</value>
          </rate>
        </memberState>
        <memberState>
          <code>DK</code>
          <name>Denmark</name>
          <rate>
            <type>STANDARD</type>
            <value>25</value>
          </rate>
        </memberState>
        <memberState>
          <code>BE</code>
          <name>Belgium</name>
          <rate>
            <type>STANDARD</type>
            <value>21</value>
          </rate>
          <rate>
            <type>REDUCED</type>
            <value>6</value>
          </rate>
          <rate>
            <type>PARKING</type>
            <value>12</value>
          </rate>
        </memberState>
      </vatRateResults>
    </retrieveVatRatesRespMsg>
  </soap:Body>
</soap:Envelope>`

func TestBuildTEDBRequest_WellFormedXML(t *testing.T) {
	date := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	envelope := buildTEDBRequest(date)

	// Verify the XML is well-formed by attempting to unmarshal.
	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(envelope, &doc); err != nil {
		t.Fatalf("buildTEDBRequest produced malformed XML: %v", err)
	}

	xmlStr := string(envelope)

	// Verify it contains the correct date.
	if !strings.Contains(xmlStr, "2026-02-14") {
		t.Error("SOAP envelope does not contain the expected date 2026-02-14")
	}

	// Verify it contains the SOAP namespace.
	if !strings.Contains(xmlStr, "http://schemas.xmlsoap.org/soap/envelope/") {
		t.Error("SOAP envelope missing SOAP namespace")
	}

	// Verify it contains the TEDB namespace.
	if !strings.Contains(xmlStr, "urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService") {
		t.Error("SOAP envelope missing TEDB namespace")
	}

	// Verify all 27 EU member states are included.
	for _, code := range euMemberStates {
		tag := fmt.Sprintf("<urn:memberState>%s</urn:memberState>", code)
		if !strings.Contains(xmlStr, tag) {
			t.Errorf("SOAP envelope missing member state %s", code)
		}
	}
}

func TestBuildTEDBRequest_Contains27MemberStates(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	envelope := buildTEDBRequest(date)
	xmlStr := string(envelope)

	count := strings.Count(xmlStr, "<urn:memberState>")
	if count != 27 {
		t.Errorf("expected 27 member states, found %d", count)
	}
}

func TestParseTEDBResponse_ValidResponse(t *testing.T) {
	rates, err := parseTEDBResponse([]byte(sampleTEDBResponse))
	if err != nil {
		t.Fatalf("parseTEDBResponse failed: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("parseTEDBResponse returned no rates")
	}

	// Build a lookup for easy verification.
	rateMap := make(map[string]decimal.Decimal)
	for _, r := range rates {
		key := r.CountryCode + ":" + r.RateType
		rateMap[key] = r.Rate
	}

	// Verify specific rates.
	expected := map[string]float64{
		"DE:standard":      19.0,
		"DE:reduced":       7.0,
		"FR:standard":      20.0,
		"FR:reduced":       5.5,
		"FR:super_reduced": 2.1,
		"FR:reduced_alt":   10.0,
		"ES:standard":      21.0,
		"ES:reduced":       10.0,
		"ES:super_reduced": 4.0,
		"DK:standard":      25.0,
		"BE:standard":      21.0,
		"BE:reduced":       6.0,
		"BE:parking":       12.0,
	}

	for key, expectedVal := range expected {
		got, ok := rateMap[key]
		if !ok {
			t.Errorf("missing rate for %s", key)
			continue
		}
		want := decimal.NewFromFloat(expectedVal)
		if !got.Equal(want) {
			t.Errorf("rate for %s: want %s, got %s", key, want.String(), got.String())
		}
	}

	// Verify metadata fields.
	for _, r := range rates {
		if r.ID == "" {
			t.Error("rate missing ID")
		}
		if r.Source != SourceECTEDB {
			t.Errorf("rate source: want %q, got %q", SourceECTEDB, r.Source)
		}
		if r.ValidTo != nil {
			t.Error("rate should have nil ValidTo (currently active)")
		}
		if r.Description == "" {
			t.Error("rate missing description")
		}
	}
}

func TestParseTEDBResponse_TotalRateCount(t *testing.T) {
	rates, err := parseTEDBResponse([]byte(sampleTEDBResponse))
	if err != nil {
		t.Fatalf("parseTEDBResponse failed: %v", err)
	}

	// Sample has: DE(2) + FR(4) + ES(3) + DK(1) + BE(3) = 13 rates.
	if len(rates) != 13 {
		t.Errorf("expected 13 rates, got %d", len(rates))
	}
}

func TestParseTEDBResponse_SOAPFault(t *testing.T) {
	faultXML := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Server</faultcode>
      <faultstring>Internal server error: database unavailable</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`

	_, err := parseTEDBResponse([]byte(faultXML))
	if err == nil {
		t.Fatal("expected error for SOAP fault, got nil")
	}
	if !strings.Contains(err.Error(), "SOAP fault") {
		t.Errorf("error should mention SOAP fault, got: %v", err)
	}
}

func TestParseTEDBResponse_EmptyMemberStates(t *testing.T) {
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <retrieveVatRatesRespMsg xmlns="urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService">
      <vatRateResults>
      </vatRateResults>
    </retrieveVatRatesRespMsg>
  </soap:Body>
</soap:Envelope>`

	_, err := parseTEDBResponse([]byte(emptyXML))
	if err == nil {
		t.Fatal("expected error for empty member states, got nil")
	}
	if !strings.Contains(err.Error(), "no member state data") {
		t.Errorf("error should mention no member state data, got: %v", err)
	}
}

func TestParseTEDBResponse_MalformedXML(t *testing.T) {
	_, err := parseTEDBResponse([]byte(`<not valid xml`))
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestParseTEDBResponse_UnknownRateTypesSkipped(t *testing.T) {
	xmlWithUnknown := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <retrieveVatRatesRespMsg xmlns="urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService">
      <vatRateResults>
        <memberState>
          <code>DE</code>
          <name>Germany</name>
          <rate>
            <type>STANDARD</type>
            <value>19</value>
          </rate>
          <rate>
            <type>UNKNOWN_FUTURE_TYPE</type>
            <value>15</value>
          </rate>
          <rate>
            <type>EXPERIMENTAL</type>
            <value>3</value>
          </rate>
        </memberState>
      </vatRateResults>
    </retrieveVatRatesRespMsg>
  </soap:Body>
</soap:Envelope>`

	rates, err := parseTEDBResponse([]byte(xmlWithUnknown))
	if err != nil {
		t.Fatalf("parseTEDBResponse failed: %v", err)
	}

	// Only STANDARD should be parsed; unknown types skipped.
	if len(rates) != 1 {
		t.Errorf("expected 1 rate (standard only), got %d", len(rates))
	}
	if len(rates) > 0 && rates[0].RateType != RateTypeStandard {
		t.Errorf("expected standard rate type, got %q", rates[0].RateType)
	}
}

func TestParseTEDBResponse_ZeroAndNegativeRatesSkipped(t *testing.T) {
	xmlWithZero := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <retrieveVatRatesRespMsg xmlns="urn:ec.europa.eu:taxud:tedb:services:v1:IVatRetrievalService">
      <vatRateResults>
        <memberState>
          <code>DE</code>
          <name>Germany</name>
          <rate>
            <type>STANDARD</type>
            <value>19</value>
          </rate>
          <rate>
            <type>REDUCED</type>
            <value>0</value>
          </rate>
          <rate>
            <type>PARKING</type>
            <value>-1</value>
          </rate>
        </memberState>
      </vatRateResults>
    </retrieveVatRatesRespMsg>
  </soap:Body>
</soap:Envelope>`

	rates, err := parseTEDBResponse([]byte(xmlWithZero))
	if err != nil {
		t.Fatalf("parseTEDBResponse failed: %v", err)
	}

	if len(rates) != 1 {
		t.Errorf("expected 1 rate (standard only, zero/negative skipped), got %d", len(rates))
	}
}

func TestMapTEDBRateType(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantOK   bool
	}{
		{"STANDARD", RateTypeStandard, true},
		{"standard", RateTypeStandard, true},
		{"Standard", RateTypeStandard, true},
		{"REDUCED", RateTypeReduced, true},
		{"REDUCED_RATE", RateTypeReduced, true},
		{"REDUCED RATE", RateTypeReduced, true},
		{"REDUCED_ALT", RateTypeReducedAlt, true},
		{"SECOND_REDUCED", RateTypeReducedAlt, true},
		{"SECOND REDUCED", RateTypeReducedAlt, true},
		{"SUPER_REDUCED", RateTypeSuperReduced, true},
		{"SUPER REDUCED", RateTypeSuperReduced, true},
		{"SUPER-REDUCED", RateTypeSuperReduced, true},
		{"PARKING", RateTypeParking, true},
		{"ZERO", RateTypeZero, true},
		{"  STANDARD  ", RateTypeStandard, true}, // with whitespace
		{"UNKNOWN", "", false},
		{"", "", false},
		{"CUSTOM_TYPE", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotType, gotOK := mapTEDBRateType(tt.input)
			if gotOK != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, gotOK)
			}
			if gotType != tt.wantType {
				t.Errorf("type: want %q, got %q", tt.wantType, gotType)
			}
		})
	}
}

func TestFetchFromTEDB_HTTPIntegration(t *testing.T) {
	// Create a mock TEDB server that returns the sample response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request has correct headers.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/xml") {
			t.Errorf("expected Content-Type text/xml, got %s", contentType)
		}
		soapAction := r.Header.Get("SOAPAction")
		if soapAction == "" {
			t.Error("missing SOAPAction header")
		}

		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleTEDBResponse))
	}))
	defer server.Close()

	// Create a RateSyncer that points to our mock server.
	syncer := &RateSyncer{
		cfg: config.VATConfig{
			TEDBTimeout: 10 * time.Second,
		},
		logger: slog.Default(),
		cache:  NewRateCache(),
		client: server.Client(),
	}

	// Override the endpoint for testing by using a custom fetchFromTEDBWithURL method.
	// Since fetchFromTEDB uses the const tedbEndpoint, we test the full pipeline
	// by hitting the mock server directly.
	rates, err := fetchFromTEDBWithURL(context.Background(), syncer, server.URL)
	if err != nil {
		t.Fatalf("fetchFromTEDB failed: %v", err)
	}

	if len(rates) != 13 {
		t.Errorf("expected 13 rates, got %d", len(rates))
	}
}

// fetchFromTEDBWithURL is a test helper that performs the same logic as
// fetchFromTEDB but allows overriding the endpoint URL for mock server testing.
func fetchFromTEDBWithURL(ctx context.Context, s *RateSyncer, url string) ([]VATRate, error) {
	envelope := buildTEDBRequest(time.Now().UTC())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(envelope)))
	if err != nil {
		return nil, fmt.Errorf("creating TEDB SOAP request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", tedbSOAPAction)
	req.Header.Set("User-Agent", "ForgeCommerce/1.0 (VAT rate sync)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling TEDB SOAP service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TEDB SOAP service returned HTTP %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("reading TEDB response body: %w", err)
	}

	return parseTEDBResponse(buf.Bytes())
}

func TestFetchFromTEDB_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Service unavailable"))
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg: config.VATConfig{
			TEDBTimeout: 10 * time.Second,
		},
		logger: slog.Default(),
		cache:  NewRateCache(),
		client: server.Client(),
	}

	_, err := fetchFromTEDBWithURL(context.Background(), syncer, server.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention HTTP 500, got: %v", err)
	}
}

func TestFetchFromTEDB_Timeout(t *testing.T) {
	// Server that delays longer than the client timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleTEDBResponse))
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg: config.VATConfig{
			TEDBTimeout: 100 * time.Millisecond,
		},
		logger: slog.Default(),
		cache:  NewRateCache(),
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
	}

	_, err := fetchFromTEDBWithURL(context.Background(), syncer, server.URL)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestFetchFromTEDB_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<this is not valid xml at all`))
	}))
	defer server.Close()

	syncer := &RateSyncer{
		cfg: config.VATConfig{
			TEDBTimeout: 10 * time.Second,
		},
		logger: slog.Default(),
		cache:  NewRateCache(),
		client: server.Client(),
	}

	_, err := fetchFromTEDBWithURL(context.Background(), syncer, server.URL)
	if err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
}

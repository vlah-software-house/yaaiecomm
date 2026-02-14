package vat

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeVATNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already clean",
			input: "DE123456789",
			want:  "DE123456789",
		},
		{
			name:  "lowercase to uppercase",
			input: "de123456789",
			want:  "DE123456789",
		},
		{
			name:  "strips spaces",
			input: "DE 123 456 789",
			want:  "DE123456789",
		},
		{
			name:  "strips tabs and newlines",
			input: "DE\t123\n456\r789",
			want:  "DE123456789",
		},
		{
			name:  "strips dots",
			input: "ES.B.12345678",
			want:  "ESB12345678",
		},
		{
			name:  "mixed spaces and lowercase",
			input: " es b1234 5678 ",
			want:  "ESB12345678",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   \t  ",
			want:  "",
		},
		{
			name:  "French VAT number with spaces",
			input: "FR 12 345 678 901",
			want:  "FR12345678901",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeVATNumber(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeVATNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVIESResponse_ValidXML(t *testing.T) {
	// Construct a valid VIES SOAP response XML.
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <checkVatResponse xmlns="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
      <countryCode>ES</countryCode>
      <vatNumber>B12345678</vatNumber>
      <requestDate>2026-02-14+01:00</requestDate>
      <valid>true</valid>
      <name>Forja Comercio S.L.</name>
      <address>Calle Mayor 1, Madrid</address>
    </checkVatResponse>
  </soap:Body>
</soap:Envelope>`

	var soapResp viesSOAPResponse
	err := xml.Unmarshal([]byte(xmlBody), &soapResp)
	if err != nil {
		t.Fatalf("unexpected error parsing valid VIES XML: %v", err)
	}

	data := soapResp.Body.CheckVatResponse
	if !data.Valid {
		t.Error("expected Valid=true, got false")
	}
	if data.CountryCode != "ES" {
		t.Errorf("expected CountryCode ES, got %q", data.CountryCode)
	}
	if data.VATNumber != "B12345678" {
		t.Errorf("expected VATNumber B12345678, got %q", data.VATNumber)
	}
	if strings.TrimSpace(data.Name) != "Forja Comercio S.L." {
		t.Errorf("expected company name %q, got %q", "Forja Comercio S.L.", data.Name)
	}
	if strings.TrimSpace(data.Address) != "Calle Mayor 1, Madrid" {
		t.Errorf("expected address %q, got %q", "Calle Mayor 1, Madrid", data.Address)
	}
}

func TestParseVIESResponse_InvalidVATNumber(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <checkVatResponse xmlns="urn:ec.europa.eu:taxud:vies:services:checkVat:types">
      <countryCode>DE</countryCode>
      <vatNumber>000000000</vatNumber>
      <requestDate>2026-02-14+01:00</requestDate>
      <valid>false</valid>
      <name>---</name>
      <address>---</address>
    </checkVatResponse>
  </soap:Body>
</soap:Envelope>`

	var soapResp viesSOAPResponse
	err := xml.Unmarshal([]byte(xmlBody), &soapResp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if soapResp.Body.CheckVatResponse.Valid {
		t.Error("expected Valid=false for invalid VAT number")
	}
}

func TestParseVIESResponse_MalformedXML(t *testing.T) {
	xmlBody := `<not>valid</xml`

	var soapResp viesSOAPResponse
	err := xml.Unmarshal([]byte(xmlBody), &soapResp)
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

func TestBuildSOAPEnvelope(t *testing.T) {
	countryCode := "ES"
	number := "B12345678"

	envelope := fmt.Sprintf(viesSOAPEnvelope, countryCode, number)

	if !strings.Contains(envelope, "<urn:countryCode>ES</urn:countryCode>") {
		t.Error("SOAP envelope missing country code element")
	}
	if !strings.Contains(envelope, "<urn:vatNumber>B12345678</urn:vatNumber>") {
		t.Error("SOAP envelope missing VAT number element")
	}
	if !strings.Contains(envelope, "soapenv:Envelope") {
		t.Error("SOAP envelope missing Envelope wrapper")
	}
	if !strings.Contains(envelope, "checkVat") {
		t.Error("SOAP envelope missing checkVat action")
	}
}

func TestNilIfEmpty(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool // true = expect nil
	}{
		{"empty string returns nil", "", true},
		{"non-empty returns pointer", "hello", false},
		{"single char returns pointer", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nilIfEmpty(tt.in)
			if tt.want && got != nil {
				t.Errorf("expected nil for empty string, got %q", *got)
			}
			if !tt.want {
				if got == nil {
					t.Errorf("expected non-nil for %q", tt.in)
				} else if *got != tt.in {
					t.Errorf("expected %q, got %q", tt.in, *got)
				}
			}
		})
	}
}

func TestGenerateConsultationNumber(t *testing.T) {
	num := generateConsultationNumber()
	if !strings.HasPrefix(num, "FC-") {
		t.Errorf("expected consultation number to start with 'FC-', got %q", num)
	}
	if len(num) != 11 { // "FC-" (3) + 8 hex chars = 11
		t.Errorf("expected consultation number length 11, got %d (%q)", len(num), num)
	}

	// Generate two and verify they are different (extremely unlikely to collide).
	num2 := generateConsultationNumber()
	if num == num2 {
		t.Errorf("expected unique consultation numbers, got two identical: %q", num)
	}
}

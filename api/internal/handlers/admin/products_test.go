package admin

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// --------------------------------------------------------------------------
// Tests for helper functions in products.go
// --------------------------------------------------------------------------

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{"valid integer", "42", true},
		{"valid decimal", "19.99", true},
		{"empty string", "", false},
		{"whitespace only", "   ", false},
		{"zero", "0", true},
		{"negative", "-5.50", true},
		{"invalid text", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := parseNumeric(tt.input)
			if n.Valid != tt.isValid {
				t.Errorf("parseNumeric(%q).Valid = %v, want %v", tt.input, n.Valid, tt.isValid)
			}
		})
	}
}

func TestFormatNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Numeric
		want  string
	}{
		{"invalid returns empty", pgtype.Numeric{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNumeric(tt.input)
			if got != tt.want {
				t.Errorf("formatNumeric() = %q, want %q", got, tt.want)
			}
		})
	}

	// Valid numeric round-trip.
	n := parseNumeric("19.99")
	s := formatNumeric(n)
	if s == "" {
		t.Error("formatNumeric of valid numeric should not be empty")
	}
}

func TestParseInt32(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int32
	}{
		{"valid", "42", 42},
		{"zero", "0", 0},
		{"negative", "-10", -10},
		{"empty", "", 0},
		{"whitespace", "  ", 0},
		{"invalid", "abc", 0},
		{"float", "3.14", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt32(tt.input)
			if got != tt.want {
				t.Errorf("parseInt32(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatInt32(t *testing.T) {
	tests := []struct {
		name  string
		input int32
		want  string
	}{
		{"zero returns empty", 0, ""},
		{"positive", 42, "42"},
		{"negative", -5, "-5"},
		{"large", 9999, "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatInt32(tt.input)
			if got != tt.want {
				t.Errorf("formatInt32(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStrPtr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{"empty returns nil", "", true},
		{"whitespace returns nil", "   ", true},
		{"non-empty returns pointer", "hello", false},
		{"trimmed", "  hello  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strPtr(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("strPtr(%q) = %q, want nil", tt.input, *got)
				}
			} else {
				if got == nil {
					t.Fatalf("strPtr(%q) = nil, want non-nil", tt.input)
				}
			}
		})
	}
}

func TestDerefString(t *testing.T) {
	s := "hello"
	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{"nil returns empty", nil, ""},
		{"non-nil returns value", &s, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derefString(tt.input)
			if got != tt.want {
				t.Errorf("derefString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProductToFormData(t *testing.T) {
	p := db.Product{
		ID:     uuid.New(),
		Name:   "Test Product",
		Slug:   "test-product",
		Status: "draft",
	}

	data := productToFormData(p, "csrf_123")
	if data.ID != p.ID.String() {
		t.Errorf("ID = %q, want %q", data.ID, p.ID.String())
	}
	if data.Name != "Test Product" {
		t.Errorf("Name = %q, want %q", data.Name, "Test Product")
	}
	if data.IsNew {
		t.Error("IsNew should be false for existing product")
	}
	if data.CSRFToken != "csrf_123" {
		t.Errorf("CSRFToken = %q, want %q", data.CSRFToken, "csrf_123")
	}
	if data.Status != "draft" {
		t.Errorf("Status = %q, want %q", data.Status, "draft")
	}
}

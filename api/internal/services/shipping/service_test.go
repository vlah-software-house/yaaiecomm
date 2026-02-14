package shipping

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// --------------------------------------------------------------------------
// Tests for pure calculation functions
// --------------------------------------------------------------------------

func TestCalculateWeightBasedFee(t *testing.T) {
	brackets := []WeightBracket{
		{MinWeightG: 0, MaxWeightG: 500, Fee: decimal.NewFromFloat(5.00)},
		{MinWeightG: 501, MaxWeightG: 1000, Fee: decimal.NewFromFloat(8.50)},
		{MinWeightG: 1001, MaxWeightG: 5000, Fee: decimal.NewFromFloat(12.00)},
		{MinWeightG: 5001, MaxWeightG: 0, Fee: decimal.NewFromFloat(20.00)}, // open-ended
	}
	ratesJSON, _ := json.Marshal(brackets)

	tests := []struct {
		name    string
		weight  int
		want    decimal.Decimal
		wantErr bool
	}{
		{
			name:   "under 500g - first bracket",
			weight: 250,
			want:   decimal.NewFromFloat(5.00),
		},
		{
			name:   "exactly 500g - first bracket",
			weight: 500,
			want:   decimal.NewFromFloat(5.00),
		},
		{
			name:   "501g - second bracket",
			weight: 501,
			want:   decimal.NewFromFloat(8.50),
		},
		{
			name:   "between brackets - 750g",
			weight: 750,
			want:   decimal.NewFromFloat(8.50),
		},
		{
			name:   "1001g - third bracket",
			weight: 1001,
			want:   decimal.NewFromFloat(12.00),
		},
		{
			name:   "heavy - 10000g - open-ended bracket",
			weight: 10000,
			want:   decimal.NewFromFloat(20.00),
		},
		{
			name:   "zero weight",
			weight: 0,
			want:   decimal.NewFromFloat(5.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateWeightBasedFee(ratesJSON, tt.weight)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("calculateWeightBasedFee(%d) = %s, want %s", tt.weight, got.String(), tt.want.String())
			}
		})
	}
}

func TestCalculateWeightBasedFee_NoMatchingBracket(t *testing.T) {
	// Brackets with gaps: 0-100, 500-1000.
	brackets := []WeightBracket{
		{MinWeightG: 0, MaxWeightG: 100, Fee: decimal.NewFromFloat(5.00)},
		{MinWeightG: 500, MaxWeightG: 1000, Fee: decimal.NewFromFloat(10.00)},
	}
	ratesJSON, _ := json.Marshal(brackets)

	_, err := calculateWeightBasedFee(ratesJSON, 250)
	if err != ErrNoMatchingBracket {
		t.Errorf("expected ErrNoMatchingBracket, got %v", err)
	}
}

func TestCalculateWeightBasedFee_EmptyRates(t *testing.T) {
	_, err := calculateWeightBasedFee(nil, 500)
	if err != ErrNoMatchingBracket {
		t.Errorf("expected ErrNoMatchingBracket for nil rates, got %v", err)
	}

	_, err = calculateWeightBasedFee(json.RawMessage(`[]`), 500)
	if err != ErrNoMatchingBracket {
		t.Errorf("expected ErrNoMatchingBracket for empty brackets, got %v", err)
	}
}

func TestCalculateWeightBasedFee_InvalidJSON(t *testing.T) {
	_, err := calculateWeightBasedFee(json.RawMessage(`{invalid`), 500)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestCalculateSizeBasedFee(t *testing.T) {
	tests := []struct {
		name    string
		rate    SizeRate
		weight  int
		want    decimal.Decimal
		wantErr bool
	}{
		{
			name: "base fee + per kg - 2500g rounds up to 3kg",
			rate: SizeRate{
				BaseFee:  decimal.NewFromFloat(3.00),
				PerKgFee: decimal.NewFromFloat(1.50),
				MinFee:   decimal.NewFromFloat(5.00),
			},
			weight: 2500,
			want:   decimal.NewFromFloat(7.50), // 3.00 + 1.50 * 3 = 7.50
		},
		{
			name: "minimum fee applied",
			rate: SizeRate{
				BaseFee:  decimal.NewFromFloat(1.00),
				PerKgFee: decimal.NewFromFloat(0.50),
				MinFee:   decimal.NewFromFloat(5.00),
			},
			weight: 100, // rounds to 1 kg: 1.00 + 0.50 * 1 = 1.50 < 5.00
			want:   decimal.NewFromFloat(5.00),
		},
		{
			name: "exact 1000g = 1kg",
			rate: SizeRate{
				BaseFee:  decimal.NewFromFloat(2.00),
				PerKgFee: decimal.NewFromFloat(3.00),
				MinFee:   decimal.NewFromFloat(0.00),
			},
			weight: 1000,
			want:   decimal.NewFromFloat(5.00), // 2.00 + 3.00 * 1
		},
		{
			name: "zero weight rounds up to 0kg",
			rate: SizeRate{
				BaseFee:  decimal.NewFromFloat(2.00),
				PerKgFee: decimal.NewFromFloat(3.00),
				MinFee:   decimal.NewFromFloat(0.00),
			},
			weight: 0,
			want:   decimal.NewFromFloat(2.00), // 2.00 + 3.00 * 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratesJSON, _ := json.Marshal(tt.rate)
			got, err := calculateSizeBasedFee(ratesJSON, tt.weight)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("calculateSizeBasedFee(%d) = %s, want %s", tt.weight, got.String(), tt.want.String())
			}
		})
	}
}

func TestCalculateSizeBasedFee_EmptyRates(t *testing.T) {
	_, err := calculateSizeBasedFee(nil, 500)
	if err == nil {
		t.Fatal("expected error for empty size rates, got nil")
	}
}

func TestIsCountryEnabled(t *testing.T) {
	enabled := []db.EuCountry{
		{CountryCode: "DE"},
		{CountryCode: "ES"},
		{CountryCode: "FR"},
	}

	tests := []struct {
		code string
		want bool
	}{
		{"DE", true},
		{"ES", true},
		{"FR", true},
		{"IT", false},
		{"XX", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := isCountryEnabled(tt.code, enabled)
			if got != tt.want {
				t.Errorf("isCountryEnabled(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestIsCountryEnabled_EmptyList(t *testing.T) {
	got := isCountryEnabled("DE", nil)
	if got {
		t.Error("expected false for empty enabled list")
	}
}

func TestNumericToDecimal(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Numeric
		want  decimal.Decimal
	}{
		{
			name: "21.00",
			input: pgtype.Numeric{
				Int:   big.NewInt(2100),
				Exp:   -2,
				Valid: true,
			},
			want: decimal.NewFromFloat(21.00),
		},
		{
			name:  "invalid returns zero",
			input: pgtype.Numeric{Valid: false},
			want:  decimal.Zero,
		},
		{
			name:  "nil Int returns zero",
			input: pgtype.Numeric{Valid: true},
			want:  decimal.Zero,
		},
		{
			name: "integer 100",
			input: pgtype.Numeric{
				Int:   big.NewInt(100),
				Exp:   0,
				Valid: true,
			},
			want: decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numericToDecimal(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("numericToDecimal() = %s, want %s", got.String(), tt.want.String())
			}
		})
	}
}

func TestDecimalToNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input decimal.Decimal
	}{
		{"21.00", decimal.NewFromFloat(21.00)},
		{"0.50", decimal.NewFromFloat(0.50)},
		{"0", decimal.Zero},
		{"999.99", decimal.NewFromFloat(999.99)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := decimalToNumeric(tt.input)
			if !n.Valid {
				t.Fatal("expected Valid=true")
			}
			// Round-trip test: convert back and compare.
			roundTrip := numericToDecimal(n)
			if !roundTrip.Equal(tt.input) {
				t.Errorf("round-trip failed: %s -> %s", tt.input.String(), roundTrip.String())
			}
		})
	}
}

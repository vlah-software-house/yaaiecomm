package discount

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// --------------------------------------------------------------------------
// Tests for pure functions: computeAmountCents, numericToCents, centsToNumeric,
// formatNumeric, numericZero
// --------------------------------------------------------------------------

func TestComputeAmountCents_Percentage(t *testing.T) {
	tests := []struct {
		name         string
		discountType string
		value        pgtype.Numeric // percentage value (e.g., 10.00 for 10%)
		baseCents    int64          // base amount in cents
		wantCents    int64          // expected discount in cents
	}{
		{
			name:         "10% off 242.00 (24200 cents)",
			discountType: "percentage",
			value:        makeNumeric(1000, -2), // 10.00
			baseCents:    24200,
			wantCents:    2420,
		},
		{
			name:         "50% off 100.00",
			discountType: "percentage",
			value:        makeNumeric(5000, -2), // 50.00
			baseCents:    10000,
			wantCents:    5000,
		},
		{
			name:         "100% off 50.00 (full discount)",
			discountType: "percentage",
			value:        makeNumeric(10000, -2), // 100.00
			baseCents:    5000,
			wantCents:    5000,
		},
		{
			name:         "5% off 33.33 (1666.5 -> truncated to 1666 cents)",
			discountType: "percentage",
			value:        makeNumeric(500, -2), // 5.00
			baseCents:    3333,
			wantCents:    166, // 3333 * 500 / 10000 = 166.65 -> truncated to 166
		},
		{
			name:         "percentage with zero base",
			discountType: "percentage",
			value:        makeNumeric(1000, -2),
			baseCents:    0,
			wantCents:    0,
		},
		{
			name:         "percentage with nil value",
			discountType: "percentage",
			value:        pgtype.Numeric{Valid: true}, // Int is nil
			baseCents:    10000,
			wantCents:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := db.Discount{
				Type:  tt.discountType,
				Value: tt.value,
			}
			base := big.NewInt(tt.baseCents)
			got := computeAmountCents(d, base)
			if got.Cmp(big.NewInt(tt.wantCents)) != 0 {
				t.Errorf("computeAmountCents() = %s, want %d", got.String(), tt.wantCents)
			}
		})
	}
}

func TestComputeAmountCents_FixedAmount(t *testing.T) {
	tests := []struct {
		name      string
		value     pgtype.Numeric // fixed amount in EUR
		baseCents int64          // base amount in cents
		wantCents int64          // expected discount in cents
	}{
		{
			name:      "5.00 off 100.00",
			value:     makeNumeric(500, -2), // 5.00
			baseCents: 10000,
			wantCents: 500,
		},
		{
			name:      "fixed amount exceeds base - capped",
			value:     makeNumeric(15000, -2), // 150.00
			baseCents: 10000,                  // 100.00
			wantCents: 10000,                  // capped at 100.00
		},
		{
			name:      "zero fixed amount",
			value:     makeNumeric(0, -2),
			baseCents: 10000,
			wantCents: 0,
		},
		{
			name:      "fixed amount on zero base",
			value:     makeNumeric(500, -2),
			baseCents: 0,
			wantCents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := db.Discount{
				Type:  "fixed_amount",
				Value: tt.value,
			}
			base := big.NewInt(tt.baseCents)
			got := computeAmountCents(d, base)
			if got.Cmp(big.NewInt(tt.wantCents)) != 0 {
				t.Errorf("computeAmountCents() = %s, want %d", got.String(), tt.wantCents)
			}
		})
	}
}

func TestComputeAmountCents_UnknownType(t *testing.T) {
	d := db.Discount{
		Type:  "unknown",
		Value: makeNumeric(1000, -2),
	}
	got := computeAmountCents(d, big.NewInt(10000))
	if got.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected 0 for unknown discount type, got %s", got.String())
	}
}

func TestNumericToCents(t *testing.T) {
	tests := []struct {
		name      string
		input     pgtype.Numeric
		wantCents int64
	}{
		{
			name:      "21.00 -> 2100 cents",
			input:     makeNumeric(2100, -2),
			wantCents: 2100,
		},
		{
			name:      "21 (exp 0) -> 2100 cents",
			input:     makeNumeric(21, 0),
			wantCents: 2100,
		},
		{
			name:      "5 (exp 0) -> 500 cents",
			input:     makeNumeric(5, 0),
			wantCents: 500,
		},
		{
			name:      "0.50 -> 50 cents",
			input:     makeNumeric(50, -2),
			wantCents: 50,
		},
		{
			name:      "invalid numeric -> 0",
			input:     pgtype.Numeric{Valid: false},
			wantCents: 0,
		},
		{
			name:      "nil Int -> 0",
			input:     pgtype.Numeric{Valid: true},
			wantCents: 0,
		},
		{
			name:      "high precision -> truncated",
			input:     makeNumeric(12345, -4), // 1.2345 -> 1 cent (123 / 100 -> truncated)
			wantCents: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numericToCents(tt.input)
			if got.Cmp(big.NewInt(tt.wantCents)) != 0 {
				t.Errorf("numericToCents() = %s, want %d", got.String(), tt.wantCents)
			}
		})
	}
}

func TestCentsToNumeric(t *testing.T) {
	tests := []struct {
		name    string
		cents   int64
		wantInt int64
		wantExp int32
	}{
		{"2100 cents", 2100, 2100, -2},
		{"0 cents", 0, 0, -2},
		{"1 cent", 1, 1, -2},
		{"100000 cents (1000.00)", 100000, 100000, -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := centsToNumeric(big.NewInt(tt.cents))
			if !got.Valid {
				t.Fatal("expected Valid=true")
			}
			if got.Int.Cmp(big.NewInt(tt.wantInt)) != 0 {
				t.Errorf("Int: got %s, want %d", got.Int.String(), tt.wantInt)
			}
			if got.Exp != tt.wantExp {
				t.Errorf("Exp: got %d, want %d", got.Exp, tt.wantExp)
			}
		})
	}
}

func TestCentsToNumeric_NilInput(t *testing.T) {
	got := centsToNumeric(nil)
	if !got.Valid {
		t.Fatal("expected Valid=true even for nil (defaults to zero)")
	}
	if got.Int.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected 0, got %s", got.Int.String())
	}
}

func TestNumericZero(t *testing.T) {
	z := numericZero()
	if !z.Valid {
		t.Fatal("expected Valid=true")
	}
	if z.Int.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected Int=0, got %s", z.Int.String())
	}
	if z.Exp != -2 {
		t.Errorf("expected Exp=-2, got %d", z.Exp)
	}
}

func TestFormatNumeric(t *testing.T) {
	tests := []struct {
		name  string
		input pgtype.Numeric
		want  string
	}{
		{
			name:  "21.00",
			input: makeNumeric(2100, -2),
			want:  "21.00",
		},
		{
			name:  "0.50",
			input: makeNumeric(50, -2),
			want:  "0.50",
		},
		{
			name:  "100 (exp 0)",
			input: makeNumeric(100, 0),
			want:  "100.00",
		},
		{
			name:  "invalid returns 0.00",
			input: pgtype.Numeric{Valid: false},
			want:  "0.00",
		},
		{
			name:  "nil Int returns 0.00",
			input: pgtype.Numeric{Valid: true},
			want:  "0.00",
		},
		{
			name:  "negative value",
			input: makeNumeric(-500, -2),
			want:  "-5.00",
		},
		{
			name:  "small fractional 0.01",
			input: makeNumeric(1, -2),
			want:  "0.01",
		},
		{
			name:  "large number 12345.67",
			input: makeNumeric(1234567, -2),
			want:  "12345.67",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNumeric(tt.input)
			if got != tt.want {
				t.Errorf("formatNumeric() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func makeNumeric(intVal int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(intVal),
		Exp:   exp,
		Valid: true,
	}
}

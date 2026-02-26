package report

import (
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// --------------------------------------------------------------------------
// Tests for toNumeric
// --------------------------------------------------------------------------

func TestToNumeric_Nil(t *testing.T) {
	n := toNumeric(nil)
	if n.Valid {
		t.Error("expected invalid Numeric for nil input")
	}
}

func TestToNumeric_PgNumeric(t *testing.T) {
	input := pgtype.Numeric{Int: big.NewInt(4250), Exp: -2, Valid: true}
	n := toNumeric(input)
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	if n.Int.Int64() != 4250 || n.Exp != -2 {
		t.Errorf("got Int=%d Exp=%d, want 4250 / -2", n.Int.Int64(), n.Exp)
	}
}

func TestToNumeric_PgNumericPointer(t *testing.T) {
	input := &pgtype.Numeric{Int: big.NewInt(1000), Exp: -2, Valid: true}
	n := toNumeric(input)
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	if n.Int.Int64() != 1000 {
		t.Errorf("Int: got %d, want 1000", n.Int.Int64())
	}
}

func TestToNumeric_PgNumericPointerNil(t *testing.T) {
	var input *pgtype.Numeric
	n := toNumeric(input)
	if n.Valid {
		t.Error("expected invalid Numeric for nil pointer")
	}
}

func TestToNumeric_Int64(t *testing.T) {
	n := toNumeric(int64(42))
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	if n.Int.Int64() != 42 || n.Exp != 0 {
		t.Errorf("got Int=%d Exp=%d, want 42 / 0", n.Int.Int64(), n.Exp)
	}
}

func TestToNumeric_Int32(t *testing.T) {
	n := toNumeric(int32(7))
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	if n.Int.Int64() != 7 || n.Exp != 0 {
		t.Errorf("got Int=%d Exp=%d, want 7 / 0", n.Int.Int64(), n.Exp)
	}
}

func TestToNumeric_Float64(t *testing.T) {
	n := toNumeric(float64(42.50))
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	// 42.50 * 100 = 4250, Exp = -2
	if n.Int.Int64() != 4250 || n.Exp != -2 {
		t.Errorf("got Int=%d Exp=%d, want 4250 / -2", n.Int.Int64(), n.Exp)
	}
}

func TestToNumeric_String(t *testing.T) {
	n := toNumeric("99.99")
	if !n.Valid {
		t.Fatal("expected valid Numeric")
	}
	f, err := n.Float64Value()
	if err != nil {
		t.Fatalf("Float64Value: %v", err)
	}
	if f.Float64 != 99.99 {
		t.Errorf("got %f, want 99.99", f.Float64)
	}
}

func TestToNumeric_InvalidString(t *testing.T) {
	n := toNumeric("not-a-number")
	if n.Valid {
		t.Error("expected invalid Numeric for bad string")
	}
}

func TestToNumeric_ByteSlice(t *testing.T) {
	// []byte falls through to the default case's Scan; pgtype.Numeric.Scan
	// does not accept []byte, so it returns invalid.
	n := toNumeric([]byte("123.45"))
	if n.Valid {
		t.Error("expected invalid Numeric for []byte (Scan does not accept []byte)")
	}
}

func TestToNumeric_UnsupportedType(t *testing.T) {
	n := toNumeric(struct{}{})
	if n.Valid {
		t.Error("expected invalid Numeric for unsupported type")
	}
}

// --------------------------------------------------------------------------
// Tests for pgDateToTime
// --------------------------------------------------------------------------

func TestPgDateToTime_Valid(t *testing.T) {
	want := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC)
	d := pgtype.Date{Time: want, Valid: true}
	got := pgDateToTime(d)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPgDateToTime_Invalid(t *testing.T) {
	d := pgtype.Date{Valid: false}
	got := pgDateToTime(d)
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid date, got %v", got)
	}
}

// --------------------------------------------------------------------------
// Tests for numericToFloat (additional edge cases)
// --------------------------------------------------------------------------

func TestNumericToFloat_PointerNil(t *testing.T) {
	var n *pgtype.Numeric
	got := numericToFloat(n)
	if got != 0 {
		t.Errorf("got %f, want 0", got)
	}
}

func TestNumericToFloat_PointerValid(t *testing.T) {
	n := &pgtype.Numeric{Int: big.NewInt(2150), Exp: -2, Valid: true}
	got := numericToFloat(n)
	if got != 21.50 {
		t.Errorf("got %f, want 21.50", got)
	}
}

func TestNumericToFloat_PointerInvalid(t *testing.T) {
	n := &pgtype.Numeric{Valid: false}
	got := numericToFloat(n)
	if got != 0 {
		t.Errorf("got %f, want 0 for invalid pointer", got)
	}
}

func TestNumericToFloat_Int32(t *testing.T) {
	got := numericToFloat(int32(42))
	if got != 42 {
		t.Errorf("got %f, want 42", got)
	}
}

func TestNumericToFloat_ByteSlice(t *testing.T) {
	// []byte falls through to the default case; Scan does not accept []byte.
	got := numericToFloat([]byte("55.55"))
	if got != 0 {
		t.Errorf("got %f, want 0 for []byte (Scan rejects []byte)", got)
	}
}

func TestNumericToFloat_String(t *testing.T) {
	// string falls to the default case; pgtype.Numeric.Scan accepts string.
	got := numericToFloat("42.50")
	if got != 42.50 {
		t.Errorf("got %f, want 42.50", got)
	}
}

func TestNumericToFloat_InvalidString(t *testing.T) {
	// An invalid string still falls to default case, Scan fails.
	got := numericToFloat("not-a-number")
	if got != 0 {
		t.Errorf("got %f, want 0 for invalid string", got)
	}
}

func TestNumericToFloat_InvalidNumeric(t *testing.T) {
	// A valid pgtype.Numeric but with Valid=false.
	got := numericToFloat(pgtype.Numeric{Valid: false})
	if got != 0 {
		t.Errorf("got %f, want 0 for invalid Numeric value", got)
	}
}

func TestNumericToFloat_Float64(t *testing.T) {
	got := numericToFloat(float64(99.99))
	if got != 99.99 {
		t.Errorf("got %f, want 99.99", got)
	}
}

func TestNumericToFloat_Int64(t *testing.T) {
	got := numericToFloat(int64(100))
	if got != 100 {
		t.Errorf("got %f, want 100", got)
	}
}

func TestNumericToFloat_Nil(t *testing.T) {
	got := numericToFloat(nil)
	if got != 0 {
		t.Errorf("got %f, want 0 for nil", got)
	}
}

func TestNumericToFloat_ValidNumeric(t *testing.T) {
	n := pgtype.Numeric{Int: big.NewInt(9999), Exp: -2, Valid: true}
	got := numericToFloat(n)
	if got != 99.99 {
		t.Errorf("got %f, want 99.99", got)
	}
}

func TestNumericToFloat_UnsupportedType(t *testing.T) {
	got := numericToFloat(struct{}{})
	if got != 0 {
		t.Errorf("got %f, want 0 for unsupported type", got)
	}
}

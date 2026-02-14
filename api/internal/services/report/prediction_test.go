package report

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// --------------------------------------------------------------------------
// Tests for pure prediction functions
// --------------------------------------------------------------------------

func TestWeightedMovingAverageField_Empty(t *testing.T) {
	result := weightedMovingAverageField(nil, time.Monday, func(d DailyMetrics) float64 {
		return numericToFloat(d.GrossRevenue)
	})
	if result != 0 {
		t.Errorf("expected 0 for empty data, got %f", result)
	}
}

func TestWeightedMovingAverageField_SingleDay(t *testing.T) {
	data := []DailyMetrics{
		{
			Date:         time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC), // Saturday
			GrossRevenue: makeTestNumeric(100.0),
		},
	}

	result := weightedMovingAverageField(data, time.Saturday, func(d DailyMetrics) float64 {
		return numericToFloat(d.GrossRevenue)
	})

	// With single data point matching DOW and boost 1.5x, the WMA = value itself.
	if result != 100.0 {
		t.Errorf("expected 100.0, got %f", result)
	}
}

func TestWeightedMovingAverageField_DOWBoost(t *testing.T) {
	// Create 7 days of data. Monday=100, all others=10.
	// When predicting for Monday, Monday value should be boosted.
	baseDate := time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC) // Monday
	data := make([]DailyMetrics, 7)
	for i := 0; i < 7; i++ {
		d := baseDate.AddDate(0, 0, i)
		val := 10.0
		if d.Weekday() == time.Monday {
			val = 100.0
		}
		data[i] = DailyMetrics{
			Date:         d,
			GrossRevenue: makeTestNumeric(val),
		}
	}

	resultMon := weightedMovingAverageField(data, time.Monday, func(d DailyMetrics) float64 {
		return numericToFloat(d.GrossRevenue)
	})
	resultTue := weightedMovingAverageField(data, time.Tuesday, func(d DailyMetrics) float64 {
		return numericToFloat(d.GrossRevenue)
	})

	// Monday prediction should be higher than Tuesday because Monday value (100)
	// gets a 1.5x boost when targeting Monday.
	if resultMon <= resultTue {
		t.Errorf("Monday prediction (%f) should be > Tuesday prediction (%f) due to DOW boost", resultMon, resultTue)
	}
}

func TestYoyAdjusted(t *testing.T) {
	tests := []struct {
		name          string
		lastYearValue float64
		recentAvg     float64
		lastYearAvg   float64
		want          float64
	}{
		{
			name:          "no growth (multiplier=1)",
			lastYearValue: 100,
			recentAvg:     50,
			lastYearAvg:   50,
			want:          100,
		},
		{
			name:          "double growth",
			lastYearValue: 100,
			recentAvg:     100,
			lastYearAvg:   50,
			want:          200, // 100 * (100/50) = 200
		},
		{
			name:          "half decline",
			lastYearValue: 100,
			recentAvg:     25,
			lastYearAvg:   50,
			want:          50, // 100 * (25/50) = 50
		},
		{
			name:          "lastYearAvg zero returns raw value",
			lastYearValue: 100,
			recentAvg:     50,
			lastYearAvg:   0,
			want:          100, // no adjustment possible
		},
		{
			name:          "zero last year value",
			lastYearValue: 0,
			recentAvg:     100,
			lastYearAvg:   50,
			want:          0, // 0 * anything = 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yoyAdjusted(tt.lastYearValue, tt.recentAvg, tt.lastYearAvg)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("yoyAdjusted() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		name          string
		hasPrevYear   bool
		dataPoints    int
		wantMin       float64
		wantMax       float64
		wantExact     float64
		checkExact    bool
	}{
		{
			name:       "full data with prev year",
			hasPrevYear: true,
			dataPoints:  28,
			wantExact:  0.95, // 0.5 + 1.0*0.45
			checkExact: true,
		},
		{
			name:       "no data with prev year",
			hasPrevYear: true,
			dataPoints:  0,
			wantExact:  0.5, // 0.5 + 0*0.45
			checkExact: true,
		},
		{
			name:       "half data with prev year",
			hasPrevYear: true,
			dataPoints:  14,
			wantExact:  0.725, // 0.5 + 0.5*0.45
			checkExact: true,
		},
		{
			name:       "full data without prev year",
			hasPrevYear: false,
			dataPoints:  28,
			wantExact:  0.7, // 0.1 + 1.0*0.6
			checkExact: true,
		},
		{
			name:       "no data without prev year",
			hasPrevYear: false,
			dataPoints:  0,
			wantExact:  0.1, // 0.1 + 0*0.6
			checkExact: true,
		},
		{
			name:       "overflow data points capped at 1.0",
			hasPrevYear: false,
			dataPoints:  56, // more than windowSize
			wantExact:  0.7, // 0.1 + 1.0*0.6 (capped)
			checkExact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateConfidence(tt.hasPrevYear, tt.dataPoints)
			if tt.checkExact {
				if math.Abs(got-tt.wantExact) > 0.001 {
					t.Errorf("calculateConfidence() = %f, want %f", got, tt.wantExact)
				}
			} else {
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("calculateConfidence() = %f, want [%f, %f]", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestNumericToFloat(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want float64
	}{
		{
			name: "pgtype.Numeric 100.50",
			val: pgtype.Numeric{
				Int:   big.NewInt(10050),
				Exp:   -2,
				Valid: true,
			},
			want: 100.50,
		},
		{
			name: "invalid pgtype.Numeric",
			val:  pgtype.Numeric{Valid: false},
			want: 0,
		},
		{
			name: "float64",
			val:  float64(42.5),
			want: 42.5,
		},
		{
			name: "int64",
			val:  int64(100),
			want: 100.0,
		},
		{
			name: "int32",
			val:  int32(50),
			want: 50.0,
		},
		{
			name: "nil",
			val:  nil,
			want: 0,
		},
		{
			name: "pointer to valid Numeric",
			val: &pgtype.Numeric{
				Int:   big.NewInt(5000),
				Exp:   -2,
				Valid: true,
			},
			want: 50.0,
		},
		{
			name: "nil pointer to Numeric",
			val:  (*pgtype.Numeric)(nil),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numericToFloat(tt.val)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("numericToFloat() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestIndexByDate(t *testing.T) {
	d1 := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC)

	data := []DailyMetrics{
		{Date: d1, OrderCount: 5},
		{Date: d2, OrderCount: 3},
	}

	m := indexByDate(data)
	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}

	if m[d1].OrderCount != 5 {
		t.Errorf("expected OrderCount 5 for d1, got %d", m[d1].OrderCount)
	}
}

func TestFillWindow(t *testing.T) {
	startDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	byDate := map[time.Time]DailyMetrics{
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC): {Date: startDate, OrderCount: 10},
		time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC): {Date: time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC), OrderCount: 5},
	}

	result := fillWindow(startDate, 5, byDate)
	if len(result) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(result))
	}

	// Day 0 (Feb 1) has data.
	if result[0].OrderCount != 10 {
		t.Errorf("expected OrderCount 10 for Feb 1, got %d", result[0].OrderCount)
	}

	// Day 1 (Feb 2) has no data (zero-filled).
	if result[1].OrderCount != 0 {
		t.Errorf("expected OrderCount 0 for Feb 2, got %d", result[1].OrderCount)
	}

	// Day 2 (Feb 3) has data.
	if result[2].OrderCount != 5 {
		t.Errorf("expected OrderCount 5 for Feb 3, got %d", result[2].OrderCount)
	}
}

func TestCountNonZeroDays(t *testing.T) {
	data := []DailyMetrics{
		{OrderCount: 5},
		{OrderCount: 0},
		{OrderCount: 3},
		{OrderCount: 0},
		{OrderCount: 1},
	}

	got := countNonZeroDays(data)
	if got != 3 {
		t.Errorf("expected 3 non-zero days, got %d", got)
	}
}

func TestCountNonZeroDays_AllZero(t *testing.T) {
	data := []DailyMetrics{
		{OrderCount: 0},
		{OrderCount: 0},
	}

	got := countNonZeroDays(data)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestAvgGross(t *testing.T) {
	tests := []struct {
		name string
		data []DailyMetrics
		want float64
	}{
		{
			name: "empty returns 0",
			data: nil,
			want: 0,
		},
		{
			name: "single value",
			data: []DailyMetrics{
				{GrossRevenue: makeTestNumeric(100.0)},
			},
			want: 100.0,
		},
		{
			name: "average of three",
			data: []DailyMetrics{
				{GrossRevenue: makeTestNumeric(100.0)},
				{GrossRevenue: makeTestNumeric(200.0)},
				{GrossRevenue: makeTestNumeric(300.0)},
			},
			want: 200.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := avgGross(tt.data)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("avgGross() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestTruncateDay(t *testing.T) {
	input := time.Date(2026, 2, 14, 15, 30, 45, 123, time.UTC)
	expected := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)

	got := truncateDay(input)
	if !got.Equal(expected) {
		t.Errorf("truncateDay() = %v, want %v", got, expected)
	}
}

func TestLookupDate(t *testing.T) {
	d := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	byDate := map[time.Time]DailyMetrics{
		d: {Date: d, OrderCount: 42},
	}

	// Found.
	got := lookupDate(d, byDate)
	if got.OrderCount != 42 {
		t.Errorf("expected OrderCount 42, got %d", got.OrderCount)
	}

	// Not found - returns zero value.
	notFound := lookupDate(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), byDate)
	if notFound.OrderCount != 0 {
		t.Errorf("expected OrderCount 0 for missing date, got %d", notFound.OrderCount)
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

func makeTestNumeric(val float64) pgtype.Numeric {
	scaled := int64(val * 100)
	return pgtype.Numeric{
		Int:   big.NewInt(scaled),
		Exp:   -2,
		Valid: true,
	}
}

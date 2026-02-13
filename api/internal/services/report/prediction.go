package report

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// PredictionResult holds the predicted revenue for a single future date.
type PredictionResult struct {
	Date           time.Time `json:"date"`
	PredictedGross float64   `json:"predicted_gross"`
	PredictedNet   float64   `json:"predicted_net"`
	Confidence     float64   `json:"confidence"` // 0.0-1.0 based on data availability
	Method         string    `json:"method"`     // "yoy_blended" or "wma_only"
}

// windowSize is the number of trailing days used for the weighted moving average.
const windowSize = 28

// PredictSales generates revenue predictions for the next numDays days.
//
// Algorithm (from CLAUDE.md):
//
//	IF has_previous_year_data:
//	    prediction = 0.6 * yoy_adjusted + 0.4 * weighted_moving_average_28d
//	ELSE:
//	    prediction = weighted_moving_average_28d_day_of_week_adjusted
func (s *Service) PredictSales(ctx context.Context, numDays int) ([]PredictionResult, error) {
	if numDays <= 0 {
		return nil, fmt.Errorf("numDays must be positive, got %d", numDays)
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Fetch last 28 days of data (today exclusive, so [today-28, today)).
	recentFrom := today.AddDate(0, 0, -windowSize)
	recentReport, err := s.GetSalesReport(ctx, recentFrom, today)
	if err != nil {
		return nil, fmt.Errorf("fetching recent sales data: %w", err)
	}

	// Build a map of date -> DailyMetrics for quick lookup.
	recentByDate := indexByDate(recentReport.DailyData)

	// Fill in zero-value entries for days with no orders so the window is complete.
	recentDaily := fillWindow(recentFrom, windowSize, recentByDate)

	// Fetch the same 28-day window from one year ago, plus enough to cover the
	// prediction range so we have YoY values for each target date.
	lastYearFrom := recentFrom.AddDate(-1, 0, 0)
	lastYearTo := today.AddDate(-1, 0, numDays)
	lastYearReport, err := s.GetSalesReport(ctx, lastYearFrom, lastYearTo)
	if err != nil {
		return nil, fmt.Errorf("fetching last year sales data: %w", err)
	}

	lastYearByDate := indexByDate(lastYearReport.DailyData)

	// Last year's 28-day window aligned with the recent window.
	lastYearWindowFrom := recentFrom.AddDate(-1, 0, 0)
	lastYearWindow := fillWindow(lastYearWindowFrom, windowSize, lastYearByDate)

	hasPrevYear := countNonZeroDays(lastYearWindow) >= 7

	// Compute the trend multiplier: current 28d avg / last year same 28d avg.
	// This captures overall growth or decline year-over-year.
	recentAvgGross := avgGross(recentDaily)
	lastYearAvgGross := avgGross(lastYearWindow)

	results := make([]PredictionResult, 0, numDays)

	for i := 1; i <= numDays; i++ {
		targetDate := today.AddDate(0, 0, i)
		targetDOW := targetDate.Weekday()

		wmaGross := weightedMovingAverage(recentDaily, targetDOW)
		wmaNet := weightedMovingAverageNet(recentDaily, targetDOW)

		var predGross, predNet float64
		var method string

		if hasPrevYear {
			// Look up last year's value for the same calendar date.
			lyDate := targetDate.AddDate(-1, 0, 0)
			lyMetrics := lookupDate(lyDate, lastYearByDate)

			lyGross := numericToFloat(lyMetrics.GrossRevenue)
			lyNet := numericToFloat(lyMetrics.NetRevenue)

			adjGross := yoyAdjusted(lyGross, recentAvgGross, lastYearAvgGross)
			adjNet := yoyAdjusted(lyNet, recentAvgGross, lastYearAvgGross)

			predGross = 0.6*adjGross + 0.4*wmaGross
			predNet = 0.6*adjNet + 0.4*wmaNet
			method = "yoy_blended"
		} else {
			predGross = wmaGross
			predNet = wmaNet
			method = "wma_only"
		}

		// Clamp to zero: predictions should never be negative.
		predGross = math.Max(predGross, 0)
		predNet = math.Max(predNet, 0)

		// Round to 2 decimal places.
		predGross = math.Round(predGross*100) / 100
		predNet = math.Round(predNet*100) / 100

		confidence := calculateConfidence(hasPrevYear, countNonZeroDays(recentDaily))

		results = append(results, PredictionResult{
			Date:           targetDate,
			PredictedGross: predGross,
			PredictedNet:   predNet,
			Confidence:     confidence,
			Method:         method,
		})
	}

	return results, nil
}

// weightedMovingAverage computes a weighted moving average of gross revenue
// over the given daily data slice, using exponential decay weights and giving
// 1.5x extra weight to entries that fall on the same day of week as targetDOW.
func weightedMovingAverage(dailyData []DailyMetrics, targetDOW time.Weekday) float64 {
	return weightedMovingAverageField(dailyData, targetDOW, func(d DailyMetrics) float64 {
		return numericToFloat(d.GrossRevenue)
	})
}

// weightedMovingAverageNet is the same as weightedMovingAverage but for net revenue.
func weightedMovingAverageNet(dailyData []DailyMetrics, targetDOW time.Weekday) float64 {
	return weightedMovingAverageField(dailyData, targetDOW, func(d DailyMetrics) float64 {
		return numericToFloat(d.NetRevenue)
	})
}

// weightedMovingAverageField computes a weighted moving average using an
// exponential decay across the window, with 1.5x extra weight for entries
// matching targetDOW. The extract function pulls the desired numeric field.
func weightedMovingAverageField(dailyData []DailyMetrics, targetDOW time.Weekday, extract func(DailyMetrics) float64) float64 {
	n := len(dailyData)
	if n == 0 {
		return 0
	}

	// Decay factor: the most recent day has the highest weight.
	// weight_i = exp(-lambda * (n - 1 - i)) where i=0 is the oldest day.
	// A lambda of 0.05 gives roughly 4x more weight to the most recent day
	// vs the oldest in a 28-day window.
	const lambda = 0.05
	const dowBoost = 1.5

	var weightedSum, totalWeight float64
	for i, d := range dailyData {
		age := float64(n - 1 - i) // 0 for most recent, n-1 for oldest
		w := math.Exp(-lambda * age)

		if d.Date.Weekday() == targetDOW {
			w *= dowBoost
		}

		weightedSum += extract(d) * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// yoyAdjusted takes last year's value for the same date and adjusts it by the
// recent trend. The trend is the ratio of the current 28-day average gross
// to the previous year's 28-day average gross for the same period.
//
// If lastYearAvg is zero (no prior year data for the window), the raw
// lastYearValue is returned without adjustment.
func yoyAdjusted(lastYearValue float64, recentAvg float64, lastYearAvg float64) float64 {
	if lastYearAvg == 0 {
		// Cannot compute trend ratio; return the raw historical value.
		return lastYearValue
	}
	trendMultiplier := recentAvg / lastYearAvg
	return lastYearValue * trendMultiplier
}

// calculateConfidence returns a confidence score from 0.0 to 1.0 based on data
// completeness. Having previous-year data and more non-zero data points both
// increase confidence.
func calculateConfidence(hasPrevYear bool, dataPointCount int) float64 {
	// Base: proportion of the 28-day window that has real data, capped at 1.0.
	base := float64(dataPointCount) / float64(windowSize)
	if base > 1.0 {
		base = 1.0
	}

	if hasPrevYear {
		// With YoY data the model is more reliable: scale the base from
		// a floor of 0.5 up to 0.95.
		return 0.5 + base*0.45
	}

	// WMA-only: scale from 0.1 up to 0.7.
	return 0.1 + base*0.6
}

// numericToFloat converts a pgtype.Numeric (or an interface{} that may hold one)
// to a float64. Returns 0 for nil or invalid values.
func numericToFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}

	switch val := v.(type) {
	case pgtype.Numeric:
		if !val.Valid {
			return 0
		}
		f, err := val.Float64Value()
		if err != nil || !f.Valid {
			return 0
		}
		return f.Float64
	case *pgtype.Numeric:
		if val == nil || !val.Valid {
			return 0
		}
		f, err := val.Float64Value()
		if err != nil || !f.Valid {
			return 0
		}
		return f.Float64
	case float64:
		return val
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		// Try to interpret as pgtype.Numeric via Scan.
		var n pgtype.Numeric
		if err := n.Scan(val); err != nil {
			return 0
		}
		if !n.Valid {
			return 0
		}
		f, err := n.Float64Value()
		if err != nil || !f.Valid {
			return 0
		}
		return f.Float64
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// indexByDate builds a map from date (truncated to midnight UTC) to DailyMetrics.
func indexByDate(data []DailyMetrics) map[time.Time]DailyMetrics {
	m := make(map[time.Time]DailyMetrics, len(data))
	for _, d := range data {
		key := truncateDay(d.Date)
		m[key] = d
	}
	return m
}

// fillWindow returns a slice of exactly numDays DailyMetrics starting from
// startDate. Days without data get a zero-value entry with the correct Date.
func fillWindow(startDate time.Time, numDays int, byDate map[time.Time]DailyMetrics) []DailyMetrics {
	out := make([]DailyMetrics, 0, numDays)
	for i := 0; i < numDays; i++ {
		d := startDate.AddDate(0, 0, i)
		key := truncateDay(d)
		if m, ok := byDate[key]; ok {
			out = append(out, m)
		} else {
			out = append(out, DailyMetrics{Date: key})
		}
	}
	return out
}

// lookupDate returns the DailyMetrics for a date, or a zero-value entry if not found.
func lookupDate(date time.Time, byDate map[time.Time]DailyMetrics) DailyMetrics {
	key := truncateDay(date)
	if m, ok := byDate[key]; ok {
		return m
	}
	return DailyMetrics{Date: key}
}

// countNonZeroDays counts how many entries in the slice had at least one order.
func countNonZeroDays(data []DailyMetrics) int {
	count := 0
	for _, d := range data {
		if d.OrderCount > 0 {
			count++
		}
	}
	return count
}

// avgGross returns the arithmetic mean of gross revenue across the slice.
func avgGross(data []DailyMetrics) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, d := range data {
		sum += numericToFloat(d.GrossRevenue)
	}
	return sum / float64(len(data))
}

// truncateDay returns the date truncated to midnight UTC.
func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

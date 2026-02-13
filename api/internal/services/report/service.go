package report

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// SalesReport contains daily sales data and a summary for a date range.
type SalesReport struct {
	From      time.Time
	To        time.Time
	Summary   SalesSummary
	DailyData []DailyMetrics
}

// SalesSummary contains aggregate metrics for a date range.
type SalesSummary struct {
	OrderCount        int64
	NetRevenue        pgtype.Numeric
	VATCollected      pgtype.Numeric
	GrossRevenue      pgtype.Numeric
	TotalDiscounts    pgtype.Numeric
	AverageOrderValue pgtype.Numeric
}

// DailyMetrics contains sales metrics for a single day.
type DailyMetrics struct {
	Date           time.Time
	OrderCount     int64
	NetRevenue     pgtype.Numeric
	VATCollected   pgtype.Numeric
	GrossRevenue   pgtype.Numeric
	TotalDiscounts pgtype.Numeric
}

// VATReport contains per-country VAT breakdown and reverse charge data.
type VATReport struct {
	From                time.Time
	To                  time.Time
	CountryBreakdown    []VATCountryLine
	ReverseCharge       VATReverseChargeSummary
	ReverseChargeOrders []VATReverseChargeOrder
}

// VATCountryLine represents a single row in the per-country VAT breakdown.
type VATCountryLine struct {
	CountryCode  string
	CountryName  string
	RateType     string
	Rate         pgtype.Numeric
	NetSales     pgtype.Numeric
	VATCollected pgtype.Numeric
	GrossSales   pgtype.Numeric
	OrderCount   int64
}

// VATReverseChargeSummary contains aggregate B2B reverse charge metrics.
type VATReverseChargeSummary struct {
	OrderCount int64
	TotalNet   pgtype.Numeric
}

// VATReverseChargeOrder represents a single B2B reverse charge order.
type VATReverseChargeOrder struct {
	ID          uuid.UUID
	OrderNumber int64
	Email       string
	VATNumber   *string
	CompanyName *string
	CountryCode *string
	NetTotal    pgtype.Numeric
	CreatedAt   time.Time
}

// TopProduct represents a product ranked by revenue.
type TopProduct struct {
	ProductName   string
	TotalQuantity int64
	TotalRevenue  pgtype.Numeric
}

// Service provides business logic for sales and VAT reporting.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new report service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		pool:    pool,
		logger:  logger,
	}
}

// GetSalesReport returns daily sales data and a summary for the given date range.
func (s *Service) GetSalesReport(ctx context.Context, from, to time.Time) (*SalesReport, error) {
	params := db.SalesReportDailyParams{
		FromDate: from,
		ToDate:   to,
	}

	dailyRows, err := s.queries.SalesReportDaily(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("querying daily sales report: %w", err)
	}

	daily := make([]DailyMetrics, 0, len(dailyRows))
	for _, row := range dailyRows {
		daily = append(daily, DailyMetrics{
			Date:           pgDateToTime(row.ReportDate),
			OrderCount:     row.OrderCount,
			NetRevenue:     toNumeric(row.NetRevenue),
			VATCollected:   toNumeric(row.VatCollected),
			GrossRevenue:   toNumeric(row.GrossRevenue),
			TotalDiscounts: toNumeric(row.TotalDiscounts),
		})
	}

	summaryParams := db.SalesReportSummaryParams{
		FromDate: from,
		ToDate:   to,
	}

	summaryRow, err := s.queries.SalesReportSummary(ctx, summaryParams)
	if err != nil {
		return nil, fmt.Errorf("querying sales report summary: %w", err)
	}

	summary := SalesSummary{
		OrderCount:        summaryRow.OrderCount,
		NetRevenue:        toNumeric(summaryRow.NetRevenue),
		VATCollected:      toNumeric(summaryRow.VatCollected),
		GrossRevenue:      toNumeric(summaryRow.GrossRevenue),
		TotalDiscounts:    toNumeric(summaryRow.TotalDiscounts),
		AverageOrderValue: toNumericFromInt32(summaryRow.AverageOrderValue),
	}

	return &SalesReport{
		From:      from,
		To:        to,
		Summary:   summary,
		DailyData: daily,
	}, nil
}

// GetVATReport returns per-country VAT breakdown and reverse charge data for the given date range.
func (s *Service) GetVATReport(ctx context.Context, from, to time.Time) (*VATReport, error) {
	countryParams := db.VATReportByCountryParams{
		FromDate: from,
		ToDate:   to,
	}

	countryRows, err := s.queries.VATReportByCountry(ctx, countryParams)
	if err != nil {
		return nil, fmt.Errorf("querying VAT report by country: %w", err)
	}

	breakdown := make([]VATCountryLine, 0, len(countryRows))
	for _, row := range countryRows {
		countryCode := ""
		if row.CountryCode != nil {
			countryCode = *row.CountryCode
		}
		countryName := ""
		if row.CountryName != nil {
			countryName = *row.CountryName
		}
		rateType := ""
		if row.RateType != nil {
			rateType = *row.RateType
		}

		breakdown = append(breakdown, VATCountryLine{
			CountryCode:  countryCode,
			CountryName:  countryName,
			RateType:     rateType,
			Rate:         row.Rate,
			NetSales:     toNumeric(row.NetSales),
			VATCollected: toNumeric(row.VatCollected),
			GrossSales:   toNumeric(row.GrossSales),
			OrderCount:   row.OrderCount,
		})
	}

	rcSummaryParams := db.VATReverseChargeSummaryParams{
		FromDate: from,
		ToDate:   to,
	}

	rcSummaryRow, err := s.queries.VATReverseChargeSummary(ctx, rcSummaryParams)
	if err != nil {
		return nil, fmt.Errorf("querying reverse charge summary: %w", err)
	}

	rcSummary := VATReverseChargeSummary{
		OrderCount: rcSummaryRow.OrderCount,
		TotalNet:   toNumeric(rcSummaryRow.TotalNet),
	}

	rcParams := db.VATReverseChargeReportParams{
		FromDate: from,
		ToDate:   to,
	}

	rcRows, err := s.queries.VATReverseChargeReport(ctx, rcParams)
	if err != nil {
		return nil, fmt.Errorf("querying reverse charge orders: %w", err)
	}

	rcOrders := make([]VATReverseChargeOrder, 0, len(rcRows))
	for _, row := range rcRows {
		rcOrders = append(rcOrders, VATReverseChargeOrder{
			ID:          row.ID,
			OrderNumber: row.OrderNumber,
			Email:       row.Email,
			VATNumber:   row.VatNumber,
			CompanyName: row.VatCompanyName,
			CountryCode: row.VatCountryCode,
			NetTotal:    row.NetTotal,
			CreatedAt:   row.CreatedAt,
		})
	}

	return &VATReport{
		From:                from,
		To:                  to,
		CountryBreakdown:    breakdown,
		ReverseCharge:       rcSummary,
		ReverseChargeOrders: rcOrders,
	}, nil
}

// GetTopProducts returns the top-selling products by revenue for the given date range.
func (s *Service) GetTopProducts(ctx context.Context, from, to time.Time, limit int32) ([]TopProduct, error) {
	params := db.TopProductsByRevenueParams{
		FromDate:   from,
		ToDate:     to,
		MaxResults: limit,
	}

	rows, err := s.queries.TopProductsByRevenue(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("querying top products by revenue: %w", err)
	}

	products := make([]TopProduct, 0, len(rows))
	for _, row := range rows {
		products = append(products, TopProduct{
			ProductName:   row.ProductName,
			TotalQuantity: row.TotalQuantity,
			TotalRevenue:  toNumeric(row.TotalRevenue),
		})
	}

	return products, nil
}

// toNumeric converts an interface{} value (as returned by sqlc for COALESCE/SUM
// expressions) into a pgtype.Numeric. It handles pgtype.Numeric directly, numeric
// strings, and common numeric Go types.
func toNumeric(v interface{}) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{Valid: false}
	}

	switch val := v.(type) {
	case pgtype.Numeric:
		return val
	case *pgtype.Numeric:
		if val == nil {
			return pgtype.Numeric{Valid: false}
		}
		return *val
	case int64:
		return pgtype.Numeric{
			Int:   big.NewInt(val),
			Exp:   0,
			Valid: true,
		}
	case int32:
		return pgtype.Numeric{
			Int:   big.NewInt(int64(val)),
			Exp:   0,
			Valid: true,
		}
	case float64:
		// Convert float64 to a scaled integer representation.
		// Multiply by 100 to preserve 2 decimal places.
		scaled := int64(val * 100)
		return pgtype.Numeric{
			Int:   big.NewInt(scaled),
			Exp:   -2,
			Valid: true,
		}
	case string:
		var n pgtype.Numeric
		if err := n.Scan(val); err != nil {
			return pgtype.Numeric{Valid: false}
		}
		return n
	default:
		// Try Scan as a last resort (handles []byte etc.)
		var n pgtype.Numeric
		if err := n.Scan(val); err != nil {
			return pgtype.Numeric{Valid: false}
		}
		return n
	}
}

// toNumericFromInt32 converts an int32 value to a pgtype.Numeric.
func toNumericFromInt32(v int32) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(v)),
		Exp:   0,
		Valid: true,
	}
}

// pgDateToTime converts a pgtype.Date to a time.Time.
func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

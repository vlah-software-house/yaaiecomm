package admin

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/report"
)

// ReportHandler handles admin report endpoints.
type ReportHandler struct {
	reportSvc *report.Service
	logger    *slog.Logger
}

// NewReportHandler creates a new report handler.
func NewReportHandler(reportSvc *report.Service, logger *slog.Logger) *ReportHandler {
	return &ReportHandler{
		reportSvc: reportSvc,
		logger:    logger,
	}
}

// RegisterRoutes registers report admin routes on the given mux.
func (h *ReportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/reports/sales", h.SalesReportPage)
	mux.HandleFunc("GET /admin/reports/sales/data", h.SalesReportData)
	mux.HandleFunc("GET /admin/reports/sales/csv", h.SalesReportCSV)
	mux.HandleFunc("GET /admin/reports/vat", h.VATReportPage)
	mux.HandleFunc("GET /admin/reports/vat/data", h.VATReportData)
	mux.HandleFunc("GET /admin/reports/vat/csv", h.VATReportCSV)
}

// parseDateRange extracts "from" and "to" query parameters in YYYY-MM-DD format.
// Defaults to the current month if not provided.
func parseDateRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := time.Parse("2006-01-02", f); err == nil {
			from = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			// Set "to" to the start of the next day so the range is inclusive of the end date.
			to = parsed.AddDate(0, 0, 1)
		}
	}

	return from, to
}

// parseVATPeriod interprets the "period" query parameter (monthly, quarterly, yearly)
// and returns the corresponding from/to range. Falls back to parseDateRange if no
// period is specified.
func parseVATPeriod(r *http.Request) (time.Time, time.Time) {
	period := r.URL.Query().Get("period")
	now := time.Now().UTC()

	switch period {
	case "quarterly":
		quarter := (now.Month() - 1) / 3
		from := time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(0, 3, 0)
		return from, to
	case "yearly":
		from := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		return from, to
	case "monthly":
		return parseDateRange(r)
	default:
		// No period specified; use from/to params or default to current month.
		return parseDateRange(r)
	}
}

// formatNumericValue formats a pgtype.Numeric as a plain decimal string (e.g. "1234.56").
// Returns "0.00" if the value is not valid.
func formatNumericValue(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	bf := new(big.Float).SetInt(n.Int)
	if n.Exp < 0 {
		divisor := new(big.Float).SetInt(
			new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil),
		)
		bf.Quo(bf, divisor)
	} else if n.Exp > 0 {
		multiplier := new(big.Float).SetInt(
			new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil),
		)
		bf.Mul(bf, multiplier)
	}
	return bf.Text('f', 2)
}

// --- Sales Report ---

// SalesReportPage handles GET /admin/reports/sales.
// Renders the full page shell with HTMX attributes that load data from the data endpoint.
func (h *ReportHandler) SalesReportPage(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	// "to" was already shifted +1 day for the query; display the original end date.
	displayTo := to.AddDate(0, 0, -1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Sales Report - ForgeCommerce Admin</title>
  <script src="/static/js/htmx.min.js"></script>
  <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
<div class="container">
  <h1>Sales Report</h1>

  <form id="report-filter" method="get" action="/admin/reports/sales">
    <label for="from">From:</label>
    <input type="date" id="from" name="from" value="%s">
    <label for="to">To:</label>
    <input type="date" id="to" name="to" value="%s">
    <button type="submit">Apply</button>
    <a href="/admin/reports/sales/csv?from=%s&to=%s" class="btn btn-secondary">Export CSV</a>
  </form>

  <div id="report-data"
       hx-get="/admin/reports/sales/data?from=%s&to=%s"
       hx-trigger="load"
       hx-swap="innerHTML">
    <p>Loading report data...</p>
  </div>
</div>
</body>
</html>`,
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
	)
}

// SalesReportData handles GET /admin/reports/sales/data.
// Returns an HTML fragment with summary metrics and a daily data table.
func (h *ReportHandler) SalesReportData(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)

	salesReport, err := h.reportSvc.GetSalesReport(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get sales report", "error", err)
		http.Error(w, "Failed to load sales report", http.StatusInternalServerError)
		return
	}

	topProducts, err := h.reportSvc.GetTopProducts(r.Context(), from, to, 10)
	if err != nil {
		h.logger.Error("failed to get top products", "error", err)
		http.Error(w, "Failed to load top products", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	s := salesReport.Summary

	// Summary cards.
	fmt.Fprintf(w, `<div class="report-summary">
  <div class="summary-card">
    <span class="summary-label">Orders</span>
    <span class="summary-value">%d</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">Net Revenue</span>
    <span class="summary-value">&euro;%s</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">VAT Collected</span>
    <span class="summary-value">&euro;%s</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">Gross Revenue</span>
    <span class="summary-value">&euro;%s</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">Discounts</span>
    <span class="summary-value">&euro;%s</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">Avg Order Value</span>
    <span class="summary-value">&euro;%s</span>
  </div>
</div>`,
		s.OrderCount,
		formatNumericValue(s.NetRevenue),
		formatNumericValue(s.VATCollected),
		formatNumericValue(s.GrossRevenue),
		formatNumericValue(s.TotalDiscounts),
		formatNumericValue(s.AverageOrderValue),
	)

	// Daily breakdown table.
	fmt.Fprint(w, `<h2>Daily Breakdown</h2>
<table class="table">
  <thead>
    <tr>
      <th>Date</th>
      <th>Orders</th>
      <th>Net Revenue</th>
      <th>VAT Collected</th>
      <th>Gross Revenue</th>
      <th>Discounts</th>
    </tr>
  </thead>
  <tbody>`)

	if len(salesReport.DailyData) == 0 {
		fmt.Fprint(w, `<tr><td colspan="6" class="text-muted">No sales data for this period.</td></tr>`)
	} else {
		for _, d := range salesReport.DailyData {
			fmt.Fprintf(w,
				"<tr><td>%s</td><td>%d</td><td>&euro;%s</td><td>&euro;%s</td><td>&euro;%s</td><td>&euro;%s</td></tr>",
				d.Date.Format("2006-01-02"),
				d.OrderCount,
				formatNumericValue(d.NetRevenue),
				formatNumericValue(d.VATCollected),
				formatNumericValue(d.GrossRevenue),
				formatNumericValue(d.TotalDiscounts),
			)
		}
	}

	fmt.Fprint(w, `</tbody></table>`)

	// Top products table.
	fmt.Fprint(w, `<h2>Top Products by Revenue</h2>
<table class="table">
  <thead>
    <tr>
      <th>#</th>
      <th>Product</th>
      <th>Quantity Sold</th>
      <th>Revenue</th>
    </tr>
  </thead>
  <tbody>`)

	if len(topProducts) == 0 {
		fmt.Fprint(w, `<tr><td colspan="4" class="text-muted">No product data for this period.</td></tr>`)
	} else {
		for i, p := range topProducts {
			fmt.Fprintf(w,
				"<tr><td>%d</td><td>%s</td><td>%d</td><td>&euro;%s</td></tr>",
				i+1,
				p.ProductName,
				p.TotalQuantity,
				formatNumericValue(p.TotalRevenue),
			)
		}
	}

	fmt.Fprint(w, `</tbody></table>`)
}

// SalesReportCSV handles GET /admin/reports/sales/csv.
// Returns a CSV file download of the daily sales data.
func (h *ReportHandler) SalesReportCSV(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)

	salesReport, err := h.reportSvc.GetSalesReport(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get sales report for CSV", "error", err)
		http.Error(w, "Failed to generate CSV", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("sales-report-%s-to-%s.csv",
		from.Format("2006-01-02"),
		to.AddDate(0, 0, -1).Format("2006-01-02"),
	)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	// Header row.
	csvWriter.Write([]string{
		"date", "order_count", "net_revenue", "vat_collected", "gross_revenue", "discounts",
	})

	for _, d := range salesReport.DailyData {
		csvWriter.Write([]string{
			d.Date.Format("2006-01-02"),
			fmt.Sprintf("%d", d.OrderCount),
			formatNumericValue(d.NetRevenue),
			formatNumericValue(d.VATCollected),
			formatNumericValue(d.GrossRevenue),
			formatNumericValue(d.TotalDiscounts),
		})
	}
}

// --- VAT Report ---

// VATReportPage handles GET /admin/reports/vat.
// Renders the full page shell with HTMX attributes that load data from the data endpoint.
func (h *ReportHandler) VATReportPage(w http.ResponseWriter, r *http.Request) {
	from, to := parseVATPeriod(r)
	displayTo := to.AddDate(0, 0, -1)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>VAT Report - ForgeCommerce Admin</title>
  <script src="/static/js/htmx.min.js"></script>
  <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
<div class="container">
  <h1>VAT Report</h1>

  <form id="vat-filter" method="get" action="/admin/reports/vat">
    <label for="period">Period:</label>
    <select id="period" name="period">
      <option value="monthly"%s>Monthly</option>
      <option value="quarterly"%s>Quarterly</option>
      <option value="yearly"%s>Yearly</option>
    </select>
    <label for="from">From:</label>
    <input type="date" id="from" name="from" value="%s">
    <label for="to">To:</label>
    <input type="date" id="to" name="to" value="%s">
    <button type="submit">Apply</button>
    <a href="/admin/reports/vat/csv?from=%s&to=%s&period=%s" class="btn btn-secondary">Export CSV</a>
  </form>

  <div id="vat-report-data"
       hx-get="/admin/reports/vat/data?from=%s&to=%s&period=%s"
       hx-trigger="load"
       hx-swap="innerHTML">
    <p>Loading VAT report data...</p>
  </div>
</div>
</body>
</html>`,
		selectedAttr(period, "monthly"),
		selectedAttr(period, "quarterly"),
		selectedAttr(period, "yearly"),
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
		period,
		from.Format("2006-01-02"),
		displayTo.Format("2006-01-02"),
		period,
	)
}

// VATReportData handles GET /admin/reports/vat/data.
// Returns an HTML fragment with per-country VAT breakdown and reverse charge section.
func (h *ReportHandler) VATReportData(w http.ResponseWriter, r *http.Request) {
	from, to := parseVATPeriod(r)

	vatReport, err := h.reportSvc.GetVATReport(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get VAT report", "error", err)
		http.Error(w, "Failed to load VAT report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Per-country VAT breakdown table.
	fmt.Fprint(w, `<h2>VAT by Country &amp; Rate Type</h2>
<table class="table">
  <thead>
    <tr>
      <th>Country</th>
      <th>Rate Type</th>
      <th>Rate</th>
      <th>Net Sales</th>
      <th>VAT Collected</th>
      <th>Gross Sales</th>
      <th>Orders</th>
    </tr>
  </thead>
  <tbody>`)

	if len(vatReport.CountryBreakdown) == 0 {
		fmt.Fprint(w, `<tr><td colspan="7" class="text-muted">No VAT data for this period.</td></tr>`)
	} else {
		for _, line := range vatReport.CountryBreakdown {
			countryDisplay := line.CountryCode
			if line.CountryName != "" {
				countryDisplay = fmt.Sprintf("%s (%s)", line.CountryName, line.CountryCode)
			}
			fmt.Fprintf(w,
				"<tr><td>%s</td><td>%s</td><td>%s%%</td><td>&euro;%s</td><td>&euro;%s</td><td>&euro;%s</td><td>%d</td></tr>",
				countryDisplay,
				line.RateType,
				formatNumericValue(line.Rate),
				formatNumericValue(line.NetSales),
				formatNumericValue(line.VATCollected),
				formatNumericValue(line.GrossSales),
				line.OrderCount,
			)
		}
	}

	fmt.Fprint(w, `</tbody></table>`)

	// Reverse charge section.
	rc := vatReport.ReverseCharge
	fmt.Fprintf(w, `<h2>B2B Reverse Charge</h2>
<div class="report-summary">
  <div class="summary-card">
    <span class="summary-label">Reverse Charge Orders</span>
    <span class="summary-value">%d</span>
  </div>
  <div class="summary-card">
    <span class="summary-label">Total Net Value</span>
    <span class="summary-value">&euro;%s</span>
  </div>
</div>`,
		rc.OrderCount,
		formatNumericValue(rc.TotalNet),
	)

	// Reverse charge orders table.
	fmt.Fprint(w, `<table class="table">
  <thead>
    <tr>
      <th>Order</th>
      <th>Email</th>
      <th>VAT Number</th>
      <th>Company</th>
      <th>Country</th>
      <th>Net Total</th>
      <th>Date</th>
    </tr>
  </thead>
  <tbody>`)

	if len(vatReport.ReverseChargeOrders) == 0 {
		fmt.Fprint(w, `<tr><td colspan="7" class="text-muted">No reverse charge orders for this period.</td></tr>`)
	} else {
		for _, o := range vatReport.ReverseChargeOrders {
			vatNum := ""
			if o.VATNumber != nil {
				vatNum = *o.VATNumber
			}
			company := ""
			if o.CompanyName != nil {
				company = *o.CompanyName
			}
			country := ""
			if o.CountryCode != nil {
				country = *o.CountryCode
			}
			fmt.Fprintf(w,
				"<tr><td>#%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>&euro;%s</td><td>%s</td></tr>",
				o.OrderNumber,
				o.Email,
				vatNum,
				company,
				country,
				formatNumericValue(o.NetTotal),
				o.CreatedAt.Format("2006-01-02 15:04"),
			)
		}
	}

	fmt.Fprint(w, `</tbody></table>`)
}

// VATReportCSV handles GET /admin/reports/vat/csv.
// Returns a CSV file download of the per-country VAT breakdown.
func (h *ReportHandler) VATReportCSV(w http.ResponseWriter, r *http.Request) {
	from, to := parseVATPeriod(r)

	vatReport, err := h.reportSvc.GetVATReport(r.Context(), from, to)
	if err != nil {
		h.logger.Error("failed to get VAT report for CSV", "error", err)
		http.Error(w, "Failed to generate CSV", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("vat-report-%s-to-%s.csv",
		from.Format("2006-01-02"),
		to.AddDate(0, 0, -1).Format("2006-01-02"),
	)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	// Header row.
	csvWriter.Write([]string{
		"country_code", "country_name", "rate_type", "rate",
		"net_sales", "vat_collected", "gross_sales", "order_count",
	})

	for _, line := range vatReport.CountryBreakdown {
		csvWriter.Write([]string{
			line.CountryCode,
			line.CountryName,
			line.RateType,
			formatNumericValue(line.Rate),
			formatNumericValue(line.NetSales),
			formatNumericValue(line.VATCollected),
			formatNumericValue(line.GrossSales),
			fmt.Sprintf("%d", line.OrderCount),
		})
	}
}

// selectedAttr returns ` selected` if value matches target, otherwise empty string.
func selectedAttr(value, target string) string {
	if value == target {
		return " selected"
	}
	return ""
}

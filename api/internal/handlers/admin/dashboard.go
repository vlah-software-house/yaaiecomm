package admin

import (
	"fmt"
	"log/slog"
	"math/big"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
	admintmpl "github.com/forgecommerce/api/templates/admin"
)

// DashboardHandler handles admin dashboard endpoints.
type DashboardHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *slog.Logger
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(pool *pgxpool.Pool, queries *db.Queries, logger *slog.Logger) *DashboardHandler {
	return &DashboardHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// RegisterRoutes registers dashboard routes on the given mux.
func (h *DashboardHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/dashboard", h.ShowDashboard)
	mux.HandleFunc("GET /admin/dashboard/stats/orders-today", h.StatOrdersToday)
	mux.HandleFunc("GET /admin/dashboard/stats/revenue-month", h.StatRevenueMonth)
	mux.HandleFunc("GET /admin/dashboard/stats/low-stock", h.StatLowStock)
	mux.HandleFunc("GET /admin/dashboard/stats/pending-orders", h.StatPendingOrders)
	mux.HandleFunc("GET /admin/dashboard/recent-orders", h.RecentOrders)
}

// ShowDashboard handles GET /admin/dashboard.
func (h *DashboardHandler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	component := admintmpl.DashboardPage()
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render dashboard", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// StatOrdersToday handles GET /admin/dashboard/stats/orders-today.
func (h *DashboardHandler) StatOrdersToday(w http.ResponseWriter, r *http.Request) {
	count, err := h.queries.CountOrdersToday(r.Context())
	if err != nil {
		h.logger.Error("failed to count orders today", "error", err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%d", count)
}

// StatRevenueMonth handles GET /admin/dashboard/stats/revenue-month.
func (h *DashboardHandler) StatRevenueMonth(w http.ResponseWriter, r *http.Request) {
	raw, err := h.queries.SumRevenueMonth(r.Context())
	if err != nil {
		h.logger.Error("failed to sum revenue month", "error", err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	formatted := formatNumericAsEUR(raw)
	fmt.Fprint(w, formatted)
}

// StatLowStock handles GET /admin/dashboard/stats/low-stock.
func (h *DashboardHandler) StatLowStock(w http.ResponseWriter, r *http.Request) {
	count, err := h.queries.CountLowStockVariants(r.Context())
	if err != nil {
		h.logger.Error("failed to count low stock variants", "error", err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%d", count)
}

// StatPendingOrders handles GET /admin/dashboard/stats/pending-orders.
func (h *DashboardHandler) StatPendingOrders(w http.ResponseWriter, r *http.Request) {
	count, err := h.queries.CountPendingOrders(r.Context())
	if err != nil {
		h.logger.Error("failed to count pending orders", "error", err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%d", count)
}

// RecentOrders handles GET /admin/dashboard/recent-orders.
func (h *DashboardHandler) RecentOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.queries.ListRecentOrders(r.Context(), 10)
	if err != nil {
		h.logger.Error("failed to list recent orders", "error", err)
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(orders) == 0 {
		fmt.Fprint(w, `<tr><td colspan="5" class="text-muted">No orders yet.</td></tr>`)
		return
	}

	for _, o := range orders {
		total := formatNumericAsEUR(o.Total)
		date := o.CreatedAt.Format("02 Jan 2006 15:04")
		badgeClass := statusBadgeClass(o.Status)

		fmt.Fprintf(w,
			"<tr>"+
				"<td>#%d</td>"+
				"<td>%s</td>"+
				`<td><span class="badge %s">%s</span></td>`+
				"<td>%s</td>"+
				"<td>%s</td>"+
				"</tr>",
			o.OrderNumber,
			o.Email,
			badgeClass,
			o.Status,
			total,
			date,
		)
	}
}

// formatNumericAsEUR formats a pgtype.Numeric (or interface{} wrapping one) as a EUR string.
func formatNumericAsEUR(v interface{}) string {
	switch val := v.(type) {
	case pgtype.Numeric:
		if !val.Valid {
			return "\u20ac0.00"
		}
		f, err := numericToFloat64(val)
		if err != nil {
			return "\u20ac0.00"
		}
		return fmt.Sprintf("\u20ac%.2f", f)
	case *pgtype.Numeric:
		if val == nil || !val.Valid {
			return "\u20ac0.00"
		}
		f, err := numericToFloat64(*val)
		if err != nil {
			return "\u20ac0.00"
		}
		return fmt.Sprintf("\u20ac%.2f", f)
	default:
		// Fallback: try to format as a number via Sprintf.
		return fmt.Sprintf("\u20ac%v", v)
	}
}

// numericToFloat64 converts a pgtype.Numeric to float64 for display formatting.
func numericToFloat64(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, fmt.Errorf("numeric is not valid")
	}
	// Use big.Float for accurate conversion.
	bf := new(big.Float).SetInt(n.Int)
	if n.Exp < 0 {
		divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil))
		bf.Quo(bf, divisor)
	} else if n.Exp > 0 {
		multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil))
		bf.Mul(bf, multiplier)
	}
	f, _ := bf.Float64()
	return f, nil
}

// statusBadgeClass returns a CSS class for the given order status.
func statusBadgeClass(status string) string {
	switch status {
	case "pending":
		return "badge-warning"
	case "processing":
		return "badge-info"
	case "shipped":
		return "badge-primary"
	case "delivered":
		return "badge-success"
	case "cancelled":
		return "badge-danger"
	case "refunded":
		return "badge-secondary"
	default:
		return "badge-secondary"
	}
}

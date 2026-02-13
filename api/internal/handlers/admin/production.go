package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/production"
	"github.com/forgecommerce/api/templates/admin"
)

// ProductionHandler handles admin production batch management endpoints.
type ProductionHandler struct {
	production *production.Service
	products   *product.Service
	logger     *slog.Logger
}

// NewProductionHandler creates a new production handler.
func NewProductionHandler(productionSvc *production.Service, productSvc *product.Service, logger *slog.Logger) *ProductionHandler {
	return &ProductionHandler{
		production: productionSvc,
		products:   productSvc,
		logger:     logger,
	}
}

// RegisterRoutes registers production admin routes on the given mux.
func (h *ProductionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/production", h.ListBatches)
	mux.HandleFunc("GET /admin/production/new", h.NewBatch)
	mux.HandleFunc("POST /admin/production", h.CreateBatch)
	mux.HandleFunc("GET /admin/production/{id}", h.ShowBatch)
	mux.HandleFunc("POST /admin/production/{id}/start", h.StartBatch)
	mux.HandleFunc("POST /admin/production/{id}/complete", h.CompleteBatch)
	mux.HandleFunc("POST /admin/production/{id}/cancel", h.CancelBatch)
}

// ListBatches handles GET /admin/production.
func (h *ProductionHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	var items []admin.ProductionListItem

	if statusFilter != "" {
		batches, err := h.production.ListByStatus(r.Context(), db.ProductionBatchStatus(statusFilter))
		if err != nil {
			h.logger.Error("failed to list production batches by status", "error", err, "status", statusFilter)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		items = make([]admin.ProductionListItem, 0, len(batches))
		for _, b := range batches {
			items = append(items, h.batchByStatusRowToListItem(b))
		}
	} else {
		batches, err := h.production.List(r.Context(), 50, 0)
		if err != nil {
			h.logger.Error("failed to list production batches", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		items = make([]admin.ProductionListItem, 0, len(batches))
		for _, b := range batches {
			items = append(items, h.batchRowToListItem(b))
		}
	}

	data := admin.ProductionListData{
		Batches:      items,
		StatusFilter: statusFilter,
	}

	admin.ProductionListPage(data).Render(r.Context(), w)
}

// NewBatch handles GET /admin/production/new.
func (h *ProductionHandler) NewBatch(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	products, _, err := h.products.List(r.Context(), nil, 1, 100)
	if err != nil {
		h.logger.Error("failed to list products for batch form", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	formProducts := make([]admin.ProductionFormProduct, 0, len(products))
	for _, p := range products {
		formProducts = append(formProducts, admin.ProductionFormProduct{
			ID:   p.ID.String(),
			Name: p.Name,
		})
	}

	data := admin.ProductionFormData{
		Products:  formProducts,
		CSRFToken: csrfToken,
	}

	admin.ProductionNewPage(data).Render(r.Context(), w)
}

// CreateBatch handles POST /admin/production.
func (h *ProductionHandler) CreateBatch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	productID, err := uuid.Parse(r.FormValue("product_id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var variantID pgtype.UUID
	if vid := r.FormValue("variant_id"); vid != "" {
		parsed, err := uuid.Parse(vid)
		if err != nil {
			http.Error(w, "Invalid variant ID", http.StatusBadRequest)
			return
		}
		variantID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	qty, err := strconv.Atoi(r.FormValue("planned_quantity"))
	if err != nil || qty <= 0 {
		http.Error(w, "Planned quantity must be a positive integer", http.StatusBadRequest)
		return
	}

	batch, err := h.production.CreateBatch(r.Context(), production.CreateBatchParams{
		ProductID:     productID,
		VariantID:     variantID,
		PlannedQty:    qty,
		ScheduledDate: r.FormValue("scheduled_date"),
		Notes:         r.FormValue("notes"),
	})
	if err != nil {
		h.logger.Error("failed to create production batch", "error", err)
		http.Error(w, "Failed to create batch", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/production/"+batch.ID.String(), http.StatusSeeOther)
}

// ShowBatch handles GET /admin/production/{id}.
func (h *ProductionHandler) ShowBatch(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid batch ID", http.StatusBadRequest)
		return
	}

	batch, err := h.production.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, production.ErrNotFound) {
			http.Error(w, "Batch not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get production batch", "error", err, "batch_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	materials, err := h.production.ListMaterials(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list batch materials", "error", err, "batch_id", id)
		// Continue without materials rather than failing completely.
		materials = nil
	}

	materialRows := make([]admin.ProductionMaterialRow, 0, len(materials))
	for _, m := range materials {
		materialRows = append(materialRows, admin.ProductionMaterialRow{
			ID:               m.ID.String(),
			MaterialName:     m.MaterialName,
			MaterialSku:      m.MaterialSku,
			UnitOfMeasure:    m.UnitOfMeasure,
			RequiredQuantity: formatNumeric(m.RequiredQuantity),
			ConsumedQuantity: formatNumeric(m.ConsumedQuantity),
			AvailableStock:   formatNumeric(m.AvailableStock),
			UnitCost:         formatNumeric(m.UnitCost),
		})
	}

	actualQty := 0
	if batch.ActualQuantity != nil {
		actualQty = int(*batch.ActualQuantity)
	}

	data := admin.ProductionDetailData{
		Batch: admin.ProductionDetailItem{
			ID:              batch.ID.String(),
			BatchNumber:     batch.BatchNumber,
			ProductName:     batch.ProductName,
			VariantSku:      derefString(batch.VariantSku),
			PlannedQuantity: int(batch.PlannedQuantity),
			ActualQuantity:  actualQty,
			Status:          string(batch.Status),
			ScheduledDate:   formatDate(batch.ScheduledDate),
			StartedAt:       formatTimestamptz(batch.StartedAt),
			CompletedAt:     formatTimestamptz(batch.CompletedAt),
			Notes:           derefString(batch.Notes),
			CostTotal:       formatNumeric(batch.CostTotal),
			CreatedAt:       batch.CreatedAt.Format("2006-01-02 15:04"),
		},
		Materials: materialRows,
		CSRFToken: csrfToken,
	}

	admin.ProductionDetailPage(data).Render(r.Context(), w)
}

// StartBatch handles POST /admin/production/{id}/start.
func (h *ProductionHandler) StartBatch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid batch ID", http.StatusBadRequest)
		return
	}

	if _, err := h.production.Start(r.Context(), id); err != nil {
		if errors.Is(err, production.ErrInvalidStatus) {
			http.Error(w, "Batch cannot be started from its current status", http.StatusBadRequest)
			return
		}
		h.logger.Error("failed to start production batch", "error", err, "batch_id", id)
		http.Error(w, "Failed to start batch", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/production/"+id.String(), http.StatusSeeOther)
}

// CompleteBatch handles POST /admin/production/{id}/complete.
func (h *ProductionHandler) CompleteBatch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid batch ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	actualQty, err := strconv.Atoi(r.FormValue("actual_quantity"))
	if err != nil || actualQty < 0 {
		http.Error(w, "Actual quantity must be a non-negative integer", http.StatusBadRequest)
		return
	}

	var costTotal pgtype.Numeric
	if ct := r.FormValue("cost_total"); ct != "" {
		if err := costTotal.Scan(ct); err != nil {
			http.Error(w, "Invalid cost total", http.StatusBadRequest)
			return
		}
	}

	if _, err := h.production.Complete(r.Context(), id, actualQty, costTotal); err != nil {
		if errors.Is(err, production.ErrInvalidStatus) {
			http.Error(w, "Batch cannot be completed from its current status", http.StatusBadRequest)
			return
		}
		h.logger.Error("failed to complete production batch", "error", err, "batch_id", id)
		http.Error(w, "Failed to complete batch", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/production/"+id.String(), http.StatusSeeOther)
}

// CancelBatch handles POST /admin/production/{id}/cancel.
func (h *ProductionHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid batch ID", http.StatusBadRequest)
		return
	}

	if _, err := h.production.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, production.ErrInvalidStatus) {
			http.Error(w, "Batch cannot be cancelled from its current status", http.StatusBadRequest)
			return
		}
		h.logger.Error("failed to cancel production batch", "error", err, "batch_id", id)
		http.Error(w, "Failed to cancel batch", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/production", http.StatusSeeOther)
}

// batchRowToListItem converts a ListProductionBatchesRow to a template list item.
func (h *ProductionHandler) batchRowToListItem(b db.ListProductionBatchesRow) admin.ProductionListItem {
	actualQty := 0
	if b.ActualQuantity != nil {
		actualQty = int(*b.ActualQuantity)
	}
	return admin.ProductionListItem{
		ID:              b.ID.String(),
		BatchNumber:     b.BatchNumber,
		ProductName:     b.ProductName,
		VariantSku:      derefString(b.VariantSku),
		PlannedQuantity: int(b.PlannedQuantity),
		ActualQuantity:  actualQty,
		Status:          string(b.Status),
		ScheduledDate:   formatDate(b.ScheduledDate),
		CreatedAt:       b.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// batchByStatusRowToListItem converts a ListProductionBatchesByStatusRow to a template list item.
func (h *ProductionHandler) batchByStatusRowToListItem(b db.ListProductionBatchesByStatusRow) admin.ProductionListItem {
	actualQty := 0
	if b.ActualQuantity != nil {
		actualQty = int(*b.ActualQuantity)
	}
	return admin.ProductionListItem{
		ID:              b.ID.String(),
		BatchNumber:     b.BatchNumber,
		ProductName:     b.ProductName,
		VariantSku:      derefString(b.VariantSku),
		PlannedQuantity: int(b.PlannedQuantity),
		ActualQuantity:  actualQty,
		Status:          string(b.Status),
		ScheduledDate:   formatDate(b.ScheduledDate),
		CreatedAt:       b.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// formatDate formats a pgtype.Date into a readable string.
func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

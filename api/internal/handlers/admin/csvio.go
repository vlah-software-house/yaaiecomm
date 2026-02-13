package admin

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/order"
	"github.com/forgecommerce/api/internal/services/product"
	"github.com/forgecommerce/api/internal/services/rawmaterial"
)

// maxCSVImportSize limits uploaded CSV files to 10 MB.
const maxCSVImportSize = 10 << 20

// CSVIOHandler handles CSV export and import endpoints for the admin panel.
type CSVIOHandler struct {
	productSvc     *product.Service
	rawMaterialSvc *rawmaterial.Service
	orderSvc       *order.Service
	logger         *slog.Logger
}

// NewCSVIOHandler creates a new handler for CSV import/export operations.
func NewCSVIOHandler(
	productSvc *product.Service,
	rawMaterialSvc *rawmaterial.Service,
	orderSvc *order.Service,
	logger *slog.Logger,
) *CSVIOHandler {
	return &CSVIOHandler{
		productSvc:     productSvc,
		rawMaterialSvc: rawMaterialSvc,
		orderSvc:       orderSvc,
		logger:         logger,
	}
}

// RegisterRoutes registers CSV import/export admin routes on the given mux.
func (h *CSVIOHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/export/products/csv", h.ExportProductsCSV)
	mux.HandleFunc("GET /admin/export/raw-materials/csv", h.ExportRawMaterialsCSV)
	mux.HandleFunc("GET /admin/export/orders/csv", h.ExportOrdersCSV)
	mux.HandleFunc("POST /admin/import/products", h.ImportProducts)
	mux.HandleFunc("POST /admin/import/raw-materials", h.ImportRawMaterials)
	mux.HandleFunc("GET /admin/import", h.ImportPage)
}

// --- CSV Export: Products ---

// ExportProductsCSV handles GET /admin/export/products/csv.
// It exports all products as a CSV file download.
func (h *CSVIOHandler) ExportProductsCSV(w http.ResponseWriter, r *http.Request) {
	// Fetch all products via pagination. The service caps at 250 per call.
	type productRow struct {
		Name             string
		SkuPrefix        string
		Status           string
		BasePrice        pgtype.Numeric
		BaseWeightGrams  int32
		Description      string
		ShortDescription string
	}
	var allProducts []productRow

	page := 1
	for {
		products, _, err := h.productSvc.List(r.Context(), nil, page, 250)
		if err != nil {
			h.logger.Error("failed to list products for CSV export", "error", err)
			http.Error(w, "Failed to export products", http.StatusInternalServerError)
			return
		}
		for _, p := range products {
			allProducts = append(allProducts, productRow{
				Name:             p.Name,
				SkuPrefix:        derefStr(p.SkuPrefix),
				Status:           p.Status,
				BasePrice:        p.BasePrice,
				BaseWeightGrams:  p.BaseWeightGrams,
				Description:      derefStr(p.Description),
				ShortDescription: derefStr(p.ShortDescription),
			})
		}
		if len(products) < 250 {
			break
		}
		page++
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="products.csv"`)

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	// Header row.
	csvWriter.Write([]string{
		"name", "sku_prefix", "status", "base_price", "base_weight_grams",
		"description", "short_description",
	})

	for _, p := range allProducts {
		csvWriter.Write([]string{
			p.Name,
			p.SkuPrefix,
			p.Status,
			fmtNumericCSV(p.BasePrice),
			strconv.FormatInt(int64(p.BaseWeightGrams), 10),
			p.Description,
			p.ShortDescription,
		})
	}
}

// --- CSV Export: Raw Materials ---

// ExportRawMaterialsCSV handles GET /admin/export/raw-materials/csv.
// It exports all raw materials as a CSV file download.
func (h *CSVIOHandler) ExportRawMaterialsCSV(w http.ResponseWriter, r *http.Request) {
	// Build a category name lookup map.
	categories, err := h.rawMaterialSvc.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("failed to list categories for CSV export", "error", err)
		http.Error(w, "Failed to export raw materials", http.StatusInternalServerError)
		return
	}
	categoryNameByID := make(map[uuid.UUID]string, len(categories))
	for _, c := range categories {
		categoryNameByID[c.ID] = c.Name
	}

	// Fetch all raw materials via pagination.
	type rmRow struct {
		Name              string
		Sku               string
		Category          string
		UnitOfMeasure     string
		CostPerUnit       pgtype.Numeric
		StockQuantity     pgtype.Numeric
		LowStockThreshold pgtype.Numeric
		SupplierName      string
	}
	var allMaterials []rmRow

	page := 1
	for {
		materials, _, err := h.rawMaterialSvc.List(r.Context(), nil, nil, page, 250)
		if err != nil {
			h.logger.Error("failed to list raw materials for CSV export", "error", err)
			http.Error(w, "Failed to export raw materials", http.StatusInternalServerError)
			return
		}
		for _, m := range materials {
			categoryName := ""
			if m.CategoryID.Valid {
				categoryName = categoryNameByID[m.CategoryID.Bytes]
			}
			allMaterials = append(allMaterials, rmRow{
				Name:              m.Name,
				Sku:               m.Sku,
				Category:          categoryName,
				UnitOfMeasure:     m.UnitOfMeasure,
				CostPerUnit:       m.CostPerUnit,
				StockQuantity:     m.StockQuantity,
				LowStockThreshold: m.LowStockThreshold,
				SupplierName:      derefStr(m.SupplierName),
			})
		}
		if len(materials) < 250 {
			break
		}
		page++
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="raw-materials.csv"`)

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	csvWriter.Write([]string{
		"name", "sku", "category", "unit_of_measure", "cost_per_unit",
		"stock_quantity", "low_stock_threshold", "supplier_name",
	})

	for _, m := range allMaterials {
		csvWriter.Write([]string{
			m.Name,
			m.Sku,
			m.Category,
			m.UnitOfMeasure,
			fmtNumericCSV(m.CostPerUnit),
			fmtNumericCSV(m.StockQuantity),
			fmtNumericCSV(m.LowStockThreshold),
			m.SupplierName,
		})
	}
}

// --- CSV Export: Orders ---

// ExportOrdersCSV handles GET /admin/export/orders/csv.
// It exports all orders as a CSV file download.
func (h *CSVIOHandler) ExportOrdersCSV(w http.ResponseWriter, r *http.Request) {
	type orderRow struct {
		OrderNumber   int64
		Email         string
		Status        string
		PaymentStatus string
		Subtotal      pgtype.Numeric
		VatTotal      pgtype.Numeric
		ShippingFee   pgtype.Numeric
		Total         pgtype.Numeric
		CreatedAt     string
	}
	var allOrders []orderRow

	page := 1
	for {
		orders, _, err := h.orderSvc.List(r.Context(), nil, page, 250)
		if err != nil {
			h.logger.Error("failed to list orders for CSV export", "error", err)
			http.Error(w, "Failed to export orders", http.StatusInternalServerError)
			return
		}
		for _, o := range orders {
			allOrders = append(allOrders, orderRow{
				OrderNumber:   o.OrderNumber,
				Email:         o.Email,
				Status:        o.Status,
				PaymentStatus: o.PaymentStatus,
				Subtotal:      o.Subtotal,
				VatTotal:      o.VatTotal,
				ShippingFee:   o.ShippingFee,
				Total:         o.Total,
				CreatedAt:     o.CreatedAt.Format("2006-01-02"),
			})
		}
		if len(orders) < 250 {
			break
		}
		page++
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="orders.csv"`)

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	csvWriter.Write([]string{
		"order_number", "email", "status", "payment_status",
		"subtotal", "vat_total", "shipping_fee", "total", "created_at",
	})

	for _, o := range allOrders {
		csvWriter.Write([]string{
			fmt.Sprintf("FC-%04d", o.OrderNumber),
			o.Email,
			o.Status,
			o.PaymentStatus,
			fmtNumericCSV(o.Subtotal),
			fmtNumericCSV(o.VatTotal),
			fmtNumericCSV(o.ShippingFee),
			fmtNumericCSV(o.Total),
			o.CreatedAt,
		})
	}
}

// --- CSV Import: Products ---

// ImportProducts handles POST /admin/import/products.
// It parses an uploaded CSV file and creates products via the product service.
// Returns an HTML summary of the import with per-row error details.
func (h *CSVIOHandler) ImportProducts(w http.ResponseWriter, r *http.Request) {
	records, err := parseCSVUpload(r, "file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read CSV: %s", err), http.StatusBadRequest)
		return
	}

	if len(records) < 2 {
		http.Error(w, "CSV file is empty or contains only headers", http.StatusBadRequest)
		return
	}

	headers := normalizeHeaders(records[0])
	nameIdx := colIndex(headers, "name")
	skuPrefixIdx := colIndex(headers, "sku_prefix")
	statusIdx := colIndex(headers, "status")
	basePriceIdx := colIndex(headers, "base_price")
	baseWeightIdx := colIndex(headers, "base_weight_grams")
	descIdx := colIndex(headers, "description")
	shortDescIdx := colIndex(headers, "short_description")

	if nameIdx < 0 || skuPrefixIdx < 0 || basePriceIdx < 0 {
		http.Error(w, "CSV missing required columns: name, sku_prefix, base_price", http.StatusBadRequest)
		return
	}

	var imported int
	var importErrors []string

	for i, row := range records[1:] {
		lineNum := i + 2 // 1-indexed, skipping header

		name := colVal(row, nameIdx)
		if name == "" {
			importErrors = append(importErrors, fmt.Sprintf("line %d: name is required", lineNum))
			continue
		}

		skuPrefix := colVal(row, skuPrefixIdx)
		if skuPrefix == "" {
			importErrors = append(importErrors, fmt.Sprintf("line %d: sku_prefix is required", lineNum))
			continue
		}

		basePriceStr := colVal(row, basePriceIdx)
		basePrice := parseNumeric(basePriceStr)
		if !basePrice.Valid {
			importErrors = append(importErrors, fmt.Sprintf("line %d: invalid base_price %q", lineNum, basePriceStr))
			continue
		}

		status := colVal(row, statusIdx)
		if status == "" {
			status = "draft"
		}

		var baseWeight int32
		if baseWeightIdx >= 0 {
			w64, _ := strconv.ParseInt(colVal(row, baseWeightIdx), 10, 32)
			baseWeight = int32(w64)
		}

		params := product.CreateProductParams{
			Name:             name,
			SkuPrefix:        strPtr(skuPrefix),
			Status:           status,
			BasePrice:        basePrice,
			BaseWeightGrams:  baseWeight,
			Description:      strPtrOrNil(colVal(row, descIdx)),
			ShortDescription: strPtrOrNil(colVal(row, shortDescIdx)),
		}

		_, createErr := h.productSvc.Create(r.Context(), params)
		if createErr != nil {
			importErrors = append(importErrors, fmt.Sprintf("line %d: %s", lineNum, createErr))
			continue
		}
		imported++
	}

	renderImportResult(w, "Products", imported, importErrors)
}

// --- CSV Import: Raw Materials ---

// ImportRawMaterials handles POST /admin/import/raw-materials.
// It parses an uploaded CSV file and creates raw materials via the raw material service.
// Returns an HTML summary of the import with per-row error details.
func (h *CSVIOHandler) ImportRawMaterials(w http.ResponseWriter, r *http.Request) {
	records, err := parseCSVUpload(r, "file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read CSV: %s", err), http.StatusBadRequest)
		return
	}

	if len(records) < 2 {
		http.Error(w, "CSV file is empty or contains only headers", http.StatusBadRequest)
		return
	}

	// Build category name -> ID lookup for resolving the "category" column.
	categories, catErr := h.rawMaterialSvc.ListCategories(r.Context())
	if catErr != nil {
		h.logger.Error("failed to list categories for import", "error", catErr)
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	categoryIDByName := make(map[string]uuid.UUID, len(categories))
	for _, c := range categories {
		categoryIDByName[strings.ToLower(c.Name)] = c.ID
	}

	headers := normalizeHeaders(records[0])
	nameIdx := colIndex(headers, "name")
	skuIdx := colIndex(headers, "sku")
	categoryIdx := colIndex(headers, "category")
	uomIdx := colIndex(headers, "unit_of_measure")
	costIdx := colIndex(headers, "cost_per_unit")
	stockIdx := colIndex(headers, "stock_quantity")
	lowStockIdx := colIndex(headers, "low_stock_threshold")
	supplierIdx := colIndex(headers, "supplier_name")

	if nameIdx < 0 || skuIdx < 0 || uomIdx < 0 || costIdx < 0 {
		http.Error(w, "CSV missing required columns: name, sku, unit_of_measure, cost_per_unit", http.StatusBadRequest)
		return
	}

	var imported int
	var importErrors []string

	for i, row := range records[1:] {
		lineNum := i + 2

		name := colVal(row, nameIdx)
		if name == "" {
			importErrors = append(importErrors, fmt.Sprintf("line %d: name is required", lineNum))
			continue
		}

		sku := colVal(row, skuIdx)
		if sku == "" {
			importErrors = append(importErrors, fmt.Sprintf("line %d: sku is required", lineNum))
			continue
		}

		uom := colVal(row, uomIdx)
		if uom == "" {
			importErrors = append(importErrors, fmt.Sprintf("line %d: unit_of_measure is required", lineNum))
			continue
		}

		costStr := colVal(row, costIdx)
		costPerUnit := parseNumeric(costStr)
		if !costPerUnit.Valid {
			importErrors = append(importErrors, fmt.Sprintf("line %d: invalid cost_per_unit %q", lineNum, costStr))
			continue
		}

		params := rawmaterial.CreateRawMaterialParams{
			Name:          name,
			Sku:           sku,
			UnitOfMeasure: uom,
			CostPerUnit:   costPerUnit,
			IsActive:      true,
		}

		// Resolve optional category.
		if categoryIdx >= 0 {
			catName := colVal(row, categoryIdx)
			if catName != "" {
				if catID, ok := categoryIDByName[strings.ToLower(catName)]; ok {
					id := catID
					params.CategoryID = &id
				}
				// If category not found, silently skip it (don't fail the row).
			}
		}

		// Optional stock quantity.
		if stockIdx >= 0 {
			params.StockQuantity = parseNumeric(colVal(row, stockIdx))
		}

		// Optional low stock threshold.
		if lowStockIdx >= 0 {
			params.LowStockThreshold = parseNumeric(colVal(row, lowStockIdx))
		}

		// Optional supplier name.
		if supplierIdx >= 0 {
			params.SupplierName = strPtrOrNil(colVal(row, supplierIdx))
		}

		_, createErr := h.rawMaterialSvc.Create(r.Context(), params)
		if createErr != nil {
			importErrors = append(importErrors, fmt.Sprintf("line %d: %s", lineNum, createErr))
			continue
		}
		imported++
	}

	renderImportResult(w, "Raw Materials", imported, importErrors)
}

// --- Import Page ---

// ImportPage handles GET /admin/import.
// It renders a simple page with file upload forms for product and raw material CSV imports.
func (h *CSVIOHandler) ImportPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>CSV Import - ForgeCommerce Admin</title>
  <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
<div class="container">
  <h1>CSV Import</h1>

  <div class="card" style="margin-bottom:2rem;">
    <h2>Import Products</h2>
    <p>Upload a CSV file with columns: <code>name</code>, <code>sku_prefix</code>, <code>base_price</code> (required),
       <code>status</code>, <code>base_weight_grams</code>, <code>description</code>, <code>short_description</code> (optional).</p>
    <form method="post" action="/admin/import/products" enctype="multipart/form-data">
      <input type="file" name="file" accept=".csv" required>
      <button type="submit" class="btn btn-primary">Import Products</button>
    </form>
  </div>

  <div class="card">
    <h2>Import Raw Materials</h2>
    <p>Upload a CSV file with columns: <code>name</code>, <code>sku</code>, <code>unit_of_measure</code>, <code>cost_per_unit</code> (required),
       <code>category</code>, <code>stock_quantity</code>, <code>low_stock_threshold</code>, <code>supplier_name</code> (optional).</p>
    <form method="post" action="/admin/import/raw-materials" enctype="multipart/form-data">
      <input type="file" name="file" accept=".csv" required>
      <button type="submit" class="btn btn-primary">Import Raw Materials</button>
    </form>
  </div>

  <div class="card" style="margin-top:2rem;">
    <h2>Export</h2>
    <ul>
      <li><a href="/admin/export/products/csv">Export Products CSV</a></li>
      <li><a href="/admin/export/raw-materials/csv">Export Raw Materials CSV</a></li>
      <li><a href="/admin/export/orders/csv">Export Orders CSV</a></li>
    </ul>
  </div>
</div>
</body>
</html>`)
}

// --- Helpers (package-private, unique to this file) ---

// parseCSVUpload reads and parses a CSV file from a multipart form upload.
func parseCSVUpload(r *http.Request, fieldName string) ([][]string, error) {
	if err := r.ParseMultipartForm(maxCSVImportSize); err != nil {
		return nil, fmt.Errorf("parsing multipart form: %w", err)
	}

	file, _, err := r.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("reading uploaded file: %w", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true
	// Allow variable number of fields per row for resilience.
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	return records, nil
}

// normalizeHeaders lowercases and trims all header column names.
func normalizeHeaders(row []string) []string {
	out := make([]string, len(row))
	for i, h := range row {
		out[i] = strings.ToLower(strings.TrimSpace(h))
	}
	return out
}

// colIndex returns the index of the named column in the headers, or -1 if not found.
func colIndex(headers []string, name string) int {
	for i, h := range headers {
		if h == name {
			return i
		}
	}
	return -1
}

// colVal safely returns the value at index idx from a row, or "" if out of range.
func colVal(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// fmtNumericCSV formats a pgtype.Numeric as a plain decimal string for CSV output.
// Uses the same logic as formatNumericValue in reports.go. Returns "0.00" for invalid values.
func fmtNumericCSV(n pgtype.Numeric) string {
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

// strPtrOrNil returns a pointer to s if it is non-empty, otherwise nil.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// renderImportResult writes an HTML page summarizing import results.
func renderImportResult(w http.ResponseWriter, entityName string, imported int, errs []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>%s Import Results - ForgeCommerce Admin</title>
  <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
<div class="container">
  <h1>%s Import Results</h1>
  <div class="card">
    <p><strong>Successfully imported:</strong> %d</p>
    <p><strong>Errors:</strong> %d</p>`, entityName, entityName, imported, len(errs))

	if len(errs) > 0 {
		fmt.Fprint(w, `
    <h3>Error Details</h3>
    <table class="table">
      <thead><tr><th>Error</th></tr></thead>
      <tbody>`)
		for _, e := range errs {
			fmt.Fprintf(w, "<tr><td>%s</td></tr>", e)
		}
		fmt.Fprint(w, `
      </tbody>
    </table>`)
	}

	fmt.Fprint(w, `
  </div>
  <p><a href="/admin/import">Back to Import</a></p>
</div>
</body>
</html>`)
}

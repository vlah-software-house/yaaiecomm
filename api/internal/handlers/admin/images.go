package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/media"
	"github.com/forgecommerce/api/internal/services/variant"
	"github.com/forgecommerce/api/templates/admin"
)

// ImageHandler handles admin product image management endpoints.
type ImageHandler struct {
	media    *media.Service
	variants *variant.Service
	logger   *slog.Logger
}

// NewImageHandler creates a new image handler.
func NewImageHandler(mediaSvc *media.Service, variantSvc *variant.Service, logger *slog.Logger) *ImageHandler {
	return &ImageHandler{
		media:    mediaSvc,
		variants: variantSvc,
		logger:   logger,
	}
}

// RegisterRoutes registers product image admin routes on the given mux.
func (h *ImageHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/products/{id}/images", h.ShowImages)
	mux.HandleFunc("POST /admin/products/{id}/images/upload", h.UploadImage)
	mux.HandleFunc("POST /admin/products/{id}/images/{imageId}/primary", h.SetPrimary)
	mux.HandleFunc("POST /admin/products/{id}/images/{imageId}/delete", h.DeleteImage)
	mux.HandleFunc("POST /admin/products/{id}/images/reorder", h.ReorderImages)
	mux.HandleFunc("POST /admin/products/{id}/images/{imageId}/alt", h.UpdateAltText)
	mux.HandleFunc("POST /admin/products/{id}/images/{imageId}/assign", h.AssignVariant)
}

// ShowImages handles GET /admin/products/{id}/images.
func (h *ImageHandler) ShowImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	images, err := h.media.ListByProduct(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list product images", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get variants for the assignment dropdown
	variantItems := h.loadVariantItems(ctx, productID)

	items := make([]admin.ProductImageItem, 0, len(images))
	for _, img := range images {
		items = append(items, toImageItem(img))
	}

	data := admin.ProductImagesData{
		ProductID: productID.String(),
		Images:    items,
		Variants:  variantItems,
		CSRFToken: csrfToken,
	}

	admin.ProductImagesPage(data).Render(ctx, w)
}

// UploadImage handles POST /admin/products/{id}/images/upload.
// Accepts multipart file upload, returns the updated image grid via HTMX.
func (h *ImageHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Parse multipart form (32MB max memory, rest to disk)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.logger.Error("failed to parse multipart form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	// Get the current image count for position calculation
	existingImages, err := h.media.ListByProduct(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list existing images", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	nextPosition := int32(len(existingImages))
	isFirstImage := nextPosition == 0

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			h.logger.Error("failed to open uploaded file", "error", err, "filename", fileHeader.Filename)
			continue
		}

		asset, err := h.media.Upload(ctx, file, fileHeader)
		file.Close()
		if err != nil {
			h.logger.Error("failed to upload file",
				"error", err,
				"filename", fileHeader.Filename,
				"product_id", productID,
			)
			continue
		}

		// First image of the product becomes primary
		isPrimary := isFirstImage && i == 0

		_, err = h.media.AssignToProduct(ctx, productID, asset.Url,
			pgtype.UUID{}, pgtype.UUID{}, // no variant/option assignment
			nil, // no alt text initially
			nextPosition+int32(i),
			isPrimary,
		)
		if err != nil {
			h.logger.Error("failed to assign image to product",
				"error", err,
				"asset_id", asset.ID.String(),
				"product_id", productID,
			)
		}
	}

	// Return the updated image grid
	h.renderImageGrid(w, r, productID, csrfToken)
}

// SetPrimary handles POST /admin/products/{id}/images/{imageId}/primary.
func (h *ImageHandler) SetPrimary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	if err := h.media.SetPrimary(ctx, productID, imageID); err != nil {
		h.logger.Error("failed to set primary image", "error", err, "image_id", imageID)
		http.Error(w, "Failed to set primary image", http.StatusInternalServerError)
		return
	}

	h.renderImageGrid(w, r, productID, csrfToken)
}

// DeleteImage handles POST /admin/products/{id}/images/{imageId}/delete.
func (h *ImageHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	if err := h.media.RemoveFromProduct(ctx, imageID); err != nil {
		h.logger.Error("failed to delete image", "error", err, "image_id", imageID)
		http.Error(w, "Failed to delete image", http.StatusInternalServerError)
		return
	}

	h.renderImageGrid(w, r, productID, csrfToken)
}

// ReorderImages handles POST /admin/products/{id}/images/reorder.
// Expects a JSON body with ordered image IDs.
func (h *ImageHandler) ReorderImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// Parse from form value (HTMX sends form data)
	orderStr := r.FormValue("order")
	if orderStr == "" {
		// Try JSON body
		var body struct {
			Order []string `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		orderStr = strings.Join(body.Order, ",")
	}

	idStrs := strings.Split(orderStr, ",")
	imageIDs := make([]uuid.UUID, 0, len(idStrs))
	for _, s := range idStrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid image ID: %s", s), http.StatusBadRequest)
			return
		}
		imageIDs = append(imageIDs, id)
	}

	if len(imageIDs) == 0 {
		http.Error(w, "No image IDs provided", http.StatusBadRequest)
		return
	}

	if err := h.media.ReorderImages(ctx, productID, imageIDs); err != nil {
		h.logger.Error("failed to reorder images", "error", err, "product_id", productID)
		http.Error(w, "Failed to reorder images", http.StatusInternalServerError)
		return
	}

	h.renderImageGrid(w, r, productID, csrfToken)
}

// UpdateAltText handles POST /admin/products/{id}/images/{imageId}/alt.
func (h *ImageHandler) UpdateAltText(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	altText := strPtr(r.FormValue("alt_text"))

	if err := h.media.UpdateAltText(ctx, imageID, altText); err != nil {
		h.logger.Error("failed to update alt text", "error", err, "image_id", imageID)
		http.Error(w, "Failed to update alt text", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<span class="text-success" style="font-size: 0.8rem;">Saved</span>`)
}

// AssignVariant handles POST /admin/products/{id}/images/{imageId}/assign.
func (h *ImageHandler) AssignVariant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	csrfToken := middleware.CSRFToken(r)

	productID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	imageID, err := uuid.Parse(r.PathValue("imageId"))
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	variantIDStr := r.FormValue("variant_id")
	var variantID pgtype.UUID
	if variantIDStr != "" {
		parsed, err := uuid.Parse(variantIDStr)
		if err != nil {
			http.Error(w, "Invalid variant ID", http.StatusBadRequest)
			return
		}
		variantID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	if err := h.media.AssignVariant(ctx, imageID, variantID, pgtype.UUID{}); err != nil {
		h.logger.Error("failed to assign variant", "error", err, "image_id", imageID)
		http.Error(w, "Failed to assign variant", http.StatusInternalServerError)
		return
	}

	h.renderImageGrid(w, r, productID, csrfToken)
}

// --- Internal helpers ---

// renderImageGrid fetches the current images and renders the grid fragment.
func (h *ImageHandler) renderImageGrid(w http.ResponseWriter, r *http.Request, productID uuid.UUID, csrfToken string) {
	ctx := r.Context()

	images, err := h.media.ListByProduct(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list product images for grid", "error", err, "product_id", productID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	variantItems := h.loadVariantItems(ctx, productID)

	items := make([]admin.ProductImageItem, 0, len(images))
	for _, img := range images {
		items = append(items, toImageItem(img))
	}

	data := admin.ProductImagesData{
		ProductID: productID.String(),
		Images:    items,
		Variants:  variantItems,
		CSRFToken: csrfToken,
	}

	w.Header().Set("Content-Type", "text/html")
	admin.ProductImagesGrid(data).Render(ctx, w)
}

// loadVariantItems loads variant data for the assignment dropdown.
func (h *ImageHandler) loadVariantItems(ctx context.Context, productID uuid.UUID) []admin.ImageVariantItem {
	if h.variants == nil {
		return nil
	}

	variants, err := h.variants.List(ctx, productID)
	if err != nil {
		h.logger.Error("failed to list variants for image assignment", "error", err, "product_id", productID)
		return nil
	}

	items := make([]admin.ImageVariantItem, 0, len(variants))
	for _, v := range variants {
		items = append(items, admin.ImageVariantItem{
			ID:  v.ID.String(),
			SKU: v.Sku,
		})
	}
	return items
}

// toImageItem converts a db.ProductImage to a template-friendly struct.
func toImageItem(img db.ProductImage) admin.ProductImageItem {
	variantIDStr := ""
	if img.VariantID.Valid {
		vid := uuid.UUID(img.VariantID.Bytes)
		variantIDStr = vid.String()
	}

	altText := ""
	if img.AltText != nil {
		altText = *img.AltText
	}

	return admin.ProductImageItem{
		ID:        img.ID.String(),
		URL:       img.Url,
		AltText:   altText,
		Position:  int(img.Position),
		IsPrimary: img.IsPrimary,
		VariantID: variantIDStr,
	}
}

package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
	"github.com/forgecommerce/api/internal/storage"
)

var (
	// ErrNotFound is returned when an image or asset does not exist.
	ErrNotFound = errors.New("image not found")

	// ErrInvalidContentType is returned when the uploaded file is not an image.
	ErrInvalidContentType = errors.New("invalid content type: only image files are allowed")

	// ErrFileTooLarge is returned when the uploaded file exceeds the size limit.
	ErrFileTooLarge = errors.New("file too large: maximum 10MB")

	// ErrInvalidMagicBytes is returned when the file content does not match any supported image format.
	ErrInvalidMagicBytes = errors.New("invalid file: content does not match a supported image format")
)

const maxFileSize = 10 * 1024 * 1024 // 10 MB

// allowedContentTypes maps MIME types to their expected magic byte signatures.
var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// Service provides business logic for media asset and product image management.
type Service struct {
	queries        *db.Queries
	pool           *pgxpool.Pool
	publicStorage  storage.Storage
	privateStorage storage.Storage // nil until private bucket features are needed
	logger         *slog.Logger
}

// NewService creates a new media service.
// publicStore handles product images (publicly accessible).
// privateStore handles internal files (pre-signed URL access). Pass nil if not needed yet.
func NewService(pool *pgxpool.Pool, publicStore storage.Storage, privateStore storage.Storage, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries:        db.New(pool),
		pool:           pool,
		publicStorage:  publicStore,
		privateStorage: privateStore,
		logger:         logger,
	}
}

// Upload validates and stores an uploaded image file, creating a MediaAsset record.
func (s *Service) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (db.MediaAsset, error) {
	// Validate file size
	if header.Size > maxFileSize {
		return db.MediaAsset{}, ErrFileTooLarge
	}

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	if !allowedContentTypes[contentType] {
		return db.MediaAsset{}, ErrInvalidContentType
	}

	// Read first 512 bytes for magic byte detection
	magicBuf := make([]byte, 512)
	n, err := file.Read(magicBuf)
	if err != nil && err != io.EOF {
		return db.MediaAsset{}, fmt.Errorf("reading file header: %w", err)
	}
	magicBuf = magicBuf[:n]

	if !isValidImageMagicBytes(magicBuf) {
		return db.MediaAsset{}, ErrInvalidMagicBytes
	}

	// Seek back to the beginning of the file
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return db.MediaAsset{}, fmt.Errorf("seeking file: %w", err)
		}
	}

	// Generate storage key
	assetID := uuid.New()
	sanitized := sanitizeFilename(header.Filename)
	key := fmt.Sprintf("%s-%s", assetID.String(), sanitized)

	// Upload to storage backend
	url, err := s.publicStorage.Put(ctx, key, file, contentType)
	if err != nil {
		return db.MediaAsset{}, fmt.Errorf("uploading to storage: %w", err)
	}

	now := time.Now().UTC()

	asset, err := s.queries.CreateMediaAsset(ctx, db.CreateMediaAssetParams{
		ID:               assetID,
		Filename:         key,
		OriginalFilename: header.Filename,
		ContentType:      contentType,
		SizeBytes:        header.Size,
		Url:              url,
		Width:            nil,
		Height:           nil,
		Metadata:         json.RawMessage(`{}`),
		CreatedAt:        now,
	})
	if err != nil {
		// Best-effort cleanup on DB failure
		if delErr := s.publicStorage.Delete(ctx, key); delErr != nil {
			s.logger.Warn("failed to clean up uploaded file after DB error",
				slog.String("key", key),
				slog.String("error", delErr.Error()),
			)
		}
		return db.MediaAsset{}, fmt.Errorf("creating media asset record: %w", err)
	}

	s.logger.Info("media asset uploaded",
		slog.String("asset_id", asset.ID.String()),
		slog.String("key", key),
		slog.Int64("size_bytes", header.Size),
	)

	return asset, nil
}

// Delete removes a media asset file from storage and its database record.
func (s *Service) Delete(ctx context.Context, assetID uuid.UUID) error {
	asset, err := s.queries.GetMediaAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("getting media asset: %w", err)
	}

	// Remove from storage using the stored filename as key
	if err := s.publicStorage.Delete(ctx, asset.Filename); err != nil {
		s.logger.Warn("failed to remove media file from storage",
			slog.String("key", asset.Filename),
			slog.String("error", err.Error()),
		)
	}

	if err := s.queries.DeleteMediaAsset(ctx, assetID); err != nil {
		return fmt.Errorf("deleting media asset record: %w", err)
	}

	s.logger.Info("media asset deleted", slog.String("asset_id", assetID.String()))
	return nil
}

// AssignToProduct creates a ProductImage record linking a media asset URL to a product.
func (s *Service) AssignToProduct(ctx context.Context, productID uuid.UUID, url string, variantID, optionID pgtype.UUID, altText *string, position int32, isPrimary bool) (db.ProductImage, error) {
	now := time.Now().UTC()

	// If this is being set as primary, unset any existing primary
	if isPrimary {
		if err := s.queries.UnsetPrimaryProductImages(ctx, productID); err != nil {
			return db.ProductImage{}, fmt.Errorf("unsetting primary images: %w", err)
		}
	}

	img, err := s.queries.CreateProductImage(ctx, db.CreateProductImageParams{
		ID:        uuid.New(),
		ProductID: productID,
		VariantID: variantID,
		OptionID:  optionID,
		Url:       url,
		AltText:   altText,
		Position:  position,
		IsPrimary: isPrimary,
		CreatedAt: now,
	})
	if err != nil {
		return db.ProductImage{}, fmt.Errorf("creating product image: %w", err)
	}

	s.logger.Info("product image assigned",
		slog.String("image_id", img.ID.String()),
		slog.String("product_id", productID.String()),
	)

	return img, nil
}

// RemoveFromProduct deletes a product image record and its associated media file.
func (s *Service) RemoveFromProduct(ctx context.Context, imageID uuid.UUID) error {
	img, err := s.queries.GetProductImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("getting product image: %w", err)
	}

	// Extract key from URL and remove from storage
	key := keyFromURL(img.Url)
	if key != "" {
		if err := s.publicStorage.Delete(ctx, key); err != nil {
			s.logger.Warn("failed to remove image file from storage",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}

	if err := s.queries.DeleteProductImage(ctx, imageID); err != nil {
		return fmt.Errorf("deleting product image: %w", err)
	}

	s.logger.Info("product image removed",
		slog.String("image_id", imageID.String()),
		slog.String("product_id", img.ProductID.String()),
	)
	return nil
}

// SetPrimary marks a specific image as the primary image for its product,
// unsetting any other primary image.
func (s *Service) SetPrimary(ctx context.Context, productID, imageID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Unset all existing primary images for the product
	if err := qtx.UnsetPrimaryProductImages(ctx, productID); err != nil {
		return fmt.Errorf("unsetting primary images: %w", err)
	}

	// Set the specified image as primary
	if err := qtx.SetPrimaryProductImage(ctx, imageID); err != nil {
		return fmt.Errorf("setting primary image: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing primary image update: %w", err)
	}

	s.logger.Info("primary image set",
		slog.String("image_id", imageID.String()),
		slog.String("product_id", productID.String()),
	)

	return nil
}

// ReorderImages updates the position of product images based on the order
// of the provided image IDs. The first ID gets position 0, the second gets 1, etc.
func (s *Service) ReorderImages(ctx context.Context, productID uuid.UUID, imageIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	for i, imgID := range imageIDs {
		if err := qtx.UpdateProductImagePosition(ctx, db.UpdateProductImagePositionParams{
			ID:       imgID,
			Position: int32(i),
		}); err != nil {
			return fmt.Errorf("updating position for image %s: %w", imgID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing image reorder: %w", err)
	}

	s.logger.Info("images reordered",
		slog.String("product_id", productID.String()),
		slog.Int("count", len(imageIDs)),
	)

	return nil
}

// ListByProduct returns all images for a product, ordered by position.
func (s *Service) ListByProduct(ctx context.Context, productID uuid.UUID) ([]db.ProductImage, error) {
	images, err := s.queries.ListProductImagesByProduct(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing product images: %w", err)
	}
	return images, nil
}

// ListByVariant returns all images for a specific variant, ordered by position.
func (s *Service) ListByVariant(ctx context.Context, variantID uuid.UUID) ([]db.ProductImage, error) {
	pgID := pgtype.UUID{Bytes: variantID, Valid: true}
	images, err := s.queries.ListProductImagesByVariant(ctx, pgID)
	if err != nil {
		return nil, fmt.Errorf("listing variant images: %w", err)
	}
	return images, nil
}

// AssignVariant updates the variant and option assignment for a product image.
func (s *Service) AssignVariant(ctx context.Context, imageID uuid.UUID, variantID, optionID pgtype.UUID) error {
	if err := s.queries.UpdateProductImageVariant(ctx, db.UpdateProductImageVariantParams{
		ID:        imageID,
		VariantID: variantID,
		OptionID:  optionID,
	}); err != nil {
		return fmt.Errorf("updating image variant assignment: %w", err)
	}
	return nil
}

// GetImage returns a single product image by ID.
func (s *Service) GetImage(ctx context.Context, imageID uuid.UUID) (db.ProductImage, error) {
	img, err := s.queries.GetProductImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductImage{}, ErrNotFound
		}
		return db.ProductImage{}, fmt.Errorf("getting product image: %w", err)
	}
	return img, nil
}

// UpdateAltText updates the alt text of a product image.
func (s *Service) UpdateAltText(ctx context.Context, imageID uuid.UUID, altText *string) error {
	// Get existing image to preserve position and is_primary
	img, err := s.queries.GetProductImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("getting image for alt text update: %w", err)
	}

	if err := s.queries.UpdateProductImage(ctx, db.UpdateProductImageParams{
		ID:        imageID,
		AltText:   altText,
		Position:  img.Position,
		IsPrimary: img.IsPrimary,
	}); err != nil {
		return fmt.Errorf("updating alt text: %w", err)
	}
	return nil
}

// --- Helpers ---

// keyFromURL extracts the storage key from a URL.
// For local URLs like "/media/uuid-file.webp" it returns "uuid-file.webp".
// For S3 URLs like "https://cdn.example.com/uuid-file.webp" it returns "uuid-file.webp".
// Returns empty string if the URL is empty.
func keyFromURL(url string) string {
	if url == "" {
		return ""
	}
	// Local: /media/key
	if strings.HasPrefix(url, "/media/") {
		return strings.TrimPrefix(url, "/media/")
	}
	// S3: https://domain/key — take everything after the last "/"
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		return url[idx+1:]
	}
	return url
}

// isValidImageMagicBytes checks the first bytes of a file against known image signatures.
func isValidImageMagicBytes(buf []byte) bool {
	if len(buf) < 4 {
		return false
	}

	// JPEG: starts with FF D8 FF
	if buf[0] == 0xFF && buf[1] == 0xD8 && buf[2] == 0xFF {
		return true
	}

	// PNG: starts with 89 50 4E 47
	if buf[0] == 0x89 && buf[1] == 0x50 && buf[2] == 0x4E && buf[3] == 0x47 {
		return true
	}

	// GIF: starts with "GIF8"
	if buf[0] == 0x47 && buf[1] == 0x49 && buf[2] == 0x46 && buf[3] == 0x38 {
		return true
	}

	// WebP: starts with "RIFF" then 4 bytes then "WEBP"
	if len(buf) >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		return true
	}

	return false
}

// sanitizeFilename removes path separators and problematic characters,
// keeping only alphanumeric, hyphens, underscores, and dots.
func sanitizeFilename(name string) string {
	// Get just the base filename, no path
	name = name[strings.LastIndex(name, "/")+1:]
	if i := strings.LastIndex(name, "\\"); i >= 0 {
		name = name[i+1:]
	}

	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			sb.WriteRune(r)
		}
	}

	result := sb.String()
	if result == "" || result == "." {
		result = "upload"
	}

	return result
}

package media_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/media"
	"github.com/forgecommerce/api/internal/storage"
)

// ---------------------------------------------------------------------------
// Mock storage — in-memory implementation of storage.Storage
// ---------------------------------------------------------------------------

type mockStorage struct {
	mu    sync.Mutex
	files map[string][]byte

	// Optional error injection.
	putErr    error
	deleteErr error
}

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) Put(_ context.Context, key string, body io.Reader, _ string) (string, error) {
	if m.putErr != nil {
		return "", m.putErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.files[key] = data
	m.mu.Unlock()
	return "/media/" + key, nil
}

func (m *mockStorage) Delete(_ context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	delete(m.files, key)
	m.mu.Unlock()
	return nil
}

func (m *mockStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "/media/" + key, nil
}

// Verify that mockStorage satisfies storage.Storage at compile time.
var _ storage.Storage = (*mockStorage)(nil)

// ---------------------------------------------------------------------------
// Mock multipart.File — wraps bytes.Reader to satisfy io.Reader + io.Seeker + io.Closer
// ---------------------------------------------------------------------------

type mockFile struct {
	*bytes.Reader
}

func (f *mockFile) Close() error { return nil }

// makeFileHeader builds a multipart.FileHeader with the given data and content type.
func makeFileHeader(filename string, contentType string, data []byte) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: filename,
		Size:     int64(len(data)),
		Header:   textproto.MIMEHeader{"Content-Type": {contentType}},
	}
}

// jpegData returns valid JPEG magic bytes followed by padding.
func jpegData() []byte {
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	data = append(data, bytes.Repeat([]byte{0x00}, 128)...)
	return data
}

// newServiceWithMock creates a media.Service backed by the given mock storage.
func newServiceWithMock(t *testing.T, store *mockStorage) *media.Service {
	t.Helper()
	return media.NewService(testDB.Pool, store, nil, nil)
}

// ---------------------------------------------------------------------------
// Upload tests
// ---------------------------------------------------------------------------

func TestUpload_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	data := jpegData()
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("test.jpg", "image/jpeg", data)

	asset, err := svc.Upload(ctx, file, header)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if asset.ID == uuid.Nil {
		t.Error("expected non-nil asset ID")
	}
	if asset.OriginalFilename != "test.jpg" {
		t.Errorf("original filename: got %q, want %q", asset.OriginalFilename, "test.jpg")
	}
	if asset.ContentType != "image/jpeg" {
		t.Errorf("content type: got %q, want %q", asset.ContentType, "image/jpeg")
	}
	if asset.SizeBytes != int64(len(data)) {
		t.Errorf("size: got %d, want %d", asset.SizeBytes, len(data))
	}
	if !strings.HasPrefix(asset.Url, "/media/") {
		t.Errorf("URL should start with /media/, got %q", asset.Url)
	}

	// Verify file was stored in mock storage.
	store.mu.Lock()
	stored := len(store.files)
	store.mu.Unlock()
	if stored != 1 {
		t.Errorf("expected 1 file in storage, got %d", stored)
	}
}

func TestUpload_FileTooLarge(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	data := jpegData()
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("big.jpg", "image/jpeg", data)
	header.Size = 11 * 1024 * 1024 // 11 MB — exceeds 10 MB limit

	_, err := svc.Upload(ctx, file, header)
	if !errors.Is(err, media.ErrFileTooLarge) {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestUpload_InvalidContentType(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	data := []byte("not an image")
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("file.txt", "text/plain", data)

	_, err := svc.Upload(ctx, file, header)
	if !errors.Is(err, media.ErrInvalidContentType) {
		t.Errorf("expected ErrInvalidContentType, got %v", err)
	}
}

func TestUpload_InvalidMagicBytes(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	// Claims to be JPEG but has PDF magic bytes.
	data := []byte{0x25, 0x50, 0x44, 0x46, 0x2D} // %PDF-
	data = append(data, bytes.Repeat([]byte{0x00}, 128)...)
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("fake.jpg", "image/jpeg", data)

	_, err := svc.Upload(ctx, file, header)
	if !errors.Is(err, media.ErrInvalidMagicBytes) {
		t.Errorf("expected ErrInvalidMagicBytes, got %v", err)
	}
}

func TestUpload_StoragePutError(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	store.putErr = fmt.Errorf("simulated storage failure")
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	data := jpegData()
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("test.jpg", "image/jpeg", data)

	_, err := svc.Upload(ctx, file, header)
	if err == nil {
		t.Fatal("expected error from storage Put failure")
	}
	if !strings.Contains(err.Error(), "uploading to storage") {
		t.Errorf("error should mention uploading to storage, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDelete_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	// Upload first.
	data := jpegData()
	file := &mockFile{Reader: bytes.NewReader(data)}
	header := makeFileHeader("to-delete.jpg", "image/jpeg", data)

	asset, err := svc.Upload(ctx, file, header)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Delete the uploaded asset.
	err = svc.Delete(ctx, asset.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify file removed from mock storage.
	store.mu.Lock()
	stored := len(store.files)
	store.mu.Unlock()
	if stored != 0 {
		t.Errorf("expected 0 files in storage after delete, got %d", stored)
	}
}

func TestDelete_NotFound(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	err := svc.Delete(ctx, uuid.New())
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AssignToProduct tests (error paths and additional coverage)
// ---------------------------------------------------------------------------

func TestIntegration_AssignToProduct_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Assign Test", "assign-test")

	alt := "Product front"
	img, err := svc.AssignToProduct(ctx, product.ID, "/media/front.webp",
		pgtype.UUID{}, pgtype.UUID{}, &alt, 0, false)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	if img.ID == uuid.Nil {
		t.Error("expected non-nil image ID")
	}
	if img.ProductID != product.ID {
		t.Error("product ID mismatch")
	}
	if img.IsPrimary {
		t.Error("expected is_primary=false")
	}
}

func TestIntegration_AssignToProduct_SetAsPrimary(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Primary Assign", "primary-assign")

	// First image as primary.
	img1, err := svc.AssignToProduct(ctx, product.ID, "/media/first.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)
	if err != nil {
		t.Fatalf("AssignToProduct first: %v", err)
	}
	if !img1.IsPrimary {
		t.Error("first image should be primary")
	}

	// Second image as primary — should unset first.
	img2, err := svc.AssignToProduct(ctx, product.ID, "/media/second.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, true)
	if err != nil {
		t.Fatalf("AssignToProduct second: %v", err)
	}
	if !img2.IsPrimary {
		t.Error("second image should be primary")
	}

	// Verify first is no longer primary.
	got1, err := svc.GetImage(ctx, img1.ID)
	if err != nil {
		t.Fatalf("GetImage img1: %v", err)
	}
	if got1.IsPrimary {
		t.Error("first image should no longer be primary")
	}
}

// ---------------------------------------------------------------------------
// RemoveFromProduct tests (error paths)
// ---------------------------------------------------------------------------

func TestIntegration_RemoveFromProduct_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Remove Int", "remove-int")

	img, err := svc.AssignToProduct(ctx, product.ID, "/media/to-remove-int.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	err = svc.RemoveFromProduct(ctx, img.ID)
	if err != nil {
		t.Fatalf("RemoveFromProduct: %v", err)
	}

	_, err = svc.GetImage(ctx, img.ID)
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_RemoveFromProduct_NotFound(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	err := svc.RemoveFromProduct(ctx, uuid.New())
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetPrimary tests
// ---------------------------------------------------------------------------

func TestIntegration_SetPrimary(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Set Primary Int", "set-primary-int")

	img1, err := svc.AssignToProduct(ctx, product.ID, "/media/sp-1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)
	if err != nil {
		t.Fatalf("AssignToProduct img1: %v", err)
	}

	img2, err := svc.AssignToProduct(ctx, product.ID, "/media/sp-2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)
	if err != nil {
		t.Fatalf("AssignToProduct img2: %v", err)
	}

	// Switch primary to img2.
	err = svc.SetPrimary(ctx, product.ID, img2.ID)
	if err != nil {
		t.Fatalf("SetPrimary: %v", err)
	}

	// Verify img2 is now primary.
	got2, err := svc.GetImage(ctx, img2.ID)
	if err != nil {
		t.Fatalf("GetImage img2: %v", err)
	}
	if !got2.IsPrimary {
		t.Error("img2 should be primary")
	}

	// Verify img1 is no longer primary.
	got1, err := svc.GetImage(ctx, img1.ID)
	if err != nil {
		t.Fatalf("GetImage img1: %v", err)
	}
	if got1.IsPrimary {
		t.Error("img1 should NOT be primary after switching")
	}
}

// ---------------------------------------------------------------------------
// ReorderImages tests
// ---------------------------------------------------------------------------

func TestIntegration_ReorderImages(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Reorder Int", "reorder-int")

	img1, _ := svc.AssignToProduct(ctx, product.ID, "/media/r1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)
	img2, _ := svc.AssignToProduct(ctx, product.ID, "/media/r2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)
	img3, _ := svc.AssignToProduct(ctx, product.ID, "/media/r3.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 2, false)

	// Reverse order.
	err := svc.ReorderImages(ctx, product.ID, []uuid.UUID{img3.ID, img2.ID, img1.ID})
	if err != nil {
		t.Fatalf("ReorderImages: %v", err)
	}

	got3, _ := svc.GetImage(ctx, img3.ID)
	got2, _ := svc.GetImage(ctx, img2.ID)
	got1, _ := svc.GetImage(ctx, img1.ID)

	if got3.Position != 0 {
		t.Errorf("img3 position: got %d, want 0", got3.Position)
	}
	if got2.Position != 1 {
		t.Errorf("img2 position: got %d, want 1", got2.Position)
	}
	if got1.Position != 2 {
		t.Errorf("img1 position: got %d, want 2", got1.Position)
	}
}

// ---------------------------------------------------------------------------
// ListByProduct tests
// ---------------------------------------------------------------------------

func TestIntegration_ListByProduct_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "List Int", "list-int")

	svc.AssignToProduct(ctx, product.ID, "/media/li1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)
	svc.AssignToProduct(ctx, product.ID, "/media/li2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)

	images, err := svc.ListByProduct(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestIntegration_ListByProduct_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Empty List", "empty-list")

	images, err := svc.ListByProduct(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

// ---------------------------------------------------------------------------
// ListByVariant tests
// ---------------------------------------------------------------------------

func TestIntegration_ListByVariant(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Var Img Int", "var-img-int")
	v1 := testDB.FixtureVariant(t, product.ID, "VINT-001", 5)

	v1UUID := pgtype.UUID{Bytes: v1.ID, Valid: true}

	svc.AssignToProduct(ctx, product.ID, "/media/vi1.webp",
		v1UUID, pgtype.UUID{}, nil, 0, false)
	svc.AssignToProduct(ctx, product.ID, "/media/vi2.webp",
		v1UUID, pgtype.UUID{}, nil, 1, false)

	images, err := svc.ListByVariant(ctx, v1.ID)
	if err != nil {
		t.Fatalf("ListByVariant: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

// ---------------------------------------------------------------------------
// GetImage tests
// ---------------------------------------------------------------------------

func TestIntegration_GetImage_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Get Img", "get-img")

	alt := "Side view"
	img, err := svc.AssignToProduct(ctx, product.ID, "/media/side.webp",
		pgtype.UUID{}, pgtype.UUID{}, &alt, 0, false)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	got, err := svc.GetImage(ctx, img.ID)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if got.ID != img.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, img.ID)
	}
	if got.AltText == nil || *got.AltText != "Side view" {
		t.Errorf("alt text: got %v, want %q", got.AltText, "Side view")
	}
}

func TestIntegration_GetImage_NotFound(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	_, err := svc.GetImage(ctx, uuid.New())
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateAltText tests
// ---------------------------------------------------------------------------

func TestIntegration_UpdateAltText_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Alt Int", "alt-int")

	img, err := svc.AssignToProduct(ctx, product.ID, "/media/alt-int.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	newAlt := "Updated description"
	err = svc.UpdateAltText(ctx, img.ID, &newAlt)
	if err != nil {
		t.Fatalf("UpdateAltText: %v", err)
	}

	got, err := svc.GetImage(ctx, img.ID)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if got.AltText == nil || *got.AltText != newAlt {
		t.Errorf("alt text: got %v, want %q", got.AltText, newAlt)
	}
}

func TestIntegration_UpdateAltText_NotFound(t *testing.T) {
	testDB.Truncate(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	alt := "does not matter"
	err := svc.UpdateAltText(ctx, uuid.New(), &alt)
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AssignVariant tests
// ---------------------------------------------------------------------------

func TestIntegration_AssignVariant(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)

	store := newMockStorage()
	svc := newServiceWithMock(t, store)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "AV Int", "av-int")
	variant := testDB.FixtureVariant(t, product.ID, "AVINT-001", 10)

	// Create image with no variant.
	img, err := svc.AssignToProduct(ctx, product.ID, "/media/unassigned-int.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	// Assign to variant.
	variantUUID := pgtype.UUID{Bytes: variant.ID, Valid: true}
	err = svc.AssignVariant(ctx, img.ID, variantUUID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("AssignVariant: %v", err)
	}

	// Verify via ListByVariant.
	images, err := svc.ListByVariant(ctx, variant.ID)
	if err != nil {
		t.Fatalf("ListByVariant: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 image after assignment, got %d", len(images))
	}
}

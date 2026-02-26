package media_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/media"
	"github.com/forgecommerce/api/internal/storage"
	"github.com/forgecommerce/api/internal/testutil"
)

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	db, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer db.Close()
	testDB = db

	code = m.Run()
}

func newService(t *testing.T) *media.Service {
	t.Helper()
	dir := t.TempDir()
	publicStore := storage.NewLocal(dir, "/media")
	return media.NewService(testDB.Pool, publicStore, nil, nil)
}

// --------------------------------------------------------------------------
// AssignToProduct + ListByProduct
// --------------------------------------------------------------------------

func TestAssignToProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Image Product", "image-product")

	alt := "Front view"
	img, err := svc.AssignToProduct(ctx, product.ID, "/media/test-image.webp",
		pgtype.UUID{}, pgtype.UUID{}, &alt, 0, true)
	if err != nil {
		t.Fatalf("AssignToProduct: %v", err)
	}

	if img.ID == uuid.Nil {
		t.Error("expected non-nil image ID")
	}
	if img.ProductID != product.ID {
		t.Error("product ID mismatch")
	}
	if !img.IsPrimary {
		t.Error("expected is_primary=true")
	}
	if img.AltText == nil || *img.AltText != "Front view" {
		t.Errorf("alt text: got %v, want %q", img.AltText, "Front view")
	}
}

func TestListByProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Multi Image", "multi-image")

	svc.AssignToProduct(ctx, product.ID, "/media/img1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)
	svc.AssignToProduct(ctx, product.ID, "/media/img2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)
	svc.AssignToProduct(ctx, product.ID, "/media/img3.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 2, false)

	images, err := svc.ListByProduct(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(images) != 3 {
		t.Errorf("expected 3 images, got %d", len(images))
	}
}

func TestListByProduct_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "No Images", "no-images")

	images, err := svc.ListByProduct(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListByProduct: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

// --------------------------------------------------------------------------
// SetPrimary
// --------------------------------------------------------------------------

func TestSetPrimary(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Primary Test", "primary-test")

	img1, _ := svc.AssignToProduct(ctx, product.ID, "/media/img1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)
	img2, _ := svc.AssignToProduct(ctx, product.ID, "/media/img2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)

	// Switch primary to img2.
	err := svc.SetPrimary(ctx, product.ID, img2.ID)
	if err != nil {
		t.Fatalf("SetPrimary: %v", err)
	}

	// Verify img2 is now primary.
	got2, _ := svc.GetImage(ctx, img2.ID)
	if !got2.IsPrimary {
		t.Error("expected img2 to be primary")
	}

	// Verify img1 is no longer primary.
	got1, _ := svc.GetImage(ctx, img1.ID)
	if got1.IsPrimary {
		t.Error("expected img1 to NOT be primary after switching")
	}
}

// --------------------------------------------------------------------------
// RemoveFromProduct
// --------------------------------------------------------------------------

func TestRemoveFromProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Remove Test", "remove-test")

	img, _ := svc.AssignToProduct(ctx, product.ID, "/media/to-remove.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)

	err := svc.RemoveFromProduct(ctx, img.ID)
	if err != nil {
		t.Fatalf("RemoveFromProduct: %v", err)
	}

	// Verify it's gone.
	_, err = svc.GetImage(ctx, img.ID)
	if err != media.ErrNotFound {
		t.Errorf("expected ErrNotFound after removal, got %v", err)
	}
}

func TestRemoveFromProduct_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService(t)
	ctx := context.Background()

	err := svc.RemoveFromProduct(ctx, uuid.New())
	if err != media.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// ReorderImages
// --------------------------------------------------------------------------

func TestReorderImages(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Reorder Test", "reorder-test")

	img1, _ := svc.AssignToProduct(ctx, product.ID, "/media/img1.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)
	img2, _ := svc.AssignToProduct(ctx, product.ID, "/media/img2.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, false)
	img3, _ := svc.AssignToProduct(ctx, product.ID, "/media/img3.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 2, false)

	// Reverse the order: img3 first, img2 second, img1 third.
	err := svc.ReorderImages(ctx, product.ID, []uuid.UUID{img3.ID, img2.ID, img1.ID})
	if err != nil {
		t.Fatalf("ReorderImages: %v", err)
	}

	// Verify new positions.
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

// --------------------------------------------------------------------------
// ListByVariant
// --------------------------------------------------------------------------

func TestListByVariant(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Variant Images", "variant-images")
	v1 := testDB.FixtureVariant(t, product.ID, "VAR-001", 10)
	v2 := testDB.FixtureVariant(t, product.ID, "VAR-002", 10)

	v1UUID := pgtype.UUID{Bytes: v1.ID, Valid: true}
	v2UUID := pgtype.UUID{Bytes: v2.ID, Valid: true}

	// 2 images for variant 1, 1 for variant 2.
	svc.AssignToProduct(ctx, product.ID, "/media/v1-a.webp",
		v1UUID, pgtype.UUID{}, nil, 0, false)
	svc.AssignToProduct(ctx, product.ID, "/media/v1-b.webp",
		v1UUID, pgtype.UUID{}, nil, 1, false)
	svc.AssignToProduct(ctx, product.ID, "/media/v2-a.webp",
		v2UUID, pgtype.UUID{}, nil, 0, false)

	images, err := svc.ListByVariant(ctx, v1.ID)
	if err != nil {
		t.Fatalf("ListByVariant: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images for variant 1, got %d", len(images))
	}
}

// --------------------------------------------------------------------------
// AssignVariant
// --------------------------------------------------------------------------

func TestAssignVariant(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Assign Variant", "assign-variant")
	variant := testDB.FixtureVariant(t, product.ID, "ASG-001", 10)

	// Create an image with no variant.
	img, _ := svc.AssignToProduct(ctx, product.ID, "/media/unassigned.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)

	// Assign it to a variant.
	variantUUID := pgtype.UUID{Bytes: variant.ID, Valid: true}
	err := svc.AssignVariant(ctx, img.ID, variantUUID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("AssignVariant: %v", err)
	}

	// Verify via ListByVariant.
	images, _ := svc.ListByVariant(ctx, variant.ID)
	if len(images) != 1 {
		t.Errorf("expected 1 image after assignment, got %d", len(images))
	}
}

// --------------------------------------------------------------------------
// UpdateAltText
// --------------------------------------------------------------------------

func TestUpdateAltText(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Alt Text Product", "alt-text")

	img, _ := svc.AssignToProduct(ctx, product.ID, "/media/alt.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, false)

	newAlt := "Updated alt text"
	err := svc.UpdateAltText(ctx, img.ID, &newAlt)
	if err != nil {
		t.Fatalf("UpdateAltText: %v", err)
	}

	got, _ := svc.GetImage(ctx, img.ID)
	if got.AltText == nil || *got.AltText != newAlt {
		t.Errorf("alt text: got %v, want %q", got.AltText, newAlt)
	}
}

func TestUpdateAltText_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService(t)
	ctx := context.Background()

	alt := "some text"
	err := svc.UpdateAltText(ctx, uuid.New(), &alt)
	if err != media.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// GetImage
// --------------------------------------------------------------------------

func TestGetImage_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService(t)
	ctx := context.Background()

	_, err := svc.GetImage(ctx, uuid.New())
	if err != media.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// AssignToProduct unsets existing primary
// --------------------------------------------------------------------------

func TestAssignToProduct_UnsetsExistingPrimary(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService(t)
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Primary Unset", "primary-unset")

	img1, _ := svc.AssignToProduct(ctx, product.ID, "/media/first.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 0, true)

	// Assign second as primary — should unset first.
	_, err := svc.AssignToProduct(ctx, product.ID, "/media/second.webp",
		pgtype.UUID{}, pgtype.UUID{}, nil, 1, true)
	if err != nil {
		t.Fatalf("AssignToProduct second primary: %v", err)
	}

	got1, _ := svc.GetImage(ctx, img1.ID)
	if got1.IsPrimary {
		t.Error("expected first image to no longer be primary")
	}
}

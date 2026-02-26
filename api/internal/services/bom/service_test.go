package bom_test

import (
	"context"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/bom"
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

func newService() *bom.Service {
	return bom.NewService(testDB.Pool, nil)
}

func numeric(n int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(n), Exp: 0, Valid: true}
}

func numericDecimal(n int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(n), Exp: exp, Valid: true}
}

// --------------------------------------------------------------------------
// Layer 1: Product BOM Entries
// --------------------------------------------------------------------------

func TestCreateProductEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "BOM Product", "bom-product")
	material := testDB.FixtureRawMaterial(t, "Thread", "THR-001")

	entry, err := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: material.ID,
		Quantity:      numericDecimal(300, -2), // 3.00
		UnitOfMeasure: "m",
		IsRequired:    true,
	})
	if err != nil {
		t.Fatalf("CreateProductEntry: %v", err)
	}

	if entry.ID == uuid.Nil {
		t.Error("expected non-nil entry ID")
	}
	if entry.ProductID != product.ID {
		t.Error("product ID mismatch")
	}
	if entry.RawMaterialID != material.ID {
		t.Error("material ID mismatch")
	}
	if !entry.IsRequired {
		t.Error("expected is_required=true")
	}
}

func TestListProductEntries(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "BOM Product", "bom-product")
	m1 := testDB.FixtureRawMaterial(t, "Thread", "THR-001")
	m2 := testDB.FixtureRawMaterial(t, "Buckle", "BCK-001")

	svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: m1.ID,
		Quantity:      numeric(3),
		UnitOfMeasure: "m",
		IsRequired:    true,
	})
	svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: m2.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "unit",
		IsRequired:    true,
	})

	entries, err := svc.ListProductEntries(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListProductEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestListProductEntries_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Empty Product", "empty-product")

	entries, err := svc.ListProductEntries(ctx, product.ID)
	if err != nil {
		t.Fatalf("ListProductEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestUpdateProductEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "BOM Product", "bom-product")
	material := testDB.FixtureRawMaterial(t, "Thread", "THR-001")

	entry, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: material.ID,
		Quantity:      numeric(3),
		UnitOfMeasure: "m",
		IsRequired:    true,
	})

	notes := "Updated requirement"
	updated, err := svc.UpdateProductEntry(ctx, entry.ID, bom.UpdateProductEntryParams{
		Quantity:      numeric(5),
		UnitOfMeasure: "m",
		IsRequired:    false,
		Notes:         &notes,
	})
	if err != nil {
		t.Fatalf("UpdateProductEntry: %v", err)
	}

	if updated.IsRequired {
		t.Error("expected is_required=false after update")
	}
	if updated.Notes == nil || *updated.Notes != notes {
		t.Errorf("notes: got %v, want %q", updated.Notes, notes)
	}
}

func TestDeleteProductEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "BOM Product", "bom-product")
	material := testDB.FixtureRawMaterial(t, "Thread", "THR-001")

	entry, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: material.ID,
		Quantity:      numeric(3),
		UnitOfMeasure: "m",
		IsRequired:    true,
	})

	err := svc.DeleteProductEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("DeleteProductEntry: %v", err)
	}

	entries, _ := svc.ListProductEntries(ctx, product.ID)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}

// --------------------------------------------------------------------------
// Layer 2a: Option BOM Entries
// --------------------------------------------------------------------------

func TestCreateOptionEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Option Product", "option-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "color", "black")
	material := testDB.FixtureRawMaterial(t, "Black Dye", "BLK-DYE")

	entry, err := svc.CreateOptionEntry(ctx, bom.CreateOptionEntryParams{
		OptionID:      option.ID,
		RawMaterialID: material.ID,
		Quantity:      numericDecimal(50, -2), // 0.50
		UnitOfMeasure: "l",
	})
	if err != nil {
		t.Fatalf("CreateOptionEntry: %v", err)
	}

	if entry.ID == uuid.Nil {
		t.Error("expected non-nil entry ID")
	}
	if entry.OptionID != option.ID {
		t.Error("option ID mismatch")
	}
}

func TestListOptionEntries(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Option Product", "option-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "color", "black")
	m1 := testDB.FixtureRawMaterial(t, "Black Dye", "BLK-DYE")
	m2 := testDB.FixtureRawMaterial(t, "Black Leather", "BLK-LTH")

	svc.CreateOptionEntry(ctx, bom.CreateOptionEntryParams{
		OptionID:      option.ID,
		RawMaterialID: m1.ID,
		Quantity:      numericDecimal(50, -2),
		UnitOfMeasure: "l",
	})
	svc.CreateOptionEntry(ctx, bom.CreateOptionEntryParams{
		OptionID:      option.ID,
		RawMaterialID: m2.ID,
		Quantity:      numericDecimal(200, -2),
		UnitOfMeasure: "m2",
	})

	entries, err := svc.ListOptionEntries(ctx, option.ID)
	if err != nil {
		t.Fatalf("ListOptionEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 option entries, got %d", len(entries))
	}
}

func TestDeleteOptionEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Option Product", "option-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "color", "black")
	material := testDB.FixtureRawMaterial(t, "Black Dye", "BLK-DYE")

	entry, _ := svc.CreateOptionEntry(ctx, bom.CreateOptionEntryParams{
		OptionID:      option.ID,
		RawMaterialID: material.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "l",
	})

	err := svc.DeleteOptionEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("DeleteOptionEntry: %v", err)
	}

	entries, _ := svc.ListOptionEntries(ctx, option.ID)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}

// --------------------------------------------------------------------------
// Layer 2b: Option BOM Modifiers
// --------------------------------------------------------------------------

func TestCreateOptionModifier(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Modifier Product", "mod-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "size", "large")
	material := testDB.FixtureRawMaterial(t, "Leather", "LTH-001")

	// Create a product BOM entry first (Layer 1) — the modifier adjusts this.
	bomEntry, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: material.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "m2",
		IsRequired:    true,
	})

	modifier, err := svc.CreateOptionModifier(ctx, bom.CreateOptionModifierParams{
		OptionID:          option.ID,
		ProductBomEntryID: bomEntry.ID,
		ModifierType:      "multiply",
		ModifierValue:     numericDecimal(140, -2), // 1.40 = 40% more
	})
	if err != nil {
		t.Fatalf("CreateOptionModifier: %v", err)
	}

	if modifier.ID == uuid.Nil {
		t.Error("expected non-nil modifier ID")
	}
	if modifier.ModifierType != "multiply" {
		t.Errorf("type: got %q, want %q", modifier.ModifierType, "multiply")
	}
}

func TestListOptionModifiers(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Modifier Product", "mod-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "size", "large")
	m1 := testDB.FixtureRawMaterial(t, "Leather", "LTH-001")
	m2 := testDB.FixtureRawMaterial(t, "Thread", "THR-001")

	entry1, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: m1.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "m2",
		IsRequired:    true,
	})
	entry2, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: m2.ID,
		Quantity:      numeric(3),
		UnitOfMeasure: "m",
		IsRequired:    true,
	})

	svc.CreateOptionModifier(ctx, bom.CreateOptionModifierParams{
		OptionID:          option.ID,
		ProductBomEntryID: entry1.ID,
		ModifierType:      "multiply",
		ModifierValue:     numericDecimal(140, -2),
	})
	svc.CreateOptionModifier(ctx, bom.CreateOptionModifierParams{
		OptionID:          option.ID,
		ProductBomEntryID: entry2.ID,
		ModifierType:      "add",
		ModifierValue:     numeric(2),
	})

	modifiers, err := svc.ListOptionModifiers(ctx, option.ID)
	if err != nil {
		t.Fatalf("ListOptionModifiers: %v", err)
	}
	if len(modifiers) != 2 {
		t.Errorf("expected 2 modifiers, got %d", len(modifiers))
	}
}

func TestDeleteOptionModifier(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Modifier Product", "mod-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "size", "large")
	material := testDB.FixtureRawMaterial(t, "Leather", "LTH-001")

	bomEntry, _ := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: material.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "m2",
		IsRequired:    true,
	})

	modifier, _ := svc.CreateOptionModifier(ctx, bom.CreateOptionModifierParams{
		OptionID:          option.ID,
		ProductBomEntryID: bomEntry.ID,
		ModifierType:      "multiply",
		ModifierValue:     numericDecimal(140, -2),
	})

	err := svc.DeleteOptionModifier(ctx, modifier.ID)
	if err != nil {
		t.Fatalf("DeleteOptionModifier: %v", err)
	}

	modifiers, _ := svc.ListOptionModifiers(ctx, option.ID)
	if len(modifiers) != 0 {
		t.Errorf("expected 0 modifiers after delete, got %d", len(modifiers))
	}
}

// --------------------------------------------------------------------------
// Layer 3: Variant BOM Overrides
// --------------------------------------------------------------------------

func TestCreateVariantOverride(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Override Product", "override-product")
	variant := testDB.FixtureVariant(t, product.ID, "OVR-BLK-LG", 10)
	material := testDB.FixtureRawMaterial(t, "Antique Buckle", "ANT-BCK")

	uom := "unit"
	override, err := svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:     variant.ID,
		RawMaterialID: material.ID,
		OverrideType:  "add",
		Quantity:      numeric(1),
		UnitOfMeasure: &uom,
	})
	if err != nil {
		t.Fatalf("CreateVariantOverride: %v", err)
	}

	if override.ID == uuid.Nil {
		t.Error("expected non-nil override ID")
	}
	if override.OverrideType != "add" {
		t.Errorf("type: got %q, want %q", override.OverrideType, "add")
	}
}

func TestCreateVariantOverride_Replace(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Replace Product", "replace-product")
	variant := testDB.FixtureVariant(t, product.ID, "REP-001", 10)
	oldMaterial := testDB.FixtureRawMaterial(t, "Brass Buckle", "BRS-BCK")
	newMaterial := testDB.FixtureRawMaterial(t, "Antique Buckle", "ANT-BCK")

	uom := "unit"
	override, err := svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:          variant.ID,
		RawMaterialID:      newMaterial.ID,
		OverrideType:       "replace",
		ReplacesMaterialID: pgtype.UUID{Bytes: oldMaterial.ID, Valid: true},
		Quantity:           numeric(1),
		UnitOfMeasure:      &uom,
	})
	if err != nil {
		t.Fatalf("CreateVariantOverride replace: %v", err)
	}

	if override.OverrideType != "replace" {
		t.Errorf("type: got %q, want %q", override.OverrideType, "replace")
	}
	if !override.ReplacesMaterialID.Valid || override.ReplacesMaterialID.Bytes != oldMaterial.ID {
		t.Errorf("replaces_material_id: got %v, want %s", override.ReplacesMaterialID, oldMaterial.ID)
	}
}

func TestListVariantOverrides(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Override Product", "override-product")
	variant := testDB.FixtureVariant(t, product.ID, "OVR-001", 10)
	m1 := testDB.FixtureRawMaterial(t, "Material A", "MAT-A")
	m2 := testDB.FixtureRawMaterial(t, "Material B", "MAT-B")

	uom := "unit"
	svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:     variant.ID,
		RawMaterialID: m1.ID,
		OverrideType:  "add",
		Quantity:      numeric(1),
		UnitOfMeasure: &uom,
	})
	svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:     variant.ID,
		RawMaterialID: m2.ID,
		OverrideType:  "remove",
	})

	overrides, err := svc.ListVariantOverrides(ctx, variant.ID)
	if err != nil {
		t.Fatalf("ListVariantOverrides: %v", err)
	}
	if len(overrides) != 2 {
		t.Errorf("expected 2 overrides, got %d", len(overrides))
	}
}

func TestListVariantOverrides_Empty(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "No Overrides", "no-overrides")
	variant := testDB.FixtureVariant(t, product.ID, "NO-OVR-001", 10)

	overrides, err := svc.ListVariantOverrides(ctx, variant.ID)
	if err != nil {
		t.Fatalf("ListVariantOverrides: %v", err)
	}
	if len(overrides) != 0 {
		t.Errorf("expected 0 overrides, got %d", len(overrides))
	}
}

func TestDeleteVariantOverride(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Delete Override", "del-override")
	variant := testDB.FixtureVariant(t, product.ID, "DEL-OVR-001", 10)
	material := testDB.FixtureRawMaterial(t, "Extra Material", "EXT-001")

	uom := "unit"
	override, _ := svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:     variant.ID,
		RawMaterialID: material.ID,
		OverrideType:  "add",
		Quantity:      numeric(1),
		UnitOfMeasure: &uom,
	})

	err := svc.DeleteVariantOverride(ctx, override.ID)
	if err != nil {
		t.Fatalf("DeleteVariantOverride: %v", err)
	}

	overrides, _ := svc.ListVariantOverrides(ctx, variant.ID)
	if len(overrides) != 0 {
		t.Errorf("expected 0 overrides after delete, got %d", len(overrides))
	}
}

// --------------------------------------------------------------------------
// Error paths: FK constraint violations on Create operations
// --------------------------------------------------------------------------

func TestCreateProductEntry_InvalidRawMaterial(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "FK Product", "fk-product")
	bogusID := uuid.New() // non-existent raw material

	_, err := svc.CreateProductEntry(ctx, bom.CreateProductEntryParams{
		ProductID:     product.ID,
		RawMaterialID: bogusID,
		Quantity:      numeric(1),
		UnitOfMeasure: "unit",
		IsRequired:    true,
	})
	if err == nil {
		t.Fatal("expected FK violation error for non-existent raw material, got nil")
	}
}

func TestCreateOptionEntry_InvalidOption(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	material := testDB.FixtureRawMaterial(t, "Some Material", "SOM-001")
	bogusOptionID := uuid.New() // non-existent option

	_, err := svc.CreateOptionEntry(ctx, bom.CreateOptionEntryParams{
		OptionID:      bogusOptionID,
		RawMaterialID: material.ID,
		Quantity:      numeric(1),
		UnitOfMeasure: "unit",
	})
	if err == nil {
		t.Fatal("expected FK violation error for non-existent option, got nil")
	}
}

func TestCreateOptionModifier_InvalidBOMEntry(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	product := testDB.FixtureProduct(t, "Modifier FK Product", "mod-fk-product")
	option := testDB.FixtureAttributeOption(t, product.ID, "size", "large")
	bogusBOMEntryID := uuid.New() // non-existent product_bom_entry

	_, err := svc.CreateOptionModifier(ctx, bom.CreateOptionModifierParams{
		OptionID:          option.ID,
		ProductBomEntryID: bogusBOMEntryID,
		ModifierType:      "multiply",
		ModifierValue:     numericDecimal(140, -2),
	})
	if err == nil {
		t.Fatal("expected FK violation error for non-existent product BOM entry, got nil")
	}
}

func TestCreateVariantOverride_InvalidVariant(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	material := testDB.FixtureRawMaterial(t, "Override Material", "OVR-MAT")
	bogusVariantID := uuid.New() // non-existent variant
	uom := "unit"

	_, err := svc.CreateVariantOverride(ctx, bom.CreateVariantOverrideParams{
		VariantID:     bogusVariantID,
		RawMaterialID: material.ID,
		OverrideType:  "add",
		Quantity:      numeric(1),
		UnitOfMeasure: &uom,
	})
	if err == nil {
		t.Fatal("expected FK violation error for non-existent variant, got nil")
	}
}

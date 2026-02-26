package globalattr_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/forgecommerce/api/internal/services/globalattr"
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

func newService() *globalattr.Service {
	return globalattr.NewService(testDB.Pool, nil)
}

// createAttr is a helper to create a test attribute.
func createAttr(t *testing.T, svc *globalattr.Service, name string) uuid.UUID {
	t.Helper()
	attr, err := svc.Create(context.Background(), globalattr.CreateParams{
		Name:        name,
		DisplayName: name,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("creating attribute %q: %v", name, err)
	}
	return attr.ID
}

// --------------------------------------------------------------------------
// Attribute CRUD
// --------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attr, err := svc.Create(ctx, globalattr.CreateParams{
		Name:          "color",
		DisplayName:   "Color",
		AttributeType: "color_swatch",
		IsActive:      true,
		Position:      1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if attr.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if attr.Name != "color" {
		t.Errorf("name: got %q, want %q", attr.Name, "color")
	}
	if attr.AttributeType != "color_swatch" {
		t.Errorf("type: got %q, want %q", attr.AttributeType, "color_swatch")
	}
}

func TestCreate_DefaultType(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attr, err := svc.Create(ctx, globalattr.CreateParams{
		Name:     "size",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if attr.AttributeType != "select" {
		t.Errorf("type: got %q, want %q (default)", attr.AttributeType, "select")
	}
}

func TestCreate_EmptyName(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Create(ctx, globalattr.CreateParams{IsActive: true})
	if err != globalattr.ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestCreate_DuplicateName(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	svc.Create(ctx, globalattr.CreateParams{Name: "dupname", IsActive: true})
	_, err := svc.Create(ctx, globalattr.CreateParams{Name: "dupname", IsActive: true})
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestGet(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "get-test")

	got, err := svc.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("name: got %q, want %q", got.Name, "get-test")
	}
}

func TestGet_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Get(ctx, uuid.New())
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByName(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createAttr(t, svc, "by-name")

	got, err := svc.GetByName(ctx, "by-name")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name != "by-name" {
		t.Errorf("name: got %q, want %q", got.Name, "by-name")
	}
}

func TestGetByName_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetByName(ctx, "nosuch")
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListAll(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createAttr(t, svc, "a1")
	createAttr(t, svc, "a2")

	all, err := svc.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("count: got %d, want 2", len(all))
	}
}

func TestListByCategory(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	cat := "material"
	svc.Create(ctx, globalattr.CreateParams{Name: "fabric", Category: &cat, IsActive: true})
	svc.Create(ctx, globalattr.CreateParams{Name: "standalone", IsActive: true})

	filtered, err := svc.ListByCategory(ctx, "material")
	if err != nil {
		t.Fatalf("ListByCategory: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("count: got %d, want 1", len(filtered))
	}
}

func TestUpdate(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "original")

	updated, err := svc.Update(ctx, id, globalattr.UpdateParams{
		Name:          "renamed",
		DisplayName:   "Renamed",
		AttributeType: "button_group",
		IsActive:      false,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("name: got %q, want %q", updated.Name, "renamed")
	}
	if updated.AttributeType != "button_group" {
		t.Errorf("type: got %q, want %q", updated.AttributeType, "button_group")
	}
	if updated.IsActive {
		t.Error("expected is_active=false")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Update(ctx, uuid.New(), globalattr.UpdateParams{Name: "x"})
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdate_EmptyName(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "has-name")

	_, err := svc.Update(ctx, id, globalattr.UpdateParams{Name: ""})
	if err != globalattr.ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "to-delete")

	err := svc.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = svc.Get(ctx, id)
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.Delete(ctx, uuid.New())
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete_InUse(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "in-use")
	prod := testDB.FixtureProduct(t, "linked-product", "linked-product")

	// Link the attribute to a product.
	svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID:         prod.ID,
		GlobalAttributeID: attrID,
		RoleName:          "color",
	})

	err := svc.Delete(ctx, attrID)
	if err != globalattr.ErrInUse {
		t.Errorf("expected ErrInUse, got %v", err)
	}
}

func TestCountUsage(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "unused")
	count, err := svc.CountUsage(ctx, id)
	if err != nil {
		t.Fatalf("CountUsage: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

// --------------------------------------------------------------------------
// Options
// --------------------------------------------------------------------------

func TestCreateOption(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	hex := "#FF0000"

	opt, err := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID,
		Value:             "red",
		DisplayValue:      "Red",
		ColorHex:          &hex,
		Metadata:          map[string]string{"pantone": "485C"},
		Position:          1,
		IsActive:          true,
	})
	if err != nil {
		t.Fatalf("CreateOption: %v", err)
	}
	if opt.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if opt.Value != "red" {
		t.Errorf("value: got %q, want %q", opt.Value, "red")
	}
}

func TestCreateOption_EmptyValue(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")

	_, err := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID,
		Value:             "",
	})
	if err != globalattr.ErrValueRequired {
		t.Errorf("expected ErrValueRequired, got %v", err)
	}
}

func TestGetOption(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	opt, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "small", DisplayValue: "Small", IsActive: true,
	})

	got, err := svc.GetOption(ctx, opt.ID)
	if err != nil {
		t.Fatalf("GetOption: %v", err)
	}
	if got.Value != "small" {
		t.Errorf("value: got %q, want %q", got.Value, "small")
	}
}

func TestGetOption_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetOption(ctx, uuid.New())
	if err != globalattr.ErrOptionNotFound {
		t.Errorf("expected ErrOptionNotFound, got %v", err)
	}
}

func TestListOptions(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "s", DisplayValue: "S", IsActive: true, Position: 1,
	})
	svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "m", DisplayValue: "M", IsActive: true, Position: 2,
	})

	opts, err := svc.ListOptions(ctx, attrID)
	if err != nil {
		t.Fatalf("ListOptions: %v", err)
	}
	if len(opts) != 2 {
		t.Errorf("count: got %d, want 2", len(opts))
	}
}

func TestUpdateOption(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})

	updated, err := svc.UpdateOption(ctx, opt.ID, globalattr.UpdateOptionParams{
		Value:        "crimson",
		DisplayValue: "Crimson Red",
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("UpdateOption: %v", err)
	}
	if updated.Value != "crimson" {
		t.Errorf("value: got %q, want %q", updated.Value, "crimson")
	}
}

func TestUpdateOption_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateOption(ctx, uuid.New(), globalattr.UpdateOptionParams{Value: "x"})
	if err != globalattr.ErrOptionNotFound {
		t.Errorf("expected ErrOptionNotFound, got %v", err)
	}
}

func TestDeleteOption(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})

	err := svc.DeleteOption(ctx, opt.ID)
	if err != nil {
		t.Fatalf("DeleteOption: %v", err)
	}

	_, err = svc.GetOption(ctx, opt.ID)
	if err != globalattr.ErrOptionNotFound {
		t.Errorf("expected ErrOptionNotFound after delete, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Metadata Fields
// --------------------------------------------------------------------------

func TestCreateField(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")

	field, err := svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID,
		FieldName:         "pantone_code",
		DisplayName:       "Pantone Code",
		FieldType:         "text",
		IsRequired:        false,
		Position:          1,
	})
	if err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	if field.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if field.FieldName != "pantone_code" {
		t.Errorf("field_name: got %q, want %q", field.FieldName, "pantone_code")
	}
}

func TestCreateField_EmptyName(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")

	_, err := svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID,
		FieldName:         "",
	})
	if err != globalattr.ErrFieldNameRequired {
		t.Errorf("expected ErrFieldNameRequired, got %v", err)
	}
}

func TestListFields(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID, FieldName: "f1", DisplayName: "F1", Position: 1,
	})
	svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID, FieldName: "f2", DisplayName: "F2", Position: 2,
	})

	fields, err := svc.ListFields(ctx, attrID)
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	if len(fields) != 2 {
		t.Errorf("count: got %d, want 2", len(fields))
	}
}

func TestDeleteField(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	field, _ := svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID, FieldName: "temp", DisplayName: "Temp", Position: 1,
	})

	err := svc.DeleteField(ctx, field.ID)
	if err != nil {
		t.Fatalf("DeleteField: %v", err)
	}
}

func TestDeleteField_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.DeleteField(ctx, uuid.New())
	if err != globalattr.ErrFieldNotFound {
		t.Errorf("expected ErrFieldNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Product Links
// --------------------------------------------------------------------------

func TestCreateLink(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "linked-prod", "linked-prod")

	link, err := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID:         prod.ID,
		GlobalAttributeID: attrID,
		RoleName:          "primary_color",
		RoleDisplayName:   "Primary Color",
		Position:          1,
		AffectsPricing:    true,
	})
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if link.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if link.RoleName != "primary_color" {
		t.Errorf("role: got %q, want %q", link.RoleName, "primary_color")
	}
}

func TestCreateLink_EmptyRole(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		RoleName: "",
	})
	if err != globalattr.ErrRoleNameRequired {
		t.Errorf("expected ErrRoleNameRequired, got %v", err)
	}
}

func TestGetLink(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	prod := testDB.FixtureProduct(t, "get-link-prod", "get-link-prod")

	created, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "size",
	})

	got, err := svc.GetLink(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetLink: %v", err)
	}
	if got.RoleName != "size" {
		t.Errorf("role: got %q, want %q", got.RoleName, "size")
	}
}

func TestGetLink_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetLink(ctx, uuid.New())
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestListLinks(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attr1 := createAttr(t, svc, "color")
	attr2 := createAttr(t, svc, "size")
	prod := testDB.FixtureProduct(t, "multi-link", "multi-link")

	svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attr1, RoleName: "color",
	})
	svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attr2, RoleName: "size",
	})

	links, err := svc.ListLinks(ctx, prod.ID)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("count: got %d, want 2", len(links))
	}
}

func TestUpdateLink(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "upd-link", "upd-link")

	created, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	updated, err := svc.UpdateLink(ctx, created.ID, globalattr.UpdateLinkParams{
		RoleName:        "main_color",
		RoleDisplayName: "Main Color",
		AffectsPricing:  true,
	})
	if err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}
	if updated.RoleName != "main_color" {
		t.Errorf("role: got %q, want %q", updated.RoleName, "main_color")
	}
}

func TestUpdateLink_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.UpdateLink(ctx, uuid.New(), globalattr.UpdateLinkParams{RoleName: "x"})
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestDeleteLink(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "del-link", "del-link")

	created, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	err := svc.DeleteLink(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}

	_, err = svc.GetLink(ctx, created.ID)
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound after delete, got %v", err)
	}
}

func TestDeleteLink_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	err := svc.DeleteLink(ctx, uuid.New())
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Selections
// --------------------------------------------------------------------------

func TestSetSelections(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt1, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})
	opt2, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "blue", DisplayValue: "Blue", IsActive: true,
	})

	prod := testDB.FixtureProduct(t, "sel-prod", "sel-prod")
	link, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	pos1, pos2 := int32(0), int32(1)
	err := svc.SetSelections(ctx, link.ID, []globalattr.SelectionInput{
		{GlobalOptionID: opt1.ID, PriceModifier: pgtype.Numeric{}, PositionOverride: &pos1},
		{GlobalOptionID: opt2.ID, PriceModifier: pgtype.Numeric{}, PositionOverride: &pos2},
	})
	if err != nil {
		t.Fatalf("SetSelections: %v", err)
	}

	sels, err := svc.ListSelections(ctx, link.ID)
	if err != nil {
		t.Fatalf("ListSelections: %v", err)
	}
	if len(sels) != 2 {
		t.Errorf("selections: got %d, want 2", len(sels))
	}
}

func TestSetSelections_Replaces(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	opt1, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "s", DisplayValue: "S", IsActive: true,
	})
	opt2, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "m", DisplayValue: "M", IsActive: true,
	})

	prod := testDB.FixtureProduct(t, "replace-sel", "replace-sel")
	link, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "size",
	})

	// Set initial selections.
	svc.SetSelections(ctx, link.ID, []globalattr.SelectionInput{
		{GlobalOptionID: opt1.ID},
		{GlobalOptionID: opt2.ID},
	})

	// Replace with just one.
	svc.SetSelections(ctx, link.ID, []globalattr.SelectionInput{
		{GlobalOptionID: opt2.ID},
	})

	sels, _ := svc.ListSelections(ctx, link.ID)
	if len(sels) != 1 {
		t.Errorf("selections: got %d, want 1 (replaced)", len(sels))
	}
}

func TestDeleteAllSelections(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})

	prod := testDB.FixtureProduct(t, "del-all-sel", "del-all-sel")
	link, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})
	svc.SetSelections(ctx, link.ID, []globalattr.SelectionInput{{GlobalOptionID: opt.ID}})

	err := svc.DeleteAllSelections(ctx, link.ID)
	if err != nil {
		t.Fatalf("DeleteAllSelections: %v", err)
	}

	sels, _ := svc.ListSelections(ctx, link.ID)
	if len(sels) != 0 {
		t.Errorf("selections: got %d, want 0", len(sels))
	}
}

// --------------------------------------------------------------------------
// Handler aliases
// --------------------------------------------------------------------------

func TestListOptionsWithMetadata(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID,
		Value:             "red",
		DisplayValue:      "Red",
		Metadata:          json.RawMessage(`{"pantone":"485C"}`),
		IsActive:          true,
	})

	opts, err := svc.ListOptionsWithMetadata(ctx, attrID)
	if err != nil {
		t.Fatalf("ListOptionsWithMetadata: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("count: got %d, want 1", len(opts))
	}
	if opts[0].Metadata["pantone"] != "485C" {
		t.Errorf("metadata[pantone]: got %q, want %q", opts[0].Metadata["pantone"], "485C")
	}
}

func TestLinkToProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "link-alias", "link-alias")

	link, err := svc.LinkToProduct(ctx, globalattr.LinkToProductParams{
		ProductID:         prod.ID,
		GlobalAttributeID: attrID,
		Role:              "color",
		AffectsPricing:    true,
	})
	if err != nil {
		t.Fatalf("LinkToProduct: %v", err)
	}
	if link.Role != "color" {
		t.Errorf("role: got %q, want %q", link.Role, "color")
	}
	if !link.AffectsPricing {
		t.Error("expected AffectsPricing=true")
	}
}

func TestCountOptions(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "s", DisplayValue: "S", IsActive: true,
	})
	svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "m", DisplayValue: "M", IsActive: true,
	})

	count, err := svc.CountOptions(ctx, attrID)
	if err != nil {
		t.Fatalf("CountOptions: %v", err)
	}
	if count != 2 {
		t.Errorf("count: got %d, want 2", count)
	}
}

// --------------------------------------------------------------------------
// Handler alias methods
// --------------------------------------------------------------------------

func TestListAttributes_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createAttr(t, svc, "a1")
	createAttr(t, svc, "a2")

	all, err := svc.ListAttributes(ctx)
	if err != nil {
		t.Fatalf("ListAttributes: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("count: got %d, want 2", len(all))
	}
}

func TestGetAttribute_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "alias-get")
	got, err := svc.GetAttribute(ctx, id)
	if err != nil {
		t.Fatalf("GetAttribute: %v", err)
	}
	if got.Name != "alias-get" {
		t.Errorf("name: got %q, want %q", got.Name, "alias-get")
	}
}

func TestDeleteAttribute_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "alias-del")
	err := svc.DeleteAttribute(ctx, id)
	if err != nil {
		t.Fatalf("DeleteAttribute: %v", err)
	}
	_, err = svc.Get(ctx, id)
	if err != globalattr.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCountUsages_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	id := createAttr(t, svc, "alias-count")
	count, err := svc.CountUsages(ctx, id)
	if err != nil {
		t.Fatalf("CountUsages: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

func TestListMetadataFields_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID, FieldName: "f1", DisplayName: "F1", Position: 1,
	})

	fields, err := svc.ListMetadataFields(ctx, attrID)
	if err != nil {
		t.Fatalf("ListMetadataFields: %v", err)
	}
	if len(fields) != 1 {
		t.Errorf("count: got %d, want 1", len(fields))
	}
}

func TestDeleteMetadataField_Alias(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	field, _ := svc.CreateField(ctx, globalattr.CreateFieldParams{
		GlobalAttributeID: attrID, FieldName: "temp", DisplayName: "Temp", Position: 1,
	})

	err := svc.DeleteMetadataField(ctx, field.ID)
	if err != nil {
		t.Fatalf("DeleteMetadataField: %v", err)
	}
}

func TestListProductLinks(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "product-links", "product-links")

	svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
		AffectsPricing: true,
	})

	links, err := svc.ListProductLinks(ctx, prod.ID)
	if err != nil {
		t.Fatalf("ListProductLinks: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("count: got %d, want 1", len(links))
	}
	if links[0].Role != "color" {
		t.Errorf("role: got %q, want %q", links[0].Role, "color")
	}
	if !links[0].AffectsPricing {
		t.Error("expected AffectsPricing=true")
	}
}

func TestGetProductLink(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "size")
	prod := testDB.FixtureProduct(t, "get-plink", "get-plink")

	created, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "size",
	})

	got, err := svc.GetProductLink(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProductLink: %v", err)
	}
	if got.Role != "size" {
		t.Errorf("role: got %q, want %q", got.Role, "size")
	}
}

func TestGetProductLink_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetProductLink(ctx, uuid.New())
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestUnlinkFromProduct(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "unlink-prod", "unlink-prod")

	created, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	err := svc.UnlinkFromProduct(ctx, created.ID)
	if err != nil {
		t.Fatalf("UnlinkFromProduct: %v", err)
	}

	_, err = svc.GetLink(ctx, created.ID)
	if err != globalattr.ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound after unlink, got %v", err)
	}
}

func TestListProductsUsing(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	prod := testDB.FixtureProduct(t, "using-prod", "using-prod")

	svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	products, err := svc.ListProductsUsing(ctx, attrID)
	if err != nil {
		t.Fatalf("ListProductsUsing: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("count: got %d, want 1", len(products))
	}
}

func TestGetSelectedOptions(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})

	prod := testDB.FixtureProduct(t, "sel-opts", "sel-opts")
	link, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	pos := int32(0)
	svc.SetSelections(ctx, link.ID, []globalattr.SelectionInput{
		{GlobalOptionID: opt.ID, PositionOverride: &pos},
	})

	ids, modifiers, err := svc.GetSelectedOptions(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetSelectedOptions: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("ids: got %d, want 1", len(ids))
	}
	if ids[0] != opt.ID {
		t.Errorf("id: got %s, want %s", ids[0], opt.ID)
	}
	// No price modifier set, so modifiers map should be empty.
	if len(modifiers) != 0 {
		t.Errorf("modifiers: got %d, want 0", len(modifiers))
	}
}

func TestUpdateProductOptionSelections(t *testing.T) {
	testDB.Truncate(t)
	testDB.SeedEssentials(t)
	svc := newService()
	ctx := context.Background()

	attrID := createAttr(t, svc, "color")
	opt1, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "red", DisplayValue: "Red", IsActive: true,
	})
	opt2, _ := svc.CreateOption(ctx, globalattr.CreateOptionParams{
		GlobalAttributeID: attrID, Value: "blue", DisplayValue: "Blue", IsActive: true,
	})

	prod := testDB.FixtureProduct(t, "upd-sel", "upd-sel")
	link, _ := svc.CreateLink(ctx, globalattr.CreateLinkParams{
		ProductID: prod.ID, GlobalAttributeID: attrID, RoleName: "color",
	})

	err := svc.UpdateProductOptionSelections(ctx, globalattr.UpdateSelectionsParams{
		LinkID:          link.ID,
		SelectedOptions: []uuid.UUID{opt1.ID, opt2.ID},
		PriceModifiers:  map[uuid.UUID]string{opt1.ID: "5.00"},
	})
	if err != nil {
		t.Fatalf("UpdateProductOptionSelections: %v", err)
	}

	sels, _ := svc.ListSelections(ctx, link.ID)
	if len(sels) != 2 {
		t.Errorf("selections: got %d, want 2", len(sels))
	}

	// Verify price modifier via GetSelectedOptions.
	ids, modifiers, _ := svc.GetSelectedOptions(ctx, link.ID)
	if len(ids) != 2 {
		t.Errorf("ids: got %d, want 2", len(ids))
	}
	if modifiers[opt1.ID] != "5.00" {
		t.Errorf("price modifier for opt1: got %q, want %q", modifiers[opt1.ID], "5.00")
	}
}

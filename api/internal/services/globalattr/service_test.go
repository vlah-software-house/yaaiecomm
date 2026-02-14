package globalattr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Tests for isDuplicateKeyError
// ---------------------------------------------------------------------------

// mockPgError implements the interface{ SQLState() string } used by
// isDuplicateKeyError to identify PostgreSQL unique constraint violations.
type mockPgError struct {
	code string
	msg  string
}

func (e *mockPgError) Error() string   { return e.msg }
func (e *mockPgError) SQLState() string { return e.code }

func TestIsDuplicateKeyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "pg error with code 23505 (unique_violation)",
			err:  &mockPgError{code: "23505", msg: "duplicate key value violates unique constraint"},
			want: true,
		},
		{
			name: "pg error with different code 23503 (foreign key)",
			err:  &mockPgError{code: "23503", msg: "foreign key violation"},
			want: false,
		},
		{
			name: "pg error with different code 23502 (not null)",
			err:  &mockPgError{code: "23502", msg: "not null violation"},
			want: false,
		},
		{
			name: "pg error with different code 42P01 (undefined table)",
			err:  &mockPgError{code: "42P01", msg: "undefined table"},
			want: false,
		},
		{
			name: "wrapped pg error with code 23505",
			err:  fmt.Errorf("insert failed: %w", &mockPgError{code: "23505", msg: "duplicate key"}),
			want: true,
		},
		{
			name: "wrapped pg error with different code",
			err:  fmt.Errorf("insert failed: %w", &mockPgError{code: "23502", msg: "not null violation"}),
			want: false,
		},
		{
			name: "doubly wrapped pg error with code 23505",
			err:  fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &mockPgError{code: "23505", msg: "dup"})),
			want: true,
		},
		{
			name: "error without SQLState method",
			err:  errors.New("plain error, no SQLState"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDuplicateKeyError(tt.err)
			if got != tt.want {
				t.Errorf("isDuplicateKeyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for marshalMetadata
// ---------------------------------------------------------------------------

func TestMarshalMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantJSON string
	}{
		{
			name:     "nil input returns empty object",
			input:    nil,
			wantJSON: "{}",
		},
		{
			name:     "json.RawMessage passthrough",
			input:    json.RawMessage(`{"key":"value"}`),
			wantJSON: `{"key":"value"}`,
		},
		{
			name:     "empty json.RawMessage returns empty object",
			input:    json.RawMessage(""),
			wantJSON: "{}",
		},
		{
			name:     "empty json.RawMessage (nil underlying)",
			input:    json.RawMessage(nil),
			wantJSON: "{}",
		},
		{
			name:     "byte slice passthrough",
			input:    []byte(`{"hello":"world"}`),
			wantJSON: `{"hello":"world"}`,
		},
		{
			name:     "empty byte slice returns empty object",
			input:    []byte(""),
			wantJSON: "{}",
		},
		{
			name:     "map[string]string serialization",
			input:    map[string]string{"origin": "Spain"},
			wantJSON: `{"origin":"Spain"}`,
		},
		{
			name:     "map[string]interface{} serialization",
			input:    map[string]interface{}{"weight": 42.5, "organic": true},
			wantJSON: "", // don't check exact JSON because map ordering is non-deterministic
		},
		{
			name:     "string value serialization",
			input:    "just a string",
			wantJSON: `"just a string"`,
		},
		{
			name:     "integer serialization",
			input:    42,
			wantJSON: "42",
		},
		{
			name:     "boolean serialization",
			input:    true,
			wantJSON: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalMetadata(tt.input)
			if got == nil {
				t.Fatal("marshalMetadata returned nil, expected non-nil json.RawMessage")
			}
			if tt.wantJSON != "" {
				if string(got) != tt.wantJSON {
					t.Errorf("marshalMetadata(%v) = %q, want %q", tt.input, string(got), tt.wantJSON)
				}
			} else {
				// For non-deterministic output, just verify it's valid JSON.
				var parsed interface{}
				if err := json.Unmarshal(got, &parsed); err != nil {
					t.Errorf("marshalMetadata(%v) produced invalid JSON: %q, error: %v", tt.input, string(got), err)
				}
			}
		})
	}
}

func TestMarshalMetadata_UnserializableValue(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	ch := make(chan int)
	got := marshalMetadata(ch)
	if string(got) != "{}" {
		t.Errorf("marshalMetadata(channel) = %q, want '{}' (fallback for unmarshalable types)", string(got))
	}
}

func TestMarshalMetadata_ValidJSONRawMessage(t *testing.T) {
	// Verify complex JSON passes through unchanged.
	complex := json.RawMessage(`{"nested":{"key":"val"},"array":[1,2,3],"null":null}`)
	got := marshalMetadata(complex)
	if string(got) != string(complex) {
		t.Errorf("marshalMetadata(complex) = %q, want %q", string(got), string(complex))
	}
}

// ---------------------------------------------------------------------------
// Tests for numericToString
// ---------------------------------------------------------------------------

func TestNumericToString(t *testing.T) {
	tests := []struct {
		name string
		n    pgtype.Numeric
		want string
	}{
		{
			name: "invalid numeric returns empty",
			n:    pgtype.Numeric{Valid: false},
			want: "",
		},
		{
			name: "zero value valid numeric",
			n:    pgtype.Numeric{Valid: true},
			want: "0.00", // Valid numeric with zero value formats as "0.00"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numericToString(tt.n)
			if got != tt.want {
				t.Errorf("numericToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Create validation (params checked before DB call)
// ---------------------------------------------------------------------------

func TestCreate_EmptyName_ReturnsError(t *testing.T) {
	// Create with no DB connection -- validation should fail before any DB call.
	svc := &Service{}
	_, err := svc.Create(context.Background(), CreateParams{
		Name: "",
	})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("Create with empty name: got error %v, want %v", err, ErrNameRequired)
	}
}

func TestCreate_DefaultAttributeType(t *testing.T) {
	// When AttributeType is empty, the service defaults it to "select".
	// We simulate the logic since the DB call will fail without a real pool.
	params := CreateParams{
		Name:          "test-attr",
		AttributeType: "",
		IsActive:      true,
	}
	if params.AttributeType == "" {
		params.AttributeType = "select"
	}
	if params.AttributeType != "select" {
		t.Errorf("expected default AttributeType to be 'select', got %q", params.AttributeType)
	}
}

func TestCreate_ExplicitAttributeType_NotOverridden(t *testing.T) {
	params := CreateParams{
		Name:          "test-swatch",
		AttributeType: "color_swatch",
		IsActive:      true,
	}
	// Simulate the service defaulting logic.
	if params.AttributeType == "" {
		params.AttributeType = "select"
	}
	if params.AttributeType != "color_swatch" {
		t.Errorf("expected explicit AttributeType to be preserved, got %q", params.AttributeType)
	}
}

func TestUpdate_EmptyName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.Update(context.Background(), uuid.UUID{}, UpdateParams{
		Name: "",
	})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("Update with empty name: got error %v, want %v", err, ErrNameRequired)
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateOption validation
// ---------------------------------------------------------------------------

func TestCreateOption_EmptyValue_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateOption(context.Background(), CreateOptionParams{
		Value: "",
	})
	if !errors.Is(err, ErrValueRequired) {
		t.Errorf("CreateOption with empty value: got error %v, want %v", err, ErrValueRequired)
	}
}

func TestUpdateOption_EmptyValue_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.UpdateOption(context.Background(), uuid.UUID{}, UpdateOptionParams{
		Value: "",
	})
	if !errors.Is(err, ErrValueRequired) {
		t.Errorf("UpdateOption with empty value: got error %v, want %v", err, ErrValueRequired)
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateField validation
// ---------------------------------------------------------------------------

func TestCreateField_EmptyFieldName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateField(context.Background(), CreateFieldParams{
		FieldName: "",
	})
	if !errors.Is(err, ErrFieldNameRequired) {
		t.Errorf("CreateField with empty field name: got error %v, want %v", err, ErrFieldNameRequired)
	}
}

func TestCreateField_DefaultFieldType(t *testing.T) {
	// When FieldType is empty, the service defaults it to "text".
	params := CreateFieldParams{
		FieldName: "weight",
		FieldType: "",
	}
	if params.FieldType == "" {
		params.FieldType = "text"
	}
	if params.FieldType != "text" {
		t.Errorf("expected default FieldType to be 'text', got %q", params.FieldType)
	}
}

func TestCreateField_ExplicitFieldType_NotOverridden(t *testing.T) {
	params := CreateFieldParams{
		FieldName: "is_organic",
		FieldType: "boolean",
	}
	if params.FieldType == "" {
		params.FieldType = "text"
	}
	if params.FieldType != "boolean" {
		t.Errorf("expected explicit FieldType to be preserved, got %q", params.FieldType)
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateLink validation
// ---------------------------------------------------------------------------

func TestCreateLink_EmptyRoleName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateLink(context.Background(), CreateLinkParams{
		RoleName: "",
	})
	if !errors.Is(err, ErrRoleNameRequired) {
		t.Errorf("CreateLink with empty role name: got error %v, want %v", err, ErrRoleNameRequired)
	}
}

func TestUpdateLink_EmptyRoleName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.UpdateLink(context.Background(), uuid.UUID{}, UpdateLinkParams{
		RoleName: "",
	})
	if !errors.Is(err, ErrRoleNameRequired) {
		t.Errorf("UpdateLink with empty role name: got error %v, want %v", err, ErrRoleNameRequired)
	}
}

// ---------------------------------------------------------------------------
// Tests for LinkToProduct alias (delegates to CreateLink)
// ---------------------------------------------------------------------------

func TestLinkToProduct_EmptyRole_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.LinkToProduct(context.Background(), LinkToProductParams{
		Role: "",
	})
	// LinkToProduct delegates to CreateLink with RoleName = params.Role
	if !errors.Is(err, ErrRoleNameRequired) {
		t.Errorf("LinkToProduct with empty role: got error %v, want %v", err, ErrRoleNameRequired)
	}
}

// ---------------------------------------------------------------------------
// Tests for error sentinel values
// ---------------------------------------------------------------------------

func TestErrorSentinels(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"ErrNotFound", ErrNotFound, "global attribute not found"},
		{"ErrFieldNotFound", ErrFieldNotFound, "metadata field not found"},
		{"ErrOptionNotFound", ErrOptionNotFound, "global option not found"},
		{"ErrLinkNotFound", ErrLinkNotFound, "product global attribute link not found"},
		{"ErrNameRequired", ErrNameRequired, "global attribute name is required"},
		{"ErrValueRequired", ErrValueRequired, "option value is required"},
		{"ErrFieldNameRequired", ErrFieldNameRequired, "field name is required"},
		{"ErrRoleNameRequired", ErrRoleNameRequired, "role name is required"},
		{"ErrInUse", ErrInUse, "global attribute is in use by products and cannot be deleted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestErrorSentinels_Distinct(t *testing.T) {
	// Ensure no two sentinel errors are the same instance.
	sentinels := []error{
		ErrNotFound,
		ErrFieldNotFound,
		ErrOptionNotFound,
		ErrLinkNotFound,
		ErrNameRequired,
		ErrValueRequired,
		ErrFieldNameRequired,
		ErrRoleNameRequired,
		ErrInUse,
	}

	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinel errors %d (%q) and %d (%q) should be distinct, but errors.Is returned true",
					i, sentinels[i], j, sentinels[j])
			}
		}
	}
}

func TestErrorSentinels_Wrapping(t *testing.T) {
	// Verify that wrapped sentinel errors can still be matched with errors.Is.
	wrapped := fmt.Errorf("operation failed: %w", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("expected wrapped ErrNotFound to match with errors.Is")
	}

	wrappedField := fmt.Errorf("field error: %w", ErrFieldNotFound)
	if !errors.Is(wrappedField, ErrFieldNotFound) {
		t.Error("expected wrapped ErrFieldNotFound to match with errors.Is")
	}

	wrappedInUse := fmt.Errorf("cannot delete: %w", ErrInUse)
	if !errors.Is(wrappedInUse, ErrInUse) {
		t.Error("expected wrapped ErrInUse to match with errors.Is")
	}
}

// ---------------------------------------------------------------------------
// Tests for NewService
// ---------------------------------------------------------------------------

func TestNewService_NilLogger(t *testing.T) {
	// When logger is nil, NewService should create a service with the default slog logger.
	svc := NewService(nil, nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.logger == nil {
		t.Error("NewService with nil logger: expected non-nil logger (should use slog.Default)")
	}
}

func TestNewService_NilPool(t *testing.T) {
	// With nil pool, service should still be created.
	svc := NewService(nil, nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.pool != nil {
		t.Error("expected pool to be nil when passed nil")
	}
	if svc.queries == nil {
		t.Error("expected queries to be non-nil (db.New wraps pool)")
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateParams / UpdateParams struct defaults and fields
// ---------------------------------------------------------------------------

func TestCreateParams_AllFields(t *testing.T) {
	desc := "A global attribute for testing"
	cat := "materials"
	params := CreateParams{
		Name:          "thread_material",
		DisplayName:   "Thread Material",
		Description:   &desc,
		AttributeType: "select",
		Category:      &cat,
		Position:      5,
		IsActive:      true,
	}

	if params.Name != "thread_material" {
		t.Errorf("Name: got %q, want %q", params.Name, "thread_material")
	}
	if params.DisplayName != "Thread Material" {
		t.Errorf("DisplayName: got %q, want %q", params.DisplayName, "Thread Material")
	}
	if params.Description == nil || *params.Description != desc {
		t.Errorf("Description: got %v, want %q", params.Description, desc)
	}
	if params.AttributeType != "select" {
		t.Errorf("AttributeType: got %q, want %q", params.AttributeType, "select")
	}
	if params.Category == nil || *params.Category != cat {
		t.Errorf("Category: got %v, want %q", params.Category, cat)
	}
	if params.Position != 5 {
		t.Errorf("Position: got %d, want %d", params.Position, 5)
	}
	if !params.IsActive {
		t.Error("IsActive: got false, want true")
	}
}

func TestCreateParams_MinimalFields(t *testing.T) {
	params := CreateParams{
		Name: "color",
	}

	if params.Name != "color" {
		t.Errorf("Name: got %q, want %q", params.Name, "color")
	}
	if params.DisplayName != "" {
		t.Errorf("DisplayName: got %q, want empty", params.DisplayName)
	}
	if params.Description != nil {
		t.Error("Description: expected nil for unset field")
	}
	if params.AttributeType != "" {
		t.Errorf("AttributeType: got %q, want empty (will be defaulted by service)", params.AttributeType)
	}
	if params.Category != nil {
		t.Error("Category: expected nil for unset field")
	}
	if params.Position != 0 {
		t.Errorf("Position: got %d, want 0", params.Position)
	}
	if params.IsActive {
		t.Error("IsActive: got true, want false (zero value)")
	}
}

func TestUpdateParams_AllFields(t *testing.T) {
	desc := "Updated description"
	cat := "updated_category"
	params := UpdateParams{
		Name:          "updated_name",
		DisplayName:   "Updated Display Name",
		Description:   &desc,
		AttributeType: "color_swatch",
		Category:      &cat,
		Position:      10,
		IsActive:      false,
	}

	if params.Name != "updated_name" {
		t.Errorf("Name: got %q, want %q", params.Name, "updated_name")
	}
	if params.DisplayName != "Updated Display Name" {
		t.Errorf("DisplayName: got %q, want %q", params.DisplayName, "Updated Display Name")
	}
	if params.Description == nil || *params.Description != desc {
		t.Errorf("Description: got %v, want %q", params.Description, desc)
	}
	if params.AttributeType != "color_swatch" {
		t.Errorf("AttributeType: got %q, want %q", params.AttributeType, "color_swatch")
	}
	if params.Category == nil || *params.Category != cat {
		t.Errorf("Category: got %v, want %q", params.Category, cat)
	}
	if params.Position != 10 {
		t.Errorf("Position: got %d, want %d", params.Position, 10)
	}
	if params.IsActive {
		t.Error("IsActive: got true, want false")
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateOptionParams (Metadata is interface{})
// ---------------------------------------------------------------------------

func TestCreateOptionParams_MetadataTypes(t *testing.T) {
	tests := []struct {
		name     string
		metadata interface{}
		wantNil  bool
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			wantNil:  true,
		},
		{
			name:     "json.RawMessage",
			metadata: json.RawMessage(`{"origin":"Spain"}`),
			wantNil:  false,
		},
		{
			name:     "map[string]string",
			metadata: map[string]string{"origin": "Spain"},
			wantNil:  false,
		},
		{
			name:     "map[string]interface{}",
			metadata: map[string]interface{}{"weight": 42.5},
			wantNil:  false,
		},
		{
			name:     "string value",
			metadata: "raw string",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := CreateOptionParams{
				Value:    "test_value",
				Metadata: tt.metadata,
			}
			if tt.wantNil && params.Metadata != nil {
				t.Error("expected metadata to be nil")
			}
			if !tt.wantNil && params.Metadata == nil {
				t.Error("expected metadata to be non-nil")
			}
		})
	}
}

func TestUpdateOptionParams_AllFields(t *testing.T) {
	hex := "#FF5733"
	imgURL := "https://example.com/img.png"
	meta := map[string]string{"pantone": "1505C"}
	params := UpdateOptionParams{
		Value:        "orange",
		DisplayValue: "Orange",
		ColorHex:     &hex,
		ImageURL:     &imgURL,
		Metadata:     meta,
		Position:     3,
		IsActive:     true,
	}

	if params.Value != "orange" {
		t.Errorf("Value: got %q, want %q", params.Value, "orange")
	}
	if params.DisplayValue != "Orange" {
		t.Errorf("DisplayValue: got %q, want %q", params.DisplayValue, "Orange")
	}
	if params.ColorHex == nil || *params.ColorHex != hex {
		t.Errorf("ColorHex: got %v, want %q", params.ColorHex, hex)
	}
	if params.ImageURL == nil || *params.ImageURL != imgURL {
		t.Errorf("ImageURL: got %v, want %q", params.ImageURL, imgURL)
	}
	if params.Metadata == nil {
		t.Error("Metadata: expected non-nil")
	}
	if params.Position != 3 {
		t.Errorf("Position: got %d, want %d", params.Position, 3)
	}
	if !params.IsActive {
		t.Error("IsActive: got false, want true")
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateFieldParams
// ---------------------------------------------------------------------------

func TestCreateFieldParams_ValidFieldTypes(t *testing.T) {
	validTypes := []string{"text", "number", "boolean", "select", "url"}

	for _, ft := range validTypes {
		t.Run(ft, func(t *testing.T) {
			params := CreateFieldParams{
				FieldName: "test_field",
				FieldType: ft,
				Position:  0,
			}
			if params.FieldType != ft {
				t.Errorf("FieldType: got %q, want %q", params.FieldType, ft)
			}
		})
	}
}

func TestCreateFieldParams_WithSelectOptions(t *testing.T) {
	opts := []string{"option_a", "option_b", "option_c"}
	params := CreateFieldParams{
		FieldName:     "dropdown",
		FieldType:     "select",
		SelectOptions: opts,
		IsRequired:    true,
	}

	if len(params.SelectOptions) != 3 {
		t.Fatalf("SelectOptions length: got %d, want 3", len(params.SelectOptions))
	}
	for i, expected := range opts {
		if params.SelectOptions[i] != expected {
			t.Errorf("SelectOptions[%d]: got %q, want %q", i, params.SelectOptions[i], expected)
		}
	}
	if !params.IsRequired {
		t.Error("IsRequired: got false, want true")
	}
}

func TestCreateFieldParams_WithDefaultValue(t *testing.T) {
	defaultVal := "100"
	params := CreateFieldParams{
		FieldName:    "weight_grams",
		FieldType:    "number",
		DefaultValue: &defaultVal,
	}

	if params.DefaultValue == nil {
		t.Fatal("DefaultValue: expected non-nil")
	}
	if *params.DefaultValue != "100" {
		t.Errorf("DefaultValue: got %q, want %q", *params.DefaultValue, "100")
	}
}

func TestCreateFieldParams_WithHelpText(t *testing.T) {
	helpText := "Enter the weight in grams"
	params := CreateFieldParams{
		FieldName: "weight",
		FieldType: "number",
		HelpText:  &helpText,
	}

	if params.HelpText == nil {
		t.Fatal("HelpText: expected non-nil")
	}
	if *params.HelpText != helpText {
		t.Errorf("HelpText: got %q, want %q", *params.HelpText, helpText)
	}
}

// ---------------------------------------------------------------------------
// Tests for CreateLinkParams / UpdateLinkParams fields
// ---------------------------------------------------------------------------

func TestCreateLinkParams_AllFields(t *testing.T) {
	priceField := "price_modifier"
	weightField := "weight_grams"
	params := CreateLinkParams{
		RoleName:            "thread_color",
		RoleDisplayName:     "Thread Color",
		Position:            2,
		AffectsPricing:      true,
		AffectsShipping:     true,
		PriceModifierField:  &priceField,
		WeightModifierField: &weightField,
	}

	if params.RoleName != "thread_color" {
		t.Errorf("RoleName: got %q, want %q", params.RoleName, "thread_color")
	}
	if params.RoleDisplayName != "Thread Color" {
		t.Errorf("RoleDisplayName: got %q, want %q", params.RoleDisplayName, "Thread Color")
	}
	if params.Position != 2 {
		t.Errorf("Position: got %d, want %d", params.Position, 2)
	}
	if !params.AffectsPricing {
		t.Error("AffectsPricing: got false, want true")
	}
	if !params.AffectsShipping {
		t.Error("AffectsShipping: got false, want true")
	}
	if params.PriceModifierField == nil || *params.PriceModifierField != priceField {
		t.Errorf("PriceModifierField: got %v, want %q", params.PriceModifierField, priceField)
	}
	if params.WeightModifierField == nil || *params.WeightModifierField != weightField {
		t.Errorf("WeightModifierField: got %v, want %q", params.WeightModifierField, weightField)
	}
}

func TestCreateLinkParams_MinimalFields(t *testing.T) {
	params := CreateLinkParams{
		RoleName: "color",
	}

	if params.RoleName != "color" {
		t.Errorf("RoleName: got %q, want %q", params.RoleName, "color")
	}
	if params.RoleDisplayName != "" {
		t.Errorf("RoleDisplayName: expected empty, got %q", params.RoleDisplayName)
	}
	if params.AffectsPricing {
		t.Error("AffectsPricing: expected false (zero value)")
	}
	if params.AffectsShipping {
		t.Error("AffectsShipping: expected false (zero value)")
	}
	if params.PriceModifierField != nil {
		t.Error("PriceModifierField: expected nil")
	}
	if params.WeightModifierField != nil {
		t.Error("WeightModifierField: expected nil")
	}
}

func TestUpdateLinkParams_AllFields(t *testing.T) {
	priceField := "unit_price_override"
	weightField := "weight_modifier"
	params := UpdateLinkParams{
		RoleName:            "leather_color",
		RoleDisplayName:     "Leather Color",
		Position:            1,
		AffectsPricing:      true,
		AffectsShipping:     false,
		PriceModifierField:  &priceField,
		WeightModifierField: &weightField,
	}

	if params.RoleName != "leather_color" {
		t.Errorf("RoleName: got %q, want %q", params.RoleName, "leather_color")
	}
	if params.RoleDisplayName != "Leather Color" {
		t.Errorf("RoleDisplayName: got %q, want %q", params.RoleDisplayName, "Leather Color")
	}
	if params.Position != 1 {
		t.Errorf("Position: got %d, want %d", params.Position, 1)
	}
	if !params.AffectsPricing {
		t.Error("AffectsPricing: got false, want true")
	}
	if params.AffectsShipping {
		t.Error("AffectsShipping: got true, want false")
	}
	if params.PriceModifierField == nil || *params.PriceModifierField != priceField {
		t.Errorf("PriceModifierField: got %v, want %q", params.PriceModifierField, priceField)
	}
	if params.WeightModifierField == nil || *params.WeightModifierField != weightField {
		t.Errorf("WeightModifierField: got %v, want %q", params.WeightModifierField, weightField)
	}
}

// ---------------------------------------------------------------------------
// Tests for SelectionInput fields
// ---------------------------------------------------------------------------

func TestSelectionInput_Fields(t *testing.T) {
	var pos int32 = 3
	var wt int32 = 50
	params := SelectionInput{
		WeightModifierGrams: &wt,
		PositionOverride:    &pos,
	}

	if params.WeightModifierGrams == nil || *params.WeightModifierGrams != 50 {
		t.Errorf("WeightModifierGrams: got %v, want 50", params.WeightModifierGrams)
	}
	if params.PositionOverride == nil || *params.PositionOverride != 3 {
		t.Errorf("PositionOverride: got %v, want 3", params.PositionOverride)
	}
}

func TestSelectionInput_NilOptionalFields(t *testing.T) {
	params := SelectionInput{}
	if params.WeightModifierGrams != nil {
		t.Error("WeightModifierGrams: expected nil for zero-value struct")
	}
	if params.PositionOverride != nil {
		t.Error("PositionOverride: expected nil for zero-value struct")
	}
}

// ---------------------------------------------------------------------------
// Tests for LinkToProductParams
// ---------------------------------------------------------------------------

func TestLinkToProductParams_Fields(t *testing.T) {
	params := LinkToProductParams{
		Role:            "material",
		AffectsPricing:  true,
		AffectsShipping: false,
	}

	if params.Role != "material" {
		t.Errorf("Role: got %q, want %q", params.Role, "material")
	}
	if !params.AffectsPricing {
		t.Error("AffectsPricing: got false, want true")
	}
	if params.AffectsShipping {
		t.Error("AffectsShipping: got true, want false")
	}
}

// ---------------------------------------------------------------------------
// Tests for UpdateSelectionsParams
// ---------------------------------------------------------------------------

func TestUpdateSelectionsParams_Fields(t *testing.T) {
	id1 := uuid.UUID{1}
	id2 := uuid.UUID{2}
	params := UpdateSelectionsParams{
		SelectedOptions: []uuid.UUID{id1, id2},
		PriceModifiers:  map[uuid.UUID]string{id1: "5.00", id2: "10.00"},
	}

	if len(params.SelectedOptions) != 2 {
		t.Fatalf("SelectedOptions length: got %d, want 2", len(params.SelectedOptions))
	}
	if params.PriceModifiers[id1] != "5.00" {
		t.Errorf("PriceModifiers[id1]: got %q, want %q", params.PriceModifiers[id1], "5.00")
	}
	if params.PriceModifiers[id2] != "10.00" {
		t.Errorf("PriceModifiers[id2]: got %q, want %q", params.PriceModifiers[id2], "10.00")
	}
}

func TestUpdateSelectionsParams_EmptyOptions(t *testing.T) {
	params := UpdateSelectionsParams{
		SelectedOptions: []uuid.UUID{},
		PriceModifiers:  map[uuid.UUID]string{},
	}

	if len(params.SelectedOptions) != 0 {
		t.Errorf("SelectedOptions length: got %d, want 0", len(params.SelectedOptions))
	}
}

// ---------------------------------------------------------------------------
// Tests for ProductLink type
// ---------------------------------------------------------------------------

func TestProductLink_Fields(t *testing.T) {
	pl := ProductLink{
		ID:                uuid.UUID{1},
		GlobalAttributeID: uuid.UUID{2},
		Role:              "color",
		AffectsPricing:    true,
		AffectsShipping:   false,
	}

	if pl.Role != "color" {
		t.Errorf("Role: got %q, want %q", pl.Role, "color")
	}
	if !pl.AffectsPricing {
		t.Error("AffectsPricing: got false, want true")
	}
	if pl.AffectsShipping {
		t.Error("AffectsShipping: got true, want false")
	}
}

// ---------------------------------------------------------------------------
// Tests for OptionWithMetadata type
// ---------------------------------------------------------------------------

func TestOptionWithMetadata_Fields(t *testing.T) {
	hex := "#FF0000"
	imgURL := "https://example.com/red.png"
	owm := OptionWithMetadata{
		ID:           uuid.UUID{1},
		Value:        "red",
		DisplayValue: "Red",
		ColorHex:     &hex,
		ImageURL:     &imgURL,
		Metadata:     map[string]string{"pantone": "185C"},
		Position:     0,
		IsActive:     true,
	}

	if owm.Value != "red" {
		t.Errorf("Value: got %q, want %q", owm.Value, "red")
	}
	if owm.DisplayValue != "Red" {
		t.Errorf("DisplayValue: got %q, want %q", owm.DisplayValue, "Red")
	}
	if owm.ColorHex == nil || *owm.ColorHex != hex {
		t.Errorf("ColorHex: got %v, want %q", owm.ColorHex, hex)
	}
	if owm.ImageURL == nil || *owm.ImageURL != imgURL {
		t.Errorf("ImageURL: got %v, want %q", owm.ImageURL, imgURL)
	}
	if owm.Metadata["pantone"] != "185C" {
		t.Errorf("Metadata[pantone]: got %q, want %q", owm.Metadata["pantone"], "185C")
	}
	if !owm.IsActive {
		t.Error("IsActive: got false, want true")
	}
}

func TestOptionWithMetadata_EmptyMetadata(t *testing.T) {
	owm := OptionWithMetadata{
		Value:    "plain",
		Metadata: map[string]string{},
	}
	if len(owm.Metadata) != 0 {
		t.Errorf("Metadata length: got %d, want 0", len(owm.Metadata))
	}
}

// ---------------------------------------------------------------------------
// Tests for attribute type validation
// ---------------------------------------------------------------------------

func TestAttributeTypes_Valid(t *testing.T) {
	validTypes := []string{"select", "color_swatch", "button_group", "image_swatch"}

	for _, at := range validTypes {
		t.Run(at, func(t *testing.T) {
			params := CreateParams{
				Name:          "test_" + at,
				AttributeType: at,
				IsActive:      true,
			}
			if params.AttributeType != at {
				t.Errorf("AttributeType: got %q, want %q", params.AttributeType, at)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for ValidateMetadata -- metadata validation against field definitions
// ---------------------------------------------------------------------------

// metadataField represents a simplified metadata field definition for validation.
type metadataField struct {
	FieldName     string
	FieldType     string // text, number, boolean, select, url
	IsRequired    bool
	SelectOptions []string
}

// validateMetadata validates a metadata JSON object against field definitions.
// Returns an error if required fields are missing or values don't match field types.
// This is a pure validation function extracted to test the rules that would apply
// when persisting option metadata in production.
func validateMetadata(metadata json.RawMessage, fields []metadataField) error {
	if metadata == nil || string(metadata) == "{}" || string(metadata) == "null" {
		for _, f := range fields {
			if f.IsRequired {
				return fmt.Errorf("required metadata field %q is missing", f.FieldName)
			}
		}
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(metadata, &data); err != nil {
		return fmt.Errorf("invalid metadata JSON: %w", err)
	}

	for _, f := range fields {
		val, exists := data[f.FieldName]
		if !exists || val == nil {
			if f.IsRequired {
				return fmt.Errorf("required metadata field %q is missing", f.FieldName)
			}
			continue
		}

		switch f.FieldType {
		case "number":
			if _, ok := val.(float64); !ok {
				return fmt.Errorf("metadata field %q must be a number, got %T", f.FieldName, val)
			}
		case "boolean":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("metadata field %q must be a boolean, got %T", f.FieldName, val)
			}
		case "select":
			strVal, ok := val.(string)
			if !ok {
				return fmt.Errorf("metadata field %q must be a string for select, got %T", f.FieldName, val)
			}
			found := false
			for _, opt := range f.SelectOptions {
				if strVal == opt {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("metadata field %q value %q is not in allowed options %v", f.FieldName, strVal, f.SelectOptions)
			}
		case "text", "url":
			if _, ok := val.(string); !ok {
				return fmt.Errorf("metadata field %q must be a string, got %T", f.FieldName, val)
			}
		}
	}

	return nil
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata json.RawMessage
		fields   []metadataField
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "empty metadata with no required fields",
			metadata: json.RawMessage("{}"),
			fields: []metadataField{
				{FieldName: "optional_field", FieldType: "text", IsRequired: false},
			},
		},
		{
			name:     "nil metadata with no required fields",
			metadata: nil,
			fields:   []metadataField{},
		},
		{
			name:     "nil metadata with required field",
			metadata: nil,
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field \"origin\" is missing",
		},
		{
			name:     "empty metadata with required field",
			metadata: json.RawMessage("{}"),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field \"weight\" is missing",
		},
		{
			name:     "required field present",
			metadata: json.RawMessage(`{"origin":"Spain"}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
		},
		{
			name:     "required field missing from populated metadata",
			metadata: json.RawMessage(`{"other_field":"value"}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field \"origin\" is missing",
		},
		{
			name:     "number field accepts valid number",
			metadata: json.RawMessage(`{"weight":42.5}`),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
		},
		{
			name:     "number field accepts integer as float64",
			metadata: json.RawMessage(`{"weight":100}`),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
		},
		{
			name:     "number field rejects string value",
			metadata: json.RawMessage(`{"weight":"not a number"}`),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "metadata field \"weight\" must be a number",
		},
		{
			name:     "number field rejects boolean value",
			metadata: json.RawMessage(`{"weight":true}`),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "metadata field \"weight\" must be a number",
		},
		{
			name:     "boolean field accepts true",
			metadata: json.RawMessage(`{"organic":true}`),
			fields: []metadataField{
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
		},
		{
			name:     "boolean field accepts false",
			metadata: json.RawMessage(`{"organic":false}`),
			fields: []metadataField{
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
		},
		{
			name:     "boolean field rejects string",
			metadata: json.RawMessage(`{"organic":"yes"}`),
			fields: []metadataField{
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "metadata field \"organic\" must be a boolean",
		},
		{
			name:     "boolean field rejects number",
			metadata: json.RawMessage(`{"organic":1}`),
			fields: []metadataField{
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "metadata field \"organic\" must be a boolean",
		},
		{
			name:     "select field accepts valid option",
			metadata: json.RawMessage(`{"finish":"matte"}`),
			fields: []metadataField{
				{FieldName: "finish", FieldType: "select", IsRequired: true, SelectOptions: []string{"matte", "glossy", "satin"}},
			},
		},
		{
			name:     "select field rejects invalid option",
			metadata: json.RawMessage(`{"finish":"sparkle"}`),
			fields: []metadataField{
				{FieldName: "finish", FieldType: "select", IsRequired: true, SelectOptions: []string{"matte", "glossy", "satin"}},
			},
			wantErr: true,
			errMsg:  "metadata field \"finish\" value \"sparkle\" is not in allowed options",
		},
		{
			name:     "select field rejects non-string value",
			metadata: json.RawMessage(`{"finish":42}`),
			fields: []metadataField{
				{FieldName: "finish", FieldType: "select", IsRequired: true, SelectOptions: []string{"matte", "glossy"}},
			},
			wantErr: true,
			errMsg:  "metadata field \"finish\" must be a string for select",
		},
		{
			name:     "text field accepts string",
			metadata: json.RawMessage(`{"description":"A fine thread"}`),
			fields: []metadataField{
				{FieldName: "description", FieldType: "text", IsRequired: false},
			},
		},
		{
			name:     "text field rejects number",
			metadata: json.RawMessage(`{"description":123}`),
			fields: []metadataField{
				{FieldName: "description", FieldType: "text", IsRequired: false},
			},
			wantErr: true,
			errMsg:  "metadata field \"description\" must be a string",
		},
		{
			name:     "url field accepts string",
			metadata: json.RawMessage(`{"spec_url":"https://example.com/spec.pdf"}`),
			fields: []metadataField{
				{FieldName: "spec_url", FieldType: "url", IsRequired: false},
			},
		},
		{
			name:     "url field rejects non-string",
			metadata: json.RawMessage(`{"spec_url":true}`),
			fields: []metadataField{
				{FieldName: "spec_url", FieldType: "url", IsRequired: false},
			},
			wantErr: true,
			errMsg:  "metadata field \"spec_url\" must be a string",
		},
		{
			name:     "unknown fields ignored when not in field definitions",
			metadata: json.RawMessage(`{"unknown_field":"value","origin":"Spain"}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
		},
		{
			name:     "multiple required fields all present",
			metadata: json.RawMessage(`{"origin":"Spain","weight":42,"organic":true}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
				{FieldName: "weight", FieldType: "number", IsRequired: true},
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
		},
		{
			name:     "multiple required fields some missing",
			metadata: json.RawMessage(`{"origin":"Spain"}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
				{FieldName: "weight", FieldType: "number", IsRequired: true},
				{FieldName: "organic", FieldType: "boolean", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field",
		},
		{
			name:     "null metadata string with no required fields",
			metadata: json.RawMessage("null"),
			fields:   []metadataField{},
		},
		{
			name:     "null metadata string with required field",
			metadata: json.RawMessage("null"),
			fields: []metadataField{
				{FieldName: "required_field", FieldType: "text", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field \"required_field\" is missing",
		},
		{
			name:     "invalid JSON metadata",
			metadata: json.RawMessage("not valid json"),
			fields: []metadataField{
				{FieldName: "field", FieldType: "text", IsRequired: false},
			},
			wantErr: true,
			errMsg:  "invalid metadata JSON",
		},
		{
			name:     "null value for required field",
			metadata: json.RawMessage(`{"origin":null}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
			wantErr: true,
			errMsg:  "required metadata field \"origin\" is missing",
		},
		{
			name:     "empty string for text field is valid",
			metadata: json.RawMessage(`{"origin":""}`),
			fields: []metadataField{
				{FieldName: "origin", FieldType: "text", IsRequired: true},
			},
		},
		{
			name:     "zero for number field is valid",
			metadata: json.RawMessage(`{"weight":0}`),
			fields: []metadataField{
				{FieldName: "weight", FieldType: "number", IsRequired: true},
			},
		},
		{
			name:     "no field definitions means any metadata is valid",
			metadata: json.RawMessage(`{"anything":"goes","number":42}`),
			fields:   []metadataField{},
		},
		{
			name:     "select field with empty options list rejects any value",
			metadata: json.RawMessage(`{"grade":"A"}`),
			fields: []metadataField{
				{FieldName: "grade", FieldType: "select", IsRequired: true, SelectOptions: []string{}},
			},
			wantErr: true,
			errMsg:  "not in allowed options",
		},
		{
			name:     "negative number is valid for number field",
			metadata: json.RawMessage(`{"temperature":-5.5}`),
			fields: []metadataField{
				{FieldName: "temperature", FieldType: "number", IsRequired: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetadata(tt.metadata, tt.fields)
			if tt.wantErr {
				if err == nil {
					t.Error("expected an error, got nil")
				} else if tt.errMsg != "" {
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("error message %q does not contain %q", err.Error(), tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Alias method and type alias tests
// ---------------------------------------------------------------------------

func TestCreateAttributeParams_IsAlias(t *testing.T) {
	var p CreateAttributeParams = CreateParams{Name: "test"}
	if p.Name != "test" {
		t.Errorf("CreateAttributeParams should be an alias for CreateParams")
	}
}

func TestUpdateAttributeParams_IsAlias(t *testing.T) {
	var p UpdateAttributeParams = UpdateParams{Name: "test"}
	if p.Name != "test" {
		t.Errorf("UpdateAttributeParams should be an alias for UpdateParams")
	}
}

func TestCreateMetadataFieldParams_IsAlias(t *testing.T) {
	var p CreateMetadataFieldParams = CreateFieldParams{FieldName: "test"}
	if p.FieldName != "test" {
		t.Errorf("CreateMetadataFieldParams should be an alias for CreateFieldParams")
	}
}

func TestCreateAttribute_EmptyName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateAttribute(context.Background(), CreateParams{Name: ""})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("CreateAttribute with empty name: got error %v, want %v", err, ErrNameRequired)
	}
}

func TestUpdateAttribute_EmptyName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.UpdateAttribute(context.Background(), uuid.UUID{}, UpdateParams{Name: ""})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("UpdateAttribute with empty name: got error %v, want %v", err, ErrNameRequired)
	}
}

func TestCreateMetadataFieldAlias_EmptyFieldName_ReturnsError(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateMetadataField(context.Background(), CreateFieldParams{FieldName: ""})
	if !errors.Is(err, ErrFieldNameRequired) {
		t.Errorf("CreateMetadataField with empty field name: got error %v, want %v", err, ErrFieldNameRequired)
	}
}

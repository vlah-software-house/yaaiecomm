package variant

import (
	"testing"

	"github.com/google/uuid"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// --------------------------------------------------------------------------
// Tests for cartesianProduct
// --------------------------------------------------------------------------

func TestCartesianProduct_Empty(t *testing.T) {
	result := cartesianProduct(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestCartesianProduct_SingleAttribute(t *testing.T) {
	attrID := uuid.New()
	opts := []attributeWithOptions{
		{
			attribute: db.ProductAttribute{ID: attrID, Name: "color"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "black"},
				{ID: uuid.New(), Value: "tan"},
				{ID: uuid.New(), Value: "brown"},
			},
		},
	}

	result := cartesianProduct(opts)
	if len(result) != 3 {
		t.Fatalf("expected 3 combinations, got %d", len(result))
	}

	for i, combo := range result {
		if len(combo) != 1 {
			t.Errorf("combo[%d]: expected 1 entry, got %d", i, len(combo))
		}
		if combo[0].attributeID != attrID {
			t.Errorf("combo[%d]: wrong attribute ID", i)
		}
	}

	// Verify all three options are represented.
	values := map[string]bool{}
	for _, combo := range result {
		values[combo[0].option.Value] = true
	}
	for _, v := range []string{"black", "tan", "brown"} {
		if !values[v] {
			t.Errorf("missing option value %q in result", v)
		}
	}
}

func TestCartesianProduct_TwoAttributes(t *testing.T) {
	colorID := uuid.New()
	sizeID := uuid.New()

	opts := []attributeWithOptions{
		{
			attribute: db.ProductAttribute{ID: colorID, Name: "color"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "black"},
				{ID: uuid.New(), Value: "tan"},
			},
		},
		{
			attribute: db.ProductAttribute{ID: sizeID, Name: "size"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "small"},
				{ID: uuid.New(), Value: "medium"},
				{ID: uuid.New(), Value: "large"},
			},
		},
	}

	result := cartesianProduct(opts)
	// 2 colors × 3 sizes = 6 combinations
	if len(result) != 6 {
		t.Fatalf("expected 6 combinations (2×3), got %d", len(result))
	}

	for i, combo := range result {
		if len(combo) != 2 {
			t.Errorf("combo[%d]: expected 2 entries, got %d", i, len(combo))
		}
	}
}

func TestCartesianProduct_ThreeAttributes(t *testing.T) {
	opts := []attributeWithOptions{
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "color"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "black"},
				{ID: uuid.New(), Value: "white"},
			},
		},
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "size"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "small"},
				{ID: uuid.New(), Value: "large"},
			},
		},
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "material"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "cotton"},
				{ID: uuid.New(), Value: "polyester"},
			},
		},
	}

	result := cartesianProduct(opts)
	// 2 × 2 × 2 = 8
	if len(result) != 8 {
		t.Fatalf("expected 8 combinations (2×2×2), got %d", len(result))
	}

	for i, combo := range result {
		if len(combo) != 3 {
			t.Errorf("combo[%d]: expected 3 entries, got %d", i, len(combo))
		}
	}
}

func TestCartesianProduct_SingleOptionPerAttribute(t *testing.T) {
	opts := []attributeWithOptions{
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "color"},
			options:   []db.ProductAttributeOption{{ID: uuid.New(), Value: "black"}},
		},
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "size"},
			options:   []db.ProductAttributeOption{{ID: uuid.New(), Value: "onesize"}},
		},
	}

	result := cartesianProduct(opts)
	// 1 × 1 = 1
	if len(result) != 1 {
		t.Fatalf("expected 1 combination, got %d", len(result))
	}
	if len(result[0]) != 2 {
		t.Errorf("expected 2 entries in single combo, got %d", len(result[0]))
	}
}

func TestCartesianProduct_AttributeWithNoOptions(t *testing.T) {
	opts := []attributeWithOptions{
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "color"},
			options: []db.ProductAttributeOption{
				{ID: uuid.New(), Value: "black"},
				{ID: uuid.New(), Value: "white"},
			},
		},
		{
			attribute: db.ProductAttribute{ID: uuid.New(), Name: "size"},
			options:   []db.ProductAttributeOption{}, // no options
		},
	}

	result := cartesianProduct(opts)
	// 2 × 0 = 0 — any attribute with no options zeroes the product
	if len(result) != 0 {
		t.Errorf("expected 0 combinations when an attribute has no options, got %d", len(result))
	}
}

// --------------------------------------------------------------------------
// Tests for buildSKU
// --------------------------------------------------------------------------

func TestBuildSKU(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		combo  []comboEntry
		want   string
	}{
		{
			name:   "standard prefix with two options",
			prefix: "BAG",
			combo: []comboEntry{
				{option: db.ProductAttributeOption{Value: "black"}},
				{option: db.ProductAttributeOption{Value: "large"}},
			},
			want: "BAG-BLA-LAR",
		},
		{
			name:   "short option values stay intact",
			prefix: "TSH",
			combo: []comboEntry{
				{option: db.ProductAttributeOption{Value: "red"}},
				{option: db.ProductAttributeOption{Value: "xl"}},
			},
			want: "TSH-RED-XL",
		},
		{
			name:   "exactly three chars",
			prefix: "HAT",
			combo: []comboEntry{
				{option: db.ProductAttributeOption{Value: "tan"}},
			},
			want: "HAT-TAN",
		},
		{
			name:   "empty prefix",
			prefix: "",
			combo: []comboEntry{
				{option: db.ProductAttributeOption{Value: "black"}},
			},
			want: "-BLA",
		},
		{
			name:   "no options",
			prefix: "BAG",
			combo:  []comboEntry{},
			want:   "BAG",
		},
		{
			name:   "lowercase values uppercased",
			prefix: "SHOE",
			combo: []comboEntry{
				{option: db.ProductAttributeOption{Value: "blue"}},
				{option: db.ProductAttributeOption{Value: "medium"}},
			},
			want: "SHOE-BLU-MED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSKU(tt.prefix, tt.combo)
			if got != tt.want {
				t.Errorf("buildSKU(%q, ...) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for comboKey — deterministic regardless of order
// --------------------------------------------------------------------------

func TestComboKey_Deterministic(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	id3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	combo1 := []comboEntry{
		{option: db.ProductAttributeOption{ID: id1}},
		{option: db.ProductAttributeOption{ID: id2}},
		{option: db.ProductAttributeOption{ID: id3}},
	}
	combo2 := []comboEntry{
		{option: db.ProductAttributeOption{ID: id3}},
		{option: db.ProductAttributeOption{ID: id1}},
		{option: db.ProductAttributeOption{ID: id2}},
	}

	key1 := comboKey(combo1)
	key2 := comboKey(combo2)

	if key1 != key2 {
		t.Errorf("comboKey should be order-independent: %q != %q", key1, key2)
	}
}

func TestComboKey_SingleOption(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	combo := []comboEntry{
		{option: db.ProductAttributeOption{ID: id}},
	}

	key := comboKey(combo)
	if key != id.String() {
		t.Errorf("single option comboKey = %q, want %q", key, id.String())
	}
}

// --------------------------------------------------------------------------
// Tests for optionSetKey — deterministic regardless of order
// --------------------------------------------------------------------------

func TestOptionSetKey_Deterministic(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	opts1 := []db.ListVariantOptionsRow{
		{OptionID: id1},
		{OptionID: id2},
	}
	opts2 := []db.ListVariantOptionsRow{
		{OptionID: id2},
		{OptionID: id1},
	}

	key1 := optionSetKey(opts1)
	key2 := optionSetKey(opts2)

	if key1 != key2 {
		t.Errorf("optionSetKey should be order-independent: %q != %q", key1, key2)
	}
}

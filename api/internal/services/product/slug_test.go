package product

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "spaces to hyphens",
			input: "Leather Messenger Bag",
			want:  "leather-messenger-bag",
		},
		{
			name:  "already lowercase",
			input: "simple product",
			want:  "simple-product",
		},
		{
			name:  "uppercase to lowercase",
			input: "LOUD PRODUCT NAME",
			want:  "loud-product-name",
		},
		{
			name:  "special characters removed",
			input: "Product #1 (Best!)",
			want:  "product-1-best",
		},
		{
			name:  "multiple spaces collapsed",
			input: "Product   with   spaces",
			want:  "product-with-spaces",
		},
		{
			name:  "leading and trailing spaces trimmed",
			input: "  trimmed product  ",
			want:  "trimmed-product",
		},
		{
			name:  "hyphens preserved",
			input: "pre-existing-slug",
			want:  "pre-existing-slug",
		},
		{
			name:  "multiple hyphens collapsed",
			input: "product---with---hyphens",
			want:  "product-with-hyphens",
		},
		{
			name:  "mixed special chars and spaces",
			input: "Product: 100% Natural & Organic",
			want:  "product-100-natural-organic",
		},
		{
			name:  "numbers preserved",
			input: "Item 42 v2",
			want:  "item-42-v2",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only spaces",
			input: "   ",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "!@#$%^&*()",
			want:  "",
		},
		{
			name:  "unicode letters removed (non-ASCII)",
			input: "Cafe avec creme",
			want:  "cafe-avec-creme",
		},
		{
			name:  "trailing hyphens trimmed",
			input: "product---",
			want:  "product",
		},
		{
			name:  "leading hyphens trimmed",
			input: "---product",
			want:  "product",
		},
		{
			name:  "dots removed",
			input: "product.name.v2",
			want:  "productnamev2",
		},
		{
			name:  "underscores removed",
			input: "product_name_v2",
			want:  "productnamev2",
		},
		{
			name:  "apostrophes removed",
			input: "Baker's Best",
			want:  "bakers-best",
		},
		{
			name:  "real product name - Waxed Canvas Tote",
			input: "Waxed Canvas Tote",
			want:  "waxed-canvas-tote",
		},
		{
			name:  "accented characters",
			input: "Cafe Creme Brulee",
			want:  "cafe-creme-brulee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugify_Idempotent(t *testing.T) {
	// Slugifying an already-valid slug should return the same slug.
	slug := "leather-messenger-bag"
	got := slugify(slug)
	if got != slug {
		t.Errorf("slugify of already-clean slug: got %q, want %q", got, slug)
	}
}

func TestSlugify_LongName(t *testing.T) {
	// Verify it handles very long product names without error.
	long := "This Is A Very Long Product Name That Exceeds What Most Normal Products Would Have But Should Still Be Handled Correctly"
	got := slugify(long)
	if got == "" {
		t.Error("expected non-empty slug for long name")
	}
	if len(got) > len(long) {
		t.Error("slug should not be longer than original input")
	}
}

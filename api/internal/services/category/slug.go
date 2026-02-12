package category

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
	multipleHyphens = regexp.MustCompile(`-{2,}`)
)

// slugify converts a string into a URL-friendly slug.
// It lowercases the input, replaces non-alphanumeric characters with hyphens,
// collapses consecutive hyphens, and trims leading/trailing hyphens.
func slugify(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = multipleHyphens.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

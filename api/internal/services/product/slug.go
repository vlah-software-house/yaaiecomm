package product

import (
	"regexp"
	"strings"
)

var (
	// nonAlphanumeric matches anything that is not a letter, digit, or hyphen.
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
	// multipleHyphens collapses runs of hyphens into one.
	multipleHyphens = regexp.MustCompile(`-{2,}`)
)

// slugify converts a human-readable name into a URL-safe slug.
// Example: "Leather Messenger Bag" -> "leather-messenger-bag"
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlphanumeric.ReplaceAllString(s, "")
	s = multipleHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

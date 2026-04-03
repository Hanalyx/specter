package reverse

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSpecID creates a kebab-case spec ID from a file path.
// The result matches the pattern ^[a-z][a-z0-9-]*$ required by the spec schema.
func GenerateSpecID(filePath string) string {
	// Get base name without extension
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Strip common suffixes
	for _, suffix := range []string{".test", ".spec", "_test", ".route", ".handler", ".controller", ".service", ".model"} {
		name = strings.TrimSuffix(name, suffix)
	}

	// Convert camelCase/PascalCase to kebab-case
	name = camelToKebab(name)

	// Lowercase and replace non-alphanumeric with hyphens
	name = strings.ToLower(name)
	name = nonAlphanumRE.ReplaceAllString(name, "-")

	// Trim leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it starts with a letter
	if len(name) == 0 || !unicode.IsLetter(rune(name[0])) {
		name = "spec-" + name
	}

	if name == "" || name == "spec-" {
		name = "unknown-spec"
	}

	return name
}

// camelToKebab converts CamelCase to kebab-case.
func camelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				result.WriteRune('-')
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

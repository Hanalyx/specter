package reverse

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

// genericFilenames are file basenames that are too common to produce unique spec IDs.
// When detected, the parent directory name is prepended.
var genericFilenames = map[string]bool{
	"index": true, "main": true, "route": true, "utils": true, "helpers": true,
	"types": true, "constants": true, "config": true, "models": true, "schema": true,
	"service": true, "handler": true, "controller": true, "middleware": true,
}

// GenerateSpecID creates a kebab-case spec ID from a file path.
// The result matches the pattern ^[a-z][a-z0-9-]*$ required by the spec schema.
// For generic filenames (index.ts, main.go, etc.), the parent directory is prepended.
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
	name = strings.Trim(name, "-")

	// For generic filenames, prepend parent directory
	if genericFilenames[name] {
		dir := filepath.Dir(filePath)
		parentDir := filepath.Base(dir)
		if parentDir != "" && parentDir != "." && parentDir != "/" {
			parentDir = camelToKebab(parentDir)
			parentDir = strings.ToLower(parentDir)
			parentDir = nonAlphanumRE.ReplaceAllString(parentDir, "-")
			parentDir = strings.Trim(parentDir, "-")
			if parentDir != "" {
				name = parentDir + "-" + name
			}
		}
	}

	// Ensure it starts with a letter
	if len(name) == 0 || !unicode.IsLetter(rune(name[0])) {
		name = "spec-" + name
	}

	if name == "" || name == "spec-" {
		name = "unknown-spec"
	}

	return name
}

// GenerateSpecIDFromRoute creates a kebab-case spec ID from an API route path.
// e.g. "/api/webhooks/stripe" -> "webhooks-stripe", "/api/blog/[slug]" -> "blog-slug"
func GenerateSpecIDFromRoute(routePath string) string {
	// Strip /api/ prefix
	path := routePath
	if idx := strings.Index(path, "/api/"); idx >= 0 {
		path = path[idx+len("/api/"):]
	} else {
		path = strings.TrimPrefix(path, "/")
	}

	// Replace path separators and brackets with hyphens
	path = strings.ReplaceAll(path, "/", "-")
	path = strings.ReplaceAll(path, "[", "")
	path = strings.ReplaceAll(path, "]", "")

	// Clean up and ensure valid spec ID
	path = strings.ToLower(path)
	path = nonAlphanumRE.ReplaceAllString(path, "-")
	path = strings.Trim(path, "-")

	if path == "" {
		return "api-root"
	}

	// Ensure starts with letter
	if len(path) > 0 && !unicode.IsLetter(rune(path[0])) {
		path = "api-" + path
	}

	return path
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

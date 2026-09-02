package reverse

import (
	"fmt"
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

// --- C-16: spec ids are unique within a run ---

// maxDisambiguationDepth bounds how many parent directory segments may be
// prepended before the numeric fallback takes over. Paths deeper than this
// exist, but an id built from sixteen segments has stopped being a name.
const maxDisambiguationDepth = 16

// pathSegments returns the parent directory segments of filePath, nearest
// first, kebab-cased and cleaned the same way an id is. For
// "internal/coverage/coverage.go" it returns ["coverage", "internal"].
func pathSegments(filePath string) []string {
	dir := filepath.ToSlash(filepath.Dir(filePath))
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	raw := strings.Split(strings.Trim(dir, "/"), "/")
	out := make([]string, 0, len(raw))
	for i := len(raw) - 1; i >= 0; i-- {
		seg := camelToKebab(raw[i])
		seg = strings.ToLower(seg)
		seg = nonAlphanumRE.ReplaceAllString(seg, "-")
		seg = strings.Trim(seg, "-")
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// deepenID prepends up to depth parent directory segments of filePath to id.
//
// A segment the id already carries is skipped rather than repeated, and does
// not count against depth. That matters for the genericFilenames allowlist,
// which has already folded the parent directory in: without the skip,
// "src/auth/index.ts" would deepen from "auth-index" to "auth-auth-index".
func deepenID(id, filePath string, depth int) string {
	if depth <= 0 {
		return id
	}
	out := id
	used := 0
	for _, seg := range pathSegments(filePath) {
		if used >= depth {
			break
		}
		if out == seg || strings.HasPrefix(out, seg+"-") {
			continue
		}
		out = seg + "-" + out
		used++
	}
	return out
}

// SpecIDRename records an id that disambiguation changed.
type SpecIDRename struct {
	GroupKey string // the source path or directory the spec came from
	From     string // the id that collided
	To       string // the id assigned instead
}

// DisambiguateSpecIDs assigns a unique id to every group.
//
// keys must be in a deterministic order; the caller sorts them. provisional
// holds the id each group would have had on its own. Every member of a
// colliding set is deepened, not all but one, so that adding a file later does
// not silently change which spec keeps the short name.
//
// Returns the final id per group key, and one rename record per id that
// changed, in the order the keys were supplied.
func DisambiguateSpecIDs(keys []string, provisional map[string]string) (map[string]string, []SpecIDRename) {
	final := make(map[string]string, len(keys))
	for _, k := range keys {
		final[k] = provisional[k]
	}

	for depth := 1; depth <= maxDisambiguationDepth; depth++ {
		colliding := collidingKeys(keys, final)
		if len(colliding) == 0 {
			break
		}
		progressed := false
		for _, k := range keys {
			if !colliding[k] {
				continue
			}
			cand := deepenID(provisional[k], k, depth)
			if cand != final[k] {
				progressed = true
			}
			final[k] = cand
		}
		// Every colliding member is out of path segments. Deepening further
		// changes nothing, so stop and let the numeric pass finish the job.
		if !progressed {
			break
		}
	}

	// Global uniqueness, in sorted key order. This also covers a deepened id
	// that landed on an id no earlier round considered a collision, such as
	// "coverage/coverage.go" deepening onto a file literally named
	// "coverage-coverage.go".
	taken := make(map[string]bool, len(keys))
	var renames []SpecIDRename
	for _, k := range keys {
		id := final[k]
		if taken[id] {
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s-%d", final[k], n)
				if !taken[cand] {
					id = cand
					break
				}
			}
		}
		taken[id] = true
		final[k] = id
		if id != provisional[k] {
			renames = append(renames, SpecIDRename{GroupKey: k, From: provisional[k], To: id})
		}
	}

	return final, renames
}

// collidingKeys returns the set of keys whose current id is shared with at
// least one other key.
func collidingKeys(keys []string, ids map[string]string) map[string]bool {
	count := make(map[string]int, len(keys))
	for _, k := range keys {
		count[ids[k]]++
	}
	out := make(map[string]bool)
	for _, k := range keys {
		if count[ids[k]] > 1 {
			out[k] = true
		}
	}
	return out
}

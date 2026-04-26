// Glob matching for test discovery (settings.tests_glob, --tests).
//
// filepath.Glob doesn't support `**` (recursive directory match). The
// natural form users write — `tests/**/*.py`, `internal/**/*_test.go` —
// silently fell through to walking the entire repo before this helper.
//
// globMatchWalk walks `.` once, applying matchGlob to each file's relative
// path. Returns the sorted list of matching files. Skips standard noise
// dirs (.git, node_modules, dist, .venv).
//
// matchGlob handles three wildcard forms:
//   - `*` matches any sequence within ONE path component (excluding `/`).
//   - `**` matches any number of path components (including zero).
//   - `?` matches exactly one character within a component.
//
// Anchored at the start of the path; trailing components must match.
//
// @spec spec-coverage
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// globMatchWalk walks `.` and returns paths matching the given glob.
// Returns paths in sorted order. Empty result is a valid outcome —
// callers (e.g. coverage --strict) detect it and surface a warning.
func globMatchWalk(pattern string) []string {
	var matches []string
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "dist": true,
		".venv": true, "venv": true, "__pycache__": true,
	}
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Normalize "./foo" to "foo" so the pattern can match without the prefix.
		rel := strings.TrimPrefix(path, "./")
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if matchGlob(pattern, rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	sort.Strings(matches)
	return matches
}

// matchGlob reports whether path matches the glob pattern. Supports
// `*`, `**`, and `?`. Anchored at both ends.
//
// Implementation: split both pattern and path on `/`, walk in parallel,
// recurse on `**` segments to try every possible "consume zero or more
// path components" choice.
func matchGlob(pattern, path string) bool {
	patParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	return matchParts(patParts, pathParts)
}

func matchParts(pat, path []string) bool {
	for len(pat) > 0 {
		head := pat[0]
		if head == "**" {
			// `**` consumes zero or more path components. Try each tail
			// position; succeed if any matches the remaining pattern.
			rest := pat[1:]
			for i := 0; i <= len(path); i++ {
				if matchParts(rest, path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		if !matchSegment(head, path[0]) {
			return false
		}
		pat = pat[1:]
		path = path[1:]
	}
	return len(path) == 0
}

// matchSegment matches one pattern segment (no `/`) against one path
// segment. Supports `*` (any chars) and `?` (one char).
func matchSegment(pat, segment string) bool {
	// Use filepath.Match for the well-tested per-segment semantics.
	// Match returns false on syntax errors; treat as no-match.
	ok, err := filepath.Match(pat, segment)
	if err != nil {
		return false
	}
	return ok
}

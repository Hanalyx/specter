// Unit tests for matchExcludePattern — spec-manifest C-29 / AC-44 / AC-45.
//
// @spec spec-manifest
package main

import "testing"

// Bare-name patterns (no glob metacharacters) match by directory name
// exactly. Back-compat with v0.x.
//
// @ac AC-44
func TestMatchExcludePattern_BareNameMatchesByDirName(t *testing.T) {
	cases := []struct {
		pattern string
		relPath string
		dirName string
		want    bool
	}{
		{"node_modules", "node_modules", "node_modules", true},
		{"node_modules", "src/node_modules", "node_modules", true}, // anywhere
		{"node_modules", "src/components", "components", false},
		{"dist", "dist", "dist", true},
		{"dist", "subdir/dist", "dist", true},
		{"vendor", "src/components", "components", false},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-44 pattern="+tc.pattern+" path="+tc.relPath, func(t *testing.T) {
			got := matchExcludePattern(tc.pattern, tc.relPath, tc.dirName)
			if got != tc.want {
				t.Errorf("matchExcludePattern(%q, %q, %q) = %v, want %v",
					tc.pattern, tc.relPath, tc.dirName, got, tc.want)
			}
		})
	}
}

// Glob patterns match against paths using `**`/`*`/`?` semantics.
//
// @ac AC-45
func TestMatchExcludePattern_GlobMatchesPath(t *testing.T) {
	cases := []struct {
		pattern string
		relPath string
		dirName string
		want    bool
	}{
		// `.claude/**` matches `.claude` and anything under it.
		{".claude/**", ".claude", ".claude", true},
		{".claude/**", ".claude/sessions", "sessions", true},
		{".claude/**", ".claude/settings/local.json", "local.json", true},
		{".claude/**", "src/.claude", ".claude", false}, // anchored at root

		// `**/worktrees` matches any `worktrees` at any depth.
		{"**/worktrees", "worktrees", "worktrees", true},
		{"**/worktrees", "src/worktrees", "worktrees", true},
		{"**/worktrees", "deep/nested/worktrees", "worktrees", true},
		{"**/worktrees", "src/components", "components", false},

		// `tests/fixtures/*` matches direct children of tests/fixtures.
		{"tests/fixtures/*", "tests/fixtures/foo", "foo", true},
		{"tests/fixtures/*", "tests/fixtures/bar", "bar", true},
		{"tests/fixtures/*", "tests/integration", "integration", false},
		{"tests/fixtures/*", "tests/fixtures/foo/nested", "nested", false}, // depth=1 only

		// Single-segment `*` patterns match anywhere in a path component.
		{"*.tmp", "build.tmp", "build.tmp", true},
		{"*.tmp", "src/main.go", "main.go", false},

		// `?` matches exactly one character.
		{"v?", "v1", "v1", true},
		{"v?", "vX", "vX", true},
		{"v?", "v12", "v12", false},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-45 pattern="+tc.pattern+" path="+tc.relPath, func(t *testing.T) {
			got := matchExcludePattern(tc.pattern, tc.relPath, tc.dirName)
			if got != tc.want {
				t.Errorf("matchExcludePattern(%q, %q, %q) = %v, want %v",
					tc.pattern, tc.relPath, tc.dirName, got, tc.want)
			}
		})
	}
}

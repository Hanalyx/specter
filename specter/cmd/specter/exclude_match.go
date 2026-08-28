// Exclude-pattern matching for settings.exclude (spec-manifest C-29).
//
// Two pattern flavors, distinguished by the presence of glob metacharacters
// (`*`, `?`):
//
//   - Bare name (no `*`/`?`): matches by directory NAME exactly. Back-compat
//     with v0.x; `node_modules`, `dist`, `vendor` all work as they always have.
//   - Glob pattern: matches against the RELATIVE PATH of the candidate
//     directory using the same `**`/`*`/`?` semantics as settings.tests_glob
//     (matchGlob in glob.go). `.claude/**` matches `.claude/` and everything
//     under it; `**/worktrees` matches any `worktrees` at any depth.
//
// The two forms can coexist in a single exclude list — each pattern is
// dispatched on its own characters.
package main

import "strings"

// matchExcludePattern reports whether the given pattern excludes the
// candidate directory at relPath (with directory name dirName).
//
// relPath is the directory's path relative to the walk root (e.g.,
// "src/components", ".claude", "tests/fixtures/foo"). The leading "./"
// MUST be stripped before calling.
//
// Per C-29, the dispatch is:
//   - If pattern contains a glob metacharacter (`*`, `?`), match via
//     matchGlob against relPath.
//   - Otherwise, match by directory name exactly.
func matchExcludePattern(pattern, relPath, dirName string) bool {
	if patternHasGlob(pattern) {
		return matchGlob(pattern, relPath)
	}
	return pattern == dirName
}

// manifestExcludesDir reports whether any manifest exclude pattern excludes
// this directory. It is the single predicate C-29 requires every default
// discovery walk to consult.
//
// One function rather than one loop per walk. bugs/SP-SP-016 is what the
// second copy looks like: the rule was written once, implemented in
// discoverSpecs, and never reached discoverTestFiles, so a workspace could
// exclude a vendored copy from spec discovery and still have its test files
// counted. A caller that walks and does not call this is now the only way to
// reintroduce that, and it is visible at the call site.
//
// path is the raw walk path; the leading "./" is stripped here so callers do
// not each have to remember to. name is the directory's own name, which the
// bare-name branch compares against.
//
// Scope, per C-29. This covers manifest policy only. A walk MAY skip
// directories of its own, and those stay at the call site: discoverTestFiles
// skips node_modules, dist and .git whatever the manifest says, and moving
// them here would apply them to spec discovery too.
func manifestExcludesDir(patterns []string, path, name string) bool {
	relPath := strings.TrimPrefix(path, "./")
	for _, pat := range patterns {
		if matchExcludePattern(pat, relPath, name) {
			return true
		}
	}
	return false
}

// patternHasGlob reports whether a pattern contains any glob
// metacharacter that selects path-match semantics over name-match.
// Square brackets `[...]` are NOT included — they're rare in
// exclude lists, and the simple split between name-match and
// path-match is more important than supporting the full glob alphabet.
func patternHasGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

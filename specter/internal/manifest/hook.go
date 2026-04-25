// Pre-push hook script generation and diff-classification logic for
// `init --install-hook`. The on-disk hook is a small shell script that
// invokes `specter pre-push-check` (a hidden subcommand); the actual
// classification logic lives in the pure ShouldBlockPush function so it
// can be tested without spawning git.
//
// @spec spec-manifest
package manifest

import (
	"fmt"
	"strings"
)

// PushDiffSummary describes a single push's diff in terms the hook needs to
// decide block vs. allow. Implementation files are .go / .ts / .js / .py /
// etc; test files match *_test.* / *.test.*; spec files end in .spec.yaml;
// doc files are .md or under docs/. AnnotationDelta is true when ANY line
// added in the diff (across any file) contains "@spec " or "@ac ".
type PushDiffSummary struct {
	ImplFilesChanged []string
	TestFilesChanged []string
	DocFilesChanged  []string
	SpecFilesChanged []string
	AnnotationDelta  bool
}

// ShouldBlockPush decides whether the pre-push hook should reject the push.
//
// The rule (AC-28): block if and only if the diff includes implementation
// file changes AND no annotation delta. Pure docs / spec / test changes are
// always allowed; impl changes paired with an @spec/@ac delta are allowed.
//
// Rationale: the discipline being enforced is "every code change traces to
// a test annotation that traces to a spec." Pure docs/spec/test pushes
// don't need a code-side annotation; impl changes do.
func ShouldBlockPush(diff PushDiffSummary) bool {
	if len(diff.ImplFilesChanged) == 0 {
		return false
	}
	return !diff.AnnotationDelta
}

// PrePushHookScript returns the shell script body written to
// .git/hooks/pre-push. Wrapped in shell-comment markers
// (`# specter:begin v1` / `# specter:end`) so re-running `init --install-hook`
// replaces only the fenced region. HTML-comment markers (used in the AI
// instruction files) would be invalid shell syntax — `<!--` parses as a
// `<` redirection and would cause every push to fail with a syntax error.
//
// The script delegates classification to `specter pre-push-check`, which
// reads git's pre-push stdin format (one line per ref) and exits non-zero
// when ShouldBlockPush returns true.
//
// `git push --no-verify` skips this hook entirely — that's git's behavior,
// not Specter's. Documented in the hook script comments for AC-29.
func PrePushHookScript() string {
	const body = `# Specter pre-push hook (v0.11+).
#
# Blocks pushes that change implementation files without adding or
# updating @spec / @ac annotations. Bypass with: git push --no-verify
#
# Reads git's standard pre-push stdin format and delegates to
# 'specter pre-push-check' for the diff analysis.

if ! command -v specter >/dev/null 2>&1; then
  echo "specter pre-push hook: 'specter' binary not found on PATH; skipping check" >&2
  exit 0
fi

specter pre-push-check "$@"
`
	markers := ShellMarkers("v1")
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(markers.Begin)
	b.WriteString("\n")
	b.WriteString(body)
	b.WriteString(markers.End)
	b.WriteString("\n")
	return b.String()
}

// IsImplFile reports whether path looks like an implementation source file
// (.go, .ts/.tsx, .js/.jsx, .py, .rs, .java, .c/.cc/.cpp/.h/.hpp). Test files
// (matching *_test.* or *.test.*) and spec files (*.spec.yaml) are excluded.
func IsImplFile(path string) bool {
	if isTestFile(path) || isSpecFile(path) || isDocFile(path) {
		return false
	}
	for _, ext := range []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go") ||
		strings.HasSuffix(path, "_test.py") ||
		strings.HasSuffix(path, ".test.ts") ||
		strings.HasSuffix(path, ".test.tsx") ||
		strings.HasSuffix(path, ".test.js") ||
		strings.HasSuffix(path, ".test.jsx") ||
		strings.HasSuffix(path, ".spec.ts") ||
		strings.HasSuffix(path, ".spec.tsx") ||
		strings.HasSuffix(path, ".spec.js")
}

func isSpecFile(path string) bool {
	return strings.HasSuffix(path, ".spec.yaml") || strings.HasSuffix(path, ".spec.yml")
}

func isDocFile(path string) bool {
	return strings.HasSuffix(path, ".md") || strings.HasPrefix(path, "docs/")
}

// FormatBlockedPushMessage renders the human-facing message printed to stderr
// when ShouldBlockPush returns true. Names the impl files and points at the
// likely fix.
func FormatBlockedPushMessage(diff PushDiffSummary) string {
	var b strings.Builder
	b.WriteString("specter pre-push: push blocked\n\n")
	b.WriteString(fmt.Sprintf("  %d implementation file(s) changed but no @spec / @ac annotation delta found:\n", len(diff.ImplFilesChanged)))
	for _, f := range diff.ImplFilesChanged {
		b.WriteString("    - " + f + "\n")
	}
	b.WriteString("\n  Either add a test that annotates the affected ACs, or push with `git push --no-verify` to bypass.\n")
	return b.String()
}

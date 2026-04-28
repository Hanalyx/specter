// Package migrate applies known-safe schema-drift rewrites to spec YAML
// files. Drives `specter doctor --fix`.
//
// Pure functions. No CLI deps, no I/O. The CLI layer handles file reads/
// writes; this package takes bytes and returns bytes.
//
// Extending the rewrite table: add an entry to `rewrites` below. Each
// rewrite has a predicate (does this parse-error match?) and a mutator
// (apply to YAML content, return new content). Keeping this as a table
// rather than branching code makes the v0.10+ migration surface easy to
// audit and extend.
//
// @spec spec-doctor
package migrate

import (
	"regexp"
	"strings"

	"github.com/Hanalyx/specter/internal/coverage"
)

// Result is the output of Apply — the potentially-rewritten YAML plus the
// list of rewrite names that were applied. Applied is empty when no known
// pattern matched.
type Result struct {
	Content []byte
	Applied []string
}

// rewrite describes one known-safe transformation.
type rewrite struct {
	name    string
	matches func(coverage.ParseErrorEntry) bool
	apply   func([]byte) ([]byte, bool)
}

// Extracts the field name from the standard "Unknown field 'X'" message
// the parser emits for additionalProperties violations.
var unknownFieldRE = regexp.MustCompile(`Unknown field '([^']+)'`)

// rewrites is the known-safe rewrite table. Order matters only when
// multiple rewrites could match the same error — currently they don't.
var rewrites = []rewrite{
	{
		name: "strip-trust-level",
		matches: func(e coverage.ParseErrorEntry) bool {
			if e.Type != "additionalProperties" {
				return false
			}
			m := unknownFieldRE.FindStringSubmatch(e.Message)
			return len(m) == 2 && m[1] == "trust_level"
		},
		apply: stripTrustLevel,
	},
}

// Apply consults the rewrite table for every parse error and applies each
// matching rewrite at most once per YAML body. Returns the (possibly
// unchanged) content plus the list of rewrite names that fired.
//
// C-10: known-safe rewrites, applied in-place.
func Apply(content []byte, parseErrors []coverage.ParseErrorEntry) (Result, error) {
	result := Result{Content: content}
	seen := map[string]bool{}

	for _, e := range parseErrors {
		for _, rw := range rewrites {
			if seen[rw.name] {
				continue
			}
			if !rw.matches(e) {
				continue
			}
			newContent, changed := rw.apply(result.Content)
			if !changed {
				continue
			}
			result.Content = newContent
			result.Applied = append(result.Applied, rw.name)
			seen[rw.name] = true
		}
	}
	return result, nil
}

// stripTrustLevel removes a `trust_level: <value>` line under the `spec:`
// key. Operates line-by-line to preserve surrounding formatting (comments,
// blank lines, etc.) — yaml.v3 round-trip would reformat the whole
// document, which violates AC-10's byte-preservation intent for other
// fields. Simple string removal is safe here because the parse error
// guarantees the field is at the top level under `spec:` with a scalar
// value.
func stripTrustLevel(content []byte) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	out := make([]string, 0, len(lines))
	changed := false
	// trust_level appears as `  trust_level: <value>` (2-space indent under
	// spec:). Match any indented `trust_level:` key — the parse error
	// already established the field exists at spec level.
	re := regexp.MustCompile(`^\s+trust_level\s*:\s*\S.*$`)
	for _, line := range lines {
		if re.MatchString(line) {
			changed = true
			continue // drop the line
		}
		out = append(out, line)
	}
	if !changed {
		return content, false
	}
	return []byte(strings.Join(out, "\n")), true
}

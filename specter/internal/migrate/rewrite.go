// Package migrate applies known-safe schema-drift rewrites to spec YAML
// files. Drives `specter doctor --fix`.
//
// Pure functions. No CLI deps, no I/O. The CLI layer handles file reads/
// writes; this package takes bytes and returns bytes.
//
// Extending the rewrite table: add an entry to `rewrites` below. Each
// rewrite has a predicate (does this parse-error match?) and an apply
// function (transforms YAML bytes; may refuse with a reason). Keeping
// this as a table rather than branching code makes the v0.10+ migration
// surface easy to audit and extend.
//
// @spec spec-doctor
package migrate

import (
	"regexp"
	"strings"

	"github.com/Hanalyx/specter/internal/coverage"
	"gopkg.in/yaml.v3"
)

// Result is the output of Apply — the potentially-rewritten YAML plus the
// list of rewrite names that fired and any rewrites that were matched
// but refused (Unhandled). Both lists are empty when no parse error
// matched any rewrite.
type Result struct {
	Content   []byte
	Applied   []string
	Unhandled []UnhandledRewrite
}

// UnhandledRewrite names a (rewrite, file-shape) pair that the migrate
// engine declined to apply because the line-based deletion would corrupt
// the file. Spec-doctor C-15 (v0.12 review fix). The CLI surfaces these
// in the `doctor --fix` summary as `needs manual edit` entries.
type UnhandledRewrite struct {
	Rewrite string
	Reason  string
}

// rewrite describes one known-safe transformation.
//
// apply returns (content, applied, unhandledReason). When unhandledReason
// is non-empty, the rewrite predicate matched but the YAML shape is
// structurally unsafe for line-based deletion — caller should record it
// as Unhandled and leave the content alone. When unhandledReason is empty
// and applied=true, the content was rewritten. applied=false with empty
// reason means the rewrite found nothing to do (no-op).
type rewrite struct {
	name    string
	matches func(coverage.ParseErrorEntry) bool
	apply   func([]byte) (content []byte, applied bool, unhandledReason string)
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
// unchanged) content plus the list of rewrite names that fired and any
// rewrites that were matched but refused due to unsafe YAML shape.
//
// C-11: known-safe rewrites, applied in-place.
// C-15: rewrites that match but encounter unsafe shapes refuse with a
// reason instead of producing a corrupted file.
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
			newContent, applied, reason := rw.apply(result.Content)
			if reason != "" {
				result.Unhandled = append(result.Unhandled, UnhandledRewrite{
					Rewrite: rw.name,
					Reason:  reason,
				})
				seen[rw.name] = true
				continue
			}
			if !applied {
				continue
			}
			result.Content = newContent
			result.Applied = append(result.Applied, rw.name)
			seen[rw.name] = true
		}
	}
	return result, nil
}

// stripTrustLevel removes a `trust_level: <plain-scalar>` line under the
// `spec:` key. Operates line-by-line to preserve surrounding formatting
// (comments, blank lines, etc.) — yaml.v3 round-trip would reformat the
// whole document, which violates AC-12's byte-preservation intent for
// other fields.
//
// Refuses (returns reason) when yaml.v3 inspection determines the
// value's YAML shape is unsafe for line-based deletion: block scalar
// (`|` / `>`), sequence value, mapping value, anchor/alias. Per
// spec-doctor C-15. Plain scalar values (`high`, `"high"`, `0.5`)
// continue to rewrite as before.
func stripTrustLevel(content []byte) ([]byte, bool, string) {
	safe, reason := canSafelyStripTrustLevel(content)
	if !safe {
		return content, false, reason
	}
	// Safe to apply line-based deletion. The regex matches any indented
	// `trust_level:` key with a non-whitespace value on the same line —
	// yaml.v3 has already confirmed there's no continuation content.
	lines := strings.Split(string(content), "\n")
	out := make([]string, 0, len(lines))
	changed := false
	re := regexp.MustCompile(`^\s+trust_level\s*:\s*\S.*$`)
	for _, line := range lines {
		if re.MatchString(line) {
			changed = true
			continue // drop the line
		}
		out = append(out, line)
	}
	if !changed {
		return content, false, ""
	}
	return []byte(strings.Join(out, "\n")), true, ""
}

// canSafelyStripTrustLevel uses yaml.v3 to inspect the YAML shape of the
// `spec.trust_level` value (if present). Returns (true, "") for a plain
// scalar or absent key — both safe for the line-based deletion path.
// Returns (false, reason) for any non-scalar or block-scalar shape; the
// caller surfaces the reason as an Unhandled diagnostic.
//
// Spec-doctor C-15. Detection lives at the parsing layer rather than as
// a tightened regex because YAML scalar styles (literal/folded/anchored/
// aliased) cannot be reliably distinguished without parsing — a regex
// that tried would either be too loose (leaks corruption) or too strict
// (rejects safe forms).
func canSafelyStripTrustLevel(content []byte) (bool, string) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		// File doesn't even parse as YAML. Specter parse already reported
		// the error to the operator; refuse the rewrite cautiously rather
		// than blindly slicing lines out of an unparseable file.
		return false, "yaml parse failed: " + err.Error()
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return false, "empty document"
	}
	root := doc.Content[0]
	spec := findMappingValue(root, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		// No spec mapping at the root → nothing to strip; treat as safe
		// no-op (the regex will find nothing to match).
		return true, ""
	}
	val := findMappingValue(spec, "trust_level")
	if val == nil {
		return true, "" // key absent — safe no-op
	}
	if val.Kind != yaml.ScalarNode {
		switch val.Kind {
		case yaml.SequenceNode:
			return false, "trust_level value is not a scalar (sequence)"
		case yaml.MappingNode:
			return false, "trust_level value is not a scalar (mapping)"
		case yaml.AliasNode:
			return false, "trust_level value is not a scalar (alias)"
		default:
			return false, "trust_level value is not a scalar"
		}
	}
	if val.Style == yaml.LiteralStyle || val.Style == yaml.FoldedStyle {
		return false, "trust_level uses a block scalar"
	}
	return true, ""
}

// findMappingValue returns the value Node for a given key in a Mapping
// Node, or nil if the key is absent or m is not a mapping.
func findMappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

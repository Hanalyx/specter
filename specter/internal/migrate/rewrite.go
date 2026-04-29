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
	"bytes"
	"io"
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
// `spec.trust_level` value (if present) in every document of the file.
// Returns (true, "") only when EVERY document's trust_level (if any) is a
// plain scalar AND occupies exactly one source line. Returns (false,
// reason) on the first document that fails any of the checks.
//
// Spec-doctor C-15 (v1.6.0):
//
//   - (a) Kind != ScalarNode → refuse (sequence/mapping/alias)
//   - (b) Style is Literal/Folded → refuse (block scalar `|`/`>`)
//   - (c) value's source span > one line → refuse (folded plain scalar,
//     multi-line quoted scalar)
//
// Multi-document handling (AC-23): yaml.Decoder iterates each document;
// the safety check runs on each. The rewrite is per-file, so a single
// unsafe document forces refusal of the whole file — partial rewrite
// across documents is not byte-safe.
func canSafelyStripTrustLevel(content []byte) (bool, string) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			// File doesn't parse as YAML. Specter parse already reported
			// the error; refuse cautiously rather than slice lines out of
			// an unparseable file.
			return false, "yaml parse failed: " + err.Error()
		}
		if reason, ok := canSafelyStripTrustLevelInDoc(content, &doc); !ok {
			return false, reason
		}
	}
	return true, ""
}

// canSafelyStripTrustLevelInDoc runs the safety check on one document.
// Returns ("", true) for a safe plain scalar or absent key; (reason,
// false) for unsafe shapes.
func canSafelyStripTrustLevelInDoc(content []byte, doc *yaml.Node) (string, bool) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return "", true
	}
	root := doc.Content[0]
	spec := findMappingValue(root, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		// No spec mapping → no trust_level to strip; the regex will find
		// nothing in this doc. Treat as safe no-op.
		return "", true
	}
	keyNode, val := findMappingKeyValue(spec, "trust_level")
	if val == nil {
		return "", true
	}
	if val.Kind != yaml.ScalarNode {
		switch val.Kind {
		case yaml.SequenceNode:
			return "trust_level value is not a scalar (sequence)", false
		case yaml.MappingNode:
			return "trust_level value is not a scalar (mapping)", false
		case yaml.AliasNode:
			return "trust_level value is not a scalar (alias)", false
		default:
			return "trust_level value is not a scalar", false
		}
	}
	if val.Style == yaml.LiteralStyle || val.Style == yaml.FoldedStyle {
		return "trust_level uses a block scalar", false
	}
	// AC-21/22 line-span check. yaml.v3 reports the value's start position
	// via Node.Line; this walks the source to verify the value occupies
	// only that one line. Catches folded plain scalars and multi-line
	// quoted scalars that pass the kind/style checks but would still
	// orphan continuation lines under line-based deletion.
	if !valueOccupiesOneSourceLine(content, val.Line, keyNode.Column) {
		return "trust_level value spans multiple lines", false
	}
	return "", true
}

// valueOccupiesOneSourceLine returns true when the value rooted at
// (1-based) `valLine` does NOT extend onto subsequent source lines as a
// continuation of a folded plain scalar or a multi-line quoted scalar.
// `keyCol` is the 1-based column where the parent key starts; any
// non-blank line whose indentation exceeds `keyCol-1` after `valLine` is
// treated as continuation.
//
// Document separators (`---`, `...`) end the current document and the
// search. Blank lines and YAML comments are skipped.
func valueOccupiesOneSourceLine(content []byte, valLine int, keyCol int) bool {
	parentIndent := keyCol - 1 // 0-based indent count of the parent key
	lines := strings.Split(string(content), "\n")
	// 1-based valLine maps to lines[valLine-1] for the value's first
	// source line; we walk lines[valLine] onward.
	for j := valLine; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "---" || trimmed == "..." {
			// End of current YAML document — anything beyond is a separate
			// document, not a continuation of this value.
			return true
		}
		nextIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if nextIndent <= parentIndent {
			// Same or lesser indent → next sibling key (or unrelated
			// content). Value occupies just its first line.
			return true
		}
		// Greater indent → continuation. Value spans multiple lines.
		return false
	}
	return true
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

// findMappingKeyValue is findMappingValue but also returns the key Node
// (so callers can inspect the key's source position via Line/Column).
func findMappingKeyValue(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return k, m.Content[i+1]
		}
	}
	return nil, nil
}

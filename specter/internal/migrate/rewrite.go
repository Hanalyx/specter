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

// stripTrustLevel removes the `trust_level: <plain-scalar>` line under
// the `spec:` key, by yaml.v3 key-node line number (NOT by content-
// pattern regex). yaml.v3 already knows which source lines are keys
// and which are scalar content, so a mention of `trust_level: high`
// inside a description block scalar is never matched. C-17 mandates
// this mechanism; C-15 mandates refusal when the value's shape would
// require multi-line deletion.
//
// Refuses (returns reason) when the value's YAML shape is unsafe:
// block scalar (`|` / `>`), sequence/mapping value, anchor/alias, or
// any plain/quoted scalar whose source span exceeds one line. Plain
// scalar values on a single line (`high`, `"high"`, `0.5`) rewrite as
// before; surrounding lines (comments, blank lines, description blocks)
// are byte-preserved.
func stripTrustLevel(content []byte) ([]byte, bool, string) {
	lines, reason := trustLevelLineSet(content)
	if reason != "" {
		return content, false, reason
	}
	if len(lines) == 0 {
		return content, false, ""
	}
	return deleteSourceLines(content, lines), true, ""
}

// trustLevelLineSet inspects every YAML document in `content` and
// returns the set of (1-based) source line numbers occupied by the
// `spec.trust_level` key. yaml.v3 reports the key node's absolute line
// number across the stream, so multi-document files yield the correct
// per-doc lines.
//
// Returns (nil, reason) on the first document whose trust_level value
// has a structurally unsafe shape (C-15): non-scalar value, block
// scalar `|`/`>`, or a scalar whose source span exceeds one line.
// Whole-file refusal is intentional — partial rewrite across documents
// is not byte-safe.
//
// Returns (nil, "yaml parse failed: ...") when the file does not parse
// as YAML — specter parse has already reported the error; refuse rather
// than slice lines out of an unparseable file.
//
// Returns ([], "") when no document declares trust_level (no-op).
//
// C-17 (v1.9.0).
func trustLevelLineSet(content []byte) ([]int, string) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var lines []int
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "yaml parse failed: " + err.Error()
		}
		line, reason := docTrustLevelLine(content, &doc)
		if reason != "" {
			return nil, reason
		}
		if line > 0 {
			lines = append(lines, line)
		}
	}
	return lines, ""
}

// docTrustLevelLine inspects one document and returns the key-node
// source line for the `spec.trust_level` key. Returns (0, "") when the
// key is absent. Returns (0, reason) for any unsafe shape per C-15:
//
//   - (a) Kind != ScalarNode (sequence, mapping, alias)
//   - (b) Style is Literal/Folded (block scalars `|` / `>`)
//   - (c) value's source span exceeds one line (folded plain scalar,
//     multi-line quoted scalar, plain-scalar continuation)
func docTrustLevelLine(content []byte, doc *yaml.Node) (int, string) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return 0, ""
	}
	root := doc.Content[0]
	spec := findMappingValue(root, "spec")
	if spec == nil || spec.Kind != yaml.MappingNode {
		return 0, ""
	}
	keyNode, val := findMappingKeyValue(spec, "trust_level")
	if val == nil {
		return 0, ""
	}
	if val.Kind != yaml.ScalarNode {
		switch val.Kind {
		case yaml.SequenceNode:
			return 0, "trust_level value is not a scalar (sequence)"
		case yaml.MappingNode:
			return 0, "trust_level value is not a scalar (mapping)"
		case yaml.AliasNode:
			return 0, "trust_level value is not a scalar (alias)"
		default:
			return 0, "trust_level value is not a scalar"
		}
	}
	if val.Style == yaml.LiteralStyle || val.Style == yaml.FoldedStyle {
		return 0, "trust_level uses a block scalar"
	}
	if !valueOccupiesOneSourceLine(content, val.Line, keyNode.Column) {
		return 0, "trust_level value spans multiple lines"
	}
	return keyNode.Line, ""
}

// deleteSourceLines returns content with the (1-based) line numbers in
// lineNums removed. All other bytes are byte-preserved. C-17 line-
// targeted deletion mechanism: callers obtain lineNums from yaml.v3
// Node.Line so string-literal contents of the source are never touched.
func deleteSourceLines(content []byte, lineNums []int) []byte {
	skip := make(map[int]bool, len(lineNums))
	for _, n := range lineNums {
		skip[n] = true
	}
	src := strings.Split(string(content), "\n")
	out := make([]string, 0, len(src))
	for i, line := range src {
		if skip[i+1] { // i is 0-based; yaml.v3 Line is 1-based
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
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

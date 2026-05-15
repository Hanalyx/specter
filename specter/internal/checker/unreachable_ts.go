// TS/Jest/Vitest test-file reachability detection for
// unreachable_annotation (C-10).
//
// TypeScript / JavaScript don't have a Go-native parser comparable to
// go/ast, so this scanner uses a careful regex + state-machine
// approach. It handles the common test shapes without false positives
// by routing ambiguous cases to unreachable_annotation_unknown:
//
//   - it("title", ...)         — Convention A via title literal
//   - test("title", ...)       — Convention A
//   - describe("title", ...)   — Convention A (parent title)
//   - it(name, ...)            — name is identifier → AMBIGUOUS → _unknown
//   - console.log("// @spec ...") inside the test body — Convention B
//
// Supported shapes are scoped to single-line `it(...)` / `test(...)` /
// `describe(...)` header patterns. Multi-line headers (opening paren on
// one line, name string on next), template-literal titles
// (it(`[${spec}/${ac}] desc`, ...)), and string-concatenated titles
// route to _unknown. These are documented v0.13.1 candidates.
//
// @spec spec-check
package checker

import (
	"regexp"
	"strings"
)

// Match an it()/test()/describe() call on a single line whose first
// arg is a string literal (single or double quote). The capture group
// is the literal's content with quotes stripped.
//
// Examples this matches:
//   it("foo-spec/AC-01 valid", ...)
//   test('foo-spec/AC-01 valid', ...)
//   describe("foo-spec/AC-01", ...)
//   it.skip("foo-spec/AC-01", ...)        — via .skip / .only / .each
//   it.only('foo-spec/AC-01', ...)
//
// Does NOT match (routes to _unknown via the test-entry detection):
//   it(name, ...)                          — identifier first arg
//   it(`[${spec}/${ac}] desc`, ...)        — template literal
//   it("title with " + suffix, ...)        — concatenation
//   it(                                    — multi-line call header
//       "title",
//       ...
//   )
var reTsTestCall = regexp.MustCompile(`\b(?:it|test|describe)(?:\.\w+)?\s*\(\s*(?:"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)')\s*,`)

// Match an it()/test()/describe() call where the first arg is NOT a
// bare string literal — i.e., variable, template literal, or
// expression. We need to detect these to mark the corresponding @ac
// as ambiguous (routed to _unknown).
var reTsTestCallAmbiguous = regexp.MustCompile(`\b(?:it|test|describe)(?:\.\w+)?\s*\(\s*[^"'\s)][^,)]*,`)

// Match the entire `it.skip(...)` / `it.only(...)` / etc. prefix to
// confirm we're looking at a test entry (not, say, `myit(`).
var reTsTestCallHeader = regexp.MustCompile(`\b(?:it|test|describe)(?:\.\w+)?\s*\(`)

// Match a console.log / console.info / console.debug call with any
// single-line string literal arg. Group 1 = quoted content (single
// quotes), Group 2 = double quotes.
var reTsConsoleEmit = regexp.MustCompile(`\bconsole\.(?:log|info|debug|warn|error)\s*\(\s*(?:"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)')\s*[,)]`)

// scanTsFile implements the C-10 contract for TS/JS test files.
//
// Algorithm:
//  1. Scan lines, tracking multi-line string state. Collect every
//     @spec / @ac source comment (matches the existing test_annotations
//     scanner shape).
//  2. Identify test-entry boundaries: each line where a recognized
//     test-call header (it/test/describe with literal title) appears.
//     A test entry's body spans from the header line to the matching
//     closing brace/paren (depth-tracked).
//  3. Attribute each @ac to the next test entry following its line.
//     If the next test-entry header is "ambiguous" (non-literal first
//     arg), route the @ac to _unknown.
//  4. For the attributed test entry: check the title literal for
//     spec-id/AC-NN (Convention A). Then scan the body for
//     console.log emits carrying @spec/<id> or @ac/<id>
//     (Convention B). If neither, the @ac is unreachable.
//  5. If no test entry follows an @ac, route to _unknown.
func scanTsFile(path, content string, strictness string) []CheckDiagnostic {
	lines := strings.Split(content, "\n")
	entries := collectTsTestEntries(lines)
	comments := collectTsAnnotations(lines)

	var diags []CheckDiagnostic
	for _, ac := range comments.acs {
		specID := comments.activeSpecAt(ac.line)
		entry := entries.attributeAc(ac.line)
		if entry == nil {
			// @ac after every test entry — no destination.
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation_unknown",
				Severity: "warning",
				Message:  formatUnknownMessage(path, ac.line, ac.id),
			})
			continue
		}
		if entry.ambiguous {
			// Test header's first arg is not a string literal —
			// runner-visible title is dynamic; emit _unknown.
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation_unknown",
				Severity: "warning",
				Message:  formatUnknownMessage(path, ac.line, ac.id),
			})
			continue
		}
		if entry.reachesViaTitle(specID, ac.id) {
			continue
		}
		if entry.reachesViaConsoleLog(lines, specID, ac.id) {
			continue
		}
		sev, suppress := routeSeverity(strictness)
		if suppress {
			continue
		}
		diags = append(diags, CheckDiagnostic{
			Kind:     "unreachable_annotation",
			Severity: sev,
			Message:  formatUnreachableMessage(path, ac.line, ac.id, specID),
		})
	}

	return diags
}

// tsTestEntry captures one it()/test()/describe() block.
type tsTestEntry struct {
	headerLine int    // 1-indexed line of the it(...) header
	endLine    int    // last line of the entry's body (closing brace)
	title      string // literal title (empty when ambiguous)
	ambiguous  bool   // true when the header's first arg is not a literal
	kind       string // "it", "test", or "describe"
}

// reachesViaTitle reports whether the test entry's title literal
// carries the spec-id/AC-NN pair.
func (e *tsTestEntry) reachesViaTitle(specID, acID string) bool {
	if e.ambiguous {
		return false
	}
	pair := acID
	if specID != "" {
		pair = specID + "/" + acID
	}
	return pair != "" && strings.Contains(e.title, pair)
}

// reachesViaConsoleLog reports whether any console.log line in the
// entry's body emits "@spec <specID>" or "@ac <acID>".
func (e *tsTestEntry) reachesViaConsoleLog(lines []string, specID, acID string) bool {
	bodyStart := e.headerLine - 1 // include header line; console.log usually below it
	bodyEnd := e.endLine
	if bodyEnd > len(lines) {
		bodyEnd = len(lines)
	}
	for i := bodyStart; i < bodyEnd; i++ {
		matches := reTsConsoleEmit.FindAllStringSubmatch(lines[i], -1)
		for _, m := range matches {
			// m[1] is double-quoted content, m[2] is single-quoted.
			content := m[1]
			if content == "" {
				content = m[2]
			}
			if acID != "" && strings.Contains(content, "@ac "+acID) {
				return true
			}
			if specID != "" && strings.Contains(content, "@spec "+specID) {
				return true
			}
		}
	}
	return false
}

// tsTestEntries is a typed slice with helpers.
type tsTestEntries []tsTestEntry

// attributeAc returns the test entry that owns the @ac comment.
//
// Selection rules (in order; closes B2):
//  1. If an it() or test() entry's body strictly contains the @ac
//     line, pick the innermost one. (Inside the actual test body,
//     the @ac is unambiguously for that test.)
//  2. If only describe() entries contain the line, check whether an
//     it/test entry follows the @ac WITHIN the innermost containing
//     describe's body. If so, prefer that following it/test —
//     it's the test the @ac is documenting. (A `// @ac` comment
//     placed above an it() inside a describe block.)
//  3. Otherwise, fall back to the innermost containing describe —
//     the @ac is in describe scope but no specific it() follows
//     before the describe ends.
//  4. If nothing contains the line, pick the next entry whose header
//     starts at or after the @ac line.
//  5. Returns nil if no entry follows.
func (es tsTestEntries) attributeAc(line int) *tsTestEntry {
	// Rule 1: innermost containing it/test.
	var innermostIt *tsTestEntry
	for i := range es {
		if line >= es[i].headerLine && line <= es[i].endLine {
			if es[i].kind == "it" || es[i].kind == "test" {
				if innermostIt == nil || es[i].headerLine > innermostIt.headerLine {
					innermostIt = &es[i]
				}
			}
		}
	}
	if innermostIt != nil {
		return innermostIt
	}

	// Find the innermost containing describe (if any).
	var innermostDescribe *tsTestEntry
	for i := range es {
		if line >= es[i].headerLine && line <= es[i].endLine {
			if es[i].kind == "describe" {
				if innermostDescribe == nil || es[i].headerLine > innermostDescribe.headerLine {
					innermostDescribe = &es[i]
				}
			}
		}
	}

	if innermostDescribe != nil {
		// Rule 2: it/test inside the describe, AFTER the @ac line.
		for i := range es {
			if (es[i].kind == "it" || es[i].kind == "test") &&
				es[i].headerLine > line &&
				es[i].headerLine <= innermostDescribe.endLine {
				return &es[i]
			}
		}
		// Rule 3: fall back to the describe itself.
		return innermostDescribe
	}

	// Rule 4: next entry starting at/after line.
	for i := range es {
		if es[i].headerLine >= line {
			return &es[i]
		}
	}
	return nil
}

// extractTsEntryKind returns "it", "test", or "describe" based on
// the header line. The reTsTestCallHeader pattern guarantees one of
// these starts the call.
func extractTsEntryKind(line string) string {
	// Find the call shape in the line; the matcher captures
	// (?:it|test|describe)(?:\.\w+)? as part of the header pattern.
	idx := strings.Index(line, "describe")
	if idx >= 0 && isWordStart(line, idx) {
		return "describe"
	}
	idx = strings.Index(line, "test")
	if idx >= 0 && isWordStart(line, idx) {
		return "test"
	}
	idx = strings.Index(line, "it")
	if idx >= 0 && isWordStart(line, idx) {
		return "it"
	}
	return "it" // default; reTsTestCallHeader matched something
}

// isWordStart reports whether the position `idx` in `s` is the start
// of a word (preceded by non-identifier character or string start).
func isWordStart(s string, idx int) bool {
	if idx == 0 {
		return true
	}
	c := s[idx-1]
	return !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$')
}

// collectTsTestEntries scans the lines and returns one entry per
// recognized it/test/describe call. Body spans are computed via brace
// counting starting from the header's opening paren/brace, with the
// existing multi-line string state machine to skip braces inside
// template literals or multi-line strings, plus a block-comment state
// machine to skip `/* ... */` content (closes B1 from agent review).
func collectTsTestEntries(lines []string) tsTestEntries {
	var out tsTestEntries
	var inBacktick, inTripleDouble, inTripleSingle bool
	var inBlockComment bool

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		preBacktick, preTripleD, preTripleS := inBacktick, inTripleDouble, inTripleSingle
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preBacktick || preTripleD || preTripleS {
			// Line started fully inside a multi-line string; skip.
			continue
		}

		// Update block-comment state. A line that begins fully inside
		// a `/* ... */` block (started on a prior line, not yet closed)
		// cannot register a test-entry header — `it()` inside a
		// docblock example is documentation, not a real test (B1).
		preBlock := inBlockComment
		inBlockComment = updateBlockCommentState(line, inBlockComment)
		if preBlock {
			continue
		}

		// Strip in-line `/* ... */` regions before header detection
		// (closes the variant where a docblock + a real `it()` share
		// a line, e.g., `/* old */ it("new", ...)`).
		cleanedHeader := stripInlineBlockComments(line)

		// Is this a test-entry header line?
		headerMatch := reTsTestCallHeader.FindStringSubmatch(cleanedHeader)
		if headerMatch == nil {
			continue
		}

		entry := tsTestEntry{headerLine: i + 1, kind: extractTsEntryKind(cleanedHeader)}

		// Try to extract a string-literal title from the cleaned line.
		if m := reTsTestCall.FindStringSubmatch(cleanedHeader); m != nil {
			if m[1] != "" {
				entry.title = m[1]
			} else {
				entry.title = m[2]
			}
		} else {
			// The header line has it/test/describe( but no extractable
			// literal title — either ambiguous (variable/template) or
			// multi-line (title on a later line). Either way, mark
			// ambiguous to route the corresponding @ac to _unknown.
			entry.ambiguous = true
		}

		// Find the body's closing brace via paren/brace tracking.
		entry.endLine = findTsEntryEnd(lines, i)
		out = append(out, entry)
	}
	return out
}

// updateBlockCommentState walks the line tracking `/* */` block-comment
// state. Returns the state at end-of-line. Handles the common case of
// single-line `/* ... */` and multi-line spans.
//
// Does NOT track string state — caller must skip lines that start
// inside a multi-line string before calling this. For mixed lines
// (`/* */ "foo /* not a comment */" /* */`), this overcounts in rare
// cases; the BLOCKER classes (B1/M1) are the dominant target.
func updateBlockCommentState(line string, inBlock bool) bool {
	i := 0
	for i < len(line) {
		if inBlock {
			j := strings.Index(line[i:], "*/")
			if j < 0 {
				return true // still inside at EOL
			}
			i += j + 2
			inBlock = false
			continue
		}
		// Not in block — find next `/*` or string-literal start.
		// We deliberately ignore single-line strings here; combined
		// with the multi-line string state in the caller, the typical
		// false positives (`"http://"` containing `*/`) don't trigger
		// because `*/` requires a preceding `/*` to enter the state.
		j := strings.Index(line[i:], "/*")
		if j < 0 {
			return false
		}
		i += j + 2
		inBlock = true
	}
	return inBlock
}

// stripInlineBlockComments removes single-line `/* ... */` regions
// from a line. Used to clean header-detection lines so `/* old */ it("new", ...)`
// extracts the real `it()` title rather than seeing the comment's
// content. Multi-line block comments are handled by the caller's
// state machine and are not stripped here.
func stripInlineBlockComments(line string) string {
	var out strings.Builder
	out.Grow(len(line))
	i := 0
	for i < len(line) {
		j := strings.Index(line[i:], "/*")
		if j < 0 {
			out.WriteString(line[i:])
			break
		}
		out.WriteString(line[i : i+j])
		k := strings.Index(line[i+j+2:], "*/")
		if k < 0 {
			// Unterminated on this line — leave it to the multi-line
			// state machine. Don't write the rest.
			break
		}
		i += j + 2 + k + 2
	}
	return out.String()
}

// findTsEntryEnd returns the 1-indexed line where the test entry
// starting at headerIndex (0-indexed) ends, via brace counting. If
// braces never balance, returns the file's last line.
//
// Strips single-line and multi-line block-comment content from the
// line before counting braces (closes M1). A `/* } */` in source
// would otherwise over-close the entry.
func findTsEntryEnd(lines []string, headerIndex int) int {
	depth := 0
	started := false
	var inBacktick, inTripleDouble, inTripleSingle bool
	var inBlockComment bool
	for i := headerIndex; i < len(lines); i++ {
		line := lines[i]
		preBacktick, preTripleD, preTripleS := inBacktick, inTripleDouble, inTripleSingle
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preBacktick || preTripleD || preTripleS {
			continue
		}
		// Strip block-comment content (single-line `/* ... */` and
		// continuation of multi-line) and update block state.
		cleanedBlock, blockNext := stripBlockCommentAndReturnState(line, inBlockComment)
		preBlock := inBlockComment
		inBlockComment = blockNext
		if preBlock && cleanedBlock == "" {
			// Entire line was inside a block comment.
			continue
		}
		// Strip line comments and single-line strings before counting.
		clean := stripTsStringsAndComments(cleanedBlock)
		opens := strings.Count(clean, "{")
		closes := strings.Count(clean, "}")
		if opens > 0 || closes > 0 {
			started = true
		}
		depth += opens - closes
		if started && depth <= 0 {
			return i + 1
		}
	}
	return len(lines)
}

// stripBlockCommentAndReturnState removes block-comment content from
// a line and returns (cleanedLine, isInsideBlockAtEOL). Handles:
//   - Continuation of a multi-line block (`inBlock` true on entry)
//   - Inline single-line `/* ... */` regions
//   - A `/*` that opens but doesn't close on this line
func stripBlockCommentAndReturnState(line string, inBlock bool) (string, bool) {
	var out strings.Builder
	out.Grow(len(line))
	i := 0
	for i < len(line) {
		if inBlock {
			j := strings.Index(line[i:], "*/")
			if j < 0 {
				return out.String(), true // still inside
			}
			i += j + 2
			inBlock = false
			continue
		}
		j := strings.Index(line[i:], "/*")
		if j < 0 {
			out.WriteString(line[i:])
			break
		}
		out.WriteString(line[i : i+j])
		i += j + 2
		inBlock = true
	}
	return out.String(), inBlock
}

// stripTsStringsAndComments returns the line with single-line string
// literals (single/double/backtick), line comments (// ...) replaced
// by spaces. Used to remove braces inside strings/comments from the
// brace counter.
//
// Limitations: doesn't handle multi-line strings (caller's
// updateMultilineStringState handles those). Doesn't handle escaped
// quotes inside backtick strings; the false-positive risk there is
// low for braces.
func stripTsStringsAndComments(line string) string {
	var out strings.Builder
	out.Grow(len(line))

	state := 0 // 0=code, 1=double, 2=single, 3=backtick, 4=lineComment, 5=blockComment(approx)
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch state {
		case 0:
			if c == '"' {
				state = 1
				out.WriteByte(' ')
				continue
			}
			if c == '\'' {
				state = 2
				out.WriteByte(' ')
				continue
			}
			if c == '`' {
				state = 3
				out.WriteByte(' ')
				continue
			}
			if c == '/' && i+1 < len(line) && line[i+1] == '/' {
				return out.String() // rest is a line comment
			}
			out.WriteByte(c)
		case 1:
			if c == '\\' && i+1 < len(line) {
				i++ // skip escaped char
				continue
			}
			if c == '"' {
				state = 0
			}
		case 2:
			if c == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if c == '\'' {
				state = 0
			}
		case 3:
			if c == '\\' && i+1 < len(line) {
				i++
				continue
			}
			if c == '`' {
				state = 0
			}
		}
	}
	return out.String()
}

// tsAnnotations is the same shape as astAnnotations from
// unreachable_go.go but populated from TS/JS source comments.
type tsAnnotations struct {
	specs []specComment
	acs   []acComment
}

func (a *tsAnnotations) activeSpecAt(line int) string {
	var current string
	for _, s := range a.specs {
		if s.line > line {
			break
		}
		current = s.id
	}
	return current
}

// collectTsAnnotations scans lines for `// @spec <id>` and
// `// @ac AC-NN` markers (and the /* ... */ block-comment equivalents),
// returning their line positions. Skips lines fully inside multi-line
// string literals.
func collectTsAnnotations(lines []string) tsAnnotations {
	var out tsAnnotations
	var inBacktick, inTripleDouble, inTripleSingle bool
	for i, line := range lines {
		preBacktick, preTripleD, preTripleS := inBacktick, inTripleDouble, inTripleSingle
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preBacktick || preTripleD || preTripleS {
			continue
		}
		if id, ok := extractSpecID(line); ok {
			out.specs = append(out.specs, specComment{line: i + 1, id: id})
			continue
		}
		if id, ok := extractAcID(line); ok {
			out.acs = append(out.acs, acComment{line: i + 1, id: id})
		}
	}
	return out
}

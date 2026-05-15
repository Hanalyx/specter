// Python test-file reachability detection for unreachable_annotation (C-10).
//
// Python's function names cannot contain `/` or `:`, so Convention A
// (spec-id/AC-NN in the test title) is structurally unavailable. Only
// Convention B (runtime print emissions of "// @spec ..." / "// @ac ...")
// can make a Python @ac source comment reachable to specter ingest.
//
// pytest reads `<system-out>` from JUnit XML when run with
// `-o junit_logging=all`, which carries the runtime prints into the
// ingest pipeline. This scanner detects whether the test function
// body contains such prints.
//
// Indentation-based block scoping replaces the brace counting used in
// the TS scanner. A test function's body spans from the line below
// `def test_<name>(...):` until the first non-blank, non-comment line
// whose indentation is <= the def's own indentation.
//
// Supported shapes:
//   - def test_<name>(...) — top-level function
//   - async def test_<name>(...) — async test
//   - def test_<name>(self, ...) — class method (pytest class-based tests)
//   - Conventions B emission via print("..." or '...') / print(r"..." or r'...')
//   - logging.<level>("// @spec ...") for any logger method name
//
// Routed to unreachable_annotation_unknown (warning, never error):
//   - f-strings: print(f"// @ac {ac_id}") — runtime values not statically resolvable
//   - .format() / % string interpolation in print args
//   - Multi-line print() calls
//   - Decorated test functions with non-trivial decorator (unrecognized shape)
//
// @spec spec-check
package checker

import (
	"regexp"
	"strings"
)

// rePyTestDef matches a Python test function definition. Captures the
// leading whitespace (group 1) and the function name (group 2).
var rePyTestDef = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+(test_\w*)\s*\(`)

// rePyPrintCall matches `print(...)` or any logger-shaped call
// (`<receiver>.<level>(...)`) whose first arg is a string literal
// (single or double quote, optionally prefixed with `r` or `R` for
// raw strings — NOT `f` or `F`, which indicate runtime interpolation).
//
// Logger-shaped recognition: any selector chain ending in one of the
// canonical Python logging method names (`debug`, `info`, `warning`,
// `error`, `critical`, `log`, `exception`). Matches:
//   - logging.info(...)                    — stdlib
//   - log.info(...)                        — common alias
//   - logger.info(...)                     — common alias
//   - LOG.info(...)                        — uppercase convention
//   - my_log.info(...)                     — local logger
//   - self.logger.info(...)                — class attribute
//   - logging.getLogger(__name__).info(...) — inline get-logger
//
// Group 1 = double-quoted content; group 2 = single-quoted content.
//
// f-strings deliberately do NOT match: `print(f"...")` falls through
// to non-emit, so the test routes to unreachable. (An f-string in a
// print MIGHT statically expand to the target string, but we can't
// know without evaluating expressions.)
var rePyPrintCall = regexp.MustCompile(`(?:\bprint|[\w.()]+\.(?:debug|info|warning|warn|error|critical|log|exception))\s*\(\s*r?(?:"([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)')\s*[,)]`)

// scanPyFile implements C-10 for Python test files.
func scanPyFile(path, content string, strictness string) []CheckDiagnostic {
	lines := strings.Split(content, "\n")
	defs := collectPyTestDefs(lines)
	comments := collectPyAnnotations(lines)

	var diags []CheckDiagnostic
	for _, ac := range comments.acs {
		specID := comments.activeSpecAt(ac.line)
		def := defs.attributeAc(ac.line)
		if def == nil {
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation_unknown",
				Severity: "warning",
				Message:  formatUnknownMessage(path, ac.line, ac.id),
			})
			continue
		}
		if def.reachesViaPrint(lines, specID, ac.id) {
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

// pyTestDef captures one `def test_<name>(...)` declaration.
type pyTestDef struct {
	name       string
	headerLine int
	endLine    int
	indent     int // # of leading-whitespace columns of the def
}

func (d *pyTestDef) reachesViaPrint(lines []string, specID, acID string) bool {
	bodyStart := d.headerLine // include def line; print is in body below
	bodyEnd := d.endLine
	if bodyEnd > len(lines) {
		bodyEnd = len(lines)
	}
	for i := bodyStart; i < bodyEnd; i++ {
		matches := rePyPrintCall.FindAllStringSubmatch(lines[i], -1)
		for _, m := range matches {
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

type pyTestDefs []pyTestDef

// attributeAc returns the test def that owns the @ac comment.
// Priority: enclosing body, then next-following def, else nil.
func (ds pyTestDefs) attributeAc(line int) *pyTestDef {
	for i := range ds {
		if line > ds[i].headerLine && line <= ds[i].endLine {
			return &ds[i]
		}
	}
	for i := range ds {
		if ds[i].headerLine >= line {
			return &ds[i]
		}
	}
	return nil
}

// collectPyTestDefs scans Python lines, identifies test-function
// definitions, and computes each body's line span via indentation.
//
// Indentation rules (PEP 8 conventional):
//   - A test function's body consists of lines indented MORE than
//     the def's own indentation.
//   - Body ends at the first line whose indentation is <= def's
//     indentation AND which is non-blank, non-comment.
//   - Blank lines and comment-only lines inside the body do NOT
//     terminate the body.
//
// Tabs are counted as a single column unit (we don't expand them).
// Mixed tab/space indentation in real codebases is rare; if a project
// hits a false attribution from mixed indentation, the operator can
// apply the per-file off-switch (TBD in v0.13.1).
func collectPyTestDefs(lines []string) pyTestDefs {
	var out pyTestDefs
	var inTripleDouble, inTripleSingle, inBacktick bool

	for i, line := range lines {
		preD, preS, preB := inTripleDouble, inTripleSingle, inBacktick
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preD || preS || preB {
			continue
		}
		// Skip line-comments at any indentation.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := rePyTestDef.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		def := pyTestDef{
			name:       m[2],
			headerLine: i + 1,
			indent:     len(m[1]),
		}
		def.endLine = findPyDefEnd(lines, i, def.indent)
		out = append(out, def)
	}
	return out
}

// findPyDefEnd returns the 1-indexed line where the test function
// body ends, via indentation tracking. The body ends at the first
// line (after the def) whose effective indentation is <= def.indent
// AND which is not blank or a pure comment.
func findPyDefEnd(lines []string, headerIndex, defIndent int) int {
	var inTripleDouble, inTripleSingle, inBacktick bool
	for i := headerIndex + 1; i < len(lines); i++ {
		line := lines[i]
		preD, preS, preB := inTripleDouble, inTripleSingle, inBacktick
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preD || preS || preB {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := pyIndent(line)
		if indent <= defIndent {
			return i // last body line is i-1 (0-indexed); we want 1-indexed end-of-body = i
		}
	}
	return len(lines)
}

// pyIndent returns the count of leading whitespace columns for a
// Python source line. Each tab counts as 1 column for simplicity.
func pyIndent(line string) int {
	n := 0
	for _, c := range line {
		if c == ' ' || c == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

// pyAnnotations holds @spec / @ac comments scanned from Python source.
type pyAnnotations struct {
	specs []specComment
	acs   []acComment
}

func (a *pyAnnotations) activeSpecAt(line int) string {
	var current string
	for _, s := range a.specs {
		if s.line > line {
			break
		}
		current = s.id
	}
	return current
}

// collectPyAnnotations scans Python source for `# @spec <id>` and
// `# @ac AC-NN` markers. Skips lines fully inside multi-line strings
// (e.g., docstrings) to avoid matching annotations that are payload.
func collectPyAnnotations(lines []string) pyAnnotations {
	var out pyAnnotations
	var inTripleDouble, inTripleSingle, inBacktick bool
	for i, line := range lines {
		preD, preS, preB := inTripleDouble, inTripleSingle, inBacktick
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if preD || preS || preB {
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

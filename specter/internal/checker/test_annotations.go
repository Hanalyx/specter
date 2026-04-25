// Test-annotation cross-reference check (C-09).
//
// Scans test file content for `// @spec <id>` and `// @ac AC-NN` source
// comments, then emits diagnostics for references that cannot be resolved
// against the parsed spec set. Three diagnostic kinds:
//
//   - unknown_spec_ref: @spec <id> names a spec that doesn't exist.
//   - unknown_ac_ref:   @spec is valid but the referenced @ac id is not
//     declared in that spec.
//   - malformed_ac_id:  @ac value fails the ^AC-\d{2,}$ pattern
//     (e.g. AC-1 not zero-padded, ac-01 wrong case, AC_01 wrong separator).
//
// Source-only detection (annotations without a runner-visible match)
// is deferred to v0.12 `unreachable_annotation`.
//
// @spec spec-check
package checker

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Hanalyx/specter/internal/schema"
)

// Regexes scoped to the test-annotation check. Intentionally separate from
// internal/coverage's extraction regexes — the checker wants the raw @ac
// value to classify malformed vs. unknown, whereas coverage wants only the
// strictly-parsable AC ids.
//
// tacLooseAcRE catches near-attempts: `\d+\w*` extends past the digit run so
// suffixed forms like `AC-1A` still flag as malformed instead of slipping
// past both regexes silently. Anchoring on a digit keeps prose words like
// "acceleration" from falsely matching.
var (
	tacSpecRefRE  = regexp.MustCompile(`^\s*(?://|#|\*)\s*@spec\s+([\w-]+)`)
	tacAcRefRE    = regexp.MustCompile(`^\s*(?://|#|\*)\s*@ac\s+(.+)`)
	tacStrictAcRE = regexp.MustCompile(`^AC-\d{2,}$`)
	tacLooseAcRE  = regexp.MustCompile(`(?i)\bac[-_]?\d+\w*\b`)
)

// CheckTestAnnotations scans test-file contents and returns diagnostics for
// @spec / @ac references that don't resolve against the parsed spec set.
//
// testFiles maps file path → file content. Pure function; no I/O.
//
// The diagnostic list is deterministic (files sorted alphabetically; line
// order preserved within each file).
func CheckTestAnnotations(testFiles map[string]string, specs []schema.SpecAST) []CheckDiagnostic {
	// Lookup: spec id → set of valid AC ids.
	validACs := make(map[string]map[string]bool, len(specs))
	for i := range specs {
		s := &specs[i]
		acs := make(map[string]bool, len(s.AcceptanceCriteria))
		for _, ac := range s.AcceptanceCriteria {
			acs[ac.ID] = true
		}
		validACs[s.ID] = acs
	}

	paths := make([]string, 0, len(testFiles))
	for p := range testFiles {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var diags []CheckDiagnostic
	for _, path := range paths {
		diags = append(diags, scanFileAnnotations(path, testFiles[path], validACs)...)
	}
	return diags
}

func scanFileAnnotations(path, content string, validACs map[string]map[string]bool) []CheckDiagnostic {
	var diags []CheckDiagnostic
	currentSpec := ""

	// Multi-line string state. Annotations appearing inside a backtick template
	// literal (TS/JS/Go raw string) or a Python triple-quoted string are
	// payload, not real annotations — skip them. Mirrors the scanner in
	// internal/coverage.ExtractAnnotations.
	var inBacktick, inTripleDouble, inTripleSingle bool

	for lineIdx, line := range strings.Split(content, "\n") {
		lineNum := lineIdx + 1
		lineStartsInString := inBacktick || inTripleDouble || inTripleSingle

		trimmed := strings.TrimSpace(line)
		isCommentLine := !lineStartsInString && (strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*"))

		if !isCommentLine {
			// Update string state on non-comment lines, then move on. Real
			// annotations only live in comment lines.
			inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(
				line, inBacktick, inTripleDouble, inTripleSingle,
			)
			continue
		}

		if m := tacSpecRefRE.FindStringSubmatch(line); len(m) > 1 {
			currentSpec = m[1]
			if _, known := validACs[currentSpec]; !known {
				diags = append(diags, CheckDiagnostic{
					Kind:     "unknown_spec_ref",
					Severity: "error",
					SpecID:   currentSpec,
					Message: fmt.Sprintf("test %s:%d references @spec %q but no spec with that id exists in the workspace",
						path, lineNum, currentSpec),
				})
			}
			continue
		}

		m := tacAcRefRE.FindStringSubmatch(line)
		if len(m) <= 1 {
			continue
		}
		raw := strings.TrimSpace(m[1])

		// An @ac line may carry one or more ids separated by commas/whitespace.
		tokens := strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == ';'
		})
		for _, tok := range tokens {
			switch {
			case tacStrictAcRE.MatchString(tok):
				if currentSpec == "" {
					// @ac without preceding @spec — out of scope for v0.11.
					continue
				}
				valid, ok := validACs[currentSpec]
				if !ok {
					// Parent spec already flagged as unknown_spec_ref.
					continue
				}
				if !valid[tok] {
					diags = append(diags, CheckDiagnostic{
						Kind:     "unknown_ac_ref",
						Severity: "error",
						SpecID:   currentSpec,
						Message: fmt.Sprintf("test %s:%d references @ac %s but spec %q does not declare that AC",
							path, lineNum, tok, currentSpec),
					})
				}
			case tacLooseAcRE.MatchString(tok):
				diags = append(diags, CheckDiagnostic{
					Kind:     "malformed_ac_id",
					Severity: "error",
					SpecID:   currentSpec,
					Message: fmt.Sprintf("test %s:%d has malformed AC id %q (expected ^AC-\\d{2,}$, e.g. AC-01)",
						path, lineNum, tok),
				})
			}
			// Tokens that match neither pattern are free-form prose — ignore.
		}
	}
	return diags
}

// updateMultilineStringState mirrors internal/coverage.updateMultilineStringState.
// Duplicated rather than imported because internal/coverage already imports
// internal/checker (cycle). BACKLOG candidate: extract to internal/textscan.
func updateMultilineStringState(line string, inBacktick, inTripleDouble, inTripleSingle bool) (bool, bool, bool) {
	inSingle := false
	inDouble := false
	n := len(line)
	for i := 0; i < n; {
		if inBacktick {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '`' {
				inBacktick = false
			}
			i++
			continue
		}
		if inTripleDouble {
			if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
				inTripleDouble = false
				i += 3
				continue
			}
			i++
			continue
		}
		if inTripleSingle {
			if i+2 < n && line[i] == '\'' && line[i+1] == '\'' && line[i+2] == '\'' {
				inTripleSingle = false
				i += 3
				continue
			}
			i++
			continue
		}
		if inSingle {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '\'' {
				inSingle = false
			}
			i++
			continue
		}
		if inDouble {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '"' {
				inDouble = false
			}
			i++
			continue
		}
		if i+1 < n && line[i] == '/' && line[i+1] == '/' {
			return inBacktick, inTripleDouble, inTripleSingle
		}
		if line[i] == '#' {
			return inBacktick, inTripleDouble, inTripleSingle
		}
		if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
			inTripleDouble = true
			i += 3
			continue
		}
		if i+2 < n && line[i] == '\'' && line[i+1] == '\'' && line[i+2] == '\'' {
			inTripleSingle = true
			i += 3
			continue
		}
		switch line[i] {
		case '`':
			inBacktick = true
		case '"':
			inDouble = true
		case '\'':
			inSingle = true
		}
		i++
	}
	return inBacktick, inTripleDouble, inTripleSingle
}

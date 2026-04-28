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

	// declaredSpecs accumulates every `@spec <id>` header seen in the file.
	// Multi-`@spec` files are legitimate (cross-cutting tests that bridge
	// two specs); each `@ac` line is validated against the *union* of
	// declared specs, not just the most recent one (closes GH #95).
	var declaredSpecs []string
	specSeen := map[string]bool{}

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
			specID := m[1]
			if !specSeen[specID] {
				specSeen[specID] = true
				declaredSpecs = append(declaredSpecs, specID)
			}
			if _, known := validACs[specID]; !known {
				diags = append(diags, CheckDiagnostic{
					Kind:     "unknown_spec_ref",
					Severity: "error",
					SpecID:   specID,
					Message: fmt.Sprintf("test %s:%d references @spec %q but no spec with that id exists in the workspace",
						path, lineNum, specID),
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
				if len(declaredSpecs) == 0 {
					// @ac without any preceding @spec — out of scope for v0.11.
					continue
				}
				// Collect the subset of declared specs that exist in the
				// workspace. If none exist, cascade-suppress (parent specs
				// already flagged as unknown_spec_ref).
				var knownDeclared []string
				for _, sid := range declaredSpecs {
					if _, ok := validACs[sid]; ok {
						knownDeclared = append(knownDeclared, sid)
					}
				}
				if len(knownDeclared) == 0 {
					continue
				}
				// Valid if the AC exists in ANY known declared spec
				// (multi-@spec files bridge specs; an AC need only be
				// declared by one of them).
				inAny := false
				for _, sid := range knownDeclared {
					if validACs[sid][tok] {
						inAny = true
						break
					}
				}
				if !inAny {
					// Name the declared specs in the error so the operator
					// can see which specs were checked.
					specsNamed := strings.Join(knownDeclared, ", ")
					reportSpec := knownDeclared[0]
					if len(knownDeclared) > 1 {
						reportSpec = "(" + specsNamed + ")"
					}
					diags = append(diags, CheckDiagnostic{
						Kind:     "unknown_ac_ref",
						Severity: "error",
						SpecID:   knownDeclared[0],
						Message: fmt.Sprintf("test %s:%d references @ac %s but no declared spec %s declares that AC",
							path, lineNum, tok, reportSpec),
					})
				}
			case tacLooseAcRE.MatchString(tok):
				// Use the most-recently-declared spec for context (just the
				// label on the diagnostic; the malformed-ID check itself is
				// independent of any spec).
				lastSpec := ""
				if len(declaredSpecs) > 0 {
					lastSpec = declaredSpecs[len(declaredSpecs)-1]
				}
				diags = append(diags, CheckDiagnostic{
					Kind:     "malformed_ac_id",
					Severity: "error",
					SpecID:   lastSpec,
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

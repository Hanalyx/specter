// Unreachable annotation detection per spec-check C-10.
//
// Detects source-comment @ac annotations whose corresponding test
// produces no runner-visible <spec-id>/<AC-NN> token — either as
// Convention A (in the test title/subtest name) or as Convention B
// (a runtime print/log call inside the test body emitting
// "// @spec" / "// @ac"). Such annotations would silently demote
// under `coverage --strict` because `specter ingest` never sees them.
//
// Language-aware reachability:
//   - Go: testing.T.Run subtest names + fmt.Println / t.Log / t.Logf body calls
//   - TS/Jest/Vitest: it / describe / test titles + console.log body calls
//   - Python: print / logging.* body calls (function names cannot carry / or :,
//     so Convention A is structurally unavailable)
//
// For unsupported test shapes (custom runners, dynamically-generated tests,
// non-standard helpers), the parser emits unreachable_annotation_unknown
// (severity always warning, never error) rather than a false-positive
// unreachable_annotation diagnostic.
//
// Strictness mode routing (per C-10):
//   - "annotation":     unreachable_annotation diagnostics suppressed
//   - "threshold":      unreachable_annotation severity = warning (exits 0)
//   - "zero-tolerance": unreachable_annotation severity = error (exits non-zero)
//
// unreachable_annotation_unknown is always severity = warning regardless
// of strictness.
//
// @spec spec-check
package checker

import (
	"path/filepath"
	"strings"
)

// CheckUnreachableAnnotations scans testFiles for source-comment @ac
// annotations whose enclosing test produces no runner-visible
// spec-id/AC-NN token, and returns the resulting diagnostics per C-10.
//
// strictness must be one of "annotation", "threshold", or
// "zero-tolerance"; unknown values are treated as "threshold".
func CheckUnreachableAnnotations(testFiles map[string]string, strictness string) []CheckDiagnostic {
	var diags []CheckDiagnostic

	for path, content := range testFiles {
		lang := detectLanguage(path)
		switch lang {
		case "go":
			diags = append(diags, scanGoFile(path, content, strictness)...)
		case "ts", "js":
			diags = append(diags, scanTsFile(path, content, strictness)...)
		case "py":
			// Stub for commit 3c — same reasoning as TS above.
			diags = append(diags, emitUnknownForAllAcs(path, content)...)
		default:
			// Truly unsupported language — emit _unknown for any @ac in the file.
			diags = append(diags, emitUnknownForAllAcs(path, content)...)
		}
	}

	return diags
}

// detectLanguage returns "go" | "ts" | "js" | "py" | "unknown" based
// on the file extension. Test files in other languages currently fall
// through to "unknown" and emit unreachable_annotation_unknown.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "ts"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "js"
	case ".py":
		return "py"
	default:
		return "unknown"
	}
}

// emitUnknownForAllAcs emits unreachable_annotation_unknown for every
// @ac source comment in the file. Used as the fallback for languages
// whose parser is not yet implemented (or whose test shape is
// genuinely unrecognized).
//
// Severity is always warning, never error, per C-10's contract:
// "_unknown is always severity = warning regardless of strictness."
func emitUnknownForAllAcs(path, content string) []CheckDiagnostic {
	var diags []CheckDiagnostic
	for _, ac := range findAcSourceComments(content) {
		diags = append(diags, CheckDiagnostic{
			Kind:     "unreachable_annotation_unknown",
			Severity: "warning",
			Message:  formatUnknownMessage(path, ac.line, ac.id),
		})
	}
	return diags
}

// acComment tracks an @ac source comment's location and id.
type acComment struct {
	line int    // 1-indexed
	id   string // e.g., "AC-01"
}

// specComment tracks an @spec source comment's location and id.
type specComment struct {
	line int    // 1-indexed
	id   string // e.g., "spec-check"
}

// findAcSourceComments returns every "@ac AC-NN" source comment in
// the file, regardless of language. Matches both `// @ac AC-NN`
// (Go/TS/JS style) and `# @ac AC-NN` (Python style). Skips lines
// inside multi-line string literals — reuses the same multiline-state
// machine as scanFileAnnotations in test_annotations.go.
func findAcSourceComments(content string) []acComment {
	var out []acComment
	lines := strings.Split(content, "\n")

	var inBacktick, inTripleDouble, inTripleSingle bool
	for i, line := range lines {
		// Update multi-line string state. Annotations inside string
		// literals are payload (e.g., docs strings), not directives.
		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(line, inBacktick, inTripleDouble, inTripleSingle)
		if inBacktick || inTripleDouble || inTripleSingle {
			continue
		}

		// Match `// @ac AC-NN` or `# @ac AC-NN`.
		acID, ok := extractAcID(line)
		if !ok {
			continue
		}
		out = append(out, acComment{line: i + 1, id: acID})
	}
	return out
}

// extractAcID returns the AC id and true if the line declares
// `// @ac <id>` or `# @ac <id>`. Trims surrounding whitespace.
// Does not validate the id format here — the malformed_ac_id check
// in CheckTestAnnotations covers that.
func extractAcID(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// Strip leading comment markers.
	switch {
	case strings.HasPrefix(trimmed, "// "):
		trimmed = strings.TrimPrefix(trimmed, "// ")
	case strings.HasPrefix(trimmed, "//"):
		trimmed = strings.TrimPrefix(trimmed, "//")
	case strings.HasPrefix(trimmed, "# "):
		trimmed = strings.TrimPrefix(trimmed, "# ")
	case strings.HasPrefix(trimmed, "#"):
		trimmed = strings.TrimPrefix(trimmed, "#")
	default:
		return "", false
	}

	trimmed = strings.TrimSpace(trimmed)
	if !strings.HasPrefix(trimmed, "@ac ") {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(trimmed, "@ac "))
	if id == "" {
		return "", false
	}
	return id, true
}

// extractSpecID returns the spec id and true if the line declares
// `// @spec <id>` or `# @spec <id>`.
func extractSpecID(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	switch {
	case strings.HasPrefix(trimmed, "// "):
		trimmed = strings.TrimPrefix(trimmed, "// ")
	case strings.HasPrefix(trimmed, "//"):
		trimmed = strings.TrimPrefix(trimmed, "//")
	case strings.HasPrefix(trimmed, "# "):
		trimmed = strings.TrimPrefix(trimmed, "# ")
	case strings.HasPrefix(trimmed, "#"):
		trimmed = strings.TrimPrefix(trimmed, "#")
	default:
		return "", false
	}

	trimmed = strings.TrimSpace(trimmed)
	if !strings.HasPrefix(trimmed, "@spec ") {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(trimmed, "@spec "))
	if id == "" {
		return "", false
	}
	return id, true
}

// formatUnreachableMessage builds the user-visible message for an
// unreachable_annotation diagnostic.
func formatUnreachableMessage(path string, line int, acID, specID string) string {
	pair := acID
	if specID != "" {
		pair = specID + "/" + acID
	}
	return path + ":" + itoa(line) + " declares @ac " + acID +
		" but the enclosing test produces no runner-visible " + pair +
		" token (neither in the test title/subtest name nor as a runtime print of \"// @spec\" / \"// @ac\")"
}

// formatUnknownMessage builds the user-visible message for an
// unreachable_annotation_unknown diagnostic.
func formatUnknownMessage(path string, line int, acID string) string {
	return path + ":" + itoa(line) + " declares @ac " + acID +
		" but the test shape is not recognized by any language-aware reachability parser; reachability cannot be determined"
}

// itoa is a small allocation-free Itoa for line numbers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// routeSeverity applies the strictness mode to an
// unreachable_annotation diagnostic, returning (severity, suppress).
// _unknown diagnostics route through their own constant warning rule
// and do not call this helper.
func routeSeverity(strictness string) (severity string, suppress bool) {
	switch strictness {
	case "annotation":
		return "", true
	case "zero-tolerance":
		return "error", false
	case "threshold":
		return "warning", false
	default:
		// Unknown strictness value falls back to threshold per C-10.
		return "warning", false
	}
}

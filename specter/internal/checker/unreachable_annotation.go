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

// CheckUnreachableAnnotations scans testFiles for source-comment @ac
// annotations whose enclosing test produces no runner-visible
// spec-id/AC-NN token, and returns the resulting diagnostics per C-10.
//
// strictness must be one of "annotation", "threshold", or
// "zero-tolerance"; unknown values are treated as "threshold".
//
// This is a stub. The real implementation lands in the next commit on
// release/v0.13.0 to make the tests in unreachable_annotation_test.go
// pass. Returning nil here is intentional: it lets the test file
// compile while keeping every AC-13..AC-18 assertion red, which is
// the contract the SDD cycle expects between commit 2 (test) and
// commit 3 (implementation).
func CheckUnreachableAnnotations(testFiles map[string]string, strictness string) []CheckDiagnostic {
	return nil
}

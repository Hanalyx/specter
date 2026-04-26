// Pure-function tests for test-annotation cross-reference (C-09).
//
// @spec spec-check
package checker

import (
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

// makeSpecWithACs builds a minimal SpecAST with the given id and AC ids.
// Distinct from the existing makeSpec(id, tier) in check_test.go.
func makeSpecWithACs(id string, acIDs ...string) schema.SpecAST {
	s := schema.SpecAST{ID: id}
	for _, ac := range acIDs {
		s.AcceptanceCriteria = append(s.AcceptanceCriteria, schema.AcceptanceCriterion{ID: ac})
	}
	return s
}

// @ac AC-09
func TestCheckTestAnnotations_UnknownSpecRef(t *testing.T) {
	t.Run("spec-check/AC-09 unknown spec reference in test emits diagnostic", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01")}
		testFiles := map[string]string{
			"foo_test.go": "// @spec bogus-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		if len(diags) == 0 {
			t.Fatal("expected at least one diagnostic, got none")
		}
		var found bool
		for _, d := range diags {
			if d.Kind == "unknown_spec_ref" {
				found = true
				if !strings.Contains(d.Message, "bogus-spec") {
					t.Errorf("expected bogus-spec in message, got: %s", d.Message)
				}
				if !strings.Contains(d.Message, "foo_test.go") {
					t.Errorf("expected file path in message, got: %s", d.Message)
				}
			}
		}
		if !found {
			t.Errorf("expected unknown_spec_ref diagnostic, got: %+v", diags)
		}
	})
}

// @ac AC-10
func TestCheckTestAnnotations_UnknownAcRef(t *testing.T) {
	t.Run("spec-check/AC-10 unknown AC reference in test emits diagnostic", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01")}
		testFiles := map[string]string{
			"foo_test.go": "// @spec real-spec\n// @ac AC-99\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		if len(diags) == 0 {
			t.Fatal("expected at least one diagnostic, got none")
		}
		var found bool
		for _, d := range diags {
			if d.Kind == "unknown_ac_ref" {
				found = true
				if !strings.Contains(d.Message, "AC-99") {
					t.Errorf("expected AC-99 in message, got: %s", d.Message)
				}
				if !strings.Contains(d.Message, "real-spec") {
					t.Errorf("expected real-spec in message, got: %s", d.Message)
				}
			}
		}
		if !found {
			t.Errorf("expected unknown_ac_ref diagnostic, got: %+v", diags)
		}
	})
}

// @ac AC-11
func TestCheckTestAnnotations_MalformedAcId(t *testing.T) {
	t.Run("spec-check/AC-11 malformed AC id emits diagnostic for each occurrence", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01")}
		testFiles := map[string]string{
			"foo_test.go": "// @spec real-spec\n// @ac AC-1\n// @ac ac-01\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		var malformed int
		for _, d := range diags {
			if d.Kind == "malformed_ac_id" {
				malformed++
			}
		}
		if malformed != 2 {
			t.Errorf("expected exactly 2 malformed_ac_id diagnostics (AC-1 and ac-01), got %d; all diags: %+v", malformed, diags)
		}
	})
}

// GH #95 — multi-@spec test files: an @ac is valid if it exists in ANY
// declared @spec, not just the most recently declared one. Cross-cutting
// tests legitimately bridge two specs.
func TestCheckTestAnnotations_MultipleSpecs_UnionValidation(t *testing.T) {
	t.Run("spec-check/multi-@spec validates against union of declared specs", func(t *testing.T) {
		specs := []schema.SpecAST{
			makeSpecWithACs("spec-foo", "AC-01", "AC-02"),
			makeSpecWithACs("spec-bar", "AC-10", "AC-11"),
		}
		// Test file declares both specs at the top, then references ACs from each.
		// AC-01 is in spec-foo; AC-10 is in spec-bar. Both should validate.
		testFiles := map[string]string{
			"foo_test.go": "// @spec spec-foo\n// @spec spec-bar\n// @ac AC-01\n// @ac AC-10\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		if len(diags) != 0 {
			t.Errorf("expected zero diagnostics for valid multi-spec references, got %d: %+v", len(diags), diags)
		}
	})

	t.Run("spec-check/multi-@spec emits unknown_ac_ref only when AC is in no declared spec", func(t *testing.T) {
		specs := []schema.SpecAST{
			makeSpecWithACs("spec-foo", "AC-01"),
			makeSpecWithACs("spec-bar", "AC-10"),
		}
		// AC-99 doesn't exist in either declared spec — should flag.
		// AC-01 is in spec-foo (one of the declared specs) — should not flag.
		testFiles := map[string]string{
			"foo_test.go": "// @spec spec-foo\n// @spec spec-bar\n// @ac AC-01\n// @ac AC-99\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		// Exactly one unknown_ac_ref for AC-99.
		var ac99Diag int
		for _, d := range diags {
			if d.Kind == "unknown_ac_ref" && strings.Contains(d.Message, "AC-99") {
				ac99Diag++
			}
		}
		if ac99Diag != 1 {
			t.Errorf("expected exactly 1 unknown_ac_ref for AC-99, got %d; all diags: %+v", ac99Diag, diags)
		}
		// No false positive for AC-01.
		for _, d := range diags {
			if d.Kind == "unknown_ac_ref" && strings.Contains(d.Message, "AC-01") {
				t.Errorf("false positive: AC-01 exists in spec-foo (declared in same file), should not flag. Diag: %+v", d)
			}
		}
	})

	t.Run("spec-check/multi-@spec — one declared spec unknown does not suppress checks against the other", func(t *testing.T) {
		specs := []schema.SpecAST{
			makeSpecWithACs("real-spec", "AC-01"),
		}
		// File declares both real-spec and bogus-spec; bogus-spec is unknown
		// but real-spec exists. AC-01 should still validate (it's in real-spec).
		testFiles := map[string]string{
			"foo_test.go": "// @spec real-spec\n// @spec bogus-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		// Expect one unknown_spec_ref for bogus-spec.
		var unknownSpec int
		var unknownAc int
		for _, d := range diags {
			switch d.Kind {
			case "unknown_spec_ref":
				unknownSpec++
			case "unknown_ac_ref":
				unknownAc++
			}
		}
		if unknownSpec != 1 {
			t.Errorf("expected 1 unknown_spec_ref (for bogus-spec), got %d", unknownSpec)
		}
		if unknownAc != 0 {
			t.Errorf("expected 0 unknown_ac_ref (AC-01 exists in real-spec), got %d; all diags: %+v", unknownAc, diags)
		}
	})
}

// Regression guard: annotations inside backtick template strings (TS/JS) and
// triple-quoted Python strings are payload, not real annotations — must NOT
// flag. Common pattern: tests for an annotation parser embed example text.
func TestCheckTestAnnotations_StringLiterals_NotFlagged(t *testing.T) {
	t.Run("spec-check/string-literal annotations skipped not flagged", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01")}
		testFiles := map[string]string{
			"ts_test.ts": "const fixture = `\n// @spec bogus-spec\n// @ac AC-99\n`;\n",
			"py_test.py": "EXAMPLE = \"\"\"\n# @spec bogus-spec\n# @ac AC-99\n\"\"\"\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		if len(diags) != 0 {
			t.Errorf("expected zero diagnostics for annotations inside string literals, got %d: %+v", len(diags), diags)
		}
	})
}

// AC-1A and similar suffixed forms must flag as malformed_ac_id, not be
// silently skipped. Earlier loose regex `\d+\b` missed digit-then-letter.
func TestCheckTestAnnotations_MalformedAcId_SuffixedForm(t *testing.T) {
	t.Run("spec-check/AC-11 suffixed AC id (AC-1A) flags as malformed", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01")}
		testFiles := map[string]string{
			"foo_test.go": "// @spec real-spec\n// @ac AC-1A\nfunc TestFoo(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		var found bool
		for _, d := range diags {
			if d.Kind == "malformed_ac_id" && strings.Contains(d.Message, "AC-1A") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected malformed_ac_id for AC-1A, got: %+v", diags)
		}
	})
}

// Regression guard: all-valid file produces zero diagnostics. Keeps the new
// check from introducing false positives on the existing codebase.
func TestCheckTestAnnotations_ValidReferences_NoFalsePositives(t *testing.T) {
	t.Run("spec-check/valid references produce zero diagnostics", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpecWithACs("real-spec", "AC-01", "AC-02")}
		testFiles := map[string]string{
			"foo_test.go": "// @spec real-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n",
			"bar_test.go": "// @spec real-spec\n// @ac AC-02\nfunc TestBar(t *testing.T) {}\n",
		}
		diags := CheckTestAnnotations(testFiles, specs)

		if len(diags) != 0 {
			t.Errorf("expected zero diagnostics for valid references, got %d: %+v", len(diags), diags)
		}
	})
}

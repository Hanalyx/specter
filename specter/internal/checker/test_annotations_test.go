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

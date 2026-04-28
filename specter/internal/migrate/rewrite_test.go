// @spec spec-doctor
package migrate

import (
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

// @ac AC-12
// Given a spec YAML with `trust_level: high` under `spec:` and a parse error
// signature naming trust_level, Apply returns a new YAML that omits the
// trust_level line and an applied list containing "strip-trust-level".
func TestApply_StripsTrustLevel(t *testing.T) {
	t.Run("spec-doctor/AC-12 strip trust_level rewrite removes the field", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  version: "1.0.0"
  status: draft
  tier: 3
  trust_level: high
  context:
    system: test
    feature: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "MUST something"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      priority: high
`
		parseErrors := []coverage.ParseErrorEntry{
			{
				File:    "legacy.spec.yaml",
				Path:    "spec",
				Type:    "additionalProperties",
				Message: "Unknown field 'trust_level'. Remove it or check for a typo in the field name.",
			},
		}

		result, err := Apply([]byte(yamlIn), parseErrors)
		if err != nil {
			t.Fatalf("Apply returned error: %v", err)
		}
		if len(result.Applied) != 1 || result.Applied[0] != "strip-trust-level" {
			t.Errorf("Applied = %v, want [strip-trust-level]", result.Applied)
		}
		if strings.Contains(string(result.Content), "trust_level") {
			t.Errorf("trust_level should be stripped; got:\n%s", result.Content)
		}
		// Other fields must survive (byte-preserving outside the rewrite).
		for _, mustHave := range []string{"id: legacy-spec", "tier: 3", "AC-01"} {
			if !strings.Contains(string(result.Content), mustHave) {
				t.Errorf("rewrite dropped %q unexpectedly; got:\n%s", mustHave, result.Content)
			}
		}
	})
}

// @ac AC-15
// No parse errors → Apply returns zero rewrites, content byte-identical.
// This is the "no changes" path that drives the doctor --fix summary.
func TestApply_NoErrors_ReturnsEmpty(t *testing.T) {
	t.Run("spec-doctor/AC-15 no parse errors returns empty applied list and unchanged content", func(t *testing.T) {
		yamlIn := `spec:
  id: clean
  version: "1.0.0"
`
		result, err := Apply([]byte(yamlIn), nil)
		if err != nil {
			t.Fatalf("Apply error: %v", err)
		}
		if len(result.Applied) != 0 {
			t.Errorf("expected no rewrites applied, got %v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-identical when nothing to do")
		}
	})
}

// @ac AC-15
// Parse errors that don't match any known-rewrite pattern are ignored:
// zero rewrites, content unchanged. Drives the doctor --fix "no changes"
// path when drift exists but isn't in the rewrite table yet.
func TestApply_UnknownErrorPattern_NoOp(t *testing.T) {
	t.Run("spec-doctor/AC-15 unknown error pattern leaves content untouched", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
`
		parseErrors := []coverage.ParseErrorEntry{
			{
				File:    "legacy.spec.yaml",
				Path:    "spec.some_other_field",
				Type:    "some-unknown-error-type",
				Message: "Totally unrecognized error",
			},
		}
		result, _ := Apply([]byte(yamlIn), parseErrors)
		if len(result.Applied) != 0 {
			t.Errorf("unknown pattern must not apply any rewrite, got %v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be unchanged for unknown pattern")
		}
	})
}

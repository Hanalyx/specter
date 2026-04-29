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

// trustLevelErr is the parse-error signature `--fix` matches for the
// strip-trust-level rewrite. Reused across the unsafe-shape tests below.
var trustLevelErr = []coverage.ParseErrorEntry{{
	File:    "legacy.spec.yaml",
	Path:    "spec",
	Type:    "additionalProperties",
	Message: "Unknown field 'trust_level'. Remove it or check for a typo in the field name.",
}}

// @ac AC-19
// trust_level using a literal-style block scalar (`|`) MUST be refused.
// File byte-unchanged; Result.Unhandled names the file with reason
// containing "block scalar".
func TestApply_BlockScalarLiteral_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-19 block scalar literal trust_level refused with byte-unchanged content", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: |
    high
    confidence
  context:
    system: test
`
		result, err := Apply([]byte(yamlIn), trustLevelErr)
		if err != nil {
			t.Fatalf("Apply error: %v", err)
		}
		if len(result.Applied) != 0 {
			t.Errorf("block scalar must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged on refusal; got:\n%s", result.Content)
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d: %+v", len(result.Unhandled), result.Unhandled)
		}
		if !strings.Contains(strings.ToLower(result.Unhandled[0].Reason), "block scalar") {
			t.Errorf("expected reason naming `block scalar`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-19
// Folded-style block scalar (`>`) gets the same refusal.
func TestApply_BlockScalarFolded_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-19 block scalar folded trust_level refused", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: >
    high confidence
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("folded block scalar must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged on refusal")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
	})
}

// @ac AC-20
// trust_level with sequence value (next-line `- entries`) MUST be refused.
// Reason names "not a scalar".
func TestApply_Sequence_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-20 sequence trust_level value refused", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level:
    - high
    - medium
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("sequence value must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged on refusal")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
		if !strings.Contains(strings.ToLower(result.Unhandled[0].Reason), "not a scalar") {
			t.Errorf("expected reason naming `not a scalar`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-20
// trust_level with mapping value (next-line nested key:value) MUST be
// refused with the same "not a scalar" reason.
func TestApply_Mapping_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-20 mapping trust_level value refused", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level:
    level: high
    rationale: tested
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("mapping value must not be rewritten")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
	})
}

// @ac AC-20
// Regression guard: plain scalar values continue to rewrite as before.
// Refusal is narrowed to non-scalar shapes only.
func TestApply_PlainScalar_TrustLevel_StillRewrites(t *testing.T) {
	t.Run("spec-doctor/AC-20 plain scalar trust_level still rewrites (regression guard)", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: high
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 1 || result.Applied[0] != "strip-trust-level" {
			t.Errorf("plain scalar must rewrite cleanly; got Applied=%v", result.Applied)
		}
		if len(result.Unhandled) != 0 {
			t.Errorf("plain scalar must not produce Unhandled diagnostic; got %+v", result.Unhandled)
		}
		if strings.Contains(string(result.Content), "trust_level") {
			t.Errorf("trust_level should be stripped from plain scalar form")
		}
	})
}

// @ac AC-20
// Numeric and quoted plain scalars also rewrite — these are still
// `yaml.ScalarNode` with non-block style.
func TestApply_QuotedAndNumeric_TrustLevel_StillRewrite(t *testing.T) {
	t.Run("spec-doctor/AC-20 quoted and numeric trust_level scalars rewrite", func(t *testing.T) {
		cases := []struct {
			name string
			in   string
		}{
			{"double-quoted", "spec:\n  id: x\n  trust_level: \"high\"\n  context:\n    system: t\n"},
			{"single-quoted", "spec:\n  id: x\n  trust_level: 'high'\n  context:\n    system: t\n"},
			{"numeric", "spec:\n  id: x\n  trust_level: 0.5\n  context:\n    system: t\n"},
		}
		for _, tc := range cases {
			result, _ := Apply([]byte(tc.in), trustLevelErr)
			if len(result.Applied) != 1 {
				t.Errorf("%s: expected 1 Applied, got %v", tc.name, result.Applied)
			}
			if len(result.Unhandled) != 0 {
				t.Errorf("%s: expected 0 Unhandled, got %+v", tc.name, result.Unhandled)
			}
		}
	})
}

// @ac AC-21
// Plain scalar value with folded continuation (no `|`/`>` indicator).
// yaml.v3 reports Kind=ScalarNode, Style=PlainStyle — passes the v1.5.0
// kind/style check. The line-span branch must catch it: a non-blank line
// at greater indent than the key follows the value's first line.
func TestApply_PlainFoldedContinuation_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-21 plain scalar with folded continuation refused via line-span check", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: high
    confidence
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("folded plain scalar must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged on refusal; got:\n%s", result.Content)
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d: %+v", len(result.Unhandled), result.Unhandled)
		}
		if !strings.Contains(strings.ToLower(result.Unhandled[0].Reason), "spans multiple lines") {
			t.Errorf("expected reason naming `spans multiple lines`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-22
// Quoted scalar value spanning multiple source lines. Style is
// DoubleQuotedStyle / SingleQuotedStyle, not Literal/Folded — passes
// the v1.5.0 style check. Line-span branch must catch it.
func TestApply_DoubleQuotedMultiline_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-22 double-quoted multi-line trust_level refused", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: "line1
    line2"
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("double-quoted multi-line must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged on refusal")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
		if !strings.Contains(strings.ToLower(result.Unhandled[0].Reason), "spans multiple lines") {
			t.Errorf("expected reason naming `spans multiple lines`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-22
// Single-quoted multi-line gets the same refusal.
func TestApply_SingleQuotedMultiline_TrustLevel_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-22 single-quoted multi-line trust_level refused", func(t *testing.T) {
		yamlIn := `spec:
  id: legacy-spec
  trust_level: 'line1
    line2'
  context:
    system: test
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("single-quoted multi-line must not be rewritten")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
		if !strings.Contains(strings.ToLower(result.Unhandled[0].Reason), "spans multiple lines") {
			t.Errorf("expected reason naming `spans multiple lines`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-22
// Regression guard: single-line quoted values continue to rewrite.
// Already covered by TestApply_QuotedAndNumeric_TrustLevel_StillRewrite
// above; this run re-verifies after the line-span check is added.
func TestApply_SingleLineQuoted_TrustLevel_StillRewrites(t *testing.T) {
	t.Run("spec-doctor/AC-22 single-line quoted trust_level still rewrites after line-span check", func(t *testing.T) {
		yamlIn := `spec:
  id: x
  trust_level: "high"
  context:
    system: t
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 1 {
			t.Errorf("single-line quoted must still rewrite; got Applied=%v", result.Applied)
		}
		if len(result.Unhandled) != 0 {
			t.Errorf("single-line quoted must not produce Unhandled; got %+v", result.Unhandled)
		}
	})
}

// @ac AC-23
// Multi-document YAML file: doc 0 has a safe trust_level, doc 1 has an
// unsafe shape. The whole file must be refused — partial rewrite is not
// safe across documents.
func TestApply_MultiDoc_UnsafeInLaterDoc_Refused(t *testing.T) {
	t.Run("spec-doctor/AC-23 multi-doc with unsafe shape in non-first doc refused", func(t *testing.T) {
		yamlIn := `spec:
  id: doc0
  trust_level: high
  context:
    system: t
---
spec:
  id: doc1
  trust_level: |
    high
    confidence
  context:
    system: t
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 0 {
			t.Errorf("multi-doc with unsafe shape in doc 1 must not be rewritten; got Applied=%v", result.Applied)
		}
		if string(result.Content) != yamlIn {
			t.Errorf("content must be byte-unchanged when any doc is unsafe")
		}
		if len(result.Unhandled) != 1 {
			t.Fatalf("expected 1 Unhandled entry, got %d", len(result.Unhandled))
		}
		// Reason should reference the unsafe shape in doc 1 (block scalar
		// or spans-multiple-lines — either is acceptable per AC-23).
		reason := strings.ToLower(result.Unhandled[0].Reason)
		if !strings.Contains(reason, "block scalar") && !strings.Contains(reason, "spans multiple lines") {
			t.Errorf("expected reason naming `block scalar` or `spans multiple lines`, got: %q", result.Unhandled[0].Reason)
		}
	})
}

// @ac AC-23
// Regression guard: multi-doc where ALL docs are safe must continue to
// rewrite (each doc's trust_level: line is deleted).
func TestApply_MultiDoc_AllSafe_Rewrites(t *testing.T) {
	t.Run("spec-doctor/AC-23 multi-doc all safe still rewrites", func(t *testing.T) {
		yamlIn := `spec:
  id: doc0
  trust_level: high
  context:
    system: t
---
spec:
  id: doc1
  trust_level: medium
  context:
    system: t
`
		result, _ := Apply([]byte(yamlIn), trustLevelErr)
		if len(result.Applied) != 1 {
			t.Errorf("multi-doc all-safe must rewrite; got Applied=%v", result.Applied)
		}
		if strings.Contains(string(result.Content), "trust_level") {
			t.Errorf("trust_level should be stripped from both docs")
		}
	})
}

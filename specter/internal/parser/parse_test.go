// @spec spec-parse
package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", relPath))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", relPath, err)
	}
	return string(data)
}

// @ac AC-01
func TestParseValidSpec(t *testing.T) {
	t.Run("spec-parse/AC-01 parse valid spec", func(t *testing.T) {
		yaml := readFixture(t, "valid/simple.spec.yaml")
		result := ParseSpec(yaml)

		if !result.OK {
			t.Fatalf("expected OK, got errors: %v", result.Errors)
		}
		if result.Value.ID != "test-simple" {
			t.Errorf("expected id 'test-simple', got %q", result.Value.ID)
		}
		if result.Value.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", result.Value.Version)
		}
		if result.Value.Status != "approved" {
			t.Errorf("expected status 'approved', got %q", result.Value.Status)
		}
		if result.Value.Tier != 2 {
			t.Errorf("expected tier 2, got %d", result.Value.Tier)
		}
		if result.Value.Context.System != "Test system" {
			t.Errorf("expected system 'Test system', got %q", result.Value.Context.System)
		}
		if len(result.Value.Constraints) != 1 {
			t.Errorf("expected 1 constraint, got %d", len(result.Value.Constraints))
		}
		if len(result.Value.AcceptanceCriteria) != 1 {
			t.Errorf("expected 1 AC, got %d", len(result.Value.AcceptanceCriteria))
		}
	})
}

// @ac AC-02
func TestParseMissingID(t *testing.T) {
	t.Run("spec-parse/AC-02 parse missing id", func(t *testing.T) {
		yaml := readFixture(t, "invalid/missing-id.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "required" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'required' error type, got: %v", result.Errors)
		}
	})
}

// @ac AC-03
func TestParseExtraField(t *testing.T) {
	t.Run("spec-parse/AC-03 parse extra field", func(t *testing.T) {
		yaml := readFixture(t, "invalid/extra-field.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "additionalProperties" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'additionalProperties' error, got: %v", result.Errors)
		}
	})
}

// @ac AC-04
func TestParseBadYAML(t *testing.T) {
	t.Run("spec-parse/AC-04 parse bad yaml", func(t *testing.T) {
		yaml := readFixture(t, "invalid/bad-yaml.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		if len(result.Errors) == 0 {
			t.Fatal("expected errors")
		}
	})
}

// @ac AC-05
func TestParseBadVersion(t *testing.T) {
	t.Run("spec-parse/AC-05 parse bad version", func(t *testing.T) {
		yaml := readFixture(t, "invalid/bad-version.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "pattern" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'pattern' error, got: %v", result.Errors)
		}
	})
}

// @ac AC-06
func TestParseMinimalSpec(t *testing.T) {
	t.Run("spec-parse/AC-06 parse minimal spec", func(t *testing.T) {
		yaml := readFixture(t, "valid/minimal.spec.yaml")
		result := ParseSpec(yaml)

		if !result.OK {
			t.Fatalf("expected OK, got errors: %v", result.Errors)
		}
		if result.Value.ID != "test-minimal" {
			t.Errorf("expected id 'test-minimal', got %q", result.Value.ID)
		}
		if result.Value.DependsOn != nil {
			t.Error("expected nil depends_on")
		}
		if result.Value.Tags != nil {
			t.Error("expected nil tags")
		}
	})
}

// @ac AC-07
func TestParseWithAnchors(t *testing.T) {
	t.Run("spec-parse/AC-07 parse with anchors", func(t *testing.T) {
		yaml := readFixture(t, "valid/with-anchors.spec.yaml")
		result := ParseSpec(yaml)

		if !result.OK {
			t.Fatalf("expected OK, got errors: %v", result.Errors)
		}
		if len(result.Value.Constraints) < 2 {
			t.Fatalf("expected at least 2 constraints, got %d", len(result.Value.Constraints))
		}
		if result.Value.Constraints[0].Type != "technical" {
			t.Errorf("expected constraint type 'technical', got %q", result.Value.Constraints[0].Type)
		}
		if result.Value.Constraints[1].Type != "technical" {
			t.Errorf("expected constraint type 'technical', got %q", result.Value.Constraints[1].Type)
		}
	})
}

// @ac AC-08
func TestParseMultipleErrors(t *testing.T) {
	t.Run("spec-parse/AC-08 parse multiple errors", func(t *testing.T) {
		yaml := readFixture(t, "invalid/multiple-errors.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		if len(result.Errors) < 2 {
			t.Errorf("expected at least 2 errors, got %d: %v", len(result.Errors), result.Errors)
		}
	})
}

// @ac AC-09
func TestParseBadConstraintID(t *testing.T) {
	t.Run("spec-parse/AC-09 parse bad constraint id", func(t *testing.T) {
		yaml := readFixture(t, "invalid/bad-constraint-id.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "pattern" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'pattern' error for constraint ID, got: %v", result.Errors)
		}
	})
}

// @ac AC-10
func TestParseBadACID(t *testing.T) {
	t.Run("spec-parse/AC-10 parse bad ac id", func(t *testing.T) {
		yaml := readFixture(t, "invalid/bad-ac-id.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "pattern" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'pattern' error for AC ID, got: %v", result.Errors)
		}
	})
}

// @ac AC-11
func TestParseHumanReadable_ConstraintID(t *testing.T) {
	t.Run("spec-parse/AC-11 parse human readable constraint id", func(t *testing.T) {
		yaml := readFixture(t, "invalid/bad-constraint-id.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		for _, e := range result.Errors {
			if e.Type == "pattern" {
				if !strings.Contains(e.Message, "C-NN") {
					t.Errorf("expected message to mention C-NN pattern, got: %q", e.Message)
				}
				return
			}
		}
		t.Errorf("no pattern error found: %v", result.Errors)
	})
}

// @ac AC-12
func TestParseHumanReadable_MissingID(t *testing.T) {
	t.Run("spec-parse/AC-12 parse human readable missing id", func(t *testing.T) {
		yaml := readFixture(t, "invalid/missing-id.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		for _, e := range result.Errors {
			if e.Type == "required" {
				if !strings.Contains(e.Message, "kebab-case") {
					t.Errorf("expected message to mention kebab-case, got: %q", e.Message)
				}
				return
			}
		}
		t.Errorf("no required error found: %v", result.Errors)
	})
}

// @ac AC-13
func TestParseHumanReadable_ExtraField(t *testing.T) {
	t.Run("spec-parse/AC-13 parse human readable extra field", func(t *testing.T) {
		yaml := readFixture(t, "invalid/extra-field.spec.yaml")
		result := ParseSpec(yaml)

		if result.OK {
			t.Fatal("expected failure, got OK")
		}
		for _, e := range result.Errors {
			if e.Type == "additionalProperties" {
				if strings.Contains(e.Message, "additionalProperties") {
					t.Errorf("message should not expose raw 'additionalProperties', got: %q", e.Message)
				}
				return
			}
		}
		t.Errorf("no additionalProperties error found: %v", result.Errors)
	})
}

func TestParsePureFunction(t *testing.T) {
	yaml := readFixture(t, "valid/simple.spec.yaml")
	r1 := ParseSpec(yaml)
	r2 := ParseSpec(yaml)

	if r1.OK != r2.OK {
		t.Error("pure function violation: different OK values")
	}
	if r1.Value.ID != r2.Value.ID {
		t.Error("pure function violation: different IDs")
	}
}

// @ac AC-14 (v0.7.0 — context.additionalProperties tightened to false)
func TestParse_UnknownContextField_Rejected(t *testing.T) {
	t.Run("spec-parse/AC-14 parse unknown context field rejected", func(t *testing.T) {
		yaml := `spec:
  id: test-unknown-context
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: test
    role: "this field is not in the schema"
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
`
		result := ParseSpec(yaml)
		if result.OK {
			t.Fatal("expected parse to fail on unknown context field")
		}
		found := false
		for _, e := range result.Errors {
			if strings.Contains(e.Message, "role") || strings.Contains(e.Path, "context") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error mentioning 'role' or 'context', got: %v", result.Errors)
		}
	})
}

// @ac AC-15 (v0.7.0 — AC metadata fields)
func TestParse_ACNotesAndApprovalFields(t *testing.T) {
	t.Run("spec-parse/AC-15 parse ac notes and approval fields", func(t *testing.T) {
		yaml := `spec:
  id: test-ac-metadata
  version: "1.0.0"
  status: approved
  tier: 1
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test constraint"
  acceptance_criteria:
    - id: AC-01
      description: "test AC"
      references_constraints: ["C-01"]
      notes: "Financial op — see also AC-03."
      approval_gate: true
      approval_date: "2026-04-17"
`
		result := ParseSpec(yaml)
		if !result.OK {
			t.Fatalf("expected OK, got errors: %v", result.Errors)
		}
		ac := result.Value.AcceptanceCriteria[0]
		if ac.Notes != "Financial op — see also AC-03." {
			t.Errorf("Notes not preserved, got %q", ac.Notes)
		}
		if !ac.ApprovalGate {
			t.Error("ApprovalGate should be true")
		}
		if ac.ApprovalDate != "2026-04-17" {
			t.Errorf("ApprovalDate mismatch, got %q", ac.ApprovalDate)
		}
	})
}

// @ac AC-15
func TestParse_ACApprovalDateInvalidFormat_Rejected(t *testing.T) {
	t.Run("spec-parse/AC-15 parse ac approval date invalid format rejected", func(t *testing.T) {
		yaml := `spec:
  id: test-bad-date
  version: "1.0.0"
  status: approved
  tier: 1
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      approval_gate: true
      approval_date: "not-a-date"
`
		result := ParseSpec(yaml)
		if result.OK {
			t.Fatal("expected parse to fail on invalid approval_date format")
		}
	})
}

// @ac AC-16 (v0.7.0 — parse-time cross-reference validation)
func TestParse_DanglingConstraintReference_Rejected(t *testing.T) {
	t.Run("spec-parse/AC-16 parse dangling constraint reference rejected", func(t *testing.T) {
		yaml := `spec:
  id: test-dangling
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "only declared constraint"
  acceptance_criteria:
    - id: AC-01
      description: "references something real"
      references_constraints: ["C-01"]
    - id: AC-02
      description: "references something fake"
      references_constraints: ["C-99"]
`
		result := ParseSpec(yaml)
		if result.OK {
			t.Fatal("expected parse to fail on dangling reference")
		}
		found := false
		for _, e := range result.Errors {
			if e.Type == "dangling_reference" && strings.Contains(e.Message, "C-99") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected dangling_reference error mentioning C-99, got: %v", result.Errors)
		}
	})
}

// @ac AC-17 (v0.7.0 — optional title on spec)
func TestParse_SpecTitleOptional(t *testing.T) {
	t.Run("spec-parse/AC-17 parse spec title optional", func(t *testing.T) {
		yamlWith := `spec:
  id: test-title
  title: "My Custom Title"
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
`
		yamlWithout := `spec:
  id: test-no-title
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
`
		r1 := ParseSpec(yamlWith)
		if !r1.OK {
			t.Fatalf("spec with title should parse, got: %v", r1.Errors)
		}
		if r1.Value.Title != "My Custom Title" {
			t.Errorf("expected title preserved, got %q", r1.Value.Title)
		}
		r2 := ParseSpec(yamlWithout)
		if !r2.OK {
			t.Fatalf("spec without title should parse (field optional), got: %v", r2.Errors)
		}
		if r2.Value.Title != "" {
			t.Errorf("expected empty title when absent, got %q", r2.Value.Title)
		}
	})
}

// @ac AC-20
func TestParseReportsEveryUnknownFieldSorted(t *testing.T) {
	t.Run("spec-parse/AC-20 one error per unknown field, sorted and stable", func(t *testing.T) {
		yaml := readFixture(t, "invalid/multiple-unknown-fields.spec.yaml")
		first := ParseSpec(yaml)
		if first.OK {
			t.Fatal("expected parse to fail on three unknown fields")
		}

		var fields []string
		for _, e := range first.Errors {
			if e.Type != "additionalProperties" {
				t.Fatalf("fixture must fail only on unknown fields, got %s error: %s", e.Type, e.Message)
			}
			if e.Path != "spec" {
				t.Errorf("expected path %q, got %q", "spec", e.Path)
			}
			fields = append(fields, e.Message)
		}

		want := []string{
			"Unknown field 'aa_trust_level_note'. Remove it or check for a typo in the field name.",
			"Unknown field 'mm_reviewer'. Remove it or check for a typo in the field name.",
			"Unknown field 'zz_legacy_owner'. Remove it or check for a typo in the field name.",
		}
		if len(fields) != len(want) {
			t.Fatalf("expected %d additionalProperties errors, got %d: %q", len(want), len(fields), fields)
		}
		for i := range want {
			if fields[i] != want[i] {
				t.Errorf("error %d: expected %q, got %q", i, want[i], fields[i])
			}
		}

		for i := 0; i < 50; i++ {
			again := ParseSpec(yaml)
			if len(again.Errors) != len(first.Errors) {
				t.Fatalf("parse %d: expected %d errors, got %d", i+2, len(first.Errors), len(again.Errors))
			}
			for j := range first.Errors {
				if again.Errors[j] != first.Errors[j] {
					t.Fatalf("parse %d, error %d: expected %+v, got %+v", i+2, j, first.Errors[j], again.Errors[j])
				}
			}
		}
	})
}

// @ac AC-21
func TestParseOrdersErrorsAcrossMappings(t *testing.T) {
	t.Run("spec-parse/AC-21 errors from several mappings come out in one stable order", func(t *testing.T) {
		yaml := readFixture(t, "invalid/unknown-fields-several-mappings.spec.yaml")
		first := ParseSpec(yaml)
		if first.OK {
			t.Fatal("expected parse to fail on unknown fields in three mappings")
		}

		type want struct{ path, field string }
		wants := []want{
			{"spec", "'aa_root'"},
			{"spec", "'zz_root'"},
			{"spec.context", "'context.aa_ctx'"},
			{"spec.context", "'context.zz_ctx'"},
			{"spec.objective", "'aa_obj'"},
			{"spec.objective", "'zz_obj'"},
		}
		if len(first.Errors) != len(wants) {
			t.Fatalf("expected %d errors, got %d: %+v", len(wants), len(first.Errors), first.Errors)
		}
		for i, w := range wants {
			e := first.Errors[i]
			if e.Type != "additionalProperties" || e.Path != w.path || !strings.Contains(e.Message, "Unknown field "+w.field) {
				t.Errorf("error %d: want %s at %q, got [%s] %q: %s", i, w.field, w.path, e.Type, e.Path, e.Message)
			}
		}

		for i := 0; i < 50; i++ {
			again := ParseSpec(yaml)
			if len(again.Errors) != len(first.Errors) {
				t.Fatalf("parse %d: expected %d errors, got %d", i+2, len(first.Errors), len(again.Errors))
			}
			for j := range first.Errors {
				if again.Errors[j] != first.Errors[j] {
					t.Fatalf("parse %d, error %d: expected %+v, got %+v", i+2, j, first.Errors[j], again.Errors[j])
				}
			}
		}
	})
}

// @ac AC-20
func TestParseNamesAnUnknownFieldWithAQuoteInFull(t *testing.T) {
	t.Run("spec-parse/AC-20 a field name with an apostrophe is printed as written", func(t *testing.T) {
		yaml := `spec:
  id: test-quote-field
  version: "1.0.0"
  status: draft
  tier: 2
  it's: "x"
  context:
    system: Test system
  objective:
    summary: A spec with an unknown field whose name holds an apostrophe.
  constraints:
    - id: C-01
      description: "MUST name the field as written"
  acceptance_criteria:
    - id: AC-01
      description: "Should name it's in full"
      references_constraints: ["C-01"]
`
		result := ParseSpec(yaml)
		if result.OK || len(result.Errors) != 1 {
			t.Fatalf("expected exactly one error, got OK=%v errors=%+v", result.OK, result.Errors)
		}
		want := "Unknown field 'it's'. Remove it or check for a typo in the field name."
		if result.Errors[0].Message != want {
			t.Errorf("expected %q, got %q", want, result.Errors[0].Message)
		}
	})
}

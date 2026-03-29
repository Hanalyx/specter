// @spec spec-parse
package parser

import (
	"os"
	"path/filepath"
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
}

// @ac AC-02
func TestParseMissingID(t *testing.T) {
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
}

// @ac AC-03
func TestParseExtraField(t *testing.T) {
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
}

// @ac AC-04
func TestParseBadYAML(t *testing.T) {
	yaml := readFixture(t, "invalid/bad-yaml.spec.yaml")
	result := ParseSpec(yaml)

	if result.OK {
		t.Fatal("expected failure, got OK")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors")
	}
}

// @ac AC-05
func TestParseBadVersion(t *testing.T) {
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
}

// @ac AC-06
func TestParseMinimalSpec(t *testing.T) {
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
	if result.Value.TrustLevel != "" {
		t.Error("expected empty trust_level")
	}
	if result.Value.Tags != nil {
		t.Error("expected nil tags")
	}
}

// @ac AC-07
func TestParseWithAnchors(t *testing.T) {
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
}

// @ac AC-08
func TestParseMultipleErrors(t *testing.T) {
	yaml := readFixture(t, "invalid/multiple-errors.spec.yaml")
	result := ParseSpec(yaml)

	if result.OK {
		t.Fatal("expected failure, got OK")
	}
	if len(result.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

// @ac AC-09
func TestParseBadConstraintID(t *testing.T) {
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
}

// @ac AC-10
func TestParseBadACID(t *testing.T) {
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

// @spec spec-reverse
package reverse

import "testing"

// @ac AC-07
func TestDetectGaps_ConstraintWithNoMatchingAssertion(t *testing.T) {
	constraints := []ExtractedConstraint{
		{Field: "email", Rule: "format", Value: "email", SourceFile: "schema.ts", Line: 5},
		{Field: "name", Rule: "required", SourceFile: "schema.ts", Line: 6},
	}
	// Only the name field has a test
	assertions := []ExtractedAssertion{
		{TestName: "test_name_required", Description: "returns 400 when name is missing", IsError: true,
			SourceFile: "schema.test.ts", Line: 10},
	}

	gaps := DetectGaps(constraints, assertions)
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	if !gaps[0].Gap {
		t.Error("expected gap AC to have Gap=true")
	}
	if gaps[0].Priority != "high" {
		t.Errorf("expected priority 'high', got %q", gaps[0].Priority)
	}
	if gaps[0].Description == "" {
		t.Error("expected gap AC to have a description")
	}
}

// @ac AC-07
func TestDetectGaps_AllConstraintsCovered(t *testing.T) {
	constraints := []ExtractedConstraint{
		{Field: "email", Rule: "format", Value: "email"},
	}
	assertions := []ExtractedAssertion{
		{TestName: "test_valid_email", Description: "validates email format", SourceFile: "test.ts"},
	}

	gaps := DetectGaps(constraints, assertions)
	if len(gaps) != 0 {
		t.Fatalf("expected 0 gaps, got %d", len(gaps))
	}
}

// @ac AC-07
func TestDetectGaps_ErrorTestCoversConstraint(t *testing.T) {
	constraints := []ExtractedConstraint{
		{Field: "age", Rule: "min", Value: 18},
	}
	assertions := []ExtractedAssertion{
		{TestName: "test_age_validation", Description: "returns error for invalid age",
			IsError: true, SourceFile: "test.ts"},
	}

	gaps := DetectGaps(constraints, assertions)
	if len(gaps) != 0 {
		t.Fatalf("expected 0 gaps (error test covers age field), got %d", len(gaps))
	}
}

// @ac AC-07
func TestDetectGaps_NoConstraints(t *testing.T) {
	gaps := DetectGaps(nil, nil)
	if len(gaps) != 0 {
		t.Fatalf("expected 0 gaps for nil input, got %d", len(gaps))
	}
}

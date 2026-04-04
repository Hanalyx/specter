// @spec spec-coverage
package coverage

import (
	"testing"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/schema"
)

func makeSpec(id string, tier int, acIDs ...string) schema.SpecAST {
	var acs []schema.AcceptanceCriterion
	for _, aid := range acIDs {
		acs = append(acs, schema.AcceptanceCriterion{ID: aid, Description: "test"})
	}
	return schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: tier,
		Context:            schema.SpecContext{System: "test"},
		Objective:          schema.SpecObjective{Summary: "test"},
		Constraints:        []schema.Constraint{{ID: "C-01", Description: "test"}},
		AcceptanceCriteria: acs,
	}
}

// @ac AC-01
func TestAnnotationExtraction(t *testing.T) {
	content := "// @spec user-auth\n// @ac AC-01\n// @ac AC-02\n"
	matches := ExtractAnnotations(content, "test.ts")

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SpecID != "user-auth" {
		t.Errorf("expected spec_id 'user-auth', got %q", matches[0].SpecID)
	}
	if len(matches[0].ACIDs) != 2 {
		t.Errorf("expected 2 AC IDs, got %d", len(matches[0].ACIDs))
	}
}

// @ac AC-01
func TestCoverageMapping(t *testing.T) {
	specs := []schema.SpecAST{
		makeSpec("user-auth", 2, "AC-01", "AC-02", "AC-03"),
	}
	anns := []AnnotationMatch{
		{File: "test.ts", SpecID: "user-auth", ACIDs: []string{"AC-01", "AC-02"}},
	}

	report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
	e := report.Entries[0]

	if len(e.CoveredACs) != 2 {
		t.Errorf("expected 2 covered ACs, got %d", len(e.CoveredACs))
	}
	if len(e.UncoveredACs) != 1 {
		t.Errorf("expected 1 uncovered AC, got %d", len(e.UncoveredACs))
	}
	if e.CoveragePct < 66.0 || e.CoveragePct > 67.0 {
		t.Errorf("expected ~66.7%%, got %.1f%%", e.CoveragePct)
	}
}

// @ac AC-02
func TestZeroCoverage(t *testing.T) {
	specs := []schema.SpecAST{makeSpec("orphan", 2, "AC-01", "AC-02")}
	report := BuildCoverageReport(specs, nil, checker.CoverageThresholdByTier)

	if report.Entries[0].CoveragePct != 0 {
		t.Errorf("expected 0%%, got %.1f%%", report.Entries[0].CoveragePct)
	}
	if report.Summary.Uncovered != 1 {
		t.Errorf("expected 1 uncovered, got %d", report.Summary.Uncovered)
	}
}

// @ac AC-03
func TestTier1Below100Fails(t *testing.T) {
	specs := []schema.SpecAST{makeSpec("payment", 1, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")}
	anns := []AnnotationMatch{
		{File: "test.ts", SpecID: "payment", ACIDs: []string{"AC-01", "AC-02", "AC-03", "AC-04"}},
	}

	report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
	e := report.Entries[0]

	if e.PassesThreshold {
		t.Error("expected Tier 1 at 80% to fail (threshold 100%)")
	}
	if e.Threshold != 100 {
		t.Errorf("expected threshold 100, got %d", e.Threshold)
	}
}

// @ac AC-04
func TestTier3At60Passes(t *testing.T) {
	specs := []schema.SpecAST{makeSpec("utils", 3, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")}
	anns := []AnnotationMatch{
		{File: "test.ts", SpecID: "utils", ACIDs: []string{"AC-01", "AC-02", "AC-03"}},
	}

	report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
	e := report.Entries[0]

	if !e.PassesThreshold {
		t.Error("expected Tier 3 at 60% to pass (threshold 50%)")
	}
}

// @ac AC-05
func TestPythonAnnotations(t *testing.T) {
	content := "# @spec user-auth\n# @ac AC-01\n"
	matches := ExtractAnnotations(content, "test.py")

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SpecID != "user-auth" {
		t.Errorf("expected 'user-auth', got %q", matches[0].SpecID)
	}
}

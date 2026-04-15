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

// Regression: BUG-001 — multiple AC IDs on a single @ac line must all be registered.
func TestAnnotationExtraction_MultiACOnOneLine(t *testing.T) {
	content := "// @spec deadman-timer\n// @ac AC-02 AC-03 AC-04\n"
	matches := ExtractAnnotations(content, "timer_test.go")

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	acSet := make(map[string]bool)
	for _, id := range matches[0].ACIDs {
		acSet[id] = true
	}
	for _, want := range []string{"AC-02", "AC-03", "AC-04"} {
		if !acSet[want] {
			t.Errorf("expected %s to be covered from single-line @ac annotation, but it was not", want)
		}
	}
}

// @ac AC-06
func TestPerSpecCoverageThreshold_OverridesTierDefault(t *testing.T) {
	// Tier 1 spec (default threshold 100%) with coverage_threshold: 75
	// 4 of 5 ACs covered = 80%. Should PASS because 80 >= 75.
	spec := makeSpec("payment", 1, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")
	spec.CoverageThreshold = 75
	specs := []schema.SpecAST{spec}
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "payment", ACIDs: []string{"AC-01", "AC-02", "AC-03", "AC-04"}},
	}
	report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
	e := report.Entries[0]
	if e.Threshold != 75 {
		t.Errorf("expected threshold 75 (per-spec override), got %d", e.Threshold)
	}
	if !e.PassesThreshold {
		t.Errorf("expected to pass at 80%% with threshold 75")
	}
}

// @ac AC-07
func TestPassRateAwareCoverage_FailedResultNotCounted(t *testing.T) {
	// Tier 1 spec: annotation exists but result entry says passed: false
	spec := makeSpec("engine", 1, "AC-01")
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "engine", ACIDs: []string{"AC-01"}},
	}
	results := &ResultsFile{
		Results: []ResultEntry{{SpecID: "engine", ACID: "AC-01", Passed: false}},
	}
	report := BuildCoverageReportWithResults([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, results)
	e := report.Entries[0]
	if e.CoveragePct != 0 {
		t.Errorf("expected 0%% coverage when result failed, got %.1f%%", e.CoveragePct)
	}
}

// @ac AC-08
func TestCheckDependencyCoverage_EmitsWarning(t *testing.T) {
	// spec A depends on spec B; spec B is below threshold
	specA := makeSpec("spec-a", 1, "AC-01")
	specB := makeSpec("spec-b", 1, "AC-01")
	specs := []schema.SpecAST{specA, specB}
	// Only spec A has annotations; spec B has none
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "spec-a", ACIDs: []string{"AC-01"}},
	}
	report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
	edges := []DepEdge{{From: "spec-a", To: "spec-b"}}
	warnings := CheckDependencyCoverage(edges, report)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 dependency_coverage warning, got %d", len(warnings))
	}
	if warnings[0].DependsOn != "spec-b" {
		t.Errorf("expected warning about spec-b, got %q", warnings[0].DependsOn)
	}
}

// Regression: @spec inside a string literal must not hijack the current spec context.
func TestAnnotationExtraction_StringLiteralNotHijacked(t *testing.T) {
	// Simulate a test file where a Go string literal contains "// @spec other-spec".
	// The annotation extractor must not switch context to "other-spec".
	content := `// @spec my-spec
// @ac AC-01
func TestFoo(t *testing.T) {
	content := "// @spec other-spec\n// @ac AC-02\n"
	_ = content
}
// @ac AC-02
func TestBar(t *testing.T) {}
`
	matches := ExtractAnnotations(content, "foo_test.go")

	// Find the my-spec entry
	var mySpec *AnnotationMatch
	for i := range matches {
		if matches[i].SpecID == "my-spec" {
			mySpec = &matches[i]
		}
	}
	if mySpec == nil {
		t.Fatal("expected annotation for my-spec, got none")
	}

	// Both AC-01 and AC-02 should be under my-spec, not hijacked to other-spec
	acSet := make(map[string]bool)
	for _, id := range mySpec.ACIDs {
		acSet[id] = true
	}
	if !acSet["AC-01"] {
		t.Error("expected AC-01 under my-spec")
	}
	if !acSet["AC-02"] {
		t.Error("expected AC-02 under my-spec (string literal must not hijack spec context)")
	}

	// other-spec must not appear in results
	for _, m := range matches {
		if m.SpecID == "other-spec" {
			t.Error("other-spec from inside a string literal must not appear in results")
		}
	}
}

// @spec spec-coverage
package coverage

import (
	"errors"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/schema"
)

// @ac AC-19
// Under StrictMode=true, a Tier 2 spec whose annotated AC failed in results
// MUST be reported as uncovered. Under StrictMode=false (today's behavior),
// tier 2 ignores results entirely.
func TestStrictMode_FailedResultDemotesAllTiers(t *testing.T) {
	spec := makeSpec("svc", 2, "AC-03")
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "svc", ACIDs: []string{"AC-03"}},
	}
	results := &ResultsFile{
		Results: []ResultEntry{{SpecID: "svc", ACID: "AC-03", Status: "failed"}},
	}

	// strict=false → today's behavior, AC-03 counted as covered (tier 2)
	nonStrict, err := BuildCoverageReportStrict([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, results, false, nil)
	if err != nil {
		t.Fatalf("non-strict returned error: %v", err)
	}
	if len(nonStrict.Entries[0].CoveredACs) != 1 {
		t.Errorf("non-strict: expected AC-03 covered for tier 2, got covered=%v", nonStrict.Entries[0].CoveredACs)
	}

	// strict=true → AC-03 uncovered regardless of tier
	strict, err := BuildCoverageReportStrict([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, results, true, nil)
	if err != nil {
		t.Fatalf("strict returned error: %v", err)
	}
	if len(strict.Entries[0].UncoveredACs) != 1 || strict.Entries[0].UncoveredACs[0] != "AC-03" {
		t.Errorf("strict: expected AC-03 uncovered, got uncovered=%v covered=%v",
			strict.Entries[0].UncoveredACs, strict.Entries[0].CoveredACs)
	}
}

// @ac AC-19
// Skipped results also demote under strict.
func TestStrictMode_SkippedResultIsUncovered(t *testing.T) {
	spec := makeSpec("svc", 3, "AC-01")
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "svc", ACIDs: []string{"AC-01"}},
	}
	results := &ResultsFile{
		Results: []ResultEntry{{SpecID: "svc", ACID: "AC-01", Status: "skipped"}},
	}
	report, _ := BuildCoverageReportStrict([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, results, true, nil)
	if len(report.Entries[0].UncoveredACs) != 1 {
		t.Errorf("skipped under strict should be uncovered, got %+v", report.Entries[0])
	}
}

// @ac AC-20
func TestStrictMode_MissingResultsFile_IsHardFail(t *testing.T) {
	spec := makeSpec("svc", 2, "AC-01")
	anns := []AnnotationMatch{
		{File: "t.go", SpecID: "svc", ACIDs: []string{"AC-01"}},
	}

	_, err := BuildCoverageReportStrict([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, nil, true, nil)
	if err == nil {
		t.Fatal("strict=true with nil results must return an error")
	}
	if !strings.Contains(err.Error(), "--strict requires .specter-results.json") {
		t.Errorf("error message must mention `--strict requires .specter-results.json`, got: %v", err)
	}

	// v1.10.0 / AC-23: empty parseable results (non-nil, zero entries) no
	// longer errors — proceeds with demotion; the CLI layer emits a
	// self-diagnosing warning. Supports staged adoption where zero tests
	// have been migrated to runner-visible annotations yet.
	_, err = BuildCoverageReportStrict([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, &ResultsFile{}, true, nil)
	if err != nil {
		t.Fatalf("strict=true with empty (non-nil) results must succeed (warn-and-continue is CLI-layer); got: %v", err)
	}
}

// @ac AC-20
// Confirm the error is distinguishable (sentinel or wrapped).
func TestStrictMode_MissingResultsError_IsErrMissingResults(t *testing.T) {
	_, err := BuildCoverageReportStrict(nil, nil, checker.CoverageThresholdByTier, nil, true, nil)
	if !errors.Is(err, ErrMissingResults) {
		t.Errorf("expected errors.Is(err, ErrMissingResults), got: %v", err)
	}
}

// @ac AC-21
// Back-compat: ParseResultsFile accepts the old {"passed": true} shape.
func TestParseResultsFile_BackCompatBooleanOnly(t *testing.T) {
	data := []byte(`{"results":[{"spec_id":"s","ac_id":"AC-01","passed":true}]}`)
	rf, err := ParseResultsFile(data)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(rf.Results) != 1 {
		t.Fatalf("expected 1 entry")
	}
	e := rf.Results[0]
	if e.Status != "passed" {
		t.Errorf("expected derived Status=passed, got %q", e.Status)
	}
	if !e.Passed {
		t.Errorf("expected Passed=true")
	}
}

// @ac AC-21
// New format: status-only entries produce a consistent Passed boolean.
func TestParseResultsFile_StatusFieldDerivesPassedBool(t *testing.T) {
	data := []byte(`{"results":[{"spec_id":"s","ac_id":"AC-01","status":"failed"}]}`)
	rf, _ := ParseResultsFile(data)
	e := rf.Results[0]
	if e.Status != "failed" {
		t.Errorf("Status = %q, want failed", e.Status)
	}
	if e.Passed {
		t.Errorf("Passed should be false when status=failed")
	}
}

// @ac AC-22
// The result-lookup function returns the specific status or "unknown" if absent.
func TestResultsFile_Status_ReturnsUnknownWhenAbsent(t *testing.T) {
	rf := &ResultsFile{
		Results: []ResultEntry{{SpecID: "a", ACID: "AC-01", Status: "passed", Passed: true}},
	}
	if got := rf.status("a", "AC-01"); got != "passed" {
		t.Errorf("status(a, AC-01) = %q, want passed", got)
	}
	if got := rf.status("a", "AC-99"); got != "unknown" {
		t.Errorf("status(a, AC-99) = %q, want unknown", got)
	}
}

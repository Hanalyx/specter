// coverage_test.go -- CLI-level tests for `specter coverage`.
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

// @ac AC-10
// Parse-failure workspace: `specter coverage --json` must still emit a valid
// CoverageReport JSON document with parse_errors populated, and exit non-zero.
// Fixes B1: the VS Code extension needs a structured document in every state
// to distinguish "no specs yet" from "specs present but failed to parse".
func TestCoverage_JSON_EmitsReportOnParseFailure(t *testing.T) {
	dir := t.TempDir()
	// Write a spec that fails schema validation — missing `objective`, a
	// required top-level field.
	broken := `spec:
  id: broken-spec
  version: "1.0.0"
  status: draft
  tier: 3

  context:
    system: Test
    feature: Test

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
	writeSpec(t, dir, "broken.spec.yaml", broken)

	out, code := runCLI(t, dir, "coverage", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit on parse failure, got 0. output:\n%s", out)
	}

	// Parse failures print to stderr; JSON is on stdout. runCLI combines
	// them, so find the JSON substring.
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no JSON document in output:\n%s", out)
	}
	end := strings.LastIndex(out, "}")
	if end < start {
		t.Fatalf("malformed JSON in output:\n%s", out)
	}

	var report coverage.CoverageReport
	if err := json.Unmarshal([]byte(out[start:end+1]), &report); err != nil {
		t.Fatalf("coverage --json did not emit valid CoverageReport JSON: %v\nraw:\n%s", err, out[start:end+1])
	}

	if len(report.ParseErrors) == 0 {
		t.Fatalf("expected parse_errors in JSON output, got none. report: %+v", report)
	}
	if report.ParseErrors[0].File == "" {
		t.Fatalf("parse error entry missing File: %+v", report.ParseErrors[0])
	}
	if report.ParseErrors[0].Message == "" {
		t.Fatalf("parse error entry missing Message: %+v", report.ParseErrors[0])
	}
	if len(report.Entries) != 0 {
		t.Fatalf("expected empty entries when all specs failed parse, got %d", len(report.Entries))
	}
}

// @spec spec-coverage
// @ac AC-12
// spec_candidates_count reflects the number of .spec.yaml files on disk,
// distinct from the (possibly zero) count of parseable entries. Lets the
// VS Code sidebar distinguish "no specs exist" from "specs exist but drift."
func TestCoverage_JSON_SpecCandidatesCount(t *testing.T) {
	dir := t.TempDir()
	broken := `spec:
  id: broken
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: x
    feature: y
  constraints:
    - id: C-01
      description: "x"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "y"
      references_constraints: ["C-01"]
      priority: high
`
	writeSpec(t, dir, "one.spec.yaml", broken)
	writeSpec(t, dir, "two.spec.yaml", broken)

	out, code := runCLI(t, dir, "coverage", "--json")
	if code == 0 {
		t.Fatalf("expected non-zero exit with broken specs; out:\n%s", out)
	}
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	var report coverage.CoverageReport
	if err := json.Unmarshal([]byte(out[start:end+1]), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.SpecCandidatesCount != 2 {
		t.Errorf("expected spec_candidates_count 2, got %d", report.SpecCandidatesCount)
	}
	if len(report.Entries) != 0 {
		t.Errorf("expected no entries, got %d", len(report.Entries))
	}
}

// @spec spec-coverage
// @ac AC-13
// parse_error_patterns groups errors by type×path and surfaces the dominant
// pattern for downstream drift diagnosis.
func TestCoverage_JSON_ParseErrorPatterns(t *testing.T) {
	dir := t.TempDir()
	// Two files, both missing the required `objective` field.
	broken := `spec:
  id: broken
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: x
    feature: y
  constraints:
    - id: C-01
      description: "x"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "y"
      references_constraints: ["C-01"]
      priority: high
`
	writeSpec(t, dir, "a.spec.yaml", broken)
	writeSpec(t, dir, "b.spec.yaml", broken)

	out, _ := runCLI(t, dir, "coverage", "--json")
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	var report coverage.CoverageReport
	if err := json.Unmarshal([]byte(out[start:end+1]), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.ParseErrorPatterns) == 0 {
		t.Fatal("expected parse_error_patterns populated")
	}
	if report.ParseErrorPatterns[0].Count < 2 {
		t.Errorf("expected top pattern to cover both files, got count %d", report.ParseErrorPatterns[0].Count)
	}
}

// @ac AC-10
// Happy-path JSON emission still works: a valid spec produces an entry and no
// parse_errors. Prevents the 1.5.0 change from regressing the normal case.
// Exit code may still be non-zero (spec has AC but no annotation → below
// threshold) — the assertion is on the JSON shape, not the exit code.
func TestCoverage_JSON_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "good.spec.yaml", minimalValidSpec("good-spec", 3))

	out, _ := runCLI(t, dir, "coverage", "--json")

	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end < start {
		t.Fatalf("no JSON document in output:\n%s", out)
	}

	var report coverage.CoverageReport
	if err := json.Unmarshal([]byte(out[start:end+1]), &report); err != nil {
		t.Fatalf("coverage --json did not emit valid JSON: %v", err)
	}
	if len(report.ParseErrors) != 0 {
		t.Fatalf("expected no parse_errors on happy path, got %+v", report.ParseErrors)
	}
	if len(report.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(report.Entries))
	}
}

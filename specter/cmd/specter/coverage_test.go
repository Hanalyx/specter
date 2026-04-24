// coverage_test.go -- CLI-level tests for `specter coverage`.
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	t.Run("spec-coverage/AC-10 json emits report on parse failure", func(t *testing.T) {
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
	})
}

// @spec spec-coverage
// @ac AC-12
// spec_candidates_count reflects the number of .spec.yaml files on disk,
// distinct from the (possibly zero) count of parseable entries. Lets the
// VS Code sidebar distinguish "no specs exist" from "specs exist but drift."
func TestCoverage_JSON_SpecCandidatesCount(t *testing.T) {
	t.Run("spec-coverage/AC-12 json spec candidates count", func(t *testing.T) {
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
	})
}

// @spec spec-coverage
// @ac AC-13
// parse_error_patterns groups errors by type×path and surfaces the dominant
// pattern for downstream drift diagnosis.
func TestCoverage_JSON_ParseErrorPatterns(t *testing.T) {
	t.Run("spec-coverage/AC-13 json parse error patterns", func(t *testing.T) {
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
	})
}

// @ac AC-10
// Happy-path JSON emission still works: a valid spec produces an entry and no
// parse_errors. Prevents the 1.5.0 change from regressing the normal case.
// Exit code may still be non-zero (spec has AC but no annotation → below
// threshold) — the assertion is on the JSON shape, not the exit code.
func TestCoverage_JSON_HappyPath(t *testing.T) {
	t.Run("spec-coverage/AC-10 json happy path", func(t *testing.T) {
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
	})
}

// --- v0.9.2 UX polish tests ---

// @spec spec-coverage
// @ac AC-16
// Default table output must include a summary header: `Spec Coverage Report —
// N specs · P% avg coverage` followed by per-tier breakdown lines.
func TestCoverage_Table_HasSummaryHeader(t *testing.T) {
	t.Run("spec-coverage/AC-16 table has summary header", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "alpha.spec.yaml", minimalValidSpec("alpha", 2, "AC-01"))
		writeSpec(t, dir, "beta.spec.yaml", minimalValidSpec("beta", 3, "AC-01"))

		out, _ := runCLI(t, dir, "coverage")

		// Header must include the em-dash form AND the spec count.
		if !strings.Contains(out, "Spec Coverage Report — 2 specs") {
			t.Errorf("expected summary header `Spec Coverage Report — 2 specs` in output, got:\n%s", out)
		}
		// Must include per-tier breakdown for every tier present.
		if !strings.Contains(out, "Tier 2:") {
			t.Errorf("expected `Tier 2:` breakdown line in output, got:\n%s", out)
		}
		if !strings.Contains(out, "Tier 3:") {
			t.Errorf("expected `Tier 3:` breakdown line in output, got:\n%s", out)
		}
		// Tier 1 not present in this workspace — MUST NOT be in output.
		if strings.Contains(out, "Tier 1:") {
			t.Errorf("unexpected `Tier 1:` breakdown line (no T1 specs in workspace):\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-15
// Entries are sorted worst-first: failing below threshold → partial but
// passing threshold → 100% covered. Within each bucket, tier desc (T1 > T2 > T3).
func TestCoverage_Table_SortsWorstFirst(t *testing.T) {
	t.Run("spec-coverage/AC-15 table sorts worst first", func(t *testing.T) {
		dir := t.TempDir()
		// failing-t2: tier 2, 1 AC, no coverage → 0%, below 80% threshold, FAIL
		writeSpec(t, dir, "failing-t2.spec.yaml", minimalValidSpec("failing-t2", 2, "AC-01"))
		// complete-t1: tier 1, 1 AC covered → 100%, PASS
		writeSpec(t, dir, "complete-t1.spec.yaml", minimalValidSpec("complete-t1", 1, "AC-01"))
		// The tier-1 spec gets covered via an annotation file.
		testDir := filepath.Join(dir, "tests")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(testDir, "complete_test.go"),
			[]byte("// @spec complete-t1\n// @ac AC-01\nfunc TestC(t *testing.T) {}\n"), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage")

		// The failing spec must appear before the complete spec in the output.
		failingIdx := strings.Index(out, "failing-t2")
		completeIdx := strings.Index(out, "complete-t1")
		if failingIdx < 0 || completeIdx < 0 {
			t.Fatalf("both specs must appear in output; got:\n%s", out)
		}
		if failingIdx > completeIdx {
			t.Errorf("failing spec must sort before complete spec; complete-t1 at %d appeared before failing-t2 at %d", completeIdx, failingIdx)
		}
	})
}

// @spec spec-coverage
// @ac AC-17
// `--failing` filters the table to entries below 100% coverage. When all
// specs are at 100%, the flag produces a single-line confirmation.
func TestCoverage_Failing_HidesPassingEntries(t *testing.T) {
	t.Run("spec-coverage/AC-17 failing hides passing entries", func(t *testing.T) {
		dir := t.TempDir()
		// complete: 100% covered
		writeSpec(t, dir, "complete.spec.yaml", minimalValidSpec("complete", 3, "AC-01"))
		testDir := filepath.Join(dir, "tests")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(testDir, "complete_test.go"),
			[]byte("// @spec complete\n// @ac AC-01\nfunc TestC(t *testing.T) {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		// failing: no annotations, 0% covered
		writeSpec(t, dir, "failing.spec.yaml", minimalValidSpec("failing", 2, "AC-01"))

		out, _ := runCLI(t, dir, "coverage", "--failing")

		// Must still print the summary header (reflects full report).
		if !strings.Contains(out, "Spec Coverage Report") {
			t.Errorf("expected summary header even with --failing, got:\n%s", out)
		}
		// Must include the failing spec in the table.
		if !strings.Contains(out, "failing") {
			t.Errorf("expected failing spec in --failing output, got:\n%s", out)
		}
		// Must NOT include the complete spec's table row.
		// Check as a word boundary — the spec ID "complete" MUST not show up
		// as a table row entry (it can still appear inside the summary line).
		lines := strings.Split(out, "\n")
		for _, ln := range lines {
			if strings.HasPrefix(strings.TrimSpace(ln), "complete ") {
				t.Errorf("--failing MUST hide 100%%-covered specs, but found row: %s", ln)
			}
		}
	})
}

// @spec spec-coverage
// @ac AC-17
// When every spec is 100% covered, `--failing` emits a single-line
// confirmation instead of an empty table.
func TestCoverage_Failing_AllPassing_SingleLine(t *testing.T) {
	t.Run("spec-coverage/AC-17 failing all passing single line", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "a.spec.yaml", minimalValidSpec("a", 3, "AC-01"))
		writeSpec(t, dir, "b.spec.yaml", minimalValidSpec("b", 3, "AC-01"))
		testDir := filepath.Join(dir, "tests")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(testDir, "t_test.go"), []byte(
			"// @spec a\n// @ac AC-01\nfunc TestA(t *testing.T) {}\n"+
				"// @spec b\n// @ac AC-01\nfunc TestB(t *testing.T) {}\n"),
			0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage", "--failing")
		if !strings.Contains(out, "All 2 specs at 100% coverage.") {
			t.Errorf("expected single-line confirmation, got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-18
// Spec IDs longer than 40 chars are truncated in the default table output.
// JSON output is unaffected.
func TestCoverage_Table_TruncatesLongSpecIDs(t *testing.T) {
	t.Run("spec-coverage/AC-18 table truncates long spec ids", func(t *testing.T) {
		dir := t.TempDir()
		// 50-char spec ID.
		longID := "app-api-admin-appointments-id-service-override-xxx" // 50 chars
		if len(longID) != 50 {
			t.Fatalf("test setup: expected 50-char ID, got %d chars: %q", len(longID), longID)
		}
		writeSpec(t, dir, "long.spec.yaml", minimalValidSpec(longID, 2, "AC-01"))

		// Table output MUST contain a truncated form, with an ellipsis.
		tableOut, _ := runCLI(t, dir, "coverage")
		if !strings.Contains(tableOut, "…") {
			t.Errorf("expected ellipsis in truncated output; got:\n%s", tableOut)
		}
		if strings.Contains(tableOut, longID) {
			t.Errorf("table output must NOT contain the full 50-char spec ID (should be truncated); got:\n%s", tableOut)
		}

		// JSON output MUST contain the full ID unchanged.
		jsonOut, _ := runCLI(t, dir, "coverage", "--json")
		if !strings.Contains(jsonOut, longID) {
			t.Errorf("--json output must contain full spec ID, got:\n%s", jsonOut)
		}
	})
}

// --- v0.10 CI-gated coverage (--strict) tests ---

// @spec spec-coverage
// @ac AC-20
// `specter coverage --strict` without a .specter-results.json must fail with
// an explanatory stderr message. Silently falling back to annotation-only
// under --strict would defeat the gate's purpose.
func TestCoverage_Strict_MissingResultsFile_Fails(t *testing.T) {
	t.Run("spec-coverage/AC-20 strict missing results file fails", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "alpha.spec.yaml", minimalValidSpec("alpha", 2, "AC-01"))

		out, code := runCLI(t, dir, "coverage", "--strict")
		if code == 0 {
			t.Fatalf("expected non-zero exit, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "--strict requires .specter-results.json") {
			t.Errorf("expected error mentioning `--strict requires .specter-results.json`; got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-19
// --strict: annotated AC whose result failed is reported as uncovered,
// even on tier 2/3 (which today's pass-rate-aware logic ignores).
func TestCoverage_Strict_FailedResultDemotesTier2(t *testing.T) {
	t.Run("spec-coverage/AC-19 strict failed result demotes tier 2", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "svc.spec.yaml", minimalValidSpec("svc", 2, "AC-01"))

		// Annotated test file matching the spec.
		testDir := filepath.Join(dir, "tests")
		_ = os.MkdirAll(testDir, 0755)
		_ = os.WriteFile(filepath.Join(testDir, "svc_test.go"), []byte(
			"// @spec svc\n// @ac AC-01\nfunc TestX(t *testing.T) {}\n"), 0644)

		// Write a results file marking AC-01 as failed.
		results := `{"results":[{"spec_id":"svc","ac_id":"AC-01","status":"failed"}]}`
		_ = os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644)

		// Non-strict: tier 2 annotation alone counts as covered → passes.
		out, code := runCLI(t, dir, "coverage", "--tests", "tests/*_test.go")
		if code != 0 {
			t.Fatalf("non-strict should pass (tier 2 annotation-only); got exit=%d\n%s", code, out)
		}

		// Strict: failed result demotes the AC → coverage should fail.
		strictOut, strictCode := runCLI(t, dir, "coverage", "--strict", "--tests", "tests/*_test.go")
		if strictCode == 0 {
			t.Fatalf("strict mode should fail when AC-01's result is failed; got exit=0\n%s", strictOut)
		}
	})
}

// @spec spec-coverage
// @ac AC-19
// --strict + all-passed results: coverage passes normally.
func TestCoverage_Strict_AllPassed_Passes(t *testing.T) {
	t.Run("spec-coverage/AC-19 strict all passed passes", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "svc.spec.yaml", minimalValidSpec("svc", 2, "AC-01"))

		testDir := filepath.Join(dir, "tests")
		_ = os.MkdirAll(testDir, 0755)
		_ = os.WriteFile(filepath.Join(testDir, "svc_test.go"), []byte(
			"// @spec svc\n// @ac AC-01\nfunc TestX(t *testing.T) {}\n"), 0644)

		results := `{"results":[{"spec_id":"svc","ac_id":"AC-01","status":"passed"}]}`
		_ = os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644)

		_, code := runCLI(t, dir, "coverage", "--strict", "--tests", "tests/*_test.go")
		if code != 0 {
			t.Errorf("strict with all-passed should exit 0, got %d", code)
		}
	})
}

// --- v0.10.0 adoption affordances ---

// @spec spec-coverage
// @ac AC-23
// Empty .specter-results.json under --strict must emit a self-diagnosing
// warning (naming the likely cause + pointing at the conventions doc)
// BEFORE the demotion report. Without this, a Day-1 operator sees 100%
// silent demotion with no hint about why.
func TestCoverage_Strict_EmptyResults_WarnsWithGuidance(t *testing.T) {
	t.Run("spec-coverage/AC-23 strict empty results warns with guidance", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "svc.spec.yaml", minimalValidSpec("svc", 2, "AC-01"))

		testDir := filepath.Join(dir, "tests")
		_ = os.MkdirAll(testDir, 0755)
		_ = os.WriteFile(filepath.Join(testDir, "svc_test.go"), []byte(
			"// @spec svc\n// @ac AC-01\nfunc TestX(t *testing.T) {}\n"), 0644)

		// Parseable but empty results file.
		_ = os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(`{"results":[]}`), 0644)

		out, _ := runCLI(t, dir, "coverage", "--strict", "--tests", "tests/*_test.go")

		if !strings.Contains(out, "no (spec_id, ac_id) pairs were extracted") {
			t.Errorf("expected warning naming the cause; got:\n%s", out)
		}
		if !strings.Contains(out, "docs/explainer/v0.10-ci-gated-coverage.md") {
			t.Errorf("expected warning to reference the conventions doc; got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-24
// --scope narrows --strict's demand set to specs in the named domain.
// ACs of out-of-scope specs fall back to v0.9 boolean-passed semantics
// (annotation alone = covered). The report still includes all specs.
func TestCoverage_Strict_Scope_NarrowsDemandToDomain(t *testing.T) {
	t.Run("spec-coverage/AC-24 strict scope narrows demand to domain", func(t *testing.T) {
		dir := t.TempDir()

		// specter.yaml with two domains: one in scope, one not.
		manifest := `system:
  name: test-system
  description: "test"
  tier: 2

domains:
  approval-gate:
    tier: 1
    description: "Financial"
    specs:
      - stripe-charge
  general:
    tier: 2
    description: "Rest"
    specs:
      - user-profile
`
		_ = os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(manifest), 0644)

		writeSpec(t, dir, "stripe-charge.spec.yaml", minimalValidSpec("stripe-charge", 1, "AC-01", "AC-02"))
		writeSpec(t, dir, "user-profile.spec.yaml", minimalValidSpec("user-profile", 2, "AC-01"))

		testDir := filepath.Join(dir, "tests")
		_ = os.MkdirAll(testDir, 0755)
		_ = os.WriteFile(filepath.Join(testDir, "stripe_test.go"), []byte(
			"// @spec stripe-charge\n// @ac AC-01\n// @ac AC-02\nfunc TestStripe(t *testing.T) {}\n"), 0644)
		_ = os.WriteFile(filepath.Join(testDir, "profile_test.go"), []byte(
			"// @spec user-profile\n// @ac AC-01\nfunc TestProfile(t *testing.T) {}\n"), 0644)

		// Results: stripe-charge/AC-01 passed; AC-02 has no entry (missing).
		// user-profile has NO results entries at all — but it's outside --scope.
		results := `{"results":[{"spec_id":"stripe-charge","ac_id":"AC-01","status":"passed"}]}`
		_ = os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644)

		out, code := runCLI(t, dir, "coverage", "--strict", "--scope", "approval-gate", "--tests", "tests/*_test.go")

		// stripe-charge/AC-02 must demote (in scope, no passing result) → non-zero exit.
		if code == 0 {
			t.Fatalf("expected non-zero (stripe-charge/AC-02 should demote under scope); got 0\n%s", out)
		}

		// user-profile must appear covered — it's outside --scope, so boolean-passed
		// logic applies: annotation alone counts as covered.
		if !strings.Contains(out, "user-profile") {
			t.Errorf("report should still include out-of-scope user-profile; got:\n%s", out)
		}
		// The demotion message for AC-02 should be visible (somewhere in output).
		if !strings.Contains(out, "AC-02") {
			t.Errorf("expected demotion report to mention AC-02; got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-25
// --scope with unknown domain must fail fast with a helpful stderr message.
// --scope without --strict must also fail fast (no silent degradation).
func TestCoverage_Scope_FailFast_UnknownAndMissingStrict(t *testing.T) {
	t.Run("spec-coverage/AC-25 scope fail fast unknown and missing strict", func(t *testing.T) {
		dir := t.TempDir()

		manifest := `system:
  name: t
  description: "t"
  tier: 2

domains:
  approval-gate:
    tier: 1
    description: "f"
    specs: [svc]
`
		_ = os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(manifest), 0644)
		writeSpec(t, dir, "svc.spec.yaml", minimalValidSpec("svc", 2, "AC-01"))

		// Scenario 1: --scope <unknown>
		out1, code1 := runCLI(t, dir, "coverage", "--strict", "--scope", "nonexistent-domain")
		if code1 == 0 {
			t.Errorf("expected non-zero exit on unknown domain; got 0\n%s", out1)
		}
		if !strings.Contains(out1, "unknown") || !strings.Contains(out1, "nonexistent-domain") {
			t.Errorf("expected message naming the unknown domain; got:\n%s", out1)
		}
		if !strings.Contains(out1, "approval-gate") {
			t.Errorf("expected message listing valid domain names; got:\n%s", out1)
		}

		// Scenario 2: --scope without --strict
		out2, code2 := runCLI(t, dir, "coverage", "--scope", "approval-gate")
		if code2 == 0 {
			t.Errorf("expected non-zero exit when --scope used without --strict; got 0\n%s", out2)
		}
		if !strings.Contains(out2, "--scope requires --strict") {
			t.Errorf("expected `--scope requires --strict` message; got:\n%s", out2)
		}
	})
}

// @spec spec-coverage
// @ac AC-26
// --scope + --tests combine as AND: annotations scanned only in glob-matching
// files, AND --strict demand applies only to ACs of specs in scope domain.
func TestCoverage_Strict_Scope_And_Tests_CombineAsAND(t *testing.T) {
	t.Run("spec-coverage/AC-26 strict scope and tests combine as AND", func(t *testing.T) {
		dir := t.TempDir()

		manifest := `system:
  name: t
  description: "t"
  tier: 2

domains:
  approval-gate:
    tier: 1
    description: "f"
    specs: [scoped-spec]
`
		_ = os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(manifest), 0644)

		writeSpec(t, dir, "scoped.spec.yaml", minimalValidSpec("scoped-spec", 2, "AC-01"))
		writeSpec(t, dir, "other.spec.yaml", minimalValidSpec("other-spec", 2, "AC-01"))

		// Two test dirs: only one is in the --tests glob.
		inGlob := filepath.Join(dir, "tests", "in")
		outGlob := filepath.Join(dir, "other-tests")
		_ = os.MkdirAll(inGlob, 0755)
		_ = os.MkdirAll(outGlob, 0755)

		// Test file IN glob annotates the in-scope spec.
		_ = os.WriteFile(filepath.Join(inGlob, "svc_test.go"), []byte(
			"// @spec scoped-spec\n// @ac AC-01\nfunc TestS(t *testing.T) {}\n"), 0644)

		// Test file OUT of glob annotates other-spec (should not be scanned at all).
		_ = os.WriteFile(filepath.Join(outGlob, "other_test.go"), []byte(
			"// @spec other-spec\n// @ac AC-01\nfunc TestO(t *testing.T) {}\n"), 0644)

		// scoped-spec/AC-01 has no passing result → must demote under --strict + in scope.
		_ = os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(`{"results":[]}`), 0644)

		out, code := runCLI(t, dir, "coverage", "--strict", "--scope", "approval-gate", "--tests", "tests/in/*_test.go")

		// In-scope + in-glob → demoted → non-zero exit.
		if code == 0 {
			t.Fatalf("expected non-zero (scoped-spec/AC-01 should demote); got 0\n%s", out)
		}

		// other-spec's test is out of glob — annotation isn't even scanned for it.
		// The report may still list other-spec (because it exists as a .spec.yaml),
		// but its ACs should be uncovered-by-no-annotation (not demoted-by-strict).
		// The diagnostic we care about: scoped-spec must be the one reported demoted.
		if !strings.Contains(out, "scoped-spec") {
			t.Errorf("expected scoped-spec to appear in demotion report; got:\n%s", out)
		}
	})
}

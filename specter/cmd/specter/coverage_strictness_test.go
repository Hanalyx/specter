// coverage_strictness_test.go -- CLI-level tests for the v0.11
// settings.strictness gate (Wave C).
//
// @spec spec-coverage
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest creates a minimal specter.yaml at the given path with the
// settings.strictness value set.
func writeManifestWithStrictness(t *testing.T, dir, strictness string) {
	t.Helper()
	body := "system:\n  name: test\nsettings:\n  specs_dir: .\n"
	if strictness != "" {
		body += "  strictness: " + strictness + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// @ac AC-27
func TestCoverageStrictness_CLIFlagOverridesManifest(t *testing.T) {
	t.Run("spec-coverage/AC-27 --strictness flag overrides manifest setting", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))

		// --strictness annotation rejects --strict (per C-24).
		out, code := runCLI(t, dir, "coverage", "--strict", "--strictness", "annotation")
		if code == 0 {
			t.Errorf("expected nonzero exit when --strict combined with annotation strictness, got 0; output:\n%s", out)
		}
		if !strings.Contains(strings.ToLower(out), "strictness") {
			t.Errorf("expected 'strictness' in error message, got:\n%s", out)
		}
	})
}

// @ac AC-28
func TestCoverageStrictness_ZeroTolerance_FailsOnNonPassedAC(t *testing.T) {
	t.Run("spec-coverage/AC-28 zero-tolerance fails on non-passed annotated AC even when tier threshold met", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "zero-tolerance")

		// One Tier 2 spec with two ACs. Both annotated. Results say AC-01 failed.
		// Coverage % = 100% (both annotated). Tier 2 threshold = 80%, so under
		// `threshold` mode this would pass. Under zero-tolerance, the failed AC
		// triggers a non-zero exit.
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01", "AC-02"))
		testFile := "// @spec my-spec\n// @ac AC-01\n// @ac AC-02\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "failed", "test_name": "TestFoo"},
			{"spec_id": "my-spec", "ac_id": "AC-02", "status": "passed", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "coverage", "--strict")
		if code == 0 {
			t.Fatalf("expected nonzero exit under zero-tolerance with one failed AC, got 0; output:\n%s", out)
		}
	})
}

// @ac AC-29
func TestCoverageStrictness_ZeroTolerance_FailsOnApprovalGate(t *testing.T) {
	t.Run("spec-coverage/AC-29 zero-tolerance fails on approval_gate=true with unset approval_date (exit 3)", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "zero-tolerance")

		// Spec carries approval_gate=true on AC-01 with no approval_date.
		// Build it inline because minimalValidSpec doesn't emit gate metadata.
		specBody := `spec:
  id: gated-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: { system: x, feature: x }
  objective: { summary: x }
  constraints:
    - id: C-01
      description: "MUST do thing"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "Thing happens"
      approval_gate: true
      references_constraints: ["C-01"]
      priority: high
`
		if err := os.WriteFile(filepath.Join(dir, "gated.spec.yaml"), []byte(specBody), 0644); err != nil {
			t.Fatal(err)
		}
		// Annotate the AC so the empty-discovery gate doesn't fire.
		testFile := "// @spec gated-spec\n// @ac AC-01\nfunc TestGated(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "gated_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// Results: AC-01 passed (so the strictness check at exit 2 is satisfied).
		// Approval-gate violation should still trigger exit 3.
		results := `{"results": [{"spec_id": "gated-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestGated"}]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "coverage", "--strict")
		if code != 3 {
			t.Errorf("expected exit code 3 for approval_gate violation under zero-tolerance, got %d", code)
		}
		// GH #94 regression: under zero-tolerance, the report MUST demote the
		// approval-gate-violating AC. v0.11.0 fired exit 3 but left the report
		// showing the AC as covered (PASS). v0.11.1 demotes in the report too.
		if !strings.Contains(out, "0%") {
			t.Errorf("expected report to show 0%% coverage after approval_gate demotion, got:\n%s", out)
		}
		if !strings.Contains(out, "uncovered: AC-01") {
			t.Errorf("expected report to list AC-01 as uncovered after demotion, got:\n%s", out)
		}
		if strings.Contains(out, "100%") {
			t.Errorf("did not expect 100%% coverage in report after demotion (v0.11.0 bug); got:\n%s", out)
		}
	})
}

// GH #94 — under threshold mode, approval_gate violations stay metadata.
// The report must show the AC as PASS (no demotion). Regression guard for
// the v0.11.1 fix to ensure it doesn't accidentally demote in threshold mode.
func TestCoverageStrictness_ThresholdMode_DoesNotDemoteApprovalGate(t *testing.T) {
	t.Run("spec-coverage/AC-29 threshold mode does not demote approval_gate violations", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")

		specBody := `spec:
  id: gated-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: { system: x, feature: x }
  objective: { summary: x }
  constraints:
    - id: C-01
      description: "MUST do thing"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "Thing happens"
      approval_gate: true
      references_constraints: ["C-01"]
      priority: high
`
		if err := os.WriteFile(filepath.Join(dir, "gated.spec.yaml"), []byte(specBody), 0644); err != nil {
			t.Fatal(err)
		}
		testFile := "// @spec gated-spec\n// @ac AC-01\nfunc TestGated(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "gated_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		results := `{"results": [{"spec_id": "gated-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestGated"}]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "coverage", "--strict")
		if code != 0 {
			t.Errorf("expected exit 0 under threshold mode (approval_gate is metadata), got %d", code)
		}
		if !strings.Contains(out, "100%") || !strings.Contains(out, "PASS") {
			t.Errorf("expected 100%% PASS under threshold (no demotion), got:\n%s", out)
		}
	})
}

// @ac AC-30
func TestCoverageStrictness_EmptyTestDiscovery_WarnsThenFailsUnderZeroTolerance(t *testing.T) {
	t.Run("spec-coverage/AC-30 empty test discovery warns under threshold, errors under zero-tolerance", func(t *testing.T) {
		// Threshold mode: warn but don't fail (current behavior preserved).
		dirT := t.TempDir()
		writeManifestWithStrictness(t, dirT, "threshold")
		writeSpec(t, dirT, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))
		// No test files at all.
		out, _ := runCLI(t, dirT, "coverage", "--strict")
		if !strings.Contains(strings.ToLower(out), "no test files") &&
			!strings.Contains(strings.ToLower(out), "no @spec") &&
			!strings.Contains(strings.ToLower(out), "tests_glob") {
			t.Errorf("expected warning about empty test discovery under threshold, got:\n%s", out)
		}

		// Zero-tolerance mode: same setup must exit non-zero.
		dirZ := t.TempDir()
		writeManifestWithStrictness(t, dirZ, "zero-tolerance")
		writeSpec(t, dirZ, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))
		_, codeZ := runCLI(t, dirZ, "coverage", "--strict")
		if codeZ == 0 {
			t.Errorf("expected nonzero exit under zero-tolerance with empty test discovery, got 0")
		}
	})
}

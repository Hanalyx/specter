// sync_strictness_exit_test.go -- CLI-level tests for spec-sync C-09:
// zero-tolerance exit codes 2/3 mirror `coverage` (AC-12/AC-13), in
// text and --json modes alike.
//
// @spec spec-sync
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @ac AC-12
func TestSyncZeroTolerance_ExitCode2OnNonPassedAC(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		// Tier 3 spec, two annotated ACs, one failed in results.
		// Demoted coverage is 50%, which MEETS the Tier 3 threshold —
		// only the zero-tolerance gate can catch the failing AC.
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01", "AC-02"))
		testFile := "// @spec my-spec\n// @ac AC-01\n// @ac AC-02\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestFoo"},
			{"spec_id": "my-spec", "ac_id": "AC-02", "status": "failed", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("spec-sync/AC-12 sync --strictness zero-tolerance exits 2 on failed AC despite tier threshold met", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "zero-tolerance")
		if code != 2 {
			t.Errorf("expected exit code 2 (zero-tolerance non-passed AC, matching coverage), got %d; output:\n%s", code, out)
		}
		if !strings.Contains(out, "zero-tolerance") {
			t.Errorf("expected output to name the zero-tolerance violation, got:\n%s", out)
		}
	})

	t.Run("spec-sync/AC-12 sync --strictness zero-tolerance --json exits 2 (text/JSON parity)", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "zero-tolerance", "--json")
		if code != 2 {
			t.Errorf("expected exit code 2 in --json mode, got %d; output:\n%s", code, out)
		}
		// The JSON document must still be emitted before the exit.
		if !strings.Contains(out, "\"phases\"") {
			t.Errorf("expected JSON document emitted before exit, got:\n%s", out)
		}
	})

	t.Run("spec-sync/AC-12 sync --strictness threshold passes the same workspace", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "threshold")
		if code != 0 {
			t.Errorf("expected exit 0 under threshold (50%% meets Tier 3 threshold), got %d; output:\n%s", code, out)
		}
	})
}

// @ac AC-13
func TestSyncZeroTolerance_ExitCode3OnApprovalGate(t *testing.T) {
	setup := func(t *testing.T) string {
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
		writeSpec(t, dir, "gated.spec.yaml", specBody)
		testFile := "// @spec gated-spec\n// @ac AC-01\nfunc TestGated(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "gated_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// AC-01 passed — only the approval-gate violation remains.
		results := `{"results": [{"spec_id": "gated-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestGated"}]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("spec-sync/AC-13 sync --strictness zero-tolerance exits 3 on approval_gate violation", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "zero-tolerance")
		if code != 3 {
			t.Errorf("expected exit code 3 (approval_gate violation, matching coverage), got %d; output:\n%s", code, out)
		}
		if !strings.Contains(out, "approval_gate") {
			t.Errorf("expected output to name the approval_gate violation, got:\n%s", out)
		}
	})

	t.Run("spec-sync/AC-13 sync --strictness zero-tolerance --json exits 3 (text/JSON parity)", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "zero-tolerance", "--json")
		if code != 3 {
			t.Errorf("expected exit code 3 in --json mode, got %d; output:\n%s", code, out)
		}
		if !strings.Contains(out, "\"phases\"") {
			t.Errorf("expected JSON document emitted before exit, got:\n%s", out)
		}
	})

	t.Run("spec-sync/AC-13 sync --strictness threshold passes (approval_gate is metadata)", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "sync", "--strictness", "threshold")
		if code != 0 {
			t.Errorf("expected exit 0 under threshold, got %d; output:\n%s", code, out)
		}
	})
}

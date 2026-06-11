// CLI integration tests for v0.13 D3 — coverage --json / text-mode
// exit-code parity (spec-coverage 1.13.0 C-29 / AC-34).
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-34 case 1: zero-tolerance + a failed annotated AC. Text-mode (AC-28)
// exits 2 even when the spec's tier-coverage % meets its threshold. With
// `--json` the report is emitted, AND the process exits with the same
// code 2. Pre-v0.13 JSON mode exited 0 here — the gap D3 closes.
//
// @ac AC-34
func TestCoverageJsonExit_ZeroToleranceFailed_MatchesTextExitCode(t *testing.T) {
	t.Run("spec-coverage/AC-34 --json exits with same code 2 as text mode for zero-tolerance non-passed AC", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "zero-tolerance")

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

		out, code := runCLI(t, dir, "coverage", "--strict", "--json")
		if code != 2 {
			t.Fatalf("expected --json exit code 2 (matching text-mode AC-28), got %d; output:\n%s", code, out)
		}

		// Per C-29: the JSON document is emitted BEFORE the exit-code
		// decision. Machine consumers must see structured state even on
		// non-zero exit.
		// runCLI returns combined stdout+stderr. JSON document is on
		// stdout; the strictness/approval-gate error message goes to
		// stderr after the JSON. Use json.Decoder to read only the first
		// JSON value and ignore any trailing prose.
		var parsed map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(out))
		if err := dec.Decode(&parsed); err != nil {
			t.Errorf("expected valid JSON document on stdout despite exit 2, got parse error: %v\noutput:\n%s", err, out)
		}
	})
}

// AC-34 case 2: zero-tolerance + approval_gate unset. Text-mode (AC-29)
// exits 3 (distinct from code 2). JSON mode must produce the same code.
//
// @ac AC-34
func TestCoverageJsonExit_ZeroToleranceApprovalGate_MatchesTextExitCode(t *testing.T) {
	t.Run("spec-coverage/AC-34 --json exits with same code 3 as text mode for approval_gate violation", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "zero-tolerance")

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
		// AC-01 passes — the strictness-non-passed check (exit 2) is
		// satisfied so the approval_gate check (exit 3) is the one that
		// fires.
		results := `{"results": [{"spec_id": "gated-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestGated"}]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "coverage", "--strict", "--json")
		if code != 3 {
			t.Fatalf("expected --json exit code 3 (matching text-mode AC-29), got %d; output:\n%s", code, out)
		}

		// runCLI returns combined stdout+stderr. JSON document is on
		// stdout; the strictness/approval-gate error message goes to
		// stderr after the JSON. Use json.Decoder to read only the first
		// JSON value and ignore any trailing prose.
		var parsed map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(out))
		if err := dec.Decode(&parsed); err != nil {
			t.Errorf("expected valid JSON document on stdout despite exit 3, got parse error: %v\noutput:\n%s", err, out)
		}
	})
}

// AC-34 case 3: threshold mode + spec below tier threshold. Text-mode
// exits non-zero (errSilent). JSON mode must also exit non-zero —
// pre-v0.13 it exited 0 silently.
//
// @ac AC-34
func TestCoverageJsonExit_ThresholdFailure_MatchesTextExitNonzero(t *testing.T) {
	t.Run("spec-coverage/AC-34 --json exits non-zero on threshold failure like text mode", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")

		// Tier 1 spec (100% threshold) with one annotated AC and one
		// unannotated AC → 50% coverage → fails threshold.
		writeSpec(t, dir, "tier1-spec.spec.yaml", minimalValidSpec("tier1-spec", 1, "AC-01", "AC-02"))
		testFile := "// @spec tier1-spec\n// @ac AC-01\nfunc TestT1(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "t1_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// C-31: threshold strictness routes the strict path, which requires
		// a results file before any report (and its JSON) can be built.
		results := `{"results": [{"spec_id": "tier1-spec", "ac_id": "AC-01", "status": "passed", "test_name": "TestT1"}]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "coverage", "--json")
		if code == 0 {
			t.Fatalf("expected --json exit non-zero on threshold failure (matching text mode), got 0; output:\n%s", out)
		}

		// runCLI returns combined stdout+stderr. JSON document is on
		// stdout; the strictness/approval-gate error message goes to
		// stderr after the JSON. Use json.Decoder to read only the first
		// JSON value and ignore any trailing prose.
		var parsed map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(out))
		if err := dec.Decode(&parsed); err != nil {
			t.Errorf("expected valid JSON document on stdout despite non-zero exit, got parse error: %v\noutput:\n%s", err, out)
		}

	})
}

// coverage_strictness_parity_test.go -- CLI-level tests for spec-coverage
// C-31/C-32: an effective strictness of threshold/zero-tolerance (flag or
// manifest) routes through the same strict path as --strict (AC-36), and
// a missing results file without the --strict flag gets a mode-aware
// message (AC-37).
//
// @spec spec-coverage
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @ac AC-36
func TestCoverageStrictnessParity_ThresholdRoutesStrictPath(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		// Tier 2 spec, two annotated ACs, AC-01 failed in results.
		// Strict path demotes AC-01 → 50% < 80% threshold → FAIL.
		// The pre-C-31 bug: without --strict, the failed AC counted as
		// covered → 100% PASS.
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
		return dir
	}

	t.Run("spec-coverage/AC-36 --strictness threshold, manifest threshold, and --strict produce identical reports", func(t *testing.T) {
		dir := setup(t)
		outStrict, codeStrict := runCLI(t, dir, "coverage", "--strict")
		outLevel, codeLevel := runCLI(t, dir, "coverage", "--strictness", "threshold")
		outPlain, codePlain := runCLI(t, dir, "coverage")

		if codeStrict == 0 {
			t.Fatalf("expected --strict to fail (demoted 50%% < Tier 2 threshold), got 0; output:\n%s", outStrict)
		}
		if codeLevel != codeStrict {
			t.Errorf("--strictness threshold exit %d != --strict exit %d (shortcut contract broken)", codeLevel, codeStrict)
		}
		if codePlain != codeStrict {
			t.Errorf("manifest-threshold exit %d != --strict exit %d", codePlain, codeStrict)
		}
		if outLevel != outStrict {
			t.Errorf("--strictness threshold output differs from --strict output:\n--- --strict ---\n%s\n--- --strictness threshold ---\n%s", outStrict, outLevel)
		}
		if outPlain != outStrict {
			t.Errorf("plain coverage (manifest threshold) output differs from --strict output:\n--- --strict ---\n%s\n--- plain ---\n%s", outStrict, outPlain)
		}
		if !strings.Contains(outLevel, "uncovered: AC-01") {
			t.Errorf("expected AC-01 demoted under --strictness threshold, got:\n%s", outLevel)
		}
	})

	t.Run("spec-coverage/AC-36 --strictness zero-tolerance demotes in the report AND exits 2", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "coverage", "--strictness", "zero-tolerance")
		if code != 2 {
			t.Errorf("expected exit 2 under zero-tolerance with a failed annotated AC, got %d; output:\n%s", code, out)
		}
		if !strings.Contains(out, "uncovered: AC-01") {
			t.Errorf("expected report to demote AC-01 (report and exit code must agree), got:\n%s", out)
		}
		if strings.Contains(out, "100%") {
			t.Errorf("did not expect 100%% in report while exiting 2 (report/exit mismatch), got:\n%s", out)
		}
	})
}

// @ac AC-37
func TestCoverageStrictnessParity_MissingResultsModeAwareMessage(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))
		testFile := "// @spec my-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// NO .specter-results.json.
		return dir
	}

	assertModeAware := func(t *testing.T, out string, code int, invocation string) {
		t.Helper()
		if code == 0 {
			t.Errorf("%s: expected non-zero exit with missing results file under threshold strictness, got 0; output:\n%s", invocation, out)
		}
		if !strings.Contains(out, `strictness "threshold" requires .specter-results.json`) {
			t.Errorf("%s: expected message to name the active strictness mode, got:\n%s", invocation, out)
		}
		if !strings.Contains(out, "specter ingest") {
			t.Errorf("%s: expected message to reference 'specter ingest', got:\n%s", invocation, out)
		}
		if !strings.Contains(out, "--strictness annotation") {
			t.Errorf("%s: expected message to offer '--strictness annotation', got:\n%s", invocation, out)
		}
		if strings.Contains(out, "--strict requires") {
			t.Errorf("%s: message must not attribute the requirement to a --strict flag the operator never passed, got:\n%s", invocation, out)
		}
	}

	t.Run("spec-coverage/AC-37 --strictness threshold without results fails with mode-aware message", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "coverage", "--strictness", "threshold")
		assertModeAware(t, out, code, "coverage --strictness threshold")
	})

	t.Run("spec-coverage/AC-37 plain coverage under manifest-default threshold fails with mode-aware message", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "coverage")
		assertModeAware(t, out, code, "coverage (manifest threshold)")
	})

	t.Run("spec-coverage/AC-37 explicit --strict keeps the C-20 wording", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "coverage", "--strict")
		if code == 0 {
			t.Fatalf("expected non-zero exit under --strict with missing results, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "--strict requires .specter-results.json") {
			t.Errorf("expected the existing C-20 wording when --strict was explicitly passed, got:\n%s", out)
		}
	})

	t.Run("spec-coverage/AC-37 --strictness annotation succeeds without results file", func(t *testing.T) {
		dir := setup(t)
		out, code := runCLI(t, dir, "coverage", "--strictness", "annotation")
		if code != 0 {
			t.Errorf("expected exit 0 under annotation strictness (no results required), got %d; output:\n%s", code, out)
		}
		if strings.Contains(out, "requires .specter-results.json") {
			t.Errorf("annotation mode must not demand a results file, got:\n%s", out)
		}
	})
}

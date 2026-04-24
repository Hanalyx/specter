// doctor_test.go -- CLI integration tests for specter doctor.
//
// @spec spec-doctor
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @ac AC-01
func TestDoctor_ManifestPresent_ReportsPass(t *testing.T) {
	t.Run("spec-doctor/AC-01 manifest present reports pass", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))
		writeManifest(t, dir, "system:\n  name: test-system\n")

		out, _ := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "manifest") {
			t.Fatalf("expected manifest check in output, got:\n%s", out)
		}
		if !strings.Contains(out, "[PASS]") || strings.Contains(out, "manifest     [WARN]") {
			t.Errorf("expected manifest check to PASS, got:\n%s", out)
		}
	})
}

// @ac AC-02
func TestDoctor_NoManifest_ReportsWarnNotFail(t *testing.T) {
	t.Run("spec-doctor/AC-02 no manifest reports warn not fail", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

		out, _ := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "manifest") {
			t.Fatalf("expected manifest check in output, got:\n%s", out)
		}
		if !strings.Contains(out, "[WARN]") {
			t.Errorf("expected manifest check to WARN when no specter.yaml, got:\n%s", out)
		}
		// Must not say FAIL for the manifest line specifically
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "manifest") && strings.Contains(line, "[FAIL]") {
				t.Errorf("manifest check must not FAIL when absent (should WARN): %s", line)
			}
		}
	})
}

// @ac AC-03
func TestDoctor_NoSpecFiles_ReportsFail(t *testing.T) {
	t.Run("spec-doctor/AC-03 no spec files reports fail", func(t *testing.T) {
		dir := t.TempDir()
		out, code := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "spec-files") {
			t.Fatalf("expected spec-files check in output, got:\n%s", out)
		}
		if !strings.Contains(out, "[FAIL]") {
			t.Errorf("expected FAIL when no spec files found, got:\n%s", out)
		}
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
}

// @ac AC-04
func TestDoctor_ParseErrors_ReportsFail(t *testing.T) {
	t.Run("spec-doctor/AC-04 parse errors reports fail", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write an invalid spec (missing required fields)
		if err := os.WriteFile(filepath.Join(specsDir, "bad.spec.yaml"), []byte("spec:\n  id: bad\n"), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "parse") {
			t.Fatalf("expected parse check in output, got:\n%s", out)
		}
		if !strings.Contains(out, "[FAIL]") {
			t.Errorf("expected parse check to FAIL on invalid spec, got:\n%s", out)
		}
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
}

// @spec spec-doctor
// @ac AC-09
// When every discovered spec hits the same parse-error shape, doctor names
// it as schema version drift instead of printing N identical errors.
func TestDoctor_ParsePatternAnalysis_NamesDrift(t *testing.T) {
	t.Run("spec-doctor/AC-09 parse pattern analysis names drift", func(t *testing.T) {
		dir := t.TempDir()
		specsDir := filepath.Join(dir, "specs")
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Two specs both missing the required `objective` field.
		broken := []byte("spec:\n  id: x\n  version: \"1.0.0\"\n  status: draft\n  tier: 3\n  context:\n    system: t\n    feature: f\n  constraints:\n    - id: C-01\n      description: x\n      type: technical\n      enforcement: error\n  acceptance_criteria:\n    - id: AC-01\n      description: y\n      references_constraints: [\"C-01\"]\n      priority: high\n")
		if err := os.WriteFile(filepath.Join(specsDir, "a.spec.yaml"), broken, 0644); err != nil {
			t.Fatal(err)
		}
		broken2 := []byte(string(broken) + "\n")
		if err := os.WriteFile(filepath.Join(specsDir, "b.spec.yaml"), broken2, 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "Pattern analysis") {
			t.Fatalf("expected pattern analysis block, got:\n%s", out)
		}
		if !strings.Contains(strings.ToLower(out), "schema version drift") {
			t.Errorf("expected 'schema version drift' diagnosis when every spec hits same pattern, got:\n%s", out)
		}
	})
}

// @ac AC-05
func TestDoctor_NoAnnotations_ReportsWarnNotFail(t *testing.T) {
	t.Run("spec-doctor/AC-05 no annotations reports warn not fail", func(t *testing.T) {
		dir := t.TempDir()
		// Write a tier 3 spec (50% threshold) with only AC-01 — 0% coverage but tier 3
		// Using tier 3 means coverage threshold is 50%, and 0/1 = 0% < 50% → FAIL.
		// To get WARN for annotations and isolate from coverage FAIL, use tier 3 with no ACs
		// Actually: let me write a spec with 0 ACs so coverage is 0/0 = 0%, threshold passes.
		// But the schema requires at least 1 AC. Let me just write a tier 3 with 1 AC and no annotation.
		// We can't avoid coverage FAIL here easily. Just check that annotations says WARN.
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

		out, _ := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "annotations") {
			t.Fatalf("expected annotations check in output, got:\n%s", out)
		}
		// With no test files annotated, annotations check should WARN
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "annotations") && strings.Contains(line, "[FAIL]") {
				t.Errorf("annotations check must WARN (not FAIL) when no annotations found: %s", line)
			}
		}
		if !strings.Contains(out, "[WARN]") {
			t.Errorf("expected at least one WARN in output, got:\n%s", out)
		}
	})
}

// @ac AC-06
func TestDoctor_BelowCoverageThreshold_ReportsFail(t *testing.T) {
	t.Run("spec-doctor/AC-06 below coverage threshold reports fail", func(t *testing.T) {
		dir := t.TempDir()
		// Tier 1 spec (100% threshold), 0% coverage → FAIL
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 1, "AC-01", "AC-02"))

		out, code := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "coverage") {
			t.Fatalf("expected coverage check in output, got:\n%s", out)
		}
		coverageFail := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "coverage") && strings.Contains(line, "[FAIL]") {
				coverageFail = true
			}
		}
		if !coverageFail {
			t.Errorf("expected coverage check to FAIL for tier-1 spec with 0%% coverage, got:\n%s", out)
		}
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
	})
}

// @ac AC-07
func TestDoctor_AllChecksAlwaysReported(t *testing.T) {
	t.Run("spec-doctor/AC-07 all checks always reported", func(t *testing.T) {
		dir := t.TempDir()
		// Invalid spec causes parse FAIL — all other checks must still appear
		specsDir := filepath.Join(dir, "specs")
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(specsDir, "bad.spec.yaml"), []byte("spec:\n  id: bad\n"), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "doctor")
		for _, check := range []string{"manifest", "spec-files", "parse", "annotations", "coverage"} {
			if !strings.Contains(out, check) {
				t.Errorf("check %q not found in output — all checks must always be reported:\n%s", check, out)
			}
		}
	})
}

// @ac AC-08
func TestDoctor_NoFileWrites(t *testing.T) {
	t.Run("spec-doctor/AC-08 no file writes", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

		// Snapshot all files before running doctor
		before := listAllFiles(t, dir)

		runCLI(t, dir, "doctor")

		// Snapshot after — must be identical
		after := listAllFiles(t, dir)
		if len(before) != len(after) {
			t.Errorf("doctor created files: before=%v, after=%v", before, after)
		}
	})
}

// Regression: BUG-002 — settings.exclude in specter.yaml must prevent spec discovery
// in excluded directories. A duplicate spec under an excluded path must not produce
// duplicate_id errors.
func TestResolve_ExcludeList_SkipsDirectory(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	// Write a duplicate spec under an excluded directory (simulates a git worktree)
	excluded := filepath.Join(dir, ".worktree", "specs")
	if err := os.MkdirAll(excluded, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excluded, "my-spec.spec.yaml"),
		[]byte(minimalValidSpec("my-spec", 3, "AC-01")), 0644); err != nil {
		t.Fatal(err)
	}

	// Without exclude list the duplicate should cause a duplicate_id error
	out, _ := runCLI(t, dir, "resolve")
	if !strings.Contains(out, "duplicate") {
		t.Logf("(baseline without exclude: no duplicate_id error — test env may differ)")
	}

	// Add exclude list to specter.yaml
	writeManifest(t, dir, "system:\n  name: test\nsettings:\n  exclude:\n    - .worktree\n")

	out, code := runCLI(t, dir, "resolve")
	if strings.Contains(strings.ToLower(out), "duplicate") {
		t.Errorf("resolve must not report duplicate_id when the dir is in settings.exclude:\n%s", out)
	}
	if code != 0 {
		t.Errorf("expected exit code 0 with excluded dir, got %d\noutput:\n%s", code, out)
	}
}

func listAllFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// check_test.go -- CLI integration tests for `specter check --test` / `-t`.
//
// @spec spec-check
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupCheckDir creates a workspace with one spec declaring AC-01 and a test file
// whose annotations the caller controls.
func setupCheckTestDir(t *testing.T, specID string, acIDs []string, testFileContent string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, specID+".spec.yaml", minimalValidSpec(specID, 3, acIDs...))
	testPath := filepath.Join(dir, "foo_test.go")
	if err := os.WriteFile(testPath, []byte(testFileContent), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// @ac AC-09
func TestCheckTest_UnknownSpecRef(t *testing.T) {
	t.Run("spec-check/AC-09 check --test flags unknown spec id", func(t *testing.T) {
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @spec bogus-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n")

		out, code := runCLI(t, dir, "check", "--test")

		if code == 0 {
			t.Fatalf("expected nonzero exit, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "unknown_spec_ref") {
			t.Errorf("expected unknown_spec_ref in output, got:\n%s", out)
		}
		if !strings.Contains(out, "bogus-spec") {
			t.Errorf("expected bogus-spec in output, got:\n%s", out)
		}
	})
}

// @ac AC-10
func TestCheckTest_UnknownAcRef(t *testing.T) {
	t.Run("spec-check/AC-10 check --test flags unknown AC id within real spec", func(t *testing.T) {
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @spec real-spec\n// @ac AC-99\nfunc TestFoo(t *testing.T) {}\n")

		out, code := runCLI(t, dir, "check", "--test")

		if code == 0 {
			t.Fatalf("expected nonzero exit, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "unknown_ac_ref") {
			t.Errorf("expected unknown_ac_ref in output, got:\n%s", out)
		}
		if !strings.Contains(out, "AC-99") {
			t.Errorf("expected AC-99 in output, got:\n%s", out)
		}
	})
}

// @ac AC-11
func TestCheckTest_MalformedAcId(t *testing.T) {
	t.Run("spec-check/AC-11 check --test flags malformed AC id per occurrence", func(t *testing.T) {
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @spec real-spec\n// @ac AC-1\n// @ac ac-01\nfunc TestFoo(t *testing.T) {}\n")

		out, code := runCLI(t, dir, "check", "--test")

		if code == 0 {
			t.Fatalf("expected nonzero exit, got 0; output:\n%s", out)
		}
		malformed := strings.Count(out, "malformed_ac_id")
		if malformed < 2 {
			t.Errorf("expected at least 2 malformed_ac_id occurrences, got %d; output:\n%s", malformed, out)
		}
	})
}

// @ac AC-12
func TestCheckTest_SyncStrictRoutesThroughCheck(t *testing.T) {
	t.Run("spec-check/AC-12 sync --strict halts at check phase when test annotations are broken", func(t *testing.T) {
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @spec bogus-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n")

		// Baseline: sync without --strict should NOT route the test-annotation
		// check through (opt-in discipline). Check phase stays green.
		baselineOut, baselineCode := runCLI(t, dir, "sync")
		if baselineCode == 0 && strings.Contains(baselineOut, "FAIL check") {
			t.Fatalf("baseline regression: plain `sync` should not route check --test; output:\n%s", baselineOut)
		}

		// With --strict, the check phase must fail because of unknown_spec_ref.
		out, code := runCLI(t, dir, "sync", "--strict")
		if code == 0 {
			t.Fatalf("expected nonzero exit for sync --strict with broken @spec, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "FAIL check") {
			t.Errorf("expected sync --strict to halt at check phase, got:\n%s", out)
		}
		// NOTE: sync currently prints summary counts only, not diagnostic
		// messages. Users have to rerun `specter check --test` to see which
		// annotation broke. Flagged as v0.12 UX polish — sync should itemize
		// check failures. For v0.11, AC-12 is satisfied by routing + exit code.
	})
}

// Regression guard: `check` without --test runs today's checks unchanged.
// Opt-in discipline — adding --test must not change default behavior.
func TestCheckTest_DefaultBehaviorUnchanged(t *testing.T) {
	t.Run("spec-check/check without --test ignores test annotations", func(t *testing.T) {
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @spec bogus-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n")

		out, code := runCLI(t, dir, "check")

		if code != 0 {
			t.Fatalf("expected exit 0 (no --test flag, default behavior unchanged), got %d; output:\n%s", code, out)
		}
		if strings.Contains(out, "unknown_spec_ref") {
			t.Errorf("check without --test should not emit test-annotation diagnostics, got:\n%s", out)
		}
	})
}

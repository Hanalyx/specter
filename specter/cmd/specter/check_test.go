// check_test.go -- CLI integration tests for `specter check --test` / `-t`.
//
// @spec spec-check
package main

import (
	"bytes"
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

// spec-check C-10 wiring test — v0.13 F3.
//
// CheckUnreachableAnnotations exists in internal/checker/ with full
// unit coverage. v0.13 pre-release smoke testing caught that the
// function was never invoked from cmd/specter/ — the headline
// diagnostic was dead code from the user's perspective. This test
// is the regression guard: a `check --test` invocation over a test
// file whose @ac is unreachable (no spec-id/AC-NN token in the test
// title, no runtime print) MUST emit the unreachable_annotation
// diagnostic.
//
// Strictness routing per C-10: under default `threshold` strictness,
// severity is warning (exit code remains 0).
//
// @ac AC-13
func TestCheckTest_UnreachableAnnotationFiresFromCLI(t *testing.T) {
	t.Run("spec-check/AC-13 check --test surfaces unreachable_annotation diagnostic", func(t *testing.T) {
		// Test function with @ac AC-01 but no runner-visible
		// spec-id/AC-01 token in the subtest name AND no runtime print
		// of the annotation. Unreachable per C-10.
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"package foo\n\nimport \"testing\"\n\n// @spec real-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {\n}\n")

		out, _ := runCLI(t, dir, "check", "--test")

		if !strings.Contains(out, "unreachable_annotation") {
			t.Errorf("expected unreachable_annotation diagnostic in CLI output; got:\n%s", out)
		}
		// Must name the AC so the operator can locate it.
		if !strings.Contains(out, "AC-01") {
			t.Errorf("expected diagnostic to name AC-01; got:\n%s", out)
		}
	})
}

// spec-check C-11 wiring test — the @reachable manual marker MUST
// suppress unreachable_annotation diagnostics for every @ac in the
// file. Companion to the C-10 wiring test above; verifies the
// off-switch is reachable end-to-end through the CLI, not just
// inside the pure function.
//
// @ac AC-15
func TestCheckTest_ReachableManualSuppressesFromCLI(t *testing.T) {
	t.Run("spec-check/AC-15 // @reachable manual suppresses CLI unreachable_annotation diagnostic", func(t *testing.T) {
		// Same scenario as the C-10 test, but with the file-level
		// // @reachable manual marker prepended.
		dir := setupCheckTestDir(t, "real-spec", []string{"AC-01"},
			"// @reachable manual\npackage foo\n\nimport \"testing\"\n\n// @spec real-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {\n}\n")

		out, code := runCLI(t, dir, "check", "--test")

		if strings.Contains(out, "unreachable_annotation") {
			t.Errorf("@reachable manual must suppress unreachable_annotation diagnostic; got:\n%s", out)
		}
		// File-level marker should also suppress _unknown, so the
		// run should be clean.
		if code != 0 {
			t.Errorf("expected exit 0 when off-switch suppresses all unreachable diagnostics, got %d:\n%s", code, out)
		}
	})
}

// spec-check C-12 / AC-20 — v0.13 H5 security fix.
//
// A test file larger than MaxTestFileBytes (4 MiB) discovered under
// `check --test` MUST be skipped — the file is NOT read into memory,
// no diagnostics produced for any @ac it contains, a stderr warning
// names the file, and the run continues with remaining files (exit 0
// on the skip alone). Closes the H5 audit finding: unreachable
// scanner read every test file unbounded, exposing OOM via a multi-
// GiB malicious test file.
//
// @ac AC-20
func TestCheckTest_OversizedTestFileSkipped(t *testing.T) {
	t.Run("spec-check/AC-20 check --test skips test file larger than MaxTestFileBytes", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "x.spec.yaml", minimalValidSpec("x-spec", 3, "AC-01"))

		// 4 MiB + 1 byte — one past the cap. Content is arbitrary
		// non-Go bytes; size check must fire before any parse runs.
		const cap = 4 << 20
		body := bytes.Repeat([]byte("x"), cap+1)
		oversized := filepath.Join(dir, "huge_test.go")
		if err := os.WriteFile(oversized, body, 0644); err != nil {
			t.Fatal(err)
		}

		// A small, valid test file with an @ac annotation. This file
		// MUST still be scanned (skip applies per-file, not whole run).
		small := "package foo\n\nimport \"testing\"\n\n// @spec x-spec\n// @ac AC-01\nfunc TestSmall(t *testing.T) {\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "small_test.go"), []byte(small), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "check", "--test")

		// The skip itself must not exit non-zero — other tests still run.
		// The small file's @ac AC-01 is unreachable (no spec-id/AC-NN in
		// title) so a warning is expected; warnings don't drive exit
		// under default threshold strictness.
		if code != 0 {
			t.Errorf("oversized test file skip should not drive non-zero exit (got %d); output:\n%s", code, out)
		}

		// Warning must name the offending file and contain the skip
		// reason. Per AC-20 the message contains `exceeds` and
		// `skipping` (operator vocabulary for "this file was not
		// scanned").
		if !strings.Contains(out, "huge_test.go") {
			t.Errorf("expected warning to name the oversized file `huge_test.go`, got:\n%s", out)
		}
		if !strings.Contains(out, "exceeds") || !strings.Contains(out, "skipping") {
			t.Errorf("expected warning to contain `exceeds` and `skipping`, got:\n%s", out)
		}

		// The small file's annotation MUST have been scanned — confirm
		// by checking that the unreachable_annotation diagnostic fired
		// for AC-01 (it has source-only @ac with no runner-visible
		// token in TestSmall's name).
		if !strings.Contains(out, "unreachable_annotation") {
			t.Errorf("expected small_test.go to still be scanned (unreachable_annotation diagnostic for AC-01), got:\n%s", out)
		}
	})

	t.Run("spec-check/AC-20 file at exactly the cap is scanned", func(t *testing.T) {
		// Boundary condition: len == cap (4 MiB) must scan
		// successfully. Use a valid Go test file padded with comments
		// up to (but not over) the cap.
		dir := t.TempDir()
		writeSpec(t, dir, "x.spec.yaml", minimalValidSpec("x-spec", 3, "AC-01"))

		header := "package foo\n\nimport \"testing\"\n\n// @spec x-spec\n// @ac AC-01\nfunc TestBoundary(t *testing.T) {\n}\n// padding:\n"
		const cap = 4 << 20
		padding := bytes.Repeat([]byte("/"), cap-len(header))
		body := append([]byte(header), padding...)
		if len(body) > cap {
			t.Fatalf("padding overshot the cap: %d > %d", len(body), cap)
		}
		if err := os.WriteFile(filepath.Join(dir, "boundary_test.go"), body, 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "check", "--test")

		// At-cap file must NOT be skipped. We verify by checking that
		// no `exceeds` warning was emitted for boundary_test.go.
		if strings.Contains(out, "boundary_test.go") && strings.Contains(out, "exceeds") {
			t.Errorf("at-cap file (4 MiB) must NOT be skipped, got:\n%s", out)
		}
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

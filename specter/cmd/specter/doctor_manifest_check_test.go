// doctor_manifest_check_test.go -- CLI integration tests for spec-doctor
// 1.10.0 C-18: the manifest health check may not report PASS for a file the
// same run rejects (AC-32 and AC-33).
//
// C-18 is a statement about one line of doctor's report and about which lines
// print after it, so both criteria run the binary and read its report.
//
// @spec spec-doctor
package main

import (
	"strings"
	"testing"
)

// doctorCheckLine returns the report line for one named check, or the empty
// string when that check did not print. The name is matched as the line's
// first field, so a check name mentioned inside another line's message is not
// mistaken for the line itself.
func doctorCheckLine(stdout, name string) string {
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name && strings.HasPrefix(fields[1], "[") {
			return line
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// AC-32 -- a rejected manifest reports FAIL and the run continues
// ---------------------------------------------------------------------------

// The workspace is otherwise healthy, so the manifest is the only thing that
// can produce a failure. The criterion has three separate claims and this
// test keeps them separate: the verdict on the check line, the reason on that
// same line, and the four checks below it still printing.
//
// The check-lines assertion is what makes this more than a wording change. A
// fix that changes PASS to FAIL but leaves the run stopping after the
// annotations check still leaves the operator without a coverage verdict.
//
// @ac AC-32
func TestDoctorManifestCheck_RejectedManifestReportsFail(t *testing.T) {
	t.Run("spec-doctor/AC-32 a rejected specter.yaml reports FAIL and the remaining checks still print", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  test_glob: 'tests/**/*.go'\n")
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01"})

		stdout, stderr, code := runCLISplit(t, dir, "doctor")

		manifestLine := doctorCheckLine(stdout, "manifest")
		if manifestLine == "" {
			t.Fatalf("doctor must print a manifest check line;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if !strings.Contains(manifestLine, "FAIL") {
			t.Errorf("the manifest check must report FAIL, got: %s", manifestLine)
		}
		if !strings.Contains(manifestLine, `unknown settings key "settings.test_glob"`) {
			t.Errorf("the manifest check line must name the parser's reason, got: %s", manifestLine)
		}

		for _, name := range []string{"manifest", "spec-files", "parse", "annotations", "coverage"} {
			if doctorCheckLine(stdout, name) == "" {
				t.Errorf("doctor must print the %s check line;\nstdout:\n%s", name, stdout)
			}
		}

		if code == 0 {
			t.Errorf("expected a non-zero exit, got 0;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-33 -- a valid manifest still reports PASS
// ---------------------------------------------------------------------------

// The control for AC-32. Its manifest is valid and its workspace covers every
// declared criterion, so nothing in the run has a reason to fail. A fix that
// reports FAIL whenever a manifest is present, or that treats a present
// manifest as a reason to exit non-zero, is caught here and nowhere else.
//
// @ac AC-33
func TestDoctorManifestCheck_ValidManifestReportsPass(t *testing.T) {
	t.Run("spec-doctor/AC-33 a valid specter.yaml reports PASS and the healthy workspace exits 0", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  strictness: annotation\n")
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01", "AC-02"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})

		stdout, stderr, code := runCLISplit(t, dir, "doctor")

		manifestLine := doctorCheckLine(stdout, "manifest")
		if manifestLine == "" {
			t.Fatalf("doctor must print a manifest check line;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if !strings.Contains(manifestLine, "PASS") {
			t.Errorf("the manifest check must report PASS for a valid manifest, got: %s", manifestLine)
		}
		if !strings.Contains(manifestLine, "specter.yaml found at") {
			t.Errorf("the manifest check line must name the file it read, got: %s", manifestLine)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// explain_test.go -- CLI integration tests for specter explain.
//
// @spec spec-explain
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupExplainDir creates a temp dir with one spec and optional annotated test files.
func setupExplainDir(t *testing.T, coveredACs []string, testFileExt string) string {
	t.Helper()
	dir := t.TempDir()

	// Write a spec with AC-01 and AC-02
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01", "AC-02"))

	if len(coveredACs) > 0 && testFileExt != "" {
		// Write a test file annotating the covered ACs
		var annotation string
		switch {
		case strings.HasSuffix(testFileExt, ".py"):
			annotation = "# @spec my-spec\n"
			for _, ac := range coveredACs {
				annotation += fmt.Sprintf("# @ac %s\n", ac)
			}
			annotation += "def test_my_spec(): pass\n"
		default:
			annotation = "// @spec my-spec\n"
			for _, ac := range coveredACs {
				annotation += fmt.Sprintf("// @ac %s\n", ac)
			}
			annotation += "func TestMySpec(t *testing.T) {}\n"
		}
		testFile := filepath.Join(dir, "my_spec_test"+testFileExt)
		if err := os.WriteFile(testFile, []byte(annotation), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// @ac AC-01
func TestExplain_ListMode_ShowsCoveredAndUncovered(t *testing.T) {
	t.Run("spec-explain/AC-01 list mode shows covered and uncovered", func(t *testing.T) {
		dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
		out, _ := runCLI(t, dir, "explain", "my-spec")

		if !strings.Contains(out, "COVERED") {
			t.Errorf("expected COVERED label in list output, got:\n%s", out)
		}
		if !strings.Contains(out, "UNCOVERED") {
			t.Errorf("expected UNCOVERED label in list output, got:\n%s", out)
		}
		if !strings.Contains(out, "AC-01") {
			t.Errorf("expected AC-01 in output, got:\n%s", out)
		}
		if !strings.Contains(out, "AC-02") {
			t.Errorf("expected AC-02 in output, got:\n%s", out)
		}
	})
}

// @ac AC-02
func TestExplain_DetailMode_UncoveredAC_ShowsAnnotationExample(t *testing.T) {
	t.Run("spec-explain/AC-02 detail mode uncovered ac shows annotation example", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")
		out, _ := runCLI(t, dir, "explain", "my-spec:AC-01")

		if !strings.Contains(out, "// @spec my-spec") {
			t.Errorf("expected // @spec my-spec in annotation example, got:\n%s", out)
		}
		if !strings.Contains(out, "// @ac AC-01") {
			t.Errorf("expected // @ac AC-01 in annotation example, got:\n%s", out)
		}
	})
}

// @ac AC-03
func TestExplain_PythonTestFiles_ShowsPythonSyntax(t *testing.T) {
	t.Run("spec-explain/AC-03 python test files shows python syntax", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")
		// Write a Python test file with the naming pattern that discoverTestFiles finds (_test.py)
		if err := os.WriteFile(filepath.Join(dir, "user_test.py"), []byte("# empty\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out, _ := runCLI(t, dir, "explain", "my-spec:AC-01")

		if !strings.Contains(out, "# @spec") {
			t.Errorf("expected Python-style annotation (# @spec) in output, got:\n%s", out)
		}
	})
}

// @ac AC-04
func TestExplain_CoveredAC_ShowsFile_NotAnnotationExample(t *testing.T) {
	t.Run("spec-explain/AC-04 covered ac shows file not annotation example", func(t *testing.T) {
		dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
		out, _ := runCLI(t, dir, "explain", "my-spec:AC-01")

		if !strings.Contains(out, "COVERED") {
			t.Errorf("expected COVERED in output, got:\n%s", out)
		}
		if !strings.Contains(out, "Covered in:") {
			t.Errorf("expected 'Covered in:' section, got:\n%s", out)
		}
		// Must NOT show "To cover this AC" annotation example for covered ACs
		if strings.Contains(out, "To cover this AC") {
			t.Errorf("must not show annotation example for covered AC, got:\n%s", out)
		}
	})
}

// @ac AC-05
func TestExplain_UnknownSpec_ExitsOneWithNotFound(t *testing.T) {
	t.Run("spec-explain/AC-05 unknown spec exits one with not found", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")
		out, code := runCLI(t, dir, "explain", "does-not-exist")

		if code != 1 {
			t.Errorf("expected exit code 1 for unknown spec, got %d", code)
		}
		if !strings.Contains(strings.ToLower(out), "not found") {
			t.Errorf("expected 'not found' in error output, got:\n%s", out)
		}
	})
}

// @ac AC-06
func TestExplain_OutputIncludesTestFileCount(t *testing.T) {
	t.Run("spec-explain/AC-06 output includes test file count", func(t *testing.T) {
		dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
		out, _ := runCLI(t, dir, "explain", "my-spec")

		if !strings.Contains(out, "test file") {
			t.Errorf("expected test file count in output, got:\n%s", out)
		}
	})
}

// @ac AC-11
// GH #77: when the project has Python test files, the uncovered-AC
// example must teach the dual-channel pattern (source comments +
// pytest.mark.spec decorator + conftest autouse fixture + pytest.ini),
// not just the source-comment-only pattern. Source comments alone don't
// satisfy coverage --strict because pytest's JUnit doesn't include them.
func TestExplain_DetailMode_PythonProject_ShowsDualChannelPattern(t *testing.T) {
	t.Run("spec-explain/AC-11 python project uncovered ac shows dual-channel pattern", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")
		// Python test file present → triggers Python-language detection.
		if err := os.WriteFile(filepath.Join(dir, "tests_user_test.py"), []byte("# empty\n"), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "explain", "my-spec:AC-02")

		// Must include the pytest.mark.spec decorator (Convention A for pytest).
		if !strings.Contains(out, "@pytest.mark.spec") {
			t.Errorf("expected @pytest.mark.spec decorator in Python example, got:\n%s", out)
		}
		// Must include the conftest autouse fixture pattern.
		if !strings.Contains(out, "autouse=True") {
			t.Errorf("expected conftest autouse fixture (autouse=True) in Python example, got:\n%s", out)
		}
		// Must include the pytest.ini block with junit_logging = system-out.
		if !strings.Contains(out, "junit_logging = system-out") {
			t.Errorf("expected pytest.ini block with junit_logging = system-out, got:\n%s", out)
		}
		// Must still include the source-comment annotation.
		if !strings.Contains(out, "# @spec my-spec") {
			t.Errorf("expected source-comment annotation # @spec my-spec, got:\n%s", out)
		}
		// Go-only example must NOT be shown when only Python test files are detected.
		if strings.Contains(out, "// @spec my-spec") {
			t.Errorf("Go example must not be shown for Python-only test discovery, got:\n%s", out)
		}
	})
}

// @ac AC-12
// Regression guard: Go-only test files must continue to show the t.Run
// example without any Python pytest-mark / conftest boilerplate.
func TestExplain_DetailMode_GoOnlyProject_DoesNotShowPythonPattern(t *testing.T) {
	t.Run("spec-explain/AC-12 go only project does not show python pattern", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")
		// Go test file present → triggers Go-language detection only.
		if err := os.WriteFile(filepath.Join(dir, "user_test.go"), []byte("package x\n"), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "explain", "my-spec:AC-02")

		// Go example must be shown (Convention A: t.Run).
		if !strings.Contains(out, "// @spec my-spec") {
			t.Errorf("expected Go-style annotation comment in output, got:\n%s", out)
		}
		// Python pytest-mark / conftest patterns must NOT be shown.
		if strings.Contains(out, "@pytest.mark.spec") {
			t.Errorf("Python @pytest.mark.spec must not appear for Go-only test discovery, got:\n%s", out)
		}
		if strings.Contains(out, "autouse=True") {
			t.Errorf("Python autouse fixture must not appear for Go-only test discovery, got:\n%s", out)
		}
		if strings.Contains(out, "junit_logging = system-out") {
			t.Errorf("pytest.ini block must not appear for Go-only test discovery, got:\n%s", out)
		}
	})
}

// @ac AC-13
// Empty test discovery → fall back to a generic example, but the dual-
// channel requirement must be noted explicitly so a developer doesn't
// adopt source-only annotations and silently fail coverage --strict.
func TestExplain_DetailMode_NoTestFiles_NotesDualChannelRequirement(t *testing.T) {
	t.Run("spec-explain/AC-13 empty test discovery notes dual-channel requirement", func(t *testing.T) {
		dir := setupExplainDir(t, nil, "")

		out, _ := runCLI(t, dir, "explain", "my-spec:AC-02")

		// Generic Go example is still shown (matches detectAnnotationLanguages
		// fallback to "Go / generic" on empty discovery).
		if !strings.Contains(out, "@spec my-spec") {
			t.Errorf("expected at least the source annotation @spec my-spec, got:\n%s", out)
		}
		// Must explicitly mention coverage --strict so the developer knows
		// source comments alone don't satisfy the strict gate.
		if !strings.Contains(out, "coverage --strict") {
			t.Errorf("expected dual-channel note mentioning coverage --strict, got:\n%s", out)
		}
	})
}

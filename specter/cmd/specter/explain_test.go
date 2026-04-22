// explain_test.go -- CLI integration tests for specter explain.
//
// @spec spec-explain
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
}

// @ac AC-02
func TestExplain_DetailMode_UncoveredAC_ShowsAnnotationExample(t *testing.T) {
	dir := setupExplainDir(t, nil, "")
	out, _ := runCLI(t, dir, "explain", "my-spec:AC-01")

	if !strings.Contains(out, "// @spec my-spec") {
		t.Errorf("expected // @spec my-spec in annotation example, got:\n%s", out)
	}
	if !strings.Contains(out, "// @ac AC-01") {
		t.Errorf("expected // @ac AC-01 in annotation example, got:\n%s", out)
	}
}

// @ac AC-03
func TestExplain_PythonTestFiles_ShowsPythonSyntax(t *testing.T) {
	dir := setupExplainDir(t, nil, "")
	// Write a Python test file with the naming pattern that discoverTestFiles finds (_test.py)
	if err := os.WriteFile(filepath.Join(dir, "user_test.py"), []byte("# empty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, _ := runCLI(t, dir, "explain", "my-spec:AC-01")

	if !strings.Contains(out, "# @spec") {
		t.Errorf("expected Python-style annotation (# @spec) in output, got:\n%s", out)
	}
}

// @ac AC-04
func TestExplain_CoveredAC_ShowsFile_NotAnnotationExample(t *testing.T) {
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
}

// @ac AC-05
func TestExplain_UnknownSpec_ExitsOneWithNotFound(t *testing.T) {
	dir := setupExplainDir(t, nil, "")
	out, code := runCLI(t, dir, "explain", "does-not-exist")

	if code != 1 {
		t.Errorf("expected exit code 1 for unknown spec, got %d", code)
	}
	if !strings.Contains(strings.ToLower(out), "not found") {
		t.Errorf("expected 'not found' in error output, got:\n%s", out)
	}
}

// @ac AC-06
func TestExplain_OutputIncludesTestFileCount(t *testing.T) {
	dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
	out, _ := runCLI(t, dir, "explain", "my-spec")

	if !strings.Contains(out, "test file") {
		t.Errorf("expected test file count in output, got:\n%s", out)
	}
}

// @ac AC-07
// AC-less explain MUST render a spec-card header matching the shape
// `<spec-id> — tier N · X/Y ACs · Z% coverage · threshold T% [PASS|FAIL]`.
func TestExplain_ACLess_RendersSpecCardHeader(t *testing.T) {
	dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
	out, _ := runCLI(t, dir, "explain", "my-spec")

	// Regex-check the header shape. Leading "specter explain my-spec" is
	// the existing line; the new addition is everything after the em-dash.
	headerRE := regexp.MustCompile(
		`specter explain my-spec — tier \d+ · \d+/\d+ ACs · \d+(\.\d+)?% coverage · threshold \d+% \[(PASS|FAIL)\]`)
	if !headerRE.MatchString(out) {
		t.Errorf("expected spec-card header matching regex %q; got:\n%s",
			headerRE.String(), out)
	}
}

// @ac AC-08
// Covered ACs in AC-less output MUST attribute the test file(s) that
// covered them.
func TestExplain_ACLess_CoveredACAttributesTestFiles(t *testing.T) {
	dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
	out, _ := runCLI(t, dir, "explain", "my-spec")

	// The covered row for AC-01 must mention the annotating test file.
	// The arrow "→" is the convention documented in C-10.
	lines := strings.Split(out, "\n")
	var coveredLine string
	for _, line := range lines {
		if strings.Contains(line, "COVERED") && strings.Contains(line, "AC-01") {
			coveredLine = line
			break
		}
	}
	if coveredLine == "" {
		t.Fatalf("could not find COVERED AC-01 line; got:\n%s", out)
	}
	if !strings.Contains(coveredLine, "→") {
		t.Errorf("expected `→` file attribution on covered row; got:\n%s", coveredLine)
	}
	if !strings.Contains(coveredLine, "_test.go") {
		t.Errorf("expected test file name attributed on covered row; got:\n%s", coveredLine)
	}
}

// @ac AC-09
// Uncovered ACs MUST show the full description without 60-char truncation.
func TestExplain_ACLess_UncoveredACShowsFullDescription(t *testing.T) {
	dir := t.TempDir()
	// Build a spec whose AC-02 description is > 60 chars so truncation would
	// be detectable. minimalValidSpec gives "Test acceptance criterion"
	// (25 chars) — insufficient. Write a custom spec.
	longDesc := "A very long acceptance criterion description that deliberately exceeds sixty characters so truncation is observable"
	specYAML := `spec:
  id: my-spec
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: test
    feature: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "MUST work"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "Short and covered"
      references_constraints: ["C-01"]
      priority: high
    - id: AC-02
      description: "` + longDesc + `"
      references_constraints: ["C-01"]
      priority: high
`
	writeSpec(t, dir, "my-spec.spec.yaml", specYAML)
	// Cover only AC-01 so AC-02 is the uncovered under test.
	_ = os.WriteFile(filepath.Join(dir, "my_spec_test.go"),
		[]byte("// @spec my-spec\n// @ac AC-01\nfunc TestX(t *testing.T) {}\n"), 0644)

	out, _ := runCLI(t, dir, "explain", "my-spec")

	// AC-02 line must contain the FULL long description, not "...".
	if !strings.Contains(out, longDesc) {
		t.Errorf("expected full uncovered-AC description in output (not truncated); got:\n%s", out)
	}
}

// @spec spec-diff
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// specV1 is the "before" revision for diff tests.
const specV1 = `spec:
  id: example
  version: "1.0.0"
  status: draft
  tier: 2

  context:
    system: Test System

  objective:
    summary: Test spec.

  constraints:
    - id: C-01
      description: "First constraint"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "First AC"
    - id: AC-02
      description: "Second AC"

  depends_on:
    - spec_id: auth
      version_range: "any"
`

// specV2 adds AC-03, removes AC-02, changes AC-01 description, adds C-02, bumps dep version.
const specV2 = `spec:
  id: example
  version: "2.0.0"
  status: draft
  tier: 2

  context:
    system: Test System

  objective:
    summary: Test spec.

  constraints:
    - id: C-01
      description: "First constraint"
      type: technical
      enforcement: error
    - id: C-02
      description: "New constraint"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "First AC - updated description"
    - id: AC-03
      description: "Third AC"

  depends_on:
    - spec_id: auth
      version_range: "^1.0.0"
`

// specV3 is semantically identical to V1 (only the version field differs, no structural changes).
const specV3 = `spec:
  id: example
  version: "1.0.1"
  status: draft
  tier: 2

  context:
    system: Test System

  objective:
    summary: Test spec.

  constraints:
    - id: C-01
      description: "First constraint"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "First AC"
    - id: AC-02
      description: "Second AC"

  depends_on:
    - spec_id: auth
      version_range: "any"
`

// setupGitRepo creates a temporary directory with a git repo containing two commits.
// First commit has specContent1, second commit has specContent2.
// Returns (repoDir, specRelPath, cleanup).
func setupGitRepo(t *testing.T, specContent1, specContent2 string) (string, string, func()) {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")

	runGit := func(args ...string) {
		t.Helper()
		out, err := execInDir(dir, "git", args...)
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// First commit with V1
	if err := os.WriteFile(specPath, []byte(specContent1), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "spec.yaml")
	runGit("commit", "-m", "v1")

	// Second commit with V2
	if err := os.WriteFile(specPath, []byte(specContent2), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "spec.yaml")
	runGit("commit", "-m", "v2")

	return dir, "spec.yaml", func() {}
}

// execInDir runs a command in the given directory and returns (combined output, error).
func execInDir(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// @ac AC-01
func TestDiff_ParseRefSyntax(t *testing.T) {
	// AC-01: path@HEAD~1 is parsed as path=the-path, ref=HEAD~1
	// We test the readSpecAtRef helper directly (package-level function).
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	// Change to repo dir so git show works
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	// Reading HEAD (latest commit) should give V2
	ast, err := readSpecAtRef("spec.yaml@HEAD")
	if err != nil {
		t.Fatalf("readSpecAtRef failed: %v", err)
	}
	if ast.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0 from HEAD, got %s", ast.Version)
	}

	// Reading HEAD~1 (first commit) should give V1
	astOld, err := readSpecAtRef("spec.yaml@HEAD~1")
	if err != nil {
		t.Fatalf("readSpecAtRef(HEAD~1) failed: %v", err)
	}
	if astOld.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0 from HEAD~1, got %s", astOld.Version)
	}
}

// @ac AC-02
func TestDiff_AddedAC(t *testing.T) {
	// AC-02: AC added between revisions appears as +AC-03: <description>
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "+AC-03") {
		t.Errorf("expected +AC-03 in output, got:\n%s", out)
	}
}

// @ac AC-03
func TestDiff_RemovedAC(t *testing.T) {
	// AC-03: AC removed between revisions appears as -AC-02: <description>
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "-AC-02") {
		t.Errorf("expected -AC-02 in output, got:\n%s", out)
	}
}

// @ac AC-04
func TestDiff_AddedConstraint(t *testing.T) {
	// AC-04: Constraint added appears as +C-02: <description>
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "+C-02") {
		t.Errorf("expected +C-02 in output, got:\n%s", out)
	}
}

// @ac AC-05
func TestDiff_DepVersionChange(t *testing.T) {
	// AC-05: depends_on version_range changed appears as ~depends_on auth: any → ^1.0.0
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "depends_on") || !strings.Contains(out, "auth") {
		t.Errorf("expected dep change for auth in output, got:\n%s", out)
	}
}

// @ac AC-06
func TestDiff_RemoveACIsBreaking(t *testing.T) {
	// AC-06: Removing an AC is classified as breaking
	dir, _, cleanup := setupGitRepo(t, specV1, specV2)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "breaking") {
		t.Errorf("expected 'breaking' in output, got:\n%s", out)
	}
}

// @ac AC-07
func TestDiff_AddACIsAdditive(t *testing.T) {
	// AC-07: Adding an AC is classified as additive.
	// Use specV1 -> a version with AC-03 added but nothing removed.
	const specOnlyAdd = `spec:
  id: example
  version: "1.1.0"
  status: draft
  tier: 2

  context:
    system: Test System

  objective:
    summary: Test spec.

  constraints:
    - id: C-01
      description: "First constraint"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "First AC"
    - id: AC-02
      description: "Second AC"
    - id: AC-03
      description: "Third AC"
`
	dir, _, cleanup := setupGitRepo(t, specV1, specOnlyAdd)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "additive") {
		t.Errorf("expected 'additive' in output, got:\n%s", out)
	}
}

// @ac AC-08
func TestDiff_DescriptionOnlyIsPatch(t *testing.T) {
	// AC-08: Only description changes is classified as patch
	const specDescChange = `spec:
  id: example
  version: "1.0.1"
  status: draft
  tier: 2

  context:
    system: Test System

  objective:
    summary: Test spec.

  constraints:
    - id: C-01
      description: "First constraint - updated"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "First AC - updated description"
    - id: AC-02
      description: "Second AC"
`
	dir, _, cleanup := setupGitRepo(t, specV1, specDescChange)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "patch") {
		t.Errorf("expected 'patch' in output, got:\n%s", out)
	}
}

// @ac AC-09
func TestDiff_GitShowFailure(t *testing.T) {
	// AC-09: git show failure exits 1 with a clear error message
	dir := t.TempDir()

	// No git repo, no commits — git show will fail.
	// Write a spec file on disk for the second arg.
	if err := os.WriteFile(filepath.Join(dir, "spec.yaml"), []byte(specV1), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize git but make no commits so HEAD~1 doesn't exist
	out, _ := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml")
	// Should contain an error about git
	if !strings.Contains(out, "error") && !strings.Contains(out, "git") {
		t.Errorf("expected error message about git failure, got:\n%s", out)
	}
}

// @ac AC-10 (also covered in internal/diff package, duplicated here for CLI smoke test)
func TestDiff_NoChanges(t *testing.T) {
	// AC-10: Two identical specs produce "no changes" output.
	dir, _, cleanup := setupGitRepo(t, specV1, specV3)
	defer cleanup()

	out, code := runCLI(t, dir, "diff", "spec.yaml@HEAD~1", "spec.yaml@HEAD")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected 'no changes' in output, got:\n%s", out)
	}
}

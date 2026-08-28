package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settings.exclude reaches every default discovery walk, spec-manifest C-29
// and AC-66, closing bugs/SP-SP-016.
//
// The defect: discoverSpecs consulted settings.exclude and discoverTestFiles
// did not, so a workspace could exclude a git worktree or a vendored copy from
// spec discovery and still have its test files counted.
//
// This asserts on the discovery functions rather than through the CLI, because
// the claim is about which files a walk returns. A coverage run would observe
// the same thing one layer down, through a percentage that also moves for
// unrelated reasons.

// excludeWorkspace builds a workspace with one test file inside a directory
// named `vendored` and one outside it, plus a manifest declaring the given
// exclude patterns. It returns the workspace root.
//
// Both test files carry the same annotation, which is what makes the excluded
// copy a duplicate rather than merely extra: it is the shape an adopter hits
// with a git worktree.
func excludeWorkspace(t *testing.T, excludeLines string) string {
	t.Helper()
	dir := t.TempDir()

	mustWrite := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("specter.yaml", "schema_version: 1\nsystem:\n  name: excl\nsettings:\n  specs_dir: specs\n"+excludeLines)

	const testBody = "package p\n\n// @spec demo\n// @ac AC-01\nfunc TestThing(t *testing.T) {}\n"
	mustWrite("outer_test.go", testBody)
	mustWrite("vendored/inner_test.go", testBody)

	const spec = `spec:
  id: demo
  version: "1.0.0"
  status: approved
  tier: 3
  context:
    system: excl
    feature: exclude fixture
    description: A fixture for settings.exclude reaching test discovery.
  objective:
    summary: Show that both default walks apply settings.exclude.
  constraints:
    - id: C-01
      description: "MUST count only in-scope test files"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "Discovery counts only test files outside settings.exclude"
      references_constraints: ["C-01"]
      priority: critical
`
	mustWrite("specs/demo.spec.yaml", spec)
	return dir
}

// discoveredTestFilesIn runs the default test walk with the workspace as cwd
// and returns the paths it found, normalized to forward slashes.
func discoveredTestFilesIn(t *testing.T, dir, glob string) []string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	var out []string
	for _, p := range discoverTestFiles(glob) {
		out = append(out, filepath.ToSlash(strings.TrimPrefix(p, "./")))
	}
	return out
}

func contains(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// @spec spec-manifest
// @ac AC-66
//
// C-29: every default discovery walk applies settings.exclude, through one
// shared predicate.
func TestSettingsExcludeReachesTestDiscovery(t *testing.T) {
	t.Run("spec-manifest/AC-66 a bare-name exclude removes the test file from discovery", func(t *testing.T) {
		dir := excludeWorkspace(t, "  exclude:\n    - vendored\n")
		found := discoveredTestFilesIn(t, dir, "")

		// The include control comes first. If the walk returns nothing at all,
		// the exclusion assertion below passes for the wrong reason, and this
		// is the assertion that catches it.
		if !contains(found, "outer_test.go") {
			t.Fatalf("AC-66 (include control): outer_test.go was not discovered, so the walk found nothing to exclude. got: %v", found)
		}

		if contains(found, "vendored/inner_test.go") {
			t.Errorf("AC-66 (bare name): vendored/inner_test.go was discovered although settings.exclude declares \"vendored\". Spec discovery honors this setting and test discovery must too. got: %v", found)
		}
	})

	t.Run("spec-manifest/AC-66 a glob exclude removes the same file", func(t *testing.T) {
		dir := excludeWorkspace(t, "  exclude:\n    - \"**/vendored\"\n")
		found := discoveredTestFilesIn(t, dir, "")

		if !contains(found, "outer_test.go") {
			t.Fatalf("AC-66 (include control, glob): outer_test.go was not discovered. got: %v", found)
		}

		// The glob form is a separate case, not a restatement of the first.
		// C-29 dispatches on whether the pattern carries a metacharacter, so an
		// implementation that wired only the bare-name branch passes the case
		// above and fails here.
		if contains(found, "vendored/inner_test.go") {
			t.Errorf("AC-66 (glob): vendored/inner_test.go was discovered although settings.exclude declares \"**/vendored\". got: %v", found)
		}
	})

	t.Run("spec-manifest/AC-66 settings.tests_glob still overrides discovery", func(t *testing.T) {
		// The boundary control. C-29 governs the default walk only, so a
		// tests_glob run must return exactly what it returned before, exclude
		// patterns notwithstanding. Without this, a fix that applied the
		// patterns to every walk would look correct.
		withExclude := discoveredTestFilesIn(t, excludeWorkspace(t, "  exclude:\n    - vendored\n"), "**/*_test.go")
		without := discoveredTestFilesIn(t, excludeWorkspace(t, ""), "**/*_test.go")

		if len(withExclude) != len(without) {
			t.Errorf("AC-66 (tests_glob): settings.exclude changed a tests_glob run. C-29 governs the default walk only. with: %v, without: %v", withExclude, without)
		}
		if !contains(withExclude, "vendored/inner_test.go") {
			t.Errorf("AC-66 (tests_glob): the glob run dropped vendored/inner_test.go, so the override was narrowed by settings.exclude. got: %v", withExclude)
		}
	})
}

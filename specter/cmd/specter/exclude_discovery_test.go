package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	// A second spec inside the excluded directory. AC-66 declares one spec on
	// each side, because the spec walk has to be observed too: a fix that
	// wired only the test walk leaves this one discovered.
	mustWrite("vendored/inner.spec.yaml", strings.Replace(spec, "id: demo", "id: demo-inner", 1))
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

// discoveredSpecsIn runs the default spec walk with the workspace as cwd.
func discoveredSpecsIn(t *testing.T, dir string) []string {
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
	for _, p := range discoverSpecs() {
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

		// The spec walk, observed rather than assumed. C-29 covers both default
		// walks, and a fix that wired only the test walk would pass everything
		// above while leaving this one wrong.
		specs := discoveredSpecsIn(t, dir)
		if !contains(specs, "specs/demo.spec.yaml") {
			t.Fatalf("AC-66 (spec include control): specs/demo.spec.yaml was not discovered, so the spec walk found nothing to exclude. got: %v", specs)
		}
		if contains(specs, "vendored/inner.spec.yaml") {
			t.Errorf("AC-66 (spec exclusion): vendored/inner.spec.yaml was discovered although settings.exclude declares \"vendored\". got: %v", specs)
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

// @spec spec-manifest
// @ac AC-66
//
// C-29's shared-owner half, asserted structurally because no runtime
// observation can see it.
//
// Inlining an equivalent pattern loop into discoverSpecs produces identical
// output for every input, so every behavioral assertion above stays green while
// the second copy of exclusion policy is back. That second copy is the defect
// C-29 exists to prevent, and bugs/SP-SP-016 is what it looks like once the two
// copies drift.
func TestExclusionPolicyHasOneOwner(t *testing.T) {
	t.Run("spec-manifest/AC-66 both default walks call the shared predicate and only it loops over patterns", func(t *testing.T) {
		fset := token.NewFileSet()
		// Production sources only. The rule is about the implementation: the
		// helper's own unit tests call matchExcludePattern directly and are
		// supposed to, and this file reads the identifier too.
		pkg, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parsing cmd/specter failed: %v", err)
		}
		files := map[string]*ast.File{}
		for _, p := range pkg {
			for name, f := range p.Files {
				files[name] = f
			}
		}
		if len(files) == 0 {
			t.Fatal("AC-66: parsed no files, so every claim below would pass vacuously")
		}

		// callsIn reports the function names called directly inside the named
		// top-level function, and whether that function was found at all.
		callsIn := func(fn string) (map[string]bool, bool) {
			calls := map[string]bool{}
			found := false
			for _, f := range files {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok || fd.Name.Name != fn || fd.Recv != nil {
						continue
					}
					found = true
					ast.Inspect(fd, func(n ast.Node) bool {
						if ce, ok := n.(*ast.CallExpr); ok {
							if id, ok := ce.Fun.(*ast.Ident); ok {
								calls[id.Name] = true
							}
						}
						return true
					})
				}
			}
			return calls, found
		}

		for _, walk := range []string{"discoverSpecs", "discoverTestFiles"} {
			calls, found := callsIn(walk)
			if !found {
				t.Fatalf("AC-66: %s not found, so the ownership claim cannot be checked", walk)
			}
			// Positive control: the walk really was read.
			if !calls["filepath.Walk"] && len(calls) == 0 {
				t.Fatalf("AC-66: %s parsed with no calls at all, which cannot be right", walk)
			}
			if !calls["manifestExcludesDir"] {
				t.Errorf("AC-66: %s does not call manifestExcludesDir. C-29 requires both default walks to use the one shared predicate; a private loop that behaves identically is the second copy of exclusion policy this constraint forbids", walk)
			}
		}

		// The other half of ownership. Calling the predicate does not stop a
		// walk from also running its own loop, so the loop itself must live in
		// exactly one function.
		owners := map[string]bool{}
		for _, f := range files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok {
					continue
				}
				ast.Inspect(fd, func(n ast.Node) bool {
					if ce, ok := n.(*ast.CallExpr); ok {
						if id, ok := ce.Fun.(*ast.Ident); ok && id.Name == "matchExcludePattern" {
							owners[fd.Name.Name] = true
						}
					}
					return true
				})
			}
		}
		want := map[string]bool{"manifestExcludesDir": true}
		if !reflect.DeepEqual(owners, want) {
			t.Errorf("AC-66: matchExcludePattern is called from %v, want only manifestExcludesDir. Every caller beyond the predicate is another copy of the exclusion decision", sortedNames(owners))
		}
	})
}

func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

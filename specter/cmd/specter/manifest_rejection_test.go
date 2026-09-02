// manifest_rejection_test.go -- CLI integration tests for spec-manifest
// 1.13.0 C-31: a command that loads specter.yaml surfaces a rejection rather
// than discarding it (AC-47 through AC-51).
//
// Every criterion here is a statement about what a command writes to stderr
// and what it exits with, so all of them run the binary.
//
// There are two workspaces, because the warn-and-continue criteria and the
// hard-fail criterion need opposite things from one.
//
// AC-47 through AC-50 use manifestWorkspace, built so that a discarded
// rejection is visible in the output as well as absent from stderr: the
// manifest points specs_dir at customSpecs, and a second valid spec sits under
// the default specs directory. A run against defaults finds both; a run
// against the configured setting finds one. Those four criteria rest on that
// difference.
//
// AC-51 uses manifestAllAnnotatedWorkspace, which carries the same manifest
// and the same two specs with every criterion annotated. That workspace
// succeeds once the manifest is removed, so a non-zero exit under the manifest
// can have no cause but the rejection. AC-51 ran on the two-spec workspace
// until 1.13.0, where coverage and sync exited non-zero at 0 percent coverage
// whether or not they honored the rejection (`bugs/SP-SP-041`).
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rejectedManifest = `schema_version: 1
system:
  name: demo
settings:
  specs_dir: customSpecs
  test_glob: 'tests/**/*.go'
`

const acceptedManifest = `schema_version: 1
system:
  name: demo
settings:
  specs_dir: customSpecs
`

// manifestWorkspace writes the two-spec workspace with the given manifest
// body. The manifest is the only thing that varies between AC-47 and AC-50,
// so the two cases see byte-identical specs.
func manifestWorkspace(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	writeManifest(t, dir, manifest)
	writeRawFile(t, dir, "customSpecs/demo.spec.yaml",
		defectSpecYAML(defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01"}}))
	writeRawFile(t, dir, "specs/decoy.spec.yaml",
		defectSpecYAML(defectSpec{id: "decoy-spec", tier: 3, acIDs: []string{"AC-01"}}))
	return dir
}

// manifestAllAnnotatedWorkspace writes the AC-51 workspace: the same two spec
// files, each declaring two criteria, and one test file annotating all four.
// The manifest body is a parameter so the criterion's precondition can be run
// on a byte-identical workspace with no manifest at all.
//
// The spec ids and paths match manifestWorkspace on purpose. The one thing
// that differs is the annotation coverage, which is what turns a run against
// defaults from a failure into a success.
func manifestAllAnnotatedWorkspace(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if manifest != "" {
		writeManifest(t, dir, manifest)
	}
	writeRawFile(t, dir, "customSpecs/demo.spec.yaml",
		defectSpecYAML(defectSpec{id: "demo-spec", tier: 3, acIDs: []string{"AC-01", "AC-02"}}))
	writeRawFile(t, dir, "specs/decoy.spec.yaml",
		defectSpecYAML(defectSpec{id: "decoy-spec", tier: 3, acIDs: []string{"AC-01", "AC-02"}}))
	writeRawFile(t, dir, "tests/all_test.go", `// @spec demo-spec
// @ac AC-01
// @ac AC-02
func TestDemoFixture(t *testing.T) {}

// @spec decoy-spec
// @ac AC-01
// @ac AC-02
func TestDecoyFixture(t *testing.T) {}
`)
	return dir
}

// assertRejectionWarning checks the three things AC-47 through AC-49 name on
// stderr: the manifest path, the parser's reason, and the statement that
// default settings are in effect.
func assertRejectionWarning(t *testing.T, stderr string) {
	t.Helper()
	for _, want := range []string{"specter.yaml", "settings.test_glob", "default settings"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr must name %q; got:\n%s", want, stderr)
		}
	}
}

// countSpecLines counts the lines of stdout that name a spec file.
func countSpecLines(stdout string) int {
	n := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, ".spec.yaml") {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// AC-47 -- parse warns and continues
// ---------------------------------------------------------------------------

// The stdout assertion is what makes the stderr assertion worth anything. It
// pins the behavior the warning is about: the run really did fall back to
// defaults, so it found the decoy the configured specs_dir would have
// excluded. A fix that prints the warning and then honors the rejected
// manifest fails here.
//
// @ac AC-47
func TestManifestRejection_ParseWarnsAndContinues(t *testing.T) {
	t.Run("spec-manifest/AC-47 parse warns on a rejected manifest and parses with defaults", func(t *testing.T) {
		dir := manifestWorkspace(t, rejectedManifest)
		stdout, stderr, code := runCLISplit(t, dir, "parse")

		assertRejectionWarning(t, stderr)
		for _, want := range []string{"customSpecs/demo.spec.yaml", "specs/decoy.spec.yaml"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout must list %q; got:\n%s", want, stdout)
			}
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-48 -- resolve warns and continues
// ---------------------------------------------------------------------------

// @ac AC-48
func TestManifestRejection_ResolveWarnsAndContinues(t *testing.T) {
	t.Run("spec-manifest/AC-48 resolve warns on a rejected manifest and builds the graph from defaults", func(t *testing.T) {
		dir := manifestWorkspace(t, rejectedManifest)
		stdout, stderr, code := runCLISplit(t, dir, "resolve")

		assertRejectionWarning(t, stderr)
		if !strings.Contains(stdout, "2 specs") {
			t.Errorf("stdout must report `2 specs`; got:\n%s", stdout)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-49 -- explain warns and continues
// ---------------------------------------------------------------------------

// @ac AC-49
func TestManifestRejection_ExplainWarnsAndContinues(t *testing.T) {
	t.Run("spec-manifest/AC-49 explain warns on a rejected manifest and still prints its table", func(t *testing.T) {
		dir := manifestWorkspace(t, rejectedManifest)
		stdout, stderr, code := runCLISplit(t, dir, "explain", "demo-spec")

		assertRejectionWarning(t, stderr)
		if !strings.Contains(stdout, "specter explain demo-spec") {
			t.Errorf("stdout must carry `specter explain demo-spec`; got:\n%s", stdout)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-50 -- a valid manifest produces no warning
// ---------------------------------------------------------------------------

// This is the control for the three absence-of-defect claims above. The
// manifest differs from the AC-47 one by the unknown key and by nothing else,
// so a warning that appears here is unconditional rather than caused by the
// rejection. The one-spec stdout is the second half: it proves the manifest
// was honored rather than ignored quietly.
//
// @ac AC-50
func TestManifestRejection_ValidManifestProducesNoWarning(t *testing.T) {
	t.Run("spec-manifest/AC-50 a valid manifest is honored and writes nothing to stderr", func(t *testing.T) {
		dir := manifestWorkspace(t, acceptedManifest)
		stdout, stderr, code := runCLISplit(t, dir, "parse")

		if !strings.Contains(stdout, "customSpecs/demo.spec.yaml") {
			t.Errorf("stdout must list `customSpecs/demo.spec.yaml`; got:\n%s", stdout)
		}
		if n := countSpecLines(stdout); n != 1 {
			t.Errorf("stdout must name exactly 1 spec, named %d; got:\n%s", n, stdout)
		}
		if stderr != "" {
			t.Errorf("a valid manifest must write nothing to stderr; got:\n%s", stderr)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s", code, stdout)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-51 -- the commands that already reject a bad manifest keep rejecting it
// ---------------------------------------------------------------------------

// Four commands, one workspace each, so a command that mutates the workspace
// cannot affect another's result. The non-zero exit plus the parser's reason
// on stderr is also the control for the `init --refresh` byte-comparison: it
// proves the command ran and reached the manifest, so an unchanged file is a
// refusal to rewrite rather than a command that never started.
//
// The exit code is the discriminator here, not the stderr string. A
// warn-and-continue path prints a warning that names `settings.test_glob`
// itself, so the stderr check alone is satisfied by the behavior the criterion
// forbids. What forecloses it is the workspace: every declared criterion is
// annotated, so a run that ignored the manifest and continued against defaults
// finds both specs at 100 percent and exits 0. TestManifestRejection_
// AllAnnotatedWorkspaceSucceedsWithoutTheManifest is the positive control for
// that claim, and it must be read as part of this criterion rather than as a
// separate one.
//
// `init --refresh` is the exception the criterion calls out. It exits non-zero
// with no manifest at all, so its exit code cannot separate the two behaviors.
// The byte-unchanged clause is what pins it: a refresh against defaults would
// rewrite the file.
//
// @ac AC-51
func TestManifestRejection_HardFailingCommandsKeepFailing(t *testing.T) {
	commands := []struct {
		args []string
		// checkManifestUnchanged is set only for init --refresh. The
		// criterion states the byte-comparison for that command alone, and
		// the other three are not asked to leave the file alone.
		checkManifestUnchanged bool
	}{
		{args: []string{"check"}},
		{args: []string{"coverage", "--strictness", "annotation"}},
		{args: []string{"sync", "--strictness", "annotation"}},
		{args: []string{"init", "--refresh"}, checkManifestUnchanged: true},
	}

	for _, c := range commands {
		t.Run("spec-manifest/AC-51 "+strings.Join(c.args, " ")+" rejects a bad manifest", func(t *testing.T) {
			dir := manifestAllAnnotatedWorkspace(t, rejectedManifest)
			before, err := os.ReadFile(filepath.Join(dir, "specter.yaml"))
			if err != nil {
				t.Fatalf("read specter.yaml: %v", err)
			}

			stdout, stderr, code := runCLISplit(t, dir, c.args...)
			if code == 0 {
				t.Errorf("expected a non-zero exit, got 0;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			if !strings.Contains(stderr, "settings.test_glob") {
				t.Errorf("stderr must name the parser's reason; got:\n%s", stderr)
			}

			if c.checkManifestUnchanged {
				after, err := os.ReadFile(filepath.Join(dir, "specter.yaml"))
				if err != nil {
					t.Fatalf("re-read specter.yaml: %v", err)
				}
				if string(after) != string(before) {
					t.Errorf("specter.yaml must be byte-unchanged;\nbefore:\n%s\nafter:\n%s", before, after)
				}
			}
		})
	}
}

// AC-51 states a precondition on its workspace, and this is it. Without the
// manifest, check, coverage and sync all exit 0, so the non-zero exits above
// have one available cause and it is the rejection. Drop this case and AC-51
// goes back to being satisfiable by a command that fails for its own reasons.
//
// `init --refresh` is asserted differently because it reports a missing
// manifest as its own error, so its exit code is non-zero either way. What the
// precondition can show is that the reason differs: with no manifest, stderr
// does not name the parser's reason. The positive control for that absence is
// the case above, which runs the same command on the same workspace with the
// manifest present and requires the reason to be there.
//
// @ac AC-51
func TestManifestRejection_AllAnnotatedWorkspaceSucceedsWithoutTheManifest(t *testing.T) {
	for _, args := range [][]string{
		{"check"},
		{"coverage", "--strictness", "annotation"},
		{"sync", "--strictness", "annotation"},
	} {
		t.Run("spec-manifest/AC-51 "+strings.Join(args, " ")+" exits 0 on the AC-51 workspace with no manifest", func(t *testing.T) {
			dir := manifestAllAnnotatedWorkspace(t, "")
			stdout, stderr, code := runCLISplit(t, dir, args...)
			if code != 0 {
				t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
		})
	}

	t.Run("spec-manifest/AC-51 init --refresh reports the missing manifest as its own error", func(t *testing.T) {
		dir := manifestAllAnnotatedWorkspace(t, "")
		stdout, stderr, code := runCLISplit(t, dir, "init", "--refresh")
		if code == 0 {
			t.Errorf("expected a non-zero exit with no manifest, got 0;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if strings.Contains(stderr, "settings.test_glob") {
			t.Errorf("with no manifest, stderr must not name the parser's reason; got:\n%s", stderr)
		}
	})
}

// manifest_rejection_test.go -- CLI integration tests for spec-manifest
// 1.12.0 C-31: a command that loads specter.yaml surfaces a rejection rather
// than discarding it (AC-47 through AC-51).
//
// Every criterion here is a statement about what a command writes to stderr
// and what it exits with, so all of them run the binary.
//
// The workspace is built so that a discarded rejection is visible in the
// output as well as absent from stderr: the manifest points specs_dir at
// customSpecs, and a second valid spec sits under the default specs
// directory. A run against defaults finds both; a run against the configured
// setting finds one.
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
			dir := manifestWorkspace(t, rejectedManifest)
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

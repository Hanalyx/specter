// settings_annotation_cli_test.go — CLI-level tests for the v0.15 1D-a
// `settings.annotation` surface and its conflict rule (C-32, C-34; AC-54,
// AC-56, AC-57, AC-58, AC-59).
//
// These cross the CLI boundary on purpose. AC-54 is a claim about an exit
// code and a framework error, AC-57 and AC-58 are claims about a process
// stderr line and stdout bytes, and none of the three is observable from
// inside a pure package.
//
// AC-56 names `CheckAnnotationStrictnessConflict` as the unit under test.
// That function does not exist in the tree these tests were written against,
// and Go offers no runtime lookup for a package-level function, so a direct
// call is a compile error that would take down every other manifest test.
// The declaredness discrimination AC-56 describes is asserted here at the
// CLI layer instead, one command deep. The unit-level test C-34(b) asks for
// is reported as blocked rather than faked.
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// annotationManifestBody builds a manifest from two independent inputs: the
// `settings.strictness` value and the `settings.annotation.permissive` value.
// Passing "" for either omits that key entirely. The fixtures therefore vary
// one key at a time by construction, and every byte outside the varied key is
// identical across cases.
func annotationManifestBody(strictness, permissive string) string {
	body := "system:\n  name: annotation-cli\nsettings:\n  specs_dir: specs\n"
	if strictness != "" {
		body += "  strictness: " + strictness + "\n"
	}
	if permissive != "" {
		body += "  annotation:\n    permissive: " + permissive + "\n"
	}
	return body
}

// annotationWorkspace builds the workspace AC-57 and AC-58 name: one spec
// with one acceptance criterion, one test file annotating it, and no
// .specter-results.json. The manifest is written by the caller, because
// several cases need it rewritten between runs.
func annotationWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))
	testFile := "// @spec my-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// putManifest writes specter.yaml into dir, overwriting whatever is there.
// `sync` rewrites the manifest (C-06), so any run that follows a sync needs a
// fresh one or it is no longer testing the named input.
func putManifest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

// conflictLines returns the stderr lines carrying both `settings.strictness`
// and `settings.annotation`. That pair on one line is the signature of the
// C-34 warning. The match is line-scoped rather than whole-stream, because
// the strictness error already prints `--strictness annotation` and a
// whole-stream substring test would be one refactor away from a false
// positive.
func conflictLines(stderr string) []string {
	var found []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "settings.strictness") && strings.Contains(line, "settings.annotation") {
			found = append(found, line)
		}
	}
	return found
}

// requireManifestAccepted fails when the run rejected the manifest at parse
// time. Every criterion below that declares an annotation block presupposes
// the block parses; without this anchor a rejected manifest silently satisfies
// each "no warning appeared" assertion.
func requireManifestAccepted(t *testing.T, label, stderr string) {
	t.Helper()
	if strings.Contains(stderr, "unknown settings key") {
		t.Fatalf("%s: the manifest was rejected at parse, so nothing downstream was exercised. C-32 requires `annotation` to be a valid settings key. stderr:\n%s",
			label, stderr)
	}
}

// ---------------------------------------------------------------------------
// AC-54 — no command registers an --annotation flag
// ---------------------------------------------------------------------------

// @ac AC-54
// A guard criterion. It holds in the tree these tests were written against
// and it is meant to: SSRB-104 section 7.3 makes flag-and-manifest divergence
// unreachable by having no flag, so registering one later in either form is
// what this must catch.
func TestAnnotationFlag_NotRegisteredOnAnyCommand(t *testing.T) {
	for _, command := range []string{"check", "coverage", "sync"} {
		for _, flag := range []string{"--annotation", "--annotation=true"} {
			t.Run("spec-manifest/AC-54 "+command+" rejects "+flag, func(t *testing.T) {
				dir := annotationWorkspace(t)
				putManifest(t, dir, annotationManifestBody("", ""))
				_, stderr, code := runCLISplit(t, dir, command, flag)
				if code == 0 {
					t.Errorf("expected a non-zero exit for %s %s, got 0; stderr:\n%s", command, flag, stderr)
				}
				if !strings.Contains(stderr, "unknown flag") {
					t.Errorf("expected an 'unknown flag' error from the CLI framework for %s %s, got stderr:\n%s",
						command, flag, stderr)
				}
			})
		}
	}

	// Positive control. Without it this test cannot tell "rejects
	// --annotation" from "rejects every flag". `--strictness annotation` is
	// the nearest accepted case: same command, a real registered flag, and
	// the same word as a value.
	t.Run("spec-manifest/AC-54 positive control, coverage accepts --strictness annotation", func(t *testing.T) {
		dir := annotationWorkspace(t)
		putManifest(t, dir, annotationManifestBody("", ""))
		_, stderr, code := runCLISplit(t, dir, "coverage", "--strictness", "annotation")
		if strings.Contains(stderr, "unknown flag") {
			t.Errorf("expected --strictness to be a registered flag, got stderr:\n%s", stderr)
		}
		if code != 0 {
			t.Errorf("expected exit 0 for a registered flag on a covered workspace, got %d; stderr:\n%s", code, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-56 — the conflict keys on declaredness, not on value
// ---------------------------------------------------------------------------

// @ac AC-56
// Three manifests, one variable apart each. `threshold` is the strictness
// value on purpose: C-24 rewrites an absent `strictness` to `threshold` at
// parse time, so an implementation reading the parsed string rather than the
// raw key set warns on the block-only manifest and fails the second case.
func TestAnnotationStrictnessConflict_KeysOnDeclarednessNotValue(t *testing.T) {
	t.Run("spec-manifest/AC-56 both keys declared produces the conflict warning", func(t *testing.T) {
		dir := annotationWorkspace(t)
		putManifest(t, dir, annotationManifestBody("threshold", "false"))
		_, stderr, _ := runCLISplit(t, dir, "check")
		requireManifestAccepted(t, "both keys declared", stderr)

		lines := conflictLines(stderr)
		if len(lines) == 0 {
			t.Fatalf("expected a conflict warning naming settings.strictness and settings.annotation, got stderr:\n%s", stderr)
		}
		if !strings.Contains(lines[0], "threshold") {
			t.Errorf("expected the warning to name the declared value 'threshold', got line:\n%s", lines[0])
		}
	})

	t.Run("spec-manifest/AC-56 annotation block alone produces no warning", func(t *testing.T) {
		dir := annotationWorkspace(t)
		putManifest(t, dir, annotationManifestBody("", "false"))
		_, stderr, _ := runCLISplit(t, dir, "check")
		requireManifestAccepted(t, "annotation block alone", stderr)

		if lines := conflictLines(stderr); len(lines) > 0 {
			t.Errorf("expected no conflict warning when no strictness key is declared, got:\n%s", strings.Join(lines, "\n"))
		}
	})

	t.Run("spec-manifest/AC-56 strictness alone produces no warning", func(t *testing.T) {
		dir := annotationWorkspace(t)
		putManifest(t, dir, annotationManifestBody("threshold", ""))
		_, stderr, _ := runCLISplit(t, dir, "check")

		if lines := conflictLines(stderr); len(lines) > 0 {
			t.Errorf("expected no conflict warning when no annotation block is declared, got:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// ---------------------------------------------------------------------------
// AC-57 — the warning reaches the operator and changes no exit code
// ---------------------------------------------------------------------------

// @ac AC-57
// Each command runs twice on the same workspace shape, with only the
// `settings.strictness` line removed the second time. The second half is what
// pins the exit code to the ignored key rather than to the warning. Each run
// gets a freshly written manifest, because `sync` writes the registry back
// into specter.yaml (C-06) and a rerun against the rewritten file is no
// longer the input this criterion names.
func TestAnnotationStrictnessConflict_WarningOnStderrAndExitCodeUnchanged(t *testing.T) {
	for _, command := range []string{"check", "coverage", "sync"} {
		t.Run("spec-manifest/AC-57 "+command+" warns on the conflict without changing its exit code", func(t *testing.T) {
			conflictDir := annotationWorkspace(t)
			putManifest(t, conflictDir, annotationManifestBody("annotation", "true"))
			_, conflictErr, conflictCode := runCLISplit(t, conflictDir, command)
			requireManifestAccepted(t, command+" with the conflict", conflictErr)

			lines := conflictLines(conflictErr)
			if len(lines) == 0 {
				t.Fatalf("expected %s to write a conflict warning to stderr, got:\n%s", command, conflictErr)
			}
			if !strings.Contains(lines[0], "annotation") {
				t.Errorf("expected the warning to name the declared value 'annotation', got line:\n%s", lines[0])
			}

			blockOnlyDir := annotationWorkspace(t)
			putManifest(t, blockOnlyDir, annotationManifestBody("", "true"))
			_, blockOnlyErr, blockOnlyCode := runCLISplit(t, blockOnlyDir, command)
			requireManifestAccepted(t, command+" with the strictness line removed", blockOnlyErr)

			if l := conflictLines(blockOnlyErr); len(l) > 0 {
				t.Errorf("expected no conflict warning from %s once settings.strictness is removed, got:\n%s",
					command, strings.Join(l, "\n"))
			}
			if conflictCode != blockOnlyCode {
				t.Errorf("expected %s to exit the same with and without the ignored settings.strictness key, got %d with and %d without",
					command, conflictCode, blockOnlyCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-58 — while the block is declared, the run does not vary with strictness
// ---------------------------------------------------------------------------

// @ac AC-58
// The precondition run is what makes the assertion meaningful: without an
// annotation block the two strictness values demonstrably diverge on this
// workspace, so invariance under them afterwards is a fact about the block
// and not about the workspace. The criterion says nothing about what the
// common result is, and neither does this test.
func TestAnnotationBlock_RunDoesNotVaryWithStrictnessValue(t *testing.T) {
	t.Run("spec-manifest/AC-58 precondition, strictness values diverge with no annotation block", func(t *testing.T) {
		annotationDir := annotationWorkspace(t)
		putManifest(t, annotationDir, annotationManifestBody("annotation", ""))
		_, annotationErr, annotationCode := runCLISplit(t, annotationDir, "coverage")
		if annotationCode != 0 {
			t.Fatalf("expected coverage to exit 0 under settings.strictness: annotation, got %d; stderr:\n%s",
				annotationCode, annotationErr)
		}

		thresholdDir := annotationWorkspace(t)
		putManifest(t, thresholdDir, annotationManifestBody("threshold", ""))
		_, thresholdErr, thresholdCode := runCLISplit(t, thresholdDir, "coverage")
		if thresholdCode == 0 {
			t.Fatalf("expected coverage to exit non-zero under settings.strictness: threshold, got 0; stderr:\n%s", thresholdErr)
		}
		if !strings.Contains(thresholdErr, ".specter-results.json") {
			t.Fatalf("expected the threshold failure to name the results file, got stderr:\n%s", thresholdErr)
		}
	})

	t.Run("spec-manifest/AC-58 with the block declared, exit code and stdout are invariant", func(t *testing.T) {
		levels := []string{"annotation", "threshold", "zero-tolerance"}
		codes := make([]int, len(levels))
		stdouts := make([]string, len(levels))

		for i, level := range levels {
			dir := annotationWorkspace(t)
			// The annotation block is emitted by the same builder for all
			// three, so it is byte-identical by construction. Only the
			// strictness value differs.
			putManifest(t, dir, annotationManifestBody(level, "true"))
			stdout, stderr, code := runCLISplit(t, dir, "coverage")
			requireManifestAccepted(t, "coverage under settings.strictness: "+level, stderr)
			codes[i] = code
			stdouts[i] = stdout
		}

		for i := 1; i < len(levels); i++ {
			if codes[i] != codes[0] {
				t.Errorf("expected the same exit code under every strictness value while the annotation block is declared: %s gave %d, %s gave %d",
					levels[0], codes[0], levels[i], codes[i])
			}
			// stdout is the comparison and stderr is not: the C-34 warning
			// names the declared value, so stderr differs across the three
			// runs by design.
			if stdouts[i] != stdouts[0] {
				t.Errorf("expected byte-identical stdout under every strictness value while the annotation block is declared.\n--- %s ---\n%s\n--- %s ---\n%s",
					levels[0], stdouts[0], levels[i], stdouts[i])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC-59 — the conflict rule reads manifest keys only
// ---------------------------------------------------------------------------

// @ac AC-59
// An implementation that computes the conflict from the effective strictness,
// after the flag has been merged with the manifest value, warns here. The
// criterion claims nothing about what the flag does to the run, so neither
// does this test: it asserts the absence of the warning and the exit code is
// left alone. The positive control is the warning-present run in AC-57.
func TestAnnotationStrictnessConflict_FlagCannotTriggerTheWarning(t *testing.T) {
	t.Run("spec-manifest/AC-59 --strictness threshold does not trigger the conflict warning", func(t *testing.T) {
		dir := annotationWorkspace(t)
		putManifest(t, dir, annotationManifestBody("", "true"))
		_, stderr, _ := runCLISplit(t, dir, "coverage", "--strictness", "threshold")
		requireManifestAccepted(t, "coverage --strictness threshold", stderr)

		if lines := conflictLines(stderr); len(lines) > 0 {
			t.Errorf("expected no conflict warning: the manifest declares no settings.strictness key and the rule reads manifest keys only. Got:\n%s",
				strings.Join(lines, "\n"))
		}
	})
}

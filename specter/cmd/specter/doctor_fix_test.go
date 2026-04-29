// doctor_fix_test.go -- CLI tests for `specter doctor --fix`.
//
// @spec spec-doctor
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacySpecWithTrustLevel returns spec YAML carrying a trust_level key that
// will fail schema parse — the scenario doctor --fix is built for.
const legacySpecWithTrustLevel = `spec:
  id: legacy-spec
  version: "1.0.0"
  status: draft
  tier: 3
  trust_level: high
  context:
    system: test
    feature: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "MUST something"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      priority: high
`

// @ac AC-12
// doctor --fix strips trust_level and the file parses cleanly afterwards.
func TestDoctor_Fix_StripsTrustLevel(t *testing.T) {
	t.Run("spec-doctor/AC-12 fix strips trust_level and file parses cleanly", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		out, _ := runCLI(t, dir, "doctor", "--fix", "--yes")
		// Post-fix: the file must parse, and the trust_level line must be gone.
		after, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read after fix: %v", err)
		}
		if strings.Contains(string(after), "trust_level") {
			t.Errorf("trust_level not stripped; file:\n%s", after)
		}

		// Summary should name the file.
		if !strings.Contains(out, "legacy.spec.yaml") {
			t.Errorf("summary must name the rewritten file; got:\n%s", out)
		}
		// Verify the file now parses.
		_, code := runCLI(t, dir, "parse", specPath)
		if code != 0 {
			t.Errorf("expected clean parse after doctor --fix; exit=%d", code)
		}
	})
}

// @ac AC-13
// doctor --fix --dry-run prints the plan but writes nothing.
func TestDoctor_Fix_DryRun_DoesNotWrite(t *testing.T) {
	t.Run("spec-doctor/AC-13 dry-run prints plan and leaves file byte-identical", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		out, code := runCLI(t, dir, "doctor", "--fix", "--dry-run")
		if code != 0 {
			t.Fatalf("dry-run expected exit 0, got %d. output:\n%s", code, out)
		}
		if !strings.Contains(out, "would rewrite") && !strings.Contains(out, "would be rewritten") {
			t.Errorf("dry-run output must indicate `would rewrite`; got:\n%s", out)
		}
		// File must be byte-identical.
		after, _ := os.ReadFile(specPath)
		if string(after) != legacySpecWithTrustLevel {
			t.Errorf("dry-run must not modify the file; diff detected")
		}
	})
}

// @ac AC-14
// doctor (no --fix) must never write, even when drift is detected.
// Regression guard for C-07 — drift surfaces in the parse-error pattern
// analysis, but the filesystem stays untouched.
func TestDoctor_NoFix_DoesNotWrite(t *testing.T) {
	t.Run("spec-doctor/AC-14 plain doctor does not modify any files", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		_, _ = runCLI(t, dir, "doctor")
		after, _ := os.ReadFile(specPath)
		if string(after) != legacySpecWithTrustLevel {
			t.Errorf("plain doctor must not modify specs; file changed")
		}
	})
}

// @ac AC-15
// doctor --fix on a clean workspace (all specs parse) prints "no changes".
func TestDoctor_Fix_NoChanges_Exits0(t *testing.T) {
	t.Run("spec-doctor/AC-15 fix on clean workspace prints no changes", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "clean.spec.yaml", minimalValidSpec("clean", 3, "AC-01"))

		out, code := runCLI(t, dir, "doctor", "--fix", "--yes")
		if code != 0 {
			t.Errorf("expected exit 0 on clean workspace, got %d. output:\n%s", code, out)
		}
		if !strings.Contains(out, "no changes") {
			t.Errorf("expected `no changes` in output; got:\n%s", out)
		}
	})
}

// manifestWithoutSchemaVersion is the pre-v0.12 specter.yaml shape (no
// schema_version line). doctor --fix should canonicalize it.
const manifestWithoutSchemaVersion = `system:
  name: demo
  tier: 2
domains:
  default:
    tier: 2
    specs: []
`

// @ac AC-16
// doctor --fix on a workspace whose specter.yaml lacks schema_version adds
// schema_version: 1 at the top. ParseManifest then reports SchemaVersion=1
// and original content is byte-preserved after the new line.
func TestDoctor_Fix_Manifest_AddsSchemaVersion(t *testing.T) {
	t.Run("spec-doctor/AC-16 fix prepends schema_version to manifest lacking it", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "specter.yaml")
		if err := os.WriteFile(manifestPath, []byte(manifestWithoutSchemaVersion), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "doctor", "--fix", "--yes")

		after, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read after: %v", err)
		}
		// First non-empty line must be schema_version: 1.
		lines := strings.Split(string(after), "\n")
		var first string
		for _, l := range lines {
			if t := strings.TrimSpace(l); t != "" {
				first = t
				break
			}
		}
		if first != "schema_version: 1" {
			t.Errorf("first non-empty line = %q, want %q\nfile:\n%s", first, "schema_version: 1", after)
		}
		// Original content must still be present after the schema_version line.
		if !strings.Contains(string(after), "system:") || !strings.Contains(string(after), "name: demo") {
			t.Errorf("original manifest content not preserved; got:\n%s", after)
		}
		// Summary must mention the rewrite name.
		if !strings.Contains(out, "add-schema-version") {
			t.Errorf("summary must reference `add-schema-version` rewrite; got:\n%s", out)
		}
	})
}

// @ac AC-17
// doctor --fix on a manifest that already declares schema_version leaves
// the file byte-unchanged.
func TestDoctor_Fix_Manifest_AlreadyCanonical_IsNoOp(t *testing.T) {
	t.Run("spec-doctor/AC-17 fix on already-canonical manifest is byte no-op", func(t *testing.T) {
		dir := t.TempDir()
		canonical := "schema_version: 1\n" + manifestWithoutSchemaVersion
		manifestPath := filepath.Join(dir, "specter.yaml")
		if err := os.WriteFile(manifestPath, []byte(canonical), 0644); err != nil {
			t.Fatal(err)
		}
		writeSpec(t, dir, "clean.spec.yaml", minimalValidSpec("clean", 3, "AC-01"))

		out, _ := runCLI(t, dir, "doctor", "--fix", "--yes")

		after, _ := os.ReadFile(manifestPath)
		if string(after) != canonical {
			t.Errorf("already-canonical manifest must be byte-unchanged; got:\n%s\nwant:\n%s", after, canonical)
		}
		// Should not name the manifest as rewritten in the summary.
		if strings.Contains(out, "add-schema-version") {
			t.Errorf("must not report add-schema-version when manifest already has schema_version; got:\n%s", out)
		}
	})
}

// legacySpecBlockScalarTrustLevel is a spec where trust_level uses the
// literal-style block scalar (`|`). doctor --fix must refuse to rewrite
// this shape (per AC-19) — line-based deletion would orphan the
// continuation lines and corrupt the file.
const legacySpecBlockScalarTrustLevel = `spec:
  id: legacy-spec
  version: "1.0.0"
  status: draft
  tier: 3
  trust_level: |
    high
    confidence
  context:
    system: test
    feature: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "MUST something"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
      priority: high
`

// @ac AC-19
// CLI integration: doctor --fix on a spec whose trust_level is a block
// scalar emits the `needs manual edit` summary block, leaves the file
// byte-unchanged, and does NOT include the file in the rewritten count.
func TestDoctor_Fix_BlockScalar_PrintsManualEditSummary(t *testing.T) {
	t.Run("spec-doctor/AC-19 block scalar trust_level produces manual-edit summary", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecBlockScalarTrustLevel), 0644)

		out, _ := runCLI(t, dir, "doctor", "--fix", "--yes")

		// File must be byte-unchanged after refusal.
		after, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read after: %v", err)
		}
		if string(after) != legacySpecBlockScalarTrustLevel {
			t.Errorf("file must be byte-unchanged when --fix refuses; got:\n%s", after)
		}

		// Summary must include the manual-edit block.
		if !strings.Contains(out, "need manual edit") {
			t.Errorf("expected `need manual edit` summary block; got:\n%s", out)
		}
		// Block-scalar reason must surface.
		if !strings.Contains(strings.ToLower(out), "block scalar") {
			t.Errorf("expected reason naming `block scalar`; got:\n%s", out)
		}
		// File name must appear in the manual-edit listing.
		if !strings.Contains(out, "legacy.spec.yaml") {
			t.Errorf("expected manual-edit entry to name legacy.spec.yaml; got:\n%s", out)
		}
		// Must NOT appear under the rewritten block — search for the
		// "rewritten" header that would only appear if at least one
		// successful rewrite happened.
		if strings.Contains(out, "doctor --fix: 1 file(s) rewritten") ||
			strings.Contains(out, "doctor --fix: 2 file(s) rewritten") {
			t.Errorf("file must not be counted as rewritten when refused; got:\n%s", out)
		}
	})
}

// @ac AC-18
// doctor --fix in a workspace with NO specter.yaml does not create one.
// The manifest canonicalization is a silent no-op when no manifest exists.
func TestDoctor_Fix_NoManifest_DoesNotCreate(t *testing.T) {
	t.Run("spec-doctor/AC-18 fix without manifest does not create one", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "clean.spec.yaml", minimalValidSpec("clean", 3, "AC-01"))

		out, _ := runCLI(t, dir, "doctor", "--fix", "--yes")

		if _, err := os.Stat(filepath.Join(dir, "specter.yaml")); err == nil {
			t.Errorf("doctor --fix must not create specter.yaml when one does not exist")
		}
		if strings.Contains(out, "add-schema-version") {
			t.Errorf("must not report add-schema-version when no manifest exists; got:\n%s", out)
		}
	})
}

// @ac AC-24
// confirmFixWithUser with TTY stdin and operator entering an affirmative
// answer ("y", "Y", "yes", "YES") → returns proceed=true, prints BETA
// warning and the prompt to stderr.
//
// Direct unit test of the helper rather than an exec.Cmd subprocess: a
// Go test process spawning a child via exec.Cmd cannot present a real
// pty to the child, so any subprocess test of "interactive y proceeds"
// is fundamentally a non-TTY test. The helper takes an isTTY bool and
// an io.Reader, so the unit-test path injects both directly.
func TestDoctor_Fix_BetaGate_YesProceeds(t *testing.T) {
	cases := []struct {
		name  string
		stdin string
	}{
		{"y", "y\n"},
		{"Y", "Y\n"},
		{"yes", "yes\n"},
		{"YES", "YES\n"},
	}
	for _, tc := range cases {
		t.Run("spec-doctor/AC-24 interactive "+tc.name+" proceeds", func(t *testing.T) {
			var stderr bytes.Buffer
			proceed, err := confirmFixWithUser(strings.NewReader(tc.stdin), true, &stderr)
			if err != nil {
				t.Fatalf("unexpected error from helper with TTY+%s: %v", tc.name, err)
			}
			if !proceed {
				t.Errorf("expected proceed=true for affirmative %q, got false", tc.stdin)
			}
			if !strings.Contains(stderr.String(), "[BETA]") {
				t.Errorf("expected [BETA] warning in stderr; got:\n%s", stderr.String())
			}
			if !strings.Contains(strings.ToLower(stderr.String()), "cycle 6") {
				t.Errorf("expected warning to name cycle 6 known limitation; got:\n%s", stderr.String())
			}
			if !strings.Contains(stderr.String(), "Continue? (y/N)") {
				t.Errorf("expected `Continue? (y/N)` prompt; got:\n%s", stderr.String())
			}
		})
	}
}

// @ac AC-25
// confirmFixWithUser with TTY stdin and any non-affirmative answer
// (empty, n, N, no, NO, unrecognized string) → returns proceed=false.
// The CLI integration layer turns proceed=false into "Aborted. No files
// modified." and exit 0 — that wiring is covered by the live AC-26 test
// (--yes proceeds) and AC-27 test (non-TTY refuses) which cross-check
// the file is byte-unchanged.
func TestDoctor_Fix_BetaGate_DeclineAborts(t *testing.T) {
	declineInputs := []struct {
		name  string
		stdin string
	}{
		{"empty (Enter)", "\n"},
		{"n", "n\n"},
		{"N", "N\n"},
		{"no", "no\n"},
		{"NO", "NO\n"},
		{"unrecognized", "maybe later\n"},
	}
	for _, tc := range declineInputs {
		t.Run("spec-doctor/AC-25 decline aborts ("+tc.name+")", func(t *testing.T) {
			var stderr bytes.Buffer
			proceed, err := confirmFixWithUser(strings.NewReader(tc.stdin), true, &stderr)
			if err != nil {
				t.Fatalf("unexpected error from helper with TTY+%q: %v", tc.stdin, err)
			}
			if proceed {
				t.Errorf("expected proceed=false for non-affirmative %q, got true", tc.stdin)
			}
		})
	}
}

// @ac AC-26
// `--yes` (and `-y`) bypass the warning AND prompt; rewrite proceeds.
func TestDoctor_Fix_BetaGate_YesFlagBypasses(t *testing.T) {
	for _, flag := range []string{"--yes", "-y"} {
		t.Run("spec-doctor/AC-26 "+flag+" bypasses warning and prompt", func(t *testing.T) {
			dir := t.TempDir()
			specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
			_ = os.MkdirAll(filepath.Dir(specPath), 0755)
			_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

			out, _ := runCLI(t, dir, "doctor", "--fix", flag)

			if strings.Contains(out, "[BETA]") {
				t.Errorf("%s must suppress [BETA] warning; got:\n%s", flag, out)
			}
			if strings.Contains(out, "Continue? (y/N)") {
				t.Errorf("%s must suppress prompt; got:\n%s", flag, out)
			}
			after, _ := os.ReadFile(specPath)
			if strings.Contains(string(after), "trust_level") {
				t.Errorf("%s must rewrite the file; trust_level still present", flag)
			}
		})
	}
}

// @ac AC-27
// Non-TTY stdin (no input available) without --yes → error, file unchanged,
// exit non-zero. runCLI's default behavior gives the child process EOF on
// stdin immediately (no input piped), which simulates the CI scenario.
func TestDoctor_Fix_BetaGate_NonTTY_WithoutYes_Errors(t *testing.T) {
	t.Run("spec-doctor/AC-27 non-tty stdin without --yes errors with guidance", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		out, code := runCLIWithStdin(t, dir, "", "doctor", "--fix")

		if code == 0 {
			t.Errorf("expected non-zero exit when --fix with non-tty stdin and no --yes; got 0:\n%s", out)
		}
		if !strings.Contains(out, "--yes") {
			t.Errorf("expected error message naming --yes flag; got:\n%s", out)
		}
		after, _ := os.ReadFile(specPath)
		if string(after) != legacySpecWithTrustLevel {
			t.Errorf("file must be byte-unchanged on non-tty refusal; got:\n%s", after)
		}
	})
}

// @ac AC-28
// `--fix --dry-run` skips the BETA warning and prompt entirely (preview
// mode is read-only). Summary still prints.
func TestDoctor_Fix_BetaGate_DryRun_SkipsPrompt(t *testing.T) {
	t.Run("spec-doctor/AC-28 dry-run skips warning and prompt", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		out, code := runCLI(t, dir, "doctor", "--fix", "--dry-run")

		if code != 0 {
			t.Errorf("expected exit 0 on --dry-run; got %d", code)
		}
		if strings.Contains(out, "[BETA]") {
			t.Errorf("--dry-run must suppress [BETA] warning; got:\n%s", out)
		}
		if strings.Contains(out, "Continue? (y/N)") {
			t.Errorf("--dry-run must suppress prompt; got:\n%s", out)
		}
		if !strings.Contains(out, "would be rewritten") && !strings.Contains(out, "would rewrite") {
			t.Errorf("expected dry-run summary; got:\n%s", out)
		}
		after, _ := os.ReadFile(specPath)
		if string(after) != legacySpecWithTrustLevel {
			t.Errorf("--dry-run must not modify the file; got:\n%s", after)
		}
	})
}

// @ac AC-29
// Non-TTY stdin with piped affirmative content (echo y | specter doctor
// --fix) without --yes MUST refuse with the same error and exit code as
// AC-27. The TTY check fires BEFORE stdin is read, so what the pipe
// contains is irrelevant.
//
// Two layers cover this: (a) the helper unit-test path verifies that
// confirmFixWithUser with isTTY=false refuses regardless of the reader
// content; (b) the exec.Cmd integration test verifies the wiring
// produces the same error message + non-zero exit + byte-unchanged file.
func TestDoctor_Fix_BetaGate_NonTTY_WithContent_Errors(t *testing.T) {
	t.Run("spec-doctor/AC-29 helper non-tty with piped content refuses", func(t *testing.T) {
		piped := []string{"y\n", "yes\n", "Y\n", "arbitrary content from CI\n"}
		for _, content := range piped {
			var stderr bytes.Buffer
			proceed, err := confirmFixWithUser(strings.NewReader(content), false, &stderr)
			if err == nil {
				t.Errorf("expected refusal error for non-tty with content %q, got nil", content)
			}
			if proceed {
				t.Errorf("expected proceed=false for non-tty with content %q, got true", content)
			}
			if !strings.Contains(err.Error(), "--yes") {
				t.Errorf("expected error to name --yes flag; got %q", err.Error())
			}
		}
	})

	t.Run("spec-doctor/AC-29 cli non-tty with piped y refuses", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
		_ = os.MkdirAll(filepath.Dir(specPath), 0755)
		_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

		out, code := runCLIWithStdin(t, dir, "y\n", "doctor", "--fix")

		if code == 0 {
			t.Errorf("expected non-zero exit when --fix with non-tty stdin (piped y) and no --yes; got 0:\n%s", out)
		}
		if !strings.Contains(out, "--yes") {
			t.Errorf("expected error message naming --yes flag; got:\n%s", out)
		}
		after, _ := os.ReadFile(specPath)
		if string(after) != legacySpecWithTrustLevel {
			t.Errorf("file must be byte-unchanged on non-tty refusal regardless of pipe content; got:\n%s", after)
		}
	})
}

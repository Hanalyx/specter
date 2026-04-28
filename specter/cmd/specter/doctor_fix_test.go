// doctor_fix_test.go -- CLI tests for `specter doctor --fix`.
//
// @spec spec-doctor
package main

import (
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

		out, _ := runCLI(t, dir, "doctor", "--fix")
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

		out, code := runCLI(t, dir, "doctor", "--fix")
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

		out, _ := runCLI(t, dir, "doctor", "--fix")

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

		out, _ := runCLI(t, dir, "doctor", "--fix")

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

// @ac AC-18
// doctor --fix in a workspace with NO specter.yaml does not create one.
// The manifest canonicalization is a silent no-op when no manifest exists.
func TestDoctor_Fix_NoManifest_DoesNotCreate(t *testing.T) {
	t.Run("spec-doctor/AC-18 fix without manifest does not create one", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "clean.spec.yaml", minimalValidSpec("clean", 3, "AC-01"))

		out, _ := runCLI(t, dir, "doctor", "--fix")

		if _, err := os.Stat(filepath.Join(dir, "specter.yaml")); err == nil {
			t.Errorf("doctor --fix must not create specter.yaml when one does not exist")
		}
		// Should not name a manifest in the summary.
		if strings.Contains(out, "add-schema-version") {
			t.Errorf("must not report add-schema-version when no manifest exists; got:\n%s", out)
		}
	})
}

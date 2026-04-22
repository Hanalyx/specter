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

// @ac AC-10
// doctor --fix strips trust_level and the file parses cleanly afterwards.
func TestDoctor_Fix_StripsTrustLevel(t *testing.T) {
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
}

// @ac AC-11
// doctor --fix --dry-run prints the plan but writes nothing.
func TestDoctor_Fix_DryRun_DoesNotWrite(t *testing.T) {
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
}

// @ac AC-12
// doctor (no --fix) must never write, even when drift is detected.
func TestDoctor_NoFix_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "specs", "legacy.spec.yaml")
	_ = os.MkdirAll(filepath.Dir(specPath), 0755)
	_ = os.WriteFile(specPath, []byte(legacySpecWithTrustLevel), 0644)

	_, _ = runCLI(t, dir, "doctor")
	after, _ := os.ReadFile(specPath)
	if string(after) != legacySpecWithTrustLevel {
		t.Errorf("plain doctor must not modify specs; file changed")
	}
}

// @ac AC-13
// doctor --fix on a clean workspace (all specs parse) prints "no changes".
func TestDoctor_Fix_NoChanges_Exits0(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "clean.spec.yaml", minimalValidSpec("clean", 3, "AC-01"))

	out, code := runCLI(t, dir, "doctor", "--fix")
	if code != 0 {
		t.Errorf("expected exit 0 on clean workspace, got %d. output:\n%s", code, out)
	}
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected `no changes` in output; got:\n%s", out)
	}
}

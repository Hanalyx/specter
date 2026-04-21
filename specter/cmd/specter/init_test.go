// init_test.go -- CLI-level tests for `specter init`, including the v0.9.2
// `--refresh` flag.
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper: write a pre-existing specter.yaml with content the test owns.
func writeManifestRaw(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func readManifest(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "specter.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return string(b)
}

// @spec spec-manifest
// @ac AC-23
// `specter init --refresh` updates only domains.default.specs and preserves
// every other field — settings, custom domains, system metadata.
func TestInit_Refresh_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing manifest: domains.default.specs lists spec-a; custom
	// domains.auth.specs lists spec-b; settings.strict is true.
	writeManifestRaw(t, dir, `system:
  name: test-system
  tier: 2
settings:
  specs_dir: specs
  strict: true
domains:
  default:
    tier: 2
    description: "default domain"
    specs:
      - spec-a
  auth:
    tier: 1
    description: "auth domain"
    specs:
      - spec-b
`)
	// On-disk specs: spec-a (existing), spec-b (in auth domain), spec-c (new).
	writeSpec(t, dir, "spec-a.spec.yaml", minimalValidSpec("spec-a", 2, "AC-01"))
	writeSpec(t, dir, "spec-b.spec.yaml", minimalValidSpec("spec-b", 1, "AC-01"))
	writeSpec(t, dir, "spec-c.spec.yaml", minimalValidSpec("spec-c", 3, "AC-01"))

	_, code := runCLI(t, dir, "init", "--refresh")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	got := readManifest(t, dir)
	// default.specs MUST contain spec-a AND spec-c; MUST NOT contain spec-b
	// (claimed by custom domain).
	if !strings.Contains(got, "- spec-a") || !strings.Contains(got, "- spec-c") {
		t.Errorf("default.specs must contain spec-a and spec-c after refresh; got:\n%s", got)
	}
	// settings.strict preserved.
	if !strings.Contains(got, "strict: true") {
		t.Errorf("settings.strict must be preserved; got:\n%s", got)
	}
	// Custom auth domain preserved — name, tier, specs:spec-b.
	if !strings.Contains(got, "auth:") {
		t.Errorf("custom auth domain must be preserved; got:\n%s", got)
	}
	// spec-b must not migrate into default.specs (it belongs to auth).
	// Simple heuristic: count occurrences; spec-b should appear exactly once
	// (under auth.specs, not also under default.specs).
	if strings.Count(got, "- spec-b") != 1 {
		t.Errorf("spec-b must appear exactly once in output (under auth only), got %d times in:\n%s",
			strings.Count(got, "- spec-b"), got)
	}
}

// @spec spec-manifest
// @ac AC-24
// Specs that used to be in domains.default.specs but are no longer on disk
// get removed. The summary line names the change counts.
func TestInit_Refresh_RemovesDeletedSpecs(t *testing.T) {
	dir := t.TempDir()
	writeManifestRaw(t, dir, `system:
  name: test
  tier: 2
settings:
  specs_dir: specs
domains:
  default:
    tier: 2
    description: "default"
    specs:
      - spec-a
      - spec-b
`)
	// Only spec-a exists on disk; spec-b was deleted.
	writeSpec(t, dir, "spec-a.spec.yaml", minimalValidSpec("spec-a", 2, "AC-01"))

	out, _ := runCLI(t, dir, "init", "--refresh")

	got := readManifest(t, dir)
	if strings.Contains(got, "- spec-b") {
		t.Errorf("spec-b must be removed (not on disk); got:\n%s", got)
	}
	if !strings.Contains(got, "- spec-a") {
		t.Errorf("spec-a must be preserved; got:\n%s", got)
	}
	// Summary line indicates the removal.
	if !strings.Contains(out, "-1 removed") && !strings.Contains(out, "1 removed") {
		t.Errorf("summary must mention removed-count; got:\n%s", out)
	}
}

// @spec spec-manifest
// @ac AC-25
// `--dry-run` prints the diff but writes nothing.
func TestInit_Refresh_DryRun_DoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	original := `system:
  name: test
  tier: 2
settings:
  specs_dir: specs
domains:
  default:
    tier: 2
    description: "default"
    specs:
      - spec-a
`
	writeManifestRaw(t, dir, original)
	writeSpec(t, dir, "spec-a.spec.yaml", minimalValidSpec("spec-a", 2, "AC-01"))
	writeSpec(t, dir, "spec-c.spec.yaml", minimalValidSpec("spec-c", 2, "AC-01"))

	out, code := runCLI(t, dir, "init", "--refresh", "--dry-run")
	if code != 0 {
		t.Fatalf("expected exit 0 for --dry-run, got %d", code)
	}

	// File on disk MUST be byte-identical to the original.
	after := readManifest(t, dir)
	if after != original {
		t.Errorf("dry-run must not modify specter.yaml; diff:\nbefore:\n%s\nafter:\n%s", original, after)
	}

	// Output must indicate what WOULD happen (spec-c added).
	if !strings.Contains(out, "spec-c") {
		t.Errorf("dry-run output must name the would-be change (spec-c); got:\n%s", out)
	}
}

// @spec spec-manifest
// @ac AC-26
// --refresh and --force are mutually exclusive. Attempting both fails with
// a clear message and writes nothing.
func TestInit_Refresh_ForceConflict_Errors(t *testing.T) {
	dir := t.TempDir()
	original := `system:
  name: test
  tier: 2
settings:
  specs_dir: specs
domains:
  default:
    tier: 2
    description: "default"
    specs:
      - spec-a
`
	writeManifestRaw(t, dir, original)
	writeSpec(t, dir, "spec-a.spec.yaml", minimalValidSpec("spec-a", 2, "AC-01"))

	out, code := runCLI(t, dir, "init", "--refresh", "--force")
	if code == 0 {
		t.Fatalf("expected non-zero exit for flag conflict, got 0. out:\n%s", out)
	}
	// Error message must mention the conflict.
	if !strings.Contains(strings.ToLower(out), "mutually exclusive") {
		t.Errorf("error message must name the conflict; got:\n%s", out)
	}
	// File on disk unchanged.
	after := readManifest(t, dir)
	if after != original {
		t.Errorf("conflict-error path must not modify specter.yaml")
	}
}

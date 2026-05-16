// Pure-function tests for settings.specs_dir workspace-scope validation
// (spec-manifest 1.11.0 C-30 / AC-46) — v0.13 security audit H1 fix.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// AC-46: workspace-escape paths in settings.specs_dir MUST be rejected
// at parse time. The check happens here so every downstream consumer
// (discoverSpecs, doctor --fix, coverage, sync) inherits the guarantee.
//
// @ac AC-46
func TestParseManifest_SpecsDir_RejectsAbsolutePath(t *testing.T) {
	t.Run("spec-manifest/AC-46 specs_dir rejects absolute Unix path", func(t *testing.T) {
		_, err := ParseManifest("system: { name: x }\nsettings:\n  specs_dir: /etc\n")
		if err == nil {
			t.Fatal("expected error for absolute path, got nil")
		}
		if !strings.Contains(err.Error(), "specs_dir") {
			t.Errorf("expected error to name `specs_dir`, got: %v", err)
		}
	})

	// Platform-independent rejection of Windows drive-letter paths via
	// the explicit drive-letter check in validateSpecsDir. Does NOT
	// carry the AC-46 token in the subtest name because skipping on
	// non-Windows would pollute strict coverage (status: skipped). The
	// Unix absolute case above covers AC-46's canonical evidence.
	t.Run("Windows drive-letter path rejected on every platform", func(t *testing.T) {
		_, err := ParseManifest(`system: { name: x }
settings:
  specs_dir: C:\Users\victim
`)
		if err == nil {
			t.Fatal("expected error for Windows drive-letter path, got nil")
		}
		if !strings.Contains(err.Error(), "specs_dir") {
			t.Errorf("expected error to name `specs_dir`, got: %v", err)
		}
	})
}

// @ac AC-46
func TestParseManifest_SpecsDir_RejectsParentTraversal(t *testing.T) {
	t.Run("spec-manifest/AC-46 specs_dir rejects path containing .. segments", func(t *testing.T) {
		_, err := ParseManifest("system: { name: x }\nsettings:\n  specs_dir: ../../../home/victim\n")
		if err == nil {
			t.Fatal("expected error for parent-traversal path, got nil")
		}
		if !strings.Contains(err.Error(), "specs_dir") {
			t.Errorf("expected error to name `specs_dir`, got: %v", err)
		}
	})

	t.Run("spec-manifest/AC-46 specs_dir rejects lexically-cleaned escape", func(t *testing.T) {
		// foo/../../../home/victim cleans to ../../home/victim — still
		// escapes the workspace. The check must catch this even though
		// the raw string starts with foo/ (not ..).
		_, err := ParseManifest("system: { name: x }\nsettings:\n  specs_dir: foo/../../../home/victim\n")
		if err == nil {
			t.Fatal("expected error for clean-escape path, got nil")
		}
		if !strings.Contains(err.Error(), "specs_dir") {
			t.Errorf("expected error to name `specs_dir`, got: %v", err)
		}
	})
}

// @ac AC-46
func TestParseManifest_SpecsDir_AcceptsRelative(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"single segment", "specs"},
		{"nested two segments", "src/specs"},
		{"current-dir explicit", "./specs"},
		{"current-dir with subdir", "./src/specs"},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-46 specs_dir accepts safe relative "+tc.name, func(t *testing.T) {
			m, err := ParseManifest("system: { name: x }\nsettings:\n  specs_dir: " + tc.path + "\n")
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.path, err)
				return
			}
			if m.Settings.SpecsDir == "" {
				t.Errorf("expected SpecsDir to be set, got empty")
			}
		})
	}
}

// @ac AC-46
func TestParseManifest_SpecsDir_DefaultUnchanged(t *testing.T) {
	t.Run("spec-manifest/AC-46 unset specs_dir defaults to specs (no error)", func(t *testing.T) {
		m, err := ParseManifest("system: { name: x }\n")
		if err != nil {
			t.Fatalf("ParseManifest with default settings: %v", err)
		}
		if m.SpecsDir() != "specs" {
			t.Errorf("expected default SpecsDir() = `specs`, got %q", m.SpecsDir())
		}
	})
}

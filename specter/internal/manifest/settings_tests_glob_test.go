// Pure-function tests for settings.tests_glob parsing (C-25, AC-39).
//
// @spec spec-manifest
package manifest

import (
	"reflect"
	"testing"
)

// @ac AC-39
func TestParseManifest_TestsGlob_StringForm(t *testing.T) {
	t.Run("spec-manifest/AC-39 tests_glob string form normalizes to single-element slice", func(t *testing.T) {
		m, err := ParseManifest("system: { name: x }\nsettings:\n  tests_glob: tests/**/*.py\n")
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}
		want := []string{"tests/**/*.py"}
		if !reflect.DeepEqual(m.Settings.TestsGlob, want) {
			t.Errorf("TestsGlob = %v, want %v", m.Settings.TestsGlob, want)
		}
	})
}

// @ac AC-39
func TestParseManifest_TestsGlob_ListForm(t *testing.T) {
	t.Run("spec-manifest/AC-39 tests_glob list form preserves all entries", func(t *testing.T) {
		body := "system: { name: x }\nsettings:\n  tests_glob:\n    - tests/**/*.py\n    - integration/**/*.py\n"
		m, err := ParseManifest(body)
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}
		want := []string{"tests/**/*.py", "integration/**/*.py"}
		if !reflect.DeepEqual(m.Settings.TestsGlob, want) {
			t.Errorf("TestsGlob = %v, want %v", m.Settings.TestsGlob, want)
		}
	})
}

// @ac AC-39
func TestParseManifest_TestsGlob_UnsetIsEmpty(t *testing.T) {
	t.Run("spec-manifest/AC-39 unset tests_glob is empty (not nil panic)", func(t *testing.T) {
		m, err := ParseManifest("system: { name: x }\n")
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}
		if len(m.Settings.TestsGlob) != 0 {
			t.Errorf("expected empty TestsGlob, got %v", m.Settings.TestsGlob)
		}
	})
}

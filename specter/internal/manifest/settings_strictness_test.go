// Pure-function tests for settings.strictness parsing (C-24, AC-37, AC-38).
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// @ac AC-37
func TestParseManifest_Strictness_AcceptsAllThreeValues(t *testing.T) {
	t.Run("spec-manifest/AC-37 settings.strictness accepts annotation/threshold/zero-tolerance", func(t *testing.T) {
		cases := map[string]string{
			"annotation":     "system: { name: x }\nsettings:\n  strictness: annotation\n",
			"threshold":      "system: { name: x }\nsettings:\n  strictness: threshold\n",
			"zero-tolerance": "system: { name: x }\nsettings:\n  strictness: zero-tolerance\n",
		}
		for want, yamlBody := range cases {
			m, err := ParseManifest(yamlBody)
			if err != nil {
				t.Errorf("ParseManifest(%q): %v", want, err)
				continue
			}
			if m.Settings.Strictness != want {
				t.Errorf("ParseManifest(%q): Settings.Strictness = %q, want %q", want, m.Settings.Strictness, want)
			}
		}
	})
}

// @ac AC-37
func TestParseManifest_Strictness_DefaultsToThreshold(t *testing.T) {
	t.Run("spec-manifest/AC-37 unset strictness defaults to threshold", func(t *testing.T) {
		m, err := ParseManifest("system: { name: x }\nsettings:\n  specs_dir: specs\n")
		if err != nil {
			t.Fatalf("ParseManifest: %v", err)
		}
		if m.Settings.Strictness != "threshold" {
			t.Errorf("expected default Strictness = \"threshold\", got %q", m.Settings.Strictness)
		}
	})
}

// @ac AC-38
func TestParseManifest_Strictness_RejectsInvalidValue(t *testing.T) {
	t.Run("spec-manifest/AC-38 invalid strictness errors with clear message", func(t *testing.T) {
		_, err := ParseManifest("system: { name: x }\nsettings:\n  strictness: bogus\n")
		if err == nil {
			t.Fatal("expected error for invalid strictness, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "strictness") {
			t.Errorf("expected 'strictness' in error, got: %s", msg)
		}
		// Error must list the valid values.
		for _, valid := range []string{"annotation", "threshold", "zero-tolerance"} {
			if !strings.Contains(msg, valid) {
				t.Errorf("expected error to list valid value %q, got: %s", valid, msg)
			}
		}
	})
}

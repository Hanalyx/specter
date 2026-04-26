// Pure-function tests for unknown-key rejection in the settings block (C-26, AC-40).
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// @ac AC-40
func TestParseManifest_UnknownSettingsKey_ErrorsWithDidYouMean(t *testing.T) {
	t.Run("spec-manifest/AC-40 typo'd settings key errors with did-you-mean suggestion", func(t *testing.T) {
		// `test_glob` is a one-character distance from the real `tests_glob`.
		body := "system: { name: x }\nsettings:\n  test_glob: tests/**/*.py\n"
		_, err := ParseManifest(body)
		if err == nil {
			t.Fatal("expected error for unknown settings key, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "test_glob") {
			t.Errorf("expected error to name the offending key 'test_glob', got: %s", msg)
		}
		if !strings.Contains(strings.ToLower(msg), "did you mean") {
			t.Errorf("expected 'did you mean' suggestion, got: %s", msg)
		}
		if !strings.Contains(msg, "tests_glob") {
			t.Errorf("expected suggestion to name 'tests_glob', got: %s", msg)
		}
	})
}

// @ac AC-40
func TestParseManifest_UnknownSettingsKey_FarFromValid_NoSuggestion(t *testing.T) {
	t.Run("spec-manifest/AC-40 wildly typo'd key errors but omits did-you-mean", func(t *testing.T) {
		// 'xyz_random_thing' is far from any valid settings key (Levenshtein >> 3).
		body := "system: { name: x }\nsettings:\n  xyz_random_thing: 123\n"
		_, err := ParseManifest(body)
		if err == nil {
			t.Fatal("expected error for unknown settings key, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "xyz_random_thing") {
			t.Errorf("expected error to name the offending key, got: %s", msg)
		}
		// No close match → no did-you-mean.
		if strings.Contains(strings.ToLower(msg), "did you mean") {
			t.Errorf("expected no did-you-mean for far-distance typo, got: %s", msg)
		}
	})
}

// @ac AC-40
func TestParseManifest_UnknownTopLevelKey_ErrorsCleanly(t *testing.T) {
	t.Run("spec-manifest/AC-40 unknown top-level manifest key also errors", func(t *testing.T) {
		// Top-level (sibling of system/domains/settings/registry) — same rule.
		body := "system: { name: x }\nbogus_top_level: yes\n"
		_, err := ParseManifest(body)
		if err == nil {
			t.Fatal("expected error for unknown top-level key, got nil")
		}
		if !strings.Contains(err.Error(), "bogus_top_level") {
			t.Errorf("expected error to name 'bogus_top_level', got: %v", err)
		}
	})
}

// Regression guard: every existing valid settings key must still parse.
func TestParseManifest_AllValidSettingsKeys_StillParse(t *testing.T) {
	t.Run("spec-manifest/regression all valid settings keys parse cleanly", func(t *testing.T) {
		body := `
system:
  name: regression-system
settings:
  specs_dir: specs
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
  exclude:
    - .git
  strict: false
  warn_on_draft: true
  tier_overrides:
    spec-foo: 1
  tests_glob: tests/**/*.py
  strictness: zero-tolerance
`
		_, err := ParseManifest(body)
		if err != nil {
			t.Errorf("expected clean parse for all valid keys, got error: %v", err)
		}
	})
}

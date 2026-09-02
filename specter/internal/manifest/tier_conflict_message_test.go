package manifest

import (
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

// @spec spec-manifest
// @ac AC-60
//
// C-14: the override is not applied, so the warning must say the declared tier
// governs and must not claim the override is in use. The negative assertion is
// the load-bearing half: the prior message ended "using override (1)" while
// coverage reported the declared tier for the same spec on the same run.
func TestTierConflictMessage_DoesNotClaimTheOverrideApplies(t *testing.T) {
	t.Run("spec-manifest/AC-60 tier conflict message does not claim the override applies", func(t *testing.T) {
		m, err := ParseManifest(`system:
  name: test
settings:
  tier_overrides:
    lo: 1
`)
		if err != nil {
			t.Fatalf("ParseManifest error: %v", err)
		}
		specs := []schema.SpecAST{{ID: "lo", Tier: 3}}

		warnings := CheckTierConflicts(specs, m)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 tier_conflict warning, got %d", len(warnings))
		}
		msg := warnings[0].Message

		// Positive control: the warning still names both tiers, so this test
		// cannot pass by the warning disappearing.
		if !strings.Contains(msg, "3") || !strings.Contains(msg, "1") {
			t.Errorf("warning must name the declared tier 3 and the override 1, got: %q", msg)
		}

		// C-14, negative half. This is what fails today.
		if strings.Contains(msg, "using override") {
			t.Errorf("message must not assert the override is in use; nothing applies it. got: %q", msg)
		}

		// C-14, positive half: it must say what actually governs.
		lower := strings.ToLower(msg)
		if !strings.Contains(lower, "declared tier governs") {
			t.Errorf("message must state that the declared tier governs, got: %q", msg)
		}
	})
}

// tier_conflict.go — detects mismatches between spec-declared tier and tier_overrides.
//
// @spec spec-manifest
package manifest

import (
	"fmt"

	"github.com/Hanalyx/specter/internal/schema"
)

// TierConflictWarning is emitted when a spec's declared tier differs from its tier_override.
type TierConflictWarning struct {
	SpecID       string
	SpecTier     int
	OverrideTier int
	Message      string
}

// CheckTierConflicts returns a warning for each spec whose declared tier disagrees
// with the manifest's settings.tier_overrides entry for that spec.
//
// A conflict exists when: TierOverrides[spec.ID] is set AND spec.Tier > 0 AND TierOverrides[spec.ID] != spec.Tier.
// If spec.Tier == 0 (not declared), the override is the intended mechanism — no conflict.
//
// C-14
func CheckTierConflicts(specs []schema.SpecAST, m *Manifest) []TierConflictWarning {
	if len(m.Settings.TierOverrides) == 0 {
		return nil
	}
	var warnings []TierConflictWarning
	for _, spec := range specs {
		override, ok := m.Settings.TierOverrides[spec.ID]
		if !ok {
			continue
		}
		if spec.Tier == 0 || spec.Tier == override {
			continue
		}
		warnings = append(warnings, TierConflictWarning{
			SpecID:       spec.ID,
			SpecTier:     spec.Tier,
			OverrideTier: override,
			Message: fmt.Sprintf(
				"spec %q declares tier: %d but specter.yaml tier_overrides assigns tier: %d — using override (%d)",
				spec.ID, spec.Tier, override, override,
			),
		})
	}
	return warnings
}

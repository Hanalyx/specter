package manifest

import (
	"fmt"
	"sort"

	"github.com/Hanalyx/specter/internal/schema"
)

// DomainTierConflictWarning reports a spec whose declared tier disagrees with
// the tier asserted by a domain listing it.
type DomainTierConflictWarning struct {
	SpecID     string
	SpecTier   int
	Domain     string
	DomainTier int
	Message    string
}

// CheckDomainTierConflicts implements spec-manifest C-35.
//
// The domain tier is a checked assertion, not a tier source. This reports a
// disagreement and resolves nothing: the declared tier still governs every
// downstream decision, per C-04.
//
// Pure, no I/O, per C-10. The precedent is CheckTierConflicts (C-14).
//
// Domains are visited in sorted name order, because Manifest.Domains is a map
// and a human diffing two runs should see the same list twice. That is the
// determinism defect bugs/SP-SP-009 recorded for test_files and
// bugs/SP-SP-051 records for resolve.
func CheckDomainTierConflicts(specs []schema.SpecAST, m *Manifest) []DomainTierConflictWarning {
	if m == nil || len(m.Domains) == 0 {
		return nil
	}
	declared := make(map[string]int, len(specs))
	for _, s := range specs {
		declared[s.ID] = s.Tier
	}

	names := make([]string, 0, len(m.Domains))
	for name := range m.Domains {
		names = append(names, name)
	}
	sort.Strings(names)

	var warnings []DomainTierConflictWarning
	for _, name := range names {
		domain := m.Domains[name]
		if domain.Tier == 0 {
			continue
		}
		for _, specID := range domain.Specs {
			tier, ok := declared[specID]
			if !ok || tier == 0 || tier == domain.Tier {
				continue
			}
			warnings = append(warnings, DomainTierConflictWarning{
				SpecID:     specID,
				SpecTier:   tier,
				Domain:     name,
				DomainTier: domain.Tier,
				Message: fmt.Sprintf(
					"spec %q declares tier: %d but domain %q asserts tier: %d. "+
						"The declared tier governs; the domain tier is a checked assertion and is not applied.",
					specID, tier, name, domain.Tier,
				),
			})
		}
	}
	return warnings
}

// DeprecatedTierKeys implements spec-manifest C-36. It returns one message per
// deprecated tier key the manifest declares, in a stable order.
//
// Both keys are validated, range-checked and inert. An operator who sets either
// currently gets no signal that it did nothing, which is the defect
// bugs/SP-SP-049 and bugs/SP-SP-001 record.
func DeprecatedTierKeys(m *Manifest) []string {
	if m == nil {
		return nil
	}
	var msgs []string
	if m.System.Tier > 0 {
		msgs = append(msgs, "system.tier is deprecated and has no effect. "+
			"Set tier: in each .spec.yaml file. It is removed at v1.0.0.")
	}
	if len(m.Settings.TierOverrides) > 0 {
		// Not "has no effect", which system.tier above can say and this key
		// cannot. It resolves no tier, and its value is still compared under
		// C-14: a mismatching override produces a tier_conflict, which
		// spec-check C-07 promotes to an error under --strict. Telling an
		// operator the setting does nothing, and then failing their strict
		// build over it, is the shape of misdescription C-14 already forbids
		// one clause up.
		msgs = append(msgs, "settings.tier_overrides is deprecated and resolves no tier. "+
			"Its value is still compared: one that disagrees with a spec's declared tier "+
			"warns, and --strict makes that an error. Set tier: in each .spec.yaml file. "+
			"It is removed at v1.0.0.")
	}
	return msgs
}

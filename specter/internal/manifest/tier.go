package manifest

// ResolveTier determines the effective tier for a spec using cascade:
// 1. Explicit spec tier (if > 0)
// 2. Domain tier (if spec belongs to a domain with a tier)
// 3. System default tier (if set)
// 4. Hardcoded default: 2
func ResolveTier(specID string, specTier int, m *Manifest) int {
	// Explicit spec tier takes priority
	if specTier > 0 {
		return specTier
	}

	// Check domain tier
	if m != nil {
		for _, domain := range m.Domains {
			for _, sid := range domain.Specs {
				if sid == specID && domain.Tier > 0 {
					return domain.Tier
				}
			}
		}

		// System default tier
		if m.System.Tier > 0 {
			return m.System.Tier
		}
	}

	// Hardcoded default
	return 2
}

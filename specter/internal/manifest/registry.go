package manifest

import (
	"sort"

	"github.com/Hanalyx/specter/internal/schema"
)

// BuildRegistryFromSpecs creates registry entries from parsed specs.
// files maps spec ID to file path.
func BuildRegistryFromSpecs(specs []schema.SpecAST, files map[string]string, m *Manifest) []RegistryEntry {
	entries := make([]RegistryEntry, 0, len(specs))

	for _, spec := range specs {
		tier := ResolveTier(spec.ID, spec.Tier, m)
		domain := SpecDomain(spec.ID, m)

		entries = append(entries, RegistryEntry{
			ID:      spec.ID,
			File:    files[spec.ID],
			Version: spec.Version,
			Status:  spec.Status,
			Tier:    tier,
			Domain:  domain,
		})
	}

	// Sort by ID for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return entries
}

// UpdateRegistry returns a new Manifest with the registry rebuilt from specs.
func UpdateRegistry(m *Manifest, specs []schema.SpecAST, files map[string]string) *Manifest {
	updated := *m
	updated.Registry = BuildRegistryFromSpecs(specs, files, m)
	return &updated
}

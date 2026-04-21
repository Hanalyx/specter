package manifest

import (
	"sort"
)

// RefreshDiff describes the change that `specter init --refresh` would apply
// to `domains.default.specs`.
type RefreshDiff struct {
	Added   []string // discovered on disk, not previously in domains.default.specs
	Removed []string // previously in domains.default.specs, no longer discoverable
}

// RefreshManifestDomains computes the non-destructive update that `specter
// init --refresh` applies. Given an existing manifest and the set of spec
// IDs currently discoverable on disk, it returns (modified manifest, diff)
// where:
//
//   - `domains.default.specs` is replaced with the deduplicated union of
//     (existing default.specs ∩ discovered) + (discovered not claimed by
//     any non-default domain).
//   - Every other field of the manifest is preserved — including custom
//     domains (anything under `domains.*` that isn't `default`), settings,
//     registry entries, tier overrides, system metadata.
//   - Specs that are in a non-default domain are NOT added to default
//     even if they were discovered on disk. A spec belongs to one domain.
//
// Pure: does not mutate the input manifest. Returns a copy with the
// updated default-domain specs list, plus a diff describing the change.
func RefreshManifestDomains(existing *Manifest, discoveredIDs []string) (*Manifest, RefreshDiff) {
	// Dedupe + index discovered IDs.
	discovered := make(map[string]bool, len(discoveredIDs))
	for _, id := range discoveredIDs {
		discovered[id] = true
	}

	// Collect IDs claimed by non-default domains so we don't migrate them.
	claimedByCustom := make(map[string]bool)
	existingDefault := []string{}
	for domainName, cfg := range existing.Domains {
		if domainName == "default" {
			existingDefault = append(existingDefault, cfg.Specs...)
			continue
		}
		for _, id := range cfg.Specs {
			claimedByCustom[id] = true
		}
	}

	// Compute the new default list: every discovered ID that isn't in a
	// custom domain. Sorted for stability.
	newDefault := make([]string, 0, len(discovered))
	for id := range discovered {
		if !claimedByCustom[id] {
			newDefault = append(newDefault, id)
		}
	}
	sort.Strings(newDefault)

	// Compute diff against the pre-existing default list.
	existingDefaultSet := make(map[string]bool, len(existingDefault))
	for _, id := range existingDefault {
		existingDefaultSet[id] = true
	}
	newDefaultSet := make(map[string]bool, len(newDefault))
	for _, id := range newDefault {
		newDefaultSet[id] = true
	}

	var diff RefreshDiff
	for _, id := range newDefault {
		if !existingDefaultSet[id] {
			diff.Added = append(diff.Added, id)
		}
	}
	for _, id := range existingDefault {
		if !newDefaultSet[id] {
			diff.Removed = append(diff.Removed, id)
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)

	// Build a copy of the manifest with only the default-domain specs
	// changed.
	out := *existing // shallow copy of top-level
	out.Domains = make(map[string]DomainConfig, len(existing.Domains))
	for name, cfg := range existing.Domains {
		if name == "default" {
			cfg.Specs = newDefault
		}
		out.Domains[name] = cfg
	}
	// If there was no `default` domain at all, create one. The scaffolder
	// (C-16) already enforces `domains:` always has a default; this guard
	// is for pre-existing manifests written by other tools.
	if _, ok := out.Domains["default"]; !ok {
		out.Domains["default"] = DomainConfig{
			Tier:        existing.System.Tier,
			Description: "Default domain",
			Specs:       newDefault,
		}
	}

	return &out, diff
}

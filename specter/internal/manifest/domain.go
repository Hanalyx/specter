package manifest

import "github.com/Hanalyx/specter/internal/coverage"

// SpecDomain returns the domain name for a spec ID, or "" if unassigned.
func SpecDomain(specID string, m *Manifest) string {
	if m == nil {
		return ""
	}
	for name, domain := range m.Domains {
		for _, sid := range domain.Specs {
			if sid == specID {
				return name
			}
		}
	}
	return ""
}

// DomainCoverage aggregates spec-level coverage into domain-level summaries.
func DomainCoverage(report *coverage.CoverageReport, m *Manifest) []DomainCoverageEntry {
	if m == nil || report == nil || len(m.Domains) == 0 {
		return nil
	}

	// Build domain -> entries map
	domainEntries := make(map[string][]coverage.SpecCoverageEntry)
	unassigned := make([]coverage.SpecCoverageEntry, 0)

	for _, entry := range report.Entries {
		domain := SpecDomain(entry.SpecID, m)
		if domain != "" {
			domainEntries[domain] = append(domainEntries[domain], entry)
		} else {
			unassigned = append(unassigned, entry)
		}
	}

	var results []DomainCoverageEntry

	// Process each domain in order
	for name, config := range m.Domains {
		entries := domainEntries[name]
		if len(entries) == 0 {
			continue
		}

		tier := config.Tier
		if tier == 0 {
			tier = 2
		}

		var totalCoverage float64
		passing := 0
		failing := 0
		for _, e := range entries {
			totalCoverage += e.CoveragePct
			if e.PassesThreshold {
				passing++
			} else {
				failing++
			}
		}

		results = append(results, DomainCoverageEntry{
			Domain:      name,
			Tier:        tier,
			TotalSpecs:  len(entries),
			Passing:     passing,
			Failing:     failing,
			AvgCoverage: totalCoverage / float64(len(entries)),
		})
	}

	// Add unassigned group if any
	if len(unassigned) > 0 {
		var totalCoverage float64
		passing := 0
		failing := 0
		for _, e := range unassigned {
			totalCoverage += e.CoveragePct
			if e.PassesThreshold {
				passing++
			} else {
				failing++
			}
		}
		results = append(results, DomainCoverageEntry{
			Domain:      "(unassigned)",
			Tier:        0,
			TotalSpecs:  len(unassigned),
			Passing:     passing,
			Failing:     failing,
			AvgCoverage: totalCoverage / float64(len(unassigned)),
		})
	}

	return results
}

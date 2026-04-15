// Package manifest implements project manifest (specter.yaml) support.
//
// Provides parsing, validation, tier inheritance, domain grouping,
// and spec registry management.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-manifest
package manifest

// Manifest is the top-level specter.yaml structure.
type Manifest struct {
	System   SystemConfig            `yaml:"system" json:"system"`
	Domains  map[string]DomainConfig `yaml:"domains,omitempty" json:"domains,omitempty"`
	Settings Settings                `yaml:"settings,omitempty" json:"settings,omitempty"`
	Registry []RegistryEntry         `yaml:"registry,omitempty" json:"registry,omitempty"`
}

// SystemConfig defines the system-level metadata.
type SystemConfig struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Tier        int    `yaml:"tier,omitempty" json:"tier,omitempty"`
}

// DomainConfig groups related specs with shared properties.
type DomainConfig struct {
	Tier        int      `yaml:"tier,omitempty" json:"tier,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Specs       []string `yaml:"specs,omitempty" json:"specs,omitempty"`
	Owner       string   `yaml:"owner,omitempty" json:"owner,omitempty"`
}

// Settings holds project-level configuration.
type Settings struct {
	SpecsDir      string         `yaml:"specs_dir,omitempty" json:"specs_dir,omitempty"`
	Coverage      CoverageConfig `yaml:"coverage,omitempty" json:"coverage,omitempty"`
	Exclude       []string       `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Strict        bool           `yaml:"strict,omitempty" json:"strict,omitempty"`             // C-11: treat warnings as errors
	WarnOnDraft   bool           `yaml:"warn_on_draft,omitempty" json:"warn_on_draft,omitempty"` // C-12: warn on draft specs
	TierOverrides map[string]int `yaml:"tier_overrides,omitempty" json:"tier_overrides,omitempty"` // C-14: per-spec tier overrides
}

// CoverageConfig defines per-tier coverage thresholds.
type CoverageConfig struct {
	Tier1 int `yaml:"tier1,omitempty" json:"tier1,omitempty"`
	Tier2 int `yaml:"tier2,omitempty" json:"tier2,omitempty"`
	Tier3 int `yaml:"tier3,omitempty" json:"tier3,omitempty"`
}

// RegistryEntry is a persistent index entry for a known spec.
type RegistryEntry struct {
	ID      string `yaml:"id" json:"id"`
	File    string `yaml:"file" json:"file"`
	Version string `yaml:"version" json:"version"`
	Status  string `yaml:"status" json:"status"`
	Tier    int    `yaml:"tier" json:"tier"`
	Domain  string `yaml:"domain,omitempty" json:"domain,omitempty"`
}

// DomainCoverageEntry is an aggregated coverage summary for a domain.
type DomainCoverageEntry struct {
	Domain      string  `json:"domain"`
	Tier        int     `json:"tier"`
	TotalSpecs  int     `json:"total_specs"`
	Passing     int     `json:"passing"`
	Failing     int     `json:"failing"`
	AvgCoverage float64 `json:"avg_coverage"`
}

// CoverageThresholds returns the coverage thresholds as a map for use by
// checker and coverage packages.
func (m *Manifest) CoverageThresholds() map[int]int {
	t := map[int]int{1: 100, 2: 80, 3: 50}
	if m.Settings.Coverage.Tier1 > 0 {
		t[1] = m.Settings.Coverage.Tier1
	}
	if m.Settings.Coverage.Tier2 > 0 {
		t[2] = m.Settings.Coverage.Tier2
	}
	if m.Settings.Coverage.Tier3 > 0 {
		t[3] = m.Settings.Coverage.Tier3
	}
	return t
}

// SpecsDir returns the configured specs directory or the default "specs".
func (m *Manifest) SpecsDir() string {
	if m.Settings.SpecsDir != "" {
		return m.Settings.SpecsDir
	}
	return "specs"
}

// ExcludePatterns returns the configured exclude patterns with defaults.
func (m *Manifest) ExcludePatterns() []string {
	if len(m.Settings.Exclude) > 0 {
		return m.Settings.Exclude
	}
	return []string{"node_modules", "dist", ".git", "vendor", "__pycache__", ".next"}
}

// ResolveTierWithOverrides returns the effective tier for a spec, applying
// TierOverrides from the manifest if present. Override takes precedence.
func (m *Manifest) ResolveTierWithOverrides(specID string, specTier int) int {
	if override, ok := m.Settings.TierOverrides[specID]; ok {
		return override
	}
	// Fall back to existing ResolveTier logic (domain -> system -> default 2)
	return ResolveTier(specID, specTier, m)
}


// Package manifest implements project manifest (specter.yaml) support.
//
// Provides parsing, validation, tier inheritance, domain grouping,
// and spec registry management.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-manifest
package manifest

import "gopkg.in/yaml.v3"

// Manifest is the top-level specter.yaml structure.
//
// SchemaVersion (spec-manifest C-27, v0.12+) records the spec-schema draft
// this project commits to. Pre-1.0 the value is a placeholder (always 1;
// the schema is draft during v0.x and locks at v1.0.0). After lock,
// breaking schema changes bump the integer and require a `doctor --fix`
// migration. The field is declared FIRST so gopkg.in/yaml.v3 emits it as
// the first key in scaffolded manifests (AC-42).
type Manifest struct {
	SchemaVersion int                     `yaml:"schema_version,omitempty" json:"schema_version,omitempty"`
	System        SystemConfig            `yaml:"system" json:"system"`
	Domains       map[string]DomainConfig `yaml:"domains,omitempty" json:"domains,omitempty"`
	Settings      Settings                `yaml:"settings,omitempty" json:"settings,omitempty"`
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
//
// The two `declared` fields record which keys the settings block actually
// carried. Value alone cannot answer that question for either of them.
// C-24 rewrites an absent `strictness` to `threshold` at parse time, so the
// parsed string reads the same for an absent key and an explicit
// `threshold`. And gopkg.in/yaml.v3 leaves Annotation nil for a bare
// `annotation:` while allocating it for `annotation: {}`, so the decoded
// pointer alone reads one declared form as absent. Both are recorded from the
// raw key set in UnmarshalYAML (C-32, C-34(a)).
type Settings struct {
	SpecsDir      string            `yaml:"specs_dir,omitempty" json:"specs_dir,omitempty"`
	Coverage      CoverageConfig    `yaml:"coverage,omitempty" json:"coverage,omitempty"`
	Exclude       []string          `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Strict        bool              `yaml:"strict,omitempty" json:"strict,omitempty"`                 // C-11: treat warnings as errors
	WarnOnDraft   bool              `yaml:"warn_on_draft,omitempty" json:"warn_on_draft,omitempty"`   // C-12: warn on draft specs
	TierOverrides map[string]int    `yaml:"tier_overrides,omitempty" json:"tier_overrides,omitempty"` // C-14: per-spec tier overrides
	TestsGlob     StringOrList      `yaml:"tests_glob,omitempty" json:"tests_glob,omitempty"`         // C-25: default test-discovery glob (string or list)
	Strictness    string            `yaml:"strictness,omitempty" json:"strictness,omitempty"`         // C-24: annotation | threshold (default) | zero-tolerance
	Annotation    *AnnotationConfig `yaml:"annotation,omitempty" json:"annotation,omitempty"`         // C-32: nil when the manifest carries no `annotation` key

	annotationDeclared bool
	strictnessDeclared bool
}

// AnnotationConfig is the body of `settings.annotation`. It carries one
// sub-key, `permissive`, which defaults to false (C-32).
//
// Declaredness of the block and the value of Permissive are separate facts.
// Both empty forms, `annotation: {}` and a bare `annotation:`, produce a
// non-nil AnnotationConfig with Permissive false.
//
// `scope` is deliberately not a field here. SSRB-104 section 7.7 stages it
// out of v0.15.0 and ParseManifest rejects it as a validation rule (C-33),
// so the manifest surface gains nothing inert.
type AnnotationConfig struct {
	Permissive bool `yaml:"permissive,omitempty" json:"permissive,omitempty"`
}

// UnmarshalYAML decodes the settings fields and records which of `annotation`
// and `strictness` the document actually carried. The alias type is what
// keeps this from recursing, the same shape CoverageConfig uses.
//
// C-32 requires Annotation to be non-nil whenever the manifest carries the
// key, so a bare `annotation:` (which decodes to a nil pointer) is allocated
// here from the raw key set rather than inferred from the pointer.
func (s *Settings) UnmarshalYAML(node *yaml.Node) error {
	type plain Settings
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	*s = Settings(p)

	// A mapping node holds keys and values in one alternating list.
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "annotation":
			s.annotationDeclared = true
		case "strictness":
			s.strictnessDeclared = true
		}
	}
	if s.annotationDeclared && s.Annotation == nil {
		s.Annotation = &AnnotationConfig{}
	}
	return nil
}

// CoverageConfig defines per-tier coverage thresholds.
//
// The three `set` fields record which keys the manifest declared. A declared
// `tier2: 0` and an absent `tier2` are different instructions (spec-coverage
// C-36) and an int reads 0 for both, so presence is captured at unmarshal
// time. They are unexported, so a CoverageConfig built in process keeps the
// older greater-than-zero behavior and existing callers are unaffected.
type CoverageConfig struct {
	Tier1 int `yaml:"tier1,omitempty" json:"tier1,omitempty"`
	Tier2 int `yaml:"tier2,omitempty" json:"tier2,omitempty"`
	Tier3 int `yaml:"tier3,omitempty" json:"tier3,omitempty"`

	tier1Set bool
	tier2Set bool
	tier3Set bool
}

// UnmarshalYAML decodes the tier fields and records which of them the document
// actually carried. The alias type is what keeps this from recursing.
func (c *CoverageConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain CoverageConfig
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	*c = CoverageConfig(p)

	// A mapping node holds keys and values in one alternating list.
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "tier1":
			c.tier1Set = true
		case "tier2":
			c.tier2Set = true
		case "tier3":
			c.tier3Set = true
		}
	}
	return nil
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
//
// C-36: a declared 0 overrides the built-in default rather than falling
// through to it. Presence decides, so `tier2: 0` gives Tier 2 specs a
// threshold of zero while an absent tier2 still inherits 80.
func (m *Manifest) CoverageThresholds() map[int]int {
	t := map[int]int{1: 100, 2: 80, 3: 50}
	c := m.Settings.Coverage
	if c.tier1Set || c.Tier1 > 0 {
		t[1] = c.Tier1
	}
	if c.tier2Set || c.Tier2 > 0 {
		t[2] = c.Tier2
	}
	if c.tier3Set || c.Tier3 > 0 {
		t[3] = c.Tier3
	}
	return t
}

// GoverningStrictness returns the strictness level that governs a run, before
// any `--strictness` flag is applied.
//
// C-34(d): while a `settings.annotation` block is declared, `settings.strictness`
// stops mattering. What governs instead is set by the coverage rule in roadmap
// item 1D-b. The v0.15.0 interim answer is the strict path at `threshold`,
// which is the C-24 default.
//
// A declared block CAN change the outcome, and an earlier version of this
// comment claimed otherwise. Measured 2026-08-22: a workspace on
// `strictness: annotation` with annotated tests and no results file exits 0,
// and adding `annotation: {}` moves it to exit 1 on the strict path. The C-34
// warning fires, so the change is visible rather than silent.
//
// `threshold` was kept anyway, because the alternative is worse. Measured on
// the same workspace shape at `strictness: threshold`, which is the C-24
// default and therefore the common case: an interim of `annotation` takes a
// run from exit 1 to exit 0 and silently drops the strict path. A visible red
// build on an opt-in key beats a gate that quietly stops gating.
//
// With no block declared the manifest value is returned verbatim, empty string
// included, so every caller's existing fallback keeps its meaning.
func (m *Manifest) GoverningStrictness() string {
	// Reads annotationDeclared, not the Annotation pointer, so this and
	// CheckAnnotationStrictnessConflict answer declaredness the same way.
	// They agree for anything from ParseManifest and diverge for a
	// hand-built Manifest, where setting Annotation alone would take the
	// override without triggering the warning.
	if m.Settings.annotationDeclared {
		return "threshold"
	}
	return m.Settings.Strictness
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

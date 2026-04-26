package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// MaxManifestBytes caps the input size before yaml.Unmarshal to prevent
// memory exhaustion via billion-laughs / anchor-expansion on a malicious
// specter.yaml. Real manifests are tiny (a few hundred lines max);
// 64 KiB is generous.
const MaxManifestBytes = 64 << 10 // 64 KiB

// ParseManifest parses and validates a specter.yaml content string.
func ParseManifest(yamlContent string) (*Manifest, error) {
	if len(yamlContent) > MaxManifestBytes {
		return nil, fmt.Errorf("specter.yaml exceeds %d byte limit (got %d bytes)", MaxManifestBytes, len(yamlContent))
	}
	var m Manifest
	if err := yaml.Unmarshal([]byte(yamlContent), &m); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if m.System.Name == "" {
		return nil, fmt.Errorf("system.name is required")
	}

	if err := validateTier(m.System.Tier, "system.tier"); err != nil {
		return nil, err
	}

	for name, domain := range m.Domains {
		if err := validateTier(domain.Tier, fmt.Sprintf("domains.%s.tier", name)); err != nil {
			return nil, err
		}
	}

	if err := validateCoverageConfig(m.Settings.Coverage); err != nil {
		return nil, err
	}

	return &m, nil
}

// Defaults returns a Manifest with sensible defaults for use when no specter.yaml exists.
func Defaults() *Manifest {
	return &Manifest{
		System: SystemConfig{
			Name: "",
		},
		Settings: Settings{
			SpecsDir: "specs",
			Coverage: CoverageConfig{
				Tier1: 100,
				Tier2: 80,
				Tier3: 50,
			},
		},
	}
}

func validateTier(tier int, field string) error {
	if tier != 0 && (tier < 1 || tier > 3) {
		return fmt.Errorf("%s must be 1, 2, or 3 (got %d)", field, tier)
	}
	return nil
}

func validateCoverageConfig(c CoverageConfig) error {
	for _, pair := range []struct {
		val  int
		name string
	}{
		{c.Tier1, "settings.coverage.tier1"},
		{c.Tier2, "settings.coverage.tier2"},
		{c.Tier3, "settings.coverage.tier3"},
	} {
		if pair.val != 0 && (pair.val < 0 || pair.val > 100) {
			return fmt.Errorf("%s must be 0-100 (got %d)", pair.name, pair.val)
		}
	}
	return nil
}

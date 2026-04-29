package manifest

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxManifestBytes caps the input size before yaml.Unmarshal to prevent
// memory exhaustion via billion-laughs / anchor-expansion on a malicious
// specter.yaml. Real manifests are tiny (a few hundred lines max);
// 64 KiB is generous.
const MaxManifestBytes = 64 << 10 // 64 KiB

// validTopLevelKeys lists every key allowed at the manifest top level.
// Updated when adding a new top-level field.
var validTopLevelKeys = []string{"system", "domains", "settings", "registry"}

// validSettingsKeys lists every key allowed under `settings:`. Updated when
// adding a new settings field.
var validSettingsKeys = []string{
	"specs_dir", "coverage", "exclude", "strict", "warn_on_draft",
	"tier_overrides", "tests_glob", "strictness",
}

// validStrictnessValues enumerates the three allowed strictness levels.
var validStrictnessValues = []string{"annotation", "threshold", "zero-tolerance"}

// ParseManifest parses and validates a specter.yaml content string.
//
// C-26: rejects unknown top-level and settings keys with a did-you-mean
// suggestion when the offending key is within Levenshtein 3 of a valid one.
// C-24: validates settings.strictness against the enum {annotation,
// threshold, zero-tolerance} and applies the default ("threshold") when unset.
func ParseManifest(yamlContent string) (*Manifest, error) {
	// Step 0: input size cap. Cheapest check first — caps a malicious
	// manifest before yaml.Unmarshal allocates on it.
	if len(yamlContent) > MaxManifestBytes {
		return nil, fmt.Errorf("specter.yaml exceeds %d byte limit (got %d bytes)", MaxManifestBytes, len(yamlContent))
	}

	// Step 1: unknown-key rejection. Parse into a generic map first so we
	// can surface offending keys with did-you-mean before the typed parse
	// silently drops them.
	if err := validateManifestKeys(yamlContent); err != nil {
		return nil, err
	}

	// Step 2: typed parse.
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

	// C-24: validate strictness enum + default.
	if err := validateStrictness(&m.Settings); err != nil {
		return nil, err
	}

	return &m, nil
}

// validateManifestKeys checks for unknown top-level and settings keys.
func validateManifestKeys(yamlContent string) error {
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		// Not our job to surface yaml-syntax errors here; the typed parse
		// will catch them with a better-shaped error.
		return nil
	}

	for key := range raw {
		if !contains(validTopLevelKeys, key) {
			return unknownKeyError(key, "", validTopLevelKeys)
		}
	}

	settingsRaw, ok := raw["settings"].(map[string]interface{})
	if !ok {
		return nil
	}
	for key := range settingsRaw {
		if !contains(validSettingsKeys, key) {
			return unknownKeyError(key, "settings", validSettingsKeys)
		}
	}
	return nil
}

// validateStrictness validates m.Settings.Strictness against the enum and
// applies the default "threshold" when unset.
func validateStrictness(s *Settings) error {
	if s.Strictness == "" {
		s.Strictness = "threshold"
		return nil
	}
	if !contains(validStrictnessValues, s.Strictness) {
		return fmt.Errorf("settings.strictness: %q is not a valid value (allowed: %s)",
			s.Strictness, strings.Join(validStrictnessValues, ", "))
	}
	return nil
}

// unknownKeyError builds a did-you-mean error for unknown manifest keys.
// scope is "" for top-level or e.g. "settings" for nested.
func unknownKeyError(offending, scope string, valid []string) error {
	prefix := offending
	if scope != "" {
		prefix = scope + "." + offending
	}
	suggestion := closestKey(offending, valid)
	sortedValid := append([]string{}, valid...)
	sort.Strings(sortedValid)
	scopeLabel := "manifest"
	if scope != "" {
		scopeLabel = scope
	}
	if suggestion != "" {
		return fmt.Errorf("unknown %s key %q — did you mean %q? (valid keys: %s)",
			scopeLabel, prefix, suggestion, strings.Join(sortedValid, ", "))
	}
	return fmt.Errorf("unknown %s key %q (valid keys: %s)",
		scopeLabel, prefix, strings.Join(sortedValid, ", "))
}

// closestKey returns the closest valid key to target by Levenshtein distance,
// or "" if no key is within distance 3.
func closestKey(target string, candidates []string) string {
	best := ""
	bestDist := 4
	for _, c := range candidates {
		d := levenshtein(target, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

// levenshtein computes edit distance between a and b.
func levenshtein(a, b string) int {
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	ra, rb := []rune(a), []rune(b)
	mLen, n := len(ra), len(rb)
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= mLen; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			a, b, c := curr[j-1]+1, prev[j]+1, prev[j-1]+cost
			minVal := a
			if b < minVal {
				minVal = b
			}
			if c < minVal {
				minVal = c
			}
			curr[j] = minVal
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// Defaults returns a Manifest with sensible defaults for use when no specter.yaml exists.
func Defaults() *Manifest {
	return &Manifest{
		System: SystemConfig{
			Name: "",
		},
		Settings: Settings{
			SpecsDir:   "specs",
			Strictness: "threshold",
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

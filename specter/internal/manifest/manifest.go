package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
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
// registry is still accepted so an existing manifest parses, and its value is
// discarded. docs/ssrb/SSRB-105.md retires the section: the code, C-06, AC-09
// and AC-10 go now, and the key leaves this list at v1.0.0 alongside
// settings.strictness, system.tier and settings.tier_overrides.
var validTopLevelKeys = []string{"schema_version", "system", "domains", "settings", "registry"}

// validSettingsKeys lists every key allowed under `settings:`. Updated when
// adding a new settings field.
var validSettingsKeys = []string{
	"specs_dir", "coverage", "exclude", "strict", "warn_on_draft",
	"tier_overrides", "tests_glob", "strictness", "annotation",
}

// validAnnotationKeys lists every key allowed under `settings.annotation`.
// C-32 gives the block exactly one sub-key. `scope` is not on this list and
// is not a schema field; C-33 rejects it earlier with its own message.
var validAnnotationKeys = []string{"permissive"}

// ErrAnnotationScopeStaged is the C-33 rejection of `settings.annotation.scope`.
// The wording follows SSRB-104 section 7.7, joined to one line and without the
// trailing period. Every value gets this same message, `test` and `all` alike,
// because the key does not ship in v0.15.0 at all and a value-specific message
// would imply one of them works.
var ErrAnnotationScopeStaged = errors.New(
	"settings.annotation.scope is accepted in SSRB-104 and not implemented in v0.15.0. " +
		"Annotation scope is test-only; remove the key")

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

	// C-27: schema_version absent → default to 1. Tool-layer code (doctor
	// --fix migration) is the right place to validate whether the declared
	// integer is supported; ParseManifest preserves the value verbatim.
	if m.SchemaVersion == 0 {
		m.SchemaVersion = 1
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

	// C-30 (v0.13 security audit H1): refuse specs_dir values that
	// could escape the workspace.
	if err := validateSpecsDir(m.Settings.SpecsDir); err != nil {
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
	return validateAnnotationKeys(settingsRaw)
}

// validateAnnotationKeys validates the sub-keys under `settings.annotation`.
//
// C-33 first: `scope` gets the SSRB-104 section 7.7 staging message rather
// than the generic unknown-key error, for every value. The presence test runs
// before the loop because Go map iteration order is random, and a manifest
// carrying both `scope` and another unknown sub-key would otherwise report
// either one.
//
// C-32 second: any other unrecognized sub-key gets the C-26 error shape.
func validateAnnotationKeys(settingsRaw map[string]interface{}) error {
	annotationRaw, ok := settingsRaw["annotation"].(map[string]interface{})
	if !ok {
		// Absent, or one of the two empty forms. Neither carries a sub-key.
		return nil
	}
	if _, declared := annotationRaw["scope"]; declared {
		return ErrAnnotationScopeStaged
	}
	for key := range annotationRaw {
		if !contains(validAnnotationKeys, key) {
			return unknownKeyError(key, "settings.annotation", validAnnotationKeys)
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

// validateSpecsDir refuses settings.specs_dir values that could escape
// the workspace. Per spec-manifest C-30 (v0.13 security audit H1):
//
//   - Empty string → allowed (default "specs" applied by SpecsDir())
//   - Absolute path (filepath.IsAbs, including Windows drive-letter
//     and forward-slash absolutes) → rejected
//   - Path containing a ".." segment (split on / or \) → rejected
//   - Lexically-cleaned path that starts with ".." or "/" → rejected
//
// Rationale: a malicious workspace's specter.yaml setting
// specs_dir: /home/victim or specs_dir: ../../../home/victim caused
// filepath.Walk and doctor --fix to operate outside the workspace.
// Catching this at parse time means every downstream consumer
// inherits the guarantee.
func validateSpecsDir(s string) error {
	if s == "" {
		return nil
	}
	// Absolute on the current GOOS. Also catch forward-slash leading
	// absolutes on Windows where filepath.IsAbs only returns true for
	// drive-letter forms (`C:\...`). And catch `\\server\share` UNC.
	if filepath.IsAbs(s) || strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\\`) {
		return fmt.Errorf("settings.specs_dir: %q must be a workspace-relative path, not absolute", s)
	}
	// Windows drive-letter form on non-Windows builds (e.g. "C:\foo"
	// would not be flagged as absolute on Linux). Reject explicitly so
	// the manifest is portable.
	if len(s) >= 2 && s[1] == ':' &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		return fmt.Errorf("settings.specs_dir: %q must be a workspace-relative path, not a Windows drive-letter path", s)
	}
	// Reject any segment equal to "..". Split on both separators
	// because a Windows path with forward slashes on Linux still wants
	// to be rejected.
	segments := strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '\\' })
	for _, seg := range segments {
		if seg == ".." {
			return fmt.Errorf("settings.specs_dir: %q must not contain `..` segments — paths must resolve inside the workspace", s)
		}
	}
	// Defense-in-depth: lexical clean must not produce a path that
	// escapes. filepath.Clean("foo/../../bar") returns "../bar", which
	// we catch via the ".." prefix below — but we already filtered raw
	// ".." segments above, so this catches the (rare) cases where
	// cleaning still ends up rooted. Use forward-slash-form for the
	// portability test.
	cleaned := filepath.ToSlash(filepath.Clean(s))
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("settings.specs_dir: %q resolves outside the workspace after cleaning", s)
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

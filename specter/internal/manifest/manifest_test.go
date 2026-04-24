// @spec spec-manifest
package manifest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/schema"
)

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return string(data)
}

// --- ParseManifest tests ---

// @ac AC-01
func TestParseManifest_FullManifest(t *testing.T) {
	t.Run("spec-manifest/AC-01 parse manifest full manifest", func(t *testing.T) {
		content := readFixture(t, "../../testdata/manifests/valid/full.specter.yaml")
		m, err := ParseManifest(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.System.Name != "jendays" {
			t.Errorf("system.name = %q, want %q", m.System.Name, "jendays")
		}
		if m.System.Tier != 1 {
			t.Errorf("system.tier = %d, want 1", m.System.Tier)
		}
		if len(m.Domains) != 3 {
			t.Errorf("domains count = %d, want 3", len(m.Domains))
		}
		if m.Settings.SpecsDir != "specs" {
			t.Errorf("specs_dir = %q, want %q", m.Settings.SpecsDir, "specs")
		}
		if len(m.Registry) != 2 {
			t.Errorf("registry count = %d, want 2", len(m.Registry))
		}
	})
}

func TestParseManifest_MinimalManifest(t *testing.T) {
	content := readFixture(t, "../../testdata/manifests/valid/minimal.specter.yaml")
	m, err := ParseManifest(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.System.Name != "my-app" {
		t.Errorf("system.name = %q, want %q", m.System.Name, "my-app")
	}
}

// @ac AC-02
func TestParseManifest_MissingName(t *testing.T) {
	t.Run("spec-manifest/AC-02 parse manifest missing name", func(t *testing.T) {
		content := readFixture(t, "../../testdata/manifests/invalid/missing-name.specter.yaml")
		_, err := ParseManifest(content)
		if err == nil {
			t.Fatal("expected error for missing system.name, got nil")
		}
	})
}

// @ac AC-03
func TestParseManifest_BadTier(t *testing.T) {
	t.Run("spec-manifest/AC-03 parse manifest bad tier", func(t *testing.T) {
		content := readFixture(t, "../../testdata/manifests/invalid/bad-tier.specter.yaml")
		_, err := ParseManifest(content)
		if err == nil {
			t.Fatal("expected error for invalid tier, got nil")
		}
	})
}

func TestParseManifest_MalformedYAML(t *testing.T) {
	_, err := ParseManifest("{{invalid yaml")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

// @ac AC-08
func TestDefaults(t *testing.T) {
	t.Run("spec-manifest/AC-08 defaults", func(t *testing.T) {
		m := Defaults()
		if m.SpecsDir() != "specs" {
			t.Errorf("default specs_dir = %q, want %q", m.SpecsDir(), "specs")
		}
		thresholds := m.CoverageThresholds()
		if thresholds[1] != 100 || thresholds[2] != 80 || thresholds[3] != 50 {
			t.Errorf("default thresholds = %v, want {1:100, 2:80, 3:50}", thresholds)
		}
	})
}

// --- Tier resolution tests ---

// @ac AC-04
func TestResolveTier_ExplicitSpecTier(t *testing.T) {
	t.Run("spec-manifest/AC-04 resolve tier explicit spec tier", func(t *testing.T) {
		m := &Manifest{
			System:  SystemConfig{Name: "test", Tier: 2},
			Domains: map[string]DomainConfig{"auth": {Tier: 1, Specs: []string{"login"}}},
		}
		tier := ResolveTier("login", 3, m)
		if tier != 3 {
			t.Errorf("ResolveTier with explicit spec tier = %d, want 3", tier)
		}
	})
}

// @ac AC-05
func TestResolveTier_InheritDomainTier(t *testing.T) {
	t.Run("spec-manifest/AC-05 resolve tier inherit domain tier", func(t *testing.T) {
		m := &Manifest{
			System:  SystemConfig{Name: "test", Tier: 2},
			Domains: map[string]DomainConfig{"auth": {Tier: 1, Specs: []string{"login"}}},
		}
		tier := ResolveTier("login", 0, m)
		if tier != 1 {
			t.Errorf("ResolveTier inheriting domain tier = %d, want 1", tier)
		}
	})
}

// @ac AC-06
func TestResolveTier_InheritSystemTier(t *testing.T) {
	t.Run("spec-manifest/AC-06 resolve tier inherit system tier", func(t *testing.T) {
		m := &Manifest{
			System: SystemConfig{Name: "test", Tier: 2},
		}
		tier := ResolveTier("orphan-spec", 0, m)
		if tier != 2 {
			t.Errorf("ResolveTier inheriting system tier = %d, want 2", tier)
		}
	})
}

// @ac AC-07
func TestResolveTier_DefaultTo2(t *testing.T) {
	t.Run("spec-manifest/AC-07 resolve tier default to 2", func(t *testing.T) {
		m := &Manifest{
			System: SystemConfig{Name: "test"},
		}
		tier := ResolveTier("orphan-spec", 0, m)
		if tier != 2 {
			t.Errorf("ResolveTier default = %d, want 2", tier)
		}
	})
}

func TestResolveTier_NilManifest(t *testing.T) {
	tier := ResolveTier("any-spec", 0, nil)
	if tier != 2 {
		t.Errorf("ResolveTier with nil manifest = %d, want 2", tier)
	}
}

// --- Domain tests ---

func TestSpecDomain_Found(t *testing.T) {
	m := &Manifest{
		Domains: map[string]DomainConfig{
			"payments": {Specs: []string{"checkout", "webhooks-stripe"}},
			"auth":     {Specs: []string{"login", "register"}},
		},
	}
	if d := SpecDomain("checkout", m); d != "payments" {
		t.Errorf("SpecDomain(checkout) = %q, want %q", d, "payments")
	}
	if d := SpecDomain("login", m); d != "auth" {
		t.Errorf("SpecDomain(login) = %q, want %q", d, "auth")
	}
}

func TestSpecDomain_NotFound(t *testing.T) {
	m := &Manifest{
		Domains: map[string]DomainConfig{
			"payments": {Specs: []string{"checkout"}},
		},
	}
	if d := SpecDomain("unknown", m); d != "" {
		t.Errorf("SpecDomain(unknown) = %q, want empty", d)
	}
}

// @ac AC-11
func TestDomainCoverage(t *testing.T) {
	t.Run("spec-manifest/AC-11 domain coverage", func(t *testing.T) {
		m := &Manifest{
			Domains: map[string]DomainConfig{
				"payments": {Tier: 1, Specs: []string{"checkout", "webhooks"}},
				"content":  {Tier: 2, Specs: []string{"blog"}},
			},
		}
		report := &coverage.CoverageReport{
			Entries: []coverage.SpecCoverageEntry{
				{SpecID: "checkout", CoveragePct: 100, PassesThreshold: true},
				{SpecID: "webhooks", CoveragePct: 80, PassesThreshold: false},
				{SpecID: "blog", CoveragePct: 90, PassesThreshold: true},
				{SpecID: "orphan", CoveragePct: 50, PassesThreshold: true},
			},
		}

		results := DomainCoverage(report, m)
		if len(results) < 2 {
			t.Fatalf("expected at least 2 domain entries, got %d", len(results))
		}

		// Find payments domain
		var payments *DomainCoverageEntry
		for i := range results {
			if results[i].Domain == "payments" {
				payments = &results[i]
			}
		}
		if payments == nil {
			t.Fatal("payments domain not found in results")
		}
		if payments.TotalSpecs != 2 {
			t.Errorf("payments total specs = %d, want 2", payments.TotalSpecs)
		}
		if payments.Passing != 1 || payments.Failing != 1 {
			t.Errorf("payments passing=%d failing=%d, want 1/1", payments.Passing, payments.Failing)
		}
	})
}

// --- Registry tests ---

// @ac AC-09
func TestBuildRegistryFromSpecs(t *testing.T) {
	t.Run("spec-manifest/AC-09 build registry from specs", func(t *testing.T) {
		specs := []schema.SpecAST{
			{ID: "checkout", Version: "1.0.0", Status: "approved", Tier: 1},
			{ID: "login", Version: "0.1.0", Status: "draft", Tier: 0},
			{ID: "blog", Version: "1.0.0", Status: "approved", Tier: 2},
		}
		files := map[string]string{
			"checkout": "specs/checkout.spec.yaml",
			"login":    "specs/login.spec.yaml",
			"blog":     "specs/blog.spec.yaml",
		}
		m := &Manifest{
			System: SystemConfig{Name: "test"},
			Domains: map[string]DomainConfig{
				"payments": {Tier: 1, Specs: []string{"checkout"}},
				"auth":     {Tier: 1, Specs: []string{"login"}},
			},
		}

		entries := BuildRegistryFromSpecs(specs, files, m)
		if len(entries) != 3 {
			t.Fatalf("registry entries = %d, want 3", len(entries))
		}

		// Entries are sorted by ID
		if entries[0].ID != "blog" {
			t.Errorf("first entry ID = %q, want %q", entries[0].ID, "blog")
		}
	})
}

// @ac AC-10
func TestBuildRegistryFromSpecs_DomainAssignment(t *testing.T) {
	t.Run("spec-manifest/AC-10 build registry from specs domain assignment", func(t *testing.T) {
		specs := []schema.SpecAST{
			{ID: "checkout", Version: "1.0.0", Status: "approved", Tier: 1},
		}
		files := map[string]string{"checkout": "specs/checkout.spec.yaml"}
		m := &Manifest{
			System:  SystemConfig{Name: "test"},
			Domains: map[string]DomainConfig{"payments": {Specs: []string{"checkout"}}},
		}

		entries := BuildRegistryFromSpecs(specs, files, m)
		if entries[0].Domain != "payments" {
			t.Errorf("registry domain = %q, want %q", entries[0].Domain, "payments")
		}
	})
}

// --- Scaffold tests ---

// @ac AC-12
func TestScaffoldManifest_RoundTrip(t *testing.T) {
	t.Run("spec-manifest/AC-12 scaffold manifest round trip", func(t *testing.T) {
		yamlStr := ScaffoldManifest("my-app", "A test application", []string{"auth", "payments"})
		if yamlStr == "" {
			t.Fatal("ScaffoldManifest returned empty string")
		}

		// Should parse back successfully
		m, err := ParseManifest(yamlStr)
		if err != nil {
			t.Fatalf("scaffold output failed parse: %v", err)
		}
		if m.System.Name != "my-app" {
			t.Errorf("scaffold system.name = %q, want %q", m.System.Name, "my-app")
		}
	})
}

// --- CoverageThresholds tests ---

func TestCoverageThresholds_CustomValues(t *testing.T) {
	m := &Manifest{
		Settings: Settings{
			Coverage: CoverageConfig{Tier1: 95, Tier2: 70, Tier3: 40},
		},
	}
	thresholds := m.CoverageThresholds()
	if thresholds[1] != 95 || thresholds[2] != 70 || thresholds[3] != 40 {
		t.Errorf("custom thresholds = %v, want {1:95, 2:70, 3:40}", thresholds)
	}
}

func TestCoverageThresholds_PartialOverride(t *testing.T) {
	m := &Manifest{
		Settings: Settings{
			Coverage: CoverageConfig{Tier2: 90},
		},
	}
	thresholds := m.CoverageThresholds()
	if thresholds[1] != 100 || thresholds[2] != 90 || thresholds[3] != 50 {
		t.Errorf("partial thresholds = %v, want {1:100, 2:90, 3:50}", thresholds)
	}
}

// @ac AC-15
func TestParseManifest_SettingsStrict(t *testing.T) {
	t.Run("spec-manifest/AC-15 parse manifest settings strict", func(t *testing.T) {
		content := `
system:
  name: test-app
settings:
  strict: true
`
		m, err := ParseManifest(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !m.Settings.Strict {
			t.Error("expected Settings.Strict=true, got false")
		}
	})
}

// @ac AC-16
func TestParseManifest_SettingsWarnOnDraft(t *testing.T) {
	t.Run("spec-manifest/AC-16 parse manifest settings warn on draft", func(t *testing.T) {
		content := `
system:
  name: test-app
settings:
  warn_on_draft: true
`
		m, err := ParseManifest(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !m.Settings.WarnOnDraft {
			t.Error("expected Settings.WarnOnDraft=true, got false")
		}
	})
}

func TestParseManifest_SettingsDefaultsFalse(t *testing.T) {
	content := `
system:
  name: test-app
`
	m, err := ParseManifest(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Settings.Strict {
		t.Error("expected Settings.Strict=false by default")
	}
	if m.Settings.WarnOnDraft {
		t.Error("expected Settings.WarnOnDraft=false by default")
	}
}

// @ac AC-17
func TestSpecTemplate_APIEndpoint(t *testing.T) {
	t.Run("spec-manifest/AC-17 spec template api endpoint", func(t *testing.T) {
		tmpl, err := SpecTemplate("api-endpoint")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		result := parser.ParseSpec(tmpl)
		if !result.OK {
			t.Fatalf("api-endpoint template failed ParseSpec: %v", result.Errors)
		}
		if result.Value.Status != "draft" {
			t.Errorf("expected status=draft, got %q", result.Value.Status)
		}
	})
}

func TestSpecTemplate_Service(t *testing.T) {
	tmpl, err := SpecTemplate("service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := parser.ParseSpec(tmpl)
	if !result.OK {
		t.Fatalf("service template failed ParseSpec: %v", result.Errors)
	}
	if result.Value.Status != "draft" {
		t.Errorf("expected status=draft, got %q", result.Value.Status)
	}
}

func TestSpecTemplate_Auth(t *testing.T) {
	tmpl, err := SpecTemplate("auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := parser.ParseSpec(tmpl)
	if !result.OK {
		t.Fatalf("auth template failed ParseSpec: %v", result.Errors)
	}
	if result.Value.Status != "draft" {
		t.Errorf("expected status=draft, got %q", result.Value.Status)
	}
	if result.Value.Tier != 1 {
		t.Errorf("expected auth template tier=1 (critical), got %d", result.Value.Tier)
	}
}

func TestSpecTemplate_DataModel(t *testing.T) {
	tmpl, err := SpecTemplate("data-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := parser.ParseSpec(tmpl)
	if !result.OK {
		t.Fatalf("data-model template failed ParseSpec: %v", result.Errors)
	}
	if result.Value.Status != "draft" {
		t.Errorf("expected status=draft, got %q", result.Value.Status)
	}
}

// @ac AC-18
func TestSpecTemplate_UnknownType(t *testing.T) {
	t.Run("spec-manifest/AC-18 spec template unknown type", func(t *testing.T) {
		_, err := SpecTemplate("nonexistent")
		if err == nil {
			t.Error("expected error for unknown template type, got nil")
		}
	})
}

// @spec spec-manifest
// @ac AC-19
func TestTierOverrides_Parsing(t *testing.T) {
	t.Run("spec-manifest/AC-19 tier overrides parsing", func(t *testing.T) {
		yaml := `
system:
  name: test
settings:
  tier_overrides:
    payment-intent: 1
    auth-login: 3
`
		m, err := ParseManifest(yaml)
		if err != nil {
			t.Fatalf("ParseManifest error: %v", err)
		}
		if m.Settings.TierOverrides["payment-intent"] != 1 {
			t.Errorf("expected tier_overrides[payment-intent]=1, got %d", m.Settings.TierOverrides["payment-intent"])
		}
		if m.Settings.TierOverrides["auth-login"] != 3 {
			t.Errorf("expected tier_overrides[auth-login]=3, got %d", m.Settings.TierOverrides["auth-login"])
		}
	})
}

// @ac AC-20
func TestCheckTierConflicts_EmitsWarning(t *testing.T) {
	t.Run("spec-manifest/AC-20 check tier conflicts emits warning", func(t *testing.T) {
		m, _ := ParseManifest(`system:
  name: test
settings:
  tier_overrides:
    payment-intent: 1
`)
		specs := []schema.SpecAST{
			{ID: "payment-intent", Tier: 2},
			{ID: "auth-login", Tier: 1}, // not in overrides — no warning
		}
		warnings := CheckTierConflicts(specs, m)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 tier_conflict warning, got %d", len(warnings))
		}
		if warnings[0].SpecID != "payment-intent" {
			t.Errorf("expected warning for payment-intent, got %q", warnings[0].SpecID)
		}
		if warnings[0].SpecTier != 2 || warnings[0].OverrideTier != 1 {
			t.Errorf("expected spec tier 2, override 1; got %d, %d", warnings[0].SpecTier, warnings[0].OverrideTier)
		}
	})
}

// @ac AC-20
func TestCheckTierConflicts_NoConflictWhenSpecTierZero(t *testing.T) {
	t.Run("spec-manifest/AC-20 check tier conflicts no conflict when spec tier zero", func(t *testing.T) {
		m, _ := ParseManifest(`system:
  name: test
settings:
  tier_overrides:
    my-spec: 2
`)
		specs := []schema.SpecAST{{ID: "my-spec", Tier: 0}}
		warnings := CheckTierConflicts(specs, m)
		if len(warnings) != 0 {
			t.Errorf("expected no conflict when spec has no declared tier, got %d warnings", len(warnings))
		}
	})
}

// @spec spec-manifest
// @ac AC-21
func TestScaffoldManifest_CanonicalGitHubURL(t *testing.T) {
	t.Run("spec-manifest/AC-21 scaffold manifest canonical github url", func(t *testing.T) {
		out := ScaffoldManifest("my-app", "", nil)
		if !strings.Contains(out, "https://github.com/Hanalyx/specter") {
			t.Errorf("scaffold must contain canonical repo URL 'https://github.com/Hanalyx/specter', got:\n%s", out)
		}
		if strings.Contains(out, "spec-dd") {
			t.Errorf("scaffold must not reference 'spec-dd' (stale slug), got:\n%s", out)
		}
	})
}

// @spec spec-manifest
// @ac AC-22
// Greenfield case: zero specs, zero candidates → placeholder default domain
// with an "Add spec IDs here" description, so the operator sees where to
// extend the manifest.
func TestScaffoldManifest_Greenfield_EmitsDefaultDomainPlaceholder(t *testing.T) {
	t.Run("spec-manifest/AC-22 scaffold manifest greenfield emits default domain placeholder", func(t *testing.T) {
		out := ScaffoldManifestWithContext("my-app", "", nil, 0)
		if !strings.Contains(out, "domains:") {
			t.Fatalf("greenfield scaffold must emit `domains:` section, got:\n%s", out)
		}
		if !strings.Contains(out, "default:") {
			t.Fatalf("greenfield scaffold must emit `default:` domain, got:\n%s", out)
		}
		if !strings.Contains(out, "Add spec IDs") {
			t.Errorf("greenfield description must explain placeholder, got:\n%s", out)
		}
		if strings.Contains(out, "could not be parsed") {
			t.Errorf("greenfield must not claim parse failure, got:\n%s", out)
		}
		// Round-trip: the placeholder must still produce valid YAML.
		if _, err := ParseManifest(out); err != nil {
			t.Errorf("greenfield scaffold failed to round-trip: %v", err)
		}
	})
}

// @spec spec-manifest
// @ac AC-22
// Drift case: zero specs parsed but N candidates on disk → domain
// description names the parse-failure mismatch and points at doctor.
func TestScaffoldManifest_Drift_DescribesParseFailure(t *testing.T) {
	t.Run("spec-manifest/AC-22 scaffold manifest drift describes parse failure", func(t *testing.T) {
		out := ScaffoldManifestWithContext("my-app", "", nil, 3)
		if !strings.Contains(out, "could not be parsed") {
			t.Errorf("drift-case scaffold must name the parse-failure mismatch, got:\n%s", out)
		}
		if !strings.Contains(out, "specter doctor") {
			t.Errorf("drift-case scaffold must point at `specter doctor`, got:\n%s", out)
		}
		if _, err := ParseManifest(out); err != nil {
			t.Errorf("drift scaffold failed to round-trip: %v", err)
		}
	})
}

// @ac AC-13 — when specter.yaml is absent, Defaults() returns a usable
// Manifest and all consumers (specs_dir resolution, thresholds, excludes)
// produce identical behavior to what a minimal explicit manifest would.
// This is the "backward compatibility" contract: running specter in a
// directory without specter.yaml must not fail.
func TestDefaults_MatchesImplicitManifestBehavior(t *testing.T) {
	t.Run("spec-manifest/AC-13 defaults matches implicit manifest behavior", func(t *testing.T) {
		implicit := Defaults()

		// An explicit manifest with the documented defaults should produce the
		// same behavior as Defaults().
		yamlBody := `
system:
  name: anything
settings:
  specs_dir: specs
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
`
		explicit, err := ParseManifest(yamlBody)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}

		if implicit.SpecsDir() != explicit.SpecsDir() {
			t.Errorf("specs_dir mismatch: implicit %q vs explicit %q",
				implicit.SpecsDir(), explicit.SpecsDir())
		}
		ithr := implicit.CoverageThresholds()
		ethr := explicit.CoverageThresholds()
		for tier := 1; tier <= 3; tier++ {
			if ithr[tier] != ethr[tier] {
				t.Errorf("tier %d threshold mismatch: implicit %d vs explicit %d",
					tier, ithr[tier], ethr[tier])
			}
		}
	})
}

// @ac AC-14 — the manifest package must be a pure, injectable unit with
// no dependencies on I/O, the CLI layer, or anything that performs
// side effects. The package is consumed by cmd/specter (which does do
// I/O) but must itself be callable from tests, the reverse compiler,
// and any future migration tooling without a working filesystem.
func TestManifestPackage_HasNoForbiddenImports(t *testing.T) {
	t.Run("spec-manifest/AC-14 manifest package has no forbidden imports", func(t *testing.T) {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("cannot determine test file path")
		}
		pkgDir := filepath.Dir(file)

		forbidden := []string{
			`"os/exec"`,                        // no subprocess spawning
			`"net/http"`,                       // no network
			`"github.com/spf13/cobra"`,         // no CLI framework
			`"github.com/Hanalyx/specter/cmd/`, // no reverse dep on CLI layer
		}

		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			t.Fatalf("read pkg dir: %v", err)
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			// Skip test files — they may legitimately import test helpers.
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(pkgDir, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			content := string(data)
			for _, imp := range forbidden {
				if strings.Contains(content, imp) {
					t.Errorf("%s imports forbidden package %s (manifest package must be I/O-free)", e.Name(), imp)
				}
			}
		}
	})
}

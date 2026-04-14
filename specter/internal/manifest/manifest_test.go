// @spec spec-manifest
package manifest

import (
	"os"
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
	content := readFixture(t, "../../testdata/manifests/invalid/missing-name.specter.yaml")
	_, err := ParseManifest(content)
	if err == nil {
		t.Fatal("expected error for missing system.name, got nil")
	}
}

// @ac AC-03
func TestParseManifest_BadTier(t *testing.T) {
	content := readFixture(t, "../../testdata/manifests/invalid/bad-tier.specter.yaml")
	_, err := ParseManifest(content)
	if err == nil {
		t.Fatal("expected error for invalid tier, got nil")
	}
}

func TestParseManifest_MalformedYAML(t *testing.T) {
	_, err := ParseManifest("{{invalid yaml")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

// @ac AC-08
func TestDefaults(t *testing.T) {
	m := Defaults()
	if m.SpecsDir() != "specs" {
		t.Errorf("default specs_dir = %q, want %q", m.SpecsDir(), "specs")
	}
	thresholds := m.CoverageThresholds()
	if thresholds[1] != 100 || thresholds[2] != 80 || thresholds[3] != 50 {
		t.Errorf("default thresholds = %v, want {1:100, 2:80, 3:50}", thresholds)
	}
}

// --- Tier resolution tests ---

// @ac AC-04
func TestResolveTier_ExplicitSpecTier(t *testing.T) {
	m := &Manifest{
		System:  SystemConfig{Name: "test", Tier: 2},
		Domains: map[string]DomainConfig{"auth": {Tier: 1, Specs: []string{"login"}}},
	}
	tier := ResolveTier("login", 3, m)
	if tier != 3 {
		t.Errorf("ResolveTier with explicit spec tier = %d, want 3", tier)
	}
}

// @ac AC-05
func TestResolveTier_InheritDomainTier(t *testing.T) {
	m := &Manifest{
		System:  SystemConfig{Name: "test", Tier: 2},
		Domains: map[string]DomainConfig{"auth": {Tier: 1, Specs: []string{"login"}}},
	}
	tier := ResolveTier("login", 0, m)
	if tier != 1 {
		t.Errorf("ResolveTier inheriting domain tier = %d, want 1", tier)
	}
}

// @ac AC-06
func TestResolveTier_InheritSystemTier(t *testing.T) {
	m := &Manifest{
		System: SystemConfig{Name: "test", Tier: 2},
	}
	tier := ResolveTier("orphan-spec", 0, m)
	if tier != 2 {
		t.Errorf("ResolveTier inheriting system tier = %d, want 2", tier)
	}
}

// @ac AC-07
func TestResolveTier_DefaultTo2(t *testing.T) {
	m := &Manifest{
		System: SystemConfig{Name: "test"},
	}
	tier := ResolveTier("orphan-spec", 0, m)
	if tier != 2 {
		t.Errorf("ResolveTier default = %d, want 2", tier)
	}
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
}

// --- Registry tests ---

// @ac AC-09
func TestBuildRegistryFromSpecs(t *testing.T) {
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
}

// @ac AC-10
func TestBuildRegistryFromSpecs_DomainAssignment(t *testing.T) {
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
}

// --- Scaffold tests ---

// @ac AC-12
func TestScaffoldManifest_RoundTrip(t *testing.T) {
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
}

// @ac AC-16
func TestParseManifest_SettingsWarnOnDraft(t *testing.T) {
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
	_, err := SpecTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for unknown template type, got nil")
	}
}

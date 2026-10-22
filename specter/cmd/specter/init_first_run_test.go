// init_first_run_test.go: the first-run path a new adopter takes.
// `init`, `init --template`, `parse` and `check` on a fresh directory.
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// @ac AC-74
func TestInitFirstRun_NoTierKeysAndCleanCheck(t *testing.T) {
	t.Run("spec-manifest/AC-74 init writes no tier keys and check prints no deprecation", func(t *testing.T) {
		dir := t.TempDir()
		if out, code := runCLI(t, dir, "init"); code != 0 {
			t.Fatalf("init exited %d:\n%s", code, out)
		}

		var doc struct {
			System  map[string]interface{}            `yaml:"system"`
			Domains map[string]map[string]interface{} `yaml:"domains"`
		}
		if err := yaml.Unmarshal([]byte(readManifest(t, dir)), &doc); err != nil {
			t.Fatalf("parse scaffolded manifest: %v", err)
		}
		if doc.System == nil {
			t.Fatal("scaffolded manifest has no system block; the tier assertion below would be vacuous")
		}
		if _, ok := doc.System["tier"]; ok {
			t.Error("scaffolded manifest declares system.tier, which C-36 deprecates")
		}
		if len(doc.Domains) == 0 {
			t.Fatal("scaffolded manifest has no domains; the domain tier assertion below would be vacuous")
		}
		for name, d := range doc.Domains {
			if _, ok := d["tier"]; ok {
				t.Errorf("scaffolded manifest declares domains.%s.tier", name)
			}
		}

		out, code := runCLI(t, dir, "check")
		if code != 0 {
			t.Errorf("check exited %d:\n%s", code, out)
		}
		if strings.Contains(out, "deprecated") {
			t.Errorf("check on a fresh workspace printed a deprecation warning:\n%s", out)
		}
		if strings.Contains(out, "domain_tier_conflict") {
			t.Errorf("check on a fresh workspace printed a domain_tier_conflict:\n%s", out)
		}
	})
}

// @ac AC-75
func TestInitFirstRun_TemplateLandsInSpecsDir(t *testing.T) {
	t.Run("spec-manifest/AC-75 init creates specs and the template lands there", func(t *testing.T) {
		dir := t.TempDir()
		if out, code := runCLI(t, dir, "init"); code != 0 {
			t.Fatalf("init exited %d:\n%s", code, out)
		}
		if info, err := os.Stat(filepath.Join(dir, "specs")); err != nil || !info.IsDir() {
			t.Errorf("init did not create specs/: %v", err)
		}

		if out, code := runCLI(t, dir, "init", "--template", "api-endpoint"); code != 0 {
			t.Fatalf("init --template exited %d:\n%s", code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "specs", "api-endpoint.spec.yaml")); err != nil {
			t.Errorf("template not written to specs/: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "api-endpoint.spec.yaml")); err == nil {
			t.Error("template was written to the current directory")
		}

		out, code := runCLI(t, dir, "parse")
		if code != 0 {
			t.Errorf("parse exited %d:\n%s", code, out)
		}
		if !strings.Contains(out, "api-endpoint.spec.yaml") {
			t.Errorf("parse did not name the template file:\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-75 template honors a custom specs_dir and creates it", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestRaw(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  specs_dir: custom\n")
		if out, code := runCLI(t, dir, "init", "--template", "service"); code != 0 {
			t.Fatalf("init --template exited %d:\n%s", code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "custom", "service.spec.yaml")); err != nil {
			t.Errorf("template not written to custom/: %v", err)
		}
	})

	t.Run("spec-manifest/AC-75 template lands in specs with no manifest", func(t *testing.T) {
		dir := t.TempDir()
		if out, code := runCLI(t, dir, "init", "--template", "auth"); code != 0 {
			t.Fatalf("init --template exited %d:\n%s", code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "specs", "auth.spec.yaml")); err != nil {
			t.Errorf("template not written to specs/ without a manifest: %v", err)
		}
	})

	t.Run("spec-manifest/AC-75 from a subdirectory the template lands in the manifest's specs dir", func(t *testing.T) {
		dir := t.TempDir()
		if out, code := runCLI(t, dir, "init"); code != 0 {
			t.Fatalf("init exited %d:\n%s", code, out)
		}
		sub := filepath.Join(dir, "src", "pkg")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		if out, code := runCLI(t, sub, "init", "--template", "service"); code != 0 {
			t.Fatalf("init --template from %s exited %d:\n%s", sub, code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "specs", "service.spec.yaml")); err != nil {
			t.Errorf("template not written to the manifest's specs/: %v", err)
		}
		for _, wrong := range []string{
			filepath.Join(sub, "specs", "service.spec.yaml"),
			filepath.Join(sub, "service.spec.yaml"),
		} {
			if _, err := os.Stat(wrong); err == nil {
				t.Errorf("template written relative to the subdirectory: %s", wrong)
			}
		}
	})

	t.Run("spec-manifest/AC-75 a rejected manifest stops the template and writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestRaw(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  specs_dir: specs\n  test_glob: 'x'\n")
		out, code := runCLI(t, dir, "init", "--template", "data-model")
		if code == 0 {
			t.Errorf("init --template exited 0 under a rejected manifest:\n%s", out)
		}
		for _, p := range []string{"data-model.spec.yaml", filepath.Join("specs", "data-model.spec.yaml")} {
			if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
				t.Errorf("init --template wrote %s under a rejected manifest", p)
			}
		}
	})
}

// @ac AC-76
func TestInitFirstRun_EveryTemplatePassesCheck(t *testing.T) {
	t.Run("spec-manifest/AC-76 every template passes check and check --concrete on a fresh workspace", func(t *testing.T) {
		for _, name := range []string{"api-endpoint", "service", "auth", "data-model"} {
			dir := t.TempDir()
			if out, code := runCLI(t, dir, "init"); code != 0 {
				t.Fatalf("%s: init exited %d:\n%s", name, code, out)
			}
			tmplOut, code := runCLI(t, dir, "init", "--template", name)
			if code != 0 {
				t.Fatalf("%s: init --template exited %d:\n%s", name, code, tmplOut)
			}
			if !strings.Contains(tmplOut, "specter check --concrete") || strings.Contains(tmplOut, "specter sync") {
				t.Errorf("%s: init --template must name specter check --concrete as the next step, not specter sync:\n%s", name, tmplOut)
			}
			// Precondition: check must see the spec, or exit 0 proves nothing.
			if _, err := os.Stat(filepath.Join(dir, "specs", name+".spec.yaml")); err != nil {
				t.Fatalf("%s: template not under specs/, so check would see no spec: %v", name, err)
			}
			if out, code := runCLI(t, dir, "parse"); code != 0 || !strings.Contains(out, name+".spec.yaml") {
				t.Fatalf("%s: parse did not find the template (exit %d), so check would see no spec:\n%s", name, code, out)
			}
			for _, args := range [][]string{{"check"}, {"check", "--concrete"}} {
				label := name + ": " + strings.Join(args, " ")
				out, code := runCLI(t, dir, args...)
				if code != 0 {
					t.Errorf("%s exited %d:\n%s", label, code, out)
				}
				for _, bad := range []string{"orphan_constraint", "vague_criterion", "warn"} {
					if strings.Contains(out, bad) {
						t.Errorf("%s printed %q:\n%s", label, bad, out)
					}
				}
			}
		}
	})
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// parityWorkspace builds one workspace carrying both failure conditions: an
// annotated criterion whose result failed, and a criterion with no test at all.
// One workspace exercises both so the two commands see byte-identical input.
func parityWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	spec := "spec:\n  id: z-spec\n  version: \"1.0.0\"\n  status: draft\n  tier: 3\n" +
		"  context:\n    system: test\n" +
		"  objective:\n    summary: One annotated failing criterion and one with no test.\n" +
		"  constraints:\n    - id: C-01\n      description: \"MUST hold\"\n" +
		"  acceptance_criteria:\n" +
		"    - id: AC-01\n      description: \"annotated, failing\"\n      references_constraints: [\"C-01\"]\n" +
		"    - id: AC-02\n      description: \"no test at all\"\n      references_constraints: [\"C-01\"]\n"
	writeSpec(t, dir, "z-spec.spec.yaml", spec)

	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	body := "package tests\n\nimport \"testing\"\n\n// @spec z-spec\n// @ac AC-01\n" +
		"func TestZ(t *testing.T) {\n\tt.Run(\"z-spec/AC-01\", func(t *testing.T) {})\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "tests", "z_test.go"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	writeResultsJSON(t, dir, `{"results":[{"spec_id":"z-spec","ac_id":"AC-01","status":"failed","test_name":"TestZ"}]}`)
	return dir
}

func parityManifest(settings string) string {
	return "system:\n  name: z\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n" + settings
}

// causeLine returns the first stderr line beginning with error: or warn:, which
// is where every coverage-contract gate names its cause.
func causeLine(stderr string) string {
	for _, l := range strings.Split(stderr, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "error:") || strings.HasPrefix(t, "warn:") {
			return t
		}
	}
	return ""
}

// @spec spec-sync
// @ac AC-15
//
// C-11: a coverage-contract exit code fires identically on sync and coverage,
// cause line included. Comparing integers alone is not enough, because two
// different conditions both map to code 2.
func TestExitCodeParity_SyncMatchesCoverage(t *testing.T) {
	t.Run("spec-sync/AC-15 sync exit codes match coverage", func(t *testing.T) {
		cases := []struct {
			name     string
			settings string
		}{
			{"zero-tolerance, an annotated criterion did not pass", "  strictness: zero-tolerance\n"},
			{"annotation block, a criterion has no test", "  annotation:\n    permissive: false\n"},
		}
		for _, c := range cases {
			covDir := parityWorkspace(t)
			putManifest(t, covDir, parityManifest(c.settings))
			_, covErr, covCode := runCLISplit(t, covDir, "coverage")

			syncDir := parityWorkspace(t)
			putManifest(t, syncDir, parityManifest(c.settings))
			_, syncErr, syncCode := runCLISplit(t, syncDir, "sync")

			if covCode != syncCode {
				t.Errorf("%s: coverage exited %d and sync exited %d on the same workspace. C-11 requires them to agree, and sync is the CI entry point",
					c.name, covCode, syncCode)
			}
			cc, sc := causeLine(covErr), causeLine(syncErr)
			if cc != "" && !strings.Contains(sc, strings.TrimPrefix(strings.TrimPrefix(cc, "error:"), "warn:")) {
				t.Errorf("%s: the two commands name different causes.\n  coverage: %q\n  sync:     %q", c.name, cc, sc)
			}
		}
	})
}

// @spec spec-sync
// @ac AC-16
//
// C-12: every exit code the binary can emit is registered in
// docs/EXIT_CODES.md.
//
// Scan scope, stated because this is a static scan: cmd/specter/*.go excluding
// _test.go, literal os.Exit(N) calls. A code emitted from internal/ would not be
// found. That is a known limit; if one is ever added there this criterion must
// widen with it.
func TestExitCodeParity_EveryEmittedCodeIsRegistered(t *testing.T) {
	t.Run("spec-sync/AC-16 every emitted exit code is registered", func(t *testing.T) {
		entries, err := os.ReadDir(".")
		if err != nil {
			t.Fatal(err)
		}
		emitted := map[string]bool{}
		exitRe := regexp.MustCompile(`os\.Exit\((\d+)\)`)
		scanned := 0
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			scanned++
			for _, m := range exitRe.FindAllStringSubmatch(string(src), -1) {
				emitted[m[1]] = true
			}
		}
		if scanned == 0 {
			t.Fatal("scanned no source files; the scan scope is wrong and a pass here would be meaningless")
		}
		if len(emitted) == 0 {
			t.Fatal("found no os.Exit calls; the regex is wrong and a pass here would be meaningless")
		}

		doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "EXIT_CODES.md"))
		if err != nil {
			t.Fatalf("cannot read the registry: %v", err)
		}
		registry := string(doc)
		for code := range emitted {
			if !strings.Contains(registry, "| `"+code+"` |") {
				t.Errorf("C-12: the binary can exit %s and docs/EXIT_CODES.md has no row for it", code)
			}
		}
	})
}

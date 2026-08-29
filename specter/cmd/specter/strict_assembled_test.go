package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Strictness reaches every diagnostic the command assembles, spec-check C-07
// and AC-45, spec-manifest C-35 and AC-67.
//
// The defect: tier_conflict and domain_tier_conflict are computed from the
// manifest and appended after checker.CheckSpecs returns, so both sit past the
// upgrade loop inside it. Under --strict they keep severity warning and the
// run exits 0, on a workspace two specs say should fail.
//
// Written on those two kinds deliberately. The orphan diagnostics AC-07, AC-31
// and AC-36 already cover are produced inside the checker, so an implementation
// that promotes there alone passes all three and fails this file.

// strictWorkspace builds a workspace whose one spec disagrees with both
// settings.tier_overrides and its domain's declared tier, so a single run
// carries a tier_conflict and a domain_tier_conflict and no other diagnostic.
//
// No orphan constraint and no structural conflict: the assertion is about the
// two kinds that escaped, and another diagnostic in the run could carry the
// exit code on its own and hide the defect.
func strictWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("specter.yaml", `schema_version: 1
system:
  name: st
domains:
  core:
    tier: 1
    specs:
      - dm
settings:
  specs_dir: specs
  tier_overrides:
    dm: 1
`)
	write("specs/dm.spec.yaml", `spec:
  id: dm
  version: "1.0.0"
  status: approved
  tier: 3
  context:
    system: st
    feature: strictness
    description: A spec whose declared tier disagrees with both the override and the domain.
  objective:
    summary: Carry one tier conflict and one domain tier conflict and nothing else.
  constraints:
    - id: C-01
      description: "MUST hold"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "It holds"
      references_constraints: ["C-01"]
      priority: critical
`)
	return dir
}

// severitiesByKind reads the check JSON document and returns each diagnostic's
// severity keyed by kind, plus the summary counts.
func severitiesByKind(t *testing.T, stdout string) (map[string]string, int, int) {
	t.Helper()
	var doc struct {
		Diagnostics []struct {
			Kind     string `json:"kind"`
			Severity string `json:"severity"`
		} `json:"diagnostics"`
		Summary struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("check --json did not produce a document: %v\nstdout:\n%s", err, stdout)
	}
	out := map[string]string{}
	for _, d := range doc.Diagnostics {
		out[d.Kind] = d.Severity
	}
	return out, doc.Summary.Errors, doc.Summary.Warnings
}

// @spec spec-check
// @ac AC-45
//
// C-07: the strict upgrade applies to every diagnostic the run reports,
// whatever stage assembled it.
func TestStrictReachesAssembledDiagnostics(t *testing.T) {
	t.Run("spec-check/AC-45 plain check reports both kinds as warnings and exits 0", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check")

		// The control. Both kinds must be present, or the strict assertions
		// below would pass on a run that produced neither.
		for _, kind := range []string{"tier_conflict", "domain_tier_conflict"} {
			if !strings.Contains(stdout, kind) {
				t.Fatalf("AC-45 (plain): %s is absent, so the fixture does not exercise the rule.\nstdout:\n%s", kind, stdout)
			}
		}
		if !strings.Contains(stdout, "warn [tier_conflict]") || !strings.Contains(stdout, "warn [domain_tier_conflict]") {
			t.Errorf("AC-45 (plain): both kinds must report at warning without --strict.\nstdout:\n%s", stdout)
		}
		if code != 0 {
			t.Errorf("AC-45 (plain): exited %d, want 0. A warning does not fail a run.", code)
		}
	})

	t.Run("spec-check/AC-45 strict promotes both kinds and exits 1", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check", "--strict")

		if !strings.Contains(stdout, "error [tier_conflict]") {
			t.Errorf("AC-45 (strict): tier_conflict is still a warning under --strict. C-07 exempts structural_conflict and nothing else, and this kind is appended after the checker's upgrade loop.\nstdout:\n%s", stdout)
		}
		if !strings.Contains(stdout, "error [domain_tier_conflict]") {
			t.Errorf("AC-45 (strict): domain_tier_conflict is still a warning under --strict. spec-manifest C-35 states that --strict promotes it.\nstdout:\n%s", stdout)
		}
		if code != 1 {
			t.Errorf("AC-45 (strict): exited %d, want 1. Two promoted diagnostics are errors, and the verdict follows severity.", code)
		}
	})

	t.Run("spec-check/AC-45 strict JSON agrees with strict text", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check", "--strict", "--json")

		sev, errors, warnings := severitiesByKind(t, stdout)
		for _, kind := range []string{"tier_conflict", "domain_tier_conflict"} {
			got, ok := sev[kind]
			if !ok {
				t.Fatalf("AC-45 (strict json): %s is absent from the document. AC-64 requires it to reach --json at all.", kind)
			}
			if got != "error" {
				t.Errorf("AC-45 (strict json): %s carries severity %q, want \"error\". The two modes must not disagree about a severity the same flag decides.", kind, got)
			}
		}
		if warnings != 0 {
			t.Errorf("AC-45 (strict json): summary reports %d warning(s), want 0. Every diagnostic was promoted, so the counts must follow.", warnings)
		}
		if errors != 2 {
			t.Errorf("AC-45 (strict json): summary reports %d error(s), want 2.", errors)
		}
		if code != 1 {
			t.Errorf("AC-45 (strict json): exited %d, want 1, matching text mode on the same workspace.", code)
		}
	})
}

// @spec spec-manifest
// @ac AC-67
//
// C-35: --strict promotes the domain tier conflict.
func TestStrictPromotesDomainTierConflict(t *testing.T) {
	t.Run("spec-manifest/AC-67 the domain tier conflict is an error under strict", func(t *testing.T) {
		dir := strictWorkspace(t)

		plainOut, _, plainCode := runCLISplit(t, dir, "check")
		if !strings.Contains(plainOut, "warn [domain_tier_conflict]") || plainCode != 0 {
			t.Fatalf("AC-67 (control): without --strict the kind must warn and exit 0; got exit %d.\nstdout:\n%s", plainCode, plainOut)
		}

		strictOut, _, strictCode := runCLISplit(t, dir, "check", "--strict")
		if !strings.Contains(strictOut, "error [domain_tier_conflict]") {
			t.Errorf("AC-67: C-35 says --strict promotes this diagnostic, and it is still a warning.\nstdout:\n%s", strictOut)
		}
		if strictCode != 1 {
			t.Errorf("AC-67: exited %d under --strict, want 1. Exiting 0 here is a false green in the command CI runs.", strictCode)
		}
	})
}

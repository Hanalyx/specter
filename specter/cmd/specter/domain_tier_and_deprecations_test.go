package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tierSpec writes a spec declaring the given tier, with one orphan constraint
// so check has something to say beyond the tier diagnostics.
func tierSpec(id string, tier int) string {
	return "spec:\n  id: " + id + "\n  version: \"1.0.0\"\n  status: draft\n  tier: " +
		string(rune('0'+tier)) + "\n  context:\n    system: test\n" +
		"  objective:\n    summary: A spec used to exercise tier diagnostics.\n" +
		"  constraints:\n    - id: C-01\n      description: \"MUST exist\"\n" +
		"  acceptance_criteria:\n    - id: AC-01\n      description: \"only criterion\"\n      references_constraints: [\"C-01\"]\n"
}

func tierWorkspace(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, "dt-spec.spec.yaml", tierSpec("dt-spec", 3))
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const domainConflictManifest = "system:\n  name: dt\ndomains:\n  payments:\n    tier: 1\n    specs:\n      - dt-spec\nsettings:\n  specs_dir: specs\n"

// @spec spec-manifest
// @ac AC-63
//
// C-35: the domain tier is a checked assertion. It warns and changes nothing.
func TestDomainTierConflict_WarnsAndChangesNothing(t *testing.T) {
	t.Run("spec-manifest/AC-63 domain tier conflict warns and changes nothing", func(t *testing.T) {
		dir := tierWorkspace(t, domainConflictManifest)

		stdout, stderr, code := runCLISplit(t, dir, "check")
		out := stdout + stderr
		if !strings.Contains(out, "domain_tier_conflict") {
			t.Errorf("C-35: expected a domain_tier_conflict diagnostic, got:\n%s", out)
		}
		for _, want := range []string{"dt-spec", "3", "payments", "1"} {
			if !strings.Contains(out, want) {
				t.Errorf("C-35: the diagnostic must name %q, got:\n%s", want, out)
			}
		}
		if code != 0 {
			t.Errorf("C-35: a warning must not fail the run, got exit %d", code)
		}

		// The load-bearing half. An implementation that made the domain tier
		// win would satisfy every assertion above and fail here.
		jsonOut, _, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation", "--json")
		var doc struct {
			Entries []struct {
				Tier      int `json:"tier"`
				Threshold int `json:"threshold"`
			} `json:"entries"`
		}
		if err := json.Unmarshal([]byte(jsonOut), &doc); err != nil {
			t.Fatalf("coverage --json did not parse: %v", err)
		}
		if len(doc.Entries) != 1 || doc.Entries[0].Tier != 3 || doc.Entries[0].Threshold != 50 {
			t.Errorf("C-35: nothing resolves, so the declared tier 3 and threshold 50 must stand, got %+v", doc.Entries)
		}
	})
}

// @spec spec-manifest
// @ac AC-64
//
// C-35 plus bugs/SP-SP-002: the diagnostic reaches check --json and the two
// modes agree on the count.
func TestDomainTierConflict_ReachesJSONAndCountsAgree(t *testing.T) {
	t.Run("spec-manifest/AC-64 domain tier conflict reaches json and counts agree", func(t *testing.T) {
		dir := tierWorkspace(t, domainConflictManifest)

		jsonOut, _, _ := runCLISplit(t, dir, "check", "--json")
		var doc struct {
			Diagnostics []struct {
				Kind string `json:"kind"`
			} `json:"diagnostics"`
			Summary struct {
				Warnings int `json:"warnings"`
			} `json:"summary"`
		}
		if err := json.Unmarshal([]byte(jsonOut), &doc); err != nil {
			t.Fatalf("check --json did not parse: %v\n%s", err, jsonOut)
		}
		found := false
		for _, d := range doc.Diagnostics {
			if d.Kind == "domain_tier_conflict" {
				found = true
			}
		}
		if !found {
			t.Errorf("SP-SP-002: domain_tier_conflict must appear in check --json, got kinds %+v", doc.Diagnostics)
		}

		textOut, textErr, _ := runCLISplit(t, dir, "check")
		combined := textOut + textErr
		textWarnings := strings.Count(combined, "warn [")
		if doc.Summary.Warnings != textWarnings {
			t.Errorf("SP-SP-002: json summary warnings = %d, text printed %d; the two modes must agree\ntext:\n%s",
				doc.Summary.Warnings, textWarnings, combined)
		}
	})
}

// @spec spec-manifest
// @ac AC-65
//
// C-36: the deprecated tier keys warn and change no exit code.
func TestDeprecatedTierKeys_WarnWithoutChangingExit(t *testing.T) {
	t.Run("spec-manifest/AC-65 deprecated tier keys warn without changing exit", func(t *testing.T) {
		base := "system:\n  name: dep\nsettings:\n  specs_dir: specs\n"
		withSystemTier := "system:\n  name: dep\n  tier: 2\nsettings:\n  specs_dir: specs\n"
		withOverrides := "system:\n  name: dep\nsettings:\n  specs_dir: specs\n  tier_overrides:\n    dt-spec: 1\n"

		_, baseErr, baseCode := runCLISplit(t, tierWorkspace(t, base), "check")
		// Control: a manifest declaring neither must warn about neither. Without
		// this the assertions below pass on a build that warns about everything.
		if strings.Contains(baseErr, "system.tier") || strings.Contains(baseErr, "tier_overrides is deprecated") {
			t.Errorf("control: a manifest declaring neither key must not warn about either, got:\n%s", baseErr)
		}

		_, sysErr, sysCode := runCLISplit(t, tierWorkspace(t, withSystemTier), "check")
		if !strings.Contains(sysErr, "system.tier") {
			t.Errorf("C-36: system.tier must warn, got:\n%s", sysErr)
		}
		if sysCode != baseCode {
			t.Errorf("C-36: the warning must not change the exit code, got %d want %d", sysCode, baseCode)
		}

		_, ovErr, ovCode := runCLISplit(t, tierWorkspace(t, withOverrides), "check")
		if !strings.Contains(ovErr, "tier_overrides") {
			t.Errorf("C-36: settings.tier_overrides must warn, got:\n%s", ovErr)
		}

		// The message's semantics, not just the key name. Asserting only that
		// the line mentions tier_overrides leaves the retired "has no effect"
		// claim passing, and that claim is what C-36 was corrected to forbid:
		// the key resolves no tier, its value is still compared, and a
		// disagreement becomes an error under --strict.
		for _, want := range []string{"resolves no tier", "still compared", "--strict", "v1.0.0"} {
			if !strings.Contains(ovErr, want) {
				t.Errorf("C-36: the tier_overrides line must state %q. Got:\n%s", want, ovErr)
			}
		}
		if strings.Contains(ovErr, "tier_overrides is deprecated and has no effect") {
			t.Errorf("C-36: the retired \"has no effect\" claim is back. The key is not inert: a mismatching value fails a strict run. Got:\n%s", ovErr)
		}

		// system.tier keeps its own wording, and genuinely has no effect. This
		// is the control that stops the assertions above being read as a rule
		// about every deprecated key.
		if !strings.Contains(sysErr, "system.tier is deprecated and has no effect") {
			t.Errorf("C-36: system.tier does have no effect and its line should still say so. Got:\n%s", sysErr)
		}

		// Cardinality, and the case where both keys are declared at once. C-36
		// says exactly one line per declared key. Contains cannot say that: a
		// duplicated line passes it, and so does a run that emits one line
		// when two keys are declared. Neither case is exercised by a fixture
		// that declares one key at a time.
		countLines := func(stderr, key string) int {
			n := 0
			for _, line := range strings.Split(stderr, "\n") {
				if strings.HasPrefix(line, "warn: "+key+" is deprecated") {
					n++
				}
			}
			return n
		}
		if got := countLines(sysErr, "system.tier"); got != 1 {
			t.Errorf("C-36: %d line(s) for system.tier, want exactly 1.\nstderr:\n%s", got, sysErr)
		}
		if got := countLines(ovErr, "settings.tier_overrides"); got != 1 {
			t.Errorf("C-36: %d line(s) for settings.tier_overrides, want exactly 1.\nstderr:\n%s", got, ovErr)
		}
		// One key declared must not produce the other key's line, in BOTH
		// directions. Asserting one direction leaves a run that emits the
		// system.tier line whenever tier_overrides is declared passing.
		if got := countLines(sysErr, "settings.tier_overrides"); got != 0 {
			t.Errorf("C-36: a manifest declaring only system.tier produced %d tier_overrides line(s), want 0.\nstderr:\n%s", got, sysErr)
		}
		if got := countLines(ovErr, "system.tier"); got != 0 {
			t.Errorf("C-36: a manifest declaring only settings.tier_overrides produced %d system.tier line(s), want 0.\nstderr:\n%s", got, ovErr)
		}

		both := "system:\n  name: dep\n  tier: 2\nsettings:\n  specs_dir: specs\n  tier_overrides:\n    dt-spec: 3\n"
		_, bothErr, bothCode := runCLISplit(t, tierWorkspace(t, both), "check")
		if got := countLines(bothErr, "system.tier"); got != 1 {
			t.Errorf("C-36 (both declared): %d line(s) for system.tier, want exactly 1.\nstderr:\n%s", got, bothErr)
		}
		if got := countLines(bothErr, "settings.tier_overrides"); got != 1 {
			t.Errorf("C-36 (both declared): %d line(s) for settings.tier_overrides, want exactly 1.\nstderr:\n%s", got, bothErr)
		}
		// A matching override, so the exit code still reflects the deprecation
		// lines alone rather than a tier_conflict.
		if bothCode != baseCode {
			t.Errorf("C-36 (both declared): exited %d, want %d. Two advisory lines change no exit code.", bothCode, baseCode)
		}
		if ovCode != baseCode {
			t.Errorf("C-36: the warning must not change the exit code, got %d want %d", ovCode, baseCode)
		}
	})
}

// @spec spec-manifest
// @ac AC-65
//
// C-36 and C-14: the deprecation line is advisory, and the conflict it
// accompanies is not. Declaring settings.tier_overrides does not fail a strict
// run; disagreeing with the declared tier does.
func TestDeprecatedOverrideStrictExit(t *testing.T) {
	t.Run("spec-manifest/AC-65 a matching override warns and still exits 0 under --strict", func(t *testing.T) {
		// dt-spec declares tier 3 in tierWorkspace, so 3 is the matching value.
		matching := "system:\n  name: dep\nsettings:\n  specs_dir: specs\n  tier_overrides:\n    dt-spec: 3\n"
		_, errOut, code := runCLISplit(t, tierWorkspace(t, matching), "check", "--strict")

		// The control. The deprecation line must be present, or this proves
		// nothing about a declared key.
		if !strings.Contains(errOut, "tier_overrides") {
			t.Fatalf("AC-65 (matching): the deprecation line is absent, so the key was not declared.\nstderr:\n%s", errOut)
		}
		if strings.Contains(errOut, "tier_conflict") {
			t.Errorf("AC-65 (matching): an override equal to the declared tier is not a conflict.\nstderr:\n%s", errOut)
		}
		if code != 0 {
			t.Errorf("AC-65 (matching): exited %d, want 0. Declaring the key is not what fails a strict run.", code)
		}
	})

	t.Run("spec-manifest/AC-65 a mismatching override exits 1 under --strict", func(t *testing.T) {
		mismatching := "system:\n  name: dep\nsettings:\n  specs_dir: specs\n  tier_overrides:\n    dt-spec: 1\n"
		out, errOut, code := runCLISplit(t, tierWorkspace(t, mismatching), "check", "--strict")

		if !strings.Contains(out, "error [tier_conflict]") {
			t.Errorf("AC-65 (mismatching): the conflict must be promoted under --strict, per spec-check C-07.\nstdout:\n%s\nstderr:\n%s", out, errOut)
		}
		if code != 1 {
			t.Errorf("AC-65 (mismatching): exited %d, want 1. Disagreeing with the declared tier is what fails a strict run.", code)
		}
	})
}

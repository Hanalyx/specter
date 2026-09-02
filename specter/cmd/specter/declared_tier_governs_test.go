package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @spec spec-manifest
// @ac AC-04
//
// C-04: the declared tier governs. Asserted end to end through the CLI across
// all three surfaces that read a tier, rather than against a resolver function.
// There is no resolver: SSRB-106 section 7.4 deleted ResolveTier, and a unit
// test calling it directly with a tier of zero was how three unreachable
// criteria stayed green.
func TestDeclaredTierGoverns(t *testing.T) {
	t.Run("spec-manifest/AC-04 the declared tier governs", func(t *testing.T) {
		dir := t.TempDir()
		// tier: 3 in the spec, contradicted by a domain at 1 and system.tier 2.
		spec := "spec:\n  id: dt-spec\n  version: \"1.0.0\"\n  status: draft\n  tier: 3\n" +
			"  context:\n    system: test\n" +
			"  objective:\n    summary: The declared tier must win over both manifest levels.\n" +
			"  constraints:\n    - id: C-01\n      description: \"MUST be referenced by nothing, so it orphans\"\n" +
			"  acceptance_criteria:\n    - id: AC-01\n      description: \"only criterion\"\n"
		writeSpec(t, dir, "dt-spec.spec.yaml", spec)
		manifest := "system:\n  name: dt\n  tier: 2\ndomains:\n  payments:\n    tier: 1\n    specs:\n      - dt-spec\nsettings:\n  specs_dir: specs\n"
		if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		// Surface 1 and 2: the tier and the threshold reported by coverage.
		stdout, _, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation", "--json")
		var doc struct {
			Entries []struct {
				Tier      int `json:"tier"`
				Threshold int `json:"threshold"`
			} `json:"entries"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("coverage --json did not parse: %v\n%s", err, stdout)
		}
		if len(doc.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(doc.Entries))
		}
		if doc.Entries[0].Tier != 3 {
			t.Errorf("C-04: tier = %d, want 3. A domain tier of 1 or system.tier 2 must not win", doc.Entries[0].Tier)
		}
		if doc.Entries[0].Threshold != 50 {
			t.Errorf("C-04: threshold = %d, want 50, the Tier 3 default", doc.Entries[0].Threshold)
		}

		// Surface 3: orphan severity, which routes by tier. Tier 3 is info;
		// a domain tier of 1 winning would make it error.
		out, errOut, _ := runCLISplit(t, dir, "check")
		combined := out + errOut
		if !strings.Contains(combined, "info [orphan_constraint]") {
			t.Errorf("C-04: orphan must render at info, the Tier 3 severity, got:\n%s", combined)
		}
	})
}

// scaffold_tier_test.go: the scaffold and refresh write no deprecated tier key.
//
// @spec spec-manifest
package manifest

import "testing"

// @ac AC-74
func TestScaffoldAndRefresh_WriteNoTier(t *testing.T) {
	t.Run("spec-manifest/AC-74 scaffold writes no system or domain tier", func(t *testing.T) {
		for _, ids := range [][]string{nil, {"auth", "payments"}} {
			m, err := ParseManifest(ScaffoldManifest("my-app", "", ids))
			if err != nil {
				t.Fatalf("scaffold does not parse: %v", err)
			}
			if m.System.Tier != 0 {
				t.Errorf("spec ids %v: scaffold sets system.tier = %d", ids, m.System.Tier)
			}
			if len(m.Domains) == 0 {
				t.Fatalf("spec ids %v: scaffold has no domains; the domain check would be vacuous", ids)
			}
			for name, d := range m.Domains {
				if d.Tier != 0 {
					t.Errorf("spec ids %v: scaffold sets domains.%s.tier = %d", ids, name, d.Tier)
				}
			}
		}
	})

	t.Run("spec-manifest/AC-74 refresh does not copy system.tier into a synthesized default domain", func(t *testing.T) {
		existing, err := ParseManifest("schema_version: 1\nsystem:\n  name: demo\n  tier: 2\n")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		// Precondition: the input really declares the tier the fix must not copy.
		if existing.System.Tier != 2 {
			t.Fatalf("fixture system.tier = %d, want 2", existing.System.Tier)
		}
		out, _ := RefreshManifestDomains(existing, []string{"spec-a"})
		d, ok := out.Domains["default"]
		if !ok {
			t.Fatal("refresh did not synthesize a default domain")
		}
		if d.Tier != 0 {
			t.Errorf("synthesized default domain tier = %d, want unset", d.Tier)
		}
	})
}

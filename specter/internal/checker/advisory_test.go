// advisory_test.go -- C-15: structural_conflict is advisory.
//
// @spec spec-check
package checker

import (
	"testing"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// conflictGraph builds the two-spec workspace the detector fires on: an
// upstream constraint requiring a subject, a downstream criterion naming that
// subject beside an absence word, joined by a requires edge.
//
// enforcement is set on the upstream constraint so AC-35 can vary it.
func conflictGraph(enforcement string) *resolver.SpecGraph {
	upstream := makeSpec("user-reg", 1)
	upstream.Constraints = []schema.Constraint{
		{ID: "C-01", Description: "email MUST be required", Enforcement: enforcement},
	}
	upstream.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "covers it", ReferencesConstraints: []string{"C-01"}},
	}

	downstream := makeSpec("guest", 2)
	downstream.Constraints = []schema.Constraint{{ID: "C-01", Description: "downstream"}}
	downstream.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "Process checkout when email is absent",
			ReferencesConstraints: []string{"C-01"}},
	}

	return makeGraph(
		map[string]*resolver.SpecNode{
			"user-reg": {Spec: upstream, File: "u.yaml"},
			"guest":    {Spec: downstream, File: "g.yaml"},
		},
		[]resolver.SpecEdge{{From: "guest", To: "user-reg", Relationship: "requires"}},
	)
}

func structuralConflicts(result *CheckResult) []CheckDiagnostic {
	var out []CheckDiagnostic
	for _, d := range result.Diagnostics {
		if d.Kind == "structural_conflict" {
			out = append(out, d)
		}
	}
	return out
}

// @ac AC-34
// The diagnostic is emitted and the run passes, with and without --strict.
//
// Without the strict half this test would pass on a fix that only lowered the
// default severity, which C-07 promotes straight back. That is the refuted
// remedy recorded in SP-004.
func TestStructuralConflictIsAdvisory(t *testing.T) {
	t.Run("spec-check/AC-34 structural conflict is advisory", func(t *testing.T) {
		for _, strict := range []bool{false, true} {
			label := "check"
			if strict {
				label = "check --strict"
			}
			result := CheckSpecs(conflictGraph(""), &CheckOptions{Strict: strict})

			found := structuralConflicts(result)
			if len(found) != 1 {
				t.Fatalf("%s: expected exactly one structural_conflict, got %d", label, len(found))
			}
			if found[0].Severity != "info" {
				t.Errorf("%s: severity is %q, want %q", label, found[0].Severity, "info")
			}
			if result.Summary.Errors != 0 {
				t.Errorf("%s: summary reports %d error(s); an advisory diagnostic must not fail the run",
					label, result.Summary.Errors)
			}
		}
	})
}

// @ac AC-35
// The upstream constraint's enforcement field does not raise it. That field
// says how strictly the constraint binds the system, not how much to trust a
// lexical match, and reading it here let a Tier 1 constraint make a heuristic
// fail a build.
func TestStructuralConflictIgnoresUpstreamEnforcement(t *testing.T) {
	t.Run("spec-check/AC-35 structural conflict ignores upstream enforcement", func(t *testing.T) {
		for _, enforcement := range []string{"error", "warning", "info", ""} {
			result := CheckSpecs(conflictGraph(enforcement), nil)
			found := structuralConflicts(result)
			if len(found) != 1 {
				t.Fatalf("enforcement=%q: expected one structural_conflict, got %d", enforcement, len(found))
			}
			if found[0].Severity != "info" {
				t.Errorf("enforcement=%q raised the diagnostic to %q; C-15 requires info regardless",
					enforcement, found[0].Severity)
			}
			if result.Summary.Errors != 0 {
				t.Errorf("enforcement=%q: run reports %d error(s)", enforcement, result.Summary.Errors)
			}
		}
	})
}

// @ac AC-36
// The advisory posture is scoped to this one diagnostic. A Tier 1 orphan in the
// same run still fails, so the change cannot be read as weakening `check`.
func TestAdvisoryDoesNotWeakenOtherChecks(t *testing.T) {
	t.Run("spec-check/AC-36 advisory does not weaken other checks", func(t *testing.T) {
		g := conflictGraph("")
		// A Tier 1 orphan: a constraint no criterion references.
		up := g.Nodes["user-reg"].Spec
		up.Constraints = append(up.Constraints,
			schema.Constraint{ID: "C-02", Description: "referenced by nothing"})
		g.Nodes["user-reg"].Spec = up

		result := CheckSpecs(g, nil)

		if result.Summary.Errors == 0 {
			t.Error("a Tier 1 orphan constraint must still fail the run")
		}
		var orphanIsError bool
		for _, d := range result.Diagnostics {
			if d.Kind == "orphan_constraint" && d.Severity == "error" {
				orphanIsError = true
			}
		}
		if !orphanIsError {
			t.Error("expected the Tier 1 orphan at error severity")
		}
		found := structuralConflicts(result)
		if len(found) != 1 || found[0].Severity != "info" {
			t.Errorf("structural conflict should still be reported at info alongside the failure; got %+v", found)
		}
	})
}

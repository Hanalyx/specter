// @spec spec-check
package checker

import (
	"testing"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

func makeSpec(id string, tier int) schema.SpecAST {
	return schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: tier,
		Context:   schema.SpecContext{System: "test"},
		Objective: schema.SpecObjective{Summary: "test"},
		Constraints: []schema.Constraint{
			{ID: "C-01", Description: "test constraint"},
		},
		AcceptanceCriteria: []schema.AcceptanceCriterion{
			{ID: "AC-01", Description: "test ac", ReferencesConstraints: []string{"C-01"}},
		},
	}
}

func makeGraph(nodes map[string]*resolver.SpecNode, edges []resolver.SpecEdge) *resolver.SpecGraph {
	return &resolver.SpecGraph{Nodes: nodes, Edges: edges}
}

// @ac AC-01
func TestOrphanConstraint(t *testing.T) {
	spec := makeSpec("test", 2)
	spec.Constraints = append(spec.Constraints,
		schema.Constraint{ID: "C-02", Description: "referenced"},
		schema.Constraint{ID: "C-03", Description: "NOT referenced"},
	)
	spec.AcceptanceCriteria = append(spec.AcceptanceCriteria,
		schema.AcceptanceCriterion{ID: "AC-02", Description: "test", ReferencesConstraints: []string{"C-02"}},
	)

	g := makeGraph(map[string]*resolver.SpecNode{"test": {Spec: spec, File: "test.yaml"}}, nil)
	result := CheckSpecs(g, nil)

	orphans := 0
	for _, d := range result.Diagnostics {
		if d.Kind == "orphan_constraint" && d.ConstraintID == "C-03" {
			orphans++
		}
	}
	if orphans != 1 {
		t.Errorf("expected 1 orphan for C-03, got %d", orphans)
	}
}

// @ac AC-02
func TestTierBasedSeverity(t *testing.T) {
	tier1 := makeSpec("t1", 1)
	tier1.Constraints = []schema.Constraint{{ID: "C-01", Description: "orphan"}}
	tier1.AcceptanceCriteria = []schema.AcceptanceCriterion{{ID: "AC-01", Description: "test"}}

	tier3 := makeSpec("t3", 3)
	tier3.Constraints = []schema.Constraint{{ID: "C-01", Description: "orphan"}}
	tier3.AcceptanceCriteria = []schema.AcceptanceCriterion{{ID: "AC-01", Description: "test"}}

	g1 := makeGraph(map[string]*resolver.SpecNode{"t1": {Spec: tier1, File: "t1.yaml"}}, nil)
	g3 := makeGraph(map[string]*resolver.SpecNode{"t3": {Spec: tier3, File: "t3.yaml"}}, nil)

	r1 := CheckSpecs(g1, nil)
	r3 := CheckSpecs(g3, nil)

	var sev1, sev3 string
	for _, d := range r1.Diagnostics {
		if d.Kind == "orphan_constraint" {
			sev1 = d.Severity
		}
	}
	for _, d := range r3.Diagnostics {
		if d.Kind == "orphan_constraint" {
			sev3 = d.Severity
		}
	}

	if sev1 != "error" {
		t.Errorf("expected tier 1 orphan severity 'error', got %q", sev1)
	}
	if sev3 != "info" {
		t.Errorf("expected tier 3 orphan severity 'info', got %q", sev3)
	}
}

// @ac AC-03
func TestStructuralConflict(t *testing.T) {
	upstream := makeSpec("user-reg", 1)
	upstream.Constraints = []schema.Constraint{{ID: "C-01", Description: "email MUST be required"}}
	upstream.AcceptanceCriteria = []schema.AcceptanceCriterion{{ID: "AC-01", Description: "test", ReferencesConstraints: []string{"C-01"}}}

	downstream := makeSpec("guest", 2)
	downstream.Constraints = []schema.Constraint{{ID: "C-01", Description: "test"}}
	downstream.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "Process checkout when email is absent", ReferencesConstraints: []string{"C-01"}},
	}

	g := makeGraph(
		map[string]*resolver.SpecNode{
			"user-reg": {Spec: upstream, File: "u.yaml"},
			"guest":    {Spec: downstream, File: "g.yaml"},
		},
		[]resolver.SpecEdge{{From: "guest", To: "user-reg", Relationship: "requires"}},
	)

	result := CheckSpecs(g, nil)
	found := false
	for _, d := range result.Diagnostics {
		if d.Kind == "structural_conflict" {
			found = true
		}
	}
	if !found {
		t.Error("expected structural_conflict diagnostic")
	}
}

// @ac AC-04
func TestBreakingChangeRemoval(t *testing.T) {
	v1 := makeSpec("test", 2)
	v1.Constraints = []schema.Constraint{
		{ID: "C-01", Description: "keep"},
		{ID: "C-02", Description: "removed"},
	}
	v1.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "test", ReferencesConstraints: []string{"C-01"}},
		{ID: "AC-02", Description: "test", ReferencesConstraints: []string{"C-02"}},
	}

	v2 := makeSpec("test", 2)

	changes := ClassifyChanges(&v1, &v2)
	if HighestClassification(changes) != "breaking" {
		t.Errorf("expected breaking, got %s", HighestClassification(changes))
	}
}

// @ac AC-05
func TestAdditiveChange(t *testing.T) {
	v1 := makeSpec("test", 2)
	v2 := makeSpec("test", 2)
	v2.AcceptanceCriteria = append(v2.AcceptanceCriteria,
		schema.AcceptanceCriterion{ID: "AC-02", Description: "new"},
	)

	changes := ClassifyChanges(&v1, &v2)
	if HighestClassification(changes) != "additive" {
		t.Errorf("expected additive, got %s", HighestClassification(changes))
	}
}

// @ac AC-06
func TestNoOrphansWhenAllReferenced(t *testing.T) {
	spec := makeSpec("full", 1)
	spec.Constraints = []schema.Constraint{
		{ID: "C-01", Description: "a"},
		{ID: "C-02", Description: "b"},
	}
	spec.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "test", ReferencesConstraints: []string{"C-01"}},
		{ID: "AC-02", Description: "test", ReferencesConstraints: []string{"C-02"}},
	}

	g := makeGraph(map[string]*resolver.SpecNode{"full": {Spec: spec, File: "f.yaml"}}, nil)
	result := CheckSpecs(g, nil)

	for _, d := range result.Diagnostics {
		if d.Kind == "orphan_constraint" {
			t.Errorf("unexpected orphan: %s", d.ConstraintID)
		}
	}
}

// @ac AC-07
func TestStrictModeUpgradesWarningsToErrors(t *testing.T) {
	// Tier 2 orphan is normally a warning
	spec := makeSpec("mid", 2)
	spec.Constraints = []schema.Constraint{
		{ID: "C-01", Description: "referenced"},
		{ID: "C-02", Description: "orphan"},
	}
	spec.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "test", ReferencesConstraints: []string{"C-01"}},
	}

	g := makeGraph(map[string]*resolver.SpecNode{"mid": {Spec: spec, File: "mid.yaml"}}, nil)

	// Without strict: warning
	r := CheckSpecs(g, nil)
	for _, d := range r.Diagnostics {
		if d.Kind == "orphan_constraint" && d.Severity != "warning" {
			t.Errorf("expected warning without strict, got %q", d.Severity)
		}
	}
	if r.Summary.Errors > 0 {
		t.Error("expected no errors without strict mode")
	}

	// With strict: error
	rs := CheckSpecs(g, &CheckOptions{Strict: true})
	for _, d := range rs.Diagnostics {
		if d.Kind == "orphan_constraint" && d.Severity != "error" {
			t.Errorf("expected error with strict=true, got %q", d.Severity)
		}
	}
	if rs.Summary.Errors == 0 {
		t.Error("expected errors > 0 in strict mode")
	}
}

// @ac AC-08
func TestWarnOnDraftEmitsDraftSpecDiagnostic(t *testing.T) {
	spec := makeSpec("draft-spec", 2)
	spec.Status = "draft"

	g := makeGraph(map[string]*resolver.SpecNode{"draft-spec": {Spec: spec, File: "d.yaml"}}, nil)

	// Without warn_on_draft: no draft diagnostic
	r := CheckSpecs(g, nil)
	for _, d := range r.Diagnostics {
		if d.Kind == "draft_spec" {
			t.Error("unexpected draft_spec diagnostic without WarnOnDraft")
		}
	}

	// With warn_on_draft: warning emitted
	rw := CheckSpecs(g, &CheckOptions{WarnOnDraft: true})
	found := false
	for _, d := range rw.Diagnostics {
		if d.Kind == "draft_spec" && d.SpecID == "draft-spec" && d.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Error("expected draft_spec warning diagnostic with WarnOnDraft=true")
	}
}

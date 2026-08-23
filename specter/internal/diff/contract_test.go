// contract_test.go -- C-13: the comparator sees the fields that make a
// criterion concrete, not only its id and description.
//
// @spec spec-diff
package diff

import (
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

// baseSpec returns a one-criterion spec carrying every field C-13 names, so a
// test can vary exactly one of them and hold the rest.
func baseSpec() schema.SpecAST {
	return schema.SpecAST{
		ID: "demo", Version: "1.0.0", Status: "approved", Tier: 2,
		Context:   schema.SpecContext{System: "fixture"},
		Objective: schema.SpecObjective{Summary: "fixture"},
		Constraints: []schema.Constraint{
			{ID: "C-01", Description: "The value MUST be present"},
		},
		AcceptanceCriteria: []schema.AcceptanceCriterion{{
			ID:                    "AC-01",
			Description:           "same description throughout",
			Inputs:                map[string]interface{}{"payload": "one"},
			ExpectedOutput:        map[string]interface{}{"order": []interface{}{1, 2, 3}},
			ErrorCases:            []schema.ErrorCase{{Condition: "missing value", ExpectedBehavior: "rejected"}},
			ReferencesConstraints: []string{"C-01"},
			Priority:              "high",
			ApprovalGate:          true,
		}},
	}
}

// acChangeFor returns the change reported for the given criterion id.
func acChangeFor(d *SpecDiff, id string) *ItemChange {
	for i := range d.ACChanges {
		if d.ACChanges[i].ID == id {
			return &d.ACChanges[i]
		}
	}
	return nil
}

// @ac AC-16
// An inverted expected_output is a changed criterion and a breaking change.
// Identical id, identical description. This is the goalpost move a freeze
// comparator exists to catch, and today it reports no changes.
func TestDiff_InvertedExpectedOutputIsBreaking(t *testing.T) {
	t.Run("spec-diff/AC-16 inverted expected output is breaking", func(t *testing.T) {
		before := baseSpec()
		after := baseSpec()
		after.AcceptanceCriteria[0].ExpectedOutput = map[string]interface{}{
			"order": []interface{}{3, 2, 1},
		}

		d := DiffSpecs(before, after)
		if c := acChangeFor(d, "AC-01"); c == nil {
			t.Error("no change reported for AC-01; inverting expected_output must be visible")
		}
		if d.Class != ChangeBreaking {
			t.Errorf("classification is %q, want %q", d.Class, ChangeBreaking)
		}
	})
}

// @ac AC-17
// The other four fields behave the same way. A table rather than four tests,
// because the property is one property and the next field added to the
// criterion schema should join it here.
func TestDiff_EveryContractFieldIsBreaking(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(*schema.AcceptanceCriterion)
	}{
		{"approval_gate", func(ac *schema.AcceptanceCriterion) { ac.ApprovalGate = false }},
		{"inputs", func(ac *schema.AcceptanceCriterion) {
			ac.Inputs = map[string]interface{}{"payload": "two"}
		}},
		{"error_cases", func(ac *schema.AcceptanceCriterion) {
			ac.ErrorCases = []schema.ErrorCase{{Condition: "value out of range", ExpectedBehavior: "rejected"}}
		}},
		{"references_constraints", func(ac *schema.AcceptanceCriterion) {
			ac.ReferencesConstraints = []string{"C-02"}
		}},
	}

	for _, c := range cases {
		t.Run("spec-diff/AC-17 "+c.field+" is breaking", func(t *testing.T) {
			before := baseSpec()
			after := baseSpec()
			c.mutate(&after.AcceptanceCriteria[0])

			d := DiffSpecs(before, after)
			if ch := acChangeFor(d, "AC-01"); ch == nil {
				t.Errorf("changing %s reported no change", c.field)
			}
			if d.Class != ChangeBreaking {
				t.Errorf("changing %s classified %q, want %q", c.field, d.Class, ChangeBreaking)
			}
		})
	}
}

// @ac AC-18
// A priority change is breaking in either direction.
//
// The upgrade is what C-06 has required since 2.0.0 with no implementation.
// The downgrade is C-13 being deliberately broader: it says a criterion matters
// less than it did, which weakens the contract without touching a word of the
// criterion. A comparator that catches the upgrade and not the downgrade
// catches the honest edit and misses the convenient one.
func TestDiff_PriorityChangeIsBreakingBothWays(t *testing.T) {
	cases := []struct{ name, from, to string }{
		{"upgrade", "low", "high"},
		{"downgrade", "high", "low"},
	}
	for _, c := range cases {
		t.Run("spec-diff/AC-18 priority "+c.name+" is breaking", func(t *testing.T) {
			before := baseSpec()
			before.AcceptanceCriteria[0].Priority = c.from
			after := baseSpec()
			after.AcceptanceCriteria[0].Priority = c.to

			d := DiffSpecs(before, after)
			if d.Class != ChangeBreaking {
				t.Errorf("priority %s to %s classified %q, want %q",
					c.from, c.to, d.Class, ChangeBreaking)
			}
		})
	}
}

// @ac AC-16
// Identical specs report nothing. The widened comparator must not invent a
// change from a field it now reads, and a map-valued field is the risk: Go map
// iteration is unordered, so a fingerprint built by ranging one would differ
// between two runs over identical input.
func TestDiff_IdenticalSpecsReportNoChange(t *testing.T) {
	t.Run("spec-diff/AC-16 identical specs report no change", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			d := DiffSpecs(baseSpec(), baseSpec())
			if len(d.ACChanges) != 0 {
				t.Fatalf("run %d reported %d change(s) between identical specs: %+v",
					i, len(d.ACChanges), d.ACChanges)
			}
		}
	})
}

// @ac AC-16
// A description-only change stays a patch. Widening the comparator must not
// promote every edit to breaking, or the classification stops carrying
// information.
func TestDiff_DescriptionOnlyChangeStaysPatch(t *testing.T) {
	t.Run("spec-diff/AC-16 description-only change stays patch", func(t *testing.T) {
		before := baseSpec()
		after := baseSpec()
		after.AcceptanceCriteria[0].Description = "reworded, same contract"

		d := DiffSpecs(before, after)
		if ch := acChangeFor(d, "AC-01"); ch == nil {
			t.Fatal("a reworded description must still be reported")
		}
		if d.Class == ChangeBreaking {
			t.Error("a description-only change classified breaking; only contract fields do that")
		}
	})
}

// diff_test.go — tests for the DiffSpecs pure function.
//
// @spec spec-diff
package diff

import (
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

func makeAC(id, desc string) schema.AcceptanceCriterion {
	return schema.AcceptanceCriterion{ID: id, Description: desc}
}

func makeConstraint(id, desc string) schema.Constraint {
	return schema.Constraint{ID: id, Description: desc}
}

func makeSpec(id, version string) schema.SpecAST {
	return schema.SpecAST{ID: id, Version: version}
}

// @ac AC-10
func TestDiffSpecs_Identical_ReturnsUnchanged(t *testing.T) {
	s := makeSpec("my-spec", "1.0.0")
	s.AcceptanceCriteria = []schema.AcceptanceCriterion{makeAC("AC-01", "foo")}
	d := DiffSpecs(s, s)
	if d.Class != ChangeUnchanged {
		t.Errorf("expected unchanged, got %s", d.Class)
	}
	if len(d.ACChanges) != 0 {
		t.Errorf("expected no AC changes, got %d", len(d.ACChanges))
	}
}

// @ac AC-02
func TestDiffSpecs_AddedAC(t *testing.T) {
	v1 := makeSpec("my-spec", "1.0.0")
	v1.AcceptanceCriteria = []schema.AcceptanceCriterion{makeAC("AC-01", "foo")}
	v2 := makeSpec("my-spec", "1.1.0")
	v2.AcceptanceCriteria = []schema.AcceptanceCriterion{
		makeAC("AC-01", "foo"),
		makeAC("AC-02", "bar"),
	}
	d := DiffSpecs(v1, v2)
	if d.Class != ChangeAdditive {
		t.Errorf("expected additive, got %s", d.Class)
	}
	found := false
	for _, c := range d.ACChanges {
		if c.Kind == "added" && c.ID == "AC-02" {
			found = true
		}
	}
	if !found {
		t.Error("expected AC-02 to be in added changes")
	}
}

// @ac AC-03
func TestDiffSpecs_RemovedAC_IsBreaking(t *testing.T) {
	v1 := makeSpec("my-spec", "1.0.0")
	v1.AcceptanceCriteria = []schema.AcceptanceCriterion{
		makeAC("AC-01", "foo"),
		makeAC("AC-02", "bar"),
	}
	v2 := makeSpec("my-spec", "2.0.0")
	v2.AcceptanceCriteria = []schema.AcceptanceCriterion{makeAC("AC-01", "foo")}
	d := DiffSpecs(v1, v2)
	if d.Class != ChangeBreaking {
		t.Errorf("expected breaking, got %s", d.Class)
	}
}

// @ac AC-04
func TestDiffSpecs_AddedConstraint(t *testing.T) {
	v1 := makeSpec("my-spec", "1.0.0")
	v1.Constraints = []schema.Constraint{makeConstraint("C-01", "must work")}
	v2 := makeSpec("my-spec", "1.1.0")
	v2.Constraints = []schema.Constraint{
		makeConstraint("C-01", "must work"),
		makeConstraint("C-02", "must scale"),
	}
	d := DiffSpecs(v1, v2)
	found := false
	for _, c := range d.ConstraintChanges {
		if c.Kind == "added" && c.ID == "C-02" {
			found = true
		}
	}
	if !found {
		t.Error("expected C-02 in added constraint changes")
	}
}

// @ac AC-05
func TestDiffSpecs_DepVersionChange(t *testing.T) {
	v1 := makeSpec("my-spec", "1.0.0")
	v1.DependsOn = []schema.DependencyRef{{SpecID: "auth", VersionRange: "any"}}
	v2 := makeSpec("my-spec", "1.1.0")
	v2.DependsOn = []schema.DependencyRef{{SpecID: "auth", VersionRange: "^1.0.0"}}
	d := DiffSpecs(v1, v2)
	if len(d.DepChanges) != 1 {
		t.Fatalf("expected 1 dep change, got %d", len(d.DepChanges))
	}
	if d.DepChanges[0].OldRange != "any" || d.DepChanges[0].NewRange != "^1.0.0" {
		t.Errorf("unexpected dep change: %+v", d.DepChanges[0])
	}
}

// @ac AC-08
func TestDiffSpecs_DescriptionOnly_IsPatch(t *testing.T) {
	v1 := makeSpec("my-spec", "1.0.0")
	v1.AcceptanceCriteria = []schema.AcceptanceCriterion{makeAC("AC-01", "old desc")}
	v2 := makeSpec("my-spec", "1.0.1")
	v2.AcceptanceCriteria = []schema.AcceptanceCriterion{makeAC("AC-01", "new desc")}
	d := DiffSpecs(v1, v2)
	if d.Class != ChangePatch {
		t.Errorf("expected patch, got %s", d.Class)
	}
}

// Package diff implements spec-diff: semantic diff between two spec versions.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-diff
package diff

import (
	"encoding/json"

	"github.com/Hanalyx/specter/internal/schema"
)

// ItemChange represents a change to a single AC or constraint.
type ItemChange struct {
	Kind        string // "added", "removed", "changed"
	ID          string
	Description string // new description (for added/changed), old for removed
	OldDesc     string // for "changed" only
	// ContractChanged is true when a criterion's inputs, expected_output,
	// error_cases, references_constraints, priority or approval_gate changed.
	// C-13: that is a breaking change even when the description is identical.
	ContractChanged bool
}

// DepChange represents a version_range change in depends_on.
type DepChange struct {
	SpecID   string
	OldRange string
	NewRange string
}

// ChangeClass is the overall classification of the diff.
type ChangeClass string

const (
	ChangeBreaking  ChangeClass = "breaking"
	ChangeAdditive  ChangeClass = "additive"
	ChangePatch     ChangeClass = "patch"
	ChangeUnchanged ChangeClass = "unchanged"
)

// SpecDiff is the full semantic diff between two spec versions.
type SpecDiff struct {
	SpecID            string
	OldVersion        string
	NewVersion        string
	ACChanges         []ItemChange
	ConstraintChanges []ItemChange
	DepChanges        []DepChange
	Class             ChangeClass
}

// DiffSpecs computes the semantic diff between two SpecASTs.
//
// C-08: pure function, no I/O
func DiffSpecs(v1, v2 schema.SpecAST) *SpecDiff {
	d := &SpecDiff{
		SpecID:     v2.ID,
		OldVersion: v1.Version,
		NewVersion: v2.Version,
	}

	d.ACChanges = diffItems(acItems(v1), acItems(v2))
	d.ConstraintChanges = diffItems(constraintItems(v1), constraintItems(v2))
	d.DepChanges = diffDeps(v1.DependsOn, v2.DependsOn)
	d.Class = classify(d)
	return d
}

type namedItem struct {
	ID   string
	Desc string
	// Contract fingerprints the fields that make an acceptance criterion
	// concrete. Two criteria with the same id and description but different
	// contracts are a changed criterion and a breaking change, per C-13.
	// Empty for constraints, which have no contract fields.
	Contract string
}

// contractOf fingerprints the fields C-13 names, plus priority per C-06.
//
// json.Marshal is the serializer rather than fmt because it sorts map keys.
// `inputs` and `expected_output` are Go maps, and a fingerprint built by
// ranging one would differ between two runs over identical input, which would
// report a change between a spec and itself.
func contractOf(ac schema.AcceptanceCriterion) string {
	payload := struct {
		Inputs                map[string]interface{} `json:"inputs,omitempty"`
		ExpectedOutput        map[string]interface{} `json:"expected_output,omitempty"`
		ErrorCases            []schema.ErrorCase     `json:"error_cases,omitempty"`
		ReferencesConstraints []string               `json:"references_constraints,omitempty"`
		Priority              string                 `json:"priority,omitempty"`
		ApprovalGate          bool                   `json:"approval_gate,omitempty"`
	}{
		Inputs:                ac.Inputs,
		ExpectedOutput:        ac.ExpectedOutput,
		ErrorCases:            ac.ErrorCases,
		ReferencesConstraints: ac.ReferencesConstraints,
		Priority:              ac.Priority,
		ApprovalGate:          ac.ApprovalGate,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// A criterion that cannot be serialized is treated as its own
		// contract, so the comparison degrades to reporting a change rather
		// than to silently reporting none.
		return "unserializable:" + ac.ID
	}
	return string(b)
}

func acItems(s schema.SpecAST) []namedItem {
	out := make([]namedItem, len(s.AcceptanceCriteria))
	for i, ac := range s.AcceptanceCriteria {
		out[i] = namedItem{ID: ac.ID, Desc: ac.Description, Contract: contractOf(ac)}
	}
	return out
}

func constraintItems(s schema.SpecAST) []namedItem {
	out := make([]namedItem, len(s.Constraints))
	for i, c := range s.Constraints {
		out[i] = namedItem{ID: c.ID, Desc: c.Description}
	}
	return out
}

func diffItems(old, new []namedItem) []ItemChange {
	oldMap := make(map[string]namedItem)
	for _, item := range old {
		oldMap[item.ID] = item
	}
	newMap := make(map[string]namedItem)
	for _, item := range new {
		newMap[item.ID] = item
	}

	var changes []ItemChange
	// Removed or changed
	for _, item := range old {
		next, ok := newMap[item.ID]
		if !ok {
			changes = append(changes, ItemChange{Kind: "removed", ID: item.ID, Description: item.Desc})
			continue
		}
		// C-13: a criterion changes when its description changes OR when the
		// fields that make it concrete change. The two are recorded
		// separately, because only the second is breaking: a reworded
		// description leaves the contract its tests were written against
		// intact, and promoting every edit to breaking would empty the
		// classification of meaning.
		descChanged := next.Desc != item.Desc
		contractChanged := next.Contract != item.Contract
		if descChanged || contractChanged {
			changes = append(changes, ItemChange{
				Kind:            "changed",
				ID:              item.ID,
				Description:     next.Desc,
				OldDesc:         item.Desc,
				ContractChanged: contractChanged,
			})
		}
	}
	// Added
	for _, item := range new {
		if _, ok := oldMap[item.ID]; !ok {
			changes = append(changes, ItemChange{Kind: "added", ID: item.ID, Description: item.Desc})
		}
	}
	return changes
}

func diffDeps(old, new []schema.DependencyRef) []DepChange {
	oldMap := make(map[string]string)
	for _, d := range old {
		oldMap[d.SpecID] = d.VersionRange
	}
	newMap := make(map[string]string)
	for _, d := range new {
		newMap[d.SpecID] = d.VersionRange
	}

	var changes []DepChange
	for id, oldRange := range oldMap {
		if newRange, ok := newMap[id]; ok && newRange != oldRange {
			changes = append(changes, DepChange{SpecID: id, OldRange: oldRange, NewRange: newRange})
		}
	}
	return changes
}

func classify(d *SpecDiff) ChangeClass {
	for _, c := range d.ACChanges {
		if c.Kind == "removed" {
			return ChangeBreaking
		}
		// C-13 / C-06: the contract a criterion's tests were written against
		// changed, which includes a priority upgrade.
		if c.ContractChanged {
			return ChangeBreaking
		}
	}
	for _, c := range d.ConstraintChanges {
		if c.Kind == "removed" {
			return ChangeBreaking
		}
	}
	for _, c := range d.ACChanges {
		if c.Kind == "added" {
			return ChangeAdditive
		}
	}
	for _, c := range d.ConstraintChanges {
		if c.Kind == "added" {
			return ChangeAdditive
		}
	}
	hasChange := len(d.ACChanges) > 0 || len(d.ConstraintChanges) > 0 || len(d.DepChanges) > 0
	if hasChange {
		return ChangePatch
	}
	return ChangeUnchanged
}

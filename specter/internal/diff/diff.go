// Package diff implements spec-diff: semantic diff between two spec versions.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-diff
package diff

import "github.com/Hanalyx/specter/internal/schema"

// ItemChange represents a change to a single AC or constraint.
type ItemChange struct {
	Kind        string // "added", "removed", "changed"
	ID          string
	Description string // new description (for added/changed), old for removed
	OldDesc     string // for "changed" only
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
}

func acItems(s schema.SpecAST) []namedItem {
	out := make([]namedItem, len(s.AcceptanceCriteria))
	for i, ac := range s.AcceptanceCriteria {
		out[i] = namedItem{ac.ID, ac.Description}
	}
	return out
}

func constraintItems(s schema.SpecAST) []namedItem {
	out := make([]namedItem, len(s.Constraints))
	for i, c := range s.Constraints {
		out[i] = namedItem{c.ID, c.Description}
	}
	return out
}

func diffItems(old, new []namedItem) []ItemChange {
	oldMap := make(map[string]string)
	for _, item := range old {
		oldMap[item.ID] = item.Desc
	}
	newMap := make(map[string]string)
	for _, item := range new {
		newMap[item.ID] = item.Desc
	}

	var changes []ItemChange
	// Removed or changed
	for _, item := range old {
		if newDesc, ok := newMap[item.ID]; !ok {
			changes = append(changes, ItemChange{Kind: "removed", ID: item.ID, Description: item.Desc})
		} else if newDesc != item.Desc {
			changes = append(changes, ItemChange{Kind: "changed", ID: item.ID, Description: newDesc, OldDesc: item.Desc})
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

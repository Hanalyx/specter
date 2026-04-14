// Package checker implements spec-check: the type checker.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-check
package checker

import (
	"fmt"
	"strings"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// CheckDiagnostic represents an issue found during type-checking.
type CheckDiagnostic struct {
	Kind         string `json:"kind"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	SpecID       string `json:"spec_id"`
	ConstraintID string `json:"constraint_id,omitempty"`
	ChangeType   string `json:"change_type,omitempty"`
	Details      string `json:"details,omitempty"`
}

// CheckResult holds the outcome of all checks.
type CheckResult struct {
	Diagnostics []CheckDiagnostic `json:"diagnostics"`
	Summary     CheckSummary      `json:"summary"`
}

type CheckSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

// CheckOptions configures the check run.
type CheckOptions struct {
	TierOverride     int
	PreviousVersions map[string]*schema.SpecAST
	Strict           bool // C-07: upgrade all warning/info diagnostics to error
	WarnOnDraft      bool // C-08: emit warning for specs with status: draft
}

// Tier-based severity for orphan constraints.
var orphanSeverityByTier = map[int]string{
	1: "error",
	2: "warning",
	3: "info",
}

// CoverageThresholdByTier defines required coverage per tier.
var CoverageThresholdByTier = map[int]int{
	1: 100,
	2: 80,
	3: 50,
}

// CheckSpecs runs all structural checks on the spec graph.
//
// C-01: Detects orphan constraints.
// C-02: Tier-based severity.
// C-03: Structural conflict detection.
// C-04: Breaking change classification.
// C-05: Zero false positives for structural checks.
// C-06: Pure function.
func CheckSpecs(graph *resolver.SpecGraph, opts *CheckOptions) *CheckResult {
	if opts == nil {
		opts = &CheckOptions{}
	}

	var diagnostics []CheckDiagnostic

	// Rule 0: Draft spec warning (AC-08)
	if opts.WarnOnDraft {
		for _, node := range graph.Nodes {
			if node.Spec.Status == "draft" {
				diagnostics = append(diagnostics, CheckDiagnostic{
					Kind:     "draft_spec",
					Severity: "warning",
					Message:  fmt.Sprintf("Spec %q has status: draft — approve or remove before shipping", node.Spec.ID),
					SpecID:   node.Spec.ID,
				})
			}
		}
	}

	// Rule 1: Orphan constraints (AC-01, AC-02, AC-06)
	for _, node := range graph.Nodes {
		spec := node.Spec
		if opts.TierOverride > 0 {
			spec.Tier = opts.TierOverride
		}
		diagnostics = append(diagnostics, checkOrphanConstraints(&spec)...)
	}

	// Rule 2: Structural conflicts (AC-03)
	diagnostics = append(diagnostics, checkStructuralConflicts(graph)...)

	// Rule 3: Breaking changes (AC-04, AC-05)
	if opts.PreviousVersions != nil {
		for id, node := range graph.Nodes {
			prev, ok := opts.PreviousVersions[id]
			if !ok {
				continue
			}
			changes := ClassifyChanges(prev, &node.Spec)
			for _, change := range changes {
				kind := "patch_change"
				severity := "info"
				if change.Classification == "breaking" {
					kind = "breaking_change"
					severity = "error"
				} else if change.Classification == "additive" {
					kind = "additive_change"
				}
				diagnostics = append(diagnostics, CheckDiagnostic{
					Kind:       kind,
					Severity:   severity,
					Message:    fmt.Sprintf("%s: %s", node.Spec.ID, change.Description),
					SpecID:     node.Spec.ID,
					ChangeType: change.Classification,
					Details:    change.Field,
				})
			}
		}
	}

	// C-07: strict mode — upgrade warnings and info to errors
	if opts.Strict {
		for i := range diagnostics {
			if diagnostics[i].Severity == "warning" || diagnostics[i].Severity == "info" {
				diagnostics[i].Severity = "error"
			}
		}
	}

	result := &CheckResult{Diagnostics: diagnostics}
	for _, d := range diagnostics {
		switch d.Severity {
		case "error":
			result.Summary.Errors++
		case "warning":
			result.Summary.Warnings++
		case "info":
			result.Summary.Info++
		}
	}

	return result
}

// checkOrphanConstraints finds constraints not referenced by any AC.
func checkOrphanConstraints(spec *schema.SpecAST) []CheckDiagnostic {
	var diagnostics []CheckDiagnostic

	referenced := make(map[string]bool)
	for _, ac := range spec.AcceptanceCriteria {
		for _, ref := range ac.ReferencesConstraints {
			referenced[ref] = true
		}
	}

	for _, c := range spec.Constraints {
		if !referenced[c.ID] {
			severity := orphanSeverityByTier[spec.Tier]
			if severity == "" {
				severity = "warning"
			}
			diagnostics = append(diagnostics, CheckDiagnostic{
				Kind:         "orphan_constraint",
				Severity:     severity,
				Message:      fmt.Sprintf("Constraint %s in %q is not referenced by any acceptance criterion", c.ID, spec.ID),
				SpecID:       spec.ID,
				ConstraintID: c.ID,
			})
		}
	}

	return diagnostics
}

// checkStructuralConflicts detects contradictions between dependent specs.
func checkStructuralConflicts(graph *resolver.SpecGraph) []CheckDiagnostic {
	var diagnostics []CheckDiagnostic

	absenceKeywords := []string{"absent", "missing", "not provided", "not present", "is empty", "is null", "without"}
	requiredKeywords := []string{"MUST", "required", "MUST be present", "MUST exist", "mandatory"}

	for _, edge := range graph.Edges {
		if edge.Relationship != "requires" {
			continue
		}
		upstream, ok1 := graph.Nodes[edge.To]
		downstream, ok2 := graph.Nodes[edge.From]
		if !ok1 || !ok2 {
			continue
		}

		for _, constraint := range upstream.Spec.Constraints {
			desc := constraint.Description
			isRequired := false
			for _, kw := range requiredKeywords {
				if strings.Contains(desc, kw) {
					isRequired = true
					break
				}
			}
			if !isRequired {
				continue
			}

			subject := extractSubject(desc)
			if subject == "" {
				continue
			}

			for _, ac := range downstream.Spec.AcceptanceCriteria {
				acDesc := strings.ToLower(ac.Description)
				if !strings.Contains(acDesc, strings.ToLower(subject)) {
					continue
				}
				for _, kw := range absenceKeywords {
					if strings.Contains(acDesc, strings.ToLower(kw)) {
						diagnostics = append(diagnostics, CheckDiagnostic{
							Kind:         "structural_conflict",
							Severity:     "error",
							Message:      fmt.Sprintf("Structural conflict: %q constraint %s requires %q but %q %s handles it as absent", upstream.Spec.ID, constraint.ID, subject, downstream.Spec.ID, ac.ID),
							SpecID:       downstream.Spec.ID,
							ConstraintID: constraint.ID,
							Details:      fmt.Sprintf("Upstream: %s | Downstream AC: %s", desc, ac.Description),
						})
						break
					}
				}
			}
		}
	}

	return diagnostics
}

func extractSubject(description string) string {
	// Pattern: "<subject> MUST"
	idx := strings.Index(description, " MUST")
	if idx > 0 {
		return strings.TrimSpace(description[:idx])
	}
	return ""
}

// VersionChange represents a classified change between spec versions.
type VersionChange struct {
	Classification string `json:"classification"`
	Field          string `json:"field"`
	Description    string `json:"description"`
}

// ClassifyChanges compares two spec versions and classifies changes.
func ClassifyChanges(v1, v2 *schema.SpecAST) []VersionChange {
	var changes []VersionChange

	// Constraint changes
	v1c := make(map[string]*schema.Constraint)
	for i := range v1.Constraints {
		v1c[v1.Constraints[i].ID] = &v1.Constraints[i]
	}
	v2c := make(map[string]*schema.Constraint)
	for i := range v2.Constraints {
		v2c[v2.Constraints[i].ID] = &v2.Constraints[i]
	}

	for id := range v1c {
		if _, ok := v2c[id]; !ok {
			changes = append(changes, VersionChange{"breaking", "constraints." + id, "Constraint " + id + " was removed"})
		}
	}
	for id := range v2c {
		if _, ok := v1c[id]; !ok {
			changes = append(changes, VersionChange{"additive", "constraints." + id, "Constraint " + id + " was added"})
		}
	}

	// AC changes
	v1ac := make(map[string]bool)
	for _, ac := range v1.AcceptanceCriteria {
		v1ac[ac.ID] = true
	}
	v2ac := make(map[string]bool)
	for _, ac := range v2.AcceptanceCriteria {
		v2ac[ac.ID] = true
	}

	for id := range v1ac {
		if !v2ac[id] {
			changes = append(changes, VersionChange{"breaking", "acceptance_criteria." + id, "Acceptance criterion " + id + " was removed"})
		}
	}
	for id := range v2ac {
		if !v1ac[id] {
			changes = append(changes, VersionChange{"additive", "acceptance_criteria." + id, "Acceptance criterion " + id + " was added"})
		}
	}

	return changes
}

// HighestClassification returns the most severe classification.
func HighestClassification(changes []VersionChange) string {
	if len(changes) == 0 {
		return "none"
	}
	for _, c := range changes {
		if c.Classification == "breaking" {
			return "breaking"
		}
	}
	for _, c := range changes {
		if c.Classification == "additive" {
			return "additive"
		}
	}
	return "patch"
}

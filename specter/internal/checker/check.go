// Package checker implements spec-check: the type checker.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-check
package checker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// CheckDiagnostic represents an issue found during type-checking.
type CheckDiagnostic struct {
	Kind           string `json:"kind"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	SpecID         string `json:"spec_id"`
	ConstraintID   string `json:"constraint_id,omitempty"`
	ConstraintType string `json:"constraint_type,omitempty"`
	Details        string `json:"details,omitempty"`
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
	TierOverride int
	Strict       bool // C-07: upgrade all warning/info diagnostics to error
	WarnOnDraft  bool // C-08: emit warning for specs with status: draft
	// Concrete opts into the C-16 concreteness rule. C-17: it is off by
	// default because `inputs` and `expected_output` are optional in the
	// canonical schema, so a criterion carrying neither is valid and failing a
	// build on it would be a gate failing on correct input.
	Concrete bool

	// ExtraDiagnostics are diagnostics the caller assembled from sources this
	// package cannot see, currently the manifest-derived tier_conflict and
	// domain_tier_conflict. They join the run's own diagnostics BEFORE the
	// C-07 strict upgrade and before the summary, which is the whole reason
	// this field exists.
	//
	// The command used to append them to the returned result instead. That put
	// them past the upgrade loop, so both kept severity warning under --strict
	// and `check --strict` exited 0 on a workspace spec-manifest C-35 says it
	// should fail. Nothing failed when that happened, because the rule lived in
	// one place and the diagnostics arrived in another.
	//
	// C-07 requires one owner for the upgrade over the complete set. Passing
	// them in rather than promoting them at the call site is what keeps that
	// true: a second promotion step would be a private copy of the
	// non-promotable set, which is enumerated once, below.
	ExtraDiagnostics []CheckDiagnostic
}

// nonPromotable is the set C-07 names: kinds --strict does not upgrade.
//
// structural_conflict, per C-15(b). An advisory posture that holds until
// someone passes --strict holds until CI runs, which is the same as not
// holding.
//
// unreachable_annotation_unknown, per C-10. The scanner could not recognize
// the test shape, so it has found no defect to report. Promoting it would fail
// a build on the checker's own blind spot, and C-10 states in its own terms
// that this kind is always a warning regardless of strictness.
//
// Enumerated here and nowhere else. Adding a kind is an amendment to C-07, not
// a local choice at a call site.
var nonPromotable = map[string]bool{
	"structural_conflict":            true,
	"unreachable_annotation_unknown": true,
}

// vagueSeverityByTier is the C-16 gradient. It is a separate table from
// orphanSeverityByTier because the two rules are independent: changing one
// should not silently move the other.
var vagueSeverityByTier = map[int]string{
	1: "error",
	2: "warning",
	3: "info",
}

// checkConcreteness reports every criterion carrying neither `inputs` nor
// `expected_output`. C-16.
//
// Presence only. Whether "completes within budget" is a checkable assertion is
// semantic reading, and a rule that rejects the word "gracefully" is theater.
func checkConcreteness(graph *resolver.SpecGraph, ids []string) []CheckDiagnostic {
	var diagnostics []CheckDiagnostic
	for _, id := range ids {
		node := graph.Nodes[id]
		if node == nil {
			continue
		}
		severity := vagueSeverityByTier[node.Spec.Tier]
		if severity == "" {
			severity = "warning"
		}
		for _, ac := range node.Spec.AcceptanceCriteria {
			if len(ac.Inputs) > 0 || len(ac.ExpectedOutput) > 0 {
				continue
			}
			diagnostics = append(diagnostics, CheckDiagnostic{
				Kind:     "vague_criterion",
				Severity: severity,
				Message: fmt.Sprintf("Criterion %s in %q carries neither inputs nor expected_output, so nothing states what it asserts",
					ac.ID, node.Spec.ID),
				SpecID: node.Spec.ID,
			})
		}
	}
	return diagnostics
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
// C-05: Zero false positives for structural checks.
// C-06: Pure function.
// C-13: Duplicate acceptance criterion ids within one spec.
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

	// Rule 3 was breaking-change classification and is gone. spec-check 2.0.0
	// retracted C-04, AC-04 and AC-05, and `spec-diff` owns version comparison.
	// Nothing ever populated CheckOptions.PreviousVersions, so this branch never
	// ran while its three criteria reported covered off tests that called
	// ClassifyChanges directly (bugs/SP-SP-018).

	// Rule 4: Duplicate acceptance criterion ids (AC-21 through AC-27)
	//
	// Severity is error at every tier, so this reads node.Spec directly and
	// ignores opts.TierOverride. Spec ids are visited in sorted order, because
	// graph.Nodes is a map and a human diffing two runs should see the same
	// list twice.
	specIDs := make([]string, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		specIDs = append(specIDs, id)
	}
	sort.Strings(specIDs)
	for _, id := range specIDs {
		diagnostics = append(diagnostics, checkDuplicateACIDs(&graph.Nodes[id].Spec)...)
	}

	// C-16 / C-17: opt-in, and off unless asked for. Reuses the sorted id list
	// above for the same reason it exists.
	if opts.Concrete {
		diagnostics = append(diagnostics, checkConcreteness(graph, specIDs)...)
	}

	// C-07: everything the run reports joins the list before the upgrade, so
	// the upgrade owns the complete set rather than whatever this function
	// happened to produce.
	diagnostics = append(diagnostics, opts.ExtraDiagnostics...)

	// C-07: strict mode, upgrade warnings and info to errors, except the kinds
	// C-07 names as non-promotable. The exemptions live here rather than at the
	// emit sites because this loop is what would undo them, and one copy of the
	// set is the point: a second would not know what the set contains.
	if opts.Strict {
		for i := range diagnostics {
			if nonPromotable[diagnostics[i].Kind] {
				continue
			}
			if diagnostics[i].Severity == "warning" || diagnostics[i].Severity == "info" {
				diagnostics[i].Severity = "error"
			}
		}
	}

	return &CheckResult{Diagnostics: diagnostics, Summary: summarize(diagnostics)}
}

// summarize counts the final diagnostics. C-07 requires the summary to be
// computed once, after the upgrade, from the list the run reports.
//
// A function rather than a loop inline, so "computed once" is a property a
// reader and a guard can both check by counting call sites. The command used
// to count its own diagnostics into the summary as it appended them, which
// reported as a warning what the run reported as an error.
func summarize(diagnostics []CheckDiagnostic) CheckSummary {
	var s CheckSummary
	for _, d := range diagnostics {
		switch d.Severity {
		case "error":
			s.Errors++
		case "warning":
			s.Warnings++
		case "info":
			s.Info++
		}
	}
	return s
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
			// Explicit constraint.enforcement overrides the tier-based default.
			// This lets an author mark a single constraint as always-error (e.g. a
			// security rule in a tier-3 spec) without raising the whole tier.
			severity := orphanSeverityByTier[spec.Tier]
			if severity == "" {
				severity = "warning"
			}
			if c.Enforcement != "" {
				severity = c.Enforcement
			}
			diagnostics = append(diagnostics, CheckDiagnostic{
				Kind:           "orphan_constraint",
				Severity:       severity,
				Message:        fmt.Sprintf("Constraint %s in %q is not referenced by any acceptance criterion", c.ID, spec.ID),
				SpecID:         spec.ID,
				ConstraintID:   c.ID,
				ConstraintType: c.Type,
			})
		}
	}

	return diagnostics
}

// checkStructuralConflicts detects contradictions between dependent specs.
func checkStructuralConflicts(graph *resolver.SpecGraph) []CheckDiagnostic {
	var diagnostics []CheckDiagnostic

	// C-21: the absence vocabulary moved into the attachment scanner, which
	// needs it split into predicates and heads rather than as one flat list.
	// C-19: only ` MUST` is here, because it is the only keyword extraction can
	// turn into a subject. `required`, `MUST be present`, `MUST exist` and
	// `mandatory` used to sit in this list, set the required flag, and then
	// produce an empty subject a few lines below, so every constraint matching
	// one of them was discarded. They were removed rather than made to work:
	// the check is advisory under C-15 and C-05 declines to bound its false
	// positives, so matching more constraints buys more noise.
	//
	// ` MUST` is matched case sensitively, so a constraint written with a
	// lowercase `must` is invisible. That is a false negative in an advisory
	// check and is left as it is, deliberately.
	requiredKeywords := []string{"MUST"}

	// C-20: every edge, not only `requires`. A `conflicts_with` edge is where a
	// contradiction check has the strongest reason to look, and it was the one
	// place this did not look at all.
	for _, edge := range graph.Edges {
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
				// C-21: attachment, not co-occurrence. The absence expression
				// has to predicate the subject.
				//
				// There is no substring pre-filter above this. There used to
				// be, asking whether the lowercased subject appeared anywhere
				// in the criterion, and it is the co-occurrence test C-21
				// replaces. Keeping it would also have re-broken the article
				// case: a subject of "The audit record" is not a substring of
				// "without an audit record", so the criterion never reached
				// the scanner that strips the article.
				if AbsenceAttachesToSubject(subject, ac.Description) {
					// C-15(a): always info. The upstream constraint's
					// `enforcement` field says how strictly the constraint
					// binds the system, not how much to trust a lexical
					// match on a sentence. Reading it here let a Tier 1
					// constraint make a heuristic fail a build.
					severity := "info"
					diagnostics = append(diagnostics, CheckDiagnostic{
						Kind:     "structural_conflict",
						Severity: severity,
						Message:  fmt.Sprintf("Structural conflict: %q constraint %s requires %q but %q %s handles it as absent", upstream.Spec.ID, constraint.ID, subject, downstream.Spec.ID, ac.ID),
						SpecID:   downstream.Spec.ID,
						// C-18: qualified with the spec that owns it. The
						// header renders as "<SpecID> <ConstraintID>", and
						// SpecID is the downstream spec, so a bare upstream
						// id here reads as one of the downstream spec's own
						// constraints. A reader looking it up finds nothing,
						// or finds a different constraint sharing the id.
						ConstraintID:   upstream.Spec.ID + "/" + constraint.ID,
						ConstraintType: constraint.Type,
						Details:        fmt.Sprintf("Upstream: %s | Downstream AC: %s", desc, ac.Description),
					})
					// No break. Every criterion that conflicts with this
					// constraint is reported. The old code broke out of the
					// keyword loop here, which left the criterion loop running;
					// a bare break in this position would instead stop after
					// the first criterion and silently drop the rest.
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

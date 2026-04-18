// @spec spec-parse
//
// Go-level enum constants and validation.
//
// The canonical enforcement of enum values is the JSON Schema — any spec that
// reaches ParseSpec goes through schema validation first. But internal code
// that builds a SpecAST without going through ParseSpec (reverse compiler,
// migration scripts, tests that construct values directly) bypasses that
// check. ValidateEnums is the safety net for those paths.

package schema

import "fmt"

// Constraint enforcement levels (JSON Schema: constraint.enforcement)
const (
	EnforcementError   = "error"
	EnforcementWarning = "warning"
	EnforcementInfo    = "info"
)

// Constraint categories (JSON Schema: constraint.type)
const (
	ConstraintTypeTechnical     = "technical"
	ConstraintTypeSecurity      = "security"
	ConstraintTypePerformance   = "performance"
	ConstraintTypeAccessibility = "accessibility"
	ConstraintTypeBusiness      = "business"
)

// Acceptance criterion priorities (JSON Schema: acceptance_criterion.priority)
const (
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityMedium   = "medium"
	PriorityLow      = "low"
)

// Spec lifecycle statuses (JSON Schema: spec.status)
const (
	StatusDraft      = "draft"
	StatusReview     = "review"
	StatusApproved   = "approved"
	StatusDeprecated = "deprecated"
	StatusRemoved    = "removed"
)

// Dependency relationships (JSON Schema: dependency_ref.relationship)
const (
	RelationshipRequires      = "requires"
	RelationshipExtends       = "extends"
	RelationshipConflictsWith = "conflicts_with"
)

// Changelog entry types (JSON Schema: changelog_entry.type)
const (
	ChangelogInitial = "initial"
	ChangelogMajor   = "major"
	ChangelogMinor   = "minor"
	ChangelogPatch   = "patch"
)

// Changelog change types (JSON Schema: changelog_change.type)
const (
	ChangeAddition     = "addition"
	ChangeRemoval      = "removal"
	ChangeModification = "modification"
	ChangeDeprecation  = "deprecation"
)

// Constraint validation rule types (JSON Schema: constraint_validation.rule)
const (
	RuleType     = "type"
	RuleMin      = "min"
	RuleMax      = "max"
	RulePattern  = "pattern"
	RuleEnum     = "enum"
	RuleRequired = "required"
	RuleFormat   = "format"
	RuleCustom   = "custom"
)

// ValidateEnums walks the SpecAST and verifies every enum-valued string
// matches its declared enum set. Empty values are allowed for optional fields.
// Returns the first violation, or nil if all enums are valid (or unset).
//
// Safety net for internal code paths that skip ParseSpec; normal parse flow
// already enforces these values via JSON Schema.
func (s *SpecAST) ValidateEnums() error {
	if err := validateInSet("spec.status", s.Status, statusEnum); err != nil {
		return err
	}
	if err := validateTier(s.Tier); err != nil {
		return err
	}
	for i, c := range s.Constraints {
		if err := validateInSet(fmt.Sprintf("constraints[%s].type", c.ID), c.Type, constraintTypeEnum); err != nil {
			return err
		}
		if err := validateInSet(fmt.Sprintf("constraints[%s].enforcement", c.ID), c.Enforcement, enforcementEnum); err != nil {
			return err
		}
		if c.Validation != nil {
			if err := validateInSet(fmt.Sprintf("constraints[%s].validation.rule", c.ID), c.Validation.Rule, ruleEnum); err != nil {
				return err
			}
		}
		_ = i
	}
	for _, ac := range s.AcceptanceCriteria {
		if err := validateInSet(fmt.Sprintf("acceptance_criteria[%s].priority", ac.ID), ac.Priority, priorityEnum); err != nil {
			return err
		}
	}
	for _, dep := range s.DependsOn {
		if err := validateInSet(fmt.Sprintf("depends_on[%s].relationship", dep.SpecID), dep.Relationship, relationshipEnum); err != nil {
			return err
		}
	}
	for _, cl := range s.Changelog {
		if err := validateInSet(fmt.Sprintf("changelog[%s].type", cl.Version), cl.Type, changelogEnum); err != nil {
			return err
		}
		for _, ch := range cl.Changes {
			if err := validateInSet(fmt.Sprintf("changelog[%s].changes.type", cl.Version), ch.Type, changeTypeEnum); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTier(t int) error {
	switch t {
	case 0, 1, 2, 3:
		return nil
	default:
		return fmt.Errorf("invalid tier %d (must be 1, 2, or 3)", t)
	}
}

func validateInSet(field, value string, allowed []string) error {
	if value == "" {
		return nil // empty means unset — optional enum fields allow this
	}
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q (allowed: %v)", field, value, allowed)
}

var (
	statusEnum         = []string{StatusDraft, StatusReview, StatusApproved, StatusDeprecated, StatusRemoved}
	constraintTypeEnum = []string{ConstraintTypeTechnical, ConstraintTypeSecurity, ConstraintTypePerformance, ConstraintTypeAccessibility, ConstraintTypeBusiness}
	enforcementEnum    = []string{EnforcementError, EnforcementWarning, EnforcementInfo}
	priorityEnum       = []string{PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow}
	relationshipEnum   = []string{RelationshipRequires, RelationshipExtends, RelationshipConflictsWith}
	changelogEnum      = []string{ChangelogInitial, ChangelogMajor, ChangelogMinor, ChangelogPatch}
	changeTypeEnum     = []string{ChangeAddition, ChangeRemoval, ChangeModification, ChangeDeprecation}
	ruleEnum           = []string{RuleType, RuleMin, RuleMax, RulePattern, RuleEnum, RuleRequired, RuleFormat, RuleCustom}
)

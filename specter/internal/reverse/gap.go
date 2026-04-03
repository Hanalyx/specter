package reverse

import (
	"fmt"
	"strings"

	"github.com/Hanalyx/specter/internal/schema"
)

// DetectGaps finds constraints that have no corresponding test assertion.
// For each unmatched constraint, a gap AC is generated with Gap: true.
func DetectGaps(constraints []ExtractedConstraint, assertions []ExtractedAssertion) []schema.AcceptanceCriterion {
	var gaps []schema.AcceptanceCriterion

	for _, c := range constraints {
		if constraintHasMatch(c, assertions) {
			continue
		}

		desc := fmt.Sprintf("UNTESTED: %s", buildGapDescription(c))
		gaps = append(gaps, schema.AcceptanceCriterion{
			// ID is assigned by the caller
			Description: desc,
			Gap:         true,
			Priority:    "high",
		})
	}

	return gaps
}

// constraintHasMatch checks if any assertion covers this constraint.
func constraintHasMatch(c ExtractedConstraint, assertions []ExtractedAssertion) bool {
	if c.Field == "" {
		return false
	}

	fieldLower := strings.ToLower(c.Field)
	ruleLower := strings.ToLower(c.Rule)

	// Build rule-related keywords
	ruleKeywords := ruleToKeywords(ruleLower)

	for _, a := range assertions {
		descLower := strings.ToLower(a.Description)
		testLower := strings.ToLower(a.TestName)
		combined := descLower + " " + testLower

		// Check if assertion mentions the field
		if !strings.Contains(combined, fieldLower) {
			// Also check inputs/expected keys
			if !keysContain(a.Inputs, fieldLower) && !keysContain(a.Expected, fieldLower) {
				continue
			}
		}

		// Field matches — check if any rule keyword matches
		for _, kw := range ruleKeywords {
			if strings.Contains(combined, kw) {
				return true
			}
		}

		// If field matches and it's an error test, likely covers validation
		if a.IsError {
			return true
		}
	}

	return false
}

func ruleToKeywords(rule string) []string {
	base := []string{rule}
	switch rule {
	case "min":
		return append(base, "minimum", "too short", "too small", "at least", "less than")
	case "max":
		return append(base, "maximum", "too long", "too large", "exceeds", "more than")
	case "required":
		return append(base, "missing", "empty", "blank", "not provided", "null")
	case "format":
		return append(base, "invalid", "malformed", "valid")
	case "email":
		return append(base, "email", "invalid email", "malformed email")
	case "enum":
		return append(base, "allowed", "valid", "invalid", "not one of")
	case "pattern":
		return append(base, "pattern", "format", "match", "invalid")
	case "type":
		return append(base, "type", "invalid type", "wrong type")
	default:
		return base
	}
}

func keysContain(m map[string]interface{}, field string) bool {
	for k := range m {
		if strings.Contains(strings.ToLower(k), field) {
			return true
		}
	}
	return false
}

func buildGapDescription(c ExtractedConstraint) string {
	field := c.Field
	if field == "" {
		field = "value"
	}
	switch c.Rule {
	case "required":
		return fmt.Sprintf("Validate that %s is required", field)
	case "min":
		return fmt.Sprintf("Validate that %s enforces minimum of %v", field, c.Value)
	case "max":
		return fmt.Sprintf("Validate that %s enforces maximum of %v", field, c.Value)
	case "format":
		return fmt.Sprintf("Validate that %s is a valid %v", field, c.Value)
	case "email":
		return fmt.Sprintf("Validate that %s is a valid email address", field)
	case "enum":
		return fmt.Sprintf("Validate that %s is one of the allowed values", field)
	default:
		return fmt.Sprintf("Validate that %s satisfies %s constraint", field, c.Rule)
	}
}

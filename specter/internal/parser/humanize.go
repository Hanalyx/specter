// humanize.go -- C-09: human-readable parse error messages.
//
// Translates raw JSON Schema validation messages into plain English
// that names the invalid field and describes the fix.
//
// @spec spec-parse
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	missingPropRe    = regexp.MustCompile(`missing propert(?:y|ies) '([^']+)'`)
	additionalPropRe = regexp.MustCompile(`additional properties? '([^']+)'`)
	enumValuesRe     = regexp.MustCompile(`value should be one of (.+)`)
)

// humanizeError converts a raw JSON Schema error into a plain-English message.
//
// Parameters:
//   - errType: the error classification (required, additionalProperties, pattern, enum, type)
//   - path:    dot-notation path to the failing field (e.g. "spec.constraints[0].id")
//   - rawMsg:  the original error string from the JSON Schema validator
//
// C-09: must name the field and describe the fix; must NOT expose raw JSON Schema paths.
func humanizeError(errType, path, rawMsg string) string {
	switch errType {
	case "required":
		return humanizeRequired(path, rawMsg)
	case "additionalProperties":
		return humanizeAdditional(path, rawMsg)
	case "pattern":
		return humanizePattern(path, rawMsg)
	case "enum":
		return humanizeEnum(path, rawMsg)
	case "type":
		return humanizeType(path, rawMsg)
	default:
		return rawMsg
	}
}

func humanizeRequired(path, rawMsg string) string {
	prop := ""
	if m := missingPropRe.FindStringSubmatch(rawMsg); len(m) > 1 {
		prop = m[1]
	}
	switch prop {
	case "id":
		return "Missing required field 'id'. Add a kebab-case identifier, e.g. id: payment-create-intent"
	case "version":
		return "Missing required field 'version'. Add a semantic version, e.g. version: \"1.0.0\""
	case "status":
		return "Missing required field 'status'. Use one of: draft, approved, deprecated"
	case "tier":
		return "Missing required field 'tier'. Use an integer 1 (critical), 2 (standard), or 3 (informational)"
	case "context":
		return "Missing required field 'context'. Add a context block with system and feature fields"
	case "objective":
		return "Missing required field 'objective'. Add an objective block with a summary"
	case "constraints":
		return "Missing required field 'constraints'. Add at least one constraint with id (C-01 format) and description"
	case "acceptance_criteria":
		return "Missing required field 'acceptance_criteria'. Add at least one AC with id (AC-01 format) and description"
	case "description":
		// could be a constraint or AC description
		if strings.Contains(path, "constraints") {
			return "Constraint is missing required field 'description'. Each constraint must have a description"
		}
		if strings.Contains(path, "acceptance_criteria") {
			return "Acceptance criterion is missing required field 'description'. Each AC must have a description"
		}
		return "Missing required field 'description'"
	case "summary":
		return "Missing required field 'summary' in objective. Add a one-line summary of what this spec achieves"
	case "system":
		return "Missing required field 'system' in context. Specify the system this spec belongs to"
	case "feature":
		return "Missing required field 'feature' in context. Describe the feature this spec covers"
	default:
		if prop != "" {
			return fmt.Sprintf("Missing required field '%s'. Add this field to fix the error", prop)
		}
		return "A required field is missing. Check that all required fields are present"
	}
}

func humanizeAdditional(path, rawMsg string) string {
	field := ""
	if m := additionalPropRe.FindStringSubmatch(rawMsg); len(m) > 1 {
		field = m[1]
	}
	// Special case: context was extensible pre-v0.7.0. Tell users explicitly
	// what to do instead of just "unknown field".
	if strings.Contains(path, "context") && field != "" {
		return fmt.Sprintf(
			"Unknown field 'context.%s'. Context was tightened in v0.7.0 — unknown keys are no longer silently dropped. "+
				"Options: (1) move the value into context.description as prose, (2) use the spec-level tags array for categorical data, "+
				"or (3) propose a new schema field at https://github.com/Hanalyx/specter/issues.",
			field,
		)
	}
	if field != "" {
		return fmt.Sprintf("Unknown field '%s'. Remove it or check for a typo in the field name.", field)
	}
	return "Unknown field found. Remove any fields not defined in the spec schema."
}

func humanizePattern(path, _ string) string {
	// Determine which pattern failed based on the path context
	switch {
	case strings.Contains(path, "constraints") && strings.HasSuffix(path, ".id"):
		return "Constraint ID must match the C-NN pattern (e.g. C-01, C-02). Use uppercase C followed by a dash and two digits"
	case strings.Contains(path, "acceptance_criteria") && strings.HasSuffix(path, ".id"):
		return "Acceptance criterion ID must match the AC-NN pattern (e.g. AC-01, AC-02). Use uppercase AC followed by a dash and two digits"
	case strings.HasSuffix(path, ".version") || path == "spec.version":
		return "Version must be a semantic version string (e.g. \"1.0.0\"). The 'v' prefix is not allowed"
	case strings.HasSuffix(path, ".id") || path == "spec.id":
		return "Spec ID must be kebab-case (e.g. payment-create-intent). Use lowercase letters, numbers, and hyphens only"
	default:
		return "Field value does not match the required pattern. Check the format requirements for this field"
	}
}

func humanizeEnum(path, rawMsg string) string {
	// Extract the allowed values from the raw message if available
	allowed := ""
	if m := enumValuesRe.FindStringSubmatch(rawMsg); len(m) > 1 {
		allowed = m[1]
	}

	switch {
	case strings.HasSuffix(path, ".status"):
		return "Invalid status value. Use one of: draft, review, approved, deprecated, removed"
	case strings.HasSuffix(path, ".tier"):
		return "Invalid tier value. Use an integer 1 (critical), 2 (standard), or 3 (informational) — no quotes"
	case strings.Contains(path, "changelog") && strings.HasSuffix(path, ".type"):
		// Could be either changelog_entry.type or changelog_entry.changes[].type
		if strings.Contains(path, "changes") {
			return "Invalid change type. Use one of: addition, removal, modification, deprecation"
		}
		return "Invalid changelog entry type. Use one of: initial, major, minor, patch"
	case strings.Contains(path, "constraints") && strings.HasSuffix(path, ".type"):
		return "Invalid constraint type. Use one of: technical, security, performance, accessibility, business"
	case strings.Contains(path, "constraints") && strings.HasSuffix(path, ".enforcement"):
		return "Invalid enforcement level. Use one of: error, warning, info"
	case strings.Contains(path, "constraints") && strings.HasSuffix(path, "validation.rule"):
		return "Invalid validation rule. Use one of: type, min, max, pattern, enum, required, format, custom"
	case strings.Contains(path, "acceptance_criteria") && strings.HasSuffix(path, ".priority"):
		return "Invalid priority value. Use one of: critical, high, medium, low"
	case strings.Contains(path, "depends_on") && strings.HasSuffix(path, ".relationship"):
		return "Invalid relationship type. Use one of: requires, extends, conflicts_with"
	default:
		if allowed != "" {
			return fmt.Sprintf("Invalid value. Must be one of: %s", allowed)
		}
		return "Invalid value. Check the allowed values for this field"
	}
}

func humanizeType(path, rawMsg string) string {
	switch {
	case strings.HasSuffix(path, ".tier"):
		return "Tier must be an integer: 1 (critical), 2 (standard), or 3 (informational)"
	default:
		return fmt.Sprintf("Wrong type for field '%s'. %s", path, rawMsg)
	}
}

// Package parser implements spec-parse: YAML-to-SpecAST parser.
//
// Pure functions. No CLI deps, no I/O beyond what's passed in.
//
// @spec spec-parse
package parser

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Hanalyx/specter/internal/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed spec-schema.json
var schemaFS embed.FS

// SchemaBytes returns the raw embedded spec-schema.json. Used by packages that
// need to walk the schema (e.g., internal/explain) without re-embedding the file.
func SchemaBytes() ([]byte, error) {
	return schemaFS.ReadFile("spec-schema.json")
}

// ParseError represents a validation error with path and optional line info.
type ParseError struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

func (e ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("[%s] %s:%d: %s", e.Type, e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Path, e.Message)
}

// ParseResult holds the outcome of parsing a spec.
type ParseResult struct {
	OK     bool            `json:"ok"`
	Value  *schema.SpecAST `json:"value,omitempty"`
	Errors []ParseError    `json:"errors,omitempty"`
}

var compiledSchema *jsonschema.Schema

func init() {
	data, err := schemaFS.ReadFile("spec-schema.json")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded schema: %v", err))
	}

	var schemaDoc interface{}
	if err := json.Unmarshal(data, &schemaDoc); err != nil {
		panic(fmt.Sprintf("failed to parse schema JSON: %v", err))
	}

	c := jsonschema.NewCompiler()
	// Turn on format assertions so `format: date`, `format: email`, etc. are
	// validated, not just annotated. Without this, `approval_date: "not-a-date"`
	// would pass schema validation since draft 2020-12 treats format as
	// annotation-only by default.
	c.AssertFormat()
	if err := c.AddResource("spec-schema.json", schemaDoc); err != nil {
		panic(fmt.Sprintf("failed to add schema resource: %v", err))
	}

	compiled, err := c.Compile("spec-schema.json")
	if err != nil {
		panic(fmt.Sprintf("failed to compile schema: %v", err))
	}
	compiledSchema = compiled
}

// ParseSpec parses YAML content into a validated SpecAST.
//
// C-01: Validates against canonical JSON Schema.
// C-02: Reports errors with paths.
// C-03: Rejects unknown fields (additionalProperties: false in schema).
// C-04: Returns typed SpecAST on success.
// C-05: Handles YAML syntax errors gracefully.
// C-06: YAML anchors resolved by yaml.v3.
// C-07: Collects all validation errors.
// C-08: Pure function.
// maxSpecBytes caps input size before YAML parsing to prevent anchor-expansion
// DoS ("billion laughs") when Specter runs in CI on externally-sourced spec files.
const maxSpecBytes = 1 << 20 // 1 MB

func ParseSpec(yamlContent string) ParseResult {
	if len(yamlContent) > maxSpecBytes {
		return ParseResult{Errors: []ParseError{{
			Path:    "",
			Type:    "file_too_large",
			Message: fmt.Sprintf("spec file exceeds %d byte limit (%d bytes)", maxSpecBytes, len(yamlContent)),
		}}}
	}

	// Step 1: Parse YAML (C-05, C-06)
	var raw interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		pe := ParseError{
			Path:    "",
			Type:    "yaml_syntax",
			Message: err.Error(),
		}
		// Extract line info from yaml error
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			pe.Message = yamlErr.Error()
		}
		// Try to get line number from the error string
		line := extractLineFromYAMLError(err.Error())
		if line > 0 {
			pe.Line = line
		}
		return ParseResult{OK: false, Errors: []ParseError{pe}}
	}

	// Convert YAML map keys to string (yaml.v3 produces map[string]interface{})
	normalized := normalizeYAML(raw)

	// Step 2: Validate against JSON Schema (C-01, C-03, C-07)
	err := compiledSchema.Validate(normalized)
	if err != nil {
		validationErr, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return ParseResult{OK: false, Errors: []ParseError{
				{Path: "", Type: "validation", Message: err.Error()},
			}}
		}

		errors := flattenValidationErrors(validationErr)
		if len(errors) > 0 {
			return ParseResult{OK: false, Errors: errors}
		}
	}

	// Step 3: Unmarshal into typed struct (C-04)
	var doc schema.SpecDocument
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		return ParseResult{OK: false, Errors: []ParseError{
			{Path: "", Type: "unmarshal", Message: err.Error()},
		}}
	}

	// Step 4: Structural cross-reference validation.
	// JSON Schema can enforce that references_constraints entries MATCH the
	// ^C-\d{2,}$ pattern, but cannot assert that the referenced constraint
	// actually exists in this spec. That's a parse-level structural error —
	// same class as "id doesn't match its pattern" — so we catch it here
	// rather than waiting for specter check downstream.
	if xrefErrors := validateReferencedConstraints(&doc.Spec); len(xrefErrors) > 0 {
		return ParseResult{OK: false, Errors: xrefErrors}
	}

	return ParseResult{OK: true, Value: &doc.Spec}
}

// validateReferencedConstraints asserts every AC's references_constraints
// entries point at a constraint declared in the same spec. Returns a ParseError
// per dangling reference.
func validateReferencedConstraints(spec *schema.SpecAST) []ParseError {
	known := make(map[string]bool, len(spec.Constraints))
	for _, c := range spec.Constraints {
		known[c.ID] = true
	}

	var errs []ParseError
	for _, ac := range spec.AcceptanceCriteria {
		for _, ref := range ac.ReferencesConstraints {
			if !known[ref] {
				errs = append(errs, ParseError{
					Type:    "dangling_reference",
					Path:    fmt.Sprintf("spec.acceptance_criteria[%s].references_constraints", ac.ID),
					Message: fmt.Sprintf("references constraint %q which is not declared in this spec (declared constraints: %v)", ref, sortedKeys(known)),
				})
			}
		}
	}
	return errs
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple sort — small lists, no need for sort package import overhead
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// flattenValidationErrors converts nested jsonschema validation errors into flat ParseErrors.
func flattenValidationErrors(ve *jsonschema.ValidationError) []ParseError {
	var errors []ParseError

	if len(ve.Causes) == 0 {
		path := locationToPath(ve.InstanceLocation)
		errType := extractErrorType(ve)
		errors = append(errors, ParseError{
			Path:    path,
			Type:    errType,
			Message: humanizeError(errType, path, ve.Error()), // C-09: human-readable messages
		})
	}

	for _, cause := range ve.Causes {
		errors = append(errors, flattenValidationErrors(cause)...)
	}

	return errors
}

// locationToPath converts a jsonschema v6 InstanceLocation ([]string) to dot notation.
func locationToPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var result []string
	for _, part := range parts {
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			if len(result) > 0 {
				result[len(result)-1] = result[len(result)-1] + "[" + part + "]"
			}
		} else {
			result = append(result, part)
		}
	}
	return strings.Join(result, ".")
}

// extractErrorType determines the type of validation error.
func extractErrorType(ve *jsonschema.ValidationError) string {
	msg := ve.Error()
	switch {
	case strings.Contains(msg, "missing property"):
		return "required"
	case strings.Contains(msg, "additional properties"):
		return "additionalProperties"
	case strings.Contains(msg, "pattern"):
		return "pattern"
	case strings.Contains(msg, "enum"):
		return "enum"
	case strings.Contains(msg, "expected"):
		return "type"
	case strings.Contains(msg, "minimum") || strings.Contains(msg, "minItems"):
		return "minItems"
	default:
		return "validation"
	}
}

// normalizeYAML ensures all map keys are strings (required by JSON Schema validation).
func normalizeYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = normalizeYAML(v)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = normalizeYAML(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = normalizeYAML(v)
		}
		return result
	default:
		return v
	}
}

func extractLineFromYAMLError(msg string) int {
	// yaml.v3 errors contain "line N:" patterns
	var line int
	_, _ = fmt.Sscanf(msg, "yaml: line %d:", &line)
	return line
}

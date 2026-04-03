// Package reverse implements spec-reverse: the reverse compiler.
//
// Extracts draft .spec.yaml files from existing source code by analyzing
// validation schemas, test assertions, route definitions, and import graphs.
// Uses a plugin adapter architecture for language-specific extraction.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-reverse
package reverse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/schema"
	"gopkg.in/yaml.v3"
)

// --- Adapter Interface ---

// Adapter extracts structured data from source code for a specific language.
// All methods are pure functions: (path, content) -> structured data.
type Adapter interface {
	Name() string
	Detect(path, content string) bool
	IsTestFile(path string) bool
	ExtractRoutes(path, content string) []ExtractedRoute
	ExtractConstraints(path, content string) []ExtractedConstraint
	ExtractAssertions(path, content string) []ExtractedAssertion
	ExtractImports(path, content string) []ExtractedImport
	InferSystemName(files []SourceFile) string
}

// --- Intermediate Types (Adapter Output) ---

// SourceFile represents a source code file's content and metadata.
type SourceFile struct {
	Path    string
	Content string
	IsTest  bool
}

// ExtractedRoute represents a discovered HTTP route/handler.
type ExtractedRoute struct {
	Method  string
	Path    string
	Handler string
	File    string
	Line    int
}

// ExtractedConstraint is a raw constraint found in source code.
type ExtractedConstraint struct {
	Field       string
	Rule        string
	Value       interface{}
	Description string
	SourceFile  string
	Line        int
}

// ExtractedAssertion is a test assertion found in test files.
type ExtractedAssertion struct {
	TestName    string
	Description string
	Inputs      map[string]interface{}
	Expected    map[string]interface{}
	IsError     bool
	ErrorDesc   string
	SourceFile  string
	Line        int
}

// ExtractedImport represents an import/dependency found in source.
type ExtractedImport struct {
	Module string
	File   string
}

// --- Core Input/Output Types ---

// ReverseInput provides source files and configuration.
type ReverseInput struct {
	Files       []SourceFile
	AdapterName string // "" = auto-detect
	GroupBy     string // "file" (default) or "directory"
	Date        string // ISO 8601 extraction date
}

// ReverseResult is the output of the reverse compiler.
type ReverseResult struct {
	Specs       []GeneratedSpec
	Diagnostics []ReverseDiagnostic
	Summary     ReverseSummary
}

// GeneratedSpec wraps a SpecAST with generation metadata.
type GeneratedSpec struct {
	Spec     schema.SpecAST
	YAML     string
	FileName string
	Warnings []string
}

// ReverseDiagnostic reports issues during reverse compilation.
type ReverseDiagnostic struct {
	Kind     string // "no_adapter", "no_constraints", "no_tests", "validation_failed"
	Severity string // "error", "warning", "info"
	Message  string
	File     string
}

// ReverseSummary is the summary statistics.
type ReverseSummary struct {
	FilesProcessed   int
	SpecsGenerated   int
	ConstraintsFound int
	AssertionsFound  int
	GapsDetected     int
}

// --- Core Engine ---

// Reverse extracts draft specs from source files using the given adapter configuration.
func Reverse(input ReverseInput, adapters []Adapter) *ReverseResult {
	result := &ReverseResult{}

	// Select adapter
	adapter := selectAdapter(input, adapters, result)
	if adapter == nil {
		return result
	}

	// Classify files
	var sourceFiles, testFiles []SourceFile
	for i := range input.Files {
		f := &input.Files[i]
		if !adapter.Detect(f.Path, f.Content) {
			continue
		}
		if adapter.IsTestFile(f.Path) {
			f.IsTest = true
			testFiles = append(testFiles, *f)
		} else {
			sourceFiles = append(sourceFiles, *f)
		}
	}

	result.Summary.FilesProcessed = len(sourceFiles) + len(testFiles)

	if len(sourceFiles) == 0 {
		result.Diagnostics = append(result.Diagnostics, ReverseDiagnostic{
			Kind:     "no_source_files",
			Severity: "warning",
			Message:  fmt.Sprintf("no source files detected for adapter %q", adapter.Name()),
		})
		return result
	}

	// Extract from all files
	var allConstraints []ExtractedConstraint
	var allAssertions []ExtractedAssertion
	var allRoutes []ExtractedRoute
	var allImports []ExtractedImport

	for _, f := range sourceFiles {
		allConstraints = append(allConstraints, adapter.ExtractConstraints(f.Path, f.Content)...)
		allRoutes = append(allRoutes, adapter.ExtractRoutes(f.Path, f.Content)...)
		allImports = append(allImports, adapter.ExtractImports(f.Path, f.Content)...)
	}
	for _, f := range testFiles {
		allAssertions = append(allAssertions, adapter.ExtractAssertions(f.Path, f.Content)...)
	}

	result.Summary.ConstraintsFound = len(allConstraints)
	result.Summary.AssertionsFound = len(allAssertions)

	// Group files
	groups := groupFiles(input.GroupBy, sourceFiles, testFiles)

	// Infer system name
	systemName := adapter.InferSystemName(input.Files)
	if systemName == "" {
		systemName = "unknown-system"
	}

	// Assemble specs per group
	for groupKey, group := range groups {
		spec := assembleSpec(groupKey, group, adapter, systemName, input.Date, result)
		if spec == nil {
			continue
		}

		// Marshal to YAML
		doc := schema.SpecDocument{Spec: *spec}
		yamlBytes, err := yaml.Marshal(doc)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, ReverseDiagnostic{
				Kind:     "marshal_failed",
				Severity: "error",
				Message:  fmt.Sprintf("failed to marshal spec %s: %v", spec.ID, err),
			})
			continue
		}
		yamlStr := string(yamlBytes)

		// Validate via parser.ParseSpec()
		parseResult := parser.ParseSpec(yamlStr)
		if !parseResult.OK {
			var errMsgs []string
			for _, e := range parseResult.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("[%s] %s: %s", e.Type, e.Path, e.Message))
			}
			result.Diagnostics = append(result.Diagnostics, ReverseDiagnostic{
				Kind:     "validation_failed",
				Severity: "error",
				Message:  fmt.Sprintf("generated spec %s failed validation: %s", spec.ID, strings.Join(errMsgs, "; ")),
			})
			continue
		}

		fileName := spec.ID + ".spec.yaml"
		gs := GeneratedSpec{
			Spec:     *spec,
			YAML:     yamlStr,
			FileName: fileName,
		}

		if len(spec.AcceptanceCriteria) > 0 {
			gapCount := 0
			for _, ac := range spec.AcceptanceCriteria {
				if ac.Gap {
					gapCount++
				}
			}
			if gapCount > 0 {
				gs.Warnings = append(gs.Warnings, fmt.Sprintf("%d gap(s) detected — constraints without test coverage", gapCount))
				result.Summary.GapsDetected += gapCount
			}
		}

		result.Specs = append(result.Specs, gs)
		result.Summary.SpecsGenerated++
	}

	return result
}

// --- Internal helpers ---

func selectAdapter(input ReverseInput, adapters []Adapter, result *ReverseResult) Adapter {
	if input.AdapterName != "" {
		for _, a := range adapters {
			if a.Name() == input.AdapterName {
				return a
			}
		}
		result.Diagnostics = append(result.Diagnostics, ReverseDiagnostic{
			Kind:     "no_adapter",
			Severity: "error",
			Message:  fmt.Sprintf("adapter %q not found", input.AdapterName),
		})
		return nil
	}
	return DetectAdapter(input.Files, adapters)
}

// fileGroup holds source and test files that will be assembled into one spec.
type fileGroup struct {
	SourceFiles []SourceFile
	TestFiles   []SourceFile
}

func groupFiles(groupBy string, sourceFiles, testFiles []SourceFile) map[string]*fileGroup {
	groups := make(map[string]*fileGroup)

	keyFn := fileGroupKey
	if groupBy == "directory" {
		keyFn = dirGroupKey
	}

	for _, f := range sourceFiles {
		key := keyFn(f.Path)
		if groups[key] == nil {
			groups[key] = &fileGroup{}
		}
		groups[key].SourceFiles = append(groups[key].SourceFiles, f)
	}
	for _, f := range testFiles {
		key := keyFn(f.Path)
		if groups[key] == nil {
			groups[key] = &fileGroup{}
		}
		groups[key].TestFiles = append(groups[key].TestFiles, f)
	}

	return groups
}

func fileGroupKey(path string) string {
	return path
}

func dirGroupKey(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

func assembleSpec(groupKey string, group *fileGroup, adapter Adapter, systemName, date string, result *ReverseResult) *schema.SpecAST {
	// Extract from this group's files
	var constraints []ExtractedConstraint
	var assertions []ExtractedAssertion
	var routes []ExtractedRoute
	var imports []ExtractedImport

	for _, f := range group.SourceFiles {
		constraints = append(constraints, adapter.ExtractConstraints(f.Path, f.Content)...)
		routes = append(routes, adapter.ExtractRoutes(f.Path, f.Content)...)
		imports = append(imports, adapter.ExtractImports(f.Path, f.Content)...)
	}
	for _, f := range group.TestFiles {
		assertions = append(assertions, adapter.ExtractAssertions(f.Path, f.Content)...)
	}

	if len(constraints) == 0 && len(assertions) == 0 && len(routes) == 0 {
		result.Diagnostics = append(result.Diagnostics, ReverseDiagnostic{
			Kind:     "no_extractable_content",
			Severity: "info",
			Message:  fmt.Sprintf("no constraints, assertions, or routes found in %s", groupKey),
			File:     groupKey,
		})
		return nil
	}

	specID := GenerateSpecID(groupKey)

	// Build constraints
	specConstraints := make([]schema.Constraint, len(constraints))
	for i, c := range constraints {
		desc := c.Description
		if desc == "" {
			desc = buildConstraintDescription(c)
		}
		specConstraints[i] = schema.Constraint{
			ID:          fmt.Sprintf("C-%02d", i+1),
			Description: desc,
			Type:        "technical",
			Enforcement: "error",
		}
		if c.Field != "" && c.Rule != "" {
			specConstraints[i].Validation = &schema.ConstraintValidation{
				Field: c.Field,
				Rule:  c.Rule,
				Value: c.Value,
			}
		}
	}

	// Build ACs from assertions
	specACs := make([]schema.AcceptanceCriterion, 0, len(assertions))
	for i, a := range assertions {
		ac := schema.AcceptanceCriterion{
			ID:          fmt.Sprintf("AC-%02d", i+1),
			Description: a.Description,
			Priority:    "medium",
		}
		if len(a.Inputs) > 0 {
			ac.Inputs = a.Inputs
		}
		if len(a.Expected) > 0 {
			ac.ExpectedOutput = a.Expected
		}
		if a.IsError && a.ErrorDesc != "" {
			ac.ErrorCases = []schema.ErrorCase{{
				Condition:        a.ErrorDesc,
				ExpectedBehavior: a.Description,
			}}
		}
		specACs = append(specACs, ac)
	}

	// Detect gaps and append gap ACs
	gapACs := DetectGaps(constraints, assertions)
	nextACNum := len(specACs) + 1
	for i, gac := range gapACs {
		gac.ID = fmt.Sprintf("AC-%02d", nextACNum+i)
		// Reference the constraint that created this gap
		constraintIdx := findGapConstraintIndex(gac.Description, constraints)
		if constraintIdx >= 0 {
			gac.ReferencesConstraints = []string{fmt.Sprintf("C-%02d", constraintIdx+1)}
		}
		specACs = append(specACs, gac)
	}

	// Ensure at least one constraint and one AC (schema requires minItems: 1)
	if len(specConstraints) == 0 {
		specConstraints = []schema.Constraint{{
			ID:          "C-01",
			Description: "MUST implement the behavior defined in this module (auto-generated placeholder)",
			Type:        "technical",
			Enforcement: "error",
		}}
	}
	if len(specACs) == 0 {
		specACs = []schema.AcceptanceCriterion{{
			ID:          "AC-01",
			Description: "Module behavior matches implementation (auto-generated placeholder)",
			Gap:         true,
			Priority:    "high",
		}}
	}

	// Build objective summary
	summary := fmt.Sprintf("Reverse-compiled spec for %s.", specID)
	if len(routes) > 0 {
		routeDescs := make([]string, 0, len(routes))
		for _, r := range routes {
			routeDescs = append(routeDescs, fmt.Sprintf("%s %s", r.Method, r.Path))
		}
		summary = fmt.Sprintf("Reverse-compiled spec for %s. Routes: %s.", specID, strings.Join(routeDescs, ", "))
	}

	// Build dependencies from imports
	var deps []string
	seen := make(map[string]bool)
	for _, imp := range imports {
		if !seen[imp.Module] {
			deps = append(deps, imp.Module)
			seen[imp.Module] = true
		}
	}
	sort.Strings(deps)

	// Source file and test file paths for provenance
	var sourceFilePath string
	var testFilePaths []string
	if len(group.SourceFiles) > 0 {
		sourceFilePath = group.SourceFiles[0].Path
	}
	for _, f := range group.TestFiles {
		testFilePaths = append(testFilePaths, f.Path)
	}

	spec := &schema.SpecAST{
		ID:      specID,
		Version: "0.1.0",
		Status:  "draft",
		Tier:    3,
		Context: schema.SpecContext{
			System:       systemName,
			Feature:      specID,
			Dependencies: deps,
		},
		Objective: schema.SpecObjective{
			Summary: summary,
		},
		Constraints:        specConstraints,
		AcceptanceCriteria: specACs,
		TrustLevel:         "auto_with_review",
		GeneratedFrom: &schema.GeneratedFrom{
			SourceFile:     sourceFilePath,
			TestFiles:      testFilePaths,
			ExtractionDate: date,
		},
	}

	return spec
}

func buildConstraintDescription(c ExtractedConstraint) string {
	field := c.Field
	if field == "" {
		field = "value"
	}
	switch c.Rule {
	case "required":
		return fmt.Sprintf("%s MUST be provided", field)
	case "min":
		return fmt.Sprintf("%s MUST have minimum value/length of %v", field, c.Value)
	case "max":
		return fmt.Sprintf("%s MUST have maximum value/length of %v", field, c.Value)
	case "format":
		return fmt.Sprintf("%s MUST be a valid %v", field, c.Value)
	case "enum":
		return fmt.Sprintf("%s MUST be one of the allowed values", field)
	case "pattern":
		return fmt.Sprintf("%s MUST match pattern %v", field, c.Value)
	case "type":
		return fmt.Sprintf("%s MUST be of type %v", field, c.Value)
	default:
		return fmt.Sprintf("%s MUST satisfy %s constraint", field, c.Rule)
	}
}

func findGapConstraintIndex(gapDesc string, constraints []ExtractedConstraint) int {
	for i, c := range constraints {
		if c.Field != "" && strings.Contains(gapDesc, c.Field) {
			return i
		}
	}
	return -1
}

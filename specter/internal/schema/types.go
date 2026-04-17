// Package schema defines the canonical types for SDD micro-specs.
//
// @spec spec-parse
package schema

// SpecDocument is the top-level YAML structure (the "spec:" wrapper).
type SpecDocument struct {
	Spec SpecAST `yaml:"spec" json:"spec"`
}

// SpecAST is the validated, typed representation of a .spec.yaml file.
type SpecAST struct {
	ID                 string                `yaml:"id" json:"id"`
	Version            string                `yaml:"version" json:"version"`
	Status             string                `yaml:"status" json:"status"`
	Tier               int                   `yaml:"tier" json:"tier"`
	CoverageThreshold  int                   `yaml:"coverage_threshold,omitempty" json:"coverage_threshold,omitempty"`
	Context            SpecContext           `yaml:"context" json:"context"`
	Objective          SpecObjective         `yaml:"objective" json:"objective"`
	Constraints        []Constraint          `yaml:"constraints" json:"constraints"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria" json:"acceptance_criteria"`
	DependsOn          []DependencyRef       `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Environment        *SpecEnvironment      `yaml:"environment,omitempty" json:"environment,omitempty"`
	Tags               []string              `yaml:"tags,omitempty" json:"tags,omitempty"`
	Changelog          []ChangelogEntry      `yaml:"changelog,omitempty" json:"changelog,omitempty"`
	GeneratedFrom      *GeneratedFrom        `yaml:"generated_from,omitempty" json:"generated_from,omitempty"`
}

type SpecContext struct {
	System           string   `yaml:"system" json:"system"`
	Feature          string   `yaml:"feature,omitempty" json:"feature,omitempty"`
	Description      string   `yaml:"description,omitempty" json:"description,omitempty"`
	Dependencies     []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	ExistingPatterns string   `yaml:"existing_patterns,omitempty" json:"existing_patterns,omitempty"`
	RelatedSpecs     []string `yaml:"related_specs,omitempty" json:"related_specs,omitempty"`
	Assumptions      []string `yaml:"assumptions,omitempty" json:"assumptions,omitempty"`
}

type SpecObjective struct {
	Summary string     `yaml:"summary" json:"summary"`
	Scope   *SpecScope `yaml:"scope,omitempty" json:"scope,omitempty"`
}

type SpecScope struct {
	Includes []string `yaml:"includes,omitempty" json:"includes,omitempty"`
	Excludes []string `yaml:"excludes,omitempty" json:"excludes,omitempty"`
}

type ConstraintValidation struct {
	Field string      `yaml:"field" json:"field"`
	Rule  string      `yaml:"rule" json:"rule"`
	Value interface{} `yaml:"value" json:"value"`
}

type Constraint struct {
	ID          string                `yaml:"id" json:"id"`
	Description string                `yaml:"description" json:"description"`
	Type        string                `yaml:"type,omitempty" json:"type,omitempty"`
	Enforcement string                `yaml:"enforcement,omitempty" json:"enforcement,omitempty"`
	Validation  *ConstraintValidation `yaml:"validation,omitempty" json:"validation,omitempty"`
}

type ErrorCase struct {
	Condition        string `yaml:"condition" json:"condition"`
	ExpectedBehavior string `yaml:"expected_behavior" json:"expected_behavior"`
}

type AcceptanceCriterion struct {
	ID                    string                 `yaml:"id" json:"id"`
	Description           string                 `yaml:"description" json:"description"`
	Inputs                map[string]interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	ExpectedOutput        map[string]interface{} `yaml:"expected_output,omitempty" json:"expected_output,omitempty"`
	ErrorCases            []ErrorCase            `yaml:"error_cases,omitempty" json:"error_cases,omitempty"`
	ReferencesConstraints []string               `yaml:"references_constraints,omitempty" json:"references_constraints,omitempty"`
	Gap                   bool                   `yaml:"gap,omitempty" json:"gap,omitempty"`
	Priority              string                 `yaml:"priority,omitempty" json:"priority,omitempty"`
}

type DependencyRef struct {
	SpecID       string `yaml:"spec_id" json:"spec_id"`
	VersionRange string `yaml:"version_range,omitempty" json:"version_range,omitempty"`
	Relationship string `yaml:"relationship,omitempty" json:"relationship,omitempty"`
}

type ChangelogChange struct {
	Type    string `yaml:"type" json:"type"`
	Section string `yaml:"section,omitempty" json:"section,omitempty"`
	Detail  string `yaml:"detail" json:"detail"`
}

type ChangelogEntry struct {
	Version     string            `yaml:"version" json:"version"`
	Date        string            `yaml:"date" json:"date"`
	Author      string            `yaml:"author,omitempty" json:"author,omitempty"`
	Type        string            `yaml:"type,omitempty" json:"type,omitempty"`
	Description string            `yaml:"description" json:"description"`
	Changes     []ChangelogChange `yaml:"changes,omitempty" json:"changes,omitempty"`
}

type SpecEnvironment struct {
	RequiredVars      []string `yaml:"required_vars,omitempty" json:"required_vars,omitempty"`
	DeploymentTargets []string `yaml:"deployment_targets,omitempty" json:"deployment_targets,omitempty"`
}

type GeneratedFrom struct {
	SourceFile     string   `yaml:"source_file,omitempty" json:"source_file,omitempty"`
	TestFiles      []string `yaml:"test_files,omitempty" json:"test_files,omitempty"`
	ExtractionDate string   `yaml:"extraction_date,omitempty" json:"extraction_date,omitempty"`
}

/**
 * TypeScript types derived from the canonical spec-schema.json.
 * These types define the SpecAST — the validated, typed output of spec-parse.
 *
 * @spec spec-parse
 */

export type SpecStatus = 'draft' | 'review' | 'approved' | 'deprecated' | 'removed';
export type SpecTier = 1 | 2 | 3;
export type TrustLevel = 'full_auto' | 'auto_with_review' | 'human_required';
export type ConstraintType =
  | 'technical'
  | 'security'
  | 'performance'
  | 'accessibility'
  | 'business';
export type EnforcementLevel = 'error' | 'warning' | 'info';
export type ConstraintRule =
  | 'type'
  | 'min'
  | 'max'
  | 'pattern'
  | 'enum'
  | 'required'
  | 'format'
  | 'custom';
export type DependencyRelationship = 'requires' | 'extends' | 'conflicts_with';
export type ChangelogType = 'initial' | 'major' | 'minor' | 'patch';
export type ChangeType = 'addition' | 'removal' | 'modification' | 'deprecation';
export type ACPriority = 'critical' | 'high' | 'medium' | 'low';

export interface ConstraintValidation {
  field: string;
  rule: ConstraintRule;
  value: string | number | boolean | string[];
}

export interface Constraint {
  id: string;
  description: string;
  type?: ConstraintType;
  enforcement?: EnforcementLevel;
  validation?: ConstraintValidation;
}

export interface ErrorCase {
  condition: string;
  expected_behavior: string;
}

export interface AcceptanceCriterion {
  id: string;
  description: string;
  inputs?: Record<string, unknown>;
  expected_output?: Record<string, unknown>;
  error_cases?: ErrorCase[];
  references_constraints?: string[];
  gap?: boolean;
  priority?: ACPriority;
}

export interface DependencyRef {
  spec_id: string;
  version_range?: string;
  relationship?: DependencyRelationship;
}

export interface ChangelogChange {
  type: ChangeType;
  section?: string;
  detail: string;
}

export interface ChangelogEntry {
  version: string;
  date: string;
  author?: string;
  type?: ChangelogType;
  description: string;
  changes?: ChangelogChange[];
}

export interface SpecContext {
  system: string;
  feature?: string;
  description?: string;
  dependencies?: string[];
  existing_patterns?: string;
  related_specs?: string[];
  assumptions?: string[];
  [key: string]: unknown;
}

export interface SpecObjective {
  summary: string;
  scope?: {
    includes?: string[];
    excludes?: string[];
  };
}

export interface SpecEnvironment {
  required_vars?: string[];
  deployment_targets?: string[];
}

export interface GeneratedFrom {
  source_file?: string;
  test_files?: string[];
  extraction_date?: string;
}

export interface SpecAST {
  id: string;
  version: string;
  status: SpecStatus;
  tier: SpecTier;
  context: SpecContext;
  objective: SpecObjective;
  constraints: Constraint[];
  acceptance_criteria: AcceptanceCriterion[];
  depends_on?: DependencyRef[];
  trust_level?: TrustLevel;
  environment?: SpecEnvironment;
  tags?: string[];
  changelog?: ChangelogEntry[];
  generated_from?: GeneratedFrom;
}

export interface SpecDocument {
  spec: SpecAST;
}

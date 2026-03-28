/**
 * Types for spec-check: the type checker.
 *
 * @spec spec-check
 */

export type CheckDiagnosticKind =
  | 'orphan_constraint'
  | 'structural_conflict'
  | 'breaking_change'
  | 'additive_change'
  | 'patch_change';

export type CheckSeverity = 'error' | 'warning' | 'info';

export interface CheckDiagnostic {
  kind: CheckDiagnosticKind;
  severity: CheckSeverity;
  message: string;
  spec_id: string;
  /** The constraint ID involved, if applicable */
  constraint_id?: string;
  /** For breaking change detection */
  change_type?: 'breaking' | 'additive' | 'patch';
  /** Details about what changed */
  details?: string;
}

/** Tier-based severity mapping for orphan constraints */
export const ORPHAN_SEVERITY_BY_TIER: Record<number, CheckSeverity> = {
  1: 'error',
  2: 'warning',
  3: 'info',
};

/** Tier-based coverage thresholds */
export const COVERAGE_THRESHOLD_BY_TIER: Record<number, number> = {
  1: 100,
  2: 80,
  3: 50,
};

export type ChangeClassification = 'breaking' | 'additive' | 'patch';

export interface VersionChange {
  classification: ChangeClassification;
  field: string;
  description: string;
}

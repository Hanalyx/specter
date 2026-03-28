/**
 * Types for spec-resolve: dependency graph builder.
 *
 * @spec spec-resolve
 */

import type { SpecAST } from '../schema/types.js';

export type DiagnosticSeverity = 'error' | 'warning' | 'info';

export type DiagnosticKind =
  | 'circular_dependency'
  | 'dangling_reference'
  | 'version_mismatch'
  | 'duplicate_id'
  | 'parse_error';

export interface Diagnostic {
  kind: DiagnosticKind;
  severity: DiagnosticSeverity;
  message: string;
  /** The spec ID that has the problem */
  spec_id?: string;
  /** For dangling references: the missing dependency ID */
  missing_dep?: string;
  /** For version mismatch: the required range */
  required_range?: string;
  /** For version mismatch: the actual version */
  actual_version?: string;
  /** For circular dependencies: the full cycle path */
  cycle_path?: string[];
  /** For duplicate IDs: the file paths that share the ID */
  files?: string[];
}

export interface SpecNode {
  /** The parsed spec */
  spec: SpecAST;
  /** The file path this spec was loaded from */
  file: string;
}

export interface SpecEdge {
  /** Source spec ID (the one that depends) */
  from: string;
  /** Target spec ID (the one being depended on) */
  to: string;
  /** Semver range requirement, if specified */
  version_range?: string;
  /** Relationship type */
  relationship: string;
}

export interface SpecGraph {
  /** All spec nodes indexed by spec ID */
  nodes: Map<string, SpecNode>;
  /** All dependency edges */
  edges: SpecEdge[];
  /** Topological ordering (dependencies before dependents). Empty if cycles exist. */
  topological_order: string[];
  /** All diagnostics found during resolution */
  diagnostics: Diagnostic[];
}

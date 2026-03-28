/**
 * spec-check: Orchestrates all check rules across the spec graph.
 *
 * Pure function. No CLI deps, no I/O.
 *
 * @spec spec-check
 */

import type { SpecGraph } from '../resolver/types.js';
import type { SpecAST } from '../schema/types.js';
import type { CheckDiagnostic } from './types.js';
import { checkOrphanConstraints } from './rules/orphan-constraints.js';
import { checkStructuralConflicts } from './rules/structural-conflicts.js';
import { classifyChanges, highestClassification } from './rules/breaking-changes.js';

export interface CheckOptions {
  /** Override tier for all specs (for testing) */
  tierOverride?: number;
  /** Previous versions of specs for breaking change detection */
  previousVersions?: Map<string, SpecAST>;
}

export interface CheckResult {
  diagnostics: CheckDiagnostic[];
  /** Summary counts by severity */
  summary: {
    errors: number;
    warnings: number;
    info: number;
  };
}

/**
 * Run all structural checks across the spec graph.
 *
 * C-06: Pure function from SpecGraph to diagnostics.
 * C-01: Detects all orphan constraints.
 * C-02: Respects tier-based severity.
 * C-03: Detects structural conflicts.
 * C-04: Classifies version changes.
 * C-05: Zero false positives for structural checks.
 */
export function checkSpecs(graph: SpecGraph, options: CheckOptions = {}): CheckResult {
  const diagnostics: CheckDiagnostic[] = [];

  // Rule 1: Orphan constraints (AC-01, AC-02, AC-06)
  for (const [, node] of graph.nodes) {
    const spec = options.tierOverride
      ? { ...node.spec, tier: options.tierOverride as 1 | 2 | 3 }
      : node.spec;
    diagnostics.push(...checkOrphanConstraints(spec));
  }

  // Rule 2: Structural conflicts across dependency edges (AC-03)
  diagnostics.push(...checkStructuralConflicts(graph));

  // Rule 3: Breaking change detection (AC-04, AC-05)
  if (options.previousVersions) {
    for (const [id, node] of graph.nodes) {
      const prev = options.previousVersions.get(id);
      if (!prev) continue;

      const changes = classifyChanges(prev, node.spec);
      const highest = highestClassification(changes);

      if (highest !== 'none') {
        for (const change of changes) {
          diagnostics.push({
            kind:
              change.classification === 'breaking'
                ? 'breaking_change'
                : change.classification === 'additive'
                  ? 'additive_change'
                  : 'patch_change',
            severity: change.classification === 'breaking' ? 'error' : 'info',
            message: `${node.spec.id}: ${change.description}`,
            spec_id: node.spec.id,
            change_type: change.classification,
            details: change.field,
          });
        }
      }
    }
  }

  return {
    diagnostics,
    summary: {
      errors: diagnostics.filter((d) => d.severity === 'error').length,
      warnings: diagnostics.filter((d) => d.severity === 'warning').length,
      info: diagnostics.filter((d) => d.severity === 'info').length,
    },
  };
}

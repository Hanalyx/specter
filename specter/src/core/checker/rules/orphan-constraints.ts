/**
 * Check rule: Orphan constraint detection.
 *
 * A constraint is orphaned if no acceptance criterion references it
 * via the references_constraints field.
 *
 * @spec spec-check
 * @ac AC-01, AC-02, AC-06
 */

import type { SpecAST } from '../../schema/types.js';
import { ORPHAN_SEVERITY_BY_TIER, type CheckDiagnostic } from '../types.js';

export function checkOrphanConstraints(spec: SpecAST): CheckDiagnostic[] {
  const diagnostics: CheckDiagnostic[] = [];

  // Collect all constraint IDs
  const constraintIds = new Set(spec.constraints.map((c) => c.id));

  // Collect all referenced constraint IDs from ACs
  const referencedIds = new Set<string>();
  for (const ac of spec.acceptance_criteria) {
    if (ac.references_constraints) {
      for (const ref of ac.references_constraints) {
        referencedIds.add(ref);
      }
    }
  }

  // Find orphans: constraints not referenced by any AC
  for (const constraintId of constraintIds) {
    if (!referencedIds.has(constraintId)) {
      // C-02: Tier-based severity
      const severity = ORPHAN_SEVERITY_BY_TIER[spec.tier] ?? 'warning';

      diagnostics.push({
        kind: 'orphan_constraint',
        severity,
        message: `Constraint ${constraintId} in "${spec.id}" is not referenced by any acceptance criterion`,
        spec_id: spec.id,
        constraint_id: constraintId,
      });
    }
  }

  return diagnostics;
}

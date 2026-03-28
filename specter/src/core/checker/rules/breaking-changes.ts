/**
 * Check rule: Breaking change detection between spec versions.
 *
 * Compares two versions of a spec and classifies changes as
 * breaking (MAJOR), additive (MINOR), or patch (PATCH).
 *
 * @spec spec-check
 * @ac AC-04, AC-05
 */

import type { SpecAST } from '../../schema/types.js';
import type { VersionChange } from '../types.js';

/**
 * Compare two spec versions and classify all changes.
 *
 * Breaking changes (MAJOR):
 * - Removing a constraint
 * - Removing an acceptance criterion
 * - Changing constraint enforcement from warning/info to error
 * - Tightening a constraint (making more restrictive)
 *
 * Additive changes (MINOR):
 * - Adding a new constraint
 * - Adding a new acceptance criterion
 * - Relaxing constraint enforcement
 *
 * Patch changes (PATCH):
 * - Changing descriptions only
 * - No structural changes
 */
export function classifyChanges(v1: SpecAST, v2: SpecAST): VersionChange[] {
  const changes: VersionChange[] = [];

  // Compare constraints
  const v1Constraints = new Map(v1.constraints.map((c) => [c.id, c]));
  const v2Constraints = new Map(v2.constraints.map((c) => [c.id, c]));

  // Removed constraints = breaking
  for (const [id] of v1Constraints) {
    if (!v2Constraints.has(id)) {
      changes.push({
        classification: 'breaking',
        field: `constraints.${id}`,
        description: `Constraint ${id} was removed`,
      });
    }
  }

  // Added constraints = additive
  for (const [id] of v2Constraints) {
    if (!v1Constraints.has(id)) {
      changes.push({
        classification: 'additive',
        field: `constraints.${id}`,
        description: `Constraint ${id} was added`,
      });
    }
  }

  // Modified constraints
  for (const [id, v1c] of v1Constraints) {
    const v2c = v2Constraints.get(id);
    if (!v2c) continue;

    // Enforcement tightened = breaking
    const enforcementRank: Record<string, number> = { info: 0, warning: 1, error: 2 };
    const v1Rank = enforcementRank[v1c.enforcement ?? 'error'] ?? 2;
    const v2Rank = enforcementRank[v2c.enforcement ?? 'error'] ?? 2;

    if (v2Rank > v1Rank) {
      changes.push({
        classification: 'breaking',
        field: `constraints.${id}.enforcement`,
        description: `Constraint ${id} enforcement tightened from ${v1c.enforcement ?? 'error'} to ${v2c.enforcement ?? 'error'}`,
      });
    } else if (v2Rank < v1Rank) {
      changes.push({
        classification: 'additive',
        field: `constraints.${id}.enforcement`,
        description: `Constraint ${id} enforcement relaxed from ${v1c.enforcement ?? 'error'} to ${v2c.enforcement ?? 'error'}`,
      });
    }

    // Description change only = patch
    if (
      v1c.description !== v2c.description &&
      v1c.enforcement === v2c.enforcement &&
      v1c.type === v2c.type
    ) {
      changes.push({
        classification: 'patch',
        field: `constraints.${id}.description`,
        description: `Constraint ${id} description updated`,
      });
    }
  }

  // Compare acceptance criteria
  const v1ACs = new Map(v1.acceptance_criteria.map((ac) => [ac.id, ac]));
  const v2ACs = new Map(v2.acceptance_criteria.map((ac) => [ac.id, ac]));

  // Removed ACs = breaking
  for (const [id] of v1ACs) {
    if (!v2ACs.has(id)) {
      changes.push({
        classification: 'breaking',
        field: `acceptance_criteria.${id}`,
        description: `Acceptance criterion ${id} was removed`,
      });
    }
  }

  // Added ACs = additive
  for (const [id] of v2ACs) {
    if (!v1ACs.has(id)) {
      changes.push({
        classification: 'additive',
        field: `acceptance_criteria.${id}`,
        description: `Acceptance criterion ${id} was added`,
      });
    }
  }

  // If no structural changes detected, check for description-only changes
  if (changes.length === 0 && v1.objective.summary !== v2.objective.summary) {
    changes.push({
      classification: 'patch',
      field: 'objective.summary',
      description: 'Objective summary updated',
    });
  }

  return changes;
}

/**
 * Get the highest classification from a set of changes.
 * breaking > additive > patch
 */
export function highestClassification(
  changes: VersionChange[],
): 'breaking' | 'additive' | 'patch' | 'none' {
  if (changes.length === 0) return 'none';
  if (changes.some((c) => c.classification === 'breaking')) return 'breaking';
  if (changes.some((c) => c.classification === 'additive')) return 'additive';
  return 'patch';
}

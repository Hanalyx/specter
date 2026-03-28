/**
 * Check rule: Structural conflict detection.
 *
 * Detects when a downstream spec's ACs contradict an upstream spec's constraints.
 * MVP: checks for MUST/required constraints in upstream that are handled as absent
 * in downstream ACs (via keyword matching on "absent", "missing", "optional", "not provided").
 *
 * @spec spec-check
 * @ac AC-03
 */

import type { SpecGraph } from '../../resolver/types.js';
import type { CheckDiagnostic } from '../types.js';

const ABSENCE_KEYWORDS = [
  'absent',
  'missing',
  'not provided',
  'not present',
  'is empty',
  'is null',
  'is undefined',
  'without',
  'no ',
  'lacks',
];

const REQUIRED_KEYWORDS = ['MUST', 'required', 'MUST be present', 'MUST exist', 'mandatory'];

export function checkStructuralConflicts(graph: SpecGraph): CheckDiagnostic[] {
  const diagnostics: CheckDiagnostic[] = [];

  for (const edge of graph.edges) {
    if (edge.relationship !== 'requires') continue;

    const upstream = graph.nodes.get(edge.to);
    const downstream = graph.nodes.get(edge.from);
    if (!upstream || !downstream) continue;

    // Find upstream constraints that assert something is MUST/required
    for (const constraint of upstream.spec.constraints) {
      const desc = constraint.description;
      const isRequired = REQUIRED_KEYWORDS.some((kw) => desc.includes(kw));
      if (!isRequired) continue;

      // Check downstream ACs for handling the required thing as absent
      for (const ac of downstream.spec.acceptance_criteria) {
        const acDesc = ac.description.toLowerCase();
        const constraintSubject = extractSubject(desc);

        if (constraintSubject && ABSENCE_KEYWORDS.some((kw) => acDesc.includes(kw.toLowerCase()))) {
          // Check if the AC description references the same subject
          if (constraintSubject && acDesc.includes(constraintSubject.toLowerCase())) {
            diagnostics.push({
              kind: 'structural_conflict',
              severity: 'error',
              message:
                `Structural conflict: "${upstream.spec.id}" constraint ${constraint.id} requires ` +
                `"${constraintSubject}" but "${downstream.spec.id}" ${ac.id} handles it as absent`,
              spec_id: downstream.spec.id,
              constraint_id: constraint.id,
              details: `Upstream: ${desc} | Downstream AC: ${ac.description}`,
            });
          }
        }
      }
    }
  }

  return diagnostics;
}

/**
 * Extract the subject of a constraint (the thing that MUST exist).
 * e.g., "email MUST be required" -> "email"
 *       "MUST have a valid token" -> "token"
 */
function extractSubject(description: string): string | null {
  // Pattern: "<subject> MUST"
  const beforeMust = description.match(/^(\w[\w\s]*?)\s+MUST/i);
  if (beforeMust) return beforeMust[1].trim();

  // Pattern: "MUST have/include/contain <subject>"
  const afterMust = description.match(/MUST\s+(?:have|include|contain|provide)\s+(?:a\s+)?(\w+)/i);
  if (afterMust) return afterMust[1].trim();

  return null;
}

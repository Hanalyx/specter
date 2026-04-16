// @spec spec-vscode

import type { SpecIndex, HoverResult, DefinitionTarget } from './types';

// ---------------------------------------------------------------------------
// AC-06: Constraint hover card
// ---------------------------------------------------------------------------

export interface ConstraintHoverContext {
  coveredACIDs: string[];
}

/**
 * Builds a hover card for a constraint ID in a .spec.yaml file, showing:
 *   • Constraint description
 *   • Which ACs reference it, with their coverage status
 *   • A prominent warning if no AC references the constraint
 */
export function buildConstraintHover(
  index: SpecIndex,
  specID: string,
  constraintID: string,
  ctx: ConstraintHoverContext,
): HoverResult {
  const spec = index.specs[specID];
  if (!spec) return { contents: '' };

  const constraint = spec.constraints?.find(c => c.id === constraintID);
  if (!constraint) return { contents: '' };

  const lines: string[] = [
    `**${constraintID}** — ${constraint.description}`,
    '',
  ];

  const referencingACs = spec.constraintReferences?.[constraintID] ?? [];

  if (referencingACs.length === 0) {
    lines.push('> ⚠ **No AC references this constraint.**');
  } else {
    lines.push('Referenced by:');
    for (const acID of referencingACs) {
      const isCovered = ctx.coveredACIDs.includes(acID);
      const status = isCovered ? 'covered' : 'uncovered';
      lines.push(`  - ${acID} (${status})`);
    }
  }

  return { contents: lines.join('\n') };
}

// ---------------------------------------------------------------------------
// AC-10: Go-to-definition
// ---------------------------------------------------------------------------

export type DefinitionKind = 'spec_id' | 'constraint_ref';

export interface DefinitionRequest {
  kind: DefinitionKind;
  value: string;
  sourceFile: string;
}

/**
 * Resolves a definition request to a file + line:
 *
 *   spec_id       → navigates to the top of the target .spec.yaml file
 *   constraint_ref → navigates to the constraint's declaration in the
 *                    current spec file (line 0 when line info unavailable)
 */
export function resolveDefinitionTarget(
  index: SpecIndex,
  request: DefinitionRequest,
): DefinitionTarget | null {
  if (request.kind === 'spec_id') {
    const spec = index.specs[request.value];
    if (!spec) return null;
    return { file: spec.file, line: 0 };
  }

  if (request.kind === 'constraint_ref') {
    // Find the spec whose file matches sourceFile
    const spec = Object.values(index.specs).find(s => s.file === request.sourceFile);
    if (!spec) return null;

    const constraint = spec.constraints?.find(c => c.id === request.value);
    if (!constraint) return null;

    // Line numbers within the file are not stored in the index; return 0
    // (the extension's real implementation would scan the document for the ID).
    return { file: request.sourceFile, line: 0 };
  }

  return null;
}

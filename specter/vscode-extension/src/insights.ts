// @spec spec-vscode

import type {
  SpecCoverageEntry,
  SpecIndex,
  SpecEntry,
  InsightCard,
} from './types';

// ---------------------------------------------------------------------------
// AC-13: Insight health cards
// ---------------------------------------------------------------------------

/**
 * Builds one InsightCard per failing spec (i.e. passesThreshold === false).
 * Each card contains a human-readable summary, uncovered AC details (with
 * full descriptions), and constraint callouts for constraints that guard
 * uncovered ACs.
 */
export function buildInsightCards(
  entries: SpecCoverageEntry[],
  index: SpecIndex,
): InsightCard[] {
  const cards: InsightCard[] = [];

  for (const entry of entries) {
    if (entry.passesThreshold) continue;

    const spec = index.specs[entry.specID];

    // Uncovered AC details with full descriptions
    const uncoveredACDetails = entry.uncoveredACs.map(acID => {
      const ac = spec?.acs.find(a => a.id === acID);
      return { id: acID, description: ac?.description ?? acID };
    });

    // Constraint callouts: constraints whose referencing ACs include any uncovered AC
    const constraintCallouts: InsightCard['constraintCallouts'] = [];
    if (spec?.constraints && spec.constraintReferences) {
      for (const constraint of spec.constraints) {
        const refs = spec.constraintReferences[constraint.id] ?? [];
        const hasUncoveredRef = refs.some(acID => entry.uncoveredACs.includes(acID));
        if (hasUncoveredRef) {
          constraintCallouts.push({
            constraintID: constraint.id,
            description: constraint.description,
          });
        }
      }
    }

    // Human-readable summary sentence
    const tierLabel = `Tier ${entry.tier}`;
    const thresholdLabel = `${entry.threshold}%`;
    const summary =
      `${entry.specID} has ${entry.uncoveredACs.length} uncovered AC` +
      `${entry.uncoveredACs.length !== 1 ? 's' : ''}. ` +
      `${tierLabel} requires ${thresholdLabel}.`;

    cards.push({ specID: entry.specID, summary, uncoveredACDetails, constraintCallouts });
  }

  return cards;
}

// ---------------------------------------------------------------------------
// AC-16: "Copy spec context for AI" formatter
// ---------------------------------------------------------------------------

/**
 * Formats a spec's tier, constraints, and ACs as a markdown ## Spec Contract
 * block suitable for pasting into an AI prompt.
 *
 * Entirely synchronous — no network call.
 */
export function formatSpecContextForAI(spec: SpecEntry): string {
  const lines: string[] = [
    `## Spec Contract`,
    ``,
    `**Spec:** ${spec.id} · **Tier:** T${spec.tier} · **Status:** ${spec.status}`,
    ``,
  ];

  if (spec.constraints && spec.constraints.length > 0) {
    lines.push('### Constraints', '');
    for (const c of spec.constraints) {
      lines.push(`- **${c.id}:** ${c.description}`);
    }
    lines.push('');
  }

  if (spec.acs.length > 0) {
    lines.push('### Acceptance Criteria', '');
    for (const ac of spec.acs) {
      lines.push(`- **${ac.id}:** ${ac.description}`);
    }
    lines.push('');
  }

  return lines.join('\n');
}

// ---------------------------------------------------------------------------
// AC-17: Onboarding walkthrough trigger
// ---------------------------------------------------------------------------

export interface WalkthroughContext {
  specFiles: string[];
  hasSpecterManifest?: boolean;
}

/**
 * Returns true when the workspace should show the getting-started walkthrough:
 *   • No .spec.yaml files exist, AND
 *   • No specter.yaml manifest is present
 *
 * If either condition is unmet, the walkthrough is suppressed.
 */
export function shouldShowWalkthrough(ctx: WalkthroughContext): boolean {
  return ctx.specFiles.length === 0 && !ctx.hasSpecterManifest;
}

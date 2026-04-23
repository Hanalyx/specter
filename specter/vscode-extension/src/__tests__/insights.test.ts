// @spec spec-vscode
//
// Tests for the Insights WebviewPanel (health cards), the "Copy spec context
// for AI" command formatter, and the onboarding walkthrough trigger logic.

import {
  buildInsightCards,
  computeInsightsStatus,
  formatSpecContextForAI,
  shouldShowWalkthrough,
} from '../insights';

import type { SpecCoverageEntry, SpecIndex } from '../types';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const makeEntry = (
  specID: string,
  tier: number,
  coveredACs: string[],
  uncoveredACs: string[],
  threshold: number,
): SpecCoverageEntry => ({
  specID,
  tier,
  totalACs: coveredACs.length + uncoveredACs.length,
  coveredACs,
  uncoveredACs,
  coveragePct: uncoveredACs.length === 0 ? 100 :
    Math.round(coveredACs.length / (coveredACs.length + uncoveredACs.length) * 100),
  threshold,
  passesThreshold: coveredACs.length / (coveredACs.length + uncoveredACs.length) * 100 >= threshold,
  testFiles: ['src/__tests__/payments.test.ts'],
});

const specIndex: SpecIndex = {
  specs: {
    'payment-create-intent': {
      id: 'payment-create-intent',
      title: 'Create a payment intent',
      tier: 1,
      file: '/project/specs/payments/create-intent.spec.yaml',
      acs: [
        { id: 'AC-01', description: 'Valid currency creates intent successfully' },
        { id: 'AC-02', description: 'Invalid currency returns 422 with error code INVALID_CURRENCY' },
        { id: 'AC-03', description: 'Duplicate idempotency key returns the existing intent' },
      ],
      constraints: [
        { id: 'C-01', description: 'MUST validate currency against ISO 4217' },
        { id: 'C-02', description: 'MUST reject duplicate idempotency keys with 422' },
      ],
      constraintReferences: {
        'C-01': ['AC-01', 'AC-02'],
        'C-02': ['AC-03'],
      },
      coveragePct: 33,
      status: 'approved',
    },
  },
};

// ---------------------------------------------------------------------------
// AC-13: Specter Insights panel — per-spec health cards
// ---------------------------------------------------------------------------

// @ac AC-13
describe('[spec-vscode/AC-13] buildInsightCards', () => {
  it('produces one health card per failing spec', () => {
    const entries = [
      makeEntry('payment-create-intent', 1, ['AC-01'], ['AC-02', 'AC-03'], 100),
      makeEntry('auth-verify-token', 1, ['AC-01', 'AC-02'], [], 100),
    ];
    const cards = buildInsightCards(entries, specIndex);
    // Only the failing spec gets a card
    expect(cards).toHaveLength(1);
    expect(cards[0].specID).toBe('payment-create-intent');
  });

  it('health card sentence summary names the spec and the uncovered AC count', () => {
    const entries = [makeEntry('payment-create-intent', 1, ['AC-01'], ['AC-02', 'AC-03'], 100)];
    const cards = buildInsightCards(entries, specIndex);
    expect(cards[0].summary).toContain('payment-create-intent');
    expect(cards[0].summary).toContain('2');  // 2 uncovered ACs
  });

  it('health card summary mentions the tier threshold requirement', () => {
    const entries = [makeEntry('payment-create-intent', 1, ['AC-01'], ['AC-02', 'AC-03'], 100)];
    const cards = buildInsightCards(entries, specIndex);
    // Tier 1 requires 100%
    expect(cards[0].summary).toMatch(/tier\s*1|100%/i);
  });

  it('health card lists uncovered ACs with their full descriptions (not just IDs)', () => {
    const entries = [makeEntry('payment-create-intent', 1, ['AC-01'], ['AC-02', 'AC-03'], 100)];
    const cards = buildInsightCards(entries, specIndex);
    const card = cards[0];
    // AC-02 and AC-03 are uncovered — check descriptions, not just IDs
    expect(card.uncoveredACDetails.some(d => d.description.includes('422'))).toBe(true);
    expect(card.uncoveredACDetails.some(d => d.description.includes('idempotency'))).toBe(true);
  });

  it('constraint callout lists constraints that guard uncovered ACs', () => {
    const entries = [makeEntry('payment-create-intent', 1, ['AC-01'], ['AC-02', 'AC-03'], 100)];
    const cards = buildInsightCards(entries, specIndex);
    const card = cards[0];
    // C-01 guards AC-02 (uncovered), C-02 guards AC-03 (uncovered)
    const calloutIDs = card.constraintCallouts.map(c => c.constraintID);
    expect(calloutIDs).toContain('C-01');
    expect(calloutIDs).toContain('C-02');
  });

  it('returns empty cards array when all specs pass threshold', () => {
    const entries = [
      makeEntry('payment-create-intent', 1, ['AC-01', 'AC-02', 'AC-03'], [], 100),
    ];
    const cards = buildInsightCards(entries, specIndex);
    expect(cards).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// AC-16: "Copy spec context for AI" — formats spec as markdown prompt preamble
// ---------------------------------------------------------------------------

// @ac AC-16
describe('[spec-vscode/AC-16] formatSpecContextForAI', () => {
  it('includes a ## Spec Contract heading', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const output = formatSpecContextForAI(spec);
    expect(output).toContain('## Spec Contract');
  });

  it('includes the spec tier in the output', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const output = formatSpecContextForAI(spec);
    expect(output).toMatch(/tier\s*1|T1/i);
  });

  it('includes all constraint descriptions', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const output = formatSpecContextForAI(spec);
    expect(output).toContain('MUST validate currency against ISO 4217');
    expect(output).toContain('MUST reject duplicate idempotency keys');
  });

  it('includes all AC IDs and descriptions', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const output = formatSpecContextForAI(spec);
    expect(output).toContain('AC-01');
    expect(output).toContain('Valid currency creates intent successfully');
    expect(output).toContain('AC-02');
    expect(output).toContain('Invalid currency returns 422');
  });

  it('produces a string (no network call — synchronous, no Promise)', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const result = formatSpecContextForAI(spec);
    expect(typeof result).toBe('string');
    expect(result).not.toBeInstanceOf(Promise);
  });

  it('output is valid markdown — contains no raw JSON or YAML syntax', () => {
    const spec = specIndex.specs['payment-create-intent'];
    const output = formatSpecContextForAI(spec);
    // Must not contain YAML block markers or raw JSON braces
    expect(output).not.toMatch(/^\s*---/m);
    expect(output).not.toMatch(/^\s*\{/m);
  });
});

// ---------------------------------------------------------------------------
// AC-17: Onboarding walkthrough — shown only in workspaces with no spec files
// ---------------------------------------------------------------------------

// @ac AC-17
describe('[spec-vscode/AC-17] shouldShowWalkthrough', () => {
  it('returns true when the workspace has no .spec.yaml files', () => {
    expect(shouldShowWalkthrough({ specFiles: [] })).toBe(true);
  });

  it('returns false when at least one .spec.yaml file exists', () => {
    expect(shouldShowWalkthrough({
      specFiles: ['/project/specs/auth.spec.yaml'],
    })).toBe(false);
  });

  it('returns false when specter.yaml exists even with no spec files', () => {
    // A specter.yaml without specs means the project is initialized — skip walkthrough
    expect(shouldShowWalkthrough({
      specFiles: [],
      hasSpecterManifest: true,
    })).toBe(false);
  });

  it('returns false when multiple spec files exist', () => {
    expect(shouldShowWalkthrough({
      specFiles: [
        '/project/specs/auth.spec.yaml',
        '/project/specs/payments.spec.yaml',
      ],
    })).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// AC-37 (v0.9.0): Insights-panel status contract
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-37
describe('[spec-vscode/AC-37] computeInsightsStatus', () => {
  it('never claims "All specs passing" when parse errors exist', () => {
    const status = computeInsightsStatus({
      parseErrorCount: 5,
      uncoveredCardCount: 0,
      entryCount: 3,
      specCandidatesCount: 8,
    });
    expect(status.header.toLowerCase()).not.toContain('all specs passing');
    expect(status.header).toContain('5');
    expect(status.showParseErrorsSection).toBe(true);
  });

  it('mixed state surfaces both counts in the header', () => {
    const status = computeInsightsStatus({
      parseErrorCount: 17,
      uncoveredCardCount: 2,
      entryCount: 4,
      specCandidatesCount: 21,
    });
    expect(status.header).toContain('17');
    expect(status.header).toContain('2');
    expect(status.showParseErrorsSection).toBe(true);
    expect(status.showCoverageSection).toBe(true);
  });

  it('every-spec-failed state is named explicitly', () => {
    const status = computeInsightsStatus({
      parseErrorCount: 22,
      uncoveredCardCount: 0,
      entryCount: 0,
      specCandidatesCount: 22,
    });
    expect(status.header.toLowerCase()).toContain('every discovered spec failed to parse');
    expect(status.showParseErrorsSection).toBe(true);
    expect(status.showCoverageSection).toBe(false);
  });

  it('true happy path ("All specs passing ✓") requires zero parse errors AND zero uncovered cards', () => {
    const status = computeInsightsStatus({
      parseErrorCount: 0,
      uncoveredCardCount: 0,
      entryCount: 14,
      specCandidatesCount: 14,
    });
    expect(status.header).toContain('All specs passing');
    expect(status.showParseErrorsSection).toBe(false);
    expect(status.showCoverageSection).toBe(false);
  });

  it('zero-specs workspace gets a distinct header (not the happy-path claim)', () => {
    const status = computeInsightsStatus({
      parseErrorCount: 0,
      uncoveredCardCount: 0,
      entryCount: 0,
      specCandidatesCount: 0,
    });
    expect(status.header.toLowerCase()).toContain('no specs');
    expect(status.header.toLowerCase()).not.toContain('all specs passing');
  });
});

// @spec spec-vscode
// @ac AC-39
// AC-39's webview-JS click handler is verified by manual VS Code testing
// (the Insights webview runs real browser JS; jest-jsdom isn't wired).
// The pure logic in the Jest-testable layer is the status decision above;
// this test guards the invariant that AC-39 doesn't regress the AC-37
// mixed-state shape by silently showing one section.
describe('[spec-vscode/AC-39] Insights status interaction with parse-error clickability (AC-39 guard)', () => {
  it('parse-errors section is shown iff there are parse errors — so AC-39 headers have a home', () => {
    const withErrors = computeInsightsStatus({
      parseErrorCount: 3, uncoveredCardCount: 0, entryCount: 0, specCandidatesCount: 3,
    });
    expect(withErrors.showParseErrorsSection).toBe(true);

    const without = computeInsightsStatus({
      parseErrorCount: 0, uncoveredCardCount: 1, entryCount: 5, specCandidatesCount: 5,
    });
    expect(without.showParseErrorsSection).toBe(false);
  });
});

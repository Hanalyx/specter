// @spec spec-vscode
//
// Tests for coverage decorations, tree view data model, status bar formatting,
// notification rate limiting, and file decoration computation.

import {
  buildACDecorations,
  buildTreeNodes,
  formatStatusBar,
  classifyNotification,
  buildFileDecoration,
  NotificationRateLimiter,
} from '../coverage';

import type { SpecCoverageEntry, CoverageReport } from '../types';

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
  testFiles: ['src/__tests__/auth.test.ts'],
});

// ---------------------------------------------------------------------------
// AC-05: Coverage gutter decorations on spec file AC lines
// ---------------------------------------------------------------------------

// @ac AC-05
describe('buildACDecorations', () => {
  it('marks covered ACs with green decoration', () => {
    const decs = buildACDecorations({
      coveredACs: ['AC-01', 'AC-02'],
      uncoveredACs: ['AC-03'],
      gapACs: [],
    });
    const covered = decs.filter(d => d.acID === 'AC-01' || d.acID === 'AC-02');
    expect(covered.every(d => d.kind === 'covered')).toBe(true);
  });

  it('marks uncovered ACs with red decoration', () => {
    const decs = buildACDecorations({
      coveredACs: [],
      uncoveredACs: ['AC-01'],
      gapACs: [],
    });
    expect(decs[0].kind).toBe('uncovered');
  });

  it('marks gap ACs with grey-dash decoration, not red', () => {
    const decs = buildACDecorations({
      coveredACs: [],
      uncoveredACs: [],
      gapACs: ['AC-01'],
    });
    expect(decs[0].kind).toBe('gap');
  });

  it('includes test count in end-of-line text for covered ACs', () => {
    const decs = buildACDecorations({
      coveredACs: ['AC-01'],
      uncoveredACs: [],
      gapACs: [],
      testCountByAC: { 'AC-01': 3 },
    });
    expect(decs[0].endOfLineText).toContain('3');
  });
});

// ---------------------------------------------------------------------------
// AC-11: Tree view data model — spec → AC → test files
// ---------------------------------------------------------------------------

// @ac AC-11
describe('buildTreeNodes', () => {
  it('builds a root node per spec file', () => {
    const report: CoverageReport = {
      entries: [
        makeEntry('auth', 1, ['AC-01'], ['AC-02'], 100),
        makeEntry('payments', 2, ['AC-01', 'AC-02'], [], 80),
      ],
      summary: { totalSpecs: 2, passing: 1, failing: 1, fullyCovered: 1, partiallyCovered: 1, uncovered: 0 },
    };
    const nodes = buildTreeNodes(report);
    expect(nodes).toHaveLength(2);
    expect(nodes.map(n => n.specID)).toContain('auth');
    expect(nodes.map(n => n.specID)).toContain('payments');
  });

  it('each spec node has AC children with coverage icons', () => {
    const report: CoverageReport = {
      entries: [makeEntry('auth', 1, ['AC-01'], ['AC-02'], 100)],
      summary: { totalSpecs: 1, passing: 0, failing: 1, fullyCovered: 0, partiallyCovered: 1, uncovered: 0 },
    };
    const nodes = buildTreeNodes(report);
    const acChildren = nodes[0].children;
    expect(acChildren.find(c => c.id === 'AC-01')?.icon).toBe('covered');
    expect(acChildren.find(c => c.id === 'AC-02')?.icon).toBe('uncovered');
  });

  it('each AC node has test file leaves', () => {
    const report: CoverageReport = {
      entries: [makeEntry('auth', 1, ['AC-01'], [], 100)],
      summary: { totalSpecs: 1, passing: 1, failing: 0, fullyCovered: 1, partiallyCovered: 0, uncovered: 0 },
    };
    const nodes = buildTreeNodes(report);
    const ac01 = nodes[0].children.find(c => c.id === 'AC-01')!;
    expect(ac01.children.some(c => c.path === 'src/__tests__/auth.test.ts')).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// AC-12: Status bar text formatting
// ---------------------------------------------------------------------------

// @ac AC-12
describe('formatStatusBar', () => {
  it('formats as "Specter: N specs · X% · F failing"', () => {
    const text = formatStatusBar({ totalSpecs: 12, coveragePct: 94, failing: 2 });
    expect(text).toBe('Specter: 12 specs · 94% · 2 failing');
  });

  it('uses "0 failing" when all specs pass', () => {
    const text = formatStatusBar({ totalSpecs: 5, coveragePct: 100, failing: 0 });
    expect(text).toContain('0 failing');
  });

  it('returns warningBackground color token when a T1/T2 spec fails threshold', () => {
    const result = formatStatusBar({
      totalSpecs: 3, coveragePct: 72, failing: 1,
      hasT1OrT2Failure: true,
    });
    expect(result.colorToken).toBe('statusBarItem.warningBackground');
  });

  it('uses default color when only T3 specs fail or all pass', () => {
    const result = formatStatusBar({
      totalSpecs: 3, coveragePct: 45, failing: 1,
      hasT1OrT2Failure: false,
    });
    expect(result.colorToken).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// AC-18: Notification rate limiting — at most 1 per spec per 60 seconds
// ---------------------------------------------------------------------------

// @ac AC-18
describe('NotificationRateLimiter', () => {
  it('allows the first notification for a spec', () => {
    const limiter = new NotificationRateLimiter({ windowMs: 60_000 });
    expect(limiter.shouldNotify('payment-create-intent')).toBe(true);
  });

  it('suppresses a second notification for the same spec within 60 seconds', () => {
    const now = Date.now();
    const limiter = new NotificationRateLimiter({ windowMs: 60_000, now: () => now });
    limiter.shouldNotify('payment-create-intent'); // first — allowed
    expect(limiter.shouldNotify('payment-create-intent')).toBe(false);
  });

  it('allows a notification after the window has elapsed', () => {
    let t = 0;
    const limiter = new NotificationRateLimiter({ windowMs: 60_000, now: () => t });
    limiter.shouldNotify('payment-create-intent');
    t = 61_000;
    expect(limiter.shouldNotify('payment-create-intent')).toBe(true);
  });

  it('tracks rate limits per spec independently', () => {
    const limiter = new NotificationRateLimiter({ windowMs: 60_000 });
    limiter.shouldNotify('spec-a');
    expect(limiter.shouldNotify('spec-b')).toBe(true);
  });

  it('T3 threshold drop never triggers a toast (only status bar)', () => {
    const result = classifyNotification({ tier: 3, droppedBelowThreshold: true });
    expect(result.kind).toBe('status-bar-only');
  });

  it('T2 threshold drop triggers info toast', () => {
    const result = classifyNotification({ tier: 2, droppedBelowThreshold: true });
    expect(result.kind).toBe('information-toast');
  });
});

// ---------------------------------------------------------------------------
// AC-19: Breaking spec change triggers warning toast; binary-not-found is modal
// ---------------------------------------------------------------------------

// @ac AC-19
describe('classifyNotification', () => {
  it('breaking spec change → warning toast', () => {
    const result = classifyNotification({ changeClass: 'breaking' });
    expect(result.kind).toBe('warning-toast');
    expect(result.actions).toContain('View Diff');
    expect(result.actions).toContain('Dismiss');
  });

  it('binary not found → modal error', () => {
    const result = classifyNotification({ binaryNotFound: true });
    expect(result.kind).toBe('modal-error');
  });

  it('additive change → no notification', () => {
    const result = classifyNotification({ changeClass: 'additive' });
    expect(result.kind).toBe('none');
  });

  it('patch change → no notification', () => {
    const result = classifyNotification({ changeClass: 'patch' });
    expect(result.kind).toBe('none');
  });
});

// ---------------------------------------------------------------------------
// AC-20: File decoration — tier badge and coverage health color
// ---------------------------------------------------------------------------

// @ac AC-20
describe('buildFileDecoration', () => {
  it('returns T1 badge for tier 1 spec', () => {
    const dec = buildFileDecoration(makeEntry('auth', 1, ['AC-01'], [], 100));
    expect(dec.badge).toBe('T1');
  });

  it('returns T2 badge for tier 2 spec', () => {
    const dec = buildFileDecoration(makeEntry('payments', 2, ['AC-01'], ['AC-02'], 80));
    expect(dec.badge).toBe('T2');
  });

  it('uses green color for passing spec', () => {
    const dec = buildFileDecoration(makeEntry('auth', 1, ['AC-01', 'AC-02'], [], 100));
    expect(dec.color).toContain('green');
  });

  it('uses red color for failing T1 spec', () => {
    const dec = buildFileDecoration(makeEntry('auth', 1, ['AC-01'], ['AC-02', 'AC-03'], 100));
    expect(dec.color).toContain('red');
  });

  it('uses yellow color for failing T2 spec', () => {
    const dec = buildFileDecoration(makeEntry('pay', 2, ['AC-01'], ['AC-02'], 80));
    expect(dec.color).toContain('yellow');
  });
});

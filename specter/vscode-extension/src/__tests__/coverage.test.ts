// @spec spec-vscode
//
// Tests for coverage decorations, tree view data model, status bar formatting,
// notification rate limiting, and file decoration computation.

import {
  buildACDecorations,
  buildTreeNodes,
  buildCoverageTreeRoot,
  formatStatusBar,
  classifyNotification,
  buildFileDecoration,
  NotificationRateLimiter,
  resolveCoveringFiles,
  formatSyncCompletion,
  resolveWorkspacePathPure,
  matchFileInIndex,
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

// ---------------------------------------------------------------------------
// v0.8.0 — Empty-state tree rendering (AC-28, AC-29)
// ---------------------------------------------------------------------------

// @ac AC-28
describe('buildCoverageTreeRoot (v0.8.0) — null report', () => {
  it('returns exactly one message node when the report is null', () => {
    const nodes = buildCoverageTreeRoot(null);
    expect(nodes).toHaveLength(1);
    expect(nodes[0].kind).toBe('message');
  });

  it('message references coverage not being available and a next-step action', () => {
    const nodes = buildCoverageTreeRoot(null);
    const node = nodes[0];
    expect(node.kind).toBe('message');
    if (node.kind !== 'message') return; // for TS narrowing
    const text = node.label + ' ' + (node.detail ?? '');
    // Must name the state and point at an action
    expect(text.toLowerCase()).toMatch(/no coverage|not yet|not loaded|not available/);
    expect(text.toLowerCase()).toMatch(/specter init|specter reverse|problems panel|run sync/);
  });
});

// @ac AC-29
describe('buildCoverageTreeRoot (v0.9.0) — parse errors present', () => {
  it('returns a Failed-to-parse group (not a message) when parseErrors is non-empty', () => {
    const erroredReport: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      parseErrors: [
        { file: 'specs/broken.spec.yaml', message: "missing required field 'objective'" },
      ],
    };
    const nodes = buildCoverageTreeRoot(erroredReport);
    expect(nodes).toHaveLength(1);
    expect(nodes[0].kind).toBe('parseErrorGroup');
  });

  it('group label surfaces the failure-count so the user sees it without expanding', () => {
    const erroredReport: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      parseErrors: [
        { file: 'specs/broken.spec.yaml', message: "missing required field 'objective'" },
        { file: 'specs/other.spec.yaml', message: 'yaml: syntax error' },
      ],
    };
    const nodes = buildCoverageTreeRoot(erroredReport);
    if (nodes[0].kind !== 'parseErrorGroup') return;
    expect(nodes[0].label).toMatch(/2/);
    expect(nodes[0].label.toLowerCase()).toContain('failed to parse');
    expect(nodes[0].children).toHaveLength(2);
  });

  it('returns real spec nodes (not a group) when entries are present and parseErrors is absent', () => {
    const populated: CoverageReport = {
      entries: [makeEntry('auth', 1, ['AC-01'], [], 100)],
      summary: { totalSpecs: 1, passing: 1, failing: 0, fullyCovered: 1, partiallyCovered: 0, uncovered: 0 },
    };
    const nodes = buildCoverageTreeRoot(populated);
    expect(nodes.length).toBeGreaterThan(0);
    expect(nodes[0].kind).toBe('spec');
  });
});

// @spec spec-vscode
// @ac AC-33
describe('resolveWorkspacePathPure (v0.9.0) — Coverage tree click-to-open', () => {
  const posixJoin = (a: string, b: string) => (a.endsWith('/') ? a + b : `${a}/${b}`);
  const posixIsAbs = (p: string) => p.startsWith('/');

  it('joins a relative path onto the workspace root', () => {
    const out = resolveWorkspacePathPure('internal/watch/watch_test.go', '/home/user/proj', posixJoin, posixIsAbs);
    expect(out).toBe('/home/user/proj/internal/watch/watch_test.go');
  });

  it('returns absolute paths unchanged', () => {
    const out = resolveWorkspacePathPure('/tmp/foo.go', '/home/user/proj', posixJoin, posixIsAbs);
    expect(out).toBe('/tmp/foo.go');
  });

  it('returns the input unchanged when there is no workspace root', () => {
    const out = resolveWorkspacePathPure('foo.go', undefined, posixJoin, posixIsAbs);
    expect(out).toBe('foo.go');
  });

  it('returns empty input unchanged', () => {
    expect(resolveWorkspacePathPure('', '/home/user/proj', posixJoin, posixIsAbs)).toBe('');
  });
});

// @spec spec-vscode
// @ac AC-31
describe('formatSyncCompletion (v0.9.0) — honest completion toast', () => {
  it('returns an info-level "sync complete" message when no folders errored', () => {
    const c = formatSyncCompletion(0);
    expect(c.kind).toBe('info');
    expect(c.message).toMatch(/sync complete/i);
  });

  it('returns a warning-level message naming the errored folder count', () => {
    const c = formatSyncCompletion(2);
    expect(c.kind).toBe('warning');
    expect(c.message).toMatch(/2 folders/);
    expect(c.message.toLowerCase()).toMatch(/error/);
    expect(c.message.toLowerCase()).toMatch(/output channel/);
  });

  it('uses singular "folder" when exactly one folder errored', () => {
    const c = formatSyncCompletion(1);
    expect(c.kind).toBe('warning');
    expect(c.message).toMatch(/1 folder\b/);
  });
});

// @spec spec-vscode
// @ac AC-32
describe('resolveCoveringFiles (v0.9.0) — hover populates from report', () => {
  const report: CoverageReport = {
    entries: [
      {
        specID: 'payment-create-intent',
        tier: 1,
        totalACs: 2,
        coveredACs: ['AC-01'],
        uncoveredACs: ['AC-02'],
        coveragePct: 50,
        threshold: 100,
        passesThreshold: false,
        testFiles: ['src/payments/create_test.ts', 'src/integration/payment_test.ts'],
      },
    ],
    summary: { totalSpecs: 1, passing: 0, failing: 1, fullyCovered: 0, partiallyCovered: 1, uncovered: 0 },
  };

  it('returns covering files (excluding the current file) for a covered AC', () => {
    const files = resolveCoveringFiles(
      report,
      'payment-create-intent',
      'AC-01',
      'src/payments/create_test.ts',
    );
    expect(files).toEqual(['src/integration/payment_test.ts']);
  });

  it('returns [] for an uncovered AC — buildAnnotationHover will render as "uncovered"', () => {
    const files = resolveCoveringFiles(
      report,
      'payment-create-intent',
      'AC-02',
      'src/payments/create_test.ts',
    );
    expect(files).toEqual([]);
  });

  it('returns [] when the report is null (no coverage run yet)', () => {
    const files = resolveCoveringFiles(null, 'payment-create-intent', 'AC-01', 'anything');
    expect(files).toEqual([]);
  });

  it('returns [] when the spec is not in the report', () => {
    const files = resolveCoveringFiles(report, 'nonexistent-spec', 'AC-01', 'anything');
    expect(files).toEqual([]);
  });
});

// @spec spec-vscode
// @ac AC-38
describe('matchFileInIndex (v0.9.0) — reveal-in-tree path match', () => {
  it('returns the value for an exact key match', () => {
    const idx = new Map([['specs/a.spec.yaml', 'A']]);
    expect(matchFileInIndex(idx, 'specs/a.spec.yaml')).toBe('A');
  });

  it('finds a relative-path key when given the absolute form', () => {
    const idx = new Map([['specs/a.spec.yaml', 'A']]);
    expect(matchFileInIndex(idx, '/home/user/proj/specs/a.spec.yaml')).toBe('A');
  });

  it('finds an absolute-path key when given the relative form', () => {
    const idx = new Map([['/home/user/proj/specs/a.spec.yaml', 'A']]);
    expect(matchFileInIndex(idx, 'specs/a.spec.yaml')).toBe('A');
  });

  it('normalizes windows-style backslashes', () => {
    const idx = new Map([['specs\\a.spec.yaml', 'A']]);
    expect(matchFileInIndex(idx, '/proj/specs/a.spec.yaml')).toBe('A');
  });

  it('returns undefined when nothing matches', () => {
    const idx = new Map([['specs/a.spec.yaml', 'A']]);
    expect(matchFileInIndex(idx, 'specs/b.spec.yaml')).toBeUndefined();
  });
});

// @spec spec-vscode
// @ac AC-36
describe('buildCoverageTreeRoot (v0.9.0) — mixed pass + fail rendering', () => {
  const passingEntry = makeEntry('payments', 1, ['AC-01'], [], 100);

  it('renders a Failed-to-parse group alongside spec nodes when both exist', () => {
    const report: CoverageReport = {
      entries: [passingEntry],
      summary: { totalSpecs: 1, passing: 1, failing: 0, fullyCovered: 1, partiallyCovered: 0, uncovered: 0 },
      specCandidatesCount: 3,
      parseErrors: [
        { file: 'specs/a.spec.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'specs/b.spec.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
      ],
    };
    const nodes = buildCoverageTreeRoot(report);
    // Group first, then passing spec.
    expect(nodes.length).toBe(2);
    expect(nodes[0].kind).toBe('parseErrorGroup');
    expect(nodes[1].kind).toBe('spec');
  });

  it('exposes each failing file as a clickable parseErrorFile child', () => {
    const report: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      parseErrors: [
        { file: 'specs/a.spec.yaml', type: 'required', message: "missing 'objective'", line: 12 },
      ],
    };
    const nodes = buildCoverageTreeRoot(report);
    expect(nodes).toHaveLength(1);
    if (nodes[0].kind !== 'parseErrorGroup') return;
    expect(nodes[0].children).toHaveLength(1);
    const leaf = nodes[0].children[0];
    expect(leaf.file).toBe('specs/a.spec.yaml');
    expect(leaf.line).toBe(12);
    expect(leaf.message).toContain('[required]');
    expect(leaf.message).toContain("missing 'objective'");
  });

  it('omits the Failed-to-parse group when every spec parsed cleanly', () => {
    const report: CoverageReport = {
      entries: [passingEntry],
      summary: { totalSpecs: 1, passing: 1, failing: 0, fullyCovered: 1, partiallyCovered: 0, uncovered: 0 },
      parseErrors: [],
    };
    const nodes = buildCoverageTreeRoot(report);
    expect(nodes.every(n => n.kind !== 'parseErrorGroup')).toBe(true);
  });

  it('names schema drift in the group label when every discovered spec hit the same pattern', () => {
    const report: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      specCandidatesCount: 3,
      parseErrors: [
        { file: 'a.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'b.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'c.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
      ],
      parseErrorPatterns: [{ type: 'required', path: 'spec.objective', count: 3 }],
    };
    const nodes = buildCoverageTreeRoot(report);
    if (nodes[0].kind !== 'parseErrorGroup') return;
    expect(nodes[0].label.toLowerCase()).toContain('schema drift');
    expect(nodes[0].label).toContain('spec.objective');
  });
});

// @spec spec-vscode
// @ac AC-35
describe('buildCoverageTreeRoot (v0.9.0) — drift diagnosis', () => {
  it('names schema drift in the group label when every discovered spec hit the same pattern', () => {
    const report: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      specCandidatesCount: 3,
      parseErrors: [
        { file: 'a.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'b.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'c.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
      ],
      parseErrorPatterns: [
        { type: 'required', path: 'spec.objective', count: 3 },
      ],
    };
    const nodes = buildCoverageTreeRoot(report);
    expect(nodes).toHaveLength(1);
    if (nodes[0].kind !== 'parseErrorGroup') return;
    expect(nodes[0].label.toLowerCase()).toContain('schema drift');
    expect(nodes[0].label).toContain('spec.objective');
  });

  it('does not claim schema drift when parse errors are heterogeneous', () => {
    const report: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      specCandidatesCount: 3,
      parseErrors: [
        { file: 'a.yaml', type: 'required', path: 'spec.objective', message: 'missing' },
        { file: 'b.yaml', type: 'enum', path: 'spec.status', message: 'bad' },
      ],
      parseErrorPatterns: [
        { type: 'required', path: 'spec.objective', count: 1 },
        { type: 'enum', path: 'spec.status', count: 1 },
      ],
    };
    const nodes = buildCoverageTreeRoot(report);
    if (nodes[0].kind !== 'parseErrorGroup') return;
    expect(nodes[0].label.toLowerCase()).not.toContain('schema drift');
  });

  it('surfaces manifest-misconfiguration when candidates exist but no parse errors and no entries', () => {
    const report: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      specCandidatesCount: 5,
      parseErrors: [],
    };
    const nodes = buildCoverageTreeRoot(report);
    if (nodes[0].kind !== 'message') return;
    const text = nodes[0].label + ' ' + (nodes[0].detail ?? '');
    expect(text.toLowerCase()).toContain('manifest');
    expect(text.toLowerCase()).toContain('domains');
    expect(text.toLowerCase()).not.toMatch(/run `specter init` to scaffold a manifest/);
  });
});

// @spec spec-vscode
// @ac AC-30
describe('buildCoverageTreeRoot (v0.9.0) — no specs yet', () => {
  it('returns a message node distinct from the parse-error state when entries is empty and parseErrors is empty/undefined', () => {
    const noSpecs: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
      parseErrors: [],
    };
    const nodes = buildCoverageTreeRoot(noSpecs);
    expect(nodes).toHaveLength(1);
    expect(nodes[0].kind).toBe('message');
    if (nodes[0].kind !== 'message') return;
    const text = nodes[0].label + ' ' + (nodes[0].detail ?? '');
    // Suggests a next step (init/reverse), NOT the Problems panel
    expect(text.toLowerCase()).toMatch(/specter init|specter reverse/);
    expect(text.toLowerCase()).not.toMatch(/problems panel/);
  });

  it('treats missing parseErrors field the same as empty parseErrors (no-specs state)', () => {
    const noSpecs: CoverageReport = {
      entries: [],
      summary: { totalSpecs: 0, passing: 0, failing: 0, fullyCovered: 0, partiallyCovered: 0, uncovered: 0 },
    };
    const nodes = buildCoverageTreeRoot(noSpecs);
    if (nodes[0].kind !== 'message') return;
    const text = nodes[0].label + ' ' + (nodes[0].detail ?? '');
    expect(text.toLowerCase()).toMatch(/specter init|specter reverse/);
  });
});

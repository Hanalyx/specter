// @spec spec-vscode

import type {
  ACDecoration,
  DecorationKind,
  SpecCoverageEntry,
  CoverageReport,
  SpecTreeNode,
  ACNode,
  FileDecoration,
  NotificationResult,
} from './types';

// ---------------------------------------------------------------------------
// AC-05: Coverage gutter decorations
// ---------------------------------------------------------------------------

export interface BuildACDecorationsInput {
  coveredACs: string[];
  uncoveredACs: string[];
  gapACs: string[];
  testCountByAC?: Record<string, number>;
}

/**
 * Returns one ACDecoration per AC ID, with the correct kind and
 * optional end-of-line test count for covered ACs.
 */
export function buildACDecorations(input: BuildACDecorationsInput): ACDecoration[] {
  const decs: ACDecoration[] = [];

  for (const id of input.coveredACs) {
    const count = input.testCountByAC?.[id];
    decs.push({
      acID: id,
      kind: 'covered',
      endOfLineText: count !== undefined ? `${count} test${count !== 1 ? 's' : ''}` : undefined,
    });
  }

  for (const id of input.uncoveredACs) {
    decs.push({ acID: id, kind: 'uncovered' });
  }

  for (const id of input.gapACs) {
    decs.push({ acID: id, kind: 'gap' });
  }

  return decs;
}

// ---------------------------------------------------------------------------
// AC-11: Tree view data model
// ---------------------------------------------------------------------------

/**
 * Converts a CoverageReport into SpecTreeNode[]  for the Specter sidebar.
 * Hierarchy: spec → AC children (with icon) → test file leaves.
 */
export function buildTreeNodes(report: CoverageReport): SpecTreeNode[] {
  return report.entries.map(entry => {
    const acNodes: ACNode[] = [];

    for (const id of entry.coveredACs) {
      acNodes.push({
        id,
        icon: 'covered',
        children: entry.testFiles.map(p => ({ path: p })),
      });
    }

    for (const id of entry.uncoveredACs) {
      acNodes.push({
        id,
        icon: 'uncovered',
        children: [],
      });
    }

    return {
      specID: entry.specID,
      file: '',
      children: acNodes,
    };
  });
}

// ---------------------------------------------------------------------------
// AC-12: Status bar text formatting
// ---------------------------------------------------------------------------

interface StatusBarBase {
  totalSpecs: number;
  coveragePct: number;
  failing: number;
}

export interface StatusBarOptions extends StatusBarBase {
  hasT1OrT2Failure?: boolean;
}

export interface StatusBarResult {
  text: string;
  colorToken?: string;
}

/**
 * Formats the status bar text as "Specter: N specs · X% · F failing".
 *
 * Overloads:
 *   - Without `hasT1OrT2Failure` → returns the plain text string.
 *   - With `hasT1OrT2Failure` (true or false) → returns a StatusBarResult
 *     object with an optional `colorToken` for the warning background.
 */
export function formatStatusBar(opts: StatusBarBase): string;
export function formatStatusBar(opts: StatusBarBase & { hasT1OrT2Failure: boolean }): StatusBarResult;
export function formatStatusBar(opts: StatusBarOptions): string | StatusBarResult {
  const text = `Specter: ${opts.totalSpecs} specs · ${opts.coveragePct}% · ${opts.failing} failing`;

  if (opts.hasT1OrT2Failure === undefined) {
    return text;
  }

  return {
    text,
    colorToken: opts.hasT1OrT2Failure ? 'statusBarItem.warningBackground' : undefined,
  };
}

// ---------------------------------------------------------------------------
// AC-18 / AC-19: Notification classification
// ---------------------------------------------------------------------------

export interface NotifyOptions {
  tier?: number;
  droppedBelowThreshold?: boolean;
  changeClass?: 'breaking' | 'additive' | 'patch';
  binaryNotFound?: boolean;
}

/**
 * Classifies what kind of user notification (if any) should be shown for
 * a given event, according to the notification discipline in C-19.
 */
export function classifyNotification(opts: NotifyOptions): NotificationResult {
  if (opts.binaryNotFound) {
    return { kind: 'modal-error' };
  }

  if (opts.changeClass === 'breaking') {
    return { kind: 'warning-toast', actions: ['View Diff', 'Dismiss'] };
  }

  if (opts.changeClass === 'additive' || opts.changeClass === 'patch') {
    return { kind: 'none' };
  }

  if (opts.droppedBelowThreshold) {
    // T3 → status-bar only; T1/T2 → information toast
    if (opts.tier === 3) {
      return { kind: 'status-bar-only' };
    }
    return { kind: 'information-toast' };
  }

  return { kind: 'none' };
}

// ---------------------------------------------------------------------------
// AC-18: Notification rate limiter — at most 1 per spec per window
// ---------------------------------------------------------------------------

export interface RateLimiterOptions {
  windowMs: number;
  now?: () => number;
}

/**
 * Prevents notification floods: at most one notification per spec per
 * `windowMs` milliseconds.
 */
export class NotificationRateLimiter {
  private readonly lastNotified = new Map<string, number>();
  private readonly windowMs: number;
  private readonly now: () => number;

  constructor(opts: RateLimiterOptions) {
    this.windowMs = opts.windowMs;
    this.now = opts.now ?? (() => Date.now());
  }

  shouldNotify(specID: string): boolean {
    const last = this.lastNotified.get(specID) ?? -Infinity;
    const elapsed = this.now() - last;
    if (elapsed >= this.windowMs) {
      this.lastNotified.set(specID, this.now());
      return true;
    }
    return false;
  }
}

// ---------------------------------------------------------------------------
// AC-20: File decoration — tier badge and coverage health color
// ---------------------------------------------------------------------------

/**
 * Returns the Explorer file decoration for a .spec.yaml node:
 * tier badge (T1/T2/T3) and a coverage health color (green / yellow / red).
 */
export function buildFileDecoration(entry: SpecCoverageEntry): FileDecoration {
  const badge = `T${entry.tier}`;

  let color: string;
  if (entry.passesThreshold) {
    color = 'specter.green';
  } else if (entry.tier === 1) {
    color = 'specter.red';
  } else {
    // T2 failing
    color = 'specter.yellow';
  }

  const tooltip = `${entry.specID} · ${entry.coveragePct}% coverage (threshold ${entry.threshold}%)`;
  return { badge, color, tooltip };
}

// @spec spec-vscode

import type {
  ACDecoration,
  SpecCoverageEntry,
  CoverageReport,
  SpecTreeNode,
  SpecTreeRootNode,
  ACNode,
  FileDecoration,
  NotificationResult,
} from './types';

// ---------------------------------------------------------------------------
// AC-31: honest sync-completion message
// ---------------------------------------------------------------------------

export interface SyncCompletion {
  kind: 'info' | 'warning';
  message: string;
}

/**
 * Returns the completion toast for `specter.runSync` based on how many
 * workspace folders ended up in the error state. Pure so the behavior is
 * testable without a VS Code mock. The v0.8.x bug was an unconditional
 * "Specter sync complete" toast; AC-31 requires the notification to reflect
 * actual outcome.
 */
export function formatSyncCompletion(erroredFolderCount: number): SyncCompletion {
  if (erroredFolderCount <= 0) {
    return { kind: 'info', message: 'Specter sync complete.' };
  }
  const folderLabel = erroredFolderCount === 1 ? 'folder' : 'folders';
  return {
    kind: 'warning',
    message: `Specter sync finished with errors in ${erroredFolderCount} ${folderLabel}. See the Specter Output channel.`,
  };
}

// ---------------------------------------------------------------------------
// AC-38: match an absolute path to a CLI-emitted (often relative) file
// ---------------------------------------------------------------------------

/**
 * Given an index (CLI-emitted path → value) and a target absolute path,
 * return the index key that corresponds to the same file. Matches by
 * (1) exact equality, (2) trailing-path suffix on either side — the CLI
 * may emit `specs/foo.spec.yaml` while VS Code's activeTextEditor returns
 * `/home/user/proj/specs/foo.spec.yaml`. Pure so `specter.revealInTree`
 * can be unit-tested without the VS Code runtime.
 */
export function matchFileInIndex<T>(
  index: ReadonlyMap<string, T>,
  absPath: string,
): T | undefined {
  const direct = index.get(absPath);
  if (direct !== undefined) return direct;
  const norm = absPath.replace(/\\/g, '/');
  for (const [key, value] of index) {
    const keyNorm = key.replace(/\\/g, '/');
    if (norm.endsWith('/' + keyNorm) || keyNorm.endsWith('/' + norm)) return value;
  }
  return undefined;
}

// ---------------------------------------------------------------------------
// AC-33: workspace path resolution for Coverage tree click-to-open
// ---------------------------------------------------------------------------

/**
 * Resolves a CLI-emitted path (often workspace-relative) to the absolute
 * path that `vscode.Uri.file` needs. Pure so the logic is unit-testable
 * without the VS Code runtime. Returns the input unchanged when there's
 * no workspace root to resolve against (zero-folder windows) or when the
 * path is already absolute.
 *
 * Uses POSIX semantics for isAbsolute to stay platform-agnostic in tests;
 * the production caller wraps this with path.resolve which handles
 * Windows-style paths correctly.
 */
export function resolveWorkspacePathPure(
  p: string,
  workspaceRoot: string | undefined,
  join: (a: string, b: string) => string,
  isAbsolute: (p: string) => boolean,
): string {
  if (!p) return p;
  if (isAbsolute(p)) return p;
  if (!workspaceRoot) return p;
  return join(workspaceRoot, p);
}

// ---------------------------------------------------------------------------
// AC-32: covering-files lookup for @ac hovers
// ---------------------------------------------------------------------------

/**
 * Returns the list of test files covering (specID, acID), derived from the
 * live CoverageReport, excluding `currentFile`. Used by the hover provider
 * in extension.ts so a hover on `// @ac AC-01` shows the actual peer test
 * files instead of always rendering as "uncovered" (the v0.8.x bug).
 *
 * Pure so it's testable without the VS Code runtime. Per-AC file mapping
 * isn't yet emitted by the CLI; entry.testFiles is a per-spec proxy and
 * coveredACs membership gates covered-vs-uncovered.
 */
export function resolveCoveringFiles(
  report: CoverageReport | null,
  specID: string,
  acID: string,
  currentFile: string,
  normalize: (p: string) => string = (p) => p,
): string[] {
  if (!report) return [];
  const entry = (report.entries ?? []).find(e => e.specID === specID);
  if (!entry) return [];
  const covered = entry.coveredACs ?? [];
  if (!covered.includes(acID)) return [];
  const current = normalize(currentFile);
  const files = entry.testFiles ?? [];
  return files.filter(f => normalize(f) !== current);
}

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
 * v0.9.0+: returns the sidebar root. Key shift from v0.8.x: passing and
 * failing specs render *together* — parse failures appear as a collapsible
 * "Failed to parse" group alongside the normal spec trees, so the user
 * can act on either kind without the UI hiding one to show the other.
 *
 * State matrix:
 *   1. report === null
 *        → single message node ("coverage not run yet"). Nothing to show.
 *   2. entries === [] AND parseErrors === []
 *        → either greenfield or manifest-excludes-everything. One message.
 *   3. any other combination
 *        → ParseErrorGroupNode (if parseErrors present) + one spec node
 *          per entry. Either list can be empty; we still render the other.
 */
export function buildCoverageTreeRoot(report: CoverageReport | null): SpecTreeRootNode[] {
  if (report === null) {
    return [{
      kind: 'message',
      label: 'No coverage data loaded yet.',
      detail: 'Run `specter init` to create a manifest, `specter reverse src/` to generate specs, or use the Specter: Run Sync command from the palette.',
      iconId: 'info',
    }];
  }

  const entries = report.entries ?? [];
  const parseErrors = report.parseErrors ?? [];

  // Both empty: degenerate case. Name the likely cause.
  if (entries.length === 0 && parseErrors.length === 0) {
    if ((report.specCandidatesCount ?? 0) > 0) {
      return [{
        kind: 'message',
        label: `${report.specCandidatesCount} spec file(s) found but none reached coverage analysis.`,
        detail: 'The manifest (`specter.yaml`) may be excluding your specs — check the `domains` and `settings.specs_dir` sections. Running `specter init` on a workspace with existing specs should auto-populate `domains`.',
        iconId: 'warning',
      }];
    }
    return [{
      kind: 'message',
      label: 'No specs found in this workspace.',
      detail: 'Run `specter init` to scaffold a manifest and your first spec, or `specter reverse src/` to bootstrap specs from existing code.',
      iconId: 'info',
    }];
  }

  const roots: SpecTreeRootNode[] = [];

  // Parse-failure group goes first so it's the first thing the user sees
  // when there's work to do. Collapsible so a workspace with many failures
  // doesn't dominate the view after the errors are triaged.
  if (parseErrors.length > 0) {
    const byFile = new Map<string, { message: string; line?: number }>();
    for (const e of parseErrors) {
      if (!byFile.has(e.file)) {
        const prefix = e.type ? `[${e.type}] ` : '';
        const pathSuffix = e.path ? ` (at ${e.path})` : '';
        byFile.set(e.file, { message: `${prefix}${e.message}${pathSuffix}`, line: e.line });
      }
    }
    const candidates = report.specCandidatesCount ?? byFile.size;
    const top = report.parseErrorPatterns?.[0];
    const topDominant = !!top && top.count === candidates && candidates > 1;
    // Label names the shape when it's a drift pattern; stays generic otherwise.
    const label = topDominant
      ? `Failed to parse: ${byFile.size} file(s) — schema drift (${top.type}${top.path ? ` at ${top.path}` : ''})`
      : `Failed to parse: ${byFile.size} file(s)`;
    roots.push({
      kind: 'parseErrorGroup',
      label,
      children: Array.from(byFile, ([file, info]) => ({
        kind: 'parseErrorFile' as const,
        file,
        message: info.message,
        line: info.line,
      })),
    });
  }

  // Then the passing specs as normal tree nodes.
  for (const n of buildTreeNodes(report)) {
    roots.push({
      kind: 'spec',
      specID: n.specID,
      file: n.file,
      children: n.children,
    });
  }
  return roots;
}

/**
 * Converts a CoverageReport into SpecTreeNode[]  for the Specter sidebar.
 * Hierarchy: spec → AC children (with icon) → test file leaves.
 *
 * Defensive against nullable arrays: the Go CLI emits `null` (not `[]`)
 * for empty slices via `omitempty`, so coveredACs/uncoveredACs/testFiles
 * can be null at runtime even though TypeScript types them as arrays.
 */
export function buildTreeNodes(report: CoverageReport): SpecTreeNode[] {
  return (report.entries ?? []).map(entry => {
    const acNodes: ACNode[] = [];
    const covered = entry.coveredACs ?? [];
    const uncovered = entry.uncoveredACs ?? [];
    const testFiles = entry.testFiles ?? [];

    for (const id of covered) {
      acNodes.push({
        id,
        icon: 'covered',
        children: testFiles.map(p => ({ path: p })),
      });
    }

    for (const id of uncovered) {
      acNodes.push({
        id,
        icon: 'uncovered',
        children: [],
      });
    }

    return {
      specID: entry.specID,
      file: entry.specFile ?? '',
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

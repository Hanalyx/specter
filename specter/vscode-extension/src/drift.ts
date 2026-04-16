// @spec spec-vscode

import type { DriftBaseline, DriftResult, HoverResult } from './types';

// Re-export DriftBaseline so tests can import it from this module
export type { DriftBaseline } from './types';

// ---------------------------------------------------------------------------
// AC-14: Intent drift detection
// ---------------------------------------------------------------------------

export interface DiffOutput {
  changeClass: string;   // 'breaking' | 'additive' | 'patch' — string for test-fixture compatibility
  changes: DiffChange[];
}

export interface DiffChange {
  kind: string;
  acID: string;
  baselineDescription: string | null;
  headDescription: string | null;
}

export interface DriftDetectionContext {
  currentSpecFileHash: string;
  getDiff: () => DiffOutput | null;
}

/**
 * Compares the current spec file hash against the baseline recorded when the
 * `@ac` annotation was last committed.
 *
 * If the hashes match → no drift.
 * If they differ → call getDiff() to classify the change. If getDiff returns
 * null (e.g. not a git repo), drift is reported with changeClass = null.
 */
export function detectDrift(
  baseline: DriftBaseline,
  ctx: DriftDetectionContext,
): DriftResult {
  if (baseline.specFileHashAtAnnotation === ctx.currentSpecFileHash) {
    return { hasDrift: false, changeClass: null };
  }

  const diff = ctx.getDiff();
  if (!diff) {
    return { hasDrift: true, changeClass: null };
  }

  return { hasDrift: true, changeClass: diff.changeClass };
}

// ---------------------------------------------------------------------------
// AC-14: Drift hover card
// ---------------------------------------------------------------------------

export interface DriftHoverOptions {
  specID: string;
  acID: string;
  changeClass: string | null;
  baselineDescription: string | null;
  headDescription: string | null;
}

/**
 * Builds a hover card for the "spec drifted" gutter icon, showing:
 *   • The AC description at baseline and at HEAD
 *   • The change class (breaking / additive / patch)
 *   • A note when an AC was removed or added
 */
export function buildDriftHover(opts: DriftHoverOptions): HoverResult {
  const lines: string[] = [
    `**Spec drift detected** — ${opts.specID} · ${opts.acID}`,
    '',
  ];

  if (opts.changeClass) {
    lines.push(`Change class: **${opts.changeClass}**`, '');
  }

  lines.push('**Baseline:**');
  if (opts.baselineDescription === null) {
    lines.push('  _(AC was not present at baseline — added at HEAD)_');
  } else {
    lines.push(`  ${opts.baselineDescription}`);
  }

  lines.push('', '**HEAD:**');
  if (opts.headDescription === null) {
    lines.push('  _(AC was removed or deleted at HEAD)_');
  } else {
    lines.push(`  ${opts.headDescription}`);
  }

  return { contents: lines.join('\n') };
}

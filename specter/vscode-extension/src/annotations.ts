// @spec spec-vscode

import * as path from 'path';
import type {
  SpecIndex,
  CompletionItem,
  HoverResult,
  QuickFixResult,
  ACSuggestion,
} from './types';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Count how many leading path components two absolute paths share. */
function pathProximity(a: string, b: string): number {
  const aParts = a.split('/');
  const bParts = b.split('/');
  let common = 0;
  for (let i = 0; i < Math.min(aParts.length, bParts.length); i++) {
    if (aParts[i] === bParts[i]) common++;
    else break;
  }
  return common;
}

// ---------------------------------------------------------------------------
// AC-07: @spec completions ranked by directory proximity
// ---------------------------------------------------------------------------

/**
 * Returns completion items for every spec in the index, ranked by directory
 * proximity to `fromFile` (closer specs sort higher).
 */
export function buildSpecCompletions(index: SpecIndex, fromFile: string): CompletionItem[] {
  return Object.values(index.specs)
    .map(spec => {
      const proximity = pathProximity(fromFile, spec.file);
      return {
        label: spec.id,
        insertText: spec.id,
        detail: spec.title,
        documentation: `T${spec.tier} · ${spec.status}`,
        // Lower sortText sorts earlier; negate proximity to sort descending
        sortText: String(1000 - proximity).padStart(6, '0') + spec.id,
      };
    })
    .sort((a, b) => a.sortText!.localeCompare(b.sortText!));
}

// ---------------------------------------------------------------------------
// AC-08: findNearestSpecAnnotation helper
// ---------------------------------------------------------------------------

/**
 * Walks backwards from `line` through `content` to find the closest
 * `// @spec <id>` annotation.  Returns the spec ID or null.
 */
export function findNearestSpecAnnotation(content: string, line: number): string | null {
  const lines = content.split('\n');
  for (let i = Math.min(line, lines.length - 1); i >= 0; i--) {
    const m = lines[i].match(/\/\/\s*@spec\s+(\S+)/);
    if (m) return m[1];
  }
  return null;
}

// ---------------------------------------------------------------------------
// AC-08: @ac completions scoped to the nearest @spec annotation
// ---------------------------------------------------------------------------

/**
 * Returns completion items for AC IDs from the spec referenced by the
 * nearest `@spec` annotation above `cursorLine`.
 */
export function buildACCompletions(
  index: SpecIndex,
  fileContent: string,
  cursorLine: number,
): CompletionItem[] {
  const specID = findNearestSpecAnnotation(fileContent, cursorLine - 1);
  if (!specID) return [];

  const spec = index.specs[specID];
  if (!spec) return [];

  return spec.acs.map(ac => ({
    label: ac.id,
    insertText: ac.id,
    detail: ac.id,
    documentation: ac.description,
  }));
}

// ---------------------------------------------------------------------------
// AC-09: Hover on @ac shows description, coverage status, other files
// ---------------------------------------------------------------------------

export interface AnnotationHoverContext {
  coveredByFiles: string[];
}

/**
 * Builds a hover card for `// @ac <acID>` in a test file, showing:
 *   • Full AC description
 *   • Whether the AC is covered (non-empty coveredByFiles) or uncovered
 *   • Other test files that also cover it
 */
export function buildAnnotationHover(
  index: SpecIndex,
  specID: string,
  acID: string,
  ctx: AnnotationHoverContext,
): HoverResult {
  const spec = index.specs[specID];
  if (!spec) return { contents: '' };

  const ac = spec.acs.find(a => a.id === acID);
  if (!ac) return { contents: '' };

  const isCovered = ctx.coveredByFiles.length > 0;
  const statusText = isCovered ? '**covered**' : '**uncovered**';

  const lines: string[] = [
    `**${acID}** — ${ac.description}`,
    '',
    `Status: ${statusText}`,
  ];

  if (isCovered && ctx.coveredByFiles.length > 0) {
    lines.push('', 'Also covered by:');
    for (const f of ctx.coveredByFiles) {
      lines.push(`  - ${f}`);
    }
  }

  return { contents: lines.join('\n') };
}

// ---------------------------------------------------------------------------
// AC-15: Quick-fix — insert @spec + @ac snippet above unannotated function
// ---------------------------------------------------------------------------

export interface QuickFixOptions {
  specID: string;
  functionLine: number;
}

/**
 * Builds the quick-fix edit that inserts `// @spec <id>` and
 * `// @ac AC-` (with a snippet tab stop at the AC ID position)
 * above the unannotated function.
 */
export function buildQuickFix(opts: QuickFixOptions): QuickFixResult {
  const text = `// @spec ${opts.specID}\n// @ac AC-\${1:??}\n`;
  return {
    insertLine: opts.functionLine,
    text,
    isSnippet: true,
  };
}

// ---------------------------------------------------------------------------
// AC-21: tf-idf AC suggestion — offline, no LM call
// ---------------------------------------------------------------------------

const STOP_WORDS = new Set([
  'the', 'a', 'an', 'is', 'are', 'was', 'were', 'be', 'been', 'being',
  'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'shall',
  'should', 'may', 'might', 'can', 'could', 'to', 'of', 'in', 'for',
  'on', 'with', 'at', 'by', 'from', 'and', 'or', 'not', 'no', 'that',
  'this', 'it', 'its', 'if', 'else', 'while', 'return', 'returns',
  'function', 'const', 'let', 'var', 'class', 'import', 'export',
  'test', 'it', 'describe', 'expect', 'result', 'results', 'when',
  'then', 'given', 'should', 'assert', 'new', 'null', 'undefined',
  'true', 'false', 'type', 'interface', 'any', 'void', 'string', 'number',
]);

/**
 * Splits text into lowercase tokens by splitting on non-word characters and
 * camelCase boundaries, then removes stop words and tokens shorter than 3 chars.
 */
function tokenize(text: string): string[] {
  // Split camelCase: insertTextHere → insert Text Here
  const withSpaces = text.replace(/([a-z])([A-Z])/g, '$1 $2');
  const words = withSpaces.toLowerCase().split(/[^a-z0-9]+/);
  return words.filter(w => w.length >= 3 && !STOP_WORDS.has(w));
}

/** Returns the count of tokens shared between two arrays (multiset intersection). */
function overlapScore(aTokens: string[], bTokens: string[]): number {
  const bSet = new Map<string, number>();
  for (const t of bTokens) bSet.set(t, (bSet.get(t) ?? 0) + 1);

  let score = 0;
  for (const t of aTokens) {
    const count = bSet.get(t) ?? 0;
    if (count > 0) {
      score++;
      bSet.set(t, count - 1);
    }
  }
  return score;
}

/**
 * AC-21 — Scores every AC in the index against the function body using a
 * simple tf-idf-inspired token overlap heuristic.  Returns the top-2
 * suggestions with score > 0, ranked highest-first.
 *
 * Entirely synchronous — no network or LM API call.
 */
export function suggestACsForFunction(index: SpecIndex, functionBody: string): ACSuggestion[] {
  const bodyTokens = tokenize(functionBody);
  if (bodyTokens.length === 0) return [];

  const candidates: ACSuggestion[] = [];

  for (const spec of Object.values(index.specs)) {
    for (const ac of spec.acs) {
      const acTokens = tokenize(ac.description);
      const score = overlapScore(bodyTokens, acTokens);
      if (score > 0) {
        candidates.push({ specID: spec.id, acID: ac.id, description: ac.description, score });
      }
    }
  }

  return candidates
    .sort((a, b) => b.score - a.score)
    .slice(0, 2);
}

// @spec spec-vscode

import type {
  ExtensionDiagnostic,
  SpecterParseError,
  SpecterCheckDiagnostic,
  CoverageParseError,
} from './types';

// ---------------------------------------------------------------------------
// AC-03 / AC-04: Build VS-Code-agnostic diagnostics from specter JSON output
// ---------------------------------------------------------------------------

/**
 * Both halves accept null and undefined (C-31). The client normalizes them to
 * [] before any caller sees a document, and the builder tolerates them anyway,
 * so a caller that reaches here by some other route cannot throw.
 */
export interface BuildDiagnosticsInput {
  parseErrors: SpecterParseError[] | null | undefined;
  checkDiagnostics: SpecterCheckDiagnostic[] | null | undefined;
}

/**
 * The range for a diagnostic the CLI reported at `line`, which is 1-indexed
 * where VS Code is 0-indexed. C-32: an absent position is 0, giving a range at
 * the start of the file, rather than arithmetic on `undefined`. The CLI emits
 * no column on a parse error and no position at all on a check diagnostic, so
 * the range is zero-width and VS Code widens it to the word it lands on.
 */
function rangeAtLine(line: number | undefined): ExtensionDiagnostic['range'] {
  const zeroBased = Math.max(0, (line ?? 1) - 1);
  return {
    start: { line: zeroBased, character: 0 },
    end: { line: zeroBased, character: 0 },
  };
}

/**
 * Converts specter parse errors and check diagnostics into the
 * VS-Code-agnostic ExtensionDiagnostic format.
 *
 * Until v1.9.0 this read `err.col` and `diag.line`, which the CLI emits under
 * no name and in no form. Every range it produced from a real document was
 * `NaN` (SP-SP-025).
 */
export function buildDiagnostics(input: BuildDiagnosticsInput): ExtensionDiagnostic[] {
  const result: ExtensionDiagnostic[] = [];

  for (const err of input.parseErrors ?? []) {
    result.push({
      severity: 'error',
      source: 'specter',
      message: err.message,
      range: rangeAtLine(err.line),
    });
  }

  for (const diag of input.checkDiagnostics ?? []) {
    result.push({
      severity: diag.severity === 'error' ? 'error' : 'warning',
      source: 'specter',
      message: diag.message,
      // A check diagnostic carries no position, so every one lands at the
      // start of the file the caller attached it to.
      range: rangeAtLine(undefined),
    });
  }

  return result;
}

// ---------------------------------------------------------------------------
// AC-34 (v0.9.0): Coverage-level parse errors → per-file diagnostics.
// The `specter coverage --json` output carries a parse_errors array that
// covers every failing spec in one CLI call; grouping by file produces the
// set of DiagnosticCollection.set(...) arguments the Problems panel needs.
// ---------------------------------------------------------------------------

export interface CoverageDiagnosticsPerFile {
  file: string;
  diagnostics: ExtensionDiagnostic[];
}

/**
 * Groups CoverageParseError entries by file and converts each into a
 * VS-Code-agnostic diagnostic. Handles missing line/column gracefully
 * (falls back to line 0) and includes the error type as a prefix so
 * "[required] field objective is missing" reads cleanly in the UI.
 */
export function buildCoverageParseDiagnostics(
  parseErrors: CoverageParseError[] | undefined | null,
): CoverageDiagnosticsPerFile[] {
  if (!parseErrors || parseErrors.length === 0) return [];

  const byFile = new Map<string, ExtensionDiagnostic[]>();
  for (const err of parseErrors) {
    const line = Math.max(0, (err.line ?? 1) - 1);
    const char = Math.max(0, (err.column ?? 1) - 1);
    const prefix = err.type ? `[${err.type}] ` : '';
    const pathSuffix = err.path && err.path !== '' ? ` (at ${err.path})` : '';
    const diag: ExtensionDiagnostic = {
      severity: 'error',
      source: 'specter',
      message: `${prefix}${err.message}${pathSuffix}`,
      // C-32: zero-width, so an error the CLI located at neither a line nor a
      // column still carries finite integers in all four positions.
      range: {
        start: { line, character: char },
        end: { line, character: char },
      },
    };
    const bucket = byFile.get(err.file);
    if (bucket) bucket.push(diag);
    else byFile.set(err.file, [diag]);
  }
  return Array.from(byFile, ([file, diagnostics]) => ({ file, diagnostics }));
}

// ---------------------------------------------------------------------------
// AC-04: Atomic diagnostic replacement
// ---------------------------------------------------------------------------

export interface DiagnosticStore {
  set: (uri: string, diagnostics: unknown[]) => void;
  delete: (uri: string) => void;
}

/**
 * Wraps a VS-Code DiagnosticCollection (or any injectable store) and ensures
 * diagnostics are always replaced atomically — never appended.
 *
 * Passing an empty array deletes all diagnostics for that URI.
 */
export class DiagnosticReplacer {
  constructor(private readonly store: DiagnosticStore) {}

  replace(uri: string, diagnostics: unknown[]): void {
    if (diagnostics.length === 0) {
      this.store.delete(uri);
    } else {
      this.store.set(uri, diagnostics);
    }
  }
}

// shouldRunCoverageForFile was removed in v1.7.0 (spec-vscode AC-55): it
// regex-scanned saved YAML for `// @spec` slash comments — a shape normal
// #-commented spec files never match — to scope a per-spec coverage run
// the CLI never supported. The on-save hook now refreshes the saved
// file's folder report wholesale; see extension.ts.

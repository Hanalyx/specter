// @spec spec-vscode

import type {
  ExtensionDiagnostic,
  SpecterParseError,
  SpecterCheckDiagnostic,
} from './types';

// ---------------------------------------------------------------------------
// AC-03 / AC-04: Build VS-Code-agnostic diagnostics from specter JSON output
// ---------------------------------------------------------------------------

export interface BuildDiagnosticsInput {
  parseErrors: SpecterParseError[];
  checkDiagnostics: SpecterCheckDiagnostic[];
}

/**
 * Converts specter parse errors and check diagnostics into the
 * VS-Code-agnostic ExtensionDiagnostic format.
 *
 * Line / column numbers in specter output are 1-indexed.
 * VS Code ranges are 0-indexed.
 */
export function buildDiagnostics(input: BuildDiagnosticsInput): ExtensionDiagnostic[] {
  const result: ExtensionDiagnostic[] = [];

  for (const err of input.parseErrors) {
    const line = err.line - 1;       // 1-indexed → 0-indexed
    const char = err.col  - 1;       // 1-indexed → 0-indexed
    result.push({
      severity: 'error',
      source: 'specter',
      message: err.message,
      range: {
        start: { line, character: char },
        end:   { line, character: char + 1 },
      },
    });
  }

  for (const diag of input.checkDiagnostics) {
    const line = Math.max(0, diag.line - 1);
    result.push({
      severity: diag.severity === 'error' ? 'error' : 'warning',
      source: 'specter',
      message: diag.message,
      range: {
        start: { line, character: 0 },
        end:   { line, character: Number.MAX_SAFE_INTEGER },
      },
    });
  }

  return result;
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

// ---------------------------------------------------------------------------
// AC-04: Extract @spec IDs from a test file to scope coverage runs
// ---------------------------------------------------------------------------

/**
 * Scans `content` for `// @spec <id>` annotations and returns the unique
 * list of spec IDs found.  Used to scope `specter coverage --spec <id>`
 * to only the specs referenced in the saved test file.
 */
export function shouldRunCoverageForFile(content: string): string[] {
  const ids: string[] = [];
  const pattern = /\/\/\s*@spec\s+(\S+)/g;
  let m: RegExpExecArray | null;
  while ((m = pattern.exec(content)) !== null) {
    const id = m[1];
    if (!ids.includes(id)) ids.push(id);
  }
  return ids;
}

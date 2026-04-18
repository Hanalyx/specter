// @spec spec-vscode

import { execFile } from 'child_process';
import * as path from 'path';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ClientOptions {
  binaryPath: string;
  manifestPath: string;
  workspaceFolder: string;
}

export interface ParseResult {
  errors: Array<{
    file: string;
    line: number;
    col: number;
    message: string;
    code: string;
  }>;
}

export interface CheckResult {
  diagnostics: Array<{
    kind: string;
    severity: string;
    specID: string;
    constraintID?: string;
    message: string;
    file: string;
    line: number;
  }>;
  summary: {
    errors: number;
    warnings: number;
  };
}

export interface CoverageResult {
  entries: Array<{
    specID: string;
    tier: number;
    totalACs: number;
    coveredACs: string[];
    uncoveredACs: string[];
    coveragePct: number;
    threshold: number;
    passesThreshold: boolean;
    testFiles: string[];
  }>;
  summary: {
    totalSpecs: number;
    passing: number;
    failing: number;
    fullyCovered: number;
    partiallyCovered: number;
    uncovered: number;
  };
}

// ---------------------------------------------------------------------------
// SpecterClient — one instance per workspace folder (AC-22)
// ---------------------------------------------------------------------------

/**
 * AC-04, AC-22 — Wraps specter CLI invocations for one workspace folder.
 * All invocations are serialised through a Promise queue to prevent
 * concurrent specter processes against the same manifest (C-04).
 */
export class SpecterClient {
  private queue: Promise<void> = Promise.resolve();
  private abortController: AbortController | null = null;

  constructor(private readonly opts: ClientOptions) {}

  /** Enqueue a task, ensuring it runs after the previous one completes. */
  private enqueue<T>(task: (signal: AbortSignal) => Promise<T>): Promise<T> {
    const controller = new AbortController();
    const result = this.queue.then(() => {
      this.abortController = controller;
      return task(controller.signal);
    });
    this.queue = result.then(
      () => {},
      () => {},
    );
    return result;
  }

  /** Run `specter parse --json <file>`.
   *
   * v0.8.2: no --manifest flag (CLI doesn't support one). The CLI discovers
   * the manifest by walking up from cwd, so we set cwd to the manifest's
   * directory before invoking. Passing --manifest previously caused every
   * parse/check/coverage invocation to fail with "unknown flag: --manifest"
   * which surfaced to users as "no specter.yaml found in workspace" and
   * an error-state status bar.
   */
  parse(filePath: string): Promise<ParseResult> {
    return this.enqueue(signal =>
      this.run(
        ['parse', '--json', path.resolve(this.opts.workspaceFolder, filePath)],
        signal,
      ).then(out => JSON.parse(out) as ParseResult),
    );
  }

  /** Run `specter check --json`. */
  check(): Promise<CheckResult> {
    return this.enqueue(signal =>
      this.run(['check', '--json'], signal).then(out => JSON.parse(out) as CheckResult),
    );
  }

  /** Run `specter coverage --json`. */
  coverage(specID?: string): Promise<CoverageResult> {
    // The CLI has no --spec filter; specID arg is preserved for API
    // compatibility but currently has no effect. Filter callers-side if needed.
    void specID;
    return this.enqueue(signal =>
      this.run(['coverage', '--json'], signal).then(out => JSON.parse(out) as CoverageResult),
    );
  }

  /** Run `specter diff <path>@<baseRef> <path>`.
   *
   * v0.8.2: The CLI takes two positional arguments in the form path[@ref],
   * NOT --base + path. Previous invocation of --json --base <ref> <specFile>
   * produced an "unknown flag" error. There's no --json output for diff;
   * the CLI emits human-readable diff text only.
   */
  diff(specFile: string, baseRef: string): Promise<string> {
    return this.enqueue(signal =>
      this.run([`diff`, `${specFile}@${baseRef}`, specFile], signal),
    );
  }

  /** Cancel any in-flight invocation — called on deactivation (C-18). */
  dispose(): void {
    this.abortController?.abort();
  }

  private run(args: string[], signal: AbortSignal): Promise<string> {
    return new Promise((resolve, reject) => {
      if (signal.aborted) {
        reject(new Error('aborted'));
        return;
      }

      // v0.8.2: set cwd to the manifest's directory so the CLI's findManifest
      // walk-up lands on the right specter.yaml. Without this, cwd inherits
      // from the extension host (often / or VS Code install dir) and the
      // CLI searches from there — finding nothing or the wrong file.
      const cwd = path.dirname(this.opts.manifestPath);
      const proc = execFile(this.opts.binaryPath, args, { cwd }, (err, stdout) => {
        if (err) reject(err);
        else resolve(stdout);
      });

      signal.addEventListener('abort', () => {
        proc.kill();
        reject(new Error('aborted'));
      }, { once: true });
    });
  }
}

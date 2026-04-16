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

  /** Run `specter parse --json <file>`. */
  parse(filePath: string): Promise<ParseResult> {
    return this.enqueue(signal =>
      this.run(
        [
          'parse',
          '--json',
          '--manifest', this.opts.manifestPath,
          path.resolve(this.opts.workspaceFolder, filePath),
        ],
        signal,
      ).then(out => JSON.parse(out) as ParseResult),
    );
  }

  /** Run `specter check --json`. */
  check(): Promise<CheckResult> {
    return this.enqueue(signal =>
      this.run(
        ['check', '--json', '--manifest', this.opts.manifestPath],
        signal,
      ).then(out => JSON.parse(out) as CheckResult),
    );
  }

  /** Run `specter coverage --json [--spec <id>]`. */
  coverage(specID?: string): Promise<CoverageResult> {
    const args = ['coverage', '--json', '--manifest', this.opts.manifestPath];
    if (specID) args.push('--spec', specID);
    return this.enqueue(signal =>
      this.run(args, signal).then(out => JSON.parse(out) as CoverageResult),
    );
  }

  /** Run `specter diff --json --base <ref> <specFile>`. */
  diff(specFile: string, baseRef: string): Promise<unknown> {
    return this.enqueue(signal =>
      this.run(
        ['diff', '--json', '--base', baseRef, specFile],
        signal,
      ).then(out => JSON.parse(out)),
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

      const proc = execFile(this.opts.binaryPath, args, (err, stdout) => {
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

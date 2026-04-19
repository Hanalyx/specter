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
    specFile?: string;
  }>;
  summary: {
    totalSpecs: number;
    passing: number;
    failing: number;
    fullyCovered: number;
    partiallyCovered: number;
    uncovered: number;
  };
  /**
   * v0.9.0+: per-file parse errors from `specter coverage --json`. Present
   * (often as []) whenever the CLI emitted a JSON report. See spec-coverage
   * 1.5.0 C-10 / AC-10 — coverage --json emits JSON in every state; the
   * exit code separately signals pass/fail.
   */
  parseErrors?: Array<{
    file: string;
    path?: string;
    type?: string;
    message: string;
    line?: number;
    column?: number;
  }>;
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

  /** Run `specter coverage --json`.
   *
   * v0.9.0+: the CLI emits a CoverageReport JSON on every run — including
   * when specs fail parse (exit non-zero). Callers branch on
   * `result.parseErrors` to distinguish success vs parse-failed vs
   * no-specs-yet. The queue/abort plumbing stays the same; the only
   * difference from parse()/check() is that a non-zero exit no longer
   * discards stdout.
   *
   * The CLI emits snake_case field names (spec_id, coverage_pct, etc.);
   * this method converts them to the camelCase shape the rest of the
   * extension uses before returning. Prior versions skipped the
   * conversion, which meant every access to `entry.specID`/`coveragePct`
   * etc. silently read `undefined` — a latent bug that would have become
   * a crash the moment any code iterated `coveredACs`.
   */
  coverage(specID?: string): Promise<CoverageResult> {
    // The CLI has no --spec filter; specID arg is preserved for API
    // compatibility but currently has no effect. Filter callers-side if needed.
    void specID;
    return this.enqueue(signal =>
      this.runAllowingNonZero(['coverage', '--json'], signal).then(({ stdout }) => {
        // Locate the JSON document — the CLI may print warn-level lines to
        // stderr that execFile sometimes folds into stdout depending on
        // platform. JSON output always begins with '{'.
        const start = stdout.indexOf('{');
        if (start < 0) {
          throw new Error(
            `specter coverage --json did not emit a JSON document.\n${stdout}`,
          );
        }
        const raw = JSON.parse(stdout.slice(start)) as unknown;
        return snakeToCamelCoverage(raw) as CoverageResult;
      }),
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

  /**
   * Like run(), but treats non-zero exits as data rather than errors:
   * resolves with stdout (and stderr, exit code) regardless. Used by
   * coverage() so the v0.9.0 "JSON on every exit" contract survives the
   * execFile layer, which otherwise rejects with Error and discards
   * stdout when the process exits non-zero.
   */
  private runAllowingNonZero(
    args: string[],
    signal: AbortSignal,
  ): Promise<{ stdout: string; stderr: string; code: number | null }> {
    return new Promise((resolve, reject) => {
      if (signal.aborted) {
        reject(new Error('aborted'));
        return;
      }
      const cwd = path.dirname(this.opts.manifestPath);
      const proc = execFile(this.opts.binaryPath, args, { cwd }, (err, stdout, stderr) => {
        // err.code may be a number (process exit code) or a string (ENOENT
        // etc.). Only reject on "failed to spawn" — for a real exit we
        // still want the output.
        if (err && typeof (err as NodeJS.ErrnoException).code === 'string') {
          reject(err);
          return;
        }
        const code = proc.exitCode;
        resolve({ stdout: stdout.toString(), stderr: stderr.toString(), code });
      });
      signal.addEventListener('abort', () => {
        proc.kill();
        reject(new Error('aborted'));
      }, { once: true });
    });
  }
}

/**
 * Rewrite the Specter CLI's snake_case coverage JSON into the camelCase
 * shape the extension's TypeScript types expect. The CLI emits
 *   spec_id, covered_acs, coverage_pct, passes_threshold, parse_errors, ...
 * but the extension reads
 *   specID, coveredACs, coveragePct, passesThreshold, parseErrors, ...
 * The domain-specific acronyms (ID / ACs) preclude a generic snake→camel
 * rewrite (which would yield specId / coveredAcs). This converter handles
 * the known coverage shape explicitly. Prior versions skipped the step
 * entirely, which meant every access to `entry.specID` silently returned
 * undefined at runtime — a latent bug fixed alongside the v0.9.0 parse
 * errors contract.
 */
const FIELD_MAP: Record<string, string> = {
  spec_id: 'specID',
  total_acs: 'totalACs',
  covered_acs: 'coveredACs',
  uncovered_acs: 'uncoveredACs',
  coverage_pct: 'coveragePct',
  passes_threshold: 'passesThreshold',
  test_files: 'testFiles',
  spec_file: 'specFile',
  spec_candidates_count: 'specCandidatesCount',
  parse_error_patterns: 'parseErrorPatterns',
  example_file: 'exampleFile',
  total_specs: 'totalSpecs',
  fully_covered: 'fullyCovered',
  partially_covered: 'partiallyCovered',
  parse_errors: 'parseErrors',
};

export function snakeToCamelCoverage(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(snakeToCamelCoverage);
  }
  if (value === null || typeof value !== 'object') {
    return value;
  }
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    const mapped = FIELD_MAP[k] ?? k;
    out[mapped] = snakeToCamelCoverage(v);
  }
  return out;
}

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
 * All invocations are serialized through a Promise queue to prevent
 * concurrent specter processes against the same manifest (C-04).
 */
export class SpecterClient {
  private queue: Promise<void> = Promise.resolve();
  private abortController: AbortController | null = null;

  constructor(private readonly opts: ClientOptions) {}

  /**
   * The directory the CLI runs in (the manifest's directory — see run()).
   * CLI-emitted relative paths are relative to this, so it is the
   * resolution root for report-path normalization (AC-33, AC-54).
   */
  get cliCwd(): string {
    return path.dirname(this.opts.manifestPath);
  }

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
      this.runAllowingNonZero(
        ['parse', '--json', path.resolve(this.opts.workspaceFolder, filePath)],
        signal,
      ).then(({ stdout }) => jsonDocumentFrom('parse', stdout) as ParseResult),
    );
  }

  /** Run `specter check --json`.
   *
   * C-30: the exit code is not an input to whether the document is kept.
   * `check --json` exits non-zero whenever it reports an error-severity
   * diagnostic (spec-check C-14), so the workspace that most needs its
   * diagnostics is the workspace whose exit code is non-zero.
   */
  check(): Promise<CheckResult> {
    return this.enqueue(signal =>
      this.runAllowingNonZero(['check', '--json'], signal).then(
        ({ stdout }) => jsonDocumentFrom('check', stdout) as CheckResult,
      ),
    );
  }

  /** Run `specter coverage --json`.
   *
   * v0.9.0+: the CLI emits a CoverageReport JSON on every run, including
   * when specs fail parse (exit non-zero). Callers branch on
   * `result.parseErrors` to distinguish success vs parse-failed vs
   * no-specs-yet. Since v0.15.0 parse() and check() route the same way, and
   * the only difference left is the snake_case conversion below.
   *
   * The CLI emits snake_case field names (spec_id, coverage_pct, etc.);
   * this method converts them to the camelCase shape the rest of the
   * extension uses before returning. Prior versions skipped the
   * conversion, which meant every access to `entry.specID`/`coveragePct`
   * etc. silently read `undefined` — a latent bug that would have become
   * a crash the moment any code iterated `coveredACs`.
   */
  coverage(): Promise<CoverageResult> {
    // The CLI has no per-spec filter — coverage() always returns the
    // whole-workspace report. Callers filter the result if they need a
    // single spec's entry (see findEntryBySpecFile, AC-55).
    // --strictness annotation: the sidebar is a structural coverage view.
    // Since spec-coverage 1.15.0 (C-31) the manifest default `threshold`
    // routes plain `coverage` through the strict path, which hard-fails
    // without .specter-results.json and emits no JSON. Annotation mode
    // preserves the "JSON document on every run" contract this method is
    // built on (and is byte-identical to the pre-1.15.0 default path,
    // including Tier-1 pass-awareness when a results file exists).
    return this.enqueue(signal =>
      this.runAllowingNonZero(['coverage', '--json', '--strictness', 'annotation'], signal).then(
        ({ stdout }) => snakeToCamelCoverage(jsonDocumentFrom('coverage', stdout)) as CoverageResult,
      ),
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

  /**
   * Invocations that read the exit code as their verdict rather than a
   * document. C-30 puts these out of scope on purpose, so a later sweep
   * cannot break them by applying the tolerant rule everywhere. Nothing that
   * passes `--json` may use this path (AC-58 fails the build if one does).
   */
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
   * The path every `--json` invocation takes (C-30). It treats a non-zero
   * exit as data rather than an error and resolves with stdout, stderr, and
   * the exit code, because execFile otherwise rejects and discards stdout.
   * A process that never spawned produced no document, so a spawn failure
   * still rejects and carries its own errno.
   *
   * v0.13 moved coverage() here and left parse() and check() on run(). The
   * v0.14 `check --json` exit-code fix then reached users while check() was
   * still on the rejecting path, so a workspace with one error-severity
   * diagnostic lost its on-save diagnostics entirely (SP-SP-023).
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
 * Read the JSON document a `--json` run wrote to stdout.
 *
 * C-30: the failure predicate is the absence of a parseable document, not the
 * exit code. A run that wrote a document succeeded whatever it exited with,
 * and a run that wrote none failed even at exit 0. Throwing here is what puts
 * the second case on the failure path.
 *
 * The document starts at the first '{'. The CLI may print warn-level lines
 * first, and execFile folds stderr into stdout on some platforms.
 */
function jsonDocumentFrom(command: string, stdout: string): unknown {
  const start = stdout.indexOf('{');
  if (start < 0) {
    throw new Error(`specter ${command} --json did not emit a JSON document.\n${stdout}`);
  }
  return JSON.parse(stdout.slice(start));
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

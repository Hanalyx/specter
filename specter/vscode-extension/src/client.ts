// @spec spec-vscode

import { execFile } from 'child_process';
import * as path from 'path';

import type { SpecterParseError, SpecterCheckDiagnostic, CoverageReport } from './types';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ClientOptions {
  binaryPath: string;
  manifestPath: string;
  workspaceFolder: string;
}

/**
 * The `parse --json` document. The CLI names the file once, on the document,
 * and `value` is the parsed spec on a successful run and null on a failed one.
 * The error shape is the one declared in types.ts, so the two declaration
 * sites cannot drift apart (C-32).
 */
export interface ParseResult {
  errors: SpecterParseError[];
  file?: string;
  ok?: boolean;
  value?: unknown;
}

/** The `check --json` document, after key conversion. */
export interface CheckResult {
  diagnostics: SpecterCheckDiagnostic[];
  summary: {
    errors: number;
    warnings: number;
  };
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
   *
   * C-32: the CLI emits `spec_id` and `constraint_id`, so the document's keys
   * are converted here the way the coverage path converts its own. Before
   * v0.15.0 the check path skipped the step, and every read of
   * `diagnostic.specID` returned undefined.
   */
  check(): Promise<CheckResult> {
    return this.enqueue(signal =>
      this.runAllowingNonZero(['check', '--json'], signal).then(
        ({ stdout }) => snakeToCamelCheck(jsonDocumentFrom('check', stdout)) as CheckResult,
      ),
    );
  }

  /** Run `specter coverage --json`.
   *
   * v0.9.0+: the CLI emits a CoverageReport JSON on every run, including
   * when specs fail parse (exit non-zero). Callers branch on
   * `result.parseErrors` to distinguish success vs parse-failed vs
   * no-specs-yet. Since v0.15.0 parse() and check() route the same way, and
   * check() converts its own document's keys with its own map.
   *
   * The CLI emits snake_case field names (spec_id, coverage_pct, etc.);
   * this method converts them to the camelCase shape the rest of the
   * extension uses before returning. Prior versions skipped the
   * conversion, which meant every access to `entry.specID`/`coveragePct`
   * etc. silently read `undefined` — a latent bug that would have become
   * a crash the moment any code iterated `coveredACs`.
   */
  coverage(): Promise<CoverageReport> {
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
        ({ stdout }) => snakeToCamelCoverage(jsonDocumentFrom('coverage', stdout)) as CoverageReport,
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
 * The array-valued fields of each `--json` document, under the names the CLI
 * emits them by. Go marshals a nil slice as `null` and drops an `omitempty`
 * field entirely, so a workspace with nothing wrong in it is the shape most
 * likely to arrive with a field missing (C-31).
 */
const ARRAY_FIELDS: Record<string, string[]> = {
  parse: ['errors'],
  check: ['diagnostics'],
  coverage: [
    'entries',
    'parse_errors',
    'parse_error_patterns',
    'diagnostic_hints',
    'invalid_status_warnings',
    'results_validation_errors',
  ],
};

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
 *
 * C-31: this is the single point where stdout becomes a document, so it is
 * where `null` and absent become `[]`, before any caller sees the document.
 * A document whose array fields are `null` is still a parseable document, so
 * it stays on the C-30 success path and a clean run still clears the
 * Problems panel. Only the named fields are touched, and only at the top
 * level: `value` on the parse document is null on a failed parse and is not
 * an array field, so normalizing it would fabricate a spec.
 */
function jsonDocumentFrom(command: string, stdout: string): unknown {
  const start = stdout.indexOf('{');
  if (start < 0) {
    throw new Error(`specter ${command} --json did not emit a JSON document.\n${stdout}`);
  }
  const doc: unknown = JSON.parse(stdout.slice(start));
  if (doc === null || typeof doc !== 'object' || Array.isArray(doc)) {
    return doc;
  }
  const out = doc as Record<string, unknown>;
  for (const field of ARRAY_FIELDS[command] ?? []) {
    if (out[field] === null || out[field] === undefined) {
      out[field] = [];
    }
  }
  return out;
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
  // spec-coverage C-10's refusal reason. Without this entry the raw
  // snake_case key survives the conversion and every access to
  // report.stopReason returns undefined, which is the defect this map's own
  // comment records for spec_id.
  stop_reason: 'stopReason',
  // C-44's document. The outer key and both coordinate names, because
  // convertKeys rewrites at every depth and an element's own keys are as much
  // the CLI's spelling as the document's.
  results_validation_errors: 'resultsValidationErrors',
  stream_index: 'streamIndex',
  result_index: 'resultIndex',
};

/**
 * The check document's own keys. Kept separate from the coverage map because
 * the two documents are different shapes and a name correct on one is not
 * automatically correct on the other. `diagnostics` is deliberately absent:
 * the CLI already names it the way the extension reads it.
 */
const CHECK_FIELD_MAP: Record<string, string> = {
  spec_id: 'specID',
  constraint_id: 'constraintID',
  constraint_type: 'constraintType',
  // `change_type` was here until spec-check 2.0.0 retracted version-change
  // classification. No check diagnostic can carry it, and converting a key the
  // CLI never sends is the mirror of the defect C-32 forbids (bugs/SP-SP-018).
};

/** Rewrite every key in `value` through `fieldMap`, at every depth. */
function convertKeys(value: unknown, fieldMap: Record<string, string>): unknown {
  if (Array.isArray(value)) {
    return value.map(v => convertKeys(v, fieldMap));
  }
  if (value === null || typeof value !== 'object') {
    return value;
  }
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    const mapped = fieldMap[k] ?? k;
    out[mapped] = convertKeys(v, fieldMap);
  }
  return out;
}

export function snakeToCamelCoverage(value: unknown): unknown {
  return convertKeys(value, FIELD_MAP);
}

/**
 * The same rewrite for the `check --json` document. The parse document gets
 * no converter: its keys are already the ones types.ts declares, and its
 * `value` member holds the parsed spec, whose keys are the spec schema's and
 * must not be rewritten.
 */
function snakeToCamelCheck(value: unknown): unknown {
  return convertKeys(value, CHECK_FIELD_MAP);
}

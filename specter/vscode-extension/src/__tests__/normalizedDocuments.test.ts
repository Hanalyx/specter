// normalizedDocuments.test.ts
//
// @spec spec-vscode
//
// AC-74 through AC-78 of spec-vscode 1.10.0, frozen at commit 441d3f4. Every
// expected value below is read from that spec's `expected_output` blocks and
// from nothing else.
//
// The five criteria close what the phase 4 mutation review of SP-SP-025 found
// unpinned (SP-SP-033) and the range end no criterion stated (SP-SP-034):
//
//   - AC-74: `client.parse()` on the document a clean parse produces returns
//     `errors` as an empty array. Deleting the parse entry from the client's
//     array-field list passed all 286 tests, and so did moving normalization
//     into the per-command key converters, which leaves the parse path with no
//     converter to move it into.
//   - AC-75: all six coverage array fields C-31 names are arrays, in one
//     document where three arrive `null` and three are absent, plus a second
//     populated document and a read of the declarations themselves.
//   - AC-76: a static scan of `client.ts` alone, asserting that every `--json`
//     invocation in that file hands stdout to the same one function.
//   - AC-77: all four check key-map entries, values under camelCase and no
//     snake key left on the returned diagnostic.
//   - AC-78: a coverage parse error carrying a line highlights that line.
//
// Separate file rather than an addition to `nullDocuments.test.ts`. That file
// is frozen at b9351a4 and takes only the four assertion corrections C-32's
// amendment forces. The AST scan below is the likeliest place for a syntax
// slip, and a compile error takes down every criterion in the file it lands
// in, so it sits with four other criteria instead of with fifteen.
//
// The scripted CLI is copied from `nullDocuments.test.ts`, which copied it from
// `clientExitCode.test.ts`, for the reason that file states: neither exports
// anything, and editing a frozen precedent to share a mock is a worse trade
// than another copy.
//
// Layers, following the same split:
//   - the client, driven through a stubbed child process (AC-74, 75, 77);
//   - a static read of `client.ts` (AC-76);
//   - the pure builder in `diagnostics.ts` (AC-78).

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// Used by the AC-76 static test only, to read the client module's call sites
// without executing them. typescript is already a devDependency (ts-jest).
import * as ts from 'typescript';

// ---------------------------------------------------------------------------
// Scripted CLI
//
// Exit code and stdout vary independently, so a client that keyed on the exit
// code and a client that read the document stay distinguishable.
// ---------------------------------------------------------------------------

interface ScriptedRun {
  exitCode: number;
  stdout: string;
  stderr?: string;
  /** When set, the process never starts and the mock reports this errno. */
  spawnError?: 'ENOENT' | 'EACCES';
}

interface Invocation {
  file: string;
  args: string[];
  /** First argument that is not a flag, e.g. `check` in `check --json`. */
  command: string;
}

const mockCli = {
  scripts: new Map<string, ScriptedRun>(),
  invocations: [] as Invocation[],

  reset(): void {
    mockCli.scripts.clear();
    mockCli.invocations = [];
  },

  /** Script the next run of `command` (`check`, `parse`, `coverage`). */
  script(command: string, run: ScriptedRun): void {
    mockCli.scripts.set(command, run);
  },

  /**
   * The subcommand a run asked for. Known names are matched first so that a
   * flag value (a manifest path, say) placed before the subcommand cannot be
   * mistaken for it.
   */
  commandOf(args: string[]): string {
    const known = ['parse', 'check', 'coverage', 'resolve', 'sync', 'diff', 'ingest', 'doctor'];
    return args.find((a) => known.includes(a)) ?? args.find((a) => !a.startsWith('-')) ?? '';
  },

  /** Called by the mocked child_process functions on every invocation. */
  take(file: string, args: string[]): ScriptedRun {
    const command = mockCli.commandOf(args);
    mockCli.invocations.push({ file, args, command });
    const scripted = mockCli.scripts.get(command);
    if (scripted) {
      return { stderr: '', ...scripted };
    }
    // Unscripted run. `--version` style probes get a version string, anything
    // asking for a document gets an empty but valid one, so an incidental
    // invocation never masquerades as one of the cases under test.
    return args.includes('--json')
      ? { exitCode: 0, stdout: '{}', stderr: '' }
      : { exitCode: 0, stdout: 'specter 0.15.0\n', stderr: '' };
  },

  jsonInvocations(command: string): Invocation[] {
    return mockCli.invocations.filter(
      (i) => i.command === command && i.args.includes('--json'),
    );
  },
};

// Reproduces Node's own semantics, not a plausible imitation: callback
// execFile delivers stdout on a non-zero exit with err.code as the numeric
// status; promisify(execFile) resolves { stdout, stderr } and carries
// util.promisify.custom; a spawn failure sets err.code to the errno string.
// spawn is mocked alongside execFile so an implementation that switches
// invocation style does not silently invalidate these tests.
function mockChildProcess(): Record<string, unknown> {
  const { EventEmitter } = require('events');
  const { Readable } = require('stream');
  const util = require('util');

  function spawnFailure(errno: string, file: string): NodeJS.ErrnoException {
    const err: NodeJS.ErrnoException = new Error(`spawn ${file} ${errno}`);
    err.code = errno;
    err.syscall = 'spawn';
    err.path = file;
    return err;
  }

  function exitFailure(
    run: ScriptedRun,
    file: string,
    args: string[],
  ): NodeJS.ErrnoException {
    const err: NodeJS.ErrnoException & { cmd?: string; killed?: boolean } =
      new Error(`Command failed: ${[file, ...args].join(' ')}\n${run.stderr ?? ''}`);
    err.code = run.exitCode as unknown as string;
    err.cmd = [file, ...args].join(' ');
    err.killed = false;
    return err;
  }

  function execFile(file: string, ...rest: unknown[]): unknown {
    const args = (Array.isArray(rest[0]) ? rest[0] : []) as string[];
    const callback = rest.filter((a) => typeof a === 'function').pop() as
      | ((e: Error | null, so: string, se: string) => void)
      | undefined;
    const run = mockCli.take(file, args);

    const child = new EventEmitter();
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    child.kill = () => true;

    process.nextTick(() => {
      if (!callback) {
        return;
      }
      if (run.spawnError) {
        callback(spawnFailure(run.spawnError, file), '', '');
        return;
      }
      if (run.exitCode === 0) {
        callback(null, run.stdout, run.stderr ?? '');
        return;
      }
      callback(exitFailure(run, file, args), run.stdout, run.stderr ?? '');
    });

    return child;
  }

  (execFile as unknown as Record<symbol, unknown>)[util.promisify.custom] = (
    file: string,
    args?: string[],
    options?: unknown,
  ) =>
    new Promise((resolve, reject) => {
      execFile(
        file,
        args ?? [],
        options ?? {},
        (err: NodeJS.ErrnoException | null, stdout: string, stderr: string) => {
          if (err) {
            (err as NodeJS.ErrnoException & { stdout?: string; stderr?: string }).stdout = stdout;
            (err as NodeJS.ErrnoException & { stdout?: string; stderr?: string }).stderr = stderr;
            reject(err);
            return;
          }
          resolve({ stdout, stderr });
        },
      );
    });

  function spawn(file: string, args: string[] = []): unknown {
    const run = mockCli.take(file, args);
    const child = new EventEmitter();
    child.stdout = new Readable({ read() { /* pushed below */ } });
    child.stderr = new Readable({ read() { /* pushed below */ } });
    child.kill = () => true;
    child.pid = 4242;

    process.nextTick(() => {
      if (run.spawnError) {
        child.stdout.push(null);
        child.stderr.push(null);
        child.emit('error', spawnFailure(run.spawnError, file));
        return;
      }
      if (run.stdout) {
        child.stdout.push(run.stdout);
      }
      if (run.stderr) {
        child.stderr.push(run.stderr);
      }
      child.stdout.push(null);
      child.stderr.push(null);
      process.nextTick(() => {
        child.emit('exit', run.exitCode, null);
        child.emit('close', run.exitCode, null);
      });
    });

    return child;
  }

  function exec(command: string, ...rest: unknown[]): unknown {
    const parts = command.split(/\s+/);
    return execFile(parts[0], parts.slice(1), ...rest);
  }

  function execFileSync(file: string, args: string[] = []): string {
    const run = mockCli.take(file, args);
    if (run.spawnError) {
      throw spawnFailure(run.spawnError, file);
    }
    if (run.exitCode !== 0) {
      const err = exitFailure(run, file, args) as NodeJS.ErrnoException & {
        status?: number; stdout?: string; stderr?: string;
      };
      err.status = run.exitCode;
      err.stdout = run.stdout;
      err.stderr = run.stderr ?? '';
      throw err;
    }
    return run.stdout;
  }

  const mod: Record<string, unknown> = {
    __esModule: true,
    execFile,
    exec,
    spawn,
    execFileSync,
  };
  mod.default = mod;
  return mod;
}

jest.mock('child_process', () => mockChildProcess());
// Same factory under the `node:` specifier, which jest treats as a separate
// module id. Registering both means the mock holds whichever form the client
// imports.
jest.mock('node:child_process', () => mockChildProcess(), { virtual: true });

import { SpecterClient } from '../client';
import { buildCoverageParseDiagnostics } from '../diagnostics';

// ---------------------------------------------------------------------------
// Documents
//
// Byte for byte, the `inputs` blocks of AC-74, AC-75, AC-77, and AC-78. Each
// fixture carries markers unique to itself, and unique to the fixtures in
// `nullDocuments.test.ts` as well, so a swapped fixture shows up in the failure
// message rather than passing on a coincidence.
// ---------------------------------------------------------------------------

/** AC-74. `parse --json` for a spec that passes validation, which is every save of a working spec. */
const PARSE_DOC_CLEAN = `{"errors": null, "file": "specs/a.spec.yaml", "ok": true,
 "value": {"id": "spec-a", "version": "1.0.0"}}`;

/**
 * AC-75. `coverage --json` on a workspace where no spec parsed and no parse
 * error repeated. Three array fields arrive `null` and three are dropped, which
 * is what Go does with a nil slice and with `omitempty`.
 */
const COVERAGE_DOC_EVERY_ARRAY_EMPTY = `{"entries": null,
 "summary": {"total_specs": 0, "fully_covered": 0, "partially_covered": 0,
             "uncovered": 0, "passing": 0, "failing": 0},
 "parse_errors": null,
 "parse_error_patterns": null,
 "spec_candidates_count": 0}`;

/**
 * AC-75, the populated half. An empty-array assertion proves normalization and
 * nothing about conversion, so this document carries elements whose own keys
 * have to be converted. Both element kinds appear, because C-44 gives
 * `undeclared_stream` a `result_index` and every other kind a `stream_index`,
 * and a converter that handled one spelling would pass on a single-kind
 * fixture.
 */
const COVERAGE_DOC_VALIDATION_ERRORS_POPULATED = `{"entries": [],
 "summary": {"total_specs": 1, "fully_covered": 0, "partially_covered": 0,
             "uncovered": 1, "passing": 0, "failing": 1},
 "results_validation_errors": [
   {"kind": "empty_stream_name", "stream": "", "stream_index": 0,
    "message": "streams[0] declares an empty name"},
   {"kind": "undeclared_stream", "stream": "js", "result_index": 0,
    "message": "results[0] names stream js, which the block does not declare"}]}`;

/** AC-77. `check --json` reporting one diagnostic that carries all four convertible keys. */
const CHECK_DOC_FOUR_CONVERTIBLE_KEYS = `{"diagnostics": [
  {"kind": "breaking_change", "severity": "error", "spec_id": "spec-o",
   "constraint_id": "C-02", "constraint_type": "technical",
   "change_type": "constraint_removed",
   "message": "Constraint C-02 was removed from \\"spec-o\\" without a major version bump"}
], "summary": {"errors": 1, "warnings": 0, "info": 0}}`;

/** AC-78. The one coverage parse error the CLI positions: a YAML syntax error. */
const COVERAGE_PARSE_ERROR_WITH_LINE = `{"file": "specs/c.spec.yaml", "path": "", "type": "yaml_syntax",
 "message": "yaml: line 3: mapping values are not allowed in this context",
 "line": 3}`;

// ---------------------------------------------------------------------------
// Result readers
//
// The client's return shapes are not read from its source. Each reader accepts
// the forms the CLI document can legitimately take on the way out and fails
// loudly on anything else, so an unreachable field reads as a named finding
// rather than as `undefined` quietly satisfying an assertion.
//
// Every reader hands back an untyped view. AC-72 and AC-73 were amended at
// 1.10.0 because a criterion that reads a declared interface loses its own red
// to a compile error the moment that interface changes. The same care applies
// here: nothing below is typed through `types.ts`.
// ---------------------------------------------------------------------------

type Loose = Record<string, unknown>;

/** The diagnostics `check()` returned, as an array. Accepts the document or its payload. */
function checkDiagnosticsOf(result: unknown): Loose[] {
  if (Array.isArray(result)) {
    return result as Loose[];
  }
  const value = ((result ?? {}) as Loose).diagnostics;
  if (Array.isArray(value)) {
    return value as Loose[];
  }
  throw new Error(`check() result carried no "diagnostics" array: ${JSON.stringify(result)}`);
}

/** The `coverage()` document. Fails loudly if the report itself is missing. */
function coverageDocOf(result: unknown): Loose {
  if (result && typeof result === 'object' && !Array.isArray(result)) {
    return result as Loose;
  }
  throw new Error(`coverage() returned no report document: ${JSON.stringify(result)}`);
}

/**
 * The `parse()` document. AC-74 reads `ok` and `errors` together, which needs
 * the document rather than an unwrapped errors array. A bare array here is a
 * finding about the layer the criterion was assigned to, so it is reported as
 * such rather than skipped past.
 */
function parseDocOf(result: unknown): Loose {
  if (result && typeof result === 'object' && !Array.isArray(result)) {
    const r = result as Loose;
    if ('ok' in r || 'value' in r) {
      return r;
    }
  }
  throw new Error(
    `parse() returned no document carrying ok or value, so AC-74 is not ` +
    `observable at the client layer: ${JSON.stringify(result)}`,
  );
}

/** Resolution or rejection, captured rather than thrown, so `threw` is assertable. */
async function outcomeOf<T>(work: Promise<T>): Promise<{ threw: boolean; value: unknown }> {
  return work.then(
    (value) => ({ threw: false, value: value as unknown }),
    (reason: unknown) => ({ threw: true, value: reason }),
  );
}

/** `array(0)`, `null`, `absent`, or the value itself, for one labeled comparison. */
function arrayShape(value: unknown): string {
  if (Array.isArray(value)) {
    return `array(${value.length})`;
  }
  if (value === undefined) {
    return 'absent';
  }
  return JSON.stringify(value) ?? String(value);
}

interface RangeLike {
  range: {
    start: { line: number; character: number };
    end: { line: number; character: number };
  };
}

/**
 * C-32 in one labeled object: finite, non-negative, integral, on all four
 * positions. `NaN` fails all three, and the label names which position broke.
 */
function rangeHealth(d: RangeLike): Record<string, boolean> {
  const positions: Record<string, number> = {
    startLine: d.range.start.line,
    startCharacter: d.range.start.character,
    endLine: d.range.end.line,
    endCharacter: d.range.end.character,
  };
  const out: Record<string, boolean> = {};
  for (const [name, value] of Object.entries(positions)) {
    out[name] = Number.isFinite(value) && Number.isInteger(value) && value >= 0;
  }
  return out;
}

const ALL_FOUR_HEALTHY = {
  startLine: true,
  startCharacter: true,
  endLine: true,
  endCharacter: true,
};

// ---------------------------------------------------------------------------
// Workspace
// ---------------------------------------------------------------------------

let workspaceDir: string;
let binaryPath: string;
let manifestPath: string;

beforeAll(() => {
  workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-normdocs-'));
  fs.mkdirSync(path.join(workspaceDir, 'specs'), { recursive: true });
  manifestPath = path.join(workspaceDir, 'specter.yaml');
  fs.writeFileSync(
    manifestPath,
    ['system:', '  name: test', '  tier: 2', 'settings:', '  specs_dir: specs', ''].join('\n'),
  );
  // A real executable file on disk, so a client that stats or checks
  // permissions before invoking sees a healthy binary. The process itself
  // never runs: child_process is mocked.
  binaryPath = path.join(workspaceDir, 'specter');
  fs.writeFileSync(binaryPath, '#!/bin/sh\nexit 0\n', { mode: 0o755 });
});

afterAll(() => {
  fs.rmSync(workspaceDir, { recursive: true, force: true });
});

beforeEach(() => {
  mockCli.reset();
});

const makeClient = () =>
  new SpecterClient({
    binaryPath,
    manifestPath,
    workspaceFolder: workspaceDir,
  });

// ---------------------------------------------------------------------------
// AC-74: the field whose absence caused SP-SP-025, at the layer the fix lives.
//
// AC-62 and AC-63 feed a document straight to the builder and never reach the
// client, so an implementation that normalizes nothing on the parse path
// satisfies both. This one does not.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-74
describe('C-31 parse --json for a spec that passes validation', () => {
  it('[spec-vscode/AC-74] the returned document carries errors as an empty array', async () => {
    // Precondition, not an assertion about the code: the fixture really is the
    // shape a clean parse produces, which is `null` rather than `[]`.
    const raw = JSON.parse(PARSE_DOC_CLEAN);
    expect({ errors: raw.errors, ok: raw.ok }).toEqual({ errors: null, ok: true });

    const client = makeClient();
    mockCli.script('parse', { exitCode: 0, stdout: PARSE_DOC_CLEAN });

    const outcome = await outcomeOf(client.parse(path.join('specs', 'a.spec.yaml')));
    // Checked before anything is read off the result: a rejection here would
    // otherwise surface as a confusing reader error.
    expect({ threw: outcome.threw }).toEqual({ threw: false });

    const doc = parseDocOf(outcome.value);
    expect(doc.ok).toBe(true);

    // The criterion itself. `null` on the wire is an array on the way out, at
    // the one place C-31 says the guarantee is observable.
    const errors = doc.errors;
    expect(Array.isArray(errors)).toBe(true);
    expect(errors).toHaveLength(0);

    // Control: the document came from an actual invocation of the CLI, rather
    // than from a short circuit somewhere earlier.
    expect(mockCli.jsonInvocations('parse')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-75: every array field C-31 names on the coverage document, in one
// document that exercises both branches of the normalizer.
//
// The names asserted are the ones the criterion names: `entries`,
// `parseErrors`, and `parseErrorPatterns` after conversion, and
// `diagnostic_hints` and `invalid_status_warnings` in their CLI spelling,
// because the coverage key map does not name them.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-75
describe('C-31 coverage --json with three null array fields and two absent', () => {
  it('[spec-vscode/AC-75] all six array fields the constraint names come back as empty arrays', async () => {
    // Preconditions. Three null and two absent are different inputs, and the
    // document has to carry both branches for one criterion to cover both.
    const raw = JSON.parse(COVERAGE_DOC_EVERY_ARRAY_EMPTY);
    expect({
      entries: raw.entries,
      parse_errors: raw.parse_errors,
      parse_error_patterns: raw.parse_error_patterns,
    }).toEqual({ entries: null, parse_errors: null, parse_error_patterns: null });
    // Three absent, not two. AC-75's absent_fields names
    // results_validation_errors alongside the other two, and the field is
    // dropped rather than nulled because it carries omitempty. A converter
    // that handles null for it but not absence passes a null fixture and
    // fails on real CLI output.
    expect({
      diagnostic_hints: 'diagnostic_hints' in raw,
      invalid_status_warnings: 'invalid_status_warnings' in raw,
      results_validation_errors: 'results_validation_errors' in raw,
    }).toEqual({
      diagnostic_hints: false,
      invalid_status_warnings: false,
      results_validation_errors: false,
    });

    const client = makeClient();
    // Exit 1, which is what the CLI does when no spec parsed. The document and
    // the exit code vary independently of each other.
    mockCli.script('coverage', { exitCode: 1, stdout: COVERAGE_DOC_EVERY_ARRAY_EMPTY });

    const outcome = await outcomeOf(client.coverage());
    expect({ threw: outcome.threw }).toEqual({ threw: false });

    const report = coverageDocOf(outcome.value);

    // One labeled comparison over all five, so a failure names which fields
    // broke and how each one broke: still null, still absent, or not an array.
    expect({
      entries: arrayShape(report.entries),
      parseErrors: arrayShape(report.parseErrors),
      parseErrorPatterns: arrayShape(report.parseErrorPatterns),
      diagnostic_hints: arrayShape(report.diagnostic_hints),
      invalid_status_warnings: arrayShape(report.invalid_status_warnings),
      resultsValidationErrors: arrayShape(report.resultsValidationErrors),
    }).toEqual({
      entries: 'array(0)',
      parseErrors: 'array(0)',
      parseErrorPatterns: 'array(0)',
      diagnostic_hints: 'array(0)',
      invalid_status_warnings: 'array(0)',
      resultsValidationErrors: 'array(0)',
    });

    // Control: the report came from an actual invocation.
    expect(mockCli.jsonInvocations('coverage')).toHaveLength(1);
  });

  it('[spec-vscode/AC-75] a populated validation array converts its own element keys', async () => {
    // Precondition. The CLI spelling is what the document carries, at both
    // levels, so the assertions below are about conversion rather than about
    // a fixture that was already camel-case.
    const raw = JSON.parse(COVERAGE_DOC_VALIDATION_ERRORS_POPULATED);
    expect({
      outer: 'results_validation_errors' in raw,
      streamIndex: 'stream_index' in raw.results_validation_errors[0],
      resultIndex: 'result_index' in raw.results_validation_errors[1],
    }).toEqual({ outer: true, streamIndex: true, resultIndex: true });

    const client = makeClient();
    mockCli.script('coverage', {
      exitCode: 0,
      stdout: COVERAGE_DOC_VALIDATION_ERRORS_POPULATED,
    });

    const outcome = await outcomeOf(client.coverage());
    expect({ threw: outcome.threw }).toEqual({ threw: false });

    const report = coverageDocOf(outcome.value);
    const errors = report.resultsValidationErrors as Loose[] | undefined;

    // The outer key converts and the array survives with both elements.
    expect({
      shape: arrayShape(report.resultsValidationErrors),
      snakeSurvives: 'results_validation_errors' in report,
    }).toEqual({ shape: 'array(2)', snakeSurvives: false });

    if (!errors || errors.length !== 2) {
      throw new Error(
        `expected two elements to inspect, got ${JSON.stringify(report.resultsValidationErrors)}`,
      );
    }

    // One labeled comparison across both elements, so a failure says which
    // spelling broke and at which coordinate. A converter that stops at the
    // outer key leaves every inner reading here in its CLI spelling.
    expect({
      emptyName: {
        kind: errors[0].kind,
        streamIndex: errors[0].streamIndex,
        snakeSurvives: 'stream_index' in errors[0],
      },
      undeclared: {
        kind: errors[1].kind,
        resultIndex: errors[1].resultIndex,
        snakeSurvives: 'result_index' in errors[1],
      },
    }).toEqual({
      emptyName: { kind: 'empty_stream_name', streamIndex: 0, snakeSurvives: false },
      undeclared: { kind: 'undeclared_stream', resultIndex: 0, snakeSurvives: false },
    });

    // Control: the report came from an actual invocation.
    expect(mockCli.jsonInvocations('coverage')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-76: normalization happens at one point, so a `--json` command added later
// to this file cannot skip it.
//
// Scope is exactly what the criterion states: one file, `client.ts`, and every
// call expression in it whose argument list carries the string literal
// `--json`. The scan says nothing about any other file and nothing about a
// command added elsewhere. AC-58 promised more than one file can deliver and
// the phase 4 review refuted it; this comment is the correction.
//
// Four rules bound what counts as taking stdout, and each is stated here
// rather than left implicit:
//   - a call whose value is discarded is logging or reporting, not the
//     conversion of stdout into a document;
//   - a call used as a guard, meaning `if (f(stdout))` or `if (!f(stdout))`,
//     is a predicate on stdout rather than a conversion of it;
//   - a call whose value is thrown builds an error out of stdout, which is a
//     report of failure rather than a document;
//   - where calls nest, only the innermost counts. In `normalize(parse(stdout))`
//     the callee that takes stdout is `parse`. `normalize` takes a document.
//
// Stdout is followed under a second name where a local holds it, so
// `const text = stdout.trim()` does not read as a handler that converts
// nothing. A value produced by passing stdout to a function is a document
// rather than stdout, and is not followed.
//
// The call that binds stdout is its origin, not a consumer of it. In
// `execFile(bin, args, opts, (err, stdout, stderr) => ...)` the name belongs to
// the callback, and counting it would report the process launcher as the place
// stdout is converted.
// ---------------------------------------------------------------------------

const CLIENT_PATH = path.resolve(__dirname, '..', 'client.ts');

function eachDescendant(root: ts.Node, visit: (n: ts.Node) => void): void {
  ts.forEachChild(root, (n) => {
    visit(n);
    eachDescendant(n, visit);
  });
}

/** `foo(...)` and `x.foo(...)` both answer `foo`. */
function calleeName(call: ts.CallExpression): string | undefined {
  const target = call.expression;
  if (ts.isIdentifier(target)) {
    return target.text;
  }
  if (ts.isPropertyAccessExpression(target)) {
    return target.name.text;
  }
  return undefined;
}

/** The name a function-like node is called by, where it has one. */
function functionName(fn: ts.Node): string | undefined {
  const named = fn as ts.NamedDeclaration;
  if (named.name && ts.isIdentifier(named.name)) {
    return named.name.text;
  }
  const parent = fn.parent;
  if (parent && ts.isVariableDeclaration(parent) && ts.isIdentifier(parent.name)) {
    return parent.name.text;
  }
  if (parent && ts.isPropertyDeclaration(parent) && ts.isIdentifier(parent.name)) {
    return parent.name.text;
  }
  return undefined;
}

function callsIn(fn: ts.Node): ts.CallExpression[] {
  const calls: ts.CallExpression[] = [];
  eachDescendant(fn, (n) => {
    if (ts.isCallExpression(n)) {
      calls.push(n);
    }
  });
  return calls;
}

function enclosingFunction(node: ts.Node): ts.Node | undefined {
  let cur: ts.Node | undefined = node.parent;
  while (cur && !ts.isFunctionLike(cur)) {
    cur = cur.parent;
  }
  return cur;
}

/**
 * The call expression that carries `literal` in its argument list, which is
 * the invocation the criterion counts. Walking up from the literal finds it
 * whether the flag sits directly in the argument list or inside an array
 * literal there.
 */
function invocationCarrying(literal: ts.Node): ts.CallExpression | undefined {
  let child: ts.Node = literal;
  let cur: ts.Node | undefined = literal.parent;
  while (cur) {
    if (ts.isCallExpression(cur) && cur.arguments.some((a) => a === child)) {
      return cur;
    }
    child = cur;
    cur = cur.parent;
  }
  return undefined;
}

/**
 * True when `node` is stdout itself rather than something derived from it by a
 * call: the identifier, a `.stdout` member, or a method called on either. A
 * value produced by passing stdout to a function is a document, not stdout,
 * and is deliberately not matched here.
 */
function isStdoutExpression(node: ts.Node, names: Set<string>): boolean {
  if (ts.isIdentifier(node)) {
    return names.has(node.text);
  }
  if (ts.isPropertyAccessExpression(node)) {
    return names.has(node.name.text) || isStdoutExpression(node.expression, names);
  }
  if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression)) {
    return isStdoutExpression(node.expression.expression, names);
  }
  if (
    ts.isAwaitExpression(node) ||
    ts.isParenthesizedExpression(node) ||
    ts.isNonNullExpression(node)
  ) {
    return isStdoutExpression(node.expression, names);
  }
  return false;
}

/**
 * The local names holding stdout inside `fn`. `const text = stdout.trim()` and
 * `const { stdout: raw } = await run(...)` both name stdout under a second
 * name, and a scan that only knows the word `stdout` would report the
 * conversion as missing. Two passes, so an alias of an alias resolves.
 */
function stdoutAliases(fn: ts.Node): Set<string> {
  const names = new Set<string>(['stdout']);
  for (let pass = 0; pass < 2; pass += 1) {
    eachDescendant(fn, (n) => {
      if (!ts.isVariableDeclaration(n)) {
        return;
      }
      if (ts.isIdentifier(n.name) && n.initializer && isStdoutExpression(n.initializer, names)) {
        names.add(n.name.text);
      }
      if (ts.isObjectBindingPattern(n.name)) {
        for (const element of n.name.elements) {
          const property = element.propertyName;
          if (
            property && ts.isIdentifier(property) && names.has(property.text) &&
            ts.isIdentifier(element.name)
          ) {
            names.add(element.name.text);
          }
        }
      }
    });
  }
  return names;
}

/**
 * True when any of `names` appears in the subtree, without descending into a
 * nested function.
 *
 * A function passed as an argument is where stdout comes from, not something
 * stdout is passed to. `execFile(bin, args, opts, (err, stdout, stderr) => ...)`
 * binds the name in the callback's own scope, and a walk that reads that
 * binding as a use reports the process launcher as a consumer of the value it
 * produces. The stdout inside a callback belongs to the callback. A conversion
 * written there is still found, because the scan reads every call in the
 * function on its own.
 */
function referencesStdout(node: ts.Node, names: Set<string>): boolean {
  if (ts.isFunctionLike(node)) {
    return false;
  }
  if (ts.isIdentifier(node)) {
    return names.has(node.text);
  }
  let hit = false;
  const walk = (n: ts.Node): void => {
    if (hit || ts.isFunctionLike(n)) {
      return;
    }
    // `{ stdout: '' }` names a key. It does not read stdout. The shorthand
    // `{ stdout }` is a different node and does read it.
    if (ts.isPropertyAssignment(n)) {
      walk(n.initializer);
      return;
    }
    if (ts.isIdentifier(n) && names.has(n.text)) {
      hit = true;
      return;
    }
    ts.forEachChild(n, walk);
  };
  ts.forEachChild(node, walk);
  return hit;
}

function takesStdout(call: ts.CallExpression, names: Set<string>): boolean {
  return call.arguments.some((a) => referencesStdout(a, names));
}

/** A call whose value nothing reads. Logging and reporting take this shape. */
function isDiscarded(call: ts.CallExpression): boolean {
  const parent = call.parent;
  return parent !== undefined && ts.isExpressionStatement(parent);
}

/** `if (f(stdout))` and `if (!f(stdout))`, which test stdout rather than convert it. */
function isGuard(call: ts.CallExpression): boolean {
  let node: ts.Node = call;
  let parent: ts.Node | undefined = call.parent;
  if (
    parent &&
    ts.isPrefixUnaryExpression(parent) &&
    parent.operator === ts.SyntaxKind.ExclamationToken
  ) {
    node = parent;
    parent = parent.parent;
  }
  if (parent && (ts.isIfStatement(parent) || ts.isWhileStatement(parent))) {
    return parent.expression === node;
  }
  return false;
}

/** `throw failure(stdout)`, which builds an error rather than a document. */
function isThrown(call: ts.CallExpression): boolean {
  const parent = call.parent;
  return parent !== undefined && ts.isThrowStatement(parent);
}

/** True when `inner` sits inside one of `outer`'s arguments. */
function nestedInArguments(outer: ts.CallExpression, inner: ts.CallExpression): boolean {
  return outer.arguments.some(
    (a) => inner.getStart() >= a.getStart() && inner.getEnd() <= a.getEnd(),
  );
}

/**
 * The calls in `fn` that take stdout as an argument and convert it, innermost
 * only. Anything holding another such call inside its own argument list is
 * receiving that call's result rather than stdout.
 */
function stdoutConvertingCalls(fn: ts.Node): ts.CallExpression[] {
  const names = stdoutAliases(fn);
  const candidates = callsIn(fn).filter(
    (c) => takesStdout(c, names) && !isDiscarded(c) && !isGuard(c) && !isThrown(c),
  );
  return candidates.filter(
    (outer) => !candidates.some((inner) => inner !== outer && nestedInArguments(outer, inner)),
  );
}

// @spec spec-vscode
// @ac AC-76
describe('C-31 the single point where stdout becomes a document', () => {
  it('[spec-vscode/AC-76] every --json invocation in client.ts hands stdout to the same one callee', () => {
    const src = fs.readFileSync(CLIENT_PATH, 'utf-8');
    const sf = ts.createSourceFile(CLIENT_PATH, src, ts.ScriptTarget.ES2020, true);

    // Every function-like in the module, indexed by the name it is called by,
    // so an invocation that delegates can be followed to where it lands.
    const functions: ts.Node[] = [];
    eachDescendant(sf, (n) => {
      if (ts.isFunctionLike(n)) {
        functions.push(n);
      }
    });
    const byName = new Map<string, ts.Node>();
    for (const fn of functions) {
      const name = functionName(fn);
      if (name && !byName.has(name)) {
        byName.set(name, fn);
      }
    }

    /**
     * Callees that take stdout as an argument, reachable from `fn` by
     * following named calls up to three hops. Three hops because an
     * implementation may build its argument list in one helper, invoke in
     * another, and convert in a third. Any implementation form is accepted
     * (AC-52 precedent); what is checked is where stdout ends up.
     *
     * The walk stops at each callee it collects. What happens inside the
     * conversion point is that point's own business, and a converter whose
     * parameter is also named `stdout` would otherwise report the calls it
     * makes internally as a second conversion point.
     */
    const stdoutCallees = (fn: ts.Node, seen: Set<ts.Node>, depth = 0): Set<string> => {
      const found = new Set<string>();
      if (depth > 3 || seen.has(fn)) {
        return found;
      }
      seen.add(fn);
      const collectedHere = new Set<string>();
      for (const call of stdoutConvertingCalls(fn)) {
        const name = calleeName(call);
        if (name) {
          found.add(name);
          collectedHere.add(name);
        }
      }
      for (const call of callsIn(fn)) {
        const name = calleeName(call);
        if (!name || collectedHere.has(name)) {
          continue;
        }
        const callee = byName.get(name);
        if (callee && callee !== fn) {
          stdoutCallees(callee, seen, depth + 1).forEach((r) => found.add(r));
        }
      }
      return found;
    };

    // Every `--json` literal in the module, with the call expression that
    // carries it and the function that call sits in.
    const literals: ts.Node[] = [];
    eachDescendant(sf, (n) => {
      if ((ts.isStringLiteral(n) || ts.isNoSubstitutionTemplateLiteral(n)) && n.text === '--json') {
        literals.push(n);
      }
    });

    const sites: Array<{ label: string; owner: ts.Node }> = [];
    const unplaced: string[] = [];
    for (const literal of literals) {
      const invocation = invocationCarrying(literal);
      const owner = invocation ? enclosingFunction(invocation) : undefined;
      if (!invocation || !owner) {
        unplaced.push(`line ${sf.getLineAndCharacterOfPosition(literal.getStart(sf)).line + 1}`);
        continue;
      }
      sites.push({ label: functionName(owner) ?? 'anonymous', owner });
    }

    // Positive control for the one-member claim below. The locator found real
    // invocations and placed every one of them. parse, check, and coverage each
    // request a document, so three is the floor the criterion states. Zero
    // sites would otherwise satisfy "they all share one callee" with nothing
    // having been examined.
    expect(sites.length).toBeGreaterThanOrEqual(3);
    expect(unplaced).toEqual([]);

    const union = new Set<string>();
    for (const site of sites) {
      const found = stdoutCallees(site.owner, new Set<ts.Node>());
      // Per site, not only across the union. An invocation whose handler never
      // hands stdout to anything contributes nothing to the union, so the
      // union alone would let exactly the mutant this criterion exists to kill
      // pass: a fourth command that never reaches the normalizer.
      expect({ site: site.label, reachesAStdoutCallee: found.size > 0 })
        .toEqual({ site: site.label, reachesAStdoutCallee: true });
      found.forEach((name) => union.add(name));
    }

    // One member. Every `--json` document in this file passes through the same
    // function on its way out, which is what "the single point where it parses
    // stdout into a document" means in C-31.
    expect([...union].sort()).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-77: all four check key-map entries.
//
// AC-72 pins `spec_id` alone, so deleting any of the other three entries from
// the map changes nothing any criterion reads. The absence half is what kills
// the mutant that converts the check document with the coverage map, because
// that map also carries `spec_id` and would leave the other three untouched.
// ---------------------------------------------------------------------------

const CONVERTED_SNAKE_KEYS = ['spec_id', 'constraint_id', 'constraint_type', 'change_type'];

// @spec spec-vscode
// @ac AC-77
describe('C-32 the check document keys the client converts', () => {
  it('[spec-vscode/AC-77] all four keys arrive under their camelCase names and none survives in snake case', async () => {
    // Precondition: the fixture really does carry all four keys the map names.
    const raw = JSON.parse(CHECK_DOC_FOUR_CONVERTIBLE_KEYS);
    expect(CONVERTED_SNAKE_KEYS.filter((k) => k in raw.diagnostics[0])).toEqual(CONVERTED_SNAKE_KEYS);

    const client = makeClient();
    // Exit 1, which is what the CLI does on an error-severity diagnostic.
    mockCli.script('check', { exitCode: 1, stdout: CHECK_DOC_FOUR_CONVERTIBLE_KEYS });

    const diagnostics = checkDiagnosticsOf(await client.check());
    expect(diagnostics).toHaveLength(1);
    const diagnostic = diagnostics[0];

    // Read through an untyped view, never through `SpecterCheckDiagnostic`, so
    // that a change to the declaration cannot stop this file compiling and
    // take every criterion in it down with it (AC-72 at 1.10.0).
    //
    // These four are also the positive control for the absence claim below.
    // Four values present under their converted names proves the document
    // arrived and the converter ran, so an empty snake-key list is conversion
    // rather than a diagnostic that never got read.
    expect({
      specID: diagnostic.specID,
      constraintID: diagnostic.constraintID,
      constraintType: diagnostic.constraintType,
      changeType: diagnostic.changeType,
    }).toEqual({
      specID: 'spec-o',
      constraintID: 'C-02',
      constraintType: 'technical',
      changeType: 'constraint_removed',
    });

    // The absence half. Exactly the four keys the map names, not any key that
    // happens to carry an underscore.
    expect(Object.keys(diagnostic).filter((k) => CONVERTED_SNAKE_KEYS.includes(k))).toEqual([]);

    // Control: the document came from an actual invocation.
    expect(mockCli.jsonInvocations('check')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-78: the coverage parse error the CLI positions.
//
// AC-71 covers the same builder with no position at all. An implementation
// that puts every coverage parse error at the start of the file satisfies
// AC-71 and fails this one, which is the AC-64 and AC-65 split applied to the
// coverage path.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-78
describe('C-32 a coverage parse error carrying a line', () => {
  it('[spec-vscode/AC-78] a YAML syntax error highlights the line the CLI reported, to the end of it', () => {
    const err = JSON.parse(COVERAGE_PARSE_ERROR_WITH_LINE);
    // The line is present and the column is absent. The CLI emits `line` on a
    // coverage parse error only for a YAML syntax error, and emits no column
    // at all, because no code path in either the parser or the coverage
    // package assigns that field.
    expect(err.line).toBe(3);
    expect('column' in err).toBe(false);

    const out = buildCoverageParseDiagnostics([err]);

    expect(out).toHaveLength(1);
    // The diagnostic still names its file, which is how AC-34 routes it to a
    // Problems-panel entry at all.
    expect(out[0].file).toBe('specs/c.spec.yaml');
    expect(out[0].diagnostics).toHaveLength(1);
    expect(rangeHealth(out[0].diagnostics[0])).toEqual(ALL_FOUR_HEALTHY);
    // CLI line 3 is 1-based; VS Code is 0-based. The absent column is 0.
    expect(out[0].diagnostics[0].range.start).toEqual({ line: 2, character: 0 });
    // Ends on the start line, at the end of it, so the highlight covers the
    // line the CLI reported rather than nothing.
    expect(out[0].diagnostics[0].range.end).toEqual({
      line: 2,
      character: Number.MAX_SAFE_INTEGER,
    });
    expect(out[0].diagnostics[0].message).toContain('mapping values are not allowed');
  });
});

// ---------------------------------------------------------------------------
// AC-75, the type half. C-31 requires one shared declaration rather than a
// runtime shape that happens to match.
//
// The runtime tests above pass on a converter that renames keys and returns
// `unknown`. Every type-level output AC-75 names would still be unmet: the
// union could live in client.ts, CoverageReport could not carry the field,
// coverage() could keep returning CoverageResult, and extension.ts could keep
// bridging the two with `as unknown as`. So these read the declarations.
//
// A structural AST read rather than the type checker. `ts.createProgram` over
// the extension needs the full tsconfig and resolves every import, which is
// slow here and fails for reasons unrelated to the criterion. The properties
// AC-75 names are syntactic: which file declares the type, which interface
// carries it, and which coordinate each union branch permits.
// ---------------------------------------------------------------------------

const TYPES_PATH = path.resolve(__dirname, '..', 'types.ts');
const EXTENSION_PATH = path.resolve(__dirname, '..', 'extension.ts');

function sourceOf(file: string): ts.SourceFile {
  return ts.createSourceFile(
    file,
    fs.readFileSync(file, 'utf-8'),
    ts.ScriptTarget.Latest,
    true,
  );
}

/** The property signatures of a named interface, or undefined if absent. */
function interfaceProps(src: ts.SourceFile, name: string): ts.PropertySignature[] | undefined {
  let found: ts.PropertySignature[] | undefined;
  ts.forEachChild(src, (n) => {
    if (ts.isInterfaceDeclaration(n) && n.name.text === name) {
      found = n.members.filter(ts.isPropertySignature);
    }
  });
  return found;
}

function propNamed(props: ts.PropertySignature[], name: string): ts.PropertySignature | undefined {
  return props.find((p) => p.name && ts.isIdentifier(p.name) && p.name.text === name);
}

/** `Foo[]` -> `Foo`. Anything else -> undefined. */
function arrayElementTypeName(node: ts.TypeNode | undefined): string | undefined {
  if (!node || !ts.isArrayTypeNode(node)) {
    return undefined;
  }
  const el = node.elementType;
  return ts.isTypeReferenceNode(el) && ts.isIdentifier(el.typeName) ? el.typeName.text : undefined;
}

/**
 * One property of a union branch. Optionality is carried, not discarded: AC-75
 * says a branch *requires* its coordinate, and `resultIndex?: number` declares
 * a branch that permits none. A parser that records only names cannot tell the
 * two apart, and the assertion built on it accepts the invalid one.
 */
interface Prop {
  name: string;
  optional: boolean;
  isNumber: boolean;
}

/** One branch of the element union, reduced to what AC-75 asserts about it. */
interface Branch {
  kind?: string;
  props: Prop[];
}

function propOf(b: Branch, name: string): Prop | undefined {
  return b.props.find((p) => p.name === name);
}

/**
 * A union alias reduced to what AC-75 asserts about it.
 *
 * nonLiteral counts members that are not object literals. They are counted
 * rather than filtered away: `A | B | C | D | E | undefined` yields the five
 * kinds a filtering parser expects and still declares a type whose values may
 * carry no discriminant and no coordinate.
 */
interface Union {
  branches: Branch[];
  nonLiteral: number;
}

/** `export type <name> = {...} | {...}` parsed, or undefined if not a union. */
function unionOf(src: ts.SourceFile, name: string): Union | undefined {
  let alias: ts.TypeAliasDeclaration | undefined;
  ts.forEachChild(src, (n) => {
    if (ts.isTypeAliasDeclaration(n) && n.name.text === name) {
      alias = n;
    }
  });
  if (!alias || !ts.isUnionTypeNode(alias.type)) {
    return undefined;
  }
  const members = alias.type.types;
  const branches = members.filter(ts.isTypeLiteralNode).map((lit) => {
    const signatures = lit.members.filter(ts.isPropertySignature);
    const props: Prop[] = signatures
      .filter((p) => p.name && ts.isIdentifier(p.name))
      .map((p) => ({
        name: (p.name as ts.Identifier).text,
        optional: p.questionToken !== undefined,
        isNumber: p.type?.kind === ts.SyntaxKind.NumberKeyword,
      }));
    const kindProp = signatures.find(
      (p) => p.name && ts.isIdentifier(p.name) && p.name.text === 'kind',
    );
    let kind: string | undefined;
    if (kindProp?.type && ts.isLiteralTypeNode(kindProp.type)) {
      const lt = kindProp.type.literal;
      if (ts.isStringLiteral(lt)) {
        kind = lt.text;
      }
    }
    return { kind, props };
  });
  return { branches, nonLiteral: members.length - branches.length };
}

/** The five kinds C-44 names. spec-coverage pins every one of these strings. */
const C44_KINDS = [
  'duplicate_stream',
  'empty_stream_name',
  'extracted_below_entries',
  'negative_count',
  'undeclared_stream',
];

describe('C-31 the validation error type is declared once and shared', () => {
  // @spec spec-vscode
  // @ac AC-75
  it('[spec-vscode/AC-75] CoverageReport carries the array and types.ts declares the element', () => {
    const types = sourceOf(TYPES_PATH);
    const props = interfaceProps(types, 'CoverageReport');
    if (!props) {
      throw new Error('types.ts declares no CoverageReport interface');
    }

    const field = propNamed(props, 'resultsValidationErrors');
    // Reported rather than thrown, so the next assertion still runs and one
    // failure names everything that is missing.
    expect({
      CoverageReport_carries_the_array: field !== undefined,
    }).toEqual({ CoverageReport_carries_the_array: true });

    const elementName = arrayElementTypeName(field?.type);
    expect({
      field_is_an_array_of_a_named_type: elementName !== undefined,
    }).toEqual({ field_is_an_array_of_a_named_type: true });

    expect({
      the_element_type_is_declared_in_types_ts:
        elementName !== undefined && unionOf(types, elementName) !== undefined,
    }).toEqual({ the_element_type_is_declared_in_types_ts: true });
  });

  // @spec spec-vscode
  // @ac AC-75
  it('[spec-vscode/AC-75] the element type is a discriminated union permitting exactly one coordinate', () => {
    const types = sourceOf(TYPES_PATH);
    const props = interfaceProps(types, 'CoverageReport');
    const elementName = arrayElementTypeName(propNamed(props ?? [], 'resultsValidationErrors')?.type);
    const union = elementName ? unionOf(types, elementName) : undefined;
    if (!union || union.branches.length === 0) {
      throw new Error(
        `no discriminated union to inspect: CoverageReport.resultsValidationErrors resolves to ${String(elementName)}`,
      );
    }
    const branches = union.branches;

    // Every member is an object literal. A member that is not one contributes
    // no kind, so it cannot break the set assertion below while still widening
    // the type to values carrying neither discriminant nor coordinate.
    expect({ members_that_are_not_object_literals: union.nonLiteral }).toEqual({
      members_that_are_not_object_literals: 0,
    });

    // The union covers every kind C-44 names, and no others. A two-branch
    // union satisfied the earlier version of this test while omitting
    // duplicate_stream, empty_stream_name, negative_count and
    // extracted_below_entries, so the branch rules below ran over a fraction
    // of the type and reported nothing.
    expect(branches.map((b) => b.kind).sort()).toEqual([...C44_KINDS]);

    // Every branch discriminates on a string literal kind and names the two
    // fields C-44(c) requires of every violation. Required, not merely
    // declared: an optional message is a violation that need not say anything.
    expect(
      branches.map((b) => ({
        kind: b.kind,
        kindIsLiteral: typeof b.kind === 'string',
        // Required. An optional discriminant permits an element with no kind,
        // which is not a discriminated union whatever the branches say.
        kindIsRequired: propOf(b, 'kind')?.optional === false,
        stream: propOf(b, 'stream')?.optional === false,
        message: propOf(b, 'message')?.optional === false,
      })),
    ).toEqual(
      branches.map((b) => ({
        kind: b.kind,
        kindIsLiteral: true,
        kindIsRequired: true,
        stream: true,
        message: true,
      })),
    );

    // Exactly one coordinate per branch, required, numeric, and the right one
    // for the kind. The other must be absent rather than optional: an optional
    // coordinate on the wrong branch is a value the type still permits.
    expect(
      branches.map((b) => {
        const wanted = b.kind === 'undeclared_stream' ? 'resultIndex' : 'streamIndex';
        const forbidden = b.kind === 'undeclared_stream' ? 'streamIndex' : 'resultIndex';
        const coordinate = propOf(b, wanted);
        return {
          kind: b.kind,
          requiredCoordinate: coordinate !== undefined && !coordinate.optional,
          coordinateIsNumber: coordinate?.isNumber === true,
          otherAbsent: propOf(b, forbidden) === undefined,
        };
      }),
    ).toEqual(
      branches.map((b) => ({
        kind: b.kind,
        requiredCoordinate: true,
        coordinateIsNumber: true,
        otherAbsent: true,
      })),
    );

    // Positive control. The set assertion above pins the kinds, and this pins
    // that both coordinate names appear somewhere in the union, which is what
    // AC-75 asks for across it rather than per branch.
    expect({
      some_branch_carries_resultIndex: branches.some((b) => propOf(b, 'resultIndex')),
      some_branch_carries_streamIndex: branches.some((b) => propOf(b, 'streamIndex')),
    }).toEqual({ some_branch_carries_resultIndex: true, some_branch_carries_streamIndex: true });
  });

  // @spec spec-vscode
  // @ac AC-75
  it('[spec-vscode/AC-75] the client returns the shared type and no unknown cast bridges it', () => {
    const clientText = fs.readFileSync(CLIENT_PATH, 'utf-8');
    const extensionText = fs.readFileSync(EXTENSION_PATH, 'utf-8');

    // Scoped to the two files the criterion names. A tree-wide search would
    // hit historical mentions in comments and fixtures elsewhere.
    expect({
      client_declares_CoverageResult: /\binterface\s+CoverageResult\b/.test(clientText),
      client_returns_the_shared_type: /coverage\(\)\s*:\s*Promise<CoverageReport>/.test(clientText),
      client_imports_CoverageReport: /import\s[^;]*\bCoverageReport\b[^;]*from\s+'\.\/types'/.test(
        clientText,
      ),
      extension_bridges_with_an_unknown_cast: /as\s+unknown\s+as\s+CoverageReport/.test(
        extensionText,
      ),
    }).toEqual({
      client_declares_CoverageResult: false,
      client_returns_the_shared_type: true,
      client_imports_CoverageReport: true,
      extension_bridges_with_an_unknown_cast: false,
    });
  });
});

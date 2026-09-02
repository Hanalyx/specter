// nullDocuments.test.ts
//
// @spec spec-vscode
//
// C-31: every array-valued field of a `--json` document MUST be an array by
// the time any consumer in the extension sees it. Go marshals a nil slice as
// `null` and drops an `omitempty` field entirely, so a workspace with nothing
// wrong in it is the one shape no fixture in this suite has ever produced.
//
// C-32: every VS Code range derived from a CLI diagnostic MUST carry finite,
// non-negative integers in all four positions, whatever the diagnostic omits.
// A schema validation error carries no position at all, and a check diagnostic
// carries none in any form.
//
// Expected behavior comes from spec-vscode C-31, C-32, and AC-61 through AC-73
// at commit 2307535. Every document fixture below is the byte string the spec
// captured from the real binary; each is parsed here rather than retyped.
//
// Four range-end assertions come from spec-vscode 1.10.0 at commit 441d3f4
// instead: AC-64, AC-65, AC-66, and AC-71 stated no end, or stated one the
// amendment overturned. C-32 now decides all three sites at once, so a range
// that reaches a DiagnosticCollection ends on its start line at
// `Number.MAX_SAFE_INTEGER` (SP-SP-034). Nothing else in this file changed.
//
// Three layers, because the on-save handler where the defect physically lives
// is unreachable from this suite (nothing imports `../extension`):
//   - the client, driven through a stubbed child process (AC-61, 68, 69, 70),
//     following the `clientExitCode.test.ts` precedent;
//   - the pure builders in `diagnostics.ts` (AC-62 through AC-66, AC-71);
//   - a static read of `types.ts` (AC-67).
//
// AC-72 and AC-73 cross two of those layers: they read the declared members
// out of `types.ts` and then assert them against the object the client returns
// for the CLI's own document.
//
// Fixtures that violate the current declarations in `types.ts` are cast at the
// call site. The casts are deliberate: red must come from a runtime assertion,
// never from tsc refusing to compile the file, because a compile error would
// take down the criteria that are expected to pass along with the rest.

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// Used by the AC-67 static test only, to read the declared types without
// executing them. typescript is already a devDependency (ts-jest).
import * as ts from 'typescript';

// ---------------------------------------------------------------------------
// Scripted CLI
//
// The same harness `clientExitCode.test.ts` established, duplicated rather
// than imported: that file exports nothing, and editing a frozen precedent to
// share a mock is a worse trade than a second copy. Exit code and stdout vary
// independently, so a client that keyed on the exit code and a client that
// read the document stay distinguishable.
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
import { buildDiagnostics, buildCoverageParseDiagnostics, DiagnosticReplacer } from '../diagnostics';
import type { SpecterCheckDiagnostic } from '../types';

// ---------------------------------------------------------------------------
// Documents
//
// Byte for byte, the `inputs` blocks of AC-61 through AC-71, which the spec
// captured from the real binary. Each fixture carries markers unique to itself
// (spec id, constraint id, file path, error type) so a swapped fixture shows
// up in the failure message rather than passing on a coincidence.
// ---------------------------------------------------------------------------

/** AC-61, AC-62. `check --json` on a workspace with nothing wrong in it. */
const CHECK_DOC_NULL_DIAGNOSTICS =
  '{"diagnostics": null, "summary": {"errors": 0, "warnings": 0, "info": 0}}';

/** AC-62, AC-70. `parse --json` for a file that failed schema validation. */
const PARSE_DOC_TWO_ERRORS = `{"errors": [
  {"path": "spec", "type": "required",
   "message": "Missing required field 'objective'. Add an objective block with a summary"},
  {"path": "spec", "type": "additionalProperties",
   "message": "Unknown field 'trust_level'. Remove it or check for a typo in the field name."}
], "file": "specs/b.spec.yaml", "ok": false, "value": null}`;

/** AC-63. `parse --json` for a spec that parses cleanly. */
const PARSE_DOC_NULL_ERRORS =
  '{"errors": null, "file": "specs/o.spec.yaml", "ok": true, "value": {"id": "spec-o"}}';

/** AC-63. `check --json` reporting two error-severity diagnostics. */
const CHECK_DOC_TWO_DIAGNOSTICS = `{"diagnostics": [
  {"kind": "orphan_constraint", "severity": "error", "spec_id": "spec-o",
   "constraint_id": "C-02", "constraint_type": "technical",
   "message": "Constraint C-02 in \\"spec-o\\" is not referenced by any acceptance criterion"},
  {"kind": "orphan_constraint", "severity": "error", "spec_id": "spec-o",
   "constraint_id": "C-03", "constraint_type": "technical",
   "message": "Constraint C-03 in \\"spec-o\\" is not referenced by any acceptance criterion"}
], "summary": {"errors": 2, "warnings": 0, "info": 0}}`;

/** AC-68. `coverage --json` where every spec in the workspace failed to parse. */
const COVERAGE_DOC_NULL_ENTRIES = `{"entries": null,
 "summary": {"total_specs": 0, "fully_covered": 0, "partially_covered": 0,
             "uncovered": 0, "passing": 0, "failing": 0},
 "parse_errors": [
   {"file": "specs/b.spec.yaml", "path": "spec", "type": "required",
    "message": "Missing required field 'objective'. Add an objective block with a summary"},
   {"file": "specs/c.spec.yaml", "path": "", "type": "yaml_syntax",
    "message": "yaml: line 3: mapping values are not allowed in this context", "line": 3}
 ],
 "spec_candidates_count": 2}`;

/** AC-69. `coverage --json` where nothing failed to parse, so the field is gone. */
const COVERAGE_DOC_NO_PARSE_ERRORS = `{"entries": [
   {"spec_id": "spec-a", "tier": 3, "total_acs": 1, "covered_acs": [],
    "uncovered_acs": ["AC-01"], "coverage_pct": 0, "threshold": 50,
    "passes_threshold": false, "test_files": [], "spec_file": "specs/a.spec.yaml"}
 ],
 "summary": {"total_specs": 1, "fully_covered": 0, "partially_covered": 0,
             "uncovered": 1, "passing": 0, "failing": 1},
 "spec_candidates_count": 1}`;

/** AC-64. A schema validation error: `path`, `type`, `message`, no position. */
const PARSE_ERROR_NO_POSITION = `{"path": "spec", "type": "required",
 "message": "Missing required field 'objective'. Add an objective block with a summary"}`;

/** AC-65. A YAML syntax error: the only parse error carrying a `line`. */
const PARSE_ERROR_LINE_NO_COLUMN = `{"path": "", "type": "yaml_syntax",
 "message": "yaml: line 3: mapping values are not allowed in this context",
 "line": 3}`;

/** AC-66. A check diagnostic, which carries no position of any kind. */
const CHECK_DIAGNOSTIC_NO_POSITION = `{"kind": "orphan_constraint", "severity": "error", "spec_id": "spec-o",
 "constraint_id": "C-02", "constraint_type": "technical",
 "message": "Constraint C-02 in \\"spec-o\\" is not referenced by any acceptance criterion"}`;

/** AC-72. `check --json` reporting one error-severity diagnostic. */
const CHECK_DOC_ONE_DIAGNOSTIC = `{"diagnostics": [
  {"kind": "orphan_constraint", "severity": "error", "spec_id": "spec-o",
   "constraint_id": "C-02", "constraint_type": "technical",
   "message": "Constraint C-02 in \\"spec-o\\" is not referenced by any acceptance criterion"}
], "summary": {"errors": 1, "warnings": 0, "info": 0}}`;

/** AC-73. `parse --json` for a schema validation failure, the shape that omits the most. */
const PARSE_DOC_ONE_ERROR = `{"errors": [
  {"path": "spec", "type": "required",
   "message": "Missing required field 'objective'. Add an objective block with a summary"}
], "file": "specs/b.spec.yaml", "ok": false, "value": null}`;

/** AC-71. A coverage parse error with no position, which is the common case. */
const COVERAGE_PARSE_ERROR_NO_POSITION = `{"file": "specs/b.spec.yaml", "path": "spec", "type": "required",
 "message": "Missing required field 'objective'. Add an objective block with a summary"}`;

// ---------------------------------------------------------------------------
// Result readers
//
// The client's return shapes are not read from its source. Each reader accepts
// the forms the CLI document can legitimately take on the way out and fails
// loudly on anything else, so an unreachable field reads as a named finding
// rather than as `undefined` quietly satisfying an assertion.
// ---------------------------------------------------------------------------

type Loose = Record<string, unknown>;

/** The value occupying the `diagnostics` role in whatever `check()` returned. */
function diagnosticsFieldOf(result: unknown): unknown {
  if (Array.isArray(result)) {
    return result;
  }
  return ((result ?? {}) as Loose).diagnostics;
}

/**
 * The diagnostics `check()` returned, as an array. Accepts either the document
 * or its payload array, because whether the client unwraps is not what AC-72
 * is about. Fails loudly when neither is there.
 */
function checkDiagnosticsOf(result: unknown): Loose[] {
  const value = diagnosticsFieldOf(result);
  if (Array.isArray(value)) {
    return value as Loose[];
  }
  throw new Error(`check() result carried no "diagnostics" array: ${JSON.stringify(result)}`);
}

/** The errors `parse()` returned, as an array. Same tolerance, same reason. */
function parseErrorsOf(result: unknown): Loose[] {
  if (Array.isArray(result)) {
    return result as Loose[];
  }
  const r = (result ?? {}) as Loose;
  if (Array.isArray(r.errors)) {
    return r.errors as Loose[];
  }
  throw new Error(`parse() result carried no "errors" array: ${JSON.stringify(result)}`);
}

/** The `coverage()` document. Fails loudly if the report itself is missing. */
function coverageDocOf(result: unknown): Loose {
  if (result && typeof result === 'object' && !Array.isArray(result)) {
    return result as Loose;
  }
  throw new Error(`coverage() returned no report document: ${JSON.stringify(result)}`);
}

/**
 * The `parse()` document. AC-70 reads `ok` and `value`, which only exist if
 * the client returns the document rather than its `errors` array. A bare array
 * here is a finding about the layer the criterion was assigned to, so it is
 * reported as such rather than skipped past.
 */
function parseDocOf(result: unknown): Loose {
  if (result && typeof result === 'object' && !Array.isArray(result)) {
    const r = result as Loose;
    if ('ok' in r || 'value' in r) {
      return r;
    }
  }
  throw new Error(
    `parse() returned no document carrying ok or value, so AC-70 is not ` +
    `observable at the client layer: ${JSON.stringify(result)}`,
  );
}

/** First key present, so a client that rewrites snake_case is not penalized. */
const either = (o: Loose, ...keys: string[]): unknown => {
  for (const k of keys) {
    if (o[k] !== undefined) {
      return o[k];
    }
  }
  return undefined;
};

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
  workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-nulldocs-'));
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
// AC-61: the clean-workspace save, end to end across client and replacer.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-61
describe('C-31 check --json on a workspace with nothing wrong in it', () => {
  it('[spec-vscode/AC-61] consumes the null-diagnostics document and clears the stale diagnostic instead of throwing', async () => {
    const client = makeClient();
    const uri = path.join(workspaceDir, 'specs', 'clean.spec.yaml');

    // The collection as the previous save left it: one diagnostic on screen.
    const store = new Map<string, unknown>([[uri, [{ message: 'stale error from the previous save' }]]]);
    expect(store.has(uri)).toBe(true);

    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_NULL_DIAGNOSTICS });

    // First, the trap C-31 names by hand. A null-array document is parseable,
    // so it takes the C-30 success path. A fix that routed it to the
    // no-document failure path would leave the Problems panel showing that
    // stale diagnostic forever, and would satisfy "did not throw" while doing
    // it. Rejection is checked before anything is read off the result.
    const outcome = await client.check().then(
      (value) => ({ resolved: true, value }),
      (reason: unknown) => ({ resolved: false, value: reason }),
    );
    expect({ resolved: outcome.resolved }).toEqual({ resolved: true });

    // Second, the normalization: `null` on the wire is an array on the way out.
    const diagnostics = diagnosticsFieldOf(outcome.value);
    expect(Array.isArray(diagnostics)).toBe(true);
    expect(diagnostics).toHaveLength(0);

    // Third, the builder consumes what the client actually returned, not a
    // hand-made empty array, and produces nothing without throwing.
    const built = buildDiagnostics({
      parseErrors: [],
      checkDiagnostics: diagnostics as SpecterCheckDiagnostic[],
    });
    expect(built).toHaveLength(0);

    // Fourth, the collection is replaced with the empty set, so the stale
    // diagnostic is gone. This is the user-visible half of the criterion.
    const replacer = new DiagnosticReplacer({
      set: (u, d) => { store.set(u, d); },
      delete: (u) => { store.delete(u); },
    });
    replacer.replace(uri, built);
    expect(store.has(uri)).toBe(false);

    // Control: the document came from an actual invocation of the CLI, rather
    // than from a short circuit somewhere earlier.
    expect(mockCli.jsonInvocations('check')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-62 / AC-63: either half being null is independently sufficient to throw,
// so each direction gets its own criterion. Both halves reach the builder
// exactly as parsed from stdout, which pins the builder's own tolerance rather
// than the client's normalization.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-62
describe('C-31 builder with a null check half', () => {
  it('[spec-vscode/AC-62] a null diagnostics field does not suppress the two parse errors beside it', () => {
    const checkHalf = JSON.parse(CHECK_DOC_NULL_DIAGNOSTICS).diagnostics;
    const parseHalf = JSON.parse(PARSE_DOC_TWO_ERRORS).errors;

    // Precondition, not an assertion about the code: the fixtures really are
    // the two shapes this criterion is about.
    expect(checkHalf).toBeNull();
    expect(parseHalf).toHaveLength(2);

    const built = buildDiagnostics({
      parseErrors: parseHalf,
      checkDiagnostics: checkHalf,
    });

    expect(built).toHaveLength(2);
    // Bound by index, in the order the document lists them. Two distinct
    // messages, so reading `[0]` twice cannot pass.
    expect(built[0].message).toContain("Missing required field 'objective'");
    expect(built[1].message).toContain("Unknown field 'trust_level'");
  });
});

// @spec spec-vscode
// @ac AC-63
describe('C-31 builder with a null parse half', () => {
  it('[spec-vscode/AC-63] a null errors field does not suppress the two check diagnostics beside it', () => {
    const parseHalf = JSON.parse(PARSE_DOC_NULL_ERRORS).errors;
    const checkHalf = JSON.parse(CHECK_DOC_TWO_DIAGNOSTICS).diagnostics;

    expect(parseHalf).toBeNull();
    expect(checkHalf).toHaveLength(2);

    const built = buildDiagnostics({
      parseErrors: parseHalf,
      checkDiagnostics: checkHalf,
    });

    expect(built).toHaveLength(2);
    expect(built[0].message).toContain('Constraint C-02');
    expect(built[1].message).toContain('Constraint C-03');
  });
});

// ---------------------------------------------------------------------------
// AC-64 / AC-65 / AC-66: C-32 positions. Absent fields become 0, never
// arithmetic on `undefined`.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-64
describe('C-32 a parse error with no position', () => {
  it('[spec-vscode/AC-64] a schema error carrying neither line nor column lands at the start of the file', () => {
    const err = JSON.parse(PARSE_ERROR_NO_POSITION);
    // The shape every schema validation error the CLI emits actually has.
    expect(Object.keys(err).sort()).toEqual(['message', 'path', 'type']);

    const built = buildDiagnostics({
      parseErrors: [err],
      checkDiagnostics: [],
    });

    expect(built).toHaveLength(1);
    expect(rangeHealth(built[0])).toEqual(ALL_FOUR_HEALTHY);
    expect(built[0].range.start).toEqual({ line: 0, character: 0 });
    expect(built[0].range.end).toEqual({ line: 0, character: Number.MAX_SAFE_INTEGER });
    // The diagnostic is still the one built from this error, not a placeholder
    // that happens to sit at 0,0.
    expect(built[0].message).toContain("Missing required field 'objective'");
  });
});

// @spec spec-vscode
// @ac AC-65
describe('C-32 a parse error with a line and no column', () => {
  it('[spec-vscode/AC-65] a YAML syntax error converts its 1-based line and defaults the absent column', () => {
    const err = JSON.parse(PARSE_ERROR_LINE_NO_COLUMN);
    // The line is present and the column is absent, and the two are what this
    // criterion separates. An implementation that defaults every position to 0
    // satisfies AC-64 and fails here.
    expect(err.line).toBe(3);
    expect('column' in err).toBe(false);
    expect('col' in err).toBe(false);

    const built = buildDiagnostics({
      parseErrors: [err],
      checkDiagnostics: [],
    });

    expect(built).toHaveLength(1);
    expect(rangeHealth(built[0])).toEqual(ALL_FOUR_HEALTHY);
    // CLI line 3 is 1-based; VS Code is 0-based. The absent column is 0.
    expect(built[0].range.start).toEqual({ line: 2, character: 0 });
    expect(built[0].range.end).toEqual({ line: 2, character: Number.MAX_SAFE_INTEGER });
    expect(built[0].message).toContain('mapping values are not allowed');
  });
});

// @spec spec-vscode
// @ac AC-66
describe('C-32 a check diagnostic, which carries no position at all', () => {
  it('[spec-vscode/AC-66] a check diagnostic lands at the start of the file and keeps its error severity', () => {
    const diag = JSON.parse(CHECK_DIAGNOSTIC_NO_POSITION);
    // No position in any form. `line` is declared on SpecterCheckDiagnostic
    // and the CLI never emits it; this is the criterion that fails if the
    // declared-but-never-emitted field is read.
    expect('line' in diag).toBe(false);
    expect('column' in diag).toBe(false);

    const built = buildDiagnostics({
      parseErrors: [],
      checkDiagnostics: [diag],
    });

    expect(built).toHaveLength(1);
    expect(rangeHealth(built[0])).toEqual(ALL_FOUR_HEALTHY);
    expect(built[0].range.start).toEqual({ line: 0, character: 0 });
    expect(built[0].range.end).toEqual({ line: 0, character: Number.MAX_SAFE_INTEGER });
    expect(built[0].severity).toBe('error');
    expect(built[0].message).toContain('Constraint C-02');
  });
});

// ---------------------------------------------------------------------------
// AC-67: the declared types do not assert what the CLI does not emit.
//
// Scope is exactly what the criterion states: two interface declarations in
// one file, three claims about their members. It says nothing about any other
// declaration, any other file, or any read site, and nothing here should be
// read as covering those.
// ---------------------------------------------------------------------------

interface DeclaredMember {
  name: string;
  optional: boolean;
}

/** `types.ts` as the compiler sees it, parsed but never executed. */
function typesSourceFile(): ts.SourceFile {
  const typesPath = path.resolve(__dirname, '..', 'types.ts');
  const src = fs.readFileSync(typesPath, 'utf-8');
  return ts.createSourceFile(typesPath, src, ts.ScriptTarget.ES2020, true);
}

/** Members of `name`, as (memberName, optional) pairs, for every declaration of it. */
function interfaceMembers(sf: ts.SourceFile, name: string): DeclaredMember[] | null {
  const decls = sf.statements.filter(
    (s): s is ts.InterfaceDeclaration => ts.isInterfaceDeclaration(s) && s.name.text === name,
  );
  if (decls.length !== 1) {
    return null;
  }
  return decls[0].members
    .filter(ts.isPropertySignature)
    .map((m) => {
      const memberName = m.name;
      const text =
        ts.isIdentifier(memberName) || ts.isStringLiteral(memberName) ? memberName.text : '';
      return { name: text, optional: m.questionToken !== undefined };
    });
}

// @spec spec-vscode
// @ac AC-67
describe('C-32 declared types for CLI documents', () => {
  it('[spec-vscode/AC-67] SpecterParseError and SpecterCheckDiagnostic declare no field the CLI never emits and require no field the CLI may omit', () => {
    const sf = typesSourceFile();

    const parseError = interfaceMembers(sf, 'SpecterParseError');
    const checkDiagnostic = interfaceMembers(sf, 'SpecterCheckDiagnostic');

    // Positive control for the two absence claims below. Each interface was
    // located exactly once and its members were read. Without this, renaming
    // an interface would satisfy "col is not declared" with nothing examined.
    expect({
      parseErrorFound: parseError !== null,
      checkDiagnosticFound: checkDiagnostic !== null,
    }).toEqual({ parseErrorFound: true, checkDiagnosticFound: true });
    expect(parseError!.length).toBeGreaterThan(0);
    expect(checkDiagnostic!.length).toBeGreaterThan(0);

    const parseNames = parseError!.map((m) => m.name);
    const checkNames = checkDiagnostic!.map((m) => m.name);

    // Neither `col` nor `code`: the CLI names the corresponding fields
    // `column` and `type`.
    expect(parseNames.filter((n) => n === 'col' || n === 'code')).toEqual([]);

    // `line` is declared required on neither. The CLI omits it on every schema
    // error and emits no position at all on a check diagnostic. Optional or
    // absent both satisfy this; required does not.
    const requiredLine = [
      ...parseError!.filter((m) => m.name === 'line' && !m.optional).map(() => 'SpecterParseError'),
      ...checkDiagnostic!.filter((m) => m.name === 'line' && !m.optional).map(() => 'SpecterCheckDiagnostic'),
    ];
    expect(requiredLine).toEqual([]);

    // `message` is declared on both, because the CLI emits it on every error
    // and every diagnostic. This claim also proves member reading works, so
    // the two absence claims above are not reading an empty list.
    expect({
      onParseError: parseNames.includes('message'),
      onCheckDiagnostic: checkNames.includes('message'),
    }).toEqual({ onParseError: true, onCheckDiagnostic: true });
  });
});

// ---------------------------------------------------------------------------
// AC-68 / AC-69: the two coverage shapes, null and absent, which are different
// inputs and reach the client by different routes.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-68
describe('C-31 coverage --json where no spec parsed', () => {
  it('[spec-vscode/AC-68] null entries become an empty array and both parse errors survive', async () => {
    const client = makeClient();
    mockCli.script('coverage', { exitCode: 1, stdout: COVERAGE_DOC_NULL_ENTRIES });

    const outcome = await client.coverage().then(
      (value) => ({ resolved: true, value }),
      (reason: unknown) => ({ resolved: false, value: reason }),
    );
    // A null-array document is parseable, so it is a success under C-30.
    expect({ resolved: outcome.resolved }).toEqual({ resolved: true });

    const report = coverageDocOf(outcome.value);

    const entries = report.entries;
    expect(Array.isArray(entries)).toBe(true);
    expect(entries).toHaveLength(0);

    // Two failing files, so the count is pinned rather than satisfied by
    // reading the first element. Each is bound to its own position, by
    // containment rather than equality: whether the client resolves a relative
    // parse-error path against the folder root is a separate question from
    // C-31, and this criterion should not turn red on the answer.
    const parseErrors = either(report, 'parseErrors', 'parse_errors') as Loose[];
    expect(Array.isArray(parseErrors)).toBe(true);
    expect(parseErrors).toHaveLength(2);
    expect(String(parseErrors[0].file)).toContain('specs/b.spec.yaml');
    expect(String(parseErrors[1].file)).toContain('specs/c.spec.yaml');

    // Control: the report came from an actual invocation.
    expect(mockCli.jsonInvocations('coverage')).toHaveLength(1);
  });
});

// @spec spec-vscode
// @ac AC-69
describe('C-31 coverage --json where every spec parsed', () => {
  it('[spec-vscode/AC-69] an omitted parse_errors field becomes an empty array while the entry survives', async () => {
    const client = makeClient();
    // The field is gone, not null. Absent and null are different inputs, and
    // this is the shape a healthy workspace produces on every save.
    expect('parse_errors' in JSON.parse(COVERAGE_DOC_NO_PARSE_ERRORS)).toBe(false);
    mockCli.script('coverage', { exitCode: 0, stdout: COVERAGE_DOC_NO_PARSE_ERRORS });

    const report = coverageDocOf(await client.coverage());

    const parseErrors = either(report, 'parseErrors', 'parse_errors');
    expect(Array.isArray(parseErrors)).toBe(true);
    expect(parseErrors).toHaveLength(0);

    // Positive control for that emptiness claim: the document really did
    // arrive and really did carry its one entry, so an empty parseErrors is
    // normalization rather than a report that never got parsed.
    const entries = report.entries as Loose[];
    expect(entries).toHaveLength(1);
    expect(either(entries[0], 'specID', 'spec_id')).toBe('spec-a');
  });
});

// ---------------------------------------------------------------------------
// AC-70: `value` is null whenever `ok` is false, and that is not an array
// field. Normalization must not reach it.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-70
describe('C-31 parse --json for a spec that failed to parse', () => {
  it('[spec-vscode/AC-70] a null value is returned as null while both errors stay iterable', async () => {
    const client = makeClient();
    mockCli.script('parse', { exitCode: 0, stdout: PARSE_DOC_TWO_ERRORS });

    const doc = parseDocOf(await client.parse(path.join('specs', 'b.spec.yaml')));

    expect(doc.ok).toBe(false);
    // Null, not an empty object and not undefined. `value` is not an
    // array-valued field, so normalizing it would be a fabrication.
    expect(doc.value).toBeNull();

    const errors = doc.errors as Loose[];
    expect(Array.isArray(errors)).toBe(true);
    expect(errors).toHaveLength(2);
    expect(errors[0].message).toContain("Missing required field 'objective'");
    expect(errors[1].message).toContain("Unknown field 'trust_level'");

    // Control: the document came from an actual invocation.
    expect(mockCli.jsonInvocations('parse')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-71: the second consumer of CLI positions.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-71
describe('C-32 a coverage parse error with no position', () => {
  it('[spec-vscode/AC-71] a coverage parse error carrying neither line nor column lands at the start of the file it names', () => {
    const err = JSON.parse(COVERAGE_PARSE_ERROR_NO_POSITION);
    expect('line' in err).toBe(false);
    expect('column' in err).toBe(false);

    const out = buildCoverageParseDiagnostics([err]);

    expect(out).toHaveLength(1);
    // The diagnostic still names its file, which is how AC-34 routes it to a
    // Problems-panel entry at all.
    expect(out[0].file).toBe('specs/b.spec.yaml');
    expect(out[0].diagnostics).toHaveLength(1);
    expect(rangeHealth(out[0].diagnostics[0])).toEqual(ALL_FOUR_HEALTHY);
    expect(out[0].diagnostics[0].range.start).toEqual({ line: 0, character: 0 });
    expect(out[0].diagnostics[0].range.end).toEqual({ line: 0, character: Number.MAX_SAFE_INTEGER });
    expect(out[0].diagnostics[0].message).toContain("Missing required field 'objective'");
  });
});

// ---------------------------------------------------------------------------
// AC-72 / AC-73: the declaration and the object agree.
//
// Both criteria state one rule: every member declared required on the
// interface is present, and not `undefined`, on the object the client returns
// for the CLI's own document. The member list is read out of `types.ts`
// through the same compiler API pass AC-67 uses, never hardcoded, so the
// assertion stays true when the interface changes and passes under any correct
// fix: converting the document's keys, renaming the declared members, stamping
// a file path, or marking a member optional. Which side moves is not the
// contract, following the AC-52 precedent.
//
// Kept as two criteria because the two interfaces are fed by different
// commands, and a fix applied to one has repeatedly been left off the other.
// ---------------------------------------------------------------------------

/** `specID`, `spec_id`, and `specId` all answer `specid`. */
function normalizeName(name: string): string {
  return name.replace(/[^a-zA-Z0-9]/g, '').toLowerCase();
}

/** Members declared required, which are the ones the returned object must carry. */
function requiredNames(members: DeclaredMember[]): string[] {
  return members.filter((m) => !m.optional).map((m) => m.name);
}

/**
 * The single declared member that names `cliKey`, under any casing or
 * separator, or null when it is not exactly one. Used to read a value back
 * under the name a consumer typed against the interface would use.
 */
function declaredNameFor(members: DeclaredMember[], cliKey: string): string | null {
  const hits = members.filter((m) => normalizeName(m.name) === normalizeName(cliKey));
  return hits.length === 1 ? hits[0].name : null;
}

/** Declared required names that are absent, or present as `undefined`, on `obj`. */
function missingRequired(obj: Loose, required: string[]): string[] {
  return required.filter((n) => !(n in obj) || obj[n] === undefined);
}

// @spec spec-vscode
// @ac AC-72
describe('C-32 SpecterCheckDiagnostic against the document check --json returns', () => {
  it('[spec-vscode/AC-72] every member declared required on the interface is present on the returned diagnostic', async () => {
    const members = interfaceMembers(typesSourceFile(), 'SpecterCheckDiagnostic');
    expect(members).not.toBeNull();
    const required = requiredNames(members!);

    // Control for the emptiness claim below. The interface was located and it
    // really does require members, so "nothing missing" cannot be an empty
    // list checked against an empty object.
    expect(required.length).toBeGreaterThan(0);

    const client = makeClient();
    // Exit 1, which is what the CLI does on an error-severity diagnostic. The
    // document and the exit code vary independently of each other.
    mockCli.script('check', { exitCode: 1, stdout: CHECK_DOC_ONE_DIAGNOSTIC });

    const diagnostics = checkDiagnosticsOf(await client.check());
    expect(diagnostics).toHaveLength(1);

    // The failure message names which declared members the object lacks.
    expect(missingRequired(diagnostics[0], required)).toEqual([]);

    // The spec id is not merely present. It is readable under the name the
    // interface declares for it, and it carries the value the CLI emitted, so
    // a member that exists but holds something else cannot pass.
    const specIdName = declaredNameFor(members!, 'spec_id');
    expect(specIdName).not.toBeNull();
    expect(diagnostics[0][specIdName!]).toBe('spec-o');

    // Control: the document came from an actual invocation.
    expect(mockCli.jsonInvocations('check')).toHaveLength(1);
  });
});

// @spec spec-vscode
// @ac AC-73
describe('C-32 SpecterParseError against the document parse --json returns', () => {
  it('[spec-vscode/AC-73] every member declared required on the interface is present on each returned error', async () => {
    const members = interfaceMembers(typesSourceFile(), 'SpecterParseError');
    expect(members).not.toBeNull();
    const required = requiredNames(members!);
    expect(required.length).toBeGreaterThan(0);

    // The shape a schema validation failure takes, which is most parse errors
    // and the one that omits the most: no position, and the file name on the
    // document rather than on the error inside it.
    const emitted = JSON.parse(PARSE_DOC_ONE_ERROR);
    expect(Object.keys(emitted.errors[0]).sort()).toEqual(['message', 'path', 'type']);

    const client = makeClient();
    mockCli.script('parse', { exitCode: 0, stdout: PARSE_DOC_ONE_ERROR });

    const errors = parseErrorsOf(await client.parse(path.join('specs', 'b.spec.yaml')));
    expect(errors).toHaveLength(1);

    expect(missingRequired(errors[0], required)).toEqual([]);

    // Readable under its declared name, and carrying the CLI's own text.
    const messageName = declaredNameFor(members!, 'message');
    expect(messageName).not.toBeNull();
    expect(errors[0][messageName!]).toBe(
      "Missing required field 'objective'. Add an objective block with a summary",
    );

    // Control: the document came from an actual invocation.
    expect(mockCli.jsonInvocations('parse')).toHaveLength(1);
  });
});

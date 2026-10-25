// clientExitCode.test.ts
//
// @spec spec-vscode
//
// C-30: a non-zero process exit MUST NOT by itself discard a document the
// client can parse. The failure predicate is the absence of a parseable
// document, not the exit code.
//
// The CLI is mocked at the child_process boundary, so a run can exit non-zero
// and still write a complete JSON document to stdout. That combination is what
// the suite could never produce before: 268 tests passed while `check --json`
// began exiting 1 on ordinary error-severity diagnostics, because no test ever
// drove a non-zero exit.
//
// These tests drive the real SpecterClient. They do not read it. Expected
// behavior comes from spec-vscode C-30 and AC-56 through AC-60 at commit
// 1a4a588, and the document shapes come from the CLI's own JSON contracts
// (internal/checker.CheckResult, internal/parser.ParseResult,
// internal/coverage.CoverageReport).
//
// What is asserted here is the client boundary. The handler-side half of
// AC-56, AC-59, and AC-60 (the Output-channel line, the untouched
// DiagnosticCollection, the status bar error state, the absent notification)
// is not reachable from this layer: SpecterClient takes no sink, and those
// effects live behind the `vscode` runtime.

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// Used by the AC-58 static test only, to read the client module's call sites
// without executing it. typescript is already a devDependency (ts-jest).
import * as ts from 'typescript';

// ---------------------------------------------------------------------------
// Scripted CLI
//
// One script per subcommand, replaced between calls. Each script sets an exit
// code, a stdout payload, and optionally a spawn errno, and the three vary
// independently: no fixture derives its exit code from its document or its
// document from its exit code. That independence is the point. A client that
// keyed on the exit code and a client that keyed on the document are
// indistinguishable until the two disagree.
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

// The mock reproduces Node's own semantics rather than a plausible imitation,
// verified against Node 18 before it was written:
//   - callback execFile: cb(err, stdout, stderr), stdout still delivered on a
//     non-zero exit, err.code is the numeric exit code.
//   - promisify(execFile): resolves { stdout, stderr }, rejects with an error
//     carrying .code, .stdout, and .stderr. Node installs
//     util.promisify.custom on the real execFile, so the mock installs it too.
//     Without it, promisify would resolve stdout alone and a destructuring
//     client would break in a way real Node never produces.
//   - spawn failure: err.code is the errno string ENOENT or EACCES, the
//     message reads `spawn <path> <ERRNO>`, and stdout is empty.
// spawn is mocked alongside execFile so that an implementation that switches
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

// ---------------------------------------------------------------------------
// Documents
//
// Every fixture carries markers unique to itself (spec id, constraint id,
// file path, line, percentage) so a swapped fixture is visible in the failure
// message rather than passing on a coincidence.
// ---------------------------------------------------------------------------

/** `check --json` document reporting one error-severity diagnostic. */
const CHECK_DOC_WITH_ERROR = JSON.stringify(
  {
    diagnostics: [
      {
        kind: 'orphan_constraint',
        severity: 'error',
        message: "Constraint C-07 in 'billing' is not referenced by any acceptance criterion",
        spec_id: 'billing',
        constraint_id: 'C-07',
      },
    ],
    summary: { errors: 1, warnings: 0, info: 0 },
  },
  null,
  2,
);

/** `check --json` document reporting nothing at all. */
const CHECK_DOC_CLEAN = JSON.stringify(
  { diagnostics: [], summary: { errors: 0, warnings: 0, info: 0 } },
  null,
  2,
);

/** `parse --json` document for one file that failed schema validation. */
const PARSE_DOC_WITH_ERROR = JSON.stringify(
  {
    file: 'specs/auth.spec.yaml',
    ok: false,
    errors: [
      {
        path: 'spec.objective',
        type: 'required',
        message: "missing property 'summary'",
        line: 7,
        column: 3,
      },
    ],
  },
  null,
  2,
);

/**
 * `coverage --json` document. `coverage_pct` is deliberately not the quotient
 * of `covered_acs` over `total_acs`: the client must report the number the CLI
 * computed, and a client that recomputed it would return 20 here.
 */
const COVERAGE_DOC = JSON.stringify(
  {
    entries: [
      {
        spec_id: 'payments',
        tier: 2,
        total_acs: 5,
        covered_acs: ['AC-01'],
        uncovered_acs: ['AC-02', 'AC-03', 'AC-04', 'AC-05'],
        coverage_pct: 42,
        threshold: 80,
        passes_threshold: false,
        test_files: ['src/__tests__/payments.test.ts'],
        spec_file: 'specs/payments.spec.yaml',
      },
    ],
    summary: {
      total_specs: 1,
      passing: 0,
      failing: 1,
      fully_covered: 0,
      partially_covered: 1,
      uncovered: 0,
    },
    parse_errors: [],
    spec_candidates_count: 1,
  },
  null,
  2,
);

// ---------------------------------------------------------------------------
// Result readers
//
// The client's own return shapes are not read from its source, so each reader
// accepts the forms the CLI document can legitimately take on the way out (the
// raw document, or the document's payload array) and fails loudly on anything
// else. Field lookups accept either the CLI's snake_case key or the
// extension's camelCase equivalent, because whether the client rewrites keys
// is not what C-30 is about.
// ---------------------------------------------------------------------------

type Loose = Record<string, unknown>;

function payloadArray(result: unknown, key: string, label: string): Loose[] {
  if (Array.isArray(result)) {
    return result as Loose[];
  }
  const r = (result ?? {}) as Loose;
  if (Array.isArray(r[key])) {
    return r[key] as Loose[];
  }
  throw new Error(
    `${label} result carried no "${key}" array: ${JSON.stringify(result)}`,
  );
}

const checkDiagnosticsOf = (r: unknown) => payloadArray(r, 'diagnostics', 'check()');
const parseErrorsOf = (r: unknown) => payloadArray(r, 'errors', 'parse()');
const coverageEntriesOf = (r: unknown) => payloadArray(r, 'entries', 'coverage()');

const either = (o: Loose, ...keys: string[]): unknown => {
  for (const k of keys) {
    if (o[k] !== undefined) {
      return o[k];
    }
  }
  return undefined;
};

// ---------------------------------------------------------------------------
// Workspace
// ---------------------------------------------------------------------------

let workspaceDir: string;
let binaryPath: string;
let manifestPath: string;

beforeAll(() => {
  workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-exitcode-'));
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

const makeClient = (binary: string = binaryPath) =>
  new SpecterClient({
    binaryPath: binary,
    manifestPath,
    workspaceFolder: workspaceDir,
  });

// ---------------------------------------------------------------------------
// AC-56: a non-zero exit with a complete document publishes the same
// diagnostics a zero exit would.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-56
describe('C-30 check --json with a non-zero exit', () => {
  it('[spec-vscode/AC-56] keeps the document and yields the same diagnostics a zero exit yields', async () => {
    const client = makeClient();

    // Exit 1 with a complete document: the workspace this rule exists for.
    mockCli.script('check', { exitCode: 1, stdout: CHECK_DOC_WITH_ERROR });
    const onExit1 = await client.check();

    const diag = checkDiagnosticsOf(onExit1)[0];
    expect(checkDiagnosticsOf(onExit1)).toHaveLength(1);
    expect(diag.severity).toBe('error');
    expect(diag.kind).toBe('orphan_constraint');
    expect(either(diag, 'constraintID', 'constraint_id')).toBe('C-07');
    expect(either(diag, 'specID', 'spec_id')).toBe('billing');

    // The same document at exit 0. The exit code is not an input to what gets
    // published, so the two results must be indistinguishable.
    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_WITH_ERROR });
    const onExit0 = await client.check();
    expect(onExit1).toEqual(onExit0);

    // The other diagonal of the same matrix: a clean document at exit 1. A
    // client that read the exit code instead of the document would invent a
    // diagnostic here.
    mockCli.script('check', { exitCode: 1, stdout: CHECK_DOC_CLEAN });
    const cleanOnExit1 = await client.check();
    expect(checkDiagnosticsOf(cleanOnExit1)).toHaveLength(0);

    // Control for the three assertions above: the CLI really was invoked for
    // a document each time, rather than short-circuited somewhere earlier.
    expect(mockCli.jsonInvocations('check')).toHaveLength(3);
  });
});

// ---------------------------------------------------------------------------
// AC-57: the rule binds every --json invocation, not check alone.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-57
describe('C-30 across every --json invocation', () => {
  it('[spec-vscode/AC-57] parse and coverage also consume a document written on a non-zero exit', async () => {
    const client = makeClient();

    // parse --json does not exit non-zero on error-severity diagnostics yet
    // (SP-SP-022). The client has to be correct for the day it does, which is
    // the whole reason C-30 was scoped to every invocation rather than check.
    mockCli.script('parse', { exitCode: 1, stdout: PARSE_DOC_WITH_ERROR });
    const parsed = await client.parse(path.join('specs', 'auth.spec.yaml'));

    const parseError = parseErrorsOf(parsed)[0];
    expect(parseErrorsOf(parsed)).toHaveLength(1);
    expect(parseError.line).toBe(7);
    expect(either(parseError, 'type', 'code')).toBe('required');
    expect(parseError.message).toBe("missing property 'summary'");

    // coverage --json, same shape of failure, different command.
    mockCli.script('coverage', { exitCode: 1, stdout: COVERAGE_DOC });
    const report = await client.coverage();

    const entry = coverageEntriesOf(report)[0];
    expect(coverageEntriesOf(report)).toHaveLength(1);
    expect(either(entry, 'specID', 'spec_id')).toBe('payments');
    expect(entry.tier).toBe(2);
    // The CLI's number, not a recomputed one. 1 of 5 covered would be 20.
    expect(either(entry, 'coveragePct', 'coverage_pct')).toBe(42);

    // Control: both documents came from an actual invocation.
    expect(mockCli.jsonInvocations('parse')).toHaveLength(1);
    expect(mockCli.jsonInvocations('coverage')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// AC-58: static assertion over call sites, so the class stays closed.
//
// The behavioral tests above cover the three commands that exist today. They
// cannot cover a fourth added next year. This one can, and it is the criterion
// that would have caught SP-SP-023 at the source: v0.13 moved `coverage` to
// the tolerant path and left `parse` and `check` behind.
//
// Following the AC-52 precedent, the contract is the routing and not the name
// of any helper. Accepted forms:
//   1. A named invocation helper (method, function, or arrow constant) that
//      every --json call site calls.
//   2. Any other spelling of the same routing, as long as the path each
//      --json site calls deals in stdout, stderr, and an exit code.
// Rejected by construction: a promisified exec-family binding, which rejects
// on a non-zero exit and drops the document.
// ---------------------------------------------------------------------------

/** Node APIs that start a process. A path is built out of one of these. */
const CHILD_PROCESS_FNS = new Set([
  'exec', 'execFile', 'execSync', 'execFileSync', 'spawn', 'spawnSync',
]);

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

/** `foo(…)` and `x.foo(…)` both answer `foo`. */
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

function eachDescendant(root: ts.Node, visit: (n: ts.Node) => void): void {
  ts.forEachChild(root, (n) => {
    visit(n);
    eachDescendant(n, visit);
  });
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

/** `promisify(execFile)` and `util.promisify(cp.exec)`, in any spelling. */
function isPromisifiedExec(node: ts.Node): boolean {
  if (!ts.isCallExpression(node) || calleeName(node) !== 'promisify') {
    return false;
  }
  const arg = node.arguments[0];
  if (!arg) {
    return false;
  }
  const name = ts.isIdentifier(arg)
    ? arg.text
    : ts.isPropertyAccessExpression(arg)
      ? arg.name.text
      : '';
  return name === 'exec' || name === 'execFile';
}

// @spec spec-vscode
// @ac AC-58
describe('C-30 routing of --json call sites', () => {
  it('[spec-vscode/AC-58] no --json call site in the client module routes through a path that rejects on a non-zero exit', () => {
    const clientPath = path.resolve(__dirname, '..', 'client.ts');
    const src = fs.readFileSync(clientPath, 'utf-8');
    const sf = ts.createSourceFile(clientPath, src, ts.ScriptTarget.ES2020, true);

    // Every function-like in the module, indexed by the name it is called by.
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

    // Paths that reject on a non-zero exit by construction: a promisified
    // exec-family binding drops stdout into an error and throws the exit code.
    const rejecting = new Set<string>();
    eachDescendant(sf, (n) => {
      if ((ts.isVariableDeclaration(n) || ts.isPropertyDeclaration(n)) &&
          n.initializer && isPromisifiedExec(n.initializer) && ts.isIdentifier(n.name)) {
        rejecting.add(n.name.text);
      }
    });

    // Paths that start a process. One of these is what a --json site must use.
    const runners = new Set<string>();
    for (const fn of functions) {
      const name = functionName(fn);
      if (!name || rejecting.has(name)) {
        continue;
      }
      if (callsIn(fn).some((c) => CHILD_PROCESS_FNS.has(calleeName(c) ?? ''))) {
        runners.add(name);
      }
    }

    /**
     * Paths reachable from a call site, following named calls up to three
     * hops so that an implementation which builds its argument list in one
     * helper and invokes in another still resolves. Any implementation form
     * is accepted (AC-52 precedent); what is checked is where the call ends
     * up, not what anything is named.
     */
    const reachable = (fn: ts.Node, depth = 0): Set<string> => {
      const found = new Set<string>();
      if (depth > 3) {
        return found;
      }
      for (const call of callsIn(fn)) {
        const name = calleeName(call);
        if (!name) {
          continue;
        }
        if (runners.has(name) || rejecting.has(name)) {
          found.add(name);
          continue;
        }
        // The function starts the process itself rather than delegating. It
        // is then its own path, and is named as such so a failure message
        // reads "three paths" rather than "no path found".
        if (CHILD_PROCESS_FNS.has(name)) {
          found.add(functionName(fn) ?? 'inline');
          continue;
        }
        const callee = byName.get(name);
        if (callee && callee !== fn) {
          reachable(callee, depth + 1).forEach((r) => found.add(r));
        }
      }
      return found;
    };

    // Every `--json` argument in the module, with the function that passes it.
    const jsonSites: Array<{ owner: ts.Node | undefined; text: string }> = [];
    eachDescendant(sf, (n) => {
      if ((ts.isStringLiteral(n) || ts.isNoSubstitutionTemplateLiteral(n)) && n.text === '--json') {
        jsonSites.push({ owner: enclosingFunction(n), text: n.text });
      }
    });

    // Positive control for the absence claims below: the locator found real
    // call sites and placed each one. parse, check, and coverage each request
    // a document, so three is the floor. Zero sites would otherwise satisfy
    // "none of them rejects" without anything having been examined.
    expect(jsonSites.length).toBeGreaterThanOrEqual(3);
    expect(jsonSites.filter((s) => s.owner === undefined)).toEqual([]);
    expect(runners.size).toBeGreaterThanOrEqual(1);

    const routes = new Set<string>();
    for (const site of jsonSites) {
      const owner = site.owner!;
      const label = functionName(owner) ?? 'anonymous';
      const paths = reachable(owner);

      // The site routes somewhere identifiable.
      expect({ site: label, routes: paths.size > 0 }).toEqual({ site: label, routes: true });
      // And nowhere that rejects on a non-zero exit, whether through a named
      // binding or an inline `promisify(execFile)(…)`.
      expect({ site: label, rejecting: [...paths].filter((p) => rejecting.has(p)) })
        .toEqual({ site: label, rejecting: [] });
      expect({
        site: label,
        inlinePromisify: callsIn(owner).some((c) => isPromisifiedExec(c.expression)),
      }).toEqual({ site: label, inlinePromisify: false });

      [...paths].filter((p) => runners.has(p)).forEach((p) => routes.add(p));
    }

    // All --json sites share one path. This is what closes the class: a new
    // --json invocation added on a different path fails here even though no
    // behavioral test knows the new command exists. A split is exactly what
    // made SP-SP-023 possible, when v0.13 moved coverage and left parse and
    // check where they were.
    expect([...routes]).toHaveLength(1);

    // That shared path deals in all three of stdout, stderr, and the exit
    // code, rather than throwing the exit code and losing the document. Any
    // name, any shape.
    const sharedName = [...routes][0];
    const shared = byName.get(sharedName)!.getText(sf);
    expect(shared).toMatch(/stdout/);
    expect(shared).toMatch(/stderr/);
    expect(shared).toMatch(/\b(?:code|exitCode|status)\b/);
  });
});

// ---------------------------------------------------------------------------
// AC-59: no parseable document means failure, whatever the exit code.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-59
describe('C-30 invocations that produce no parseable document', () => {
  it('[spec-vscode/AC-59] treats unparseable stdout as failure at any exit code, and never as a clean workspace', async () => {
    const client = makeClient();

    // Positive control first. The same client, the same mock, a document it
    // can parse: this must succeed, and it must succeed with an empty
    // diagnostics list. Everything below is a rejection, so without this the
    // rejections could be a broken harness rather than a routed failure. It
    // also fixes what a clean workspace looks like, which is the shape a
    // failed run must never be mistaken for.
    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_CLEAN });
    const clean = await client.check();
    expect(checkDiagnosticsOf(clean)).toHaveLength(0);

    // The three cases the criterion names. Exit code and stdout vary
    // independently across them: case C is a healthy exit status with
    // unusable output, which is what makes parseability the predicate rather
    // than the exit code.
    const cases = [
      { name: 'exit 1, stdout empty', exitCode: 1, stdout: '', stderr: 'panic: runtime error\n' },
      { name: 'exit 1, stdout truncated', exitCode: 1, stdout: CHECK_DOC_WITH_ERROR.slice(0, 60) },
      { name: 'exit 0, stdout not JSON', exitCode: 0, stdout: 'Checked 12 specs, 0 problems found.\n' },
    ];

    for (const c of cases) {
      mockCli.script('check', { exitCode: c.exitCode, stdout: c.stdout, stderr: c.stderr });
      const outcome = await client.check().then(
        (value) => ({ case: c.name, resolved: true, value }),
        () => ({ case: c.name, resolved: false, value: undefined }),
      );
      // None of the three may come back as a document. A resolved result is
      // the failure this criterion exists to prevent: an empty one would
      // clear the collection and assert a clean workspace that was never
      // checked, and a non-empty one would be invented.
      expect({ case: outcome.case, resolved: outcome.resolved }).toEqual({
        case: c.name,
        resolved: false,
      });
    }

    // Control: the control run plus all three cases reached the CLI.
    expect(mockCli.jsonInvocations('check')).toHaveLength(4);
  });
});

// ---------------------------------------------------------------------------
// AC-60: a process that never spawned produced no document.
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-60
describe('C-30 spawn failures', () => {
  it('[spec-vscode/AC-60] routes ENOENT and EACCES to the failure path and names the errno that occurred', async () => {
    // Positive control: a healthy binary on the same harness succeeds, so the
    // two rejections below are the spawn failure and not the setup.
    const healthy = makeClient();
    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_CLEAN });
    expect(checkDiagnosticsOf(await healthy.check())).toHaveLength(0);

    // ENOENT: the resolved path does not exist on disk, and the mock reports
    // the errno the kernel would.
    const missing = path.join(workspaceDir, 'no-such-binary');
    expect(fs.existsSync(missing)).toBe(false);
    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_CLEAN, spawnError: 'ENOENT' });
    const enoent = await makeClient(missing).check().then(
      () => ({ resolved: true, reason: undefined as unknown }),
      (reason: unknown) => ({ resolved: false, reason }),
    );
    expect(enoent.resolved).toBe(false);
    expect(errnoTextOf(enoent.reason)).toContain('ENOENT');
    expect(errnoTextOf(enoent.reason)).not.toContain('EACCES');

    // EACCES: the path exists but is not executable. Varied independently of
    // the ENOENT case, and each case must name its own errno rather than a
    // shared "could not run the CLI".
    const notExecutable = path.join(workspaceDir, 'specter-not-executable');
    fs.writeFileSync(notExecutable, '#!/bin/sh\nexit 0\n', { mode: 0o644 });
    mockCli.script('check', { exitCode: 0, stdout: CHECK_DOC_CLEAN, spawnError: 'EACCES' });
    const eacces = await makeClient(notExecutable).check().then(
      () => ({ resolved: true, reason: undefined as unknown }),
      (reason: unknown) => ({ resolved: false, reason }),
    );
    expect(eacces.resolved).toBe(false);
    expect(errnoTextOf(eacces.reason)).toContain('EACCES');
    expect(errnoTextOf(eacces.reason)).not.toContain('ENOENT');
  });
});

/**
 * The text a failure handler has available to name a spawn error: the error's
 * own errno code plus its message. Which of the two carries it is the
 * client's choice; that it is there at all is AC-60.
 */
function errnoTextOf(reason: unknown): string {
  const e = (reason ?? {}) as { code?: unknown; message?: unknown };
  return `${String(e.code ?? '')} ${String(e.message ?? '')} ${String(reason)}`;
}

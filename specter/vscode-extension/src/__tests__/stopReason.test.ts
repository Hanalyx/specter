/**
 * stopReason.test.ts -- a refused coverage run is not an empty workspace,
 * spec-vscode 5.1.0 C-33 / AC-79, bugs/SP-SP-032.
 *
 * spec-coverage C-10 chose the additive shape: a refused run still emits
 * `entries: []` and a zeroed summary, as uncomputed placeholders, with
 * `stop_reason` the only field that separates them from a clean empty
 * workspace. This extension keys its empty state on exactly that pair, so the
 * producer's fix arrives here as a new way to be wrong.
 *
 * The binding evidence for this criterion lives here. The real-binary run in
 * stopReasonIntegration.test.ts is additional, and a criterion whose only
 * annotation owner can skip is a criterion that can go unrun.
 *
 * @spec spec-vscode
 * @ac AC-79
 */

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as ts from 'typescript';

import { SpecterClient, snakeToCamelCoverage } from '../client';
import { CoverageReportStore, buildCoverageTreeRoot } from '../coverage';
import type { CoverageReport } from '../types';

/**
 * The behavioral assertions read the new fields structurally rather than
 * through the types they will introduce: importing a type before it exists is
 * a compile failure, which takes the suite down and reads as a kill. The
 * declarations themselves are checked semantically further down, through the
 * TypeScript checker, so a missing one is an ordinary failure.
 */
type Loose = Record<string, unknown>;

/** The raw document the CLI writes for a refused run, before conversion. */
const rawRefused = {
  entries: [],
  summary: { total_specs: 0, passing: 0, failing: 0 },
  parse_errors: [],
  stop_reason: { kind: 'manifest_error', message: 'unknown settings key "settings.bogus_key"' },
};

const noopOps = {
  isAbsolute: (p: string) => p.startsWith('/'),
  join: (...parts: string[]) => parts.join('/'),
  normalize: (p: string) => p,
  resolve: (base: string, p: string) => (p.startsWith('/') ? p : `${base}/${p}`),
};

describe('spec-vscode/AC-79 a refused coverage run is not an empty workspace', () => {
  it('converts stop_reason to stopReason and does not leave the raw key behind', () => {
    // Through the real converter, not a hand-built object. A document handed
    // straight to a consumer would carry a field the client never produces,
    // and the assertion would pass while the product returned a raw key.
    const converted = snakeToCamelCoverage(rawRefused) as Loose;

    const reason = converted.stopReason as Loose | undefined;
    expect(reason).toBeDefined();
    expect(reason?.kind).toBe('manifest_error');
    expect(String(reason?.message)).toContain('bogus_key');
    // The raw spelling must not survive. FIELD_MAP's own comment records the
    // spec_id case, where every access silently returned undefined.
    expect(converted).not.toHaveProperty('stop_reason');
  });

  it('keeps the placeholders distinguishable from a real empty workspace', () => {
    const refused = snakeToCamelCoverage(rawRefused) as CoverageReport & Loose;
    const empty = snakeToCamelCoverage({
      entries: [],
      summary: { total_specs: 0, passing: 0, failing: 0 },
      parse_errors: [],
    }) as CoverageReport & Loose;

    // Identical on every field a consumer would otherwise read.
    expect(refused.entries).toEqual(empty.entries);
    expect(refused.parseErrors).toEqual(empty.parseErrors);
    // The negative control. Without it, an implementation that renamed the
    // empty state would satisfy the assertion above.
    expect(empty.stopReason).toBeUndefined();
    expect(refused.stopReason).toBeDefined();
  });

  it('keeps a refusal in a merged report, keyed by the store folder key, without erasing the successful folder', () => {
    const store = new CoverageReportStore(noopOps);

    const okReport = snakeToCamelCoverage({
      entries: [
        { spec_id: 'ok-spec', total_acs: 1, covered_acs: ['AC-01'], passes_threshold: true },
      ],
      summary: { total_specs: 1, passing: 1, failing: 0 },
      parse_errors: [],
    }) as CoverageReport;
    const refusedReport = snakeToCamelCoverage(rawRefused) as CoverageReport;

    // The exact values runCoverageForFolder stores under: createClientKey of
    // the folder path. Two folders, one of each state.
    const okKey = '/ws/good';
    const refusedKey = '/ws/bad';
    store.set(okKey, okReport, okKey);
    store.set(refusedKey, refusedReport, refusedKey);

    const merged = store.merged() as (CoverageReport & Loose) | null;
    expect(merged).not.toBeNull();

    // The successful folder survives. A merge that dropped it would make the
    // refusal look like the whole workspace.
    expect(merged!.entries.map(e => e.specID)).toContain('ok-spec');

    // The refusal survives, and it names its folder. A single aggregate
    // stopReason would have no owner, and concatenating reasons would lose
    // which folder each belongs to.
    const byFolder = merged!.folderStopReasons as Record<string, Loose> | undefined;
    expect(byFolder).toBeDefined();
    expect(Object.keys(byFolder!)).toEqual([refusedKey]);
    expect(byFolder![refusedKey].kind).toBe('manifest_error');

    // The aggregate does not inherit a singular reason. If it did, a consumer
    // could read one folder's refusal as the whole workspace's.
    expect(merged!.stopReason).toBeUndefined();
  });

});

// ---------------------------------------------------------------------------
// AC-79, the product path, through the scripted CLI.
//
// Observing that stopReason exists after conversion does not stop the sidebar
// rendering "nothing to show", so the required evidence runs the client.
//
// Scripted, not the real binary. A describe.skip guarded on bin/specter lets a
// clean checkout report this criterion covered while running none of it, and a
// binary that happens to exist can be stale. The real-binary run is additional
// evidence and lives in stopReasonIntegration.test.ts.
//
// The harness follows the clientExitCode.test.ts precedent, duplicated rather
// than imported because those files export nothing.
// ---------------------------------------------------------------------------

interface ScriptedRun {
  exitCode: number;
  stdout: string;
  stderr?: string;
}

const scripted: { next?: ScriptedRun; calls: string[][] } = { calls: [] };

jest.mock('child_process', () => {
  const util = require('util');
  const { EventEmitter } = require('events');

  function execFile(file: string, ...rest: unknown[]): unknown {
    const args = (Array.isArray(rest[0]) ? rest[0] : []) as string[];
    const cb = rest.filter(a => typeof a === 'function').pop() as
      | ((e: Error | null, so: string, se: string) => void)
      | undefined;
    scripted.calls.push(args);
    const run = scripted.next ?? { exitCode: 0, stdout: '{}', stderr: '' };

    const child = new EventEmitter();
    (child as unknown as Record<string, unknown>).kill = () => true;
    process.nextTick(() => {
      if (!cb) return;
      if (run.exitCode === 0) {
        cb(null, run.stdout, run.stderr ?? '');
        return;
      }
      // Node's own semantics: stdout is still delivered on a non-zero exit,
      // with err.code carrying the numeric status.
      const err: NodeJS.ErrnoException = new Error(`Command failed: ${file}`);
      err.code = run.exitCode as unknown as string;
      cb(err, run.stdout, run.stderr ?? '');
    });
    return child;
  }
  (execFile as unknown as Record<symbol, unknown>)[util.promisify.custom] = (
    file: string,
    args?: string[],
  ) =>
    new Promise((resolve, reject) => {
      execFile(file, args ?? [], (e: Error | null, stdout: string, stderr: string) => {
        if (e) {
          (e as unknown as Record<string, unknown>).stdout = stdout;
          (e as unknown as Record<string, unknown>).stderr = stderr;
          reject(e);
          return;
        }
        resolve({ stdout, stderr });
      });
    });

  return { execFile, spawn: () => new (require('events').EventEmitter)() };
});

describe('spec-vscode/AC-79 the product path, through the scripted CLI', () => {
  beforeEach(() => {
    scripted.calls = [];
    scripted.next = undefined;
  });

  it('SpecterClient.coverage() returns a typed stopReason for a refused run', async () => {
    // The CLI refuses and still writes the document; exit 1 is the real shape.
    scripted.next = {
      exitCode: 1,
      stdout: JSON.stringify(rawRefused),
      stderr: 'error: unknown settings key "settings.bogus_key"\n',
    };

    const client = new SpecterClient({
      binaryPath: '/fake/specter',
      manifestPath: '/ws/bad/specter.yaml',
      workspaceFolder: '/ws/bad',
    });
    const report = (await client.coverage()) as CoverageReport & Loose;

    // The client really ran, so a stub that returned a canned object cannot
    // satisfy this.
    expect(scripted.calls.some(a => a.includes('coverage') && a.includes('--json'))).toBe(true);

    const reason = report.stopReason as Loose | undefined;
    expect(reason).toBeDefined();
    expect(reason?.kind).toBe('manifest_error');
    expect(report).not.toHaveProperty('stop_reason');
  });

  it('the sidebar does not render a refused run as an empty workspace', () => {
    const refused = snakeToCamelCoverage(rawRefused) as CoverageReport;
    const emptyWorkspace = snakeToCamelCoverage({
      entries: [],
      summary: { total_specs: 0, passing: 0, failing: 0 },
      parse_errors: [],
    }) as CoverageReport;
    const parseFailed = snakeToCamelCoverage({
      entries: [],
      summary: { total_specs: 0, passing: 0, failing: 0 },
      parse_errors: [{ file: 'specs/broken.spec.yaml', type: 'required', message: 'boom' }],
    }) as CoverageReport;

    const refusedRoot = JSON.stringify(buildCoverageTreeRoot(refused));
    const emptyRoot = JSON.stringify(buildCoverageTreeRoot(emptyWorkspace));
    const parseFailedRoot = JSON.stringify(buildCoverageTreeRoot(parseFailed));

    expect(refusedRoot).not.toEqual(emptyRoot);
    expect(refusedRoot).not.toEqual(parseFailedRoot);
    // The negative control: a real empty workspace still says so.
    expect(emptyRoot).toContain('No specs found in this workspace');
  });

  it('the Output channel receives the reason, with its folder, through the delivery path', () => {
    // Delivery, not formatting. An exported formatter nobody calls would pass
    // a direct-call assertion while the sidebar showed only "No specs found".
    //
    // The on-save handler itself is unreachable from this suite, since nothing
    // imports ../extension. The highest reachable seam is the function that
    // takes the merged report and writes to a channel, so that is what is
    // required and driven here.
    const mod = require('../coverage') as Record<string, unknown>;
    const report = mod.reportStopReasons as
      | ((channel: { appendLine(v: string): void }, merged: unknown) => void)
      | undefined;
    expect(typeof report).toBe('function');

    const store = new CoverageReportStore(noopOps);
    store.set('/ws/good', snakeToCamelCoverage({
      entries: [{ spec_id: 'ok-spec', total_acs: 1, covered_acs: ['AC-01'], passes_threshold: true }],
      summary: { total_specs: 1, passing: 1, failing: 0 },
      parse_errors: [],
    }) as CoverageReport, '/ws/good');
    store.set('/ws/bad', snakeToCamelCoverage(rawRefused) as CoverageReport, '/ws/bad');

    const lines: string[] = [];
    report!({ appendLine: (v: string) => lines.push(v) }, store.merged());

    // One line, naming the folder that refused and why. The successful folder
    // produces none, or an operator cannot tell which one needs attention.
    expect(lines).toHaveLength(1);
    expect(lines[0]).toContain('/ws/bad');
    expect(lines[0]).toContain('manifest_error');
    expect(lines[0]).not.toContain('/ws/good');
  });
});

// ---------------------------------------------------------------------------
// AC-79, the declarations, checked semantically.
//
// Reading declared members and printed type text is syntactic, and four
// mutants survive it: an aggregate that INHERITS the forbidden singular
// stopReason through `extends`, a fifth kind, a kind widened with `| string`,
// and a folder map whose value merely mentions the shared type rather than
// resolving to it.
//
// So the checker is asked instead: properties are resolved including inherited
// ones, the union is enumerated from its resolved members, and widening is
// tested by assignability rather than by text.
// ---------------------------------------------------------------------------

describe('spec-vscode/AC-79 the stop-reason declarations, semantically', () => {
  const typesPath = path.resolve(__dirname, '..', 'types.ts');

  /**
   * The canonical set, DERIVED from the declaration rather than restated.
   *
   * A `const KINDS = new Set([...])` here would be the private copy in a test
   * fixture C-33 forbids, and it would be the copy held by the very test that
   * forbids copies. It would also go stale silently: the scan would keep
   * looking for the old four while the type declared five.
   */
  function canonicalKinds(): Set<string> {
    const sym = prop('CoverageStopReason', 'kind');
    if (!sym) return new Set();
    const t = typeOfProp(sym);
    const members = t.isUnion() ? t.types : [t];
    const out = new Set<string>();
    for (const m of members) {
      if (m.isStringLiteral()) out.add(m.value);
    }
    return out;
  }

  // ONE compiler world for the whole block. Building a Program per lookup gave
  // every call its own universe, so an identity comparison between two
  // resolved types compared objects from different programs and could never
  // hold, however correct the declaration. The test would have stayed red
  // after a correct implementation.
  const prog = ts.createProgram([typesPath], {
    target: ts.ScriptTarget.ES2020,
    strict: true,
    noEmit: true,
  });
  const checker = prog.getTypeChecker();
  const src = prog.getSourceFile(typesPath);

  /** The declared type of a named interface, resolved in the shared world. */
  function typeOf(name: string): ts.Type | undefined {
    if (!src) return undefined;
    let found: ts.Type | undefined;
    ts.forEachChild(src, node => {
      if (ts.isInterfaceDeclaration(node) && node.name.text === name) {
        found = checker.getTypeAtLocation(node.name);
      }
    });
    return found;
  }

  function decl(name: string): ts.InterfaceDeclaration | undefined {
    if (!src) return undefined;
    let found: ts.InterfaceDeclaration | undefined;
    ts.forEachChild(src, node => {
      if (ts.isInterfaceDeclaration(node) && node.name.text === name) found = node;
    });
    return found;
  }

  /** A property resolved through the type, so inherited members are visible. */
  function prop(iface: string, name: string): ts.Symbol | undefined {
    const t = typeOf(iface);
    return t ? checker.getPropertyOfType(t, name) : undefined;
  }

  function typeOfProp(sym: ts.Symbol): ts.Type {
    return checker.getTypeOfSymbolAtLocation(sym, sym.valueDeclaration ?? sym.declarations![0]);
  }

  function nonUndefinedMembers(t: ts.Type): ts.Type[] {
    return t.isUnion()
      ? t.types.filter(x => (x.flags & ts.TypeFlags.Undefined) === 0)
      : [t];
  }

  it('CoverageStopReason is exported and declares kind and message as required', () => {
    const d = decl('CoverageStopReason');
    expect(d).toBeDefined();
    // Exported, or no consumer can name the shared type and every one of them
    // ends up declaring its own.
    const exported = d!.modifiers?.some(m => m.kind === ts.SyntaxKind.ExportKeyword) ?? false;
    expect(exported).toBe(true);

    for (const member of ['kind', 'message']) {
      const sym = prop('CoverageStopReason', member);
      expect(sym).toBeDefined();
      // Required. An optional kind or message lets a producer emit a reason
      // that names nothing or explains nothing.
      expect((sym!.flags & ts.SymbolFlags.Optional) !== 0).toBe(false);
    }
  });

  it('kind resolves to exactly the four literals', () => {
    const sym = prop('CoverageStopReason', 'kind');
    expect(sym).toBeDefined();
    const kindType = typeOfProp(sym!);

    // Enumerated from the resolved union, so `| string` appears as a
    // non-literal member rather than hiding inside printed text.
    const members = kindType.isUnion() ? kindType.types : [kindType];
    const literals = members.map(m =>
      m.isStringLiteral() ? m.value : `NON_LITERAL:${checker.typeToString(m)}`,
    );
    // The expected list is acceptance data for this one assertion. It is not
    // reused as a policy source anywhere else in this file, which is the
    // distinction between checking a set and keeping a second copy of it.
    expect(literals.sort()).toEqual(
      ['invalid_flag', 'manifest_error', 'unknown_scope', 'unmet_precondition'].sort(),
    );
  });

  it('message is exactly string, not unknown and not nullable', () => {
    const sym = prop('CoverageStopReason', 'message');
    expect(sym).toBeDefined();
    const t = typeOfProp(sym!);
    // `any` is assignable in both directions, so bidirectional assignability
    // alone accepts `message: any` and the field explains nothing while
    // type-checking everywhere. Reject it on the flags first.
    expect((t.flags & ts.TypeFlags.Any) !== 0).toBe(false);
    expect((t.flags & ts.TypeFlags.String) !== 0).toBe(true);
    // Then assignability, which is what rules out `unknown`, accepted by
    // nothing narrower, and `string | null`, which is wider.
    const stringType = checker.getStringType();
    expect(checker.isTypeAssignableTo(t, stringType)).toBe(true);
    expect(checker.isTypeAssignableTo(stringType, t)).toBe(true);
  });

  it('CoverageReport.stopReason is optional and is the shared type', () => {
    const sym = prop('CoverageReport', 'stopReason');
    expect(sym).toBeDefined();
    expect((sym!.flags & ts.SymbolFlags.Optional) !== 0).toBe(true);

    const members = nonUndefinedMembers(typeOfProp(sym!));
    expect(members).toHaveLength(1);
    // Identity in one shared world, so a structurally identical second
    // declaration does not satisfy it.
    expect(members[0]).toBe(typeOf('CoverageStopReason'));
  });

  it('MergedCoverageReport owns the folder map and does not inherit a singular reason', () => {
    const sym = prop('MergedCoverageReport', 'folderStopReasons');
    expect(sym).toBeDefined();

    const members = nonUndefinedMembers(typeOfProp(sym!));
    expect(members).toHaveLength(1);
    // Resolve the map's VALUE type. A value that merely mentions the name in
    // printed text would pass a text check.
    const index = checker.getIndexInfoOfType(members[0], ts.IndexKind.String);
    expect(index).toBeDefined();
    expect(index!.type).toBe(typeOf('CoverageStopReason'));

    // getPropertyOfType resolves INHERITED members, so an aggregate gaining
    // stopReason through `extends CoverageReport` fails here. That is the
    // mutant a declared-members-only read cannot see.
    expect(prop('MergedCoverageReport', 'stopReason')).toBeUndefined();
  });

  it('declares the kind policy once, in types.ts and nowhere else', () => {
    const canonical = canonicalKinds();
    // The control, and it has to come first: before the implementation lands
    // the derived set is empty, and an empty set is a subset of everything, so
    // every file would read as clean.
    expect(canonical.size).toBeGreaterThan(0);

    // DECLARATIONS, not uses. A branch comparing one kind is ordinary code; a
    // union, an enum, or a constant collection that reconstructs the set is a
    // second copy of the policy. Parsed rather than grepped, so a copy written
    // with double quotes is seen the same as one with single quotes.
    const declaredKindsIn = (file: string): Set<string> => {
      const source = ts.createSourceFile(
        file,
        fs.readFileSync(file, 'utf8'),
        ts.ScriptTarget.ES2020,
        true,
      );
      const found = new Set<string>();

      // Reachable through a declaration name. That is the whole rule, and it
      // replaces a per-shape list that kept missing shapes: object property
      // NAMES were inspected while their VALUES were not, so this survived:
      //
      //   const PrivateKinds = { manifest: "manifest_error", ... } as const;
      //
      // and four standalone `const A = "manifest_error"` survived too, because
      // plain initializers were never read at all.
      //
      // A canonical value a name can reach is reusable policy. One handed
      // inline to a call is acceptance data, checked once and reachable by
      // nothing, which is why the exact-kind assertion in this file is not an
      // offender.
      const POLICY_ANCESTORS = new Set<ts.SyntaxKind>([
        ts.SyntaxKind.VariableDeclaration,
        ts.SyntaxKind.PropertyAssignment,
        ts.SyntaxKind.PropertyDeclaration,
        ts.SyntaxKind.PropertySignature,
        ts.SyntaxKind.EnumMember,
        ts.SyntaxKind.TypeAliasDeclaration,
        ts.SyntaxKind.InterfaceDeclaration,
      ]);
      const reachableByName = (n: ts.Node): boolean => {
        for (let cur: ts.Node | undefined = n.parent; cur; cur = cur.parent) {
          if (POLICY_ANCESTORS.has(cur.kind)) return true;
        }
        return false;
      };

      const visit = (n: ts.Node): void => {
        if (
          ts.isStringLiteralLike(n) &&
          canonical.has(n.text) &&
          reachableByName(n)
        ) {
          found.add(n.text);
        }
        ts.forEachChild(n, visit);
      };
      visit(source);
      return found;
    };

    const srcDir = path.resolve(__dirname, '..');
    const offenders: string[] = [];
    const walk = (dir: string): void => {
      for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        if (fs.statSync(full).isDirectory()) {
          walk(full);
          continue;
        }
        if (!name.endsWith('.ts') || full === typesPath) continue;
        const declared = declaredKindsIn(full);
        if (canonical.size > 0 && [...canonical].every(k => declared.has(k))) {
          offenders.push(path.relative(srcDir, full));
        }
      }
    };
    // __tests__ is walked too. This file must not hold the copy it forbids,
    // and excluding the directory would have hidden exactly that.
    walk(srcDir);

    expect(offenders).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// AC-79, the production wiring.
//
// The behavioral test above proves the reporter works. It cannot prove anyone
// calls it, and an exported-but-unused reporter leaves users with no reason in
// the Output channel while every behavioral assertion passes.
//
// extension.ts cannot be imported by this suite, but it can be read, and
// commands.test.ts and csp.test.ts already establish static analysis over it
// as the way to bind what lives there.
// ---------------------------------------------------------------------------

const EXT_TS = path.resolve(__dirname, '..', 'extension.ts');

/** The initializer expressions bound to `name` inside `fnName`. */
function initializersInFunction(fnName: string, name: string): ts.Expression[] {
  const source = ts.createSourceFile(
    EXT_TS,
    fs.readFileSync(EXT_TS, 'utf8'),
    ts.ScriptTarget.ES2020,
    true,
  );
  const out: ts.Expression[] = [];
  const visit = (node: ts.Node): void => {
    if (
      (ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) &&
      node.name &&
      node.name.getText(source) === fnName
    ) {
      const collect = (n: ts.Node): void => {
        if (
          ts.isVariableDeclaration(n) &&
          ts.isIdentifier(n.name) &&
          n.name.text === name &&
          n.initializer
        ) {
          out.push(n.initializer);
        }
        ts.forEachChild(n, collect);
      };
      ts.forEachChild(node, collect);
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return out;
}

/** Every identifier called inside the named top-level function. */
function callsInFunction(fnName: string): string[] {
  const source = ts.createSourceFile(
    EXT_TS,
    fs.readFileSync(EXT_TS, 'utf8'),
    ts.ScriptTarget.ES2020,
    true,
  );
  const calls: string[] = [];
  let found = false;
  const visit = (node: ts.Node): void => {
    if (
      (ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) &&
      node.name &&
      node.name.getText(source) === fnName
    ) {
      found = true;
      const collect = (n: ts.Node): void => {
        if (ts.isCallExpression(n)) {
          const fn = n.expression;
          if (ts.isIdentifier(fn)) calls.push(fn.text);
          else if (ts.isPropertyAccessExpression(fn)) calls.push(fn.name.text);
        }
        ts.forEachChild(n, collect);
      };
      ts.forEachChild(node, collect);
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  if (!found) throw new Error(`${fnName} not found in extension.ts`);
  return calls;
}

describe('spec-vscode/AC-79 the coverage run reports refusals', () => {

  /** The argument NODES of each call to `callee` inside `fnName`.
   *
   * Nodes, not source text. Substring matching accepts an expression that
   * merely contains the wanted spelling, so all of these read as correct:
   *
   *   reportStopReasons(otheroutputChannel, coverageReports.merged() && null)
   *   reportStopReasons((outputChannel, null), (coverageReports.merged(), null))
   */
  function callArgsInFunction(fnName: string, callee: string): ts.Expression[][] {
    const source = ts.createSourceFile(
      EXT_TS,
      fs.readFileSync(EXT_TS, 'utf8'),
      ts.ScriptTarget.ES2020,
      true,
    );
    const out: ts.Expression[][] = [];
    const visit = (node: ts.Node): void => {
      if (
        (ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node)) &&
        node.name &&
        node.name.getText(source) === fnName
      ) {
        const collect = (n: ts.Node): void => {
          if (ts.isCallExpression(n)) {
            const fn = n.expression;
            const name = ts.isIdentifier(fn)
              ? fn.text
              : ts.isPropertyAccessExpression(fn)
                ? fn.name.text
                : '';
            if (name === callee) {
              out.push([...n.arguments]);
            }
          }
          ts.forEachChild(n, collect);
        };
        ts.forEachChild(node, collect);
      }
      ts.forEachChild(node, visit);
    };
    visit(source);
    return out;
  }

  it('runCoverageForFolder reports stop reasons exactly once, after storing the report', () => {
    const calls = callsInFunction('runCoverageForFolder');

    // Positive control: the function was really read. An empty call list would
    // make every claim below pass on nothing.
    expect(calls.length).toBeGreaterThan(0);
    expect(calls).toContain('set');

    const reporterCalls = calls.filter(c => c === 'reportStopReasons');
    expect(reporterCalls).toHaveLength(1);

    // After storing. Reporting before the store would read the previous run's
    // state, which is the wrong folder's answer on the first run of a folder.
    expect(calls.indexOf('reportStopReasons')).toBeGreaterThan(calls.indexOf('set'));
  });

  it('runCoverageForFolder passes the real channel and the post-write aggregate', () => {
    // Position alone is not wiring. `reportStopReasons(outputChannel, null)`
    // satisfies the ordering check and leaves the channel silent, and passing
    // this folder's own report instead of the aggregate loses the multi-root
    // ownership the folder map exists to carry. So the arguments are read.
    const args = callArgsInFunction('runCoverageForFolder', 'reportStopReasons');
    expect(args).toHaveLength(1);
    expect(args[0]).toHaveLength(2);

    // Unwrap only what is transparent: parentheses and a non-null assertion.
    // A comma expression is NOT unwrapped, because its value is the last
    // operand and `(outputChannel, null)` is null.
    const unwrap = (e: ts.Expression): ts.Expression => {
      let cur = e;
      for (;;) {
        if (ts.isParenthesizedExpression(cur)) {
          cur = cur.expression;
          continue;
        }
        if (ts.isNonNullExpression(cur)) {
          cur = cur.expression;
          continue;
        }
        return cur;
      }
    };

    const channelArg = unwrap(args[0][0]);
    const reportArg = unwrap(args[0][1]);

    // Argument one is the identifier itself, not something spelled like it.
    // `otheroutputChannel` contains the text and is a different binding.
    expect(ts.isIdentifier(channelArg)).toBe(true);
    expect((channelArg as ts.Identifier).text).toBe('outputChannel');

    // Argument two is a zero-argument call to merged on coverageReports.
    // `coverageReports.merged() && null` is a binary expression whose value is
    // null, and it contains the wanted text.
    // Either the call itself, or a local bound to that call in the same
    // function. Binding once and reusing is the same value, and pinning the
    // expression form would forbid computing the aggregate once instead of
    // twice. An identifier bound to anything else still fails.
    const isMergedCall = (e: ts.Expression): boolean => {
      if (!ts.isCallExpression(e) || e.arguments.length !== 0) return false;
      const ex = e.expression;
      return (
        ts.isPropertyAccessExpression(ex) &&
        ex.name.text === 'merged' &&
        ts.isIdentifier(ex.expression) &&
        ex.expression.text === 'coverageReports'
      );
    };
    const resolvesToMerged = (e: ts.Expression): boolean => {
      if (isMergedCall(e)) return true;
      if (!ts.isIdentifier(e)) return false;
      const inits = initializersInFunction('runCoverageForFolder', e.text);
      return inits.length === 1 && isMergedCall(inits[0]);
    };
    expect(resolvesToMerged(reportArg)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// AC-79, the aggregate as the product actually uses it.
//
// The sidebar test above hands a SINGULAR report to the builder. The product
// hands it coverageReports.merged(), which never carries a singular stopReason
// because C-33 forbids one on an aggregate. So the builder's singular check
// cannot fire in production, and a refused folder reaches the real sidebar as
// an empty entries list with no parse errors: "No specs found in this
// workspace", which is the exact misreport this criterion exists to remove.
// ---------------------------------------------------------------------------

describe('spec-vscode/AC-79 the aggregate reaches the real consumers', () => {
  const refusedReport = () => snakeToCamelCoverage(rawRefused) as CoverageReport;
  const okReport = () =>
    snakeToCamelCoverage({
      entries: [
        { spec_id: 'ok-spec', total_acs: 1, covered_acs: ['AC-01'], passes_threshold: true },
      ],
      summary: { total_specs: 1, passing: 1, failing: 0 },
      parse_errors: [],
    }) as CoverageReport;

  it('a single refused folder is not rendered as an empty workspace', () => {
    const store = new CoverageReportStore(noopOps);
    store.set('/ws/bad', refusedReport(), '/ws/bad');

    // Through merged(), which is what the tree provider passes.
    const root = JSON.stringify(buildCoverageTreeRoot(store.merged() as never));
    expect(root).not.toContain('No specs found in this workspace');
    // The human message and the folder that refused. The KIND belongs in the
    // Output channel, which AC-79 binds separately; requiring it here was an
    // over-specification, since a sidebar label is read by a person.
    expect(root).toContain('bogus_key');
    expect(root).toContain('/ws/bad');
  });

  it('a mixed aggregate keeps the successful entries and still surfaces the refusal', () => {
    const store = new CoverageReportStore(noopOps);
    store.set('/ws/good', okReport(), '/ws/good');
    store.set('/ws/bad', refusedReport(), '/ws/bad');

    const root = JSON.stringify(buildCoverageTreeRoot(store.merged() as never));
    // The working folder's specs are still there. A refusal must not blank the
    // rest of the workspace.
    expect(root).toContain('ok-spec');
    // And the refusal is visible, naming the folder it came from.
    expect(root).toContain('/ws/bad');
  });

  it('isFolderRefused answers for one folder within the aggregate', () => {
    const mod = require('../coverage') as Record<string, unknown>;
    const isFolderRefused = mod.isFolderRefused as
      | ((merged: unknown, key: string) => boolean)
      | undefined;
    expect(typeof isFolderRefused).toBe('function');

    const store = new CoverageReportStore(noopOps);
    store.set('/ws/good', okReport(), '/ws/good');
    store.set('/ws/bad', refusedReport(), '/ws/bad');
    const merged = store.merged();

    expect(isFolderRefused!(merged, '/ws/bad')).toBe(true);
    // The negative control. Without it a function returning true always would
    // pass, and every folder would be marked errored.
    expect(isFolderRefused!(merged, '/ws/good')).toBe(false);
    expect(isFolderRefused!(null, '/ws/bad')).toBe(false);
  });

  it('runCoverageForFolder reads refusal state before interpreting placeholders', () => {
    // A refusal carries entries [] and no parse errors, so the existing branch
    // reads it as a clean measurement: it updates the status bar from a
    // zero-entry aggregate and DELETES the folder from coverageErrorFolders.
    // The refusal check has to come first.
    const calls = callsInFunction('runCoverageForFolder');
    expect(calls.length).toBeGreaterThan(0);

    const refusedAt = calls.indexOf('isFolderRefused');
    expect(refusedAt).toBeGreaterThanOrEqual(0);

    for (const later of ['updateSpecIndex', 'updateStatusBar', 'pushCoverageParseDiagnostics']) {
      const at = calls.indexOf(later);
      if (at >= 0) {
        expect(refusedAt).toBeLessThan(at);
      }
    }
  });
});

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
 * The behavioral half runs here. The compile-time half, which needs the
 * exported type to reject against, lands with the implementation.
 */

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as ts from 'typescript';

import { SpecterClient, snakeToCamelCoverage } from '../client';
import { CoverageReportStore, buildCoverageTreeRoot } from '../coverage';
import type { CoverageReport } from '../types';

/**
 * The red commit reads the new fields structurally rather than through the
 * types it will introduce. A test that imports a type before it exists is a
 * compile failure, which takes the whole suite down and reads as a kill.
 *
 * The compile-time rejections C-33 requires, an unknown kind and a kind
 * widened to string, land with the implementation, where the exported type
 * exists to reject against. Writing them here against a copied union would be
 * the second declaration of the four-kind policy C-33 forbids.
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

  function program(): { checker: ts.TypeChecker; src: ts.SourceFile } {
    const prog = ts.createProgram([typesPath], {
      target: ts.ScriptTarget.ES2020,
      strict: true,
      noEmit: true,
    });
    const src = prog.getSourceFile(typesPath);
    if (!src) throw new Error(`types.ts not found at ${typesPath}`);
    return { checker: prog.getTypeChecker(), src };
  }

  /** The declared type of a named interface, resolved. */
  function typeOf(name: string): { checker: ts.TypeChecker; type: ts.Type } | undefined {
    const { checker, src } = program();
    let found: ts.Type | undefined;
    ts.forEachChild(src, node => {
      if (ts.isInterfaceDeclaration(node) && node.name.text === name) {
        found = checker.getTypeAtLocation(node.name);
      }
    });
    return found ? { checker, type: found } : undefined;
  }

  /** A property resolved through the type, so inherited members are visible. */
  function prop(iface: string, name: string): { checker: ts.TypeChecker; sym: ts.Symbol } | undefined {
    const t = typeOf(iface);
    if (!t) return undefined;
    const sym = t.checker.getPropertyOfType(t.type, name);
    return sym ? { checker: t.checker, sym } : undefined;
  }

  it('CoverageStopReason.kind resolves to exactly the four literals', () => {
    const p = prop('CoverageStopReason', 'kind');
    expect(p).toBeDefined();
    const kindType = p!.checker.getTypeOfSymbolAtLocation(p!.sym, p!.sym.valueDeclaration!);

    // Enumerated from the resolved union, so `| string` shows up as a
    // non-literal member rather than hiding inside printed text.
    const members = kindType.isUnion() ? kindType.types : [kindType];
    const literals = members.map(m =>
      m.isStringLiteral() ? m.value : `NON_LITERAL:${p!.checker.typeToString(m)}`,
    );
    expect(literals.sort()).toEqual(
      ['invalid_flag', 'manifest_error', 'unknown_scope', 'unmet_precondition'].sort(),
    );

    const message = prop('CoverageStopReason', 'message');
    expect(message).toBeDefined();
    // Required, not optional. An optional message names a kind and explains
    // nothing.
    expect((message!.sym.flags & ts.SymbolFlags.Optional) !== 0).toBe(false);
  });

  it('CoverageReport.stopReason is optional and is the shared type', () => {
    const p = prop('CoverageReport', 'stopReason');
    expect(p).toBeDefined();
    expect((p!.sym.flags & ts.SymbolFlags.Optional) !== 0).toBe(true);

    const shared = typeOf('CoverageStopReason');
    const declared = p!.checker.getTypeOfSymbolAtLocation(p!.sym, p!.sym.valueDeclaration!);
    const nonUndefined = declared.isUnion()
      ? declared.types.filter(x => (x.flags & ts.TypeFlags.Undefined) === 0)
      : [declared];
    expect(nonUndefined).toHaveLength(1);
    // Identity against the resolved shared type, so a structurally identical
    // second declaration does not satisfy it.
    expect(nonUndefined[0]).toBe(shared!.type);
  });

  it('MergedCoverageReport owns the folder map and does not inherit a singular reason', () => {
    const map = prop('MergedCoverageReport', 'folderStopReasons');
    expect(map).toBeDefined();

    // Resolve the map's VALUE type to the shared type. A value that merely
    // mentions the name in printed text would pass a text check.
    const mapType = map!.checker.getTypeOfSymbolAtLocation(map!.sym, map!.sym.valueDeclaration!);
    const nonUndefined = mapType.isUnion()
      ? mapType.types.filter(x => (x.flags & ts.TypeFlags.Undefined) === 0)
      : [mapType];
    expect(nonUndefined).toHaveLength(1);
    const index = map!.checker.getIndexInfoOfType(nonUndefined[0], ts.IndexKind.String);
    expect(index).toBeDefined();
    expect(index!.type).toBe(typeOf('CoverageStopReason')!.type);

    // getPropertyOfType resolves INHERITED members too, so an aggregate that
    // gains stopReason through `extends CoverageReport` fails here. That is
    // the mutant a declared-members-only read cannot see.
    expect(prop('MergedCoverageReport', 'stopReason')).toBeUndefined();
  });

  it('declares the four kinds once, in types.ts and nowhere else', () => {
    const srcDir = path.resolve(__dirname, '..');
    const offenders: string[] = [];
    const walk = (dir: string): void => {
      for (const name of fs.readdirSync(dir)) {
        const full = path.join(dir, name);
        if (fs.statSync(full).isDirectory()) {
          if (name !== '__tests__') walk(full);
          continue;
        }
        if (!name.endsWith('.ts') || full === typesPath) continue;
        if (fs.readFileSync(full, 'utf8').includes("'unmet_precondition'")) {
          offenders.push(path.relative(srcDir, full));
        }
      }
    };
    walk(srcDir);

    // The control. Without it this passes while NOTHING declares the union,
    // which is the state before the implementation lands.
    expect(fs.readFileSync(typesPath, 'utf8')).toContain("'unmet_precondition'");
    expect(offenders).toEqual([]);
  });
});

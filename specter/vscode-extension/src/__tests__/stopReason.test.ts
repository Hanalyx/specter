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
// AC-79, the product path. The assertions above read the converter and the
// store directly, which is necessary and not sufficient: observing that
// stopReason exists does not stop the sidebar rendering "nothing to show".
// ---------------------------------------------------------------------------

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const CLI = path.join(REPO_ROOT, 'bin', 'specter');
const describeWithCLI = fs.existsSync(CLI) ? describe : describe.skip;

describeWithCLI('spec-vscode/AC-79 the product path, through the real client', () => {
  let ws: string;

  beforeEach(() => {
    ws = fs.mkdtempSync(path.join(os.tmpdir(), 'stopreason-'));
    fs.mkdirSync(path.join(ws, 'specs'), { recursive: true });
    // A manifest the loader rejects. Under --strictness annotation, which is
    // what SpecterClient.coverage() passes, this still refuses.
    fs.writeFileSync(
      path.join(ws, 'specter.yaml'),
      'schema_version: 1\nsystem:\n  name: s\nsettings:\n  bogus_key: 1\n',
    );
    fs.writeFileSync(
      path.join(ws, 'specs', 'ok.spec.yaml'),
      [
        'spec:',
        '  id: ok-spec',
        '  version: "1.0.0"',
        '  status: approved',
        '  tier: 3',
        '  context: {system: s, feature: f, description: "A valid spec beside a rejected manifest"}',
        '  objective: {summary: "Reach the manifest loader"}',
        '  constraints:',
        '    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}',
        '  acceptance_criteria:',
        '    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}',
        '',
      ].join('\n'),
    );
  });

  afterEach(() => fs.rmSync(ws, { recursive: true, force: true }));

  it('SpecterClient.coverage() returns a typed stopReason for a refused run', async () => {
    // The real client, not the converter. A converter test cannot see a
    // client that drops the field, throws, or never reaches the conversion.
    const client = new SpecterClient({
      binaryPath: CLI,
      manifestPath: path.join(ws, 'specter.yaml'),
      workspaceFolder: ws,
    });
    const report = (await client.coverage()) as CoverageReport & Loose;

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

    const refusedRoot = buildCoverageTreeRoot(refused);
    const emptyRoot = buildCoverageTreeRoot(emptyWorkspace);
    const parseFailedRoot = buildCoverageTreeRoot(parseFailed);

    // The three states are distinguishable to a user. The refusal is the one
    // that used to be indistinguishable from an empty workspace.
    expect(JSON.stringify(refusedRoot)).not.toEqual(JSON.stringify(emptyRoot));
    expect(JSON.stringify(refusedRoot)).not.toEqual(JSON.stringify(parseFailedRoot));
    // The negative control: a real empty workspace still says so.
    expect(JSON.stringify(emptyRoot)).toContain('No specs found in this workspace');
  });

  it('the Output channel names the reason and the folder it came from', () => {
    // Required through a dynamic lookup rather than a static import, so the
    // absence is an ordinary failure rather than a compile error that takes
    // the whole suite down and reads as a kill.
    const mod = require('../coverage') as Record<string, unknown>;
    const format = mod.formatStopReasonNotice as
      | ((folderKey: string, reason: { kind: string; message: string }) => string)
      | undefined;

    expect(typeof format).toBe('function');
    const line = format!('/ws/bad', { kind: 'manifest_error', message: 'unknown settings key' });
    // An operator who cannot see which folder refused cannot act on it.
    expect(line).toContain('/ws/bad');
    expect(line).toContain('manifest_error');
    expect(line).toContain('unknown settings key');
  });
});

// ---------------------------------------------------------------------------
// AC-79, the declarations. C-33 fixes the shapes; these bind them.
//
// Read out of types.ts with the TypeScript compiler API at Jest runtime, so a
// missing declaration is an ordinary test failure. Writing them as
// `@ts-expect-error` against an imported type would not compile until the type
// exists, which takes the suite down and reads as a kill, and writing them
// against a copied union would be the second declaration of the four-kind
// policy C-33 forbids.
// ---------------------------------------------------------------------------

describe('spec-vscode/AC-79 the stop-reason declarations', () => {
  const typesPath = path.resolve(__dirname, '..', 'types.ts');

  /** The printed type of a named member on a named interface, or undefined. */
  function memberType(iface: string, member: string): string | undefined {
    const program = ts.createProgram([typesPath], {
      target: ts.ScriptTarget.ES2020,
      strict: true,
      noEmit: true,
    });
    const checker = program.getTypeChecker();
    const source = program.getSourceFile(typesPath);
    if (!source) throw new Error(`types.ts not found at ${typesPath}`);

    let printed: string | undefined;
    ts.forEachChild(source, node => {
      if (!ts.isInterfaceDeclaration(node) || node.name.text !== iface) return;
      for (const m of node.members) {
        if (!m.name || m.name.getText(source) !== member) continue;
        const sym = checker.getSymbolAtLocation(m.name);
        if (!sym) continue;
        const optional = (m as ts.PropertySignature).questionToken ? '?' : '';
        printed = optional + checker.typeToString(
          checker.getTypeOfSymbolAtLocation(sym, m),
        );
      }
    });
    return printed;
  }

  function aliasText(name: string): string | undefined {
    const program = ts.createProgram([typesPath], { noEmit: true });
    const source = program.getSourceFile(typesPath);
    if (!source) return undefined;
    let out: string | undefined;
    ts.forEachChild(source, node => {
      if (ts.isInterfaceDeclaration(node) && node.name.text === name) {
        out = node.getText(source);
      }
    });
    return out;
  }

  it('CoverageStopReason declares exactly the four kinds and a required message', () => {
    const decl = aliasText('CoverageStopReason');
    expect(decl).toBeDefined();
    for (const kind of ['manifest_error', 'invalid_flag', 'unknown_scope', 'unmet_precondition']) {
      expect(decl).toContain(`'${kind}'`);
    }
    // message is required, not optional. An optional message lets a producer
    // name a kind and explain nothing.
    expect(memberType('CoverageStopReason', 'message')).toBe('string');
  });

  it('CoverageReport.stopReason is optional and reuses the shared type', () => {
    // Optional: present only on a refused run, so a consumer keys on presence.
    expect(memberType('CoverageReport', 'stopReason')).toBe('?CoverageStopReason | undefined');
  });

  it('MergedCoverageReport owns the folder map and not a singular reason', () => {
    // The map reuses the same exported type rather than restating it.
    const folderMap = memberType('MergedCoverageReport', 'folderStopReasons');
    expect(folderMap).toBeDefined();
    expect(folderMap).toContain('CoverageStopReason');
    // A singular reason on the aggregate would have no owner, which is the
    // shape the map exists to prevent.
    expect(memberType('MergedCoverageReport', 'stopReason')).toBeUndefined();
  });

  it('declares the four kinds once, in types.ts and nowhere else', () => {
    // A second literal union is a copy of policy that cannot know when the set
    // changes. This repository records eight instances of that shape.
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
    // which is the state before the implementation lands, and the check would
    // read as satisfied when it is merely vacuous.
    expect(fs.readFileSync(typesPath, 'utf8')).toContain("'unmet_precondition'");
    expect(offenders).toEqual([]);
  });
});

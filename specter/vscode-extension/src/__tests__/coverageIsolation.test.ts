// @spec spec-vscode
//
// Tests for multi-root coverage-report isolation (AC-54), ingestion-time
// path normalization against the owning folder (AC-33), and the on-save
// whole-report refresh contract (AC-55). Mix of pure-function tests over
// the coverage.ts helpers and static-analysis tests over extension.ts /
// client.ts source — the same two patterns the rest of this suite uses
// (no vscode runtime mock exists in this repo; extension.ts invariants
// are enforced by grep, see commands.test.ts).

import * as fs from 'fs';
import * as path from 'path';

const posix = path.posix;

import {
  CoverageReportStore,
  normalizeReportPaths,
  findEntryBySpecFile,
} from '../coverage';

import type { CoverageReport, SpecCoverageEntry } from '../types';

const EXT_TS = path.resolve(__dirname, '..', 'extension.ts');
const CLIENT_TS = path.resolve(__dirname, '..', 'client.ts');
const readExtSrc = () => fs.readFileSync(EXT_TS, 'utf-8');
const readClientSrc = () => fs.readFileSync(CLIENT_TS, 'utf-8');

// POSIX path ops so the pure tests are platform-agnostic, mirroring
// resolveWorkspacePathPure's testing convention.
const ops = {
  resolve: (base: string, p: string) => posix.resolve(base, p),
  isAbsolute: (p: string) => posix.isAbsolute(p),
};

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const makeEntry = (
  specID: string,
  overrides: Partial<SpecCoverageEntry> = {},
): SpecCoverageEntry => ({
  specID,
  tier: 2,
  totalACs: 2,
  coveredACs: ['AC-01'],
  uncoveredACs: ['AC-02'],
  coveragePct: 50,
  threshold: 90,
  passesThreshold: false,
  testFiles: [`src/__tests__/${specID}.test.ts`],
  specFile: `specs/${specID}.spec.yaml`,
  ...overrides,
});

const makeReport = (
  entries: SpecCoverageEntry[],
  extra: Partial<CoverageReport> = {},
): CoverageReport => ({
  entries,
  summary: {
    totalSpecs: entries.length,
    passing: entries.filter(e => e.passesThreshold).length,
    failing: entries.filter(e => !e.passesThreshold).length,
    fullyCovered: 0,
    partiallyCovered: entries.length,
    uncovered: 0,
  },
  ...extra,
});

// ---------------------------------------------------------------------------
// AC-33: ingestion-time path normalization against the OWNING folder
// ---------------------------------------------------------------------------

// @ac AC-33
describe('[spec-vscode/AC-33] normalizeReportPaths', () => {
  it('resolves relative specFile, testFiles, and parseErrors[].file against the owning folder root', () => {
    const report = makeReport([makeEntry('spec-a')], {
      parseErrors: [{ file: 'specs/broken.spec.yaml', message: 'bad yaml', line: 3 }],
    });

    const out = normalizeReportPaths(report, '/ws/alpha', ops);

    expect(out.entries[0].specFile).toBe('/ws/alpha/specs/spec-a.spec.yaml');
    expect(out.entries[0].testFiles).toEqual(['/ws/alpha/src/__tests__/spec-a.test.ts']);
    expect(out.parseErrors?.[0].file).toBe('/ws/alpha/specs/broken.spec.yaml');
  });

  it('leaves already-absolute paths untouched', () => {
    const report = makeReport([
      makeEntry('spec-a', {
        specFile: '/elsewhere/specs/spec-a.spec.yaml',
        testFiles: ['/elsewhere/a.test.ts'],
      }),
    ], {
      parseErrors: [{ file: '/elsewhere/specs/broken.spec.yaml', message: 'bad' }],
    });

    const out = normalizeReportPaths(report, '/ws/alpha', ops);

    expect(out.entries[0].specFile).toBe('/elsewhere/specs/spec-a.spec.yaml');
    expect(out.entries[0].testFiles).toEqual(['/elsewhere/a.test.ts']);
    expect(out.parseErrors?.[0].file).toBe('/elsewhere/specs/broken.spec.yaml');
  });

  it('does not mutate the input report', () => {
    const report = makeReport([makeEntry('spec-a')], {
      parseErrors: [{ file: 'specs/broken.spec.yaml', message: 'bad' }],
    });
    const snapshot = JSON.parse(JSON.stringify(report));

    normalizeReportPaths(report, '/ws/alpha', ops);

    expect(report).toEqual(snapshot);
  });

  it('tolerates missing specFile, null-ish testFiles, and absent parseErrors', () => {
    const entry = makeEntry('spec-a', { specFile: undefined });
    // The Go CLI emits null (not []) for empty slices via omitempty.
    (entry as unknown as { testFiles: null }).testFiles = null;
    const report = makeReport([entry]);

    const out = normalizeReportPaths(report, '/ws/alpha', ops);

    expect(out.entries[0].specFile).toBeUndefined();
    expect(out.parseErrors).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// AC-54 / AC-22: per-folder report isolation in the store
// ---------------------------------------------------------------------------

// @ac AC-54
// @ac AC-22
describe('[spec-vscode/AC-54] CoverageReportStore — two-folder isolation', () => {
  const reportA = () => makeReport([makeEntry('spec-a')], {
    parseErrors: [{ file: 'specs/broken-a.spec.yaml', message: 'bad yaml in A', line: 2 }],
  });
  const reportB = () => makeReport([makeEntry('spec-b')], {
    parseErrors: [{ file: 'specs/broken-b.spec.yaml', message: 'bad yaml in B', line: 7 }],
  });

  it("storing folder B's report does not overwrite folder A's", () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/alpha', reportA(), '/ws/alpha');
    store.set('/ws/beta', reportB(), '/ws/beta');

    expect(store.get('/ws/alpha')?.entries.map(e => e.specID)).toEqual(['spec-a']);
    expect(store.get('/ws/beta')?.entries.map(e => e.specID)).toEqual(['spec-b']);
  });

  it("folder B's parse-error path resolves into folder B, not folder A", () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/alpha', reportA(), '/ws/alpha');
    store.set('/ws/beta', reportB(), '/ws/beta');

    expect(store.get('/ws/alpha')?.parseErrors?.[0].file)
      .toBe('/ws/alpha/specs/broken-a.spec.yaml');
    expect(store.get('/ws/beta')?.parseErrors?.[0].file)
      .toBe('/ws/beta/specs/broken-b.spec.yaml');
  });

  it('merged() contains both folders entries and parse errors', () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/alpha', reportA(), '/ws/alpha');
    store.set('/ws/beta', reportB(), '/ws/beta');

    const merged = store.merged();
    expect(merged?.entries.map(e => e.specID).sort()).toEqual(['spec-a', 'spec-b']);
    expect(merged?.parseErrors?.map(e => e.file).sort()).toEqual([
      '/ws/alpha/specs/broken-a.spec.yaml',
      '/ws/beta/specs/broken-b.spec.yaml',
    ]);
    expect(merged?.summary.totalSpecs).toBe(2);
  });

  it('delete(key) removes that folder from the merged view (folder removal)', () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/alpha', reportA(), '/ws/alpha');
    store.set('/ws/beta', reportB(), '/ws/beta');
    store.delete('/ws/alpha');

    expect(store.get('/ws/alpha')).toBeUndefined();
    expect(store.merged()?.entries.map(e => e.specID)).toEqual(['spec-b']);
  });

  it('re-storing under the same key replaces only that folder (on-save refresh path)', () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/alpha', reportA(), '/ws/alpha');
    store.set('/ws/beta', reportB(), '/ws/beta');

    const freshA = makeReport([makeEntry('spec-a', { coveragePct: 100, passesThreshold: true })]);
    store.set('/ws/alpha', freshA, '/ws/alpha');

    expect(store.get('/ws/alpha')?.entries[0].coveragePct).toBe(100);
    expect(store.get('/ws/alpha')?.parseErrors).toBeUndefined();
    expect(store.get('/ws/beta')?.entries.map(e => e.specID)).toEqual(['spec-b']);
  });
});

// @ac AC-54
describe('[spec-vscode/AC-54] CoverageReportStore — single-root regression', () => {
  it('merged() with one folder IS that folder normalized report (identity, not a copy)', () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/solo', makeReport([makeEntry('spec-a')]), '/ws/solo');

    expect(store.merged()).toBe(store.get('/ws/solo'));
  });

  it('merged() is null when no report has been stored (sidebar "not run yet" state preserved)', () => {
    const store = new CoverageReportStore(ops);
    expect(store.merged()).toBeNull();
  });

  it('single folder, all-empty report keeps the empty shape (greenfield message state)', () => {
    const store = new CoverageReportStore(ops);
    store.set('/ws/solo', makeReport([], { specCandidatesCount: 0 }), '/ws/solo');

    const merged = store.merged();
    expect(merged?.entries).toEqual([]);
    expect(merged?.parseErrors).toBeUndefined();
    expect(merged?.specCandidatesCount).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// AC-55: on-save refresh — fresh entry located by specFile, never entries[0]
// ---------------------------------------------------------------------------

// @ac AC-55
describe('[spec-vscode/AC-55] findEntryBySpecFile', () => {
  const entries = [
    makeEntry('spec-alpha', { specFile: '/ws/solo/specs/spec-alpha.spec.yaml' }),
    makeEntry('spec-zulu', { specFile: '/ws/solo/specs/spec-zulu.spec.yaml' }),
  ];

  it('returns the entry whose specFile matches the saved document, not entries[0]', () => {
    const hit = findEntryBySpecFile(entries, '/ws/solo/specs/spec-zulu.spec.yaml');
    expect(hit?.specID).toBe('spec-zulu');
  });

  it('matches a relative stored specFile against an absolute query (suffix match)', () => {
    const rel = [makeEntry('spec-zulu')]; // specFile: specs/spec-zulu.spec.yaml
    const hit = findEntryBySpecFile(rel, '/ws/solo/specs/spec-zulu.spec.yaml');
    expect(hit?.specID).toBe('spec-zulu');
  });

  it('returns undefined when nothing matches (no duplicate-entry pushes)', () => {
    expect(findEntryBySpecFile(entries, '/ws/solo/specs/spec-nope.spec.yaml')).toBeUndefined();
  });

  it('returns undefined for an empty entry list', () => {
    expect(findEntryBySpecFile([], '/ws/solo/specs/spec-zulu.spec.yaml')).toBeUndefined();
  });
});

// @ac AC-55
describe('[spec-vscode/AC-55] on-save handler — static analysis of extension.ts', () => {
  it('no longer gates the coverage refresh on the // @spec slash-comment regex', () => {
    expect(readExtSrc()).not.toMatch(/shouldRunCoverageForFile/);
  });

  it('no longer splices entries[0] of the fresh report into another spec slot', () => {
    const src = readExtSrc();
    expect(src).not.toMatch(/entries\?\.\[0\]/);
    expect(src).not.toMatch(/findIndex\(e\s*=>\s*e\.specID\s*===\s*specID\)/);
  });

  it('routes the on-save refresh through the same per-folder run the initial load uses', () => {
    const src = readExtSrc();
    // The save handler must call runCoverageForFolder and locate the saved
    // spec's fresh entry by specFile — not by regex-derived spec ID.
    expect(src).toMatch(/findEntryBySpecFile/);
    // Initial load + runSync + on-save = at least 3 call sites.
    const calls = src.match(/runCoverageForFolder\(/g) ?? [];
    expect(calls.length).toBeGreaterThanOrEqual(3);
  });
});

// @ac AC-55
describe('[spec-vscode/AC-55] client.coverage — static analysis of client.ts', () => {
  it('the dead specID parameter is gone (CLI has no per-spec filter)', () => {
    const src = readClientSrc();
    expect(src).not.toMatch(/coverage\(specID/);
    expect(src).not.toMatch(/void specID/);
  });
});

// ---------------------------------------------------------------------------
// AC-54: extension.ts holds per-folder reports, not one module global
// ---------------------------------------------------------------------------

// @ac AC-54
describe('[spec-vscode/AC-54] report state — static analysis of extension.ts', () => {
  it('replaces the single module-global coverageReport with a per-folder store', () => {
    const src = readExtSrc();
    expect(src).not.toMatch(/let coverageReport\b/);
    expect(src).toMatch(/new CoverageReportStore\(/);
  });

  it('parse-error diagnostics no longer resolve paths against workspaceFolders[0] inline', () => {
    const src = readExtSrc();
    const start = src.indexOf('function pushCoverageParseDiagnostics');
    expect(start).toBeGreaterThan(-1);
    const end = src.indexOf('\nfunction ', start + 1);
    const body = src.slice(start, end === -1 ? undefined : end);
    expect(body).not.toMatch(/workspaceFolders/);
  });
});

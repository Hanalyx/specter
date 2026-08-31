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

import { snakeToCamelCoverage } from '../client';
import { CoverageReportStore } from '../coverage';
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

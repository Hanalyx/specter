/**
 * stopReasonIntegration.test.ts -- the same AC-79 client claim, against the
 * real binary, spec-vscode 5.1.0 C-33 / AC-79.
 *
 * ADDITIONAL evidence, never the required test. It skips when bin/specter is
 * absent, and a skipped test that a criterion depends on lets a clean checkout
 * report the criterion covered while running none of it. An existing binary
 * can also be stale. The binding version runs against a scripted CLI in
 * stopReason.test.ts.
 *
 * What this adds that the scripted version cannot: proof the real producer and
 * the real consumer agree on the wire spelling. The scripted test asserts the
 * client converts a document; this one asserts the document the CLI actually
 * writes is the one it converts.
 *
 * @spec spec-vscode
 */

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

import { SpecterClient } from '../client';
import type { CoverageReport } from '../types';

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const CLI = path.join(REPO_ROOT, 'bin', 'specter');
const describeWithCLI = fs.existsSync(CLI) ? describe : describe.skip;

describeWithCLI('spec-vscode/AC-79 the real binary agrees with the client', () => {
  let ws: string;

  beforeEach(() => {
    ws = fs.mkdtempSync(path.join(os.tmpdir(), 'stopreason-int-'));
    fs.mkdirSync(path.join(ws, 'specs'), { recursive: true });
    // A manifest the loader rejects. SpecterClient.coverage() passes
    // --strictness annotation, and this still refuses under it.
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

  it('a refused run reaches the client as a typed stopReason', async () => {
    const client = new SpecterClient({
      binaryPath: CLI,
      manifestPath: path.join(ws, 'specter.yaml'),
      workspaceFolder: ws,
    });
    const report = (await client.coverage()) as CoverageReport &
      Record<string, unknown>;

    const reason = report.stopReason as Record<string, unknown> | undefined;
    expect(reason).toBeDefined();
    expect(reason?.kind).toBe('manifest_error');
    expect(report).not.toHaveProperty('stop_reason');
  });
});

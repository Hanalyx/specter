// @spec spec-vscode
//
// Integration test for SpecterClient — invokes the real `specter` CLI
// binary. Catches flag-hallucination bugs (the v0.8.2 --manifest / --spec /
// --base mismatch that shipped with every release v1.0 through v0.8.1).
//
// Requires the specter CLI built at ../bin/specter (relative to specter/).
// Skipped if the binary isn't there — so CI needs `make build` first.

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { SpecterClient } from '../client';

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..');
const CLI = path.join(REPO_ROOT, 'bin', 'specter');

const describeOrSkip = fs.existsSync(CLI) ? describe : describe.skip;

describeOrSkip('SpecterClient integration (real CLI)', () => {
  let workspaceDir: string;
  let manifestPath: string;

  beforeAll(() => {
    // Build a minimal spec workspace in a tmpdir and point SpecterClient at it.
    workspaceDir = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-client-test-'));
    fs.mkdirSync(path.join(workspaceDir, 'specs'), { recursive: true });
    fs.writeFileSync(
      path.join(workspaceDir, 'specs', 'sample.spec.yaml'),
      [
        'spec:',
        '  id: sample',
        '  version: "1.0.0"',
        '  status: draft',
        '  tier: 3',
        '  context:',
        '    system: test',
        '  objective:',
        '    summary: test',
        '  constraints:',
        '    - id: C-01',
        '      description: "test"',
        '  acceptance_criteria:',
        '    - id: AC-01',
        '      description: "test"',
        '      references_constraints: ["C-01"]',
        '',
      ].join('\n'),
    );
    manifestPath = path.join(workspaceDir, 'specter.yaml');
    fs.writeFileSync(
      manifestPath,
      [
        'system:',
        '  name: test',
        '  tier: 2',
        'settings:',
        '  specs_dir: specs',
        '',
      ].join('\n'),
    );
  });

  afterAll(() => {
    fs.rmSync(workspaceDir, { recursive: true, force: true });
  });

  const makeClient = () => new SpecterClient({
    binaryPath: CLI,
    manifestPath,
    workspaceFolder: workspaceDir,
  });

  // @ac AC-04
  it('parse() invokes the CLI without any unknown flags', async () => {
    const client = makeClient();
    const result = await client.parse(path.join('specs', 'sample.spec.yaml'));
    expect(result).toBeTruthy();
    // If the CLI had rejected a flag, execFile would have thrown before we got here.
  });

  // @ac AC-04
  it('check() runs without "unknown flag" errors', async () => {
    const client = makeClient();
    // check may emit diagnostics or nothing; we only care that it doesn't throw.
    await expect(client.check()).resolves.toBeDefined();
  });

  // @ac AC-04
  it('coverage() runs without "unknown flag" errors (the v0.8.1 regression)', async () => {
    const client = makeClient();
    const result = await client.coverage();
    expect(result).toBeTruthy();
    expect(Array.isArray(result.entries)).toBe(true);
  });
});

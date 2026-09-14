// @spec spec-vscode
//
// C-34: the CLI the extension runs is separate from the CLI the user's shell
// runs. These tests bind the pure resolution plan, the range gate, and the
// one permitted write to the user's copy.
//
// The new functions are reached through require() rather than a typed
// import. ts-jest runs strict, so a typed import of a symbol that does not
// exist yet is a build failure, and a build failure is not a red test. Each
// test asserts the function exists first, so the red names what is missing.

import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// eslint-disable-next-line @typescript-eslint/no-var-requires, @typescript-eslint/no-explicit-any
const mod: any = require('../binaryDiscovery');

const USER_BIN = '/home/u/.specter/bin/specter';
const PRIVATE_DIR = '/home/u/.specter/cli';
const RANGE = '>=0.15.0 <0.16.0';

/** A filesystem double keyed by path: which files exist and are executable. */
function fsWith(paths: string[]) {
  return {
    exists: (p: string) => paths.includes(p),
    isExecutable: (p: string) => paths.includes(p),
  };
}

/** Builds plan options for one scenario. versions maps a path to what --version reports. */
function planOpts(overrides: {
  which?: string | null;
  present?: string[];
  versions?: Record<string, string | null>;
  workspaceSetting?: string | null;
  privateVersion?: string;
}) {
  const versions = overrides.versions ?? {};
  return {
    workspaceSetting: overrides.workspaceSetting ?? null,
    which: (_: string) => overrides.which ?? null,
    fs: fsWith(overrides.present ?? []),
    probeVersion: (p: string) => (p in versions ? versions[p] : null),
    userBinPath: USER_BIN,
    privateDir: PRIVATE_DIR,
    privateVersion: overrides.privateVersion ?? '0.15.0',
    range: RANGE,
    platform: 'linux',
  };
}

// @ac AC-80
describe('[spec-vscode/AC-80] planBinaryResolution gates PATH and user-dir candidates by the declared range', () => {
  it('exports planBinaryResolution', () => {
    expect(typeof mod.planBinaryResolution).toBe('function');
  });

  it('uses a PATH binary inside the range as is: no download, no write', () => {
    const plan = mod.planBinaryResolution(planOpts({
      which: '/usr/local/bin/specter',
      present: ['/usr/local/bin/specter'],
      versions: { '/usr/local/bin/specter': '0.15.1' },
    }));
    expect(plan.resolved).toBe('/usr/local/bin/specter');
    expect(plan.source).toBe('path');
    expect(plan.download).toBeNull();
    expect(plan.writes).toEqual([]);
    expect(plan.skipped).toEqual([]);
  });

  it('skips a PATH binary above the range, names it, uses the private copy, and never writes the candidate', () => {
    const plan = mod.planBinaryResolution(planOpts({
      which: '/usr/local/bin/specter',
      present: ['/usr/local/bin/specter'],
      versions: { '/usr/local/bin/specter': '0.16.0' },
    }));
    expect(plan.source).toBe('private');
    expect(plan.resolved).toBe(path.join(PRIVATE_DIR, 'specter-0.15.0'));
    expect(plan.download).toEqual({ version: '0.15.0', target: path.join(PRIVATE_DIR, 'specter-0.15.0') });
    expect(plan.skipped).toEqual([{ path: '/usr/local/bin/specter', version: '0.16.0', range: RANGE }]);
    expect(plan.writes).not.toContain('/usr/local/bin/specter');
    expect(plan.writes).not.toContain(USER_BIN);
  });

  it('skips a user-dir binary below the range the same way', () => {
    const plan = mod.planBinaryResolution(planOpts({
      present: [USER_BIN],
      versions: { [USER_BIN]: '0.14.1' },
    }));
    expect(plan.source).toBe('private');
    expect(plan.skipped).toEqual([{ path: USER_BIN, version: '0.14.1', range: RANGE }]);
    expect(plan.writes).not.toContain(USER_BIN);
  });

  it('uses a user-dir binary inside the range as is', () => {
    const plan = mod.planBinaryResolution(planOpts({
      present: [USER_BIN],
      versions: { [USER_BIN]: '0.15.1' },
    }));
    expect(plan.resolved).toBe(USER_BIN);
    expect(plan.source).toBe('user-dir');
    expect(plan.download).toBeNull();
    expect(plan.writes).toEqual([]);
  });

  it('treats the user dir found through PATH as one candidate, not two', () => {
    // On a machine where the shell PATH command ran, which() returns the
    // user-dir path itself. It must be considered once.
    const plan = mod.planBinaryResolution(planOpts({
      which: USER_BIN,
      present: [USER_BIN],
      versions: { [USER_BIN]: '0.16.0' },
    }));
    expect(plan.skipped).toHaveLength(1);
  });

  it('a candidate whose version cannot be read is not a candidate', () => {
    const plan = mod.planBinaryResolution(planOpts({
      which: '/usr/local/bin/specter',
      present: ['/usr/local/bin/specter'],
      versions: { '/usr/local/bin/specter': null },
    }));
    expect(plan.source).toBe('private');
    expect(plan.skipped).toEqual([]);
    expect(plan.writes).not.toContain('/usr/local/bin/specter');
  });

  it('uses an existing private copy without planning a download', () => {
    const priv = path.join(PRIVATE_DIR, 'specter-0.15.0');
    const plan = mod.planBinaryResolution(planOpts({
      present: [priv],
      versions: { [priv]: '0.15.0' },
    }));
    expect(plan.resolved).toBe(priv);
    expect(plan.source).toBe('private');
    expect(plan.download).toBeNull();
  });
});

// @ac AC-81
describe('[spec-vscode/AC-81] nothing automatic writes the user copy', () => {
  it('with no candidate anywhere, the download targets the private path and nothing else', () => {
    const plan = mod.planBinaryResolution(planOpts({}));
    const target = path.join(PRIVATE_DIR, 'specter-0.15.0');
    expect(plan.download).toEqual({ version: '0.15.0', target });
    expect(plan.writes).toEqual([target]);
  });

  it('names the private copy with .exe on Windows', () => {
    expect(typeof mod.privateBinaryPath).toBe('function');
    expect(mod.privateBinaryPath(PRIVATE_DIR, '0.15.0', 'win32')).toBe(path.join(PRIVATE_DIR, 'specter-0.15.0.exe'));
    expect(mod.privateBinaryPath(PRIVATE_DIR, '0.15.0', 'linux')).toBe(path.join(PRIVATE_DIR, 'specter-0.15.0'));
  });

  it('the Re-download plan targets the private path and no other', () => {
    expect(typeof mod.planRedownload).toBe('function');
    const plan = mod.planRedownload({ privateDir: PRIVATE_DIR, version: '0.15.0', platform: 'linux' });
    const target = path.join(PRIVATE_DIR, 'specter-0.15.0');
    expect(plan.download).toEqual({ version: '0.15.0', target });
    expect(plan.writes).toEqual([target]);
  });

  it('the shell PATH command creates the user copy only when absent, and leaves an existing one alone', () => {
    expect(typeof mod.installUserCopy).toBe('function');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'specter-c34-'));
    try {
      const priv = path.join(dir, 'cli', 'specter-0.15.0');
      fs.mkdirSync(path.dirname(priv), { recursive: true });
      fs.writeFileSync(priv, 'PRIVATE', { mode: 0o755 });
      const user = path.join(dir, 'bin', 'specter');

      const first = mod.installUserCopy(priv, user);
      expect(first.wrote).toBe(true);
      expect(fs.readFileSync(user, 'utf8')).toBe('PRIVATE');

      fs.writeFileSync(user, 'USER OWNED', { mode: 0o755 });
      const second = mod.installUserCopy(priv, user);
      expect(second.wrote).toBe(false);
      expect(second.message).toMatch(/left/i);
      expect(fs.readFileSync(user, 'utf8')).toBe('USER OWNED');
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

// @ac AC-82
describe('[spec-vscode/AC-82] the declared range, its form, and the shipped CLI', () => {
  it('exports satisfiesRange', () => {
    expect(typeof mod.satisfiesRange).toBe('function');
  });

  it('lower bound inclusive, upper bound exclusive, numeric compare, pre-release suffix ignored', () => {
    for (const v of ['0.15.0', '0.15.1', '0.15.10', '0.15.2-rc.1']) {
      expect({ v, ok: mod.satisfiesRange(v, RANGE) }).toEqual({ v, ok: true });
    }
    for (const v of ['0.16.0', '0.14.9']) {
      expect({ v, ok: mod.satisfiesRange(v, RANGE) }).toEqual({ v, ok: false });
    }
  });

  it('rejects any range not in the form >=A.B.C <X.Y.Z', () => {
    for (const r of ['^0.15.0', '>=0.15.0', '0.15.x', '']) {
      expect(() => mod.satisfiesRange('0.15.1', r)).toThrow();
    }
  });

  it('package.json declares specterCli.range in that form', () => {
    const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'package.json'), 'utf8'));
    expect(pkg.specterCli).toBeDefined();
    expect(pkg.specterCli.range).toMatch(/^>=\d+\.\d+\.\d+ <\d+\.\d+\.\d+$/);
  });

  it('the repository VERSION satisfies the declared range, so the shipped extension accepts the shipped CLI', () => {
    const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'package.json'), 'utf8'));
    const version = fs.readFileSync(path.join(__dirname, '..', '..', '..', 'VERSION'), 'utf8').trim();
    expect(mod.satisfiesRange(version, pkg.specterCli.range)).toBe(true);
  });
});

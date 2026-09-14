// @spec spec-vscode
//
// The two decisions the wrapper makes on top of the resolution plan: what
// the shell PATH command may do, and how the extension names the binary
// when it types a command into a terminal for the user.

import { shellInstallDecision, terminalInvocation, BinaryPlan } from '../binaryDiscovery';

const USER_BIN = '/home/u/.specter/bin/specter';
const RANGE = '>=0.15.0 <0.16.0';

function plan(p: Partial<BinaryPlan>): BinaryPlan {
  return { resolved: '/home/u/.specter/cli/specter-0.15.0', source: 'private', download: null, skipped: [], writes: [], ...p };
}

// @ac AC-81
describe('[spec-vscode/AC-81] shellInstallDecision: the user copy is created only when the extension runs its own', () => {
  it('installs and adds PATH when the extension is on its private copy and nothing of the user\'s is on PATH', () => {
    const d = shellInstallDecision(plan({}), USER_BIN);
    expect(d).toMatchObject({ install: true, addPath: true });
  });

  it('adds PATH but does not install when the user copy already exists and is in use', () => {
    const d = shellInstallDecision(plan({ resolved: USER_BIN, source: 'user-dir' }), USER_BIN);
    expect(d).toMatchObject({ install: false, addPath: true });
  });

  it('does nothing when the extension is using a CLI on PATH', () => {
    const d = shellInstallDecision(plan({ resolved: '/usr/local/bin/specter', source: 'path' }), USER_BIN);
    expect(d).toMatchObject({ install: false, addPath: false });
    expect(d.reason).toContain('/usr/local/bin/specter');
  });

  it('does nothing when the extension is using specter.binaryPath', () => {
    const d = shellInstallDecision(plan({ resolved: '/opt/specter', source: 'workspace-setting' }), USER_BIN);
    expect(d).toMatchObject({ install: false, addPath: false });
  });

  it('does nothing when a PATH CLI outside the range was skipped, so the shell keeps the user\'s CLI', () => {
    // The review found this hole: the skipped PATH CLI set the source to
    // private, and the command would then have put ~/.specter/bin ahead of
    // the user's 0.16.0, changing what their shell runs.
    const d = shellInstallDecision(plan({ skipped: [{ path: '/usr/local/bin/specter', version: '0.16.0', range: RANGE }] }), USER_BIN);
    expect(d).toMatchObject({ install: false, addPath: false });
    expect(d.reason).toContain('0.16.0');
    expect(d.reason).toContain(RANGE);
  });

  it('a skipped user-dir copy does not count as a PATH CLI', () => {
    // An out-of-range ~/.specter/bin/specter is the user's and is left
    // alone, but it is not a reason to refuse the PATH edit.
    const d = shellInstallDecision(plan({ skipped: [{ path: USER_BIN, version: '0.14.1', range: RANGE }] }), USER_BIN);
    expect(d).toMatchObject({ install: false, addPath: true });
  });
});

// @ac AC-45
describe('[spec-vscode/AC-45] terminalInvocation names the resolved binary, so terminal commands work without a shell PATH entry', () => {
  it('uses a plain resolved path bare', () => {
    expect(terminalInvocation('/home/u/.specter/cli/specter-0.15.0', 'reverse ', 'linux')).toBe('/home/u/.specter/cli/specter-0.15.0 reverse ');
  });

  it('single-quotes a path with spaces on POSIX', () => {
    expect(terminalInvocation('/Users/a b/.specter/cli/specter-0.15.0', 'diff x', 'darwin')).toBe("'/Users/a b/.specter/cli/specter-0.15.0' diff x");
  });

  it('makes every shell-active character literal on POSIX: backslash, dollar, backtick, double quote', () => {
    const hostile = '/home/u\\x/$HOME/`id`/"q"/specter-0.15.0';
    expect(terminalInvocation(hostile, 'reverse ', 'linux')).toBe(`'${hostile}' reverse `);
  });

  it("writes an embedded single quote as '\\'' on POSIX", () => {
    expect(terminalInvocation("/home/o'brien/specter-0.15.0", 'reverse ', 'linux')).toBe("'/home/o'\\''brien/specter-0.15.0' reverse ");
  });

  it('uses the PowerShell call operator and doubled quotes on Windows', () => {
    expect(terminalInvocation('C:\\Users\\a b\\specter-0.15.0.exe', 'reverse ', 'win32')).toBe("& 'C:\\Users\\a b\\specter-0.15.0.exe' reverse ");
    expect(terminalInvocation("C:\\o'b\\specter.exe", 'reverse ', 'win32')).toBe("& 'C:\\o''b\\specter.exe' reverse ");
    expect(terminalInvocation('C:\\Users\\ab\\specter-0.15.0.exe', 'reverse ', 'win32')).toBe('C:\\Users\\ab\\specter-0.15.0.exe reverse ');
  });

  it('falls back to the bare name when nothing is resolved', () => {
    expect(terminalInvocation(null, 'reverse ', 'linux')).toBe('specter reverse ');
  });
});

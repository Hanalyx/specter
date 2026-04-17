// @spec spec-vscode

import * as path from 'path';
import {
  detectShellConfig,
  isPathAlreadyPresent,
  formatAppendBlock,
  SPECTER_MARKER,
} from '../shellPath';

const HOME = '/home/u';
const BIN = '/home/u/.specter/bin';

// @ac AC-25
describe('detectShellConfig', () => {
  it('resolves bash on Linux to ~/.bashrc', () => {
    const c = detectShellConfig({ shell: '/usr/bin/bash', platform: 'linux', home: HOME }, BIN);
    expect(c).not.toBeNull();
    expect(c!.shell).toBe('bash');
    expect(c!.rcFile).toBe(path.join(HOME, '.bashrc'));
    expect(c!.exportLine).toBe(`export PATH="${BIN}:$PATH"`);
  });

  it('resolves bash on macOS to ~/.bash_profile', () => {
    const c = detectShellConfig({ shell: '/bin/bash', platform: 'darwin', home: HOME }, BIN);
    expect(c).not.toBeNull();
    expect(c!.rcFile).toBe(path.join(HOME, '.bash_profile'));
  });

  it('resolves zsh to ~/.zshrc', () => {
    const c = detectShellConfig({ shell: '/usr/bin/zsh', platform: 'linux', home: HOME }, BIN);
    expect(c).not.toBeNull();
    expect(c!.shell).toBe('zsh');
    expect(c!.rcFile).toBe(path.join(HOME, '.zshrc'));
  });

  it('resolves fish with fish_add_path syntax', () => {
    const c = detectShellConfig({ shell: '/usr/bin/fish', platform: 'linux', home: HOME }, BIN);
    expect(c).not.toBeNull();
    expect(c!.shell).toBe('fish');
    expect(c!.rcFile).toBe(path.join(HOME, '.config', 'fish', 'config.fish'));
    // fish does NOT use POSIX export
    expect(c!.exportLine).toBe(`fish_add_path ${BIN}`);
  });

  it('returns null for unknown shells', () => {
    expect(detectShellConfig({ shell: '/bin/dash', platform: 'linux', home: HOME }, BIN)).toBeNull();
    expect(detectShellConfig({ shell: '', platform: 'linux', home: HOME }, BIN)).toBeNull();
    expect(detectShellConfig({ shell: '/bin/something-weird', platform: 'linux', home: HOME }, BIN)).toBeNull();
  });

  it('is case-insensitive on the shell binary name', () => {
    // Some environments report SHELL with unusual casing.
    const c = detectShellConfig({ shell: '/usr/bin/BASH', platform: 'linux', home: HOME }, BIN);
    expect(c).not.toBeNull();
    expect(c!.shell).toBe('bash');
  });
});

// @ac AC-26
describe('isPathAlreadyPresent', () => {
  it('returns false for empty contents', () => {
    expect(isPathAlreadyPresent('', BIN)).toBe(false);
  });

  it('returns true when a non-comment line references the bin dir', () => {
    const contents = `export FOO=1\nexport PATH="${BIN}:$PATH"\n`;
    expect(isPathAlreadyPresent(contents, BIN)).toBe(true);
  });

  it('ignores the reference if it is inside a comment', () => {
    const contents = `# old config used to be: export PATH="${BIN}:$PATH"\nexport FOO=1\n`;
    expect(isPathAlreadyPresent(contents, BIN)).toBe(false);
  });

  it('detects fish-style reference', () => {
    expect(isPathAlreadyPresent(`fish_add_path ${BIN}\n`, BIN)).toBe(true);
  });

  it('is idempotent — after appending, another check returns true', () => {
    const before = '';
    const block = formatAppendBlock(`export PATH="${BIN}:$PATH"`);
    const after = before + block;
    expect(isPathAlreadyPresent(after, BIN)).toBe(true);
  });
});

// @ac AC-26
describe('formatAppendBlock', () => {
  it('includes the Specter marker comment', () => {
    const block = formatAppendBlock('export PATH="X:$PATH"');
    expect(block).toContain(SPECTER_MARKER);
    expect(block).toContain('export PATH="X:$PATH"');
  });

  it('starts with a blank line to avoid merging with preceding content', () => {
    const block = formatAppendBlock('export PATH="X:$PATH"');
    expect(block.startsWith('\n')).toBe(true);
  });
});

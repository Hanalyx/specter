// @spec spec-vscode
//
// Tests for binary discovery and auto-download logic.
// All functions under test are pure or injectable — no VS Code runtime required.

import { resolveBinaryPath, verifyChecksum, buildDownloadUrl, isBinaryFile, validateVersion } from '../binaryDiscovery';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

const mockFs = {
  exists: (path: string): boolean => false,
  isExecutable: (path: string): boolean => false,
};

const mockWhich = (name: string): string | null => null;

// ---------------------------------------------------------------------------
// AC-02: Binary discovery — workspace setting → PATH → cache → auto-download
// ---------------------------------------------------------------------------

// @ac AC-02
describe('[spec-vscode/AC-02] resolveBinaryPath', () => {
  it('returns workspace setting path when specter.binaryPath is set and file exists', () => {
    const fs = { ...mockFs, exists: (p: string) => p === '/custom/specter' };
    const result = resolveBinaryPath({
      workspaceSetting: '/custom/specter',
      which: mockWhich,
      fs,
      cachePath: '~/.specter/bin/specter',
    });
    expect(result.resolved).toBe('/custom/specter');
    expect(result.source).toBe('workspace-setting');
  });

  it('falls through to PATH when workspace setting is absent', () => {
    const which = (name: string) => name === 'specter' ? '/usr/local/bin/specter' : null;
    const result = resolveBinaryPath({
      workspaceSetting: null,
      which,
      fs: { ...mockFs, exists: () => true },
      cachePath: '~/.specter/bin/specter',
    });
    expect(result.resolved).toBe('/usr/local/bin/specter');
    expect(result.source).toBe('path');
  });

  it('falls through to cache path when PATH lookup fails', () => {
    const fs = { ...mockFs, exists: (p: string) => p === '/home/user/.specter/bin/specter', isExecutable: () => true };
    const result = resolveBinaryPath({
      workspaceSetting: null,
      which: () => null,
      fs,
      cachePath: '/home/user/.specter/bin/specter',
    });
    expect(result.resolved).toBe('/home/user/.specter/bin/specter');
    expect(result.source).toBe('cache');
  });

  it('returns needs-download when all resolution strategies fail', () => {
    const result = resolveBinaryPath({
      workspaceSetting: null,
      which: () => null,
      fs: { ...mockFs, exists: () => false },
      cachePath: '/home/user/.specter/bin/specter',
    });
    expect(result.resolved).toBeNull();
    expect(result.source).toBe('needs-download');
  });

  it('rejects workspace setting path that does not exist on disk', () => {
    const result = resolveBinaryPath({
      workspaceSetting: '/nonexistent/specter',
      which: () => null,
      fs: { ...mockFs, exists: () => false },
      cachePath: '~/.specter/bin/specter',
    });
    // Must not return the non-existent setting; must fall through
    expect(result.source).not.toBe('workspace-setting');
  });
});

// @ac AC-02
describe('[spec-vscode/AC-02] buildDownloadUrl', () => {
  it('constructs GitHub Releases URL for linux-amd64', () => {
    const url = buildDownloadUrl({ version: '0.5.0', os: 'linux', arch: 'amd64' });
    expect(url).toContain('specter_0.5.0_linux_amd64.tar.gz');
    expect(url).toContain('releases/download/v0.5.0');
  });

  it('constructs GitHub Releases URL for darwin-arm64', () => {
    const url = buildDownloadUrl({ version: '0.5.0', os: 'darwin', arch: 'arm64' });
    expect(url).toContain('specter_0.5.0_darwin_arm64.tar.gz');
  });

  it('maps VS Code runner.arch X64 → amd64 and ARM64 → arm64', () => {
    expect(buildDownloadUrl({ version: '0.5.0', os: 'linux', arch: 'X64' }).includes('amd64')).toBe(true);
    expect(buildDownloadUrl({ version: '0.5.0', os: 'linux', arch: 'ARM64' }).includes('arm64')).toBe(true);
  });

  it('maps Node process.arch lowercase x64 → amd64 and arm64 → arm64', () => {
    // Regression: pre-v0.6.7 the switch only matched uppercase VS Code
    // runner.arch values, so Node's lowercase process.arch fell through to
    // default and produced URLs like .../specter_0.6.6_linux_x64.tar.gz
    // which 404 against goreleaser's .../specter_0.6.6_linux_amd64.tar.gz.
    expect(buildDownloadUrl({ version: '0.5.0', os: 'linux', arch: 'x64' })).toContain('specter_0.5.0_linux_amd64.tar.gz');
    expect(buildDownloadUrl({ version: '0.5.0', os: 'linux', arch: 'arm64' })).toContain('specter_0.5.0_linux_arm64.tar.gz');
    expect(buildDownloadUrl({ version: '0.5.0', os: 'darwin', arch: 'x64' })).toContain('specter_0.5.0_darwin_amd64.tar.gz');
    expect(buildDownloadUrl({ version: '0.5.0', os: 'darwin', arch: 'arm64' })).toContain('specter_0.5.0_darwin_arm64.tar.gz');
  });
});

// @ac AC-23
describe('[spec-vscode/AC-23] isBinaryFile', () => {
  function writeTmp(contents: Buffer): string {
    const p = path.join(os.tmpdir(), `specter-binary-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    fs.writeFileSync(p, contents);
    return p;
  }

  it('returns true for a file starting with ELF magic bytes', () => {
    const p = writeTmp(Buffer.from([0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01]));
    try {
      expect(isBinaryFile(p)).toBe(true);
    } finally {
      fs.unlinkSync(p);
    }
  });

  it('returns false for a file starting with text like "Not Found"', () => {
    // Reproduces the failure mode where a corrupt download left an HTTP
    // error body in place of the binary. The user would otherwise see
    // "line 1: Not: command not found" when trying to execute it.
    const p = writeTmp(Buffer.from('Not Found\n'));
    try {
      expect(isBinaryFile(p)).toBe(false);
    } finally {
      fs.unlinkSync(p);
    }
  });

  it('returns false for an HTML error page', () => {
    const p = writeTmp(Buffer.from('<!DOCTYPE html><html>error</html>'));
    try {
      expect(isBinaryFile(p)).toBe(false);
    } finally {
      fs.unlinkSync(p);
    }
  });

  it('returns false for a missing file (no throw)', () => {
    expect(isBinaryFile('/nonexistent/path/to/specter')).toBe(false);
  });
});

// @ac AC-02
describe('[spec-vscode/AC-02] verifyChecksum', () => {
  it('returns true when SHA256 of content matches expected checksum', async () => {
    // Minimal smoke test — real checksum verification uses crypto.subtle
    const content = Buffer.from('specter binary content');
    const expectedHash = require('crypto')
      .createHash('sha256')
      .update(content)
      .digest('hex');
    const ok = await verifyChecksum(content, expectedHash);
    expect(ok).toBe(true);
  });

  it('returns false when checksum does not match', async () => {
    const content = Buffer.from('specter binary content');
    const ok = await verifyChecksum(content, 'deadbeef');
    expect(ok).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// v0.6.5+ — Error-state recovery affordances (AC-24)
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-24
describe('[spec-vscode/AC-24] Specter: Re-download CLI command is declared in package.json', () => {
  // AC-24 pins the presence of the user-facing recovery command. The status-
  // bar error transition itself is vscode-runtime-coupled and exercised by
  // client.test.ts (which invokes the real CLI against a fixture workspace
  // that would fail coverage); the palette-entry existence is a pure
  // structural fact about package.json.
  it('package.json declares the specter.redownloadCli command', () => {
    const pkg = require('../../package.json');
    const commands: Array<{ command: string; title: string }> = pkg.contributes?.commands ?? [];
    const found = commands.find(c => c.command === 'specter.redownloadCli');
    expect(found).toBeTruthy();
    expect(found?.title.toLowerCase()).toContain('re-download');
  });

  it('package.json also declares the specter.showOutput command', () => {
    const pkg = require('../../package.json');
    const commands: Array<{ command: string; title: string }> = pkg.contributes?.commands ?? [];
    const found = commands.find(c => c.command === 'specter.showOutput');
    expect(found).toBeTruthy();
  });
});

// @ac AC-50
describe('[spec-vscode/AC-50] specter.version config default (C-27)', () => {
  it('package.json declares specter.version default as empty string, not "latest"', () => {
    const pkg = require('../../package.json');
    const prop = pkg.contributes?.configuration?.properties?.['specter.version'];
    expect(prop).toBeTruthy();
    expect(prop.default).toBe('');
    // Schema drift guard: if a future change reverts to 'latest' as the
    // default, version skew between the Marketplace extension and the
    // GoReleaser-produced GitHub Release reappears. Keep the default empty
    // so downloadBinary reads ctx.extension.packageJSON.version.
  });
});

describe('[spec-vscode] validateVersion — input validation for version strings used in URLs', () => {
  it('accepts plain semver MAJOR.MINOR.PATCH', () => {
    expect(() => validateVersion('0.10.2')).not.toThrow();
    expect(() => validateVersion('1.0.0')).not.toThrow();
    expect(() => validateVersion('123.456.789')).not.toThrow();
  });

  it('accepts semver with pre-release suffix', () => {
    expect(() => validateVersion('0.10.0-rc.1')).not.toThrow();
    expect(() => validateVersion('1.0.0-beta')).not.toThrow();
    expect(() => validateVersion('0.10.0-pre.20260425')).not.toThrow();
  });

  it('rejects strings with path separators (URL injection guard)', () => {
    expect(() => validateVersion('0.10.0/../../attacker/evil/releases/download/v1.0.0')).toThrow();
    expect(() => validateVersion('0.10.0/extra')).toThrow();
    expect(() => validateVersion('../../malicious')).toThrow();
  });

  it('rejects strings with whitespace, query strings, or special URL chars', () => {
    expect(() => validateVersion('0.10.0 ')).toThrow();
    expect(() => validateVersion('0.10.0?token=abc')).toThrow();
    expect(() => validateVersion('0.10.0#frag')).toThrow();
    expect(() => validateVersion('0.10.0\n0.10.0')).toThrow();
  });

  it('rejects empty string and non-string inputs', () => {
    expect(() => validateVersion('')).toThrow();
    expect(() => validateVersion(undefined as unknown as string)).toThrow();
    expect(() => validateVersion(null as unknown as string)).toThrow();
    expect(() => validateVersion(123 as unknown as string)).toThrow();
  });

  it('rejects "latest" — callers must resolve it to a concrete version first', () => {
    // resolveLatestVersion() in binaryDiscovery.ts queries the GitHub API and
    // returns a concrete tag; that result is what flows into URL construction.
    expect(() => validateVersion('latest')).toThrow();
  });

  it('buildDownloadUrl propagates the validation error', () => {
    expect(() => buildDownloadUrl({ version: '0.10.0/evil', os: 'linux', arch: 'amd64' })).toThrow(/invalid specter version/);
  });
});

describe('[spec-vscode] package.json declares machine-scope and untrusted-workspace capability', () => {
  const pkg = require('../../package.json');

  it('specter.binaryPath is machine-scoped', () => {
    expect(pkg.contributes.configuration.properties['specter.binaryPath'].scope).toBe('machine');
  });

  it('specter.version is machine-scoped', () => {
    expect(pkg.contributes.configuration.properties['specter.version'].scope).toBe('machine');
  });

  it('declares untrustedWorkspaces capability with explanation', () => {
    expect(pkg.capabilities?.untrustedWorkspaces?.supported).toBe('limited');
    expect(typeof pkg.capabilities.untrustedWorkspaces.description).toBe('string');
    expect(pkg.capabilities.untrustedWorkspaces.description.length).toBeGreaterThan(0);
  });
});

// @spec spec-vscode
//
// Tests for binary discovery and auto-download logic.
// All functions under test are pure or injectable — no VS Code runtime required.

import { resolveBinaryPath, verifyChecksum, buildDownloadUrl } from '../binaryDiscovery';

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
describe('resolveBinaryPath', () => {
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
describe('buildDownloadUrl', () => {
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
});

// @ac AC-02
describe('verifyChecksum', () => {
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

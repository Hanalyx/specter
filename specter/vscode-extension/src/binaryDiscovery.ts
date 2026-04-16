// @spec spec-vscode

import * as crypto from 'crypto';
import * as os from 'os';
import * as path from 'path';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface FsAdapter {
  exists: (path: string) => boolean;
  isExecutable: (path: string) => boolean;
}

export interface ResolveBinaryOptions {
  workspaceSetting: string | null;
  which: (name: string) => string | null;
  fs: FsAdapter;
  cachePath: string;
}

export type BinarySource = 'workspace-setting' | 'path' | 'cache' | 'needs-download';

export interface BinaryResolution {
  resolved: string | null;
  source: BinarySource;
}

export interface DownloadUrlOptions {
  version: string;
  os: string;
  arch: string;
}

// ---------------------------------------------------------------------------
// AC-02: Binary discovery — workspace setting → PATH → cache → auto-download
// ---------------------------------------------------------------------------

/**
 * Resolves the specter binary path using the documented priority order:
 * 1. Workspace setting (specter.binaryPath) — if file exists on disk
 * 2. PATH lookup via which()
 * 3. Cache path (~/.specter/bin/specter) — if file exists on disk
 * 4. Needs download
 */
export function resolveBinaryPath(opts: ResolveBinaryOptions): BinaryResolution {
  // 1. Workspace setting
  if (opts.workspaceSetting && opts.fs.exists(opts.workspaceSetting)) {
    return { resolved: opts.workspaceSetting, source: 'workspace-setting' };
  }

  // 2. PATH
  const fromPath = opts.which('specter');
  if (fromPath && opts.fs.exists(fromPath)) {
    return { resolved: fromPath, source: 'path' };
  }

  // 3. Cache
  if (opts.fs.exists(opts.cachePath)) {
    return { resolved: opts.cachePath, source: 'cache' };
  }

  // 4. Needs download
  return { resolved: null, source: 'needs-download' };
}

// ---------------------------------------------------------------------------
// AC-02: Download URL construction
// ---------------------------------------------------------------------------

/** Maps VS Code runner.arch values to Go GOARCH values. */
function normaliseArch(arch: string): string {
  switch (arch) {
    case 'X64':   return 'amd64';
    case 'ARM64': return 'arm64';
    case 'IA32':  return '386';
    default:      return arch.toLowerCase();
  }
}

/** Maps process.platform / os values to Go GOOS values. */
function normaliseOS(platform: string): string {
  switch (platform) {
    case 'win32':  return 'windows';
    case 'darwin': return 'darwin';
    default:       return 'linux';
  }
}

/**
 * Constructs the GitHub Releases download URL for a given
 * version / os / arch triple.
 */
export function buildDownloadUrl(opts: DownloadUrlOptions): string {
  const goOS   = normaliseOS(opts.os);
  const goArch = normaliseArch(opts.arch);
  const ext    = goOS === 'windows' ? '.zip' : '.tar.gz';
  const asset  = `specter_${opts.version}_${goOS}_${goArch}${ext}`;
  return `https://github.com/Hanalyx/specter/releases/download/v${opts.version}/${asset}`;
}

/** Default cache path for the auto-downloaded binary. */
export function defaultCachePath(): string {
  return path.join(os.homedir(), '.specter', 'bin', 'specter');
}

// ---------------------------------------------------------------------------
// AC-02: Checksum verification
// ---------------------------------------------------------------------------

/**
 * Returns true when the SHA-256 of `content` equals `expectedHex`.
 * Uses Node's built-in `crypto` module — no network call.
 */
export async function verifyChecksum(content: Buffer, expectedHex: string): Promise<boolean> {
  const actual = crypto.createHash('sha256').update(content).digest('hex');
  return actual === expectedHex;
}

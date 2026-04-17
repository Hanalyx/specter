// @spec spec-vscode

import * as crypto from 'crypto';
import * as fs from 'fs';
import * as https from 'https';
import * as os from 'os';
import * as path from 'path';
import { execFileSync } from 'child_process';

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
  if (opts.fs.exists(opts.cachePath) && opts.fs.isExecutable(opts.cachePath)) {
    return { resolved: opts.cachePath, source: 'cache' };
  }

  // 4. Needs download
  return { resolved: null, source: 'needs-download' };
}

/**
 * Returns true if the file at `filePath` looks like a compiled binary
 * (starts with ELF, Mach-O, or MZ magic bytes) rather than a text file.
 * This catches corrupt downloads where an HTTP error page was saved as
 * the binary (e.g. "Not Found").
 */
export function isBinaryFile(filePath: string): boolean {
  try {
    const fd = fs.openSync(filePath, 'r');
    const buf = Buffer.alloc(4);
    fs.readSync(fd, buf, 0, 4, 0);
    fs.closeSync(fd);

    // ELF (Linux): 0x7f 'E' 'L' 'F'
    if (buf[0] === 0x7f && buf[1] === 0x45 && buf[2] === 0x4c && buf[3] === 0x46) return true;
    // Mach-O (macOS): 0xFEEDFACE, 0xFEEDFACF, 0xCFFAEDFE, 0xCEFAEDFE
    if (buf[0] === 0xfe && buf[1] === 0xed && buf[2] === 0xfa) return true;
    if (buf[0] === 0xcf && buf[1] === 0xfa && buf[2] === 0xed) return true;
    if (buf[0] === 0xce && buf[1] === 0xfa && buf[2] === 0xed) return true;
    // PE (Windows): 'M' 'Z'
    if (buf[0] === 0x4d && buf[1] === 0x5a) return true;

    return false;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// AC-02: Download URL construction
// ---------------------------------------------------------------------------

/** Maps a runtime arch identifier to Go's GOARCH.
 *
 * Accepts both VS Code's `runner.arch` uppercase convention ("X64", "ARM64")
 * and Node's `process.arch` lowercase convention ("x64", "arm64"). The
 * extension calls this with `process.arch`, so the lowercase cases are the
 * hot path; the uppercase cases exist for parity with the GitHub Actions
 * composite action which uses runner.arch.
 */
function normaliseArch(arch: string): string {
  switch (arch.toLowerCase()) {
    case 'x64':   return 'amd64';
    case 'arm64': return 'arm64';
    case 'ia32':  return '386';
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
 * Returns the archive file name for a given version / os / arch triple.
 * Matches goreleaser's naming template: specter_{version}_{os}_{arch}.tar.gz
 */
export function assetName(opts: DownloadUrlOptions): string {
  const goOS   = normaliseOS(opts.os);
  const goArch = normaliseArch(opts.arch);
  const ext    = goOS === 'windows' ? '.zip' : '.tar.gz';
  return `specter_${opts.version}_${goOS}_${goArch}${ext}`;
}

/**
 * Constructs the GitHub Releases download URL for a given
 * version / os / arch triple.  Version must be a resolved semver
 * string (e.g. "0.6.0"), NOT "latest".
 */
export function buildDownloadUrl(opts: DownloadUrlOptions): string {
  return `https://github.com/Hanalyx/specter/releases/download/v${opts.version}/${assetName(opts)}`;
}

/** Default cache path for the auto-downloaded binary. */
export function defaultCachePath(): string {
  const bin = process.platform === 'win32' ? 'specter.exe' : 'specter';
  return path.join(os.homedir(), '.specter', 'bin', bin);
}

// ---------------------------------------------------------------------------
// AC-02: Resolve "latest" to an actual version tag
// ---------------------------------------------------------------------------

const GITHUB_API = 'https://api.github.com/repos/Hanalyx/specter/releases/latest';

/**
 * Resolves the "latest" tag to a concrete semver version string by
 * querying the GitHub Releases API.  Returns e.g. "0.6.0".
 */
export async function resolveLatestVersion(): Promise<string> {
  const body = await httpsGet(GITHUB_API, {
    headers: { 'User-Agent': 'specter-vscode', Accept: 'application/json' },
  });
  const json = JSON.parse(body.toString('utf-8'));
  const tag: string = json.tag_name; // e.g. "v0.6.0"
  return tag.replace(/^v/, '');
}

// ---------------------------------------------------------------------------
// AC-02: Redirect-following HTTPS helper
// ---------------------------------------------------------------------------

interface HttpsGetOptions {
  headers?: Record<string, string>;
}

/**
 * Downloads a URL as a Buffer, following up to 5 redirects.
 * Node's https.get does NOT follow redirects automatically.
 */
const HTTPS_TIMEOUT_MS = 30_000;

export function httpsGet(url: string, opts?: HttpsGetOptions, maxRedirects = 5): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const reqOpts: https.RequestOptions = {
      headers: opts?.headers ?? {},
      timeout: HTTPS_TIMEOUT_MS,
    };
    const req = https.get(url, reqOpts, (res) => {
      // Follow redirects
      if (res.statusCode && res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        if (maxRedirects <= 0) {
          reject(new Error('Too many redirects'));
          return;
        }
        resolve(httpsGet(res.headers.location, opts, maxRedirects - 1));
        return;
      }

      if (res.statusCode && res.statusCode >= 400) {
        reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        return;
      }

      const chunks: Buffer[] = [];
      res.on('data', (c: Buffer) => chunks.push(c));
      res.on('end', () => resolve(Buffer.concat(chunks)));
      res.on('error', reject);
    });
    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy(new Error(`Timed out after ${HTTPS_TIMEOUT_MS}ms fetching ${url}`));
    });
  });
}

// ---------------------------------------------------------------------------
// AC-02: Archive extraction
// ---------------------------------------------------------------------------

/**
 * Extracts the `specter` binary from a downloaded archive and places
 * it at targetPath.  Uses system tar on macOS/Linux and PowerShell
 * Expand-Archive on Windows.
 */
export async function extractBinary(
  archiveData: Buffer,
  format: 'tar.gz' | 'zip',
  targetPath: string,
): Promise<void> {
  const dir = path.dirname(targetPath);
  fs.mkdirSync(dir, { recursive: true });

  // Write archive to a temp file
  const ext = format === 'zip' ? '.zip' : '.tar.gz';
  const tmpArchive = path.join(dir, `specter-download${ext}`);
  fs.writeFileSync(tmpArchive, archiveData);

  try {
    if (format === 'tar.gz') {
      // Extract only the 'specter' binary from the archive
      execFileSync('tar', ['xzf', tmpArchive, '-C', dir, 'specter'], { timeout: 30000 });
    } else {
      // Windows: extract zip then move binary
      const tmpDir = path.join(dir, 'specter-extract');
      execFileSync('powershell', [
        '-NoProfile', '-Command',
        `Expand-Archive -Path '${tmpArchive}' -DestinationPath '${tmpDir}' -Force`,
      ], { timeout: 30000 });
      const extracted = path.join(tmpDir, 'specter.exe');
      fs.copyFileSync(extracted, targetPath);
      fs.rmSync(tmpDir, { recursive: true, force: true });
    }

    // Ensure binary is executable (no-op on Windows)
    if (process.platform !== 'win32') {
      fs.chmodSync(targetPath, 0o755);
    }
  } finally {
    // Clean up temp archive
    try { fs.unlinkSync(tmpArchive); } catch { /* ignore */ }
  }
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

/**
 * Downloads checksums.txt from the release and returns a map of
 * filename → sha256 hex string.  goreleaser format: `<sha256>  <filename>`.
 */
export async function downloadChecksums(version: string): Promise<Map<string, string>> {
  const url = `https://github.com/Hanalyx/specter/releases/download/v${version}/checksums.txt`;
  const data = await httpsGet(url);
  const map = new Map<string, string>();
  for (const line of data.toString('utf-8').split('\n')) {
    const parts = line.trim().split(/\s+/);
    if (parts.length === 2) {
      map.set(parts[1], parts[0]); // filename → hash
    }
  }
  return map;
}

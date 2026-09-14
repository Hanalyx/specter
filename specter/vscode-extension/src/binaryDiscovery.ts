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

export interface DownloadUrlOptions {
  version: string;
  os: string;
  arch: string;
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
 * Strict semver validation for version strings used in download URLs.
 * Accepts MAJOR.MINOR.PATCH with an optional pre-release suffix
 * (alphanumerics, dots, hyphens). The literal string "latest" is NOT
 * valid here — callers must resolve "latest" via resolveLatestVersion()
 * before passing the version into URL construction.
 *
 * Anything outside this shape is rejected with an Error. Without this
 * guard, an attacker-controlled `specter.version` setting (e.g. via a
 * malicious workspace's `.vscode/settings.json`) could inject path
 * separators or query strings into the download URL and redirect to a
 * different repo on the same host, bypassing TLS and checksum verification.
 */
const VALID_VERSION = /^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/;

export function validateVersion(version: string): void {
  if (typeof version !== 'string' || !VALID_VERSION.test(version)) {
    throw new Error(
      `invalid specter version ${JSON.stringify(version)}: ` +
      `expected MAJOR.MINOR.PATCH (e.g. "0.10.2") or "latest"`,
    );
  }
}

/**
 * Returns the archive file name for a given version / os / arch triple.
 * Matches goreleaser's naming template: specter_{version}_{os}_{arch}.tar.gz
 */
export function assetName(opts: DownloadUrlOptions): string {
  validateVersion(opts.version);
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
  validateVersion(opts.version);
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
  validateVersion(version);
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

// ---------------------------------------------------------------------------
// C-34: the extension's CLI is separate from the user's CLI
// ---------------------------------------------------------------------------

/** Where the extension keeps its own copies, one file per version. */
export function privateCliDir(): string {
  return path.join(os.homedir(), '.specter', 'cli');
}

/** The private copy for one version: ~/.specter/cli/specter-<version>[.exe]. */
export function privateBinaryPath(privateDir: string, version: string, platform: string): string {
  const ext = platform === 'win32' ? '.exe' : '';
  return path.join(privateDir, `specter-${version}${ext}`);
}

const RANGE_RE = /^>=(\d+)\.(\d+)\.(\d+) <(\d+)\.(\d+)\.(\d+)$/;

function versionTriple(v: string): [number, number, number] | null {
  const m = /^(\d+)\.(\d+)\.(\d+)/.exec(v);
  if (!m) return null;
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

function cmp(a: [number, number, number], b: [number, number, number]): number {
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] < b[i] ? -1 : 1;
  }
  return 0;
}

/**
 * Reports whether a CLI version satisfies a declared range of the form
 * `>=A.B.C <X.Y.Z`. The lower bound is inclusive, the upper exclusive, the
 * compare is numeric per component, and a pre-release suffix on the
 * candidate is ignored. Any other range form is an error rather than a
 * silent "no": a malformed declaration must fail the build, not disable the
 * gate.
 */
export function satisfiesRange(version: string, range: string): boolean {
  const m = RANGE_RE.exec(range);
  if (!m) {
    throw new Error(`specterCli.range must be ">=A.B.C <X.Y.Z", got ${JSON.stringify(range)}`);
  }
  const lo: [number, number, number] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const hi: [number, number, number] = [Number(m[4]), Number(m[5]), Number(m[6])];
  const v = versionTriple(version);
  if (!v) return false;
  return cmp(v, lo) >= 0 && cmp(v, hi) < 0;
}

export type PlanSource = 'workspace-setting' | 'path' | 'user-dir' | 'private';

export interface SkippedCandidate {
  path: string;
  version: string;
  range: string;
}

export interface BinaryPlan {
  /** The path to run. When a download is planned, it does not exist yet. */
  resolved: string;
  source: PlanSource;
  /** The one download this plan performs, or null. */
  download: { version: string; target: string } | null;
  /** Candidates left untouched because their version is outside the range. */
  skipped: SkippedCandidate[];
  /** Every path this plan writes. Never the user's copy, PATH, or the setting. */
  writes: string[];
}

export interface PlanOptions {
  workspaceSetting: string | null;
  which: (name: string) => string | null;
  fs: FsAdapter;
  /** Runs `--version` on a path; null when the file is not a valid CLI. */
  probeVersion: (p: string) => string | null;
  /** ~/.specter/bin/specter, the user's copy. Read, never written here. */
  userBinPath: string;
  /** ~/.specter/cli, the extension's own directory. */
  privateDir: string;
  /** The version the private copy should be, per C-27. */
  privateVersion: string;
  /** package.json specterCli.range. */
  range: string;
  platform: string;
}

/**
 * The C-34 resolution decision, pure. Order: the workspace setting, used as
 * is because it is the user's explicit choice; then PATH, then the user's
 * copy, each used only when its version satisfies the range and otherwise
 * skipped and left alone; then the private copy, downloaded when absent.
 * The plan lists every write it will make, and that list can only ever
 * name the private copy.
 */
export function planBinaryResolution(opts: PlanOptions): BinaryPlan {
  const skipped: SkippedCandidate[] = [];

  if (opts.workspaceSetting && opts.fs.exists(opts.workspaceSetting)) {
    return { resolved: opts.workspaceSetting, source: 'workspace-setting', download: null, skipped, writes: [] };
  }

  const candidates: Array<{ p: string; source: PlanSource }> = [];
  const fromPath = opts.which('specter');
  if (fromPath && opts.fs.exists(fromPath)) {
    candidates.push({ p: fromPath, source: fromPath === opts.userBinPath ? 'user-dir' : 'path' });
  }
  if (fromPath !== opts.userBinPath && opts.fs.exists(opts.userBinPath) && opts.fs.isExecutable(opts.userBinPath)) {
    candidates.push({ p: opts.userBinPath, source: 'user-dir' });
  }
  for (const c of candidates) {
    const v = opts.probeVersion(c.p);
    if (!v) continue;
    if (satisfiesRange(v, opts.range)) {
      return { resolved: c.p, source: c.source, download: null, skipped, writes: [] };
    }
    skipped.push({ path: c.p, version: v, range: opts.range });
  }

  const target = privateBinaryPath(opts.privateDir, opts.privateVersion, opts.platform);
  if (opts.fs.exists(target) && opts.fs.isExecutable(target) && opts.probeVersion(target)) {
    return { resolved: target, source: 'private', download: null, skipped, writes: [] };
  }
  return {
    resolved: target,
    source: 'private',
    download: { version: opts.privateVersion, target },
    skipped,
    writes: [target],
  };
}

/** The Re-download command's plan: refresh the private copy and nothing else. */
export function planRedownload(opts: { privateDir: string; version: string; platform: string }): { download: { version: string; target: string }; writes: string[] } {
  const target = privateBinaryPath(opts.privateDir, opts.version, opts.platform);
  return { download: { version: opts.version, target }, writes: [target] };
}

/**
 * The one permitted write to the user's copy: the shell PATH command copying
 * the private binary there when nothing is there. An existing file is the
 * user's, whatever its version, and is left byte for byte.
 */
export function installUserCopy(privatePath: string, userBinPath: string): { wrote: boolean; message: string } {
  if (fs.existsSync(userBinPath)) {
    return { wrote: false, message: `${userBinPath} already exists and was left alone. Replace it yourself if you want a different version there.` };
  }
  fs.mkdirSync(path.dirname(userBinPath), { recursive: true });
  fs.copyFileSync(privatePath, userBinPath);
  fs.chmodSync(userBinPath, 0o755);
  return { wrote: true, message: `Installed the CLI at ${userBinPath}.` };
}

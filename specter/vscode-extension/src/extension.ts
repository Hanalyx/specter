// @spec spec-vscode

import * as vscode from 'vscode';
import * as path from 'path';

import { shouldActivate, resolveManifestPath, createClientKey, isSpecFilePath } from './activation';
import {
  resolveBinaryPath,
  buildDownloadUrl,
  defaultCachePath,
  resolveLatestVersion,
  assetName,
  httpsGet,
  extractBinary,
  verifyChecksum,
  downloadChecksums,
  isBinaryFile,
} from './binaryDiscovery';
import { buildDiagnostics, buildCoverageParseDiagnostics, DiagnosticReplacer, shouldRunCoverageForFile } from './diagnostics';
import {
  buildSpecCompletions,
  buildACCompletions,
  buildAnnotationHover,
  buildQuickFix,
  suggestACsForFunction,
  findNearestSpecAnnotation,
} from './annotations';
import {
  formatStatusBar,
  classifyNotification,
  buildFileDecoration,
  buildACDecorations,
  buildCoverageTreeRoot,
  NotificationRateLimiter,
  resolveCoveringFiles,
  formatSyncCompletion,
  resolveWorkspacePathPure,
  matchFileInIndex,
} from './coverage';
import { buildConstraintHover, resolveDefinitionTarget } from './navigation';
import { buildInsightCards, computeInsightsStatus, formatSpecContextForAI, shouldShowWalkthrough } from './insights';
import { detectDrift, buildDriftHover } from './drift';
import { SpecterClient } from './client';
import { detectShellConfig, isPathAlreadyPresent, formatAppendBlock, shouldPromptAddPath } from './shellPath';
import * as crypto from 'crypto';
import * as os from 'os';
import type {
  SpecIndex,
  CoverageReport,
} from './types';

// ---------------------------------------------------------------------------
// Module-level state
// ---------------------------------------------------------------------------

const clients = new Map<string, SpecterClient>();
const diagnosticReplacers = new Map<string, DiagnosticReplacer>();
const diagnosticCollections = new Map<string, vscode.DiagnosticCollection>();
let specIndex: SpecIndex = { specs: {} };
let coverageReport: CoverageReport | null = null;
// Folders where the most recent coverage run failed (parse errors or
// process-level failure). Consulted by specter.runSync to emit an honest
// completion message — see AC-31.
const coverageErrorFolders = new Set<string>();
let statusBarItem: vscode.StatusBarItem | null = null;
let binaryPath: string | null = null;
const rateLimiter = new NotificationRateLimiter({ windowMs: 60_000 });
let treeProvider: SpecterTreeProvider | null = null;
let specterTreeView: vscode.TreeView<unknown> | null = null;
let driftDecorationType: vscode.TextEditorDecorationType | null = null;
let outputChannel: vscode.OutputChannel | null = null;

// ---------------------------------------------------------------------------
// Activation
// ---------------------------------------------------------------------------

export async function activate(ctx: vscode.ExtensionContext): Promise<void> {
  // Commands declared in package.json menus MUST be registered unconditionally.
  // If they are registered inside the shouldActivate guard, clicking a toolbar
  // button before the workspace passes that check produces "command not found".
  registerCommands(ctx);

  const folders = vscode.workspace.workspaceFolders ?? [];

  // Output channel for errors and logs — create early so every downstream
  // path (binary resolution, walkthrough, download) can route errors here.
  outputChannel = vscode.window.createOutputChannel('Specter');
  ctx.subscriptions.push(outputChannel);

  // AC-01: discover YAML to decide how much of the per-folder wiring runs.
  const allFiles = await vscode.workspace.findFiles('**/*.{yaml,yml}', undefined, 500);
  const filePaths = allFiles.map(u => u.fsPath);
  const hasSpecOrManifest = shouldActivate(filePaths);

  // AC-17 / AC-46 (v1.3.0): walkthrough for empty workspaces. Fires when
  // the workspace has neither specs nor manifest — which means it MUST be
  // checked BEFORE the `shouldActivate` early-return (the two conditions
  // are mutually exclusive, so gating the walkthrough behind shouldActivate
  // made it unreachable).
  const specFiles = filePaths.filter(f => path.basename(f).endsWith('.spec.yaml'));
  const manifestFiles = filePaths.filter(f => path.basename(f) === 'specter.yaml');
  if (shouldShowWalkthrough({ specFiles, hasSpecterManifest: manifestFiles.length > 0 })) {
    void vscode.commands.executeCommand(
      'workbench.action.openWalkthrough',
      'specter-team.specter-vscode#specter.gettingStarted',
    );
  }

  // AC-02 / AC-45 (v1.3.0): binary resolution runs UNCONDITIONALLY (subject
  // to specter.autoDownload). The user's freshly-installed extension must
  // have the CLI available even before any specs exist — otherwise
  // specter.runReverse (the walkthrough's first step) has nothing to call.
  const resolved = await resolveBinary(ctx);
  if (!resolved) return; // modal error already shown from inside resolveBinary

  binaryPath = resolved;

  // Add ~/.specter/bin to the integrated terminal PATH so users can type
  // `specter` directly without needing to configure their shell profile.
  const specterBinDir = path.dirname(defaultCachePath());
  ctx.environmentVariableCollection.prepend('PATH', specterBinDir + path.delimiter);

  // One-time prompt for existing users (and anyone whose rc file doesn't
  // include ~/.specter/bin): offer to run the shell-path command so the
  // CLI works from external terminals. Non-blocking — fire and forget.
  void maybePromptAddCliToShellPath(ctx, specterBinDir);

  // If the workspace has no specs or manifest, we're done. Commands are
  // registered, the binary is available, the walkthrough fired if needed.
  // No per-folder client wiring, no tree provider — there's nothing to
  // track coverage for yet.
  if (!hasSpecOrManifest) return;

  // Status bar (AC-12) — only when we have something to report on.
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  statusBarItem.command = 'specter.openInsights';
  statusBarItem.text = 'Specter: loading…';
  statusBarItem.show();
  ctx.subscriptions.push(statusBarItem);

  // Per-workspace-folder setup (AC-22)
  for (const folder of folders) {
    await setupFolder(ctx, folder);
  }

  vscode.workspace.onDidChangeWorkspaceFolders(async e => {
    for (const folder of e.added) await setupFolder(ctx, folder);
    for (const folder of e.removed) teardownFolder(folder);
  }, undefined, ctx.subscriptions);

  // Watch for specter.yaml creation so `specter init` works without reload.
  const manifestWatcher = vscode.workspace.createFileSystemWatcher('**/specter.yaml');
  manifestWatcher.onDidCreate(async () => {
    for (const folder of (vscode.workspace.workspaceFolders ?? [])) {
      await setupFolder(ctx, folder);
    }
  });
  ctx.subscriptions.push(manifestWatcher);

  // AC-11: Tree view sidebar
  // AC-38 (v0.9.0): createTreeView instead of registerTreeDataProvider so
  // specter.revealInTree has a handle to call .reveal() on — the command
  // was declared in package.json from the start but never actually wired,
  // surfacing as "command 'specter.revealInTree' not found" when invoked.
  treeProvider = new SpecterTreeProvider();
  specterTreeView = vscode.window.createTreeView('specterCoverage', {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });
  ctx.subscriptions.push(specterTreeView);

  // AC-14: Drift decoration type
  // AC-42 (v1.3.0): push to subscriptions so the decoration type is
  // disposed on extension deactivation. Previously it leaked across
  // Developer: Reload Window cycles.
  driftDecorationType = vscode.window.createTextEditorDecorationType({
    gutterIconPath: new vscode.ThemeIcon('warning').id ? undefined : undefined,
    overviewRulerColor: 'orange',
    overviewRulerLane: vscode.OverviewRulerLane.Left,
    after: {
      contentText: ' ⚠ spec changed',
      color: new vscode.ThemeColor('editorWarning.foreground'),
    },
  });
  ctx.subscriptions.push(driftDecorationType);

  // Providers (registered once, they read per-folder state as needed)
  registerProviders(ctx);
}

export function deactivate(): void {
  for (const client of clients.values()) client.dispose();
  clients.clear();
}

// ---------------------------------------------------------------------------
// Per-folder lifecycle
// ---------------------------------------------------------------------------

async function setupFolder(
  ctx: vscode.ExtensionContext,
  folder: vscode.WorkspaceFolder,
): Promise<void> {
  const key = createClientKey(folder.uri.fsPath);
  if (clients.has(key)) return;

  const manifestPath = resolveManifestPath(
    folder.uri.fsPath,
    p => {
      try { require('fs').accessSync(p); return true; }
      catch { return false; }
    },
    // Pre-v0.8.1, this call only passed the first two args. That caused
    // resolveManifestPath to treat folder.uri.fsPath as a file path and
    // dirname() up one level before the first check — so /home/.../jwtms
    // would search /home/.../projects/specter.yaml first and never check
    // /home/.../jwtms/specter.yaml at all. The third arg fixes that by
    // telling the resolver "this path IS the starting directory."
    p => {
      try { return require('fs').statSync(p).isDirectory(); }
      catch { return false; }
    },
  );
  if (!manifestPath) return;

  const client = new SpecterClient({
    binaryPath: binaryPath!,
    manifestPath,
    workspaceFolder: folder.uri.fsPath,
  });
  clients.set(key, client);

  const dc = vscode.languages.createDiagnosticCollection(`specter-${key}`);
  ctx.subscriptions.push(dc);
  diagnosticCollections.set(key, dc);

  const replacer = new DiagnosticReplacer({
    set: (uri, diags) => dc.set(vscode.Uri.file(uri), diags as vscode.Diagnostic[]),
    delete: uri => dc.delete(vscode.Uri.file(uri)),
  });
  diagnosticReplacers.set(key, replacer);

  // Initial coverage run
  await runCoverageForFolder(key, client);
}

function teardownFolder(folder: vscode.WorkspaceFolder): void {
  const key = createClientKey(folder.uri.fsPath);
  clients.get(key)?.dispose();
  clients.delete(key);
  diagnosticCollections.get(key)?.dispose();
  diagnosticCollections.delete(key);
  diagnosticReplacers.delete(key);
}

// ---------------------------------------------------------------------------
// Binary resolution (AC-02)
// ---------------------------------------------------------------------------

async function resolveBinary(ctx: vscode.ExtensionContext): Promise<string | null> {
  const cfg = vscode.workspace.getConfiguration('specter');
  const workspaceSetting = cfg.get<string>('binaryPath') || null;

  const { resolved, source } = resolveBinaryPath({
    workspaceSetting,
    which: name => {
      try {
        const out = require('child_process').execSync(`which ${name}`, { encoding: 'utf8' });
        return (out as string).trim() || null;
      } catch { return null; }
    },
    fs: {
      exists: p => { try { require('fs').accessSync(p); return true; } catch { return false; } },
      isExecutable: p => {
        try { require('fs').accessSync(p, require('fs').constants.X_OK); return true; }
        catch { return false; }
      },
    },
    cachePath: defaultCachePath(),
  });

  const cachePath = defaultCachePath();

  if (resolved) {
    // Always validate the resolved binary — regardless of source. A corrupt
    // file in ~/.specter/bin that also happens to be on the shell PATH would
    // otherwise slip through as source='path' and every specter invocation
    // would fail silently. See issue: https://github.com/Hanalyx/specter/issues
    if (!isBinaryFile(resolved) || !getCachedBinaryVersion(resolved)) {
      // If the corrupt file is the cache path we own, delete it and fall
      // through to auto-download. Otherwise it's user-provided (workspace
      // setting or something else on PATH) — don't touch it, just prompt.
      if (resolved === cachePath) {
        try { require('fs').unlinkSync(resolved); } catch { /* ignore */ }
        // fall through to auto-download
      } else {
        const pick = await vscode.window.showErrorMessage(
          `Specter binary at ${resolved} (via ${source}) is not a valid executable. ` +
          `It may be a corrupt download or a stale file. Re-download to ${cachePath}?`,
          'Re-download', 'Cancel',
        );
        if (pick === 'Re-download') {
          return downloadBinary(ctx);
        }
        return null;
      }
    } else {
      // Valid binary. Auto-update if CLI version != extension version.
      const cliVersion = getCachedBinaryVersion(resolved);
      const extVersion = vscode.extensions.getExtension('Hanalyx.specter-vscode')?.packageJSON?.version as string | undefined;
      if (cliVersion && extVersion && cliVersion !== extVersion) {
        const autoDownload = cfg.get<boolean>('autoDownload', true);
        if (autoDownload) {
          const updated = await downloadBinary(ctx);
          if (updated) return updated;
        }
      }
      return resolved;
    }
  }

  // Auto-download
  const autoDownload = cfg.get<boolean>('autoDownload', true);
  if (!autoDownload) {
    vscode.window.showErrorMessage(
      'Specter binary not found. Set specter.binaryPath or enable specter.autoDownload.',
      { modal: true },
    );
    return null;
  }

  return downloadBinary(ctx);
}

async function downloadBinary(ctx: vscode.ExtensionContext): Promise<string | null> {
  const cfg = vscode.workspace.getConfiguration('specter');
  const versionSetting = cfg.get<string>('version', '');

  return vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: 'Downloading Specter CLI…', cancellable: false },
    async (progress) => {
      try {
        // 1. Resolve version — default pins the CLI to the extension's own
        // version, so v0.10.0 VSIX always fetches v0.10.0 CLI. 'latest' opts
        // in to whatever GitHub's /releases/latest points at, and any other
        // string is treated as a pinned semver.
        progress.report({ message: 'resolving version…' });
        let version: string;
        if (versionSetting === 'latest') {
          version = await resolveLatestVersion();
        } else if (versionSetting) {
          version = versionSetting;
        } else {
          version = ctx.extension.packageJSON.version as string;
        }

        // 2. Build download URL
        const dlOpts = { version, os: process.platform, arch: process.arch };
        const url = buildDownloadUrl(dlOpts);
        const targetPath = defaultCachePath();
        const archiveName = assetName(dlOpts);
        const format: 'tar.gz' | 'zip' = process.platform === 'win32' ? 'zip' : 'tar.gz';

        // 3. Download archive
        progress.report({ message: `downloading v${version}…` });
        const archiveData = await httpsGet(url);

        // 4. Verify SHA256 — MANDATORY.
        // AC-47 (v1.3.0): no silent fallback. Any failure in the
        // verification chain (checksums.txt unreachable, archive absent
        // from checksums.txt, hash mismatch) is a hard fail. Prior
        // behavior let a MITM attacker selectively block checksums.txt
        // while delivering a tampered archive; that class of attack is
        // no longer possible.
        progress.report({ message: 'verifying checksum…' });
        let checksums: Map<string, string>;
        try {
          checksums = await downloadChecksums(version);
        } catch (e) {
          const msg = e instanceof Error ? e.message : String(e);
          vscode.window.showErrorMessage(
            `Specter download failed: unable to retrieve checksums.txt for v${version} (${msg}). Refusing to install unverified binary.`,
            { modal: true },
          );
          return null;
        }
        const expectedHash = checksums.get(archiveName);
        if (!expectedHash) {
          vscode.window.showErrorMessage(
            `Specter download failed: no checksum entry for ${archiveName} in checksums.txt for v${version}. Refusing to install unverified binary.`,
            { modal: true },
          );
          return null;
        }
        const valid = await verifyChecksum(archiveData, expectedHash);
        if (!valid) {
          vscode.window.showErrorMessage(
            'Specter download failed: SHA256 checksum mismatch. The binary may have been tampered with.',
            { modal: true },
          );
          return null;
        }

        // 5. Extract binary from archive
        progress.report({ message: 'extracting binary…' });
        await extractBinary(archiveData, format, targetPath);

        // 6. Validate the extracted binary is actually executable
        const installedVersion = getCachedBinaryVersion(targetPath);
        if (!installedVersion) {
          // Extraction produced a corrupt file — clean up and report
          try { require('fs').unlinkSync(targetPath); } catch { /* ignore */ }
          vscode.window.showErrorMessage(
            'Specter download failed: extracted binary is not executable. Try again or install manually.',
            { modal: true },
          );
          return null;
        }

        vscode.window.showInformationMessage(`Specter CLI v${installedVersion} installed successfully.`);
        return targetPath;
      } catch (e) {
        vscode.window.showErrorMessage(`Failed to download Specter: ${e}`, { modal: true });
        return null;
      }
    },
  );
}

// ---------------------------------------------------------------------------
// External-terminal PATH prompt
// ---------------------------------------------------------------------------

const ADD_PATH_PROMPT_DISMISSED_KEY = 'specter.addPathPromptDismissed';

/**
 * Backwards-compat DX: users who installed the extension before v0.6.8 may
 * have the CLI cached at `~/.specter/bin/specter` but not on their shell
 * PATH. Every time they drop to a non-VS-Code terminal they have to type
 * the full path. Prompt them (non-blocking) to run the shell-path command.
 *
 * Skips silently for: unknown shells, missing rc files (we don't create one
 * on someone's behalf), rc files that already reference the bin dir, and
 * users who've opted out via "Don't show again".
 */
async function maybePromptAddCliToShellPath(
  ctx: vscode.ExtensionContext,
  binDir: string,
): Promise<void> {
  const fs = require('fs');

  const dismissed = ctx.globalState.get<boolean>(ADD_PATH_PROMPT_DISMISSED_KEY, false);
  const shell = process.env.SHELL || '';
  const cfg = detectShellConfig({ shell, platform: process.platform, home: os.homedir() }, binDir);
  if (!cfg) return;

  let rcContents: string | null = null;
  try { rcContents = fs.readFileSync(cfg.rcFile, 'utf8'); } catch { /* missing file → null */ }

  if (!shouldPromptAddPath(rcContents, binDir, dismissed)) return;

  const pick = await vscode.window.showInformationMessage(
    `Specter CLI is installed at ${binDir} but not on your shell PATH. ` +
    `Run \`specter\` from external terminals by adding it to ${cfg.rcFile}.`,
    'Add to PATH',
    "Don't show again",
  );

  if (pick === 'Add to PATH') {
    await vscode.commands.executeCommand('specter.addCliToShellPath');
  } else if (pick === "Don't show again") {
    await ctx.globalState.update(ADD_PATH_PROMPT_DISMISSED_KEY, true);
  }
}

// ---------------------------------------------------------------------------
// Coverage run helpers
// ---------------------------------------------------------------------------

async function runCoverageForFolder(key: string, client: SpecterClient): Promise<void> {
  try {
    const result = await client.coverage();
    coverageReport = result as unknown as CoverageReport;
    updateSpecIndex(coverageReport);

    // v0.9.0: parse errors flow through the report now, not a rejected
    // promise. Distinguish "CLI ran but parses failed" from the happy path
    // so the sidebar + status bar reflect the real state.
    const parseErrors = coverageReport.parseErrors ?? [];
    pushCoverageParseDiagnostics(key, parseErrors);
    if (parseErrors.length > 0) {
      outputChannel?.appendLine(
        `[${new Date().toISOString()}] coverage for ${key}: ${parseErrors.length} parse error(s):`,
      );
      for (const pe of parseErrors) {
        const loc = pe.line ? `:${pe.line}` : '';
        outputChannel?.appendLine(`  ${pe.file}${loc} — ${pe.message}`);
      }
      setStatusBarError('Specter: parse errors. Click to view details.');
      coverageErrorFolders.add(key);
    } else {
      updateStatusBar(coverageReport);
      coverageErrorFolders.delete(key);
    }
    treeProvider?.refresh();
  } catch (e) {
    // runAllowingNonZero only rejects for real spawn failures (ENOENT,
    // aborted, malformed JSON). These are process-level problems, not
    // per-spec parse failures — surface as an error state with no report.
    const msg = e instanceof Error ? e.message : String(e);
    outputChannel?.appendLine(`[${new Date().toISOString()}] coverage run failed for ${key}:`);
    outputChannel?.appendLine('  ' + msg);
    setStatusBarError('Specter coverage failed. Click to view details.');
    coverageErrorFolders.add(key);
  }
}

/**
 * AC-34 (v0.9.0): push CLI-reported parse errors into the per-folder
 * DiagnosticCollection so VS Code's Problems panel shows one clickable
 * entry per broken spec file. Previously the only surfacing was a single
 * sidebar message and lines in the Output channel — users couldn't jump
 * to the offending file without copy-pasting the path.
 */
function pushCoverageParseDiagnostics(
  key: string,
  parseErrors: ReadonlyArray<{ file: string; path?: string; type?: string; message: string; line?: number; column?: number }>,
): void {
  const dc = diagnosticCollections.get(key);
  if (!dc) return;
  // Clear prior coverage-sourced diagnostics for this folder. We clear
  // everything tagged with our source; per-file replacement happens below.
  dc.clear();
  const grouped = buildCoverageParseDiagnostics(parseErrors as { file: string; path?: string; type?: string; message: string; line?: number; column?: number }[]);
  for (const { file, diagnostics } of grouped) {
    const abs = path.isAbsolute(file)
      ? file
      : path.resolve(vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '', file);
    const uri = vscode.Uri.file(abs);
    const vsDiags = diagnostics.map(d => {
      const range = new vscode.Range(
        d.range.start.line,
        d.range.start.character,
        d.range.end.line,
        Math.min(d.range.end.character, 1_000_000),
      );
      const diag = new vscode.Diagnostic(
        range,
        d.message,
        d.severity === 'error' ? vscode.DiagnosticSeverity.Error : vscode.DiagnosticSeverity.Warning,
      );
      diag.source = d.source;
      return diag;
    });
    dc.set(uri, vsDiags);
  }
}

/**
 * AC-49: curry a rejection handler that logs drift-scan failures to the
 * Specter Output channel. Replaces `.catch(() => {})` at the two drift
 * hook sites so the user has somewhere to see the failure.
 */
function logDriftFailure(filePath: string): (err: unknown) => void {
  return (err: unknown) => {
    const msg = err instanceof Error ? err.message : String(err);
    outputChannel?.appendLine(
      `[${new Date().toISOString()}] drift scan failed for ${filePath}: ${msg}`,
    );
  };
}

function setStatusBarError(tooltip: string): void {
  if (!statusBarItem) return;
  statusBarItem.text = '$(error) Specter: error';
  statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
  statusBarItem.tooltip = tooltip;
  statusBarItem.command = 'specter.showOutput';
}

function updateSpecIndex(report: CoverageReport): void {
  const entries = report.entries ?? [];
  for (const entry of entries) {
    if (!specIndex.specs[entry.specID]) {
      specIndex.specs[entry.specID] = {
        id: entry.specID,
        title: entry.specID,
        tier: entry.tier,
        file: '',
        acs: [],
        coveragePct: entry.coveragePct,
        status: 'approved',
      };
    }
  }
}

function updateStatusBar(report: CoverageReport): void {
  if (!statusBarItem) return;
  const entries = report.entries ?? [];
  const hasT1OrT2Failure = entries.some(
    e => !e.passesThreshold && (e.tier === 1 || e.tier === 2),
  );
  const totalPct = entries.length === 0
    ? 0
    : Math.round(entries.reduce((s, e) => s + e.coveragePct, 0) / entries.length);
  const failing = entries.filter(e => !e.passesThreshold).length;

  const result = formatStatusBar({
    totalSpecs: entries.length,
    coveragePct: totalPct,
    failing,
    hasT1OrT2Failure,
  });

  if (typeof result === 'string') {
    statusBarItem.text = result;
    statusBarItem.backgroundColor = undefined;
  } else {
    statusBarItem.text = result.text;
    statusBarItem.backgroundColor = result.colorToken
      ? new vscode.ThemeColor(result.colorToken)
      : undefined;
  }
}

// ---------------------------------------------------------------------------
// Provider registration
// ---------------------------------------------------------------------------

function registerProviders(ctx: vscode.ExtensionContext): void {
  const specYamlSelector: vscode.DocumentSelector = {
    language: 'yaml',
    pattern: '**/*.spec.yaml',
  };
  const testFileSelector: vscode.DocumentSelector = [
    { pattern: '**/*.test.{ts,js,go,py}' },
    { pattern: '**/*_test.go' },
    { pattern: '**/*_test.py' },
  ];

  // -- Completion: @spec and @ac in test files (AC-07, AC-08)
  ctx.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      testFileSelector,
      {
        provideCompletionItems(doc, pos) {
          const lineText = doc.lineAt(pos.line).text;

          if (/@spec\s*$/.test(lineText.slice(0, pos.character))) {
            return buildSpecCompletions(specIndex, doc.uri.fsPath).map(item =>
              toVscodeCompletion(item),
            );
          }

          if (/@ac\s*$/.test(lineText.slice(0, pos.character))) {
            return buildACCompletions(
              specIndex,
              doc.getText(),
              pos.line,
            ).map(item => toVscodeCompletion(item));
          }

          return [];
        },
      },
      '@',
    ),
  );

  // -- Hover: @ac annotation in test files (AC-09, AC-32)
  ctx.subscriptions.push(
    vscode.languages.registerHoverProvider(testFileSelector, {
      provideHover(doc, pos) {
        const line = doc.lineAt(pos.line).text;
        const m = line.match(/\/\/\s*@ac\s+(AC-\d+)/);
        if (!m) return;

        const specID = findNearestSpecAnnotation(doc.getText(), pos.line);
        if (!specID) return;

        // AC-32: populate coveredByFiles from the live CoverageReport so the
        // hover reflects reality. The previous implementation hard-coded []
        // which made every `@ac` hover display as "uncovered" — a UX
        // regression caught by the quality audit (H3).
        const coveredByFiles = resolveCoveringFiles(
          coverageReport,
          specID,
          m[1],
          doc.uri.fsPath,
          path.resolve,
        );

        const hover = buildAnnotationHover(specIndex, specID, m[1], {
          coveredByFiles,
        });
        if (!hover.contents) return;
        return new vscode.Hover(new vscode.MarkdownString(hover.contents));
      },
    }),
  );

  // -- Hover: constraint ID in spec files (AC-06)
  ctx.subscriptions.push(
    vscode.languages.registerHoverProvider(specYamlSelector, {
      provideHover(doc, pos) {
        const word = doc.getText(doc.getWordRangeAtPosition(pos, /C-\d+/));
        if (!word) return;

        const specID = guessSpecIDFromFile(doc.uri.fsPath);
        if (!specID) return;

        const hover = buildConstraintHover(specIndex, specID, word, { coveredACIDs: [] });
        if (!hover.contents) return;
        return new vscode.Hover(new vscode.MarkdownString(hover.contents));
      },
    }),
  );

  // -- Go-to-definition (AC-10)
  ctx.subscriptions.push(
    vscode.languages.registerDefinitionProvider(specYamlSelector, {
      provideDefinition(doc, pos) {
        const word = doc.getText(doc.getWordRangeAtPosition(pos, /[\w-]+/));
        if (!word) return;

        // Check if it's a constraint ref (C-NN pattern)
        if (/^C-\d+$/.test(word)) {
          const target = resolveDefinitionTarget(specIndex, {
            kind: 'constraint_ref',
            value: word,
            sourceFile: doc.uri.fsPath,
          });
          if (target) {
            return new vscode.Location(vscode.Uri.file(target.file), new vscode.Position(target.line, 0));
          }
        }

        // Check if it's a spec_id reference
        const target = resolveDefinitionTarget(specIndex, {
          kind: 'spec_id',
          value: word,
          sourceFile: doc.uri.fsPath,
        });
        if (target) {
          return new vscode.Location(vscode.Uri.file(target.file), new vscode.Position(target.line, 0));
        }
      },
    }),
  );

  // -- Code actions: quick-fix for unannotated test functions (AC-15)
  ctx.subscriptions.push(
    vscode.languages.registerCodeActionsProvider(testFileSelector, {
      provideCodeActions(doc, range, context) {
        const actions: vscode.CodeAction[] = [];
        for (const diag of context.diagnostics) {
          if ((diag as any).__specterQuickFix) {
            const fix = buildQuickFix({
              specID: (diag as any).__specterBestGuessSpecID ?? '',
              functionLine: range.start.line,
            });
            const action = new vscode.CodeAction(
              'Add @spec and @ac annotation',
              vscode.CodeActionKind.QuickFix,
            );
            action.edit = new vscode.WorkspaceEdit();
            const insertPos = new vscode.Position(fix.insertLine, 0);
            action.edit.insert(doc.uri, insertPos, fix.text);
            action.isPreferred = true;
            actions.push(action);
          }
        }
        return actions;
      },
    }),
  );

  // -- Code lens: AC suggestions (AC-21)
  ctx.subscriptions.push(
    vscode.languages.registerCodeLensProvider(testFileSelector, {
      provideCodeLenses(doc) {
        const lenses: vscode.CodeLens[] = [];
        const text = doc.getText();
        const fnPattern = /^(export\s+)?(async\s+)?function\s+(\w+)/gm;
        let m: RegExpExecArray | null;

        while ((m = fnPattern.exec(text)) !== null) {
          const pos = doc.positionAt(m.index);
          // Skip if already annotated
          const prevLine = pos.line > 0 ? doc.lineAt(pos.line - 1).text : '';
          if (/@spec|@ac/.test(prevLine)) continue;

          const body = text.slice(m.index, m.index + 400);
          const suggestions = suggestACsForFunction(specIndex, body);
          if (suggestions.length === 0) continue;

          for (const sug of suggestions) {
            lenses.push(
              new vscode.CodeLens(new vscode.Range(pos, pos), {
                title: `$(lightbulb) @ac ${sug.acID} — ${sug.description.slice(0, 50)}…`,
                command: 'specter._insertAnnotation',
                arguments: [doc.uri, pos.line, sug.specID, sug.acID],
              }),
            );
          }
        }

        return lenses;
      },
    }),
  );

  // -- File decoration provider (AC-20)
  ctx.subscriptions.push(
    vscode.window.registerFileDecorationProvider({
      provideFileDecoration(uri) {
        if (!uri.fsPath.endsWith('.spec.yaml')) return;
        if (!coverageReport) return;

        const specID = guessSpecIDFromFile(uri.fsPath);
        if (!specID) return;

        const entry = (coverageReport.entries ?? []).find(e => e.specID === specID);
        if (!entry) return;

        const dec = buildFileDecoration(entry);
        return {
          badge: dec.badge,
          color: new vscode.ThemeColor(dec.color),
          tooltip: dec.tooltip,
        };
      },
    }),
  );

  // -- AC-05: Gutter icons on ACs in spec files via CodeLens
  ctx.subscriptions.push(
    vscode.languages.registerCodeLensProvider(specYamlSelector, {
      provideCodeLenses(doc) {
        if (!coverageReport) return [];
        const specID = guessSpecIDFromFile(doc.uri.fsPath);
        if (!specID) return [];

        const entry = (coverageReport.entries ?? []).find(e => e.specID === specID);
        if (!entry) return [];

        const decorations = buildACDecorations({
          coveredACs: entry.coveredACs ?? [],
          uncoveredACs: entry.uncoveredACs ?? [],
          gapACs: [],
        });

        const lenses: vscode.CodeLens[] = [];
        for (let i = 0; i < doc.lineCount; i++) {
          const lineText = doc.lineAt(i).text;
          const acMatch = lineText.match(/id:\s*(AC-\d+)/);
          if (!acMatch) continue;
          const acID = acMatch[1];
          const dec = decorations.find(d => d.acID === acID);
          if (!dec) continue;

          const icon = dec.kind === 'covered' ? '$(check)'
            : dec.kind === 'uncovered' ? '$(circle-slash)'
            : '$(warning)';
          const label = dec.endOfLineText ? `${icon} ${dec.endOfLineText}` : `${icon} ${dec.kind}`;
          lenses.push(new vscode.CodeLens(new vscode.Range(i, 0, i, 0), {
            title: label,
            command: '',
          }));
        }
        return lenses;
      },
    }),
  );

  // -- Diagnostics on type/save (AC-03, AC-04)
  registerDiagnosticHooks(ctx);
}

function registerDiagnosticHooks(ctx: vscode.ExtensionContext): void {
  const debounceMap = new Map<string, NodeJS.Timeout>();

  // On-type debounce: 400ms, parse only (AC-03)
  ctx.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument(e => {
      // AC-40: `specter parse` is spec-schema-only. Running it against the
      // project manifest (specter.yaml) surfaces false "Missing required
      // field 'spec'" / "Unknown field 'settings'" diagnostics — the
      // manifest has a system: block, not a spec: block. Gate on
      // `.spec.yaml` like the hover, completion, and codelens hooks do.
      if (!isSpecDocument(e.document)) return;
      const uri = e.document.uri.toString();
      const existing = debounceMap.get(uri);
      if (existing) clearTimeout(existing);

      debounceMap.set(
        uri,
        setTimeout(async () => {
          debounceMap.delete(uri);
          const client = clientForDocument(e.document);
          const replacer = replacerForDocument(e.document);
          if (!client || !replacer) return;

          try {
            const result = await client.parse(e.document.uri.fsPath);
            const diags = buildDiagnostics({ parseErrors: result.errors, checkDiagnostics: [] });
            replacer.replace(
              e.document.uri.fsPath,
              diags.map(d => toVscodeDiagnostic(d)),
            );
          } catch (err) {
            // AC-48: route to Output channel instead of silent ignore.
            const msg = err instanceof Error ? err.message : String(err);
            outputChannel?.appendLine(
              `[${new Date().toISOString()}] on-type parse failed for ${e.document.uri.fsPath}: ${msg}`,
            );
          }
        }, 400),
      );
    }),
  );

  // On-save: check + coverage (AC-04)
  ctx.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async doc => {
      // AC-40: same gate as on-change — don't parse-as-spec a manifest.
      if (!isSpecDocument(doc)) return;
      const client = clientForDocument(doc);
      const replacer = replacerForDocument(doc);
      if (!client || !replacer) return;

      try {
        const [parseResult, checkResult] = await Promise.all([
          client.parse(doc.uri.fsPath),
          client.check(),
        ]);

        const diags = buildDiagnostics({
          parseErrors: parseResult.errors,
          checkDiagnostics: checkResult.diagnostics as any,
        });
        replacer.replace(doc.uri.fsPath, diags.map(d => toVscodeDiagnostic(d)));

        // Scoped coverage run
        const specIDs = shouldRunCoverageForFile(doc.getText());
        for (const specID of specIDs) {
          const result = await client.coverage(specID);
          if (coverageReport) {
            if (!coverageReport.entries) coverageReport.entries = [];
            const idx = coverageReport.entries.findIndex(e => e.specID === specID);
            const newEntry = (result as any).entries?.[0];
            if (newEntry) {
              if (idx >= 0) coverageReport.entries[idx] = newEntry;
              else coverageReport.entries.push(newEntry);
            }
            updateStatusBar(coverageReport);
            treeProvider?.refresh();

            // Notification discipline (AC-18)
            if (newEntry && !newEntry.passesThreshold) {
              const kind = classifyNotification({
                tier: newEntry.tier,
                droppedBelowThreshold: true,
              });
              maybeNotify(specID, kind.kind, newEntry.tier);
            }
          }
        }

        // AC-19: Breaking spec change notification via diff
        if (doc.uri.fsPath.endsWith('.spec.yaml')) {
          try {
            const diffResult = await client.diff(doc.uri.fsPath, 'HEAD') as any;
            if (diffResult?.changeClass) {
              const notification = classifyNotification({ changeClass: diffResult.changeClass });
              if (notification.kind === 'warning-toast') {
                const choice = await vscode.window.showWarningMessage(
                  `Breaking change detected in ${path.basename(doc.uri.fsPath)}`,
                  ...(notification.actions ?? []),
                );
                if (choice === 'View Diff') {
                  const terminal = vscode.window.createTerminal('Specter Diff');
                  terminal.sendText(`specter diff ${doc.uri.fsPath}@HEAD ${doc.uri.fsPath}`);
                  terminal.show();
                }
              }
            }
          } catch { /* diff not available — new file or not a git repo */ }
        }
      } catch { /* ignore */ }
    }),
  );

  // AC-14: Scan for drift when test files are opened or saved.
  // AC-49 (v1.3.0): route failures to the Output channel, not `.catch(() => {})`.
  ctx.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(editor => {
      if (editor) scanForDrift(editor.document).catch(logDriftFailure(editor.document.uri.fsPath));
    }),
  );
  // Also trigger drift scan after save (test file may reference a changed spec)
  ctx.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(doc => {
      if (!doc.uri.fsPath.endsWith('.spec.yaml')) {
        scanForDrift(doc).catch(logDriftFailure(doc.uri.fsPath));
      }
    }),
  );

  // Clean up diagnostics on file rename / delete (AC-04)
  ctx.subscriptions.push(
    vscode.workspace.onDidRenameFiles(e => {
      for (const { oldUri } of e.files) {
        const replacer = replacerForUri(oldUri);
        replacer?.replace(oldUri.fsPath, []);
      }
    }),
    vscode.workspace.onDidDeleteFiles(e => {
      for (const uri of e.files) {
        const replacer = replacerForUri(uri);
        replacer?.replace(uri.fsPath, []);
      }
    }),
  );
}

// ---------------------------------------------------------------------------
// Command registration
// ---------------------------------------------------------------------------

function registerCommands(ctx: vscode.ExtensionContext): void {
  // AC-13: Insights panel
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.openInsights', () => {
      if (!coverageReport) {
        vscode.window.showInformationMessage('Specter coverage not yet loaded.');
        return;
      }
      // v0.9.0: Insights now renders parse failures AND coverage cards in
      // one view. Previously it short-circuited on parse errors or — worse
      // — silently claimed "all specs passing" when entries was empty.
      const entries = coverageReport.entries ?? [];
      const parseErrors = coverageReport.parseErrors ?? [];
      const panel = vscode.window.createWebviewPanel(
        'specterInsights',
        'Specter Insights',
        vscode.ViewColumn.Beside,
        { enableScripts: true },
      );
      const cards = buildInsightCards(entries, specIndex);
      panel.webview.html = renderInsightsHTML({
        cards,
        parseErrors,
        specCandidatesCount: coverageReport.specCandidatesCount ?? 0,
        entryCount: entries.length,
      });
      // AC-39: parse-failure cards emit {openFile: path} messages when the
      // user clicks the header. Route them to vscode.open with the
      // resolved absolute URI so the file opens at the reported line.
      panel.webview.onDidReceiveMessage((msg: { openFile?: string; line?: number }) => {
        if (!msg?.openFile) return;
        const abs = resolveWorkspacePath(msg.openFile);
        const uri = vscode.Uri.file(abs);
        const selection = msg.line && msg.line > 0
          ? new vscode.Range(msg.line - 1, 0, msg.line - 1, 0)
          : undefined;
        void vscode.commands.executeCommand('vscode.open', uri, selection ? { selection } : undefined);
      }, undefined, ctx.subscriptions);
    }),
  );

  // AC-16: Copy spec context for AI
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.copySpecContext', async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) return;

      const specID = guessSpecIDFromFile(editor.document.uri.fsPath);
      const spec = specID ? specIndex.specs[specID] : undefined;
      if (!spec) {
        vscode.window.showWarningMessage('No spec found for this file.');
        return;
      }

      const text = formatSpecContextForAI(spec);
      await vscode.env.clipboard.writeText(text);
      vscode.window.showInformationMessage('Spec context copied to clipboard.');
    }),
  );

  // specter.runSync
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.runSync', async () => {
      // Retry setup if clients are empty (specter.yaml may have been created
      // after activation, e.g. via `specter init` in the terminal).
      if (clients.size === 0 && binaryPath) {
        for (const folder of (vscode.workspace.workspaceFolders ?? [])) {
          await setupFolder(ctx, folder);
        }
      }
      if (clients.size === 0) {
        vscode.window.showWarningMessage(
          'Specter: no specter.yaml found in this workspace. Run `specter init` to get started.',
        );
        return;
      }
      coverageErrorFolders.clear();
      for (const [key, client] of clients) {
        await runCoverageForFolder(key, client);
      }
      // AC-31: completion message reflects reality. A silent success toast
      // after a failed coverage run (the v0.8.x behavior) misled the user
      // into thinking everything was fine — fixed per quality-audit H1.
      const completion = formatSyncCompletion(coverageErrorFolders.size);
      if (completion.kind === 'warning') {
        vscode.window.showWarningMessage(completion.message, 'Show Output').then(pick => {
          if (pick === 'Show Output') {
            void vscode.commands.executeCommand('specter.showOutput');
          }
        });
      } else {
        vscode.window.showInformationMessage(completion.message);
      }
    }),
  );

  // Insert annotation (used by code lens)
  ctx.subscriptions.push(
    vscode.commands.registerCommand(
      'specter._insertAnnotation',
      async (uri: vscode.Uri, line: number, specID: string, acID: string) => {
        const edit = new vscode.WorkspaceEdit();
        const pos = new vscode.Position(line, 0);
        edit.insert(uri, pos, `// @spec ${specID}\n// @ac ${acID}\n`);
        await vscode.workspace.applyEdit(edit);
      },
    ),
  );

  // Show the Specter output channel. Bound to the status bar when coverage
  // fails so a user stuck on "Specter: error" can click through to details.
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.showOutput', () => {
      outputChannel?.show(true);
    }),
  );

  // AC-43: Bootstrap specs from source code via `specter reverse`. The
  // command is the first step of the onboarding walkthrough; prior to
  // v0.9.1 it was declared in package.json but unregistered, surfacing
  // as "command not found" when a new user clicked the walkthrough link.
  // Implementation opens the integrated terminal and prefills the
  // command so the user can see what's happening, pick a target
  // directory, and review output. Chosen over silent CLI invocation
  // because `reverse` needs a source path and human review of the
  // generated gaps.
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.runReverse', async () => {
      if (!binaryPath) {
        vscode.window.showErrorMessage(
          'Specter CLI is not available. Run `Specter: Re-download CLI` first.',
        );
        return;
      }
      const folders = vscode.workspace.workspaceFolders;
      if (!folders || folders.length === 0) {
        vscode.window.showErrorMessage(
          'Open a folder or workspace first, then run Specter: Run Reverse Compiler.',
        );
        return;
      }
      const folder = folders[0];
      const terminal = vscode.window.createTerminal({
        name: 'Specter Reverse',
        cwd: folder.uri.fsPath,
      });
      terminal.show();
      // Don't execute — let the user pick the source directory.
      terminal.sendText('specter reverse ', false);
    }),
  );

  // AC-38: Reveal the active file in the Coverage sidebar. Matches in this
  // priority: (1) a spec entry whose specFile resolves to the active file,
  // (2) a parse-error leaf, (3) the spec whose @spec annotations live in
  // the active test file. Opens the Specter view container if collapsed.
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.revealInTree', async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        vscode.window.showInformationMessage('Open a spec or test file first, then invoke Specter: Reveal in Tree View.');
        return;
      }
      if (!treeProvider || !specterTreeView) {
        vscode.window.showInformationMessage('Specter sidebar is not available in this workspace.');
        return;
      }
      const activePath = editor.document.uri.fsPath;
      const target = treeProvider.findElementForFile(activePath);
      if (!target) {
        vscode.window.showInformationMessage(
          'This file is not tracked by Specter coverage yet. Save it (for test files with @spec annotations) or re-run Specter: Run Sync.',
        );
        return;
      }
      try {
        await specterTreeView.reveal(target, { select: true, focus: true, expand: true });
      } catch (e) {
        outputChannel?.appendLine(`revealInTree failed: ${e instanceof Error ? e.message : String(e)}`);
      }
    }),
  );

  // Append ~/.specter/bin to the user's shell rc file so `specter` works in
  // external terminals (the integrationVariableCollection path only affects
  // VS Code's integrated terminals). Idempotent — does nothing if the path
  // is already referenced.
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.addCliToShellPath', async () => {
      const fs = require('fs');
      const binDir = path.dirname(defaultCachePath());
      const shell = process.env.SHELL || '';
      const cfg = detectShellConfig({ shell, platform: process.platform, home: os.homedir() }, binDir);

      if (!cfg) {
        const manual = `export PATH="${binDir}:$PATH"`;
        await vscode.env.clipboard.writeText(manual);
        vscode.window.showWarningMessage(
          `Specter: couldn't detect your shell (SHELL=${shell || 'unset'}). The export line has been copied to your clipboard — paste it into your shell's rc file.`,
        );
        return;
      }

      let existing = '';
      try { existing = fs.readFileSync(cfg.rcFile, 'utf8'); } catch { /* file may not exist yet */ }

      if (isPathAlreadyPresent(existing, binDir)) {
        vscode.window.showInformationMessage(
          `Specter: ${binDir} is already referenced in ${cfg.rcFile}. Nothing to do.`,
        );
        return;
      }

      try {
        fs.mkdirSync(path.dirname(cfg.rcFile), { recursive: true });
        fs.appendFileSync(cfg.rcFile, formatAppendBlock(cfg.exportLine), 'utf8');
      } catch (e) {
        vscode.window.showErrorMessage(`Specter: could not write to ${cfg.rcFile}: ${e}`);
        return;
      }

      const sourceCmd = `source "${cfg.rcFile}"`;
      const pick = await vscode.window.showInformationMessage(
        `Specter: added ${binDir} to PATH via ${cfg.rcFile}. Restart your terminal or run "${sourceCmd}" to pick it up.`,
        'Copy source command',
      );
      if (pick === 'Copy source command') {
        await vscode.env.clipboard.writeText(sourceCmd);
      }
    }),
  );

  // Force a fresh download of the Specter CLI — the explicit recovery path
  // when the cached binary is broken. Deletes the cached file first so
  // downloadBinary always writes a new copy, then re-runs activation wiring.
  ctx.subscriptions.push(
    vscode.commands.registerCommand('specter.redownloadCli', async () => {
      const cachePath = defaultCachePath();
      try { require('fs').unlinkSync(cachePath); } catch { /* ignore */ }
      const resolved = await downloadBinary(ctx);
      if (!resolved) return;
      binaryPath = resolved;
      for (const folder of (vscode.workspace.workspaceFolders ?? [])) {
        await setupFolder(ctx, folder);
      }
      vscode.window.showInformationMessage('Specter CLI re-downloaded.');
    }),
  );
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function clientForDocument(doc: vscode.TextDocument): SpecterClient | undefined {
  const folder = vscode.workspace.getWorkspaceFolder(doc.uri);
  if (!folder) return undefined;
  return clients.get(createClientKey(folder.uri.fsPath));
}

/**
 * AC-40: predicate used by the on-change / on-save hooks to gate
 * `specter parse` invocations. The CLI's parse command validates against
 * the spec schema only — running it on a manifest (specter.yaml) surfaces
 * spurious "Missing required field 'spec'" / "Unknown field 'settings'"
 * diagnostics because the manifest uses a different schema.
 *
 * Pure path predicate lives in activation.ts so it's testable without
 * importing vscode.
 */
function isSpecDocument(doc: vscode.TextDocument): boolean {
  return isSpecFilePath(doc.uri.fsPath);
}

function replacerForDocument(doc: vscode.TextDocument): DiagnosticReplacer | undefined {
  const folder = vscode.workspace.getWorkspaceFolder(doc.uri);
  if (!folder) return undefined;
  return diagnosticReplacers.get(createClientKey(folder.uri.fsPath));
}

function replacerForUri(uri: vscode.Uri): DiagnosticReplacer | undefined {
  const folder = vscode.workspace.getWorkspaceFolder(uri);
  if (!folder) return undefined;
  return diagnosticReplacers.get(createClientKey(folder.uri.fsPath));
}

function guessSpecIDFromFile(filePath: string): string | undefined {
  const base = path.basename(filePath, '.spec.yaml');
  return specIndex.specs[base] ? base : Object.keys(specIndex.specs)[0];
}

function maybeNotify(specID: string, kind: string, tier: number): void {
  if (kind === 'none' || kind === 'status-bar-only') return;
  if (!rateLimiter.shouldNotify(specID)) return;

  const msg = `Specter: ${specID} dropped below threshold (T${tier}).`;
  if (kind === 'warning-toast') {
    vscode.window.showWarningMessage(msg, 'View Diff', 'Dismiss');
  } else {
    vscode.window.showInformationMessage(msg);
  }
}

function toVscodeCompletion(item: ReturnType<typeof buildSpecCompletions>[0]): vscode.CompletionItem {
  const ci = new vscode.CompletionItem(item.label, vscode.CompletionItemKind.Reference);
  ci.insertText = item.insertText;
  ci.detail = item.detail;
  ci.documentation = item.documentation ? new vscode.MarkdownString(item.documentation) : undefined;
  ci.sortText = item.sortText;
  return ci;
}

function toVscodeDiagnostic(d: ReturnType<typeof buildDiagnostics>[0]): vscode.Diagnostic {
  const range = new vscode.Range(
    d.range.start.line, d.range.start.character,
    d.range.end.line, d.range.end.character,
  );
  const severity = d.severity === 'error'
    ? vscode.DiagnosticSeverity.Error
    : d.severity === 'warning'
    ? vscode.DiagnosticSeverity.Warning
    : vscode.DiagnosticSeverity.Information;
  const diag = new vscode.Diagnostic(range, d.message, severity);
  diag.source = d.source;
  return diag;
}

interface RenderInsightsInput {
  cards: ReturnType<typeof buildInsightCards>;
  parseErrors: Array<{ file: string; line?: number; message: string; type?: string; path?: string }>;
  specCandidatesCount: number;
  entryCount: number;
}

function renderInsightsHTML(input: RenderInsightsInput): string {
  const { cards, parseErrors, specCandidatesCount, entryCount } = input;

  // AC-37: single source of truth for "what does the panel claim?".
  // See insights.ts computeInsightsStatus — the webview is a dumb
  // renderer around that decision.
  const status = computeInsightsStatus({
    parseErrorCount: parseErrors.length,
    uncoveredCardCount: cards.length,
    entryCount,
    specCandidatesCount,
  });
  const header = `<h1>${escapeHtml(status.header)}</h1>`;

  // Parse-failures section (when applicable).
  let parseErrorHTML = '';
  if (status.showParseErrorsSection) {
    // Group by file so multiple errors on one file fold into one card.
    const byFile = new Map<string, Array<{ message: string; type?: string; path?: string; line?: number }>>();
    for (const e of parseErrors) {
      const arr = byFile.get(e.file) ?? [];
      arr.push({ message: e.message, type: e.type, path: e.path, line: e.line });
      byFile.set(e.file, arr);
    }
    const fileCards = Array.from(byFile, ([file, errs]) => {
      const items = errs.map(e => {
        const prefix = e.type ? `[${escapeHtml(e.type)}] ` : '';
        const pathSuffix = e.path ? ` <em>(at ${escapeHtml(e.path)})</em>` : '';
        const lineSuffix = e.line ? ` <span class="dim">(line ${e.line})</span>` : '';
        return `<li>${prefix}${escapeHtml(e.message)}${pathSuffix}${lineSuffix}</li>`;
      }).join('');
      // AC-39: header is a clickable link that posts {openFile} to the
      // extension host, which opens the file at the reported line.
      const firstLine = errs.find(e => e.line)?.line ?? 0;
      const payload = JSON.stringify({ openFile: file, line: firstLine });
      return `
        <div class="card parse-error-card">
          <h2><a class="open-file" href="#" data-payload='${escapeAttr(payload)}'>${escapeHtml(file)}</a></h2>
          <ul>${items}</ul>
        </div>
      `;
    }).join('');
    parseErrorHTML = `
      <section>
        <h2 class="section-heading">Parse failures</h2>
        <p class="muted">These spec files could not be parsed and are excluded from coverage analysis. Fix each one and re-run sync.</p>
        ${fileCards}
      </section>
    `;
  }

  // Normal uncovered-AC cards (may be empty).
  const cardHTML = cards.map(card => {
    const acList = card.uncoveredACDetails
      .map(ac => `<li><strong>${ac.id}</strong> — ${escapeHtml(ac.description)}</li>`)
      .join('');
    const callouts = card.constraintCallouts
      .map(c => `<li><strong>${c.constraintID}</strong>: ${escapeHtml(c.description)}</li>`)
      .join('');

    return `
      <div class="card">
        <h2>${escapeHtml(card.specID)}</h2>
        <p>${escapeHtml(card.summary)}</p>
        <h3>Uncovered ACs</h3>
        <ul>${acList}</ul>
        ${callouts ? `<h3>Relevant Constraints</h3><ul>${callouts}</ul>` : ''}
      </div>
    `;
  }).join('');

  const coverageSection = status.showCoverageSection
    ? `<section><h2 class="section-heading">Coverage gaps</h2>${cardHTML}</section>`
    : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  body { font-family: var(--vscode-font-family); padding: 1rem; }
  h1 { color: var(--vscode-foreground); }
  .section-heading { color: var(--vscode-descriptionForeground); margin-top: 1.5rem; }
  .muted { color: var(--vscode-descriptionForeground); font-size: 0.9em; }
  .dim { color: var(--vscode-descriptionForeground); }
  .card { border: 1px solid var(--vscode-panel-border); border-radius: 4px; padding: 1rem; margin-bottom: 1rem; }
  .card h2 { margin-top: 0; color: var(--vscode-errorForeground); }
  .card h3 { color: var(--vscode-descriptionForeground); }
  .parse-error-card { border-left: 3px solid var(--vscode-errorForeground); }
  .parse-error-card h2 { font-family: var(--vscode-editor-font-family); font-size: 1em; word-break: break-all; }
  .parse-error-card h2 a.open-file { color: var(--vscode-textLink-foreground); text-decoration: none; cursor: pointer; }
  .parse-error-card h2 a.open-file:hover { text-decoration: underline; }
</style>
</head>
<body>
${header}
${parseErrorHTML}
${coverageSection}
<script>
  (function() {
    const vscode = acquireVsCodeApi();
    document.querySelectorAll('a.open-file').forEach(a => {
      a.addEventListener('click', (e) => {
        e.preventDefault();
        try {
          const payload = JSON.parse(a.getAttribute('data-payload') || '{}');
          vscode.postMessage(payload);
        } catch (_) { /* ignore malformed payloads */ }
      });
    });
  })();
</script>
</body>
</html>`;
}

function escapeAttr(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/'/g, '&#39;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

// ---------------------------------------------------------------------------
// AC-11: Tree view provider
// ---------------------------------------------------------------------------

type TreeElement = { kind: 'spec'; specID: string; file: string; children: TreeElement[] }
  | { kind: 'ac'; id: string; icon: 'covered' | 'uncovered' | 'gap'; children: TreeElement[] }
  | { kind: 'testFile'; path: string }
  | { kind: 'message'; label: string; detail?: string; iconId?: string }
  | { kind: 'parseErrorGroup'; label: string; children: TreeElement[] }
  | { kind: 'parseErrorFile'; file: string; message: string; line?: number };

/**
 * AC-33 (v0.9.0): resolve a CLI-emitted relative path against the first
 * workspace folder so `vscode.Uri.file` produces an openable absolute URI.
 * Passing a relative path directly to `Uri.file` treats it as absolute from
 * '/' and silently yields a non-existent URI — the "file not found" path
 * users hit when clicking a leaf in the Coverage sidebar.
 */
function resolveWorkspacePath(p: string): string {
  const root = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  return resolveWorkspacePathPure(p, root, (a, b) => path.resolve(a, b), path.isAbsolute);
}

class SpecterTreeProvider implements vscode.TreeDataProvider<TreeElement> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  // Caches rebuilt each time getChildren(undefined) runs. Needed so
  // TreeView.reveal() can walk from a leaf back up to its parent and so
  // findElementForFile() can answer O(1) lookups by absolute path.
  private rootCache: TreeElement[] = [];
  private parentMap = new WeakMap<object, TreeElement>();
  private fileIndex = new Map<string, TreeElement>();

  refresh(): void { this._onDidChange.fire(); }

  getTreeItem(el: TreeElement): vscode.TreeItem {
    switch (el.kind) {
      case 'spec': {
        const item = new vscode.TreeItem(el.specID, vscode.TreeItemCollapsibleState.Collapsed);
        item.contextValue = 'spec';
        item.iconPath = new vscode.ThemeIcon('file-code');
        // AC-33: clicking a spec node opens the .spec.yaml. Only wired when
        // the CLI supplied a file path (spec_file field, v1.5.0+).
        if (el.file) {
          const uri = vscode.Uri.file(resolveWorkspacePath(el.file));
          item.resourceUri = uri;
          item.command = { command: 'vscode.open', title: 'Open Spec', arguments: [uri] };
          item.tooltip = el.file;
        }
        return item;
      }
      case 'ac': {
        const icon = el.icon === 'covered' ? 'pass' : el.icon === 'uncovered' ? 'error' : 'warning';
        const item = new vscode.TreeItem(el.id, vscode.TreeItemCollapsibleState.Collapsed);
        item.iconPath = new vscode.ThemeIcon(icon);
        item.description = el.icon;
        return item;
      }
      case 'testFile': {
        // AC-33: the CLI emits workspace-relative paths; resolve before use.
        const abs = resolveWorkspacePath(el.path);
        const uri = vscode.Uri.file(abs);
        const item = new vscode.TreeItem(path.basename(el.path), vscode.TreeItemCollapsibleState.None);
        item.resourceUri = uri;
        item.command = { command: 'vscode.open', title: 'Open', arguments: [uri] };
        item.tooltip = el.path;
        item.iconPath = new vscode.ThemeIcon('file');
        return item;
      }
      case 'message': {
        // v0.8.0: synthetic empty-state node shown when there is no coverage data.
        const item = new vscode.TreeItem(el.label, vscode.TreeItemCollapsibleState.None);
        item.description = el.detail;
        item.tooltip = el.detail ?? el.label;
        item.iconPath = new vscode.ThemeIcon(el.iconId ?? 'info');
        item.contextValue = 'specterMessage';
        return item;
      }
      case 'parseErrorGroup': {
        // AC-36: the "Failed to parse" collapsible group. Rendered alongside
        // passing spec nodes so the user can see both and act on either.
        const item = new vscode.TreeItem(el.label, vscode.TreeItemCollapsibleState.Expanded);
        item.iconPath = new vscode.ThemeIcon('error');
        item.contextValue = 'specterParseErrorGroup';
        item.tooltip = 'Spec files the CLI could not parse. Each child opens the failing file.';
        return item;
      }
      case 'parseErrorFile': {
        // AC-36: each failing file is a clickable leaf. Click opens the
        // file; the error message is surfaced as both description (inline)
        // and tooltip (for the full detail) so the user doesn't have to
        // visit the Problems panel to know what broke.
        const abs = resolveWorkspacePath(el.file);
        const uri = vscode.Uri.file(abs);
        const item = new vscode.TreeItem(path.basename(el.file), vscode.TreeItemCollapsibleState.None);
        item.resourceUri = uri;
        // Jump to the reported line when the CLI provided one.
        const selection = el.line && el.line > 0
          ? new vscode.Range(el.line - 1, 0, el.line - 1, 0)
          : undefined;
        item.command = {
          command: 'vscode.open',
          title: 'Open failing spec',
          arguments: selection ? [uri, { selection }] : [uri],
        };
        item.description = el.message;
        item.tooltip = `${el.file}${el.line ? `:${el.line}` : ''} — ${el.message}`;
        item.iconPath = new vscode.ThemeIcon('error');
        return item;
      }
    }
  }

  getChildren(el?: TreeElement): TreeElement[] {
    if (!el) {
      // v0.9.0: buildCoverageTreeRoot may now return a mix of spec nodes,
      // a parse-error group, and/or a message node. Map each root shape to
      // the corresponding TreeElement so the provider renders all three.
      //
      // Also rebuild parentMap + fileIndex on every root refresh so
      // TreeView.reveal() can walk leaves back to their parents and
      // findElementForFile() can answer in O(1).
      this.parentMap = new WeakMap<object, TreeElement>();
      this.fileIndex = new Map<string, TreeElement>();

      const roots = buildCoverageTreeRoot(coverageReport);
      const built: TreeElement[] = roots.map(n => {
        if (n.kind === 'message') {
          return { kind: 'message' as const, label: n.label, detail: n.detail, iconId: n.iconId };
        }
        if (n.kind === 'parseErrorGroup') {
          const group: TreeElement = { kind: 'parseErrorGroup', label: n.label, children: [] };
          group.children = n.children.map(c => {
            const leaf: TreeElement = { kind: 'parseErrorFile', file: c.file, message: c.message, line: c.line };
            this.parentMap.set(leaf as object, group);
            this.indexFile(c.file, leaf);
            return leaf;
          });
          return group;
        }
        const specEl: TreeElement = { kind: 'spec', specID: n.specID, file: n.file, children: [] };
        specEl.children = n.children.map(ac => {
          const acEl: TreeElement = {
            kind: 'ac', id: ac.id, icon: ac.icon,
            children: ac.children.map(tf => {
              const tfEl: TreeElement = { kind: 'testFile', path: tf.path };
              this.indexFile(tf.path, tfEl);
              return tfEl;
            }),
          };
          for (const child of acEl.children) this.parentMap.set(child as object, acEl);
          this.parentMap.set(acEl as object, specEl);
          return acEl;
        });
        if (n.file) this.indexFile(n.file, specEl);
        return specEl;
      });
      this.rootCache = built;
      return built;
    }
    if (el.kind === 'testFile' || el.kind === 'message' || el.kind === 'parseErrorFile') return [];
    return el.children;
  }

  getParent(el: TreeElement): TreeElement | null {
    return this.parentMap.get(el as object) ?? null;
  }

  /**
   * AC-38: resolve the active file (absolute path) to the best-matching
   * tree element. Tries absolute-path match first; falls back to suffix
   * match so a CLI-emitted relative path ("specs/foo.spec.yaml") still
   * finds its element when the query is the absolute form.
   */
  findElementForFile(absPath: string): TreeElement | undefined {
    return matchFileInIndex(this.fileIndex, absPath);
  }

  private indexFile(file: string, el: TreeElement): void {
    if (!file) return;
    this.fileIndex.set(file, el);
    // Also index the workspace-absolute form so activeTextEditor's
    // absolute URI can match the CLI-relative path we stored.
    const abs = resolveWorkspacePath(file);
    if (abs !== file) this.fileIndex.set(abs, el);
  }
}

// ---------------------------------------------------------------------------
// AC-14: Drift detection helpers
// ---------------------------------------------------------------------------

function hashContent(content: string): string {
  return crypto.createHash('sha256').update(content).digest('hex');
}

async function scanForDrift(doc: vscode.TextDocument): Promise<void> {
  if (!driftDecorationType) return;
  const editor = vscode.window.visibleTextEditors.find(e => e.document === doc);
  if (!editor) return;

  const text = doc.getText();
  const specRefMatch = text.match(/\/\/\s*@spec\s+([\w-]+)|#\s*@spec\s+([\w-]+)/);
  if (!specRefMatch) {
    editor.setDecorations(driftDecorationType, []);
    return;
  }
  const specID = specRefMatch[1] || specRefMatch[2];

  // Find the spec file
  const specFiles = await vscode.workspace.findFiles(`**/${specID}.spec.yaml`, undefined, 1);
  if (specFiles.length === 0) {
    editor.setDecorations(driftDecorationType, []);
    return;
  }
  const specFile = specFiles[0].fsPath;

  // Current spec hash
  const currentContent = (await vscode.workspace.fs.readFile(specFiles[0])).toString();
  const currentHash = hashContent(currentContent);

  // Baseline: spec at HEAD (committed version)
  let baselineHash = currentHash; // default: no drift
  try {
    const { execFile } = require('child_process');
    const baseContent: string = await new Promise((resolve, reject) => {
      execFile('git', ['show', `HEAD:${path.relative(
        vscode.workspace.workspaceFolders?.[0]?.uri.fsPath ?? '.', specFile,
      )}`], { cwd: vscode.workspace.workspaceFolders?.[0]?.uri.fsPath },
      (err: any, stdout: string) => err ? reject(err) : resolve(stdout));
    });
    baselineHash = hashContent(baseContent);
  } catch {
    // Not a git repo or file not tracked — no drift possible
    editor.setDecorations(driftDecorationType, []);
    return;
  }

  // Find @ac lines and check for drift
  const decorations: vscode.DecorationOptions[] = [];
  const acPattern = /\/\/\s*@ac\s+(AC-\d+)|#\s*@ac\s+(AC-\d+)/g;
  let match: RegExpExecArray | null;
  while ((match = acPattern.exec(text)) !== null) {
    const acID = match[1] || match[2];
    const result = detectDrift(
      { specID, acID, specFileHashAtAnnotation: baselineHash },
      {
        currentSpecFileHash: currentHash,
        getDiff: () => null, // simplified: just detect hash change
      },
    );
    if (result.hasDrift) {
      const pos = doc.positionAt(match.index);
      const line = doc.lineAt(pos.line);
      const hover = buildDriftHover({
        specID,
        acID,
        changeClass: result.changeClass,
        baselineDescription: null,
        headDescription: null,
      });
      decorations.push({
        range: line.range,
        hoverMessage: new vscode.MarkdownString(hover.contents ?? `Spec \`${specID}\` has changed since this annotation was committed.`),
      });
    }
  }
  editor.setDecorations(driftDecorationType, decorations);
}

// ---------------------------------------------------------------------------
// Binary version check
// ---------------------------------------------------------------------------

/** Runs `specter --version` and returns the semver string, or null on failure. */
function getCachedBinaryVersion(binaryPath: string): string | null {
  try {
    const { execFileSync } = require('child_process');
    const out: string = execFileSync(binaryPath, ['--version'], {
      encoding: 'utf8',
      timeout: 5000,
    });
    // Output format: "specter version 0.6.0"
    const m = out.match(/(\d+\.\d+\.\d+)/);
    return m ? m[1] : null;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

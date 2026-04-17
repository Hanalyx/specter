// @spec spec-vscode

import * as vscode from 'vscode';
import * as path from 'path';

import { shouldActivate, resolveManifestPath, createClientKey } from './activation';
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
import { buildDiagnostics, DiagnosticReplacer, shouldRunCoverageForFile } from './diagnostics';
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
  buildTreeNodes,
  NotificationRateLimiter,
} from './coverage';
import { buildConstraintHover, resolveDefinitionTarget } from './navigation';
import { buildInsightCards, formatSpecContextForAI, shouldShowWalkthrough } from './insights';
import { detectDrift, buildDriftHover } from './drift';
import { SpecterClient } from './client';
import { detectShellConfig, isPathAlreadyPresent, formatAppendBlock } from './shellPath';
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
let statusBarItem: vscode.StatusBarItem | null = null;
let binaryPath: string | null = null;
const rateLimiter = new NotificationRateLimiter({ windowMs: 60_000 });
let treeProvider: SpecterTreeProvider | null = null;
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

  // AC-01: check activation conditions
  const allFiles = await vscode.workspace.findFiles('**/*.{yaml,yml}', undefined, 500);
  const filePaths = allFiles.map(u => u.fsPath);
  if (!shouldActivate(filePaths)) return;

  // AC-17: walkthrough for empty workspaces
  const specFiles = filePaths.filter(f => path.basename(f).endsWith('.spec.yaml'));
  const manifestFiles = filePaths.filter(f => path.basename(f) === 'specter.yaml');
  if (shouldShowWalkthrough({ specFiles, hasSpecterManifest: manifestFiles.length > 0 })) {
    vscode.commands.executeCommand(
      'workbench.action.openWalkthrough',
      'specter-team.specter-vscode#specter.gettingStarted',
    );
  }

  // AC-02: binary resolution
  const resolved = await resolveBinary(ctx);
  if (!resolved) return; // modal error already shown

  binaryPath = resolved;

  // Add ~/.specter/bin to the integrated terminal PATH so users can type
  // `specter` directly without needing to configure their shell profile.
  const specterBinDir = path.dirname(defaultCachePath());
  ctx.environmentVariableCollection.prepend('PATH', specterBinDir + path.delimiter);

  // Output channel for errors and logs.
  outputChannel = vscode.window.createOutputChannel('Specter');
  ctx.subscriptions.push(outputChannel);

  // Status bar (AC-12)
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
  treeProvider = new SpecterTreeProvider();
  vscode.window.registerTreeDataProvider('specterCoverage', treeProvider);

  // AC-14: Drift decoration type
  driftDecorationType = vscode.window.createTextEditorDecorationType({
    gutterIconPath: new vscode.ThemeIcon('warning').id ? undefined : undefined,
    overviewRulerColor: 'orange',
    overviewRulerLane: vscode.OverviewRulerLane.Left,
    after: {
      contentText: ' ⚠ spec changed',
      color: new vscode.ThemeColor('editorWarning.foreground'),
    },
  });

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

async function downloadBinary(_ctx: vscode.ExtensionContext): Promise<string | null> {
  const cfg = vscode.workspace.getConfiguration('specter');
  const versionSetting = cfg.get<string>('version', 'latest');

  return vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: 'Downloading Specter CLI…', cancellable: false },
    async (progress) => {
      try {
        // 1. Resolve version
        progress.report({ message: 'resolving version…' });
        const version = versionSetting === 'latest'
          ? await resolveLatestVersion()
          : versionSetting;

        // 2. Build download URL
        const dlOpts = { version, os: process.platform, arch: process.arch };
        const url = buildDownloadUrl(dlOpts);
        const targetPath = defaultCachePath();
        const archiveName = assetName(dlOpts);
        const format: 'tar.gz' | 'zip' = process.platform === 'win32' ? 'zip' : 'tar.gz';

        // 3. Download archive
        progress.report({ message: `downloading v${version}…` });
        const archiveData = await httpsGet(url);

        // 4. Verify SHA256
        progress.report({ message: 'verifying checksum…' });
        try {
          const checksums = await downloadChecksums(version);
          const expectedHash = checksums.get(archiveName);
          if (expectedHash) {
            const valid = await verifyChecksum(archiveData, expectedHash);
            if (!valid) {
              vscode.window.showErrorMessage(
                'Specter download failed: SHA256 checksum mismatch. The binary may have been tampered with.',
                { modal: true },
              );
              return null;
            }
          }
        } catch {
          // Checksum file not available — proceed without verification
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
// Coverage run helpers
// ---------------------------------------------------------------------------

async function runCoverageForFolder(key: string, client: SpecterClient): Promise<void> {
  try {
    const result = await client.coverage();
    coverageReport = result as unknown as CoverageReport;
    updateSpecIndex(coverageReport);
    updateStatusBar(coverageReport);
    treeProvider?.refresh();
  } catch (e) {
    // Don't hang on "loading…" forever. Surface the failure so the user can
    // act — log details, flip the status bar to an error state that opens
    // the output channel on click.
    const msg = e instanceof Error ? e.message : String(e);
    outputChannel?.appendLine(`[${new Date().toISOString()}] coverage run failed for ${key}:`);
    outputChannel?.appendLine('  ' + msg);
    if (statusBarItem) {
      statusBarItem.text = '$(error) Specter: error';
      statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
      statusBarItem.tooltip = 'Specter coverage failed. Click to view details.';
      statusBarItem.command = 'specter.showOutput';
    }
  }
}

function updateSpecIndex(report: CoverageReport): void {
  for (const entry of report.entries) {
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
  const hasT1OrT2Failure = report.entries.some(
    e => !e.passesThreshold && (e.tier === 1 || e.tier === 2),
  );
  const totalPct = report.entries.length === 0
    ? 0
    : Math.round(report.entries.reduce((s, e) => s + e.coveragePct, 0) / report.entries.length);
  const failing = report.entries.filter(e => !e.passesThreshold).length;

  const result = formatStatusBar({
    totalSpecs: report.entries.length,
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

  // -- Hover: @ac annotation in test files (AC-09)
  ctx.subscriptions.push(
    vscode.languages.registerHoverProvider(testFileSelector, {
      provideHover(doc, pos) {
        const line = doc.lineAt(pos.line).text;
        const m = line.match(/\/\/\s*@ac\s+(AC-\d+)/);
        if (!m) return;

        const specID = findNearestSpecAnnotation(doc.getText(), pos.line);
        if (!specID) return;

        const hover = buildAnnotationHover(specIndex, specID, m[1], {
          coveredByFiles: [],
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
                command: 'specter.insertAnnotation',
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

        const entry = coverageReport.entries.find(e => e.specID === specID);
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

        const entry = coverageReport.entries.find(e => e.specID === specID);
        if (!entry) return [];

        const decorations = buildACDecorations({
          coveredACs: entry.coveredACs,
          uncoveredACs: entry.uncoveredACs,
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
          } catch { /* ignore parse failures */ }
        }, 400),
      );
    }),
  );

  // On-save: check + coverage (AC-04)
  ctx.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async doc => {
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

  // AC-14: Scan for drift when test files are opened or saved
  ctx.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(editor => {
      if (editor) scanForDrift(editor.document).catch(() => {});
    }),
  );
  // Also trigger drift scan after save (test file may reference a changed spec)
  ctx.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(doc => {
      if (!doc.uri.fsPath.endsWith('.spec.yaml')) {
        scanForDrift(doc).catch(() => {});
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
      const panel = vscode.window.createWebviewPanel(
        'specterInsights',
        'Specter Insights',
        vscode.ViewColumn.Beside,
        { enableScripts: false },
      );
      const cards = buildInsightCards(coverageReport.entries, specIndex);
      panel.webview.html = renderInsightsHTML(cards);
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
      for (const [key, client] of clients) {
        await runCoverageForFolder(key, client);
      }
      vscode.window.showInformationMessage('Specter sync complete.');
    }),
  );

  // Insert annotation (used by code lens)
  ctx.subscriptions.push(
    vscode.commands.registerCommand(
      'specter.insertAnnotation',
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

function renderInsightsHTML(cards: ReturnType<typeof buildInsightCards>): string {
  if (cards.length === 0) {
    return `<!DOCTYPE html><html><body><h1>All specs passing ✓</h1></body></html>`;
  }

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

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  body { font-family: var(--vscode-font-family); padding: 1rem; }
  .card { border: 1px solid var(--vscode-panel-border); border-radius: 4px; padding: 1rem; margin-bottom: 1rem; }
  h2 { margin-top: 0; color: var(--vscode-errorForeground); }
  h3 { color: var(--vscode-descriptionForeground); }
</style>
</head>
<body>
<h1>Specter Insights</h1>
${cardHTML}
</body>
</html>`;
}

// ---------------------------------------------------------------------------
// AC-11: Tree view provider
// ---------------------------------------------------------------------------

type TreeElement = { kind: 'spec'; specID: string; file: string; children: TreeElement[] }
  | { kind: 'ac'; id: string; icon: 'covered' | 'uncovered' | 'gap'; children: TreeElement[] }
  | { kind: 'testFile'; path: string };

class SpecterTreeProvider implements vscode.TreeDataProvider<TreeElement> {
  private _onDidChange = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  refresh(): void { this._onDidChange.fire(); }

  getTreeItem(el: TreeElement): vscode.TreeItem {
    switch (el.kind) {
      case 'spec': {
        const item = new vscode.TreeItem(el.specID, vscode.TreeItemCollapsibleState.Collapsed);
        item.contextValue = 'spec';
        item.iconPath = new vscode.ThemeIcon('file-code');
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
        const item = new vscode.TreeItem(path.basename(el.path), vscode.TreeItemCollapsibleState.None);
        item.resourceUri = vscode.Uri.file(el.path);
        item.command = { command: 'vscode.open', title: 'Open', arguments: [vscode.Uri.file(el.path)] };
        item.iconPath = new vscode.ThemeIcon('file');
        return item;
      }
    }
  }

  getChildren(el?: TreeElement): TreeElement[] {
    if (!el) {
      if (!coverageReport) return [];
      const nodes = buildTreeNodes(coverageReport);
      return nodes.map(n => ({
        kind: 'spec' as const,
        specID: n.specID,
        file: n.file,
        children: n.children.map(ac => ({
          kind: 'ac' as const,
          id: ac.id,
          icon: ac.icon,
          children: ac.children.map(tf => ({ kind: 'testFile' as const, path: tf.path })),
        })),
      }));
    }
    return el.kind === 'testFile' ? [] : el.children;
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

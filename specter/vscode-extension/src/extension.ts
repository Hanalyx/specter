// @spec spec-vscode

import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';

import { shouldActivate, resolveManifestPath, createClientKey } from './activation';
import {
  resolveBinaryPath,
  buildDownloadUrl,
  verifyChecksum,
  defaultCachePath,
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
  buildACDecorations,
  buildTreeNodes,
  formatStatusBar,
  classifyNotification,
  buildFileDecoration,
  NotificationRateLimiter,
} from './coverage';
import { buildConstraintHover, resolveDefinitionTarget } from './navigation';
import { buildInsightCards, formatSpecContextForAI, shouldShowWalkthrough } from './insights';
import { detectDrift, buildDriftHover } from './drift';
import { SpecterClient } from './client';
import type {
  SpecIndex,
  SpecEntry,
  CoverageReport,
  SpecCoverageEntry,
  DriftBaseline,
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

// ---------------------------------------------------------------------------
// Activation
// ---------------------------------------------------------------------------

export async function activate(ctx: vscode.ExtensionContext): Promise<void> {
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

  // Providers (registered once, they read per-folder state as needed)
  registerProviders(ctx);

  // Commands
  registerCommands(ctx);
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

  if (resolved) return resolved;

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
  const version = cfg.get<string>('version', 'latest');

  const targetPath = defaultCachePath();
  const url = buildDownloadUrl({
    version,
    os: process.platform,
    arch: process.arch,
  });

  return vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: 'Downloading Specter binary…' },
    async () => {
      try {
        const https = require('https');
        const fs = require('fs');
        const zlib = require('zlib');
        const tarStream = require('tar-stream');

        const data: Buffer = await new Promise((resolve, reject) => {
          https.get(url, (res: any) => {
            const chunks: Buffer[] = [];
            res.on('data', (c: Buffer) => chunks.push(c));
            res.on('end', () => resolve(Buffer.concat(chunks)));
            res.on('error', reject);
          }).on('error', reject);
        });

        // Write and make executable
        const dir = path.dirname(targetPath);
        fs.mkdirSync(dir, { recursive: true });
        fs.writeFileSync(targetPath, data);
        fs.chmodSync(targetPath, 0o755);
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
  } catch { /* specter not yet configured — ignore */ }
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

            // Notification discipline (AC-18, AC-19)
            if (newEntry && !newEntry.passesThreshold) {
              const kind = classifyNotification({
                tier: newEntry.tier,
                droppedBelowThreshold: true,
              });
              maybeNotify(specID, kind.kind, newEntry.tier);
            }
          }
        }
      } catch { /* ignore */ }
    }),
  );

  // Clean up diagnostics on file rename / delete (AC-04)
  ctx.subscriptions.push(
    vscode.workspace.onDidRenameFiles(e => {
      for (const { oldUri, newUri } of e.files) {
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

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// @spec spec-vscode
//
// Parity + structural tests for the VS Code extension's command wiring,
// disposable lifecycle, activation control-flow, and error-surfacing
// discipline. Static-analysis over extension.ts + package.json is the
// cheapest way to enforce structural invariants without spinning up a
// full vscode runtime mock.
//
// Every test in this file MUST pass as a single source of truth for the
// invariants described in spec-vscode v1.3.0 constraints C-22 through C-26.
// All of them currently FAIL against v0.9.0 code — that's the point; they
// define the v0.9.1 contract.

import * as fs from 'fs';
import * as path from 'path';

const EXT_ROOT = path.resolve(__dirname, '..', '..');
const EXT_TS = path.resolve(__dirname, '..', 'extension.ts');
const PKG_JSON = path.resolve(EXT_ROOT, 'package.json');

const readSrc = () => fs.readFileSync(EXT_TS, 'utf-8');
const readPkg = () => JSON.parse(fs.readFileSync(PKG_JSON, 'utf-8'));

// ---------------------------------------------------------------------------
// AC-41 / AC-43 / AC-44: command parity
// ---------------------------------------------------------------------------

// @ac AC-41
// @ac AC-43
// @ac AC-44
describe('[spec-vscode/AC-41] command parity (package.json ↔ extension.ts)', () => {
  it('every declared command has a registered handler', () => {
    const pkg = readPkg();
    const declared = new Set<string>(
      (pkg.contributes?.commands ?? []).map((c: { command: string }) => c.command),
    );

    const src = readSrc();
    const registered = new Set<string>();
    for (const m of src.matchAll(/registerCommand\(\s*['"]([^'"]+)['"]/g)) {
      registered.add(m[1]);
    }

    const unregistered = [...declared].filter(c => !registered.has(c)).sort();
    expect(unregistered).toEqual([]);
  });

  it('every public registered handler is declared in package.json', () => {
    // Catches the reverse: handlers for commands that were renamed/removed
    // and would be unreachable from the command palette. Commands whose id
    // begins with `specter._` are internal by VS Code community convention
    // (invoked programmatically from CodeActions / CodeLenses, never from
    // the palette) and are exempt from the declaration requirement.
    const pkg = readPkg();
    const declared = new Set<string>(
      (pkg.contributes?.commands ?? []).map((c: { command: string }) => c.command),
    );

    const src = readSrc();
    const registered = new Set<string>();
    for (const m of src.matchAll(/registerCommand\(\s*['"]([^'"]+)['"]/g)) {
      const id = m[1];
      if (id.startsWith('specter.') && !id.startsWith('specter._')) {
        registered.add(id);
      }
    }

    const undeclared = [...registered].filter(c => !declared.has(c)).sort();
    expect(undeclared).toEqual([]);
  });

  it('specter.runReverse is wired end-to-end', () => {
    // AC-43 explicit: walkthrough step 1 invokes this command via a link;
    // the command palette lists it. Registration MUST be present.
    const src = readSrc();
    expect(src).toMatch(/registerCommand\(\s*['"]specter\.runReverse['"]/);
  });

  it('specter.openQuickStart has no orphan declaration', () => {
    // AC-44: either register it, or remove it from package.json. An orphan
    // declaration is prohibited.
    const pkg = readPkg();
    const declared = (pkg.contributes?.commands ?? []).some(
      (c: { command: string }) => c.command === 'specter.openQuickStart',
    );
    if (!declared) return; // Removed → AC satisfied.
    const src = readSrc();
    expect(src).toMatch(/registerCommand\(\s*['"]specter\.openQuickStart['"]/);
  });
});

// ---------------------------------------------------------------------------
// AC-42: disposables pushed to ctx.subscriptions
// ---------------------------------------------------------------------------

// @ac AC-42
describe('[spec-vscode/AC-42] disposables lifecycle', () => {
  it('driftDecorationType is pushed to ctx.subscriptions', () => {
    const src = readSrc();
    // Creation site must exist (sanity check — if the factory call is gone,
    // this test is stale).
    expect(src).toMatch(
      /driftDecorationType\s*=\s*vscode\.window\.createTextEditorDecorationType\(/,
    );
    // And the disposable MUST be pushed to subscriptions somewhere.
    expect(src).toMatch(/ctx\.subscriptions\.push\(\s*driftDecorationType/);
  });
});

// ---------------------------------------------------------------------------
// AC-45 / AC-46: activation structure
// ---------------------------------------------------------------------------

// @ac AC-45
// @ac AC-46
describe('[spec-vscode/AC-45] activation control flow', () => {
  it('binary resolution runs before the hasSpecOrManifest early-return', () => {
    // AC-45: commands like specter.runReverse must work in empty workspaces.
    // That requires resolveBinary to run before we short-circuit on
    // "workspace has no specs." The invariant is encoded as: the result
    // of shouldActivate(...) is captured into a variable, and the early-
    // return happens AFTER resolveBinary. The test searches for the
    // early-return by whichever variable name the code uses; both
    // `hasSpecOrManifest` and the legacy inline `shouldActivate(...)`
    // form are accepted.
    const src = readSrc();
    const resolveBinaryIdx = src.indexOf('await resolveBinary(ctx)');
    expect(resolveBinaryIdx).toBeGreaterThan(-1);

    // The early-return can be any of:
    //   if (!shouldActivate(filePaths)) return
    //   if (!hasSpecOrManifest) return
    // Take the first that matches.
    const candidates = [
      'if (!shouldActivate(filePaths)) return',
      'if (!hasSpecOrManifest) return',
    ];
    const earlyReturnIdx = candidates
      .map(p => src.indexOf(p))
      .filter(i => i > -1)
      .sort((a, b) => a - b)[0] ?? -1;
    expect(earlyReturnIdx).toBeGreaterThan(-1);
    expect(resolveBinaryIdx).toBeLessThan(earlyReturnIdx);
  });

  it('shouldShowWalkthrough is reachable when its condition holds', () => {
    // AC-46: the walkthrough check MUST run regardless of whether the
    // workspace has spec files, since its whole point is "show me how to
    // start a new Specter project."
    const src = readSrc();
    const walkthroughIdx = src.indexOf('shouldShowWalkthrough(');
    expect(walkthroughIdx).toBeGreaterThan(-1);

    const candidates = [
      'if (!shouldActivate(filePaths)) return',
      'if (!hasSpecOrManifest) return',
    ];
    const earlyReturnIdx = candidates
      .map(p => src.indexOf(p))
      .filter(i => i > -1)
      .sort((a, b) => a - b)[0] ?? -1;
    expect(earlyReturnIdx).toBeGreaterThan(-1);
    expect(walkthroughIdx).toBeLessThan(earlyReturnIdx);
  });
});

// ---------------------------------------------------------------------------
// AC-47: mandatory checksum verification
// ---------------------------------------------------------------------------

// @ac AC-47
describe('[spec-vscode/AC-47] binary download integrity', () => {
  it('does not silently fall back when checksum verification is unavailable', () => {
    // The v0.9.0 code has:
    //   try { ...verify... } catch { /* proceed without verification */ }
    // This is the CRITICAL security finding. The fallback MUST be removed.
    const src = readSrc();
    expect(src).not.toMatch(/proceed without verification/i);
    expect(src).not.toMatch(/Checksum file not available/i);
  });

  it('missing checksum entry for the archive is treated as failure', () => {
    // Current code:
    //   if (expectedHash) { ...verify... }
    // If expectedHash is undefined (entry missing from checksums.txt), the
    // code skips verification entirely. This too must go — missing-entry
    // is a failure signal, not a green light.
    const src = readSrc();
    // The anti-pattern: an `if (expectedHash)` gate around verifyChecksum
    // that has no `else { throw }` branch. Heuristic — if the code still
    // reads "if (expectedHash) {" without a corresponding error branch
    // nearby, flag it.
    const m = src.match(/if\s*\(\s*expectedHash\s*\)/);
    // If the pattern is gone, AC satisfied.
    if (!m) return;
    // If it's still there, the code around it must include a throw/return
    // null on the else path. Check for a matching "else" or "!expectedHash"
    // failure-branch nearby.
    const idx = m.index!;
    const window = src.slice(idx, idx + 800);
    const hasFailureBranch =
      /else\s*\{[^}]*(throw|return\s+null)/s.test(window) ||
      /!\s*expectedHash/.test(window);
    expect(hasFailureBranch).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// AC-48 / AC-49: error-surfacing discipline
// ---------------------------------------------------------------------------

// @ac AC-48
describe('[spec-vscode/AC-48] on-type parse-error surfacing', () => {
  it('does not silently ignore parse failures in the on-change hook', () => {
    // v0.9.0 has:
    //   } catch { /* ignore parse failures */ }
    // inside the onDidChangeTextDocument hook. AC-48 requires routing to
    // the Output channel instead.
    const src = readSrc();
    expect(src).not.toMatch(/\/\*\s*ignore parse failures\s*\*\//);
  });
});

// @ac AC-49
describe('[spec-vscode/AC-49] drift-scan error surfacing', () => {
  it('does not silently swallow drift detection failures', () => {
    // v0.9.0 has two sites with `scanForDrift(...).catch(() => {})` — one
    // in onDidChangeActiveTextEditor, one in onDidSaveTextDocument. AC-49
    // requires each to log to the Output channel.
    const src = readSrc();
    const silentDrift = src.match(
      /scanForDrift\s*\([^)]*\)\s*\.catch\(\s*\(\s*\)\s*=>\s*\{\s*\}\s*\)/g,
    );
    expect(silentDrift).toBeNull();
  });
});

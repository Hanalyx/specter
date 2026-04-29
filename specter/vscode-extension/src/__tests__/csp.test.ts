// @spec spec-vscode
//
// M4 (chore/v0.12-security-hardening): static-analysis tests for the
// webview Content Security Policy. The invariant is "the insights
// webview MUST be served with a CSP that restricts script execution to
// a per-render nonce" — without this, a future regression that drops
// the CSP meta tag, hardcodes the nonce, or relaxes script-src would
// ship silently. The CSP is M4's whole purpose; if it's not enforced
// in tests, M4 is aspirational.
//
// Static analysis is the cheapest way to enforce the invariants without
// spinning up a vscode runtime mock. The renderer (renderInsightsHTML)
// is not exported, so we scan extension.ts directly.

import * as fs from 'fs';
import * as path from 'path';

const EXT_TS = path.resolve(__dirname, '..', 'extension.ts');
const readSrc = () => fs.readFileSync(EXT_TS, 'utf-8');

describe('[spec-vscode/M4] webview CSP invariants', () => {
  it('insights webview emits a Content-Security-Policy meta tag', () => {
    const src = readSrc();
    // The CSP meta tag MUST exist in the rendered HTML. Its absence —
    // accidental deletion, refactor that drops it — is the regression
    // M4 was designed to prevent.
    expect(src).toMatch(
      /<meta\s+http-equiv="Content-Security-Policy"\s+content="[^"]+"/,
    );
  });

  it('CSP locks default-src to none', () => {
    const src = readSrc();
    expect(src).toMatch(/default-src\s+'none'/);
  });

  it('CSP restricts script-src to a per-render nonce template', () => {
    const src = readSrc();
    // The nonce MUST come from a template variable, not a hardcoded
    // string. `script-src 'nonce-${nonce}'` is the contract; a future
    // change like `script-src 'nonce-abc123'` (literal) would defeat the
    // per-render guarantee and fail this assertion.
    expect(src).toMatch(/script-src\s+'nonce-\$\{nonce\}'/);
  });

  it('inline <script> carries the matching nonce attribute', () => {
    const src = readSrc();
    // The script tag MUST reference the same template variable as the
    // CSP. If they diverge, the script will be blocked at runtime; this
    // test is the cheap pre-flight check.
    expect(src).toMatch(/<script\s+nonce="\$\{nonce\}"/);
  });

  it('nonce is generated per render via crypto.randomBytes', () => {
    const src = readSrc();
    // The per-render guarantee depends on the nonce being freshly
    // generated for each panel. A regression to a module-level constant
    // (e.g., `const nonce = 'static'`) would still satisfy the template
    // variable check above, so we anchor the source on the cryptographic
    // call. randomBytes(16) gives 128 bits of entropy — adequate for
    // CSP nonces; if it shrinks below 8 bytes, this test will fail and
    // the change must justify itself.
    expect(src).toMatch(/randomBytes\(\s*(\d+)\s*\)/);
    const m = src.match(/randomBytes\(\s*(\d+)\s*\)/);
    if (m) {
      const bytes = parseInt(m[1], 10);
      expect(bytes).toBeGreaterThanOrEqual(8);
    }
  });

  it('webview restricts localResourceRoots to disable fs access by default', () => {
    const src = readSrc();
    // Defense-in-depth: even with the CSP, an empty localResourceRoots
    // means a script that escapes nonce restriction can't reach the
    // file system via webview URIs. Empty array (no roots) is the
    // strongest setting; we accept that exact form.
    expect(src).toMatch(/localResourceRoots:\s*\[\s*\]/);
  });
});

// @spec spec-vscode
//
// Tests for the quick-fix helper that identifies Unknown-field parse
// diagnostics matching the known-removed-fields list. Runtime-free: no
// vscode imports — pure string predicate.

import * as fs from 'fs';
import * as path from 'path';

import { matchRemovedFieldDiagnostic, KNOWN_REMOVED_FIELDS } from '../quickFix';

// @ac AC-50
describe('matchRemovedFieldDiagnostic', () => {
  it('returns the field name for a known-removed Unknown-field diagnostic', () => {
    const msg = "Unknown field 'trust_level'. Remove it or check for a typo in the field name.";
    expect(matchRemovedFieldDiagnostic(msg)).toBe('trust_level');
  });

  it('returns null for Unknown-field of a field not in the known-removed list', () => {
    const msg = "Unknown field 'totally_new_field'. Remove it or check for a typo in the field name.";
    expect(matchRemovedFieldDiagnostic(msg)).toBeNull();
  });

  it('returns null for a non-Unknown-field diagnostic', () => {
    const msg = "Missing required field 'id'";
    expect(matchRemovedFieldDiagnostic(msg)).toBeNull();
  });

  it('known-removed list includes trust_level (v0.10 seed)', () => {
    expect(KNOWN_REMOVED_FIELDS).toContain('trust_level');
  });
});

// @ac AC-51
// Static-analysis test: extension.ts must register a CodeActionProvider
// keyed on the yaml language. We do not spin up a vscode runtime — we grep.
describe('CodeActionProvider registration (static)', () => {
  it('extension.ts registers a CodeActionProvider for yaml', () => {
    const srcPath = path.resolve(__dirname, '..', 'extension.ts');
    const src = fs.readFileSync(srcPath, 'utf-8');

    if (!/registerCodeActionsProvider\s*\(/.test(src)) {
      throw new Error(
        'extension.ts does not call vscode.languages.registerCodeActionsProvider — quick-fix provider must be registered (AC-51).',
      );
    }
    // Provider must mention yaml/language key somewhere in the call.
    const providerBlockMatch = src.match(/registerCodeActionsProvider\s*\([\s\S]{0,400}?\)/);
    if (!providerBlockMatch) {
      throw new Error(
        'Could not find a complete registerCodeActionsProvider(...) invocation — AC-51 expects yaml keying.',
      );
    }
    if (!/yaml/.test(providerBlockMatch[0])) {
      throw new Error(
        `CodeActionProvider registration must key on the yaml language; found: ${providerBlockMatch[0]}`,
      );
    }
  });
});

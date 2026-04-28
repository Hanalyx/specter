// quickFix.test.ts — tests for the v0.12 deprecated-field quick-fix.
//
// @spec spec-vscode

import * as fs from 'fs';
import * as path from 'path';

import { matchRemovedFieldDiagnostic, KNOWN_REMOVED_FIELDS } from '../quickFix';

// @ac AC-51
describe('spec-vscode/AC-51 matchRemovedFieldDiagnostic', () => {
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

  it('known-removed list includes trust_level (v0.12 seed)', () => {
    expect(KNOWN_REMOVED_FIELDS).toContain('trust_level');
  });
});

// @ac AC-52
// Static-analysis test: extension.ts must register a CodeActionProvider
// keyed on the yaml language. We do not spin up a vscode runtime — we grep
// the source for the registration call.
describe('spec-vscode/AC-52 CodeActionProvider registration (static)', () => {
  it('extension.ts registers a CodeActionProvider for yaml', () => {
    const srcPath = path.resolve(__dirname, '..', 'extension.ts');
    const src = fs.readFileSync(srcPath, 'utf-8');

    // Must call vscode.languages.registerCodeActionsProvider at least once.
    expect(src).toMatch(/registerCodeActionsProvider/);

    // And it must be keyed on the yaml language (selector `'yaml'` or
    // `{ language: 'yaml' }`). Either form is acceptable.
    const yamlSelectorRE = /registerCodeActionsProvider\s*\(\s*['"`]yaml['"`]|registerCodeActionsProvider\s*\([^)]*language\s*:\s*['"`]yaml['"`]/;
    expect(src).toMatch(yamlSelectorRE);
  });
});

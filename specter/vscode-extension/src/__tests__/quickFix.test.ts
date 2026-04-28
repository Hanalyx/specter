// quickFix.test.ts — tests for the v0.12 deprecated-field quick-fix.
//
// @spec spec-vscode

import * as fs from 'fs';
import * as path from 'path';

import { matchRemovedFieldDiagnostic, KNOWN_REMOVED_FIELDS, isLineSafeToDelete } from '../quickFix';

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
// against a yaml-typed selector. Two acceptable forms (per AC-52 v1.6.0):
//   1. Inline literal: `registerCodeActionsProvider('yaml', ...)` or
//      `registerCodeActionsProvider({ language: 'yaml' }, ...)`.
//   2. Named DocumentSelector constant: `registerCodeActionsProvider(
//      mySelector, ...)` where `mySelector` is declared with
//      `language: 'yaml'` somewhere in the same source file.
// The test's job is to verify the binding resolves to yaml, not to
// dictate the surface syntax.
describe('spec-vscode/AC-52 CodeActionProvider registration (static)', () => {
  it('extension.ts registers a CodeActionProvider whose selector resolves to yaml', () => {
    const srcPath = path.resolve(__dirname, '..', 'extension.ts');
    const src = fs.readFileSync(srcPath, 'utf-8');

    // Must call vscode.languages.registerCodeActionsProvider at least once.
    const callRE = /registerCodeActionsProvider\s*\(\s*([^,)]+)/g;
    const calls: string[] = [];
    let m: RegExpExecArray | null;
    while ((m = callRE.exec(src)) !== null) {
      calls.push(m[1].trim());
    }
    expect(calls.length).toBeGreaterThanOrEqual(1);

    // For each call, the first argument is either an inline yaml literal
    // OR a named identifier that's declared with `language: 'yaml'`.
    const someYaml = calls.some((arg) => {
      // Inline literal form 1: `'yaml'` / `"yaml"`.
      if (/^['"`]yaml['"`]$/.test(arg)) return true;
      // Inline object literal form 2: starts with `{` and contains `language: 'yaml'`.
      if (/^\{.*language\s*:\s*['"`]yaml['"`]/.test(arg)) return true;
      // Named identifier form: look up its declaration in the source.
      // Match `<name>: vscode.DocumentSelector = { language: 'yaml' ... }`.
      const declRE = new RegExp(
        '\\b' + arg.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') +
        '\\s*:\\s*vscode\\.DocumentSelector\\s*=\\s*\\{[^}]*language\\s*:\\s*[\'"`]yaml[\'"`]',
      );
      return declRE.test(src);
    });
    expect(someYaml).toBe(true);
  });
});

// @ac AC-53
// isLineSafeToDelete predicate: plain scalar / quoted / numeric → true;
// block scalar (with chomp/indent) / empty value / unterminated quote → false.
describe('spec-vscode/AC-53 isLineSafeToDelete shape predicate', () => {
  const cases: Array<[string, boolean]> = [
    // safe — plain scalar variants
    ['  trust_level: high', true],
    ['  trust_level:    high', true],
    ['  trust_level: "high"', true],
    ["  trust_level: 'high'", true],
    ['  trust_level: 0.5', true],
    ['  trust_level: 1', true],
    // unsafe — block scalars (literal, folded, with chomp / indent)
    ['  trust_level: |', false],
    ['  trust_level: |-', false],
    ['  trust_level: |+', false],
    ['  trust_level: |2', false],
    ['  trust_level: >', false],
    ['  trust_level: >-', false],
    ['  trust_level: > # with comment', false],
    // unsafe — key with no value (next-line introduces sequence/mapping)
    ['  trust_level:', false],
    ['  trust_level: ', false],
    // unsafe — unterminated quoted strings
    ['  trust_level: "unterminated', false],
    ["  trust_level: 'unterminated", false],
  ];

  for (const [line, expected] of cases) {
    it(`returns ${expected} for ${JSON.stringify(line)}`, () => {
      // Helper takes the line text directly (caller passes
      // doc.lineAt(N).text in production).
      expect(isLineSafeToDelete(line)).toBe(expected);
    });
  }
});

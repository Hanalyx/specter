// @spec spec-vscode
//
// Tests for diagnostic lifecycle: debounce, atomic replacement, and on-save
// triggering of the correct specter commands.

import { buildDiagnostics, buildCoverageParseDiagnostics, DiagnosticReplacer } from '../diagnostics';
import type { SpecterParseError, SpecterCheckDiagnostic, CoverageParseError } from '../types';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// Both fixtures are the shapes `specter parse --json` and `specter check
// --json` actually emit, per spec-vscode C-32. They were wrong until v1.9.0:
// the parse fixture supplied `col` and `code`, which the CLI names `column`
// and `type`, and the check fixture supplied a `line`, which check diagnostics
// carry in no form. The `NaN` ranges that produced were invisible because
// every assertion here read a field the mock had invented.
//
// The casts are here because `types.ts` still declares those fields and
// declares `line` as required on both. They become no-ops once the
// declarations match the CLI (spec-vscode AC-67, AC-72, AC-73) and should be
// dropped then.

// A YAML syntax error, which is the only parse error the CLI emits with a
// position. It carries `line` and never `column`: `parser.ParseError.Column`
// is declared but no code path assigns it.
const parseError = {
  file: '/project/specs/auth.spec.yaml',
  path: '',
  type: 'yaml_syntax',
  message: 'yaml: line 12: mapping values are not allowed in this context',
  line: 12,
} as unknown as SpecterParseError;

// A check diagnostic, which carries `kind`, `severity`, `message`, and the
// spec id on every one, and no position of any kind.
const checkDiagnostic = {
  kind: 'orphan_constraint',
  severity: 'warning',
  specID: 'auth',
  constraintID: 'C-03',
  message: "Constraint C-03 in 'auth' is not referenced by any acceptance criterion",
  file: '/project/specs/auth.spec.yaml',
} as unknown as SpecterCheckDiagnostic;

// ---------------------------------------------------------------------------
// AC-03: On-type debounce, parse only, 400ms
// ---------------------------------------------------------------------------

// @ac AC-03
describe('[spec-vscode/AC-03] debounce timing', () => {
  it('on-type trigger invokes parse command, not check or coverage', () => {
    const invocations: string[] = [];
    const trigger = buildTrigger({ onInvoke: (cmd) => invocations.push(cmd) });
    trigger.onType('/project/specs/auth.spec.yaml');
    expect(invocations).toContain('parse');
    expect(invocations).not.toContain('check');
    expect(invocations).not.toContain('coverage');
  });

  it('on-save trigger invokes check and coverage commands', () => {
    const invocations: string[] = [];
    const trigger = buildTrigger({ onInvoke: (cmd) => invocations.push(cmd) });
    trigger.onSave('/project/specs/auth.spec.yaml');
    expect(invocations).toContain('check');
    expect(invocations).toContain('coverage');
  });
});

// ---------------------------------------------------------------------------
// AC-03 / AC-04: Diagnostics are built correctly from specter JSON output
// ---------------------------------------------------------------------------

// @ac AC-03
// @ac AC-04
describe('[spec-vscode/AC-03] buildDiagnostics', () => {
  it('maps a parse error to a VS Code diagnostic with correct severity and range', () => {
    const diags = buildDiagnostics({ parseErrors: [parseError], checkDiagnostics: [] });
    expect(diags).toHaveLength(1);
    const d = diags[0];
    expect(d.severity).toBe('error');
    expect(d.range.start.line).toBe(11); // VS Code is 0-indexed; specter line 12 becomes 11
    // The CLI emits no column on a parse error, and C-32 says an absent
    // position is 0 rather than arithmetic on `undefined`.
    expect(d.range.start.character).toBe(0);
    expect(d.message).toContain('mapping values are not allowed');
  });

  it('maps an orphan_constraint warning to DiagnosticSeverity Warning', () => {
    const diags = buildDiagnostics({ parseErrors: [], checkDiagnostics: [checkDiagnostic] });
    expect(diags[0].severity).toBe('warning');
    expect(diags[0].source).toBe('specter');
  });

  it('sets diagnostic source to "specter" on all entries', () => {
    const diags = buildDiagnostics({ parseErrors: [parseError], checkDiagnostics: [checkDiagnostic] });
    expect(diags.every(d => d.source === 'specter')).toBe(true);
  });

  it('returns empty array when both inputs are empty', () => {
    expect(buildDiagnostics({ parseErrors: [], checkDiagnostics: [] })).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// AC-04: Atomic replacement. DiagnosticReplacer never appends
// ---------------------------------------------------------------------------

// @ac AC-04
describe('[spec-vscode/AC-04] DiagnosticReplacer', () => {
  it('replaces all diagnostics for a URI atomically (set, not append)', () => {
    const store: Map<string, any[]> = new Map();
    const replacer = new DiagnosticReplacer({
      set: (uri, diags) => store.set(uri, diags),
      delete: (uri) => store.delete(uri),
    });

    replacer.replace('/project/specs/auth.spec.yaml', [{ message: 'first error' }]);
    replacer.replace('/project/specs/auth.spec.yaml', [{ message: 'second error' }]);

    const diags = store.get('/project/specs/auth.spec.yaml')!;
    expect(diags).toHaveLength(1);
    expect(diags[0].message).toBe('second error');
  });

  it('deletes all diagnostics for a URI when given an empty array', () => {
    const store: Map<string, any[]> = new Map([
      ['/project/specs/auth.spec.yaml', [{ message: 'stale' }]],
    ]);
    const replacer = new DiagnosticReplacer({
      set: (uri, diags) => store.set(uri, diags),
      delete: (uri) => store.delete(uri),
    });

    replacer.replace('/project/specs/auth.spec.yaml', []);
    expect(store.has('/project/specs/auth.spec.yaml')).toBe(false);
  });

  it('does not affect diagnostics for other URIs when replacing one file', () => {
    const store: Map<string, any[]> = new Map([
      ['/project/specs/payments.spec.yaml', [{ message: 'unrelated' }]],
    ]);
    const replacer = new DiagnosticReplacer({
      set: (uri, diags) => store.set(uri, diags),
      delete: (uri) => store.delete(uri),
    });

    replacer.replace('/project/specs/auth.spec.yaml', [{ message: 'auth error' }]);
    expect(store.get('/project/specs/payments.spec.yaml')).toHaveLength(1);
  });
});

// shouldRunCoverageForFile tests removed in v1.7.0 because the helper is gone
// (spec-vscode AC-55: on-save refreshes the saved file's folder report
// wholesale; the slash-comment regex trigger is prohibited).

// ---------------------------------------------------------------------------
// AC-34 (v0.9.0): coverage parse_errors → per-file VS Code diagnostics
// ---------------------------------------------------------------------------

// @spec spec-vscode
// @ac AC-34
describe('[spec-vscode/AC-34] buildCoverageParseDiagnostics', () => {
  it('groups one diagnostic per error and keys by file', () => {
    const errors: CoverageParseError[] = [
      { file: 'specs/a.spec.yaml', type: 'required', path: 'spec.objective', message: 'missing', line: 5, column: 3 },
      { file: 'specs/a.spec.yaml', type: 'enum', path: 'spec.status', message: 'bad value', line: 2, column: 1 },
      { file: 'specs/b.spec.yaml', type: 'required', path: 'spec.objective', message: 'missing', line: 1, column: 1 },
    ];
    const out = buildCoverageParseDiagnostics(errors);
    expect(out).toHaveLength(2);
    const a = out.find(x => x.file === 'specs/a.spec.yaml');
    expect(a?.diagnostics).toHaveLength(2);
    const b = out.find(x => x.file === 'specs/b.spec.yaml');
    expect(b?.diagnostics).toHaveLength(1);
  });

  it('converts 1-indexed line/column to 0-indexed ranges', () => {
    const out = buildCoverageParseDiagnostics([
      { file: 'x.yaml', message: 'oops', line: 10, column: 4 },
    ]);
    expect(out[0].diagnostics[0].range.start.line).toBe(9);
    expect(out[0].diagnostics[0].range.start.character).toBe(3);
  });

  it('prefixes the error type into the message', () => {
    const out = buildCoverageParseDiagnostics([
      { file: 'x.yaml', type: 'required', message: 'field is missing' },
    ]);
    expect(out[0].diagnostics[0].message).toContain('[required]');
    expect(out[0].diagnostics[0].message).toContain('field is missing');
  });

  it('returns [] for empty/null input', () => {
    expect(buildCoverageParseDiagnostics(null)).toEqual([]);
    expect(buildCoverageParseDiagnostics(undefined)).toEqual([]);
    expect(buildCoverageParseDiagnostics([])).toEqual([]);
  });

  it('falls back to line 0 when line is missing', () => {
    const out = buildCoverageParseDiagnostics([{ file: 'x.yaml', message: 'err' }]);
    expect(out[0].diagnostics[0].range.start.line).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Internal test double
// ---------------------------------------------------------------------------

function buildTrigger(opts: { onInvoke: (cmd: string) => void }) {
  return {
    onType: (file: string) => opts.onInvoke('parse'),
    onSave: (file: string) => { opts.onInvoke('check'); opts.onInvoke('coverage'); },
  };
}

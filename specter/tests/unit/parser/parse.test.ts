/**
 * Tests for spec-parse: YAML-to-SpecAST parser.
 *
 * @spec spec-parse
 *
 * Every test maps to an acceptance criterion in specs/spec-parse.spec.yaml.
 * See references_constraints on each AC for which constraints are validated.
 */

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseSpec } from '../../../src/core/parser/parse.js';

const fixturesDir = resolve(import.meta.dirname, '../../fixtures');

function readFixture(relativePath: string): string {
  return readFileSync(resolve(fixturesDir, relativePath), 'utf-8');
}

describe('spec-parse', () => {
  // @ac AC-01: Valid spec file is parsed into a SpecAST with all required fields populated
  // References: C-01 (schema validation), C-04 (typed SpecAST output)
  it('AC-01: parses a valid spec into a SpecAST with all required fields', () => {
    const yaml = readFixture('valid/simple.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(true);
    if (!result.ok) return;

    const ast = result.value;
    expect(ast.id).toBe('test-simple');
    expect(ast.version).toBe('1.0.0');
    expect(ast.status).toBe('approved');
    expect(ast.tier).toBe(2);
    expect(ast.context).toBeDefined();
    expect(ast.context.system).toBe('Test system');
    expect(ast.objective).toBeDefined();
    expect(ast.objective.summary).toBeDefined();
    expect(ast.constraints).toHaveLength(1);
    expect(ast.constraints[0].id).toBe('C-01');
    expect(ast.acceptance_criteria).toHaveLength(1);
    expect(ast.acceptance_criteria[0].id).toBe('AC-01');
  });

  // @ac AC-02: Spec missing required field 'id' returns ParseError with field path 'spec.id'
  // References: C-01 (schema validation), C-02 (error with field path)
  it('AC-02: returns error with path "spec.id" when id is missing', () => {
    const yaml = readFixture('invalid/missing-id.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    const idError = result.errors.find((e) => e.path.includes('id'));
    expect(idError).toBeDefined();
    expect(idError!.path).toBe('spec.id');
    expect(idError!.type).toBe('required');
  });

  // @ac AC-03: Spec with unknown field returns ParseError identifying the extra field
  // References: C-03 (additionalProperties enforcement)
  it('AC-03: rejects unknown fields with additionalProperties error', () => {
    const yaml = readFixture('invalid/extra-field.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    const extraError = result.errors.find((e) => e.type === 'additionalProperties');
    expect(extraError).toBeDefined();
    expect(extraError!.path).toContain('unknown_field');
  });

  // @ac AC-04: Malformed YAML (bad indentation) returns ParseError with line number
  // References: C-02 (line numbers), C-05 (graceful error handling)
  it('AC-04: handles malformed YAML gracefully with line number', () => {
    const yaml = readFixture('invalid/bad-yaml.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    expect(result.errors.length).toBeGreaterThan(0);
    expect(result.errors[0].type).toBe('yaml_syntax');
    expect(result.errors[0].line).toBeDefined();
    expect(typeof result.errors[0].line).toBe('number');
  });

  // @ac AC-05: Spec with invalid version format returns error
  // References: C-01 (schema validation)
  it('AC-05: rejects invalid version format with pattern error', () => {
    const yaml = readFixture('invalid/bad-version.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    const versionError = result.errors.find((e) => e.path === 'spec.version');
    expect(versionError).toBeDefined();
    expect(versionError!.type).toBe('pattern');
  });

  // @ac AC-06: Spec with all optional fields omitted parses successfully
  // References: C-01 (schema validation), C-04 (typed SpecAST)
  it('AC-06: parses minimal spec with only required fields', () => {
    const yaml = readFixture('valid/minimal.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(true);
    if (!result.ok) return;

    const ast = result.value;
    expect(ast.id).toBe('test-minimal');
    expect(ast.depends_on).toBeUndefined();
    expect(ast.trust_level).toBeUndefined();
    expect(ast.environment).toBeUndefined();
    expect(ast.tags).toBeUndefined();
    expect(ast.changelog).toBeUndefined();
    expect(ast.generated_from).toBeUndefined();
  });

  // @ac AC-07: Spec using YAML anchors and aliases is parsed with anchors resolved
  // References: C-06 (YAML anchors/aliases)
  it('AC-07: resolves YAML anchors and aliases correctly', () => {
    const yaml = readFixture('valid/with-anchors.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(true);
    if (!result.ok) return;

    const ast = result.value;
    // Both constraints should have type and enforcement from the anchor
    expect(ast.constraints[0].type).toBe('technical');
    expect(ast.constraints[0].enforcement).toBe('error');
    expect(ast.constraints[1].type).toBe('technical');
    expect(ast.constraints[1].enforcement).toBe('error');
  });

  // @ac AC-08: Spec with multiple errors returns all errors, not just the first
  // References: C-02 (error reporting), C-07 (collect all errors)
  it('AC-08: collects multiple validation errors', () => {
    const yaml = readFixture('invalid/multiple-errors.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    // Should have at least 2 errors: missing id + invalid version pattern
    expect(result.errors.length).toBeGreaterThanOrEqual(2);
  });

  // @ac AC-09: Invalid constraint ID format returns error
  // References: C-01 (schema validation)
  it('AC-09: rejects invalid constraint ID format', () => {
    const yaml = readFixture('invalid/bad-constraint-id.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    const constraintError = result.errors.find(
      (e) => e.path.includes('constraints') && e.type === 'pattern',
    );
    expect(constraintError).toBeDefined();
  });

  // @ac AC-10: Invalid AC ID format returns error
  // References: C-01 (schema validation)
  it('AC-10: rejects invalid AC ID format', () => {
    const yaml = readFixture('invalid/bad-ac-id.spec.yaml');
    const result = parseSpec(yaml);

    expect(result.ok).toBe(false);
    if (result.ok) return;

    const acError = result.errors.find(
      (e) => e.path.includes('acceptance_criteria') && e.type === 'pattern',
    );
    expect(acError).toBeDefined();
  });

  // Additional structural test: C-08 — pure function, no side effects
  it('C-08: parseSpec is a pure function (no I/O, no side effects)', () => {
    // Calling with the same input should produce the same output
    const yaml = readFixture('valid/simple.spec.yaml');
    const result1 = parseSpec(yaml);
    const result2 = parseSpec(yaml);

    expect(result1).toEqual(result2);
  });
});

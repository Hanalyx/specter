/**
 * Tests for spec-sync: CI pipeline orchestrator.
 *
 * @spec spec-sync
 */

import { describe, it, expect } from 'vitest';
import { runSync, type SyncInput } from '../../../src/core/sync/sync.js';

function validSpecYaml(id: string, tier: number = 2, deps?: string): string {
  const dependsOn = deps
    ? `\n  depends_on:\n    - spec_id: ${deps}\n      relationship: requires`
    : '';
  return `spec:
  id: ${id}
  version: "1.0.0"
  status: approved
  tier: ${tier}

  context:
    system: test

  objective:
    summary: test spec

  constraints:
    - id: C-01
      description: "test constraint"

  acceptance_criteria:
    - id: AC-01
      description: "test ac"
      references_constraints: ["C-01"]${dependsOn}
`;
}

function testFileContent(specId: string, acIds: string[]): string {
  const acAnnotations = acIds.map((id) => `// @ac ${id}`).join('\n');
  return `// @spec ${specId}\n${acAnnotations}\n`;
}

describe('spec-sync', () => {
  // @ac AC-01: Valid specs with full coverage produce exit code 0
  it('AC-01: passes when all phases succeed', () => {
    const input: SyncInput = {
      specFiles: [['a.spec.yaml', validSpecYaml('a')]],
      testFiles: [['a.test.ts', testFileContent('a', ['AC-01'])]],
    };

    const result = runSync(input);

    expect(result.passed).toBe(true);
    expect(result.phases).toHaveLength(4);
    expect(result.phases.every((p) => p.passed)).toBe(true);
    expect(result.stopped_at).toBeUndefined();
  });

  // @ac AC-02: Parse errors stop the pipeline
  it('AC-02: stops at parse phase on invalid YAML', () => {
    const input: SyncInput = {
      specFiles: [['bad.spec.yaml', 'not: valid: yaml: {{{']],
      testFiles: [],
    };

    const result = runSync(input);

    expect(result.passed).toBe(false);
    expect(result.stopped_at).toBe('parse');
    expect(result.phases).toHaveLength(1);
    expect(result.phases[0].passed).toBe(false);
  });

  // @ac AC-03: Dependency errors stop after resolve
  it('AC-03: stops at resolve phase on dangling dependency', () => {
    const specYaml = `spec:
  id: broken
  version: "1.0.0"
  status: approved
  tier: 2

  context:
    system: test

  objective:
    summary: test

  constraints:
    - id: C-01
      description: "test"

  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]

  depends_on:
    - spec_id: nonexistent
      relationship: requires
`;

    const input: SyncInput = {
      specFiles: [['broken.spec.yaml', specYaml]],
      testFiles: [],
    };

    const result = runSync(input);

    expect(result.passed).toBe(false);
    expect(result.stopped_at).toBe('resolve');
    expect(result.phases).toHaveLength(2);
    expect(result.phases[0].passed).toBe(true); // parse OK
    expect(result.phases[1].passed).toBe(false); // resolve fails
  });

  // @ac AC-04: Check errors produce failure
  it('AC-04: fails on check errors (Tier 1 orphan constraint)', () => {
    const specYaml = `spec:
  id: strict
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: test

  objective:
    summary: test

  constraints:
    - id: C-01
      description: "referenced"
    - id: C-02
      description: "orphan - not referenced"

  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
`;

    const input: SyncInput = {
      specFiles: [['strict.spec.yaml', specYaml]],
      testFiles: [['strict.test.ts', testFileContent('strict', ['AC-01'])]],
    };

    const result = runSync(input);

    expect(result.passed).toBe(false);
    expect(result.stopped_at).toBe('check');
  });

  // @ac AC-05: Coverage below threshold produces failure
  it('AC-05: fails when coverage below tier threshold', () => {
    // Tier 1 needs 100% — give it 0%
    const input: SyncInput = {
      specFiles: [['critical.spec.yaml', validSpecYaml('critical', 1)]],
      testFiles: [], // no test files = 0% coverage
    };

    const result = runSync(input);

    expect(result.passed).toBe(false);
    expect(result.stopped_at).toBe('coverage');
    expect(result.phases).toHaveLength(4);
    expect(result.phases[3].passed).toBe(false);
  });

  // Multi-spec pipeline
  it('handles multiple specs with dependencies', () => {
    const input: SyncInput = {
      specFiles: [
        ['a.spec.yaml', validSpecYaml('a')],
        ['b.spec.yaml', validSpecYaml('b', 2, 'a')],
      ],
      testFiles: [
        ['a.test.ts', testFileContent('a', ['AC-01'])],
        ['b.test.ts', testFileContent('b', ['AC-01'])],
      ],
    };

    const result = runSync(input);

    expect(result.passed).toBe(true);
    expect(result.phases).toHaveLength(4);
  });
});

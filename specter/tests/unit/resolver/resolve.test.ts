/**
 * Tests for spec-resolve: dependency graph builder.
 *
 * @spec spec-resolve
 *
 * Every test maps to an acceptance criterion in specs/spec-resolve.spec.yaml.
 */

import { describe, it, expect } from 'vitest';
import { resolveSpecs, type SpecInput } from '../../../src/core/resolver/resolve.js';
import type { SpecAST } from '../../../src/core/schema/types.js';

/** Helper to create a minimal valid SpecAST for testing. */
function makeSpec(overrides: Partial<SpecAST> & { id: string }): SpecAST {
  return {
    version: '1.0.0',
    status: 'approved',
    tier: 2,
    context: { system: 'test' },
    objective: { summary: 'test' },
    constraints: [{ id: 'C-01', description: 'test' }],
    acceptance_criteria: [{ id: 'AC-01', description: 'test' }],
    ...overrides,
  };
}

function makeInput(id: string, overrides?: Partial<SpecAST>): SpecInput {
  return {
    spec: makeSpec({ id, ...overrides }),
    file: `${id}.spec.yaml`,
  };
}

describe('spec-resolve', () => {
  // @ac AC-01: Three specs with valid linear dependencies produce a correct graph
  // References: C-01 (discover specs), C-07 (typed SpecGraph)
  it('AC-01: builds correct graph from linear dependencies with topological order', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', relationship: 'requires' }],
      }),
      makeInput('b', {
        depends_on: [{ spec_id: 'c', relationship: 'requires' }],
      }),
      makeInput('c'),
    ];

    const graph = resolveSpecs(inputs);

    expect(graph.nodes.size).toBe(3);
    expect(graph.edges).toHaveLength(2);
    expect(graph.diagnostics).toHaveLength(0);

    // Topological order: dependencies before dependents
    const order = graph.topological_order;
    expect(order).toHaveLength(3);
    expect(order.indexOf('c')).toBeLessThan(order.indexOf('b'));
    expect(order.indexOf('b')).toBeLessThan(order.indexOf('a'));
  });

  // @ac AC-02: Two specs in a cycle produce a CircularDependency diagnostic
  // References: C-03 (detect all cycles), C-04 (full cycle path)
  it('AC-02: detects two-node circular dependency', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', relationship: 'requires' }],
      }),
      makeInput('b', {
        depends_on: [{ spec_id: 'a', relationship: 'requires' }],
      }),
    ];

    const graph = resolveSpecs(inputs);

    const cycleDiag = graph.diagnostics.find((d) => d.kind === 'circular_dependency');
    expect(cycleDiag).toBeDefined();
    expect(cycleDiag!.cycle_path).toBeDefined();
    expect(cycleDiag!.cycle_path!.length).toBeGreaterThanOrEqual(3); // [a, b, a] or [b, a, b]

    // Topological order should be empty when cycles exist
    expect(graph.topological_order).toHaveLength(0);
  });

  // @ac AC-03: Spec depending on non-existent ID produces DanglingReference diagnostic
  // References: C-05 (detect dangling references)
  it('AC-03: detects dangling reference to non-existent spec', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'nonexistent', relationship: 'requires' }],
      }),
    ];

    const graph = resolveSpecs(inputs);

    const danglingDiag = graph.diagnostics.find((d) => d.kind === 'dangling_reference');
    expect(danglingDiag).toBeDefined();
    expect(danglingDiag!.spec_id).toBe('a');
    expect(danglingDiag!.missing_dep).toBe('nonexistent');
  });

  // @ac AC-04: Spec depending on B@^1.0.0 when B is at 2.0.0 produces VersionMismatch
  // References: C-06 (validate semver ranges)
  it('AC-04: detects semver version mismatch', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', version_range: '^1.0.0', relationship: 'requires' }],
      }),
      makeInput('b', { version: '2.0.0' }),
    ];

    const graph = resolveSpecs(inputs);

    const versionDiag = graph.diagnostics.find((d) => d.kind === 'version_mismatch');
    expect(versionDiag).toBeDefined();
    expect(versionDiag!.spec_id).toBe('a');
    expect(versionDiag!.required_range).toBe('^1.0.0');
    expect(versionDiag!.actual_version).toBe('2.0.0');
  });

  // @ac AC-05: Specs with no depends_on fields produce a graph with no edges
  // References: C-07 (typed SpecGraph)
  it('AC-05: builds graph with no edges when no dependencies', () => {
    const inputs: SpecInput[] = [makeInput('a'), makeInput('b')];

    const graph = resolveSpecs(inputs);

    expect(graph.nodes.size).toBe(2);
    expect(graph.edges).toHaveLength(0);
    expect(graph.diagnostics).toHaveLength(0);
    expect(graph.topological_order).toHaveLength(2);
  });

  // @ac AC-06: Three-node cycle (A->B->C->A) is detected with full path
  // References: C-03 (detect ALL cycles), C-04 (full cycle path)
  it('AC-06: detects three-node circular dependency with full path', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', relationship: 'requires' }],
      }),
      makeInput('b', {
        depends_on: [{ spec_id: 'c', relationship: 'requires' }],
      }),
      makeInput('c', {
        depends_on: [{ spec_id: 'a', relationship: 'requires' }],
      }),
    ];

    const graph = resolveSpecs(inputs);

    const cycleDiags = graph.diagnostics.filter((d) => d.kind === 'circular_dependency');
    expect(cycleDiags.length).toBeGreaterThanOrEqual(1);

    // At least one cycle diagnostic should include all three nodes
    const allCycleNodes = cycleDiags.flatMap((d) => d.cycle_path ?? []);
    expect(allCycleNodes).toContain('a');
    expect(allCycleNodes).toContain('b');
    expect(allCycleNodes).toContain('c');

    expect(graph.topological_order).toHaveLength(0);
  });

  // @ac AC-07: Two duplicate spec IDs produce DuplicateId diagnostic
  // References: C-07 (typed SpecGraph)
  it('AC-07: detects duplicate spec IDs', () => {
    const inputs: SpecInput[] = [
      { spec: makeSpec({ id: 'user-auth' }), file: 'file1.spec.yaml' },
      { spec: makeSpec({ id: 'user-auth' }), file: 'file2.spec.yaml' },
    ];

    const graph = resolveSpecs(inputs);

    const dupDiag = graph.diagnostics.find((d) => d.kind === 'duplicate_id');
    expect(dupDiag).toBeDefined();
    expect(dupDiag!.spec_id).toBe('user-auth');
    expect(dupDiag!.files).toContain('file1.spec.yaml');
    expect(dupDiag!.files).toContain('file2.spec.yaml');

    // Only the first instance should be in the graph
    expect(graph.nodes.size).toBe(1);
  });

  // Additional: version range that IS satisfied should produce no diagnostic
  it('accepts valid semver range dependency', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', version_range: '^1.0.0', relationship: 'requires' }],
      }),
      makeInput('b', { version: '1.2.0' }),
    ];

    const graph = resolveSpecs(inputs);

    expect(graph.diagnostics).toHaveLength(0);
    expect(graph.edges).toHaveLength(1);
  });

  // C-08: Pure function — same input, same output
  it('C-08: resolveSpecs is a pure function', () => {
    const inputs: SpecInput[] = [
      makeInput('a', {
        depends_on: [{ spec_id: 'b', relationship: 'requires' }],
      }),
      makeInput('b'),
    ];

    const result1 = resolveSpecs(inputs);
    const result2 = resolveSpecs(inputs);

    expect(result1.nodes.size).toBe(result2.nodes.size);
    expect(result1.edges).toEqual(result2.edges);
    expect(result1.topological_order).toEqual(result2.topological_order);
    expect(result1.diagnostics).toEqual(result2.diagnostics);
  });
});

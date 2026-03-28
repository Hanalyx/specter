/**
 * Tests for spec-check: the type checker.
 *
 * @spec spec-check
 */

import { describe, it, expect } from 'vitest';
import { checkSpecs } from '../../../src/core/checker/check.js';
import {
  classifyChanges,
  highestClassification,
} from '../../../src/core/checker/rules/breaking-changes.js';
import type { SpecGraph, SpecNode } from '../../../src/core/resolver/types.js';
import type { SpecAST } from '../../../src/core/schema/types.js';

function makeSpec(overrides: Partial<SpecAST> & { id: string }): SpecAST {
  return {
    version: '1.0.0',
    status: 'approved',
    tier: 2,
    context: { system: 'test' },
    objective: { summary: 'test' },
    constraints: [{ id: 'C-01', description: 'test constraint' }],
    acceptance_criteria: [
      { id: 'AC-01', description: 'test ac', references_constraints: ['C-01'] },
    ],
    ...overrides,
  };
}

function makeGraph(
  specs: Array<{ spec: SpecAST; file: string }>,
  edges: SpecGraph['edges'] = [],
): SpecGraph {
  const nodes = new Map<string, SpecNode>();
  for (const s of specs) {
    nodes.set(s.spec.id, s);
  }
  return {
    nodes,
    edges,
    topological_order: specs.map((s) => s.spec.id),
    diagnostics: [],
  };
}

describe('spec-check', () => {
  // @ac AC-01: Spec with constraint C-03 not referenced by any AC produces OrphanConstraint
  // References: C-01
  it('AC-01: detects orphan constraint not referenced by any AC', () => {
    const spec = makeSpec({
      id: 'test-orphan',
      constraints: [
        { id: 'C-01', description: 'referenced' },
        { id: 'C-02', description: 'referenced' },
        { id: 'C-03', description: 'NOT referenced' },
      ],
      acceptance_criteria: [
        { id: 'AC-01', description: 'test', references_constraints: ['C-01'] },
        { id: 'AC-02', description: 'test', references_constraints: ['C-02'] },
      ],
    });

    const graph = makeGraph([{ spec, file: 'test.spec.yaml' }]);
    const result = checkSpecs(graph);

    const orphans = result.diagnostics.filter((d) => d.kind === 'orphan_constraint');
    expect(orphans).toHaveLength(1);
    expect(orphans[0].constraint_id).toBe('C-03');
    expect(orphans[0].spec_id).toBe('test-orphan');
  });

  // @ac AC-02: Tier 1 orphan is error; Tier 3 is info
  // References: C-01, C-02
  it('AC-02: respects tier-based severity for orphan constraints', () => {
    const tier1Spec = makeSpec({
      id: 'tier1-spec',
      tier: 1,
      constraints: [{ id: 'C-01', description: 'orphan' }],
      acceptance_criteria: [{ id: 'AC-01', description: 'test' }],
    });
    const tier3Spec = makeSpec({
      id: 'tier3-spec',
      tier: 3,
      constraints: [{ id: 'C-01', description: 'orphan' }],
      acceptance_criteria: [{ id: 'AC-01', description: 'test' }],
    });

    const graph1 = makeGraph([{ spec: tier1Spec, file: 't1.spec.yaml' }]);
    const graph3 = makeGraph([{ spec: tier3Spec, file: 't3.spec.yaml' }]);

    const result1 = checkSpecs(graph1);
    const result3 = checkSpecs(graph3);

    const orphan1 = result1.diagnostics.find((d) => d.kind === 'orphan_constraint');
    const orphan3 = result3.diagnostics.find((d) => d.kind === 'orphan_constraint');

    expect(orphan1).toBeDefined();
    expect(orphan1!.severity).toBe('error');
    expect(orphan3).toBeDefined();
    expect(orphan3!.severity).toBe('info');
  });

  // @ac AC-03: Structural conflict between upstream required and downstream absent
  // References: C-03
  it('AC-03: detects structural conflict between connected specs', () => {
    const upstream = makeSpec({
      id: 'user-registration',
      constraints: [{ id: 'C-01', description: 'email MUST be required' }],
      acceptance_criteria: [{ id: 'AC-01', description: 'test', references_constraints: ['C-01'] }],
    });
    const downstream = makeSpec({
      id: 'guest-checkout',
      depends_on: [{ spec_id: 'user-registration', relationship: 'requires' }],
      constraints: [{ id: 'C-01', description: 'test' }],
      acceptance_criteria: [
        {
          id: 'AC-01',
          description: 'Process checkout when email is absent',
          references_constraints: ['C-01'],
        },
      ],
    });

    const graph = makeGraph(
      [
        { spec: upstream, file: 'upstream.spec.yaml' },
        { spec: downstream, file: 'downstream.spec.yaml' },
      ],
      [{ from: 'guest-checkout', to: 'user-registration', relationship: 'requires' }],
    );

    const result = checkSpecs(graph);
    const conflicts = result.diagnostics.filter((d) => d.kind === 'structural_conflict');

    expect(conflicts.length).toBeGreaterThanOrEqual(1);
    expect(conflicts[0].severity).toBe('error');
  });

  // @ac AC-04: Removing a constraint is classified as breaking (MAJOR)
  // References: C-04
  it('AC-04: classifies removed constraint as breaking change', () => {
    const v1 = makeSpec({
      id: 'test',
      constraints: [
        { id: 'C-01', description: 'keep' },
        { id: 'C-02', description: 'will be removed' },
      ],
      acceptance_criteria: [
        { id: 'AC-01', description: 'test', references_constraints: ['C-01'] },
        { id: 'AC-02', description: 'test', references_constraints: ['C-02'] },
      ],
    });
    const v2 = makeSpec({
      id: 'test',
      constraints: [{ id: 'C-01', description: 'keep' }],
      acceptance_criteria: [{ id: 'AC-01', description: 'test', references_constraints: ['C-01'] }],
    });

    const changes = classifyChanges(v1, v2);
    expect(highestClassification(changes)).toBe('breaking');

    const removals = changes.filter((c) => c.classification === 'breaking');
    expect(removals.length).toBeGreaterThanOrEqual(1);
  });

  // @ac AC-05: Adding an optional field is classified as additive (MINOR)
  // References: C-04
  it('AC-05: classifies added AC as additive change', () => {
    const v1 = makeSpec({
      id: 'test',
      constraints: [{ id: 'C-01', description: 'test' }],
      acceptance_criteria: [
        { id: 'AC-01', description: 'original', references_constraints: ['C-01'] },
      ],
    });
    const v2 = makeSpec({
      id: 'test',
      constraints: [{ id: 'C-01', description: 'test' }],
      acceptance_criteria: [
        { id: 'AC-01', description: 'original', references_constraints: ['C-01'] },
        { id: 'AC-02', description: 'new addition', references_constraints: ['C-01'] },
      ],
    });

    const changes = classifyChanges(v1, v2);
    expect(highestClassification(changes)).toBe('additive');
  });

  // @ac AC-06: All constraints referenced = zero orphan diagnostics
  // References: C-05
  it('AC-06: produces zero orphan diagnostics when all constraints referenced', () => {
    const spec = makeSpec({
      id: 'fully-covered',
      constraints: [
        { id: 'C-01', description: 'a' },
        { id: 'C-02', description: 'b' },
      ],
      acceptance_criteria: [
        { id: 'AC-01', description: 'test', references_constraints: ['C-01'] },
        { id: 'AC-02', description: 'test', references_constraints: ['C-02'] },
      ],
    });

    const graph = makeGraph([{ spec, file: 'test.spec.yaml' }]);
    const result = checkSpecs(graph);

    const orphans = result.diagnostics.filter((d) => d.kind === 'orphan_constraint');
    expect(orphans).toHaveLength(0);
  });

  // Breaking change detection via checkSpecs with previousVersions
  it('reports breaking changes through checkSpecs orchestrator', () => {
    const current = makeSpec({
      id: 'evolving',
      constraints: [{ id: 'C-01', description: 'kept' }],
      acceptance_criteria: [{ id: 'AC-01', description: 'test', references_constraints: ['C-01'] }],
    });
    const previous = makeSpec({
      id: 'evolving',
      constraints: [
        { id: 'C-01', description: 'kept' },
        { id: 'C-02', description: 'removed' },
      ],
      acceptance_criteria: [
        { id: 'AC-01', description: 'test', references_constraints: ['C-01'] },
        { id: 'AC-02', description: 'test', references_constraints: ['C-02'] },
      ],
    });

    const graph = makeGraph([{ spec: current, file: 'test.spec.yaml' }]);
    const result = checkSpecs(graph, {
      previousVersions: new Map([['evolving', previous]]),
    });

    const breaking = result.diagnostics.filter((d) => d.kind === 'breaking_change');
    expect(breaking.length).toBeGreaterThanOrEqual(1);
    expect(result.summary.errors).toBeGreaterThan(0);
  });
});

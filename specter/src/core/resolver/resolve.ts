/**
 * spec-resolve: Dependency graph builder.
 *
 * Pure function. No CLI deps, no I/O.
 * Takes an array of parsed SpecASTs (with file paths) and builds a directed
 * dependency graph. Detects cycles, dangling references, version mismatches,
 * and duplicate IDs.
 *
 * @spec spec-resolve
 */

import { Graph, alg } from '@dagrejs/graphlib';
import { satisfies, validRange } from 'semver';
import type { SpecAST } from '../schema/types.js';
import type { Diagnostic, SpecEdge, SpecGraph, SpecNode } from './types.js';

export interface SpecInput {
  spec: SpecAST;
  file: string;
}

/**
 * Build a dependency graph from parsed specs.
 *
 * C-07: Produces a typed SpecGraph with nodes, edges, and topological ordering.
 * C-08: Pure function — no I/O, no CLI deps.
 * C-03: Detects ALL circular dependencies.
 * C-04: Reports full cycle paths.
 * C-05: Detects dangling references.
 * C-06: Validates semver ranges.
 *
 * @param inputs - Array of parsed specs with their file paths
 * @returns SpecGraph with nodes, edges, topological order, and diagnostics
 */
export function resolveSpecs(inputs: SpecInput[]): SpecGraph {
  const diagnostics: Diagnostic[] = [];
  const nodes = new Map<string, SpecNode>();
  const edges: SpecEdge[] = [];
  const g = new Graph({ directed: true });

  // Step 1: Register all nodes, detect duplicates (AC-07)
  for (const input of inputs) {
    const id = input.spec.id;

    if (nodes.has(id)) {
      const existing = nodes.get(id)!;
      diagnostics.push({
        kind: 'duplicate_id',
        severity: 'error',
        message: `Duplicate spec ID "${id}" found in ${existing.file} and ${input.file}`,
        spec_id: id,
        files: [existing.file, input.file],
      });
      continue;
    }

    nodes.set(id, { spec: input.spec, file: input.file });
    g.setNode(id);
  }

  // Step 2: Build edges from depends_on, detect dangling refs and version mismatches
  for (const [id, node] of nodes) {
    const deps = node.spec.depends_on;
    if (!deps) continue;

    for (const dep of deps) {
      const targetId = dep.spec_id;

      // C-05: Detect dangling references (AC-03)
      if (!nodes.has(targetId)) {
        diagnostics.push({
          kind: 'dangling_reference',
          severity: 'error',
          message: `Spec "${id}" depends on "${targetId}" which does not exist`,
          spec_id: id,
          missing_dep: targetId,
        });
        continue;
      }

      // C-06: Validate semver range (AC-04)
      if (dep.version_range) {
        const targetVersion = nodes.get(targetId)!.spec.version;
        const range = validRange(dep.version_range);

        if (!range) {
          diagnostics.push({
            kind: 'version_mismatch',
            severity: 'error',
            message: `Spec "${id}" has invalid semver range "${dep.version_range}" for dependency "${targetId}"`,
            spec_id: id,
            required_range: dep.version_range,
            actual_version: targetVersion,
          });
        } else if (!satisfies(targetVersion, dep.version_range)) {
          diagnostics.push({
            kind: 'version_mismatch',
            severity: 'error',
            message: `Spec "${id}" requires "${targetId}@${dep.version_range}" but found version ${targetVersion}`,
            spec_id: id,
            required_range: dep.version_range,
            actual_version: targetVersion,
          });
        }
      }

      // Add edge: id depends on targetId
      edges.push({
        from: id,
        to: targetId,
        version_range: dep.version_range,
        relationship: dep.relationship ?? 'requires',
      });
      g.setEdge(id, targetId);
    }
  }

  // Step 3: Detect circular dependencies (C-03, C-04, AC-02, AC-06)
  const cycles = alg.findCycles(g);
  for (const cycle of cycles) {
    // findCycles returns arrays of node IDs in each cycle
    // Add the first node again to show the full loop
    const cyclePath = [...cycle, cycle[0]];
    diagnostics.push({
      kind: 'circular_dependency',
      severity: 'error',
      message: `Circular dependency detected: ${cyclePath.join(' -> ')}`,
      cycle_path: cyclePath,
    });
  }

  // Step 4: Compute topological order (empty if cycles exist)
  let topologicalOrder: string[] = [];
  if (cycles.length === 0 && alg.isAcyclic(g)) {
    // topsort returns dependents before dependencies, we want dependencies first
    topologicalOrder = alg.topsort(g).reverse();
  }

  return {
    nodes,
    edges,
    topological_order: topologicalOrder,
    diagnostics,
  };
}

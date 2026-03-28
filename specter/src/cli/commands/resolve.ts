/**
 * CLI command: specter resolve
 *
 * @spec spec-resolve
 */

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import chalk from 'chalk';
import { parseSpec } from '../../core/parser/parse.js';
import { resolveSpecs, type SpecInput } from '../../core/resolver/resolve.js';

export interface ResolveCommandOptions {
  json?: boolean;
  dot?: boolean;
}

export function runResolve(options: ResolveCommandOptions): void {
  // C-01: Discover .spec.yaml files recursively
  const specFiles = fg.globSync('**/*.spec.yaml', {
    ignore: ['node_modules/**', 'dist/**', 'tests/fixtures/**'],
  });

  if (specFiles.length === 0) {
    console.log(chalk.yellow('No .spec.yaml files found.'));
    process.exitCode = 1;
    return;
  }

  // Parse all specs
  const inputs: SpecInput[] = [];
  let parseErrors = false;

  for (const file of specFiles) {
    const absPath = resolve(file);
    const content = readFileSync(absPath, 'utf-8');
    const result = parseSpec(content);

    if (result.ok) {
      inputs.push({ spec: result.value, file });
    } else {
      parseErrors = true;
      console.log(chalk.red('PARSE FAIL') + ` ${file}`);
      for (const err of result.errors) {
        console.log(`  ${chalk.red('error')} [${err.type}] ${err.path}: ${err.message}`);
      }
    }
  }

  if (parseErrors) {
    console.log(chalk.red('\nFix parse errors before resolving dependencies.'));
    process.exitCode = 1;
    return;
  }

  // Resolve dependency graph
  const graph = resolveSpecs(inputs);

  if (options.json) {
    console.log(
      JSON.stringify(
        {
          nodes: Array.from(graph.nodes.entries()).map(([id, n]) => ({
            id,
            file: n.file,
            version: n.spec.version,
          })),
          edges: graph.edges,
          topological_order: graph.topological_order,
          diagnostics: graph.diagnostics,
        },
        null,
        2,
      ),
    );
    return;
  }

  if (options.dot) {
    console.log('digraph specs {');
    console.log('  rankdir=BT;');
    for (const [id] of graph.nodes) {
      console.log(`  "${id}";`);
    }
    for (const edge of graph.edges) {
      const label = edge.version_range ? ` [label="${edge.version_range}"]` : '';
      console.log(`  "${edge.from}" -> "${edge.to}"${label};`);
    }
    console.log('}');
    if (graph.diagnostics.length > 0) {
      console.log('');
    }
  }

  // Summary output
  if (!options.dot) {
    console.log(
      chalk.bold(`Spec Graph: ${graph.nodes.size} specs, ${graph.edges.length} dependencies`),
    );
    console.log('');

    if (graph.topological_order.length > 0) {
      console.log(chalk.dim('Resolution order:'));
      for (const id of graph.topological_order) {
        const node = graph.nodes.get(id)!;
        const deps = graph.edges.filter((e) => e.from === id);
        const depStr = deps.length > 0 ? chalk.dim(` -> ${deps.map((d) => d.to).join(', ')}`) : '';
        console.log(`  ${id}@${node.spec.version}${depStr}`);
      }
      console.log('');
    }
  }

  // Print diagnostics
  if (graph.diagnostics.length === 0) {
    console.log(chalk.green('No dependency issues found.'));
  } else {
    for (const diag of graph.diagnostics) {
      const icon = diag.severity === 'error' ? chalk.red('error') : chalk.yellow('warn');
      console.log(`${icon} [${diag.kind}] ${diag.message}`);
    }
    process.exitCode = 1;
  }
}

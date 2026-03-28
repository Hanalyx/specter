/**
 * CLI command: specter check
 *
 * @spec spec-check
 */

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import chalk from 'chalk';
import { parseSpec } from '../../core/parser/parse.js';
import { resolveSpecs, type SpecInput } from '../../core/resolver/resolve.js';
import { checkSpecs } from '../../core/checker/check.js';

export interface CheckCommandOptions {
  json?: boolean;
  tier?: number;
}

export function runCheck(options: CheckCommandOptions): void {
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
    const content = readFileSync(resolve(file), 'utf-8');
    const result = parseSpec(content);
    if (result.ok) {
      inputs.push({ spec: result.value, file });
    } else {
      parseErrors = true;
      console.log(chalk.red('PARSE FAIL') + ` ${file}`);
    }
  }

  if (parseErrors) {
    console.log(chalk.red('\nFix parse errors before checking.'));
    process.exitCode = 1;
    return;
  }

  // Resolve graph
  const graph = resolveSpecs(inputs);
  if (graph.diagnostics.length > 0) {
    for (const d of graph.diagnostics) {
      console.log(chalk.red('error') + ` [${d.kind}] ${d.message}`);
    }
    console.log(chalk.red('\nFix dependency issues before checking.'));
    process.exitCode = 1;
    return;
  }

  // Run checks
  const checkOptions = options.tier ? { tierOverride: options.tier } : {};
  const result = checkSpecs(graph, checkOptions);

  if (options.json) {
    console.log(JSON.stringify(result, null, 2));
    return;
  }

  if (result.diagnostics.length === 0) {
    console.log(chalk.green(`All ${graph.nodes.size} specs passed structural checks.`));
    return;
  }

  for (const d of result.diagnostics) {
    const icon =
      d.severity === 'error'
        ? chalk.red('error')
        : d.severity === 'warning'
          ? chalk.yellow('warn')
          : chalk.dim('info');
    const constraint = d.constraint_id ? ` ${chalk.dim(d.constraint_id)}` : '';
    console.log(`${icon} [${d.kind}] ${d.spec_id}${constraint}: ${d.message}`);
  }

  console.log('');
  console.log(
    `${result.summary.errors} error(s), ${result.summary.warnings} warning(s), ${result.summary.info} info`,
  );

  if (result.summary.errors > 0) {
    process.exitCode = 1;
  }
}

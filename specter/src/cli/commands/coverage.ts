/**
 * CLI command: specter coverage
 *
 * @spec spec-coverage
 */

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import chalk from 'chalk';
import { parseSpec } from '../../core/parser/parse.js';
import { extractAnnotations, buildCoverageReport } from '../../core/coverage/coverage.js';
import type { SpecAST } from '../../core/schema/types.js';

export interface CoverageCommandOptions {
  json?: boolean;
  tests?: string;
}

export function runCoverage(options: CoverageCommandOptions): void {
  const testGlob = options.tests ?? '**/*.test.{ts,js,py}';

  // Discover and parse specs
  const specFiles = fg.globSync('**/*.spec.yaml', {
    ignore: ['node_modules/**', 'dist/**', 'tests/fixtures/**'],
  });

  if (specFiles.length === 0) {
    console.log(chalk.yellow('No .spec.yaml files found.'));
    process.exitCode = 1;
    return;
  }

  const specs: SpecAST[] = [];
  for (const file of specFiles) {
    const content = readFileSync(resolve(file), 'utf-8');
    const result = parseSpec(content);
    if (result.ok) {
      specs.push(result.value);
    }
  }

  // Discover and scan test files
  const testFiles = fg.globSync(testGlob, {
    ignore: ['node_modules/**', 'dist/**'],
  });

  const allAnnotations = [];
  for (const file of testFiles) {
    const content = readFileSync(resolve(file), 'utf-8');
    allAnnotations.push(...extractAnnotations(content, file));
  }

  // Build report
  const report = buildCoverageReport(specs, allAnnotations);

  if (options.json) {
    console.log(JSON.stringify(report, null, 2));
    return;
  }

  // Table output
  console.log(chalk.bold('Spec Coverage Report'));
  console.log('');

  const colWidths = { id: 24, tier: 6, acs: 8, covered: 9, pct: 10, status: 8 };

  // Header
  console.log(
    chalk.dim(
      'Spec ID'.padEnd(colWidths.id) +
        'Tier'.padEnd(colWidths.tier) +
        'ACs'.padEnd(colWidths.acs) +
        'Covered'.padEnd(colWidths.covered) +
        'Coverage'.padEnd(colWidths.pct) +
        'Status',
    ),
  );
  console.log(chalk.dim('-'.repeat(65)));

  for (const entry of report.entries) {
    const pctStr = `${entry.coverage_pct}%`;
    const status = entry.passes_threshold
      ? chalk.green('PASS')
      : entry.coverage_pct === 0
        ? chalk.red('NONE')
        : chalk.red('FAIL');

    const pctColor =
      entry.coverage_pct >= entry.threshold
        ? chalk.green(pctStr)
        : entry.coverage_pct > 0
          ? chalk.yellow(pctStr)
          : chalk.red(pctStr);

    console.log(
      entry.spec_id.padEnd(colWidths.id) +
        `T${entry.tier}`.padEnd(colWidths.tier) +
        String(entry.total_acs).padEnd(colWidths.acs) +
        String(entry.covered_acs.length).padEnd(colWidths.covered) +
        pctColor.padEnd(colWidths.pct + 10) +
        status,
    );

    // Show uncovered ACs
    if (entry.uncovered_acs.length > 0) {
      console.log(chalk.dim(`  uncovered: ${entry.uncovered_acs.join(', ')}`));
    }
  }

  console.log('');
  console.log(
    `${report.summary.total_specs} specs: ` +
      `${chalk.green(String(report.summary.passing) + ' passing')}, ` +
      `${report.summary.failing > 0 ? chalk.red(String(report.summary.failing) + ' failing') : '0 failing'}`,
  );

  if (report.summary.failing > 0) {
    process.exitCode = 1;
  }
}

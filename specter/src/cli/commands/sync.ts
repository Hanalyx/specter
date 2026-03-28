/**
 * CLI command: specter sync
 *
 * Runs the full pipeline: parse -> resolve -> check -> coverage.
 *
 * @spec spec-sync
 */

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import chalk from 'chalk';
import { runSync, type SyncInput } from '../../core/sync/sync.js';

export interface SyncCommandOptions {
  json?: boolean;
  tests?: string;
}

export function runSyncCommand(options: SyncCommandOptions): void {
  const testGlob = options.tests ?? '**/*.test.{ts,js,py}';

  // Discover spec files
  const specFilePaths = fg.globSync('**/*.spec.yaml', {
    ignore: ['node_modules/**', 'dist/**', 'tests/fixtures/**'],
  });

  if (specFilePaths.length === 0) {
    console.log(chalk.yellow('No .spec.yaml files found.'));
    process.exitCode = 1;
    return;
  }

  // Discover test files
  const testFilePaths = fg.globSync(testGlob, {
    ignore: ['node_modules/**', 'dist/**'],
  });

  // Read all files
  const specFiles: Array<[string, string]> = specFilePaths.map((f) => [
    f,
    readFileSync(resolve(f), 'utf-8'),
  ]);
  const testFiles: Array<[string, string]> = testFilePaths.map((f) => [
    f,
    readFileSync(resolve(f), 'utf-8'),
  ]);

  const input: SyncInput = { specFiles, testFiles };
  const result = runSync(input);

  if (options.json) {
    console.log(
      JSON.stringify(
        {
          passed: result.passed,
          phases: result.phases,
          stopped_at: result.stopped_at,
        },
        null,
        2,
      ),
    );
    if (!result.passed) process.exitCode = 1;
    return;
  }

  console.log(chalk.bold('Specter Sync'));
  console.log('');

  for (const phase of result.phases) {
    const icon = phase.passed ? chalk.green('PASS') : chalk.red('FAIL');
    console.log(`  ${icon} ${phase.phase}: ${phase.message}`);
  }

  console.log('');

  if (result.passed) {
    console.log(chalk.green('All checks passed.'));
  } else {
    console.log(chalk.red(`Pipeline failed at ${result.stopped_at} phase.`));
    process.exitCode = 1;
  }
}

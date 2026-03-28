#!/usr/bin/env node

import { Command } from 'commander';
import { runParse } from './cli/commands/parse.js';
import { runResolve } from './cli/commands/resolve.js';
import { runCheck } from './cli/commands/check.js';
import { runCoverage } from './cli/commands/coverage.js';
import { runSyncCommand } from './cli/commands/sync.js';

const program = new Command();

program
  .name('specter')
  .description('A type system for specs. Validates, links, and type-checks .spec.yaml files.')
  .version('0.1.0');

program
  .command('parse')
  .description('Parse and validate .spec.yaml files against the canonical schema')
  .argument('[files...]', 'spec files to parse (defaults to all .spec.yaml in current directory)')
  .option('--json', 'output results as JSON')
  .action((files: string[], options: { json?: boolean }) => {
    runParse(files, options);
  });

program
  .command('resolve')
  .description('Build and validate the spec dependency graph')
  .option('--json', 'output results as JSON')
  .option('--dot', 'output graph in DOT format')
  .action((options: { json?: boolean; dot?: boolean }) => {
    runResolve(options);
  });

program
  .command('check')
  .description('Run type-checking rules across the spec graph')
  .option('--json', 'output results as JSON')
  .option('--tier <tier>', 'override tier enforcement level', parseInt)
  .action((options: { json?: boolean; tier?: number }) => {
    runCheck(options);
  });

program
  .command('coverage')
  .description('Generate spec-to-test traceability matrix')
  .option('--json', 'output results as JSON')
  .option('--tests <glob>', 'glob pattern for test files', '**/*.test.{ts,js,py}')
  .action((options: { json?: boolean; tests?: string }) => {
    runCoverage(options);
  });

program
  .command('sync')
  .description('Run full validation pipeline (parse + resolve + check + coverage)')
  .option('--json', 'output results as JSON')
  .option('--tests <glob>', 'glob pattern for test files', '**/*.test.{ts,js,py}')
  .action((options: { json?: boolean; tests?: string }) => {
    runSyncCommand(options);
  });

program
  .command('init')
  .description('Scaffold a new .spec.yaml file')
  .argument('<name>', 'spec name (kebab-case)')
  .option('--tier <tier>', 'risk tier (1, 2, or 3)', '2')
  .action((_name: string, _options: { tier?: string }) => {
    // TODO: Implement spec scaffolding
    console.log('specter init — not yet implemented');
  });

program.parse();

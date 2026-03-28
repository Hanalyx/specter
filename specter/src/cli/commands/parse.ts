/**
 * CLI command: specter parse
 *
 * @spec spec-parse
 */

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import fg from 'fast-glob';
import chalk from 'chalk';
import { parseSpec } from '../../core/parser/parse.js';

export interface ParseCommandOptions {
  json?: boolean;
}

export function runParse(files: string[], options: ParseCommandOptions): void {
  const specFiles =
    files.length > 0
      ? files.map((f) => resolve(f))
      : fg.globSync('**/*.spec.yaml', { ignore: ['node_modules/**', 'dist/**'] });

  if (specFiles.length === 0) {
    console.log(chalk.yellow('No .spec.yaml files found.'));
    process.exitCode = 1;
    return;
  }

  let hasErrors = false;

  for (const file of specFiles) {
    const content = readFileSync(file, 'utf-8');
    const result = parseSpec(content);

    if (options.json) {
      console.log(JSON.stringify({ file, ...result }, null, 2));
      continue;
    }

    if (result.ok) {
      console.log(chalk.green('PASS') + ` ${file} — ${result.value.id}@${result.value.version}`);
    } else {
      hasErrors = true;
      console.log(chalk.red('FAIL') + ` ${file}`);
      for (const err of result.errors) {
        const location = err.line ? `:${err.line}` : '';
        const path = err.path ? ` ${chalk.dim(err.path)}` : '';
        console.log(`  ${chalk.red('error')} [${err.type}]${path}${location}: ${err.message}`);
      }
    }
  }

  if (hasErrors) {
    process.exitCode = 1;
  }
}

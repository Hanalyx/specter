/**
 * spec-sync: CI pipeline orchestrator.
 *
 * Runs parse -> resolve -> check -> coverage in sequence.
 * Pure function: takes file contents, returns unified result.
 *
 * @spec spec-sync
 */

import { parseSpec } from '../parser/parse.js';
import { resolveSpecs, type SpecInput } from '../resolver/resolve.js';
import { checkSpecs, type CheckResult } from '../checker/check.js';
import { extractAnnotations, buildCoverageReport } from '../coverage/coverage.js';
import type { SpecAST } from '../schema/types.js';
import type { CoverageReport } from '../coverage/types.js';
import type { ParseError } from '../parser/errors.js';
import type { SpecGraph } from '../resolver/types.js';

export type SyncPhase = 'parse' | 'resolve' | 'check' | 'coverage';

export interface SyncPhaseResult {
  phase: SyncPhase;
  passed: boolean;
  message: string;
}

export interface SyncResult {
  passed: boolean;
  phases: SyncPhaseResult[];
  /** Phase where pipeline stopped, if it stopped early */
  stopped_at?: SyncPhase;
  /** Detailed results from each phase */
  parse_errors?: Array<{ file: string; errors: ParseError[] }>;
  graph?: SpecGraph;
  check_result?: CheckResult;
  coverage_report?: CoverageReport;
}

export interface SyncInput {
  /** Spec files: [filepath, content] */
  specFiles: Array<[string, string]>;
  /** Test files: [filepath, content] */
  testFiles: Array<[string, string]>;
}

/**
 * Run the full Specter pipeline.
 *
 * C-01: Runs all four phases in order.
 * C-02: Stops at the first phase with errors.
 * C-03: Returns pass only if all phases pass.
 * C-04: Reports results from all completed phases.
 */
export function runSync(input: SyncInput): SyncResult {
  const phases: SyncPhaseResult[] = [];
  const parseErrors: Array<{ file: string; errors: ParseError[] }> = [];

  // Phase 1: Parse
  const inputs: SpecInput[] = [];
  const specs: SpecAST[] = [];

  for (const [file, content] of input.specFiles) {
    const result = parseSpec(content);
    if (result.ok) {
      inputs.push({ spec: result.value, file });
      specs.push(result.value);
    } else {
      parseErrors.push({ file, errors: result.errors });
    }
  }

  if (parseErrors.length > 0) {
    phases.push({
      phase: 'parse',
      passed: false,
      message: `${parseErrors.length} file(s) failed to parse`,
    });
    return { passed: false, phases, stopped_at: 'parse', parse_errors: parseErrors };
  }

  phases.push({
    phase: 'parse',
    passed: true,
    message: `${inputs.length} spec(s) parsed successfully`,
  });

  // Phase 2: Resolve
  const graph = resolveSpecs(inputs);
  const resolveErrors = graph.diagnostics.filter((d) => d.severity === 'error');

  if (resolveErrors.length > 0) {
    phases.push({
      phase: 'resolve',
      passed: false,
      message: `${resolveErrors.length} dependency error(s)`,
    });
    return { passed: false, phases, stopped_at: 'resolve', graph };
  }

  phases.push({
    phase: 'resolve',
    passed: true,
    message: `${graph.nodes.size} specs, ${graph.edges.length} dependencies resolved`,
  });

  // Phase 3: Check
  const checkResult = checkSpecs(graph);

  if (checkResult.summary.errors > 0) {
    phases.push({
      phase: 'check',
      passed: false,
      message: `${checkResult.summary.errors} error(s), ${checkResult.summary.warnings} warning(s)`,
    });
    return { passed: false, phases, stopped_at: 'check', graph, check_result: checkResult };
  }

  phases.push({
    phase: 'check',
    passed: true,
    message: `${checkResult.summary.warnings} warning(s), ${checkResult.summary.info} info`,
  });

  // Phase 4: Coverage
  const allAnnotations = [];
  for (const [file, content] of input.testFiles) {
    allAnnotations.push(...extractAnnotations(content, file));
  }

  const coverageReport = buildCoverageReport(specs, allAnnotations);

  if (coverageReport.summary.failing > 0) {
    phases.push({
      phase: 'coverage',
      passed: false,
      message: `${coverageReport.summary.failing} spec(s) below coverage threshold`,
    });
    return {
      passed: false,
      phases,
      stopped_at: 'coverage',
      graph,
      check_result: checkResult,
      coverage_report: coverageReport,
    };
  }

  phases.push({
    phase: 'coverage',
    passed: true,
    message: `${coverageReport.summary.passing} spec(s) meet coverage thresholds`,
  });

  return {
    passed: true,
    phases,
    graph,
    check_result: checkResult,
    coverage_report: coverageReport,
  };
}

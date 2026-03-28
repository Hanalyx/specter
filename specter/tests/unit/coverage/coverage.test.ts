/**
 * Tests for spec-coverage: traceability matrix.
 *
 * @spec spec-coverage
 */

import { describe, it, expect } from 'vitest';
import { extractAnnotations, buildCoverageReport } from '../../../src/core/coverage/coverage.js';
import type { SpecAST } from '../../../src/core/schema/types.js';

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

describe('extractAnnotations', () => {
  // @ac AC-01 (partial): @spec and @ac in JS/TS comments
  it('extracts @spec and @ac from JS/TS comments', () => {
    const content = `
// @spec user-auth
describe('user-auth', () => {
  // @ac AC-01
  it('should authenticate', () => {});
  // @ac AC-02
  it('should reject', () => {});
});
`;
    const matches = extractAnnotations(content, 'user-auth.test.ts');
    expect(matches).toHaveLength(1);
    expect(matches[0].spec_id).toBe('user-auth');
    expect(matches[0].ac_ids).toContain('AC-01');
    expect(matches[0].ac_ids).toContain('AC-02');
  });

  // @ac AC-05: Python-style annotations
  it('AC-05: extracts @spec and @ac from Python comments', () => {
    const content = `
# @spec user-auth
class TestUserAuth:
    # @ac AC-01
    def test_authenticate(self):
        pass
`;
    const matches = extractAnnotations(content, 'test_auth.py');
    expect(matches).toHaveLength(1);
    expect(matches[0].spec_id).toBe('user-auth');
    expect(matches[0].ac_ids).toContain('AC-01');
  });

  it('handles files with no annotations', () => {
    const content = `describe('no annotations', () => { it('works', () => {}); });`;
    const matches = extractAnnotations(content, 'test.ts');
    expect(matches).toHaveLength(0);
  });
});

// @spec spec-coverage
describe('buildCoverageReport', () => {
  // @ac AC-01
  // @ac AC-02
  // @ac AC-03
  // @ac AC-04
  // @ac AC-05
  it('AC-01: maps test annotations to specs correctly', () => {
    const specs = [
      makeSpec({
        id: 'user-auth',
        acceptance_criteria: [
          { id: 'AC-01', description: 'a' },
          { id: 'AC-02', description: 'b' },
          { id: 'AC-03', description: 'c' },
        ],
      }),
    ];

    const annotations = [
      { file: 'user-auth.test.ts', spec_id: 'user-auth', ac_ids: ['AC-01', 'AC-02'] },
    ];

    const report = buildCoverageReport(specs, annotations);
    const entry = report.entries[0];

    expect(entry.spec_id).toBe('user-auth');
    expect(entry.covered_acs).toEqual(['AC-01', 'AC-02']);
    expect(entry.uncovered_acs).toEqual(['AC-03']);
    expect(entry.coverage_pct).toBeCloseTo(66.7, 0);
  });

  // @ac AC-02: Spec with no matching test files reports 0% coverage
  it('AC-02: reports 0% for specs with no test coverage', () => {
    const specs = [
      makeSpec({
        id: 'orphan-spec',
        acceptance_criteria: [
          { id: 'AC-01', description: 'a' },
          { id: 'AC-02', description: 'b' },
        ],
      }),
    ];

    const report = buildCoverageReport(specs, []);
    const entry = report.entries[0];

    expect(entry.coverage_pct).toBe(0);
    expect(entry.uncovered_acs).toEqual(['AC-01', 'AC-02']);
    expect(report.summary.uncovered).toBe(1);
  });

  // @ac AC-03: Tier 1 at 80% fails (threshold 100%)
  it('AC-03: flags Tier 1 spec below 100% threshold as failing', () => {
    const specs = [
      makeSpec({
        id: 'payment',
        tier: 1,
        acceptance_criteria: [
          { id: 'AC-01', description: 'a' },
          { id: 'AC-02', description: 'b' },
          { id: 'AC-03', description: 'c' },
          { id: 'AC-04', description: 'd' },
          { id: 'AC-05', description: 'e' },
        ],
      }),
    ];

    const annotations = [
      {
        file: 'payment.test.ts',
        spec_id: 'payment',
        ac_ids: ['AC-01', 'AC-02', 'AC-03', 'AC-04'],
      },
    ];

    const report = buildCoverageReport(specs, annotations);
    const entry = report.entries[0];

    expect(entry.coverage_pct).toBe(80);
    expect(entry.passes_threshold).toBe(false);
    expect(entry.threshold).toBe(100);
  });

  // @ac AC-04: Tier 3 at 60% passes (threshold 50%)
  it('AC-04: Tier 3 spec at 60% passes threshold', () => {
    const specs = [
      makeSpec({
        id: 'utils',
        tier: 3,
        acceptance_criteria: [
          { id: 'AC-01', description: 'a' },
          { id: 'AC-02', description: 'b' },
          { id: 'AC-03', description: 'c' },
          { id: 'AC-04', description: 'd' },
          { id: 'AC-05', description: 'e' },
        ],
      }),
    ];

    const annotations = [
      { file: 'utils.test.ts', spec_id: 'utils', ac_ids: ['AC-01', 'AC-02', 'AC-03'] },
    ];

    const report = buildCoverageReport(specs, annotations);
    const entry = report.entries[0];

    expect(entry.coverage_pct).toBe(60);
    expect(entry.passes_threshold).toBe(true);
    expect(entry.threshold).toBe(50);
  });

  it('summary counts are correct', () => {
    const specs = [
      makeSpec({ id: 'full', tier: 3, acceptance_criteria: [{ id: 'AC-01', description: 'a' }] }),
      makeSpec({ id: 'partial', tier: 3, acceptance_criteria: [{ id: 'AC-01', description: 'a' }, { id: 'AC-02', description: 'b' }] }),
      makeSpec({ id: 'empty', tier: 3, acceptance_criteria: [{ id: 'AC-01', description: 'a' }] }),
    ];

    const annotations = [
      { file: 'full.test.ts', spec_id: 'full', ac_ids: ['AC-01'] },
      { file: 'partial.test.ts', spec_id: 'partial', ac_ids: ['AC-01'] },
    ];

    const report = buildCoverageReport(specs, annotations);
    expect(report.summary.fully_covered).toBe(1);
    expect(report.summary.partially_covered).toBe(1);
    expect(report.summary.uncovered).toBe(1);
  });
});

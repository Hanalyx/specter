/**
 * Types for spec-coverage: traceability matrix.
 *
 * @spec spec-coverage
 */

export interface SpecCoverageEntry {
  spec_id: string;
  tier: number;
  total_acs: number;
  covered_acs: string[];
  uncovered_acs: string[];
  coverage_pct: number;
  threshold: number;
  passes_threshold: boolean;
  test_files: string[];
}

export interface CoverageReport {
  entries: SpecCoverageEntry[];
  summary: {
    total_specs: number;
    fully_covered: number;
    partially_covered: number;
    uncovered: number;
    passing: number;
    failing: number;
  };
}

export interface AnnotationMatch {
  file: string;
  spec_id: string;
  ac_ids: string[];
}

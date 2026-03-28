/**
 * spec-coverage: Scan test files for @spec/@ac annotations and build traceability matrix.
 *
 * Pure function. No CLI deps, no I/O.
 *
 * @spec spec-coverage
 */

import type { SpecAST } from '../schema/types.js';
import { COVERAGE_THRESHOLD_BY_TIER } from '../checker/types.js';
import type { AnnotationMatch, CoverageReport, SpecCoverageEntry } from './types.js';

// C-01: Recognize @spec in //, #, and * (JSDoc) comments
// C-02: Recognize @ac in //, #, and * (JSDoc) comments
const SPEC_ANNOTATION_RE = /(?:\/\/|#|\*)\s*@spec\s+([\w-]+)/g;

/**
 * Extract @spec and @ac annotations from test file content.
 *
 * C-01: Supports // @spec and # @spec
 * C-02: Supports // @ac and # @ac
 */
export function extractAnnotations(fileContent: string, filePath: string): AnnotationMatch[] {
  const matches = new Map<string, Set<string>>();

  // Find all @spec annotations
  for (const match of fileContent.matchAll(SPEC_ANNOTATION_RE)) {
    const specId = match[1];
    if (!matches.has(specId)) {
      matches.set(specId, new Set());
    }
  }

  // Find all @ac annotations and associate with the most recent @spec
  const lines = fileContent.split('\n');
  let currentSpecId: string | null = null;

  for (const line of lines) {
    const specMatch = line.match(/(?:\/\/|#|\*)\s*@spec\s+([\w-]+)/);
    if (specMatch) {
      currentSpecId = specMatch[1];
      if (!matches.has(currentSpecId)) {
        matches.set(currentSpecId, new Set());
      }
    }

    const acMatch = line.match(/(?:\/\/|#|\*)\s*@ac\s+(AC-\d{2,})/);
    if (acMatch && currentSpecId) {
      matches.get(currentSpecId)!.add(acMatch[1]);
    }
  }

  return Array.from(matches.entries()).map(([specId, acIds]) => ({
    file: filePath,
    spec_id: specId,
    ac_ids: Array.from(acIds),
  }));
}

/**
 * Build a coverage report from specs and test file annotations.
 *
 * C-03: Reports coverage as percentage.
 * C-04: Flags specs below tier threshold.
 * C-05: Pure function.
 *
 * @param specs - All parsed specs
 * @param testAnnotations - Annotations extracted from test files
 */
export function buildCoverageReport(
  specs: SpecAST[],
  testAnnotations: AnnotationMatch[],
): CoverageReport {
  // Group annotations by spec ID
  const annotationsBySpec = new Map<string, { ac_ids: Set<string>; files: Set<string> }>();
  for (const ann of testAnnotations) {
    if (!annotationsBySpec.has(ann.spec_id)) {
      annotationsBySpec.set(ann.spec_id, { ac_ids: new Set(), files: new Set() });
    }
    const entry = annotationsBySpec.get(ann.spec_id)!;
    entry.files.add(ann.file);
    for (const acId of ann.ac_ids) {
      entry.ac_ids.add(acId);
    }
  }

  const entries: SpecCoverageEntry[] = [];

  for (const spec of specs) {
    const allAcIds = spec.acceptance_criteria.map((ac) => ac.id);
    const annotation = annotationsBySpec.get(spec.id);
    const coveredAcIds = annotation ? allAcIds.filter((id) => annotation.ac_ids.has(id)) : [];
    const uncoveredAcIds = allAcIds.filter((id) => !coveredAcIds.includes(id));

    const totalAcs = allAcIds.length;
    const coveragePct = totalAcs > 0 ? Math.round((coveredAcIds.length / totalAcs) * 1000) / 10 : 0;
    const threshold = COVERAGE_THRESHOLD_BY_TIER[spec.tier] ?? 80;

    entries.push({
      spec_id: spec.id,
      tier: spec.tier,
      total_acs: totalAcs,
      covered_acs: coveredAcIds,
      uncovered_acs: uncoveredAcIds,
      coverage_pct: coveragePct,
      threshold,
      passes_threshold: coveragePct >= threshold,
      test_files: annotation ? Array.from(annotation.files) : [],
    });
  }

  const summary = {
    total_specs: entries.length,
    fully_covered: entries.filter((e) => e.coverage_pct === 100).length,
    partially_covered: entries.filter((e) => e.coverage_pct > 0 && e.coverage_pct < 100).length,
    uncovered: entries.filter((e) => e.coverage_pct === 0).length,
    passing: entries.filter((e) => e.passes_threshold).length,
    failing: entries.filter((e) => !e.passes_threshold).length,
  };

  return { entries, summary };
}

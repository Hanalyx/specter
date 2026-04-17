// Package coverage implements spec-coverage: traceability matrix.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-coverage
package coverage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Hanalyx/specter/internal/schema"
)

// C-01, C-02: Recognize @spec and @ac in //, #, and * (JSDoc) comments
var specAnnotationRE = regexp.MustCompile(`(?://|#|\*)\s*@spec\s+([\w-]+)`)
var acTagRE = regexp.MustCompile(`(?://|#|\*)\s*@ac\s+(.+)`)
var acIDRE = regexp.MustCompile(`AC-\d{2,}`)

// AnnotationMatch represents annotations found in a test file.
type AnnotationMatch struct {
	File   string   `json:"file"`
	SpecID string   `json:"spec_id"`
	ACIDs  []string `json:"ac_ids"`
}

// SpecCoverageEntry is coverage data for a single spec.
type SpecCoverageEntry struct {
	SpecID          string   `json:"spec_id"`
	Tier            int      `json:"tier"`
	TotalACs        int      `json:"total_acs"`
	CoveredACs      []string `json:"covered_acs"`
	UncoveredACs    []string `json:"uncovered_acs"`
	CoveragePct     float64  `json:"coverage_pct"`
	Threshold       int      `json:"threshold"`
	PassesThreshold bool     `json:"passes_threshold"`
	TestFiles       []string `json:"test_files"`
}

// CoverageReport is the full coverage result.
type CoverageReport struct {
	Entries []SpecCoverageEntry `json:"entries"`
	Summary CoverageSummary     `json:"summary"`
}

type CoverageSummary struct {
	TotalSpecs       int `json:"total_specs"`
	FullyCovered     int `json:"fully_covered"`
	PartiallyCovered int `json:"partially_covered"`
	Uncovered        int `json:"uncovered"`
	Passing          int `json:"passing"`
	Failing          int `json:"failing"`
}

// ExtractAnnotations scans test file content for @spec and @ac annotations.
//
// C-01: Supports // @spec, # @spec, * @spec
// C-02: Supports // @ac, # @ac, * @ac
func ExtractAnnotations(fileContent, filePath string) []AnnotationMatch {
	matchMap := make(map[string]map[string]bool)

	lines := strings.Split(fileContent, "\n")
	var currentSpecID string

	for _, line := range lines {
		// Only process annotations on real comment lines — not inside string literals
		// or at the end of code lines (e.g. content := "// @spec foo" must not match).
		trimmed := strings.TrimSpace(line)
		isCommentLine := strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*")
		if !isCommentLine {
			continue
		}

		if m := specAnnotationRE.FindStringSubmatch(line); len(m) > 1 {
			currentSpecID = m[1]
			if matchMap[currentSpecID] == nil {
				matchMap[currentSpecID] = make(map[string]bool)
			}
		}

		if m := acTagRE.FindStringSubmatch(line); len(m) > 1 && currentSpecID != "" {
			for _, acID := range acIDRE.FindAllString(m[1], -1) {
				matchMap[currentSpecID][acID] = true
			}
		}
	}

	var results []AnnotationMatch
	for specID, acSet := range matchMap {
		var acIDs []string
		for id := range acSet {
			acIDs = append(acIDs, id)
		}
		results = append(results, AnnotationMatch{
			File:   filePath,
			SpecID: specID,
			ACIDs:  acIDs,
		})
	}

	return results
}

// BuildCoverageReport creates a coverage report from specs and test annotations.
//
// C-03: Reports coverage as percentage.
// C-04: Flags specs below tier threshold.
// C-05: Pure function.
func BuildCoverageReport(specs []schema.SpecAST, annotations []AnnotationMatch, thresholds map[int]int) *CoverageReport {
	return BuildCoverageReportWithResults(specs, annotations, thresholds, nil)
}

// BuildCoverageReportWithResults is like BuildCoverageReport but additionally
// enforces pass-rate-aware coverage for Tier 1 specs:
// a Tier 1 AC is covered only if the annotation exists AND the result entry passed.
//
// C-07: Pass-rate-aware coverage for Tier 1
func BuildCoverageReportWithResults(specs []schema.SpecAST, annotations []AnnotationMatch, thresholds map[int]int, results *ResultsFile) *CoverageReport {
	// Group annotations by spec ID
	annotBySpec := make(map[string]struct {
		acIDs map[string]bool
		files map[string]bool
	})
	for _, ann := range annotations {
		entry, ok := annotBySpec[ann.SpecID]
		if !ok {
			entry = struct {
				acIDs map[string]bool
				files map[string]bool
			}{
				acIDs: make(map[string]bool),
				files: make(map[string]bool),
			}
		}
		entry.files[ann.File] = true
		for _, id := range ann.ACIDs {
			entry.acIDs[id] = true
		}
		annotBySpec[ann.SpecID] = entry
	}

	report := &CoverageReport{}

	for _, spec := range specs {
		// Every declared AC counts toward coverage, including gap: true. An
		// unreviewed reverse-compiled spec must fail coverage until a human
		// triages its gaps, otherwise the source-of-truth invariant is broken
		// (a 100%-gap spec would silently pass with zero captured intent).
		var allACIDs []string
		for _, ac := range spec.AcceptanceCriteria {
			allACIDs = append(allACIDs, ac.ID)
		}

		ann := annotBySpec[spec.ID]
		var coveredACs, uncoveredACs []string
		for _, id := range allACIDs {
			annotationExists := ann.acIDs != nil && ann.acIDs[id]
			var isCovered bool
			if spec.Tier == 1 {
				// C-07: Tier 1 requires annotation AND passing result
				isCovered = annotationExists && results.passed(spec.ID, id)
			} else {
				// Tier 2/3: annotation alone is sufficient
				isCovered = annotationExists
			}
			if isCovered {
				coveredACs = append(coveredACs, id)
			} else {
				uncoveredACs = append(uncoveredACs, id)
			}
		}

		totalACs := len(allACIDs)
		var coveragePct float64
		if totalACs > 0 {
			coveragePct = float64(len(coveredACs)) / float64(totalACs) * 100
			// Round to 1 decimal
			coveragePct = float64(int(coveragePct*10)) / 10
		}

		// C-06: Per-spec threshold override
		threshold := thresholds[spec.Tier]
		if threshold == 0 {
			threshold = 80
		}
		if spec.CoverageThreshold > 0 {
			threshold = spec.CoverageThreshold
		}

		var testFiles []string
		if ann.files != nil {
			for f := range ann.files {
				testFiles = append(testFiles, f)
			}
		}

		// Schema requires minItems: 1 for acceptance_criteria, so totalACs
		// should always be >= 1. Guard against empty just in case (defensive,
		// not a supported path).
		passesThreshold := totalACs == 0 || coveragePct >= float64(threshold)

		entry := SpecCoverageEntry{
			SpecID:          spec.ID,
			Tier:            spec.Tier,
			TotalACs:        totalACs,
			CoveredACs:      coveredACs,
			UncoveredACs:    uncoveredACs,
			CoveragePct:     coveragePct,
			Threshold:       threshold,
			PassesThreshold: passesThreshold,
			TestFiles:       testFiles,
		}
		report.Entries = append(report.Entries, entry)

		switch {
		case coveragePct == 100:
			report.Summary.FullyCovered++
		case coveragePct > 0:
			report.Summary.PartiallyCovered++
		default:
			report.Summary.Uncovered++
		}

		if entry.PassesThreshold {
			report.Summary.Passing++
		} else {
			report.Summary.Failing++
		}
	}

	report.Summary.TotalSpecs = len(specs)
	return report
}

// DependencyCoverageWarning is emitted when a spec's dependency has uncovered ACs.
type DependencyCoverageWarning struct {
	Kind         string   // "dependency_coverage"
	Severity     string   // "warning"
	SpecID       string   // the spec that has the failing dependency
	DependsOn    string   // the dependency spec ID
	UncoveredACs []string // uncovered ACs in the dependency
	Message      string
}

// DepEdge represents a directed dependency edge (From depends on To).
type DepEdge struct {
	From string
	To   string
}

// CheckDependencyCoverage checks whether any spec's dependencies have uncovered ACs.
// It takes edges as simple From/To pairs to avoid importing the resolver package.
//
// C-08: dependency_coverage warnings
func CheckDependencyCoverage(edges []DepEdge, report *CoverageReport) []DependencyCoverageWarning {
	// Build lookup map from specID -> coverage entry
	coverageByID := make(map[string]*SpecCoverageEntry)
	for i := range report.Entries {
		coverageByID[report.Entries[i].SpecID] = &report.Entries[i]
	}

	var warnings []DependencyCoverageWarning
	for _, edge := range edges {
		dep, ok := coverageByID[edge.To]
		if !ok || dep.PassesThreshold {
			continue
		}
		warnings = append(warnings, DependencyCoverageWarning{
			Kind:         "dependency_coverage",
			Severity:     "warning",
			SpecID:       edge.From,
			DependsOn:    edge.To,
			UncoveredACs: dep.UncoveredACs,
			Message: fmt.Sprintf(
				"spec %q depends on %q which has %d uncovered AC(s): %s",
				edge.From, edge.To, len(dep.UncoveredACs), strings.Join(dep.UncoveredACs, ", "),
			),
		})
	}
	return warnings
}

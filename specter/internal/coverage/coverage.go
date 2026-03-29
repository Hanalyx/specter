// Package coverage implements spec-coverage: traceability matrix.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-coverage
package coverage

import (
	"regexp"
	"strings"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/schema"
)

// C-01, C-02: Recognize @spec and @ac in //, #, and * (JSDoc) comments
var specAnnotationRE = regexp.MustCompile(`(?://|#|\*)\s*@spec\s+([\w-]+)`)
var acAnnotationRE = regexp.MustCompile(`(?://|#|\*)\s*@ac\s+(AC-\d{2,})`)

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
		if m := specAnnotationRE.FindStringSubmatch(line); len(m) > 1 {
			currentSpecID = m[1]
			if matchMap[currentSpecID] == nil {
				matchMap[currentSpecID] = make(map[string]bool)
			}
		}

		if m := acAnnotationRE.FindStringSubmatch(line); len(m) > 1 && currentSpecID != "" {
			matchMap[currentSpecID][m[1]] = true
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
func BuildCoverageReport(specs []schema.SpecAST, annotations []AnnotationMatch) *CoverageReport {
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
		allACIDs := make([]string, len(spec.AcceptanceCriteria))
		for i, ac := range spec.AcceptanceCriteria {
			allACIDs[i] = ac.ID
		}

		ann := annotBySpec[spec.ID]
		var coveredACs, uncoveredACs []string
		for _, id := range allACIDs {
			if ann.acIDs != nil && ann.acIDs[id] {
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

		threshold := checker.CoverageThresholdByTier[spec.Tier]
		if threshold == 0 {
			threshold = 80
		}

		var testFiles []string
		if ann.files != nil {
			for f := range ann.files {
				testFiles = append(testFiles, f)
			}
		}

		entry := SpecCoverageEntry{
			SpecID:          spec.ID,
			Tier:            spec.Tier,
			TotalACs:        totalACs,
			CoveredACs:      coveredACs,
			UncoveredACs:    uncoveredACs,
			CoveragePct:     coveragePct,
			Threshold:       threshold,
			PassesThreshold: coveragePct >= float64(threshold),
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

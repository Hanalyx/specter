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

// C-01, C-02: Recognize @spec and @ac in //, #, and * (JSDoc) comments.
// Anchored to the start of the trimmed line: an annotation must be the sole
// subject of its comment. Previously, a comment like
//
//	// Mentions "// @spec other-spec" for explanatory purposes
//
// matched and hijacked currentSpecID — caught when spec-coverage's own tests
// described string-literal handling in prose.
var specAnnotationRE = regexp.MustCompile(`^\s*(?://|#|\*)\s*@spec\s+([\w-]+)`)
var acTagRE = regexp.MustCompile(`^\s*(?://|#|\*)\s*@ac\s+(.+)`)
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
	// SpecFile is the path to the .spec.yaml that declared this spec.
	// Populated by the CLI after report construction (the pure builder
	// doesn't have file-path context). Used by downstream consumers that
	// want to open the source file — e.g. the VS Code coverage sidebar's
	// click-to-open on a spec node.
	SpecFile string `json:"spec_file,omitempty"`
}

// CoverageReport is the full coverage result.
type CoverageReport struct {
	Entries     []SpecCoverageEntry `json:"entries"`
	Summary     CoverageSummary     `json:"summary"`
	ParseErrors []ParseErrorEntry   `json:"parse_errors,omitempty"`
	// SpecCandidatesCount is the number of .spec.yaml files discovered on
	// disk before any parse was attempted. When > 0 but len(Entries) == 0,
	// the workspace has specs but none parsed — almost certainly a schema
	// mismatch, not an empty workspace. Used by downstream consumers to
	// give a targeted diagnosis instead of suggesting `specter init`.
	SpecCandidatesCount int `json:"spec_candidates_count"`
	// ParseErrorPatterns groups ParseErrors by (type, path) so downstream
	// consumers can name "all 20 specs are missing `objective`" in one
	// breath instead of 20 individual messages. Sorted by count descending.
	ParseErrorPatterns []ParseErrorPattern `json:"parse_error_patterns,omitempty"`
}

// ParseErrorEntry carries a single parse failure through --json output so
// downstream consumers (VS Code extension, CI, scripts) can render something
// useful even when the specs didn't parse. Shape mirrors parser.ParseError but
// is redeclared here to keep the coverage package free of a parser dependency.
type ParseErrorEntry struct {
	File    string `json:"file"`
	Path    string `json:"path,omitempty"`
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

// ParseErrorPattern is a grouping of parse errors that share the same
// (type, path) signature. When the same missing/invalid field shows up in
// many specs, it's almost always schema drift, not independent typos. The
// pattern surfaces that shape at the CLI layer so consumers don't need to
// re-group.
type ParseErrorPattern struct {
	Type        string   `json:"type"`           // e.g. "required", "enum"
	Path        string   `json:"path,omitempty"` // e.g. "spec.objective"
	Count       int      `json:"count"`          // how many specs hit this
	ExampleFile string   `json:"example_file,omitempty"`
	Files       []string `json:"files,omitempty"`
}

// SummarizeParseErrors groups a flat list of parse errors by (type, path).
// Patterns are returned sorted by count desc so the most widespread issue
// surfaces first. The top pattern plus total-file count is usually enough
// to name schema drift without further analysis.
func SummarizeParseErrors(errs []ParseErrorEntry) []ParseErrorPattern {
	if len(errs) == 0 {
		return nil
	}
	type key struct{ t, p string }
	seen := map[key]*ParseErrorPattern{}
	order := []key{}
	for _, e := range errs {
		k := key{t: e.Type, p: e.Path}
		p, ok := seen[k]
		if !ok {
			p = &ParseErrorPattern{Type: e.Type, Path: e.Path, ExampleFile: e.File}
			seen[k] = p
			order = append(order, k)
		}
		p.Count++
		// Dedupe files within the same pattern; order preserved.
		already := false
		for _, f := range p.Files {
			if f == e.File {
				already = true
				break
			}
		}
		if !already {
			p.Files = append(p.Files, e.File)
		}
	}
	out := make([]ParseErrorPattern, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	// Sort by count desc, stable for ties by first-encountered order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Count > out[j-1].Count; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
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
// C-11 (v1.5.0): Respects multi-line string literals so `// @spec foo`
// appearing inside a TypeScript/JS template literal (backtick), a Go raw
// string (backtick), or a Python triple-quoted string is NOT treated as a
// real annotation. Previously, any line whose trimmed text began with //
// was parsed as a comment, which caused annotation bleed in any test file
// containing an example payload that happened to start with `//`.
func ExtractAnnotations(fileContent, filePath string) []AnnotationMatch {
	matchMap := make(map[string]map[string]bool)

	lines := strings.Split(fileContent, "\n")
	var currentSpecID string

	var inBacktick, inTripleDouble, inTripleSingle bool

	for _, line := range lines {
		lineStartsInString := inBacktick || inTripleDouble || inTripleSingle

		trimmed := strings.TrimSpace(line)
		isCommentLine := !lineStartsInString && (strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*"))

		if isCommentLine {
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
			// Line comments consume the rest of the line in all supported
			// languages (//, #) and we don't flip multi-line string state
			// inside JSDoc (*) blocks — so the string-literal scanner can
			// skip this line entirely.
			continue
		}

		inBacktick, inTripleDouble, inTripleSingle = updateMultilineStringState(
			line, inBacktick, inTripleDouble, inTripleSingle,
		)
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
		// C-14 (v1.7.0): initialize as empty slices so JSON marshals `[]`,
		// not `null`, for entries where one side ends up empty (e.g.
		// 100%-covered spec has zero uncovered ACs). Downstream TS
		// consumers declare these as non-nullable arrays.
		coveredACs := []string{}
		uncoveredACs := []string{}
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

		testFiles := []string{}
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

// updateMultilineStringState walks one non-comment line and reports whether a
// TypeScript/JS/Go backtick template/raw string, a Python triple-double, or a
// Python triple-single string is open at end of line. Single-line strings
// ('...', "...") and Go/TS/JS line comments (//...) are handled locally: they
// cannot cross line boundaries, so we don't propagate their state.
//
// This is a deliberately small scanner — the aim is "don't mis-detect a `//`
// inside a template literal," not full lexical analysis. Edge cases it
// doesn't handle (and doesn't need to for annotation extraction):
//   - JSDoc /* ... */ block comments: the existing extractor already treats
//     lines starting with `*` as comment-like, and block comments don't bleed
//     annotations into string literals.
//   - Template literal interpolations ${...}: strings inside interpolations
//     are treated as independent single-line strings, which is fine.
//   - Escape sequences inside template literals: \` is respected; other
//     escapes are skipped without interpretation.
func updateMultilineStringState(line string, inBacktick, inTripleDouble, inTripleSingle bool) (bool, bool, bool) {
	inSingle := false
	inDouble := false
	n := len(line)
	for i := 0; i < n; {
		// Multi-line string closers take priority — nothing else inside matters.
		if inBacktick {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '`' {
				inBacktick = false
			}
			i++
			continue
		}
		if inTripleDouble {
			if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
				inTripleDouble = false
				i += 3
				continue
			}
			i++
			continue
		}
		if inTripleSingle {
			if i+2 < n && line[i] == '\'' && line[i+1] == '\'' && line[i+2] == '\'' {
				inTripleSingle = false
				i += 3
				continue
			}
			i++
			continue
		}
		// Single-line string closers.
		if inSingle {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '\'' {
				inSingle = false
			}
			i++
			continue
		}
		if inDouble {
			if line[i] == '\\' && i+1 < n {
				i += 2
				continue
			}
			if line[i] == '"' {
				inDouble = false
			}
			i++
			continue
		}
		// Not in any string — check for line-comment and string openers.
		// A `//` line comment consumes the rest of the line; a `#` line
		// comment does the same in Python / shell. JSDoc `*` prefixes only
		// appear on comment lines (handled by the caller).
		if i+1 < n && line[i] == '/' && line[i+1] == '/' {
			return inBacktick, inTripleDouble, inTripleSingle
		}
		if line[i] == '#' {
			return inBacktick, inTripleDouble, inTripleSingle
		}
		// Triple-quote openers before single-quote openers.
		if i+2 < n && line[i] == '"' && line[i+1] == '"' && line[i+2] == '"' {
			inTripleDouble = true
			i += 3
			continue
		}
		if i+2 < n && line[i] == '\'' && line[i+1] == '\'' && line[i+2] == '\'' {
			inTripleSingle = true
			i += 3
			continue
		}
		switch line[i] {
		case '`':
			inBacktick = true
		case '"':
			inDouble = true
		case '\'':
			inSingle = true
		}
		i++
	}
	return inBacktick, inTripleDouble, inTripleSingle
}

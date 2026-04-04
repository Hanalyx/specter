// Package sync implements spec-sync: CI pipeline orchestrator.
//
// Runs parse -> resolve -> check -> coverage in sequence.
//
// @spec spec-sync
package sync

import (
	"fmt"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/coverage"
	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// PhaseResult represents the outcome of one pipeline phase.
type PhaseResult struct {
	Phase   string `json:"phase"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// SyncResult is the unified pipeline result.
type SyncResult struct {
	Passed         bool                    `json:"passed"`
	Phases         []PhaseResult           `json:"phases"`
	StoppedAt      string                  `json:"stopped_at,omitempty"`
	Graph          *resolver.SpecGraph     `json:"graph,omitempty"`
	CheckResult    *checker.CheckResult    `json:"check_result,omitempty"`
	CoverageReport *coverage.CoverageReport `json:"coverage_report,omitempty"`
}

// SyncInput provides spec and test file contents.
type SyncInput struct {
	SpecFiles  []FileContent // [filepath, content]
	TestFiles  []FileContent
	Thresholds map[int]int // optional coverage thresholds by tier; nil uses defaults
}

type FileContent struct {
	Path    string
	Content string
}

// RunSync executes the full pipeline.
//
// C-01: Runs all four phases in order.
// C-02: Stops at first phase with errors.
// C-03: Returns pass only if all pass.
// C-04: Reports results from all completed phases.
func RunSync(input SyncInput) *SyncResult {
	result := &SyncResult{}

	// Phase 1: Parse
	var inputs []resolver.SpecInput
	var specs []schema.SpecAST
	parseFailCount := 0

	for _, f := range input.SpecFiles {
		pr := parser.ParseSpec(f.Content)
		if pr.OK {
			inputs = append(inputs, resolver.SpecInput{Spec: *pr.Value, File: f.Path})
			specs = append(specs, *pr.Value)
		} else {
			parseFailCount++
		}
	}

	if parseFailCount > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "parse", Passed: false,
			Message: fmt.Sprintf("%d file(s) failed to parse", parseFailCount),
		})
		result.StoppedAt = "parse"
		return result
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase: "parse", Passed: true,
		Message: fmt.Sprintf("%d spec(s) parsed successfully", len(inputs)),
	})

	// Phase 2: Resolve
	graph := resolver.ResolveSpecs(inputs)
	result.Graph = graph

	errorCount := 0
	for _, d := range graph.Diagnostics {
		if d.Severity == "error" {
			errorCount++
		}
	}

	if errorCount > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "resolve", Passed: false,
			Message: fmt.Sprintf("%d dependency error(s)", errorCount),
		})
		result.StoppedAt = "resolve"
		return result
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase: "resolve", Passed: true,
		Message: fmt.Sprintf("%d specs, %d dependencies resolved", len(graph.Nodes), len(graph.Edges)),
	})

	// Phase 3: Check
	checkResult := checker.CheckSpecs(graph, nil)
	result.CheckResult = checkResult

	if checkResult.Summary.Errors > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "check", Passed: false,
			Message: fmt.Sprintf("%d error(s), %d warning(s)", checkResult.Summary.Errors, checkResult.Summary.Warnings),
		})
		result.StoppedAt = "check"
		return result
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase: "check", Passed: true,
		Message: fmt.Sprintf("%d warning(s), %d info", checkResult.Summary.Warnings, checkResult.Summary.Info),
	})

	// Phase 4: Coverage
	var allAnnotations []coverage.AnnotationMatch
	for _, f := range input.TestFiles {
		allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(f.Content, f.Path)...)
	}

	thresholds := input.Thresholds
	if thresholds == nil {
		thresholds = checker.CoverageThresholdByTier
	}
	coverageReport := coverage.BuildCoverageReport(specs, allAnnotations, thresholds)
	result.CoverageReport = coverageReport

	if coverageReport.Summary.Failing > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "coverage", Passed: false,
			Message: fmt.Sprintf("%d spec(s) below coverage threshold", coverageReport.Summary.Failing),
		})
		result.StoppedAt = "coverage"
		return result
	}

	result.Phases = append(result.Phases, PhaseResult{
		Phase: "coverage", Passed: true,
		Message: fmt.Sprintf("%d spec(s) meet coverage thresholds", coverageReport.Summary.Passing),
	})

	result.Passed = true
	return result
}

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
	Passed              bool                                 `json:"passed"`
	Phases              []PhaseResult                        `json:"phases"`
	StoppedAt           string                               `json:"stopped_at,omitempty"`
	Graph               *resolver.SpecGraph                  `json:"graph,omitempty"`
	CheckResult         *checker.CheckResult                 `json:"check_result,omitempty"`
	CoverageReport      *coverage.CoverageReport             `json:"coverage_report,omitempty"`
	DepCoverageWarnings []coverage.DependencyCoverageWarning `json:"dep_coverage_warnings,omitempty"`
}

// SyncInput provides spec and test file contents.
type SyncInput struct {
	SpecFiles  []FileContent // [filepath, content]
	TestFiles  []FileContent
	Thresholds map[int]int           // optional coverage thresholds by tier; nil uses defaults
	CheckOpts  *checker.CheckOptions // optional check options (strict, warn_on_draft)
	OnlyPhase  string                // C-05: if set, run prerequisites without halting then run this phase
	Results    *coverage.ResultsFile // optional: pass-rate-aware coverage for Tier 1
}

type FileContent struct {
	Path    string
	Content string
}

// RunSync executes the full pipeline.
//
// C-01: Runs all four phases in order.
// C-02: Stops at first phase with errors (unless OnlyPhase is set).
// C-03: Returns pass only if all pass (or only target phase in OnlyPhase mode).
// C-04: Reports results from all completed phases.
// C-05: OnlyPhase runs prerequisites without halting; exit code is target phase only.
func RunSync(input SyncInput) *SyncResult {
	result := &SyncResult{}
	onlyPhase := input.OnlyPhase

	// haltOnFail: in normal mode always halt; in --only mode only halt at the target phase.
	haltOnFail := func(phase string) bool {
		return onlyPhase == "" || phase == onlyPhase
	}

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
		if haltOnFail("parse") {
			result.StoppedAt = "parse"
			return result
		}
	} else {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "parse", Passed: true,
			Message: fmt.Sprintf("%d spec(s) parsed successfully", len(inputs)),
		})
	}

	if onlyPhase == "parse" {
		result.Passed = parseFailCount == 0
		return result
	}

	// Phase 2: Resolve
	graph := resolver.ResolveSpecs(inputs)
	result.Graph = graph

	resolveErrorCount := 0
	for _, d := range graph.Diagnostics {
		if d.Severity == "error" {
			resolveErrorCount++
		}
	}

	if resolveErrorCount > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "resolve", Passed: false,
			Message: fmt.Sprintf("%d dependency error(s)", resolveErrorCount),
		})
		if haltOnFail("resolve") {
			result.StoppedAt = "resolve"
			return result
		}
	} else {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "resolve", Passed: true,
			Message: fmt.Sprintf("%d specs, %d dependencies resolved", len(graph.Nodes), len(graph.Edges)),
		})
	}

	if onlyPhase == "resolve" {
		result.Passed = resolveErrorCount == 0
		return result
	}

	// Phase 3: Check
	checkResult := checker.CheckSpecs(graph, input.CheckOpts)
	result.CheckResult = checkResult

	if checkResult.Summary.Errors > 0 {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "check", Passed: false,
			Message: fmt.Sprintf("%d error(s), %d warning(s)", checkResult.Summary.Errors, checkResult.Summary.Warnings),
		})
		if haltOnFail("check") {
			result.StoppedAt = "check"
			return result
		}
	} else {
		result.Phases = append(result.Phases, PhaseResult{
			Phase: "check", Passed: true,
			Message: fmt.Sprintf("%d warning(s), %d info", checkResult.Summary.Warnings, checkResult.Summary.Info),
		})
	}

	if onlyPhase == "check" {
		result.Passed = checkResult.Summary.Errors == 0
		return result
	}

	// Phase 4: Coverage
	var allAnnotations []coverage.AnnotationMatch
	for _, f := range input.TestFiles {
		allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(f.Content, f.Path)...)
	}

	thresholds := input.Thresholds
	if thresholds == nil {
		thresholds = checker.CoverageThresholdByTier
	}
	coverageReport := coverage.BuildCoverageReportWithResults(specs, allAnnotations, thresholds, input.Results)
	result.CoverageReport = coverageReport

	// Dependency coverage warnings (C-08)
	var edges []coverage.DepEdge
	for _, e := range graph.Edges {
		edges = append(edges, coverage.DepEdge{From: e.From, To: e.To})
	}
	result.DepCoverageWarnings = coverage.CheckDependencyCoverage(edges, coverageReport)

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

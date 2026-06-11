// Package sync implements spec-sync: CI pipeline orchestrator.
//
// Runs parse -> resolve -> check -> coverage in sequence.
//
// @spec spec-sync
package sync

import (
	"errors"
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

	// spec-sync C-09: zero-tolerance violation counts, mirrored from
	// spec-coverage C-25/C-26. The CLI maps a non-zero
	// ZeroToleranceNonPassed to exit code 2 and a non-zero
	// ApprovalGateViolations to exit code 3, matching `coverage`.
	ZeroToleranceNonPassed int `json:"zero_tolerance_non_passed,omitempty"`
	ApprovalGateViolations int `json:"approval_gate_violations,omitempty"`
}

// SyncInput provides spec and test file contents.
type SyncInput struct {
	SpecFiles            []FileContent // [filepath, content]
	TestFiles            []FileContent
	Thresholds           map[int]int           // optional coverage thresholds by tier; nil uses defaults
	CheckOpts            *checker.CheckOptions // optional check options (strict, warn_on_draft)
	OnlyPhase            string                // C-05: if set, run prerequisites without halting then run this phase
	Results              *coverage.ResultsFile // optional: pass-rate-aware coverage for Tier 1
	CheckTestAnnotations bool                  // spec-check C-09: run CheckTestAnnotations in the check phase (opt-in; `sync --strict` sets this)

	// Strictness routes sync's coverage phase per spec-sync C-06/C-07/C-08.
	// Accepted values: "" (manifest default), "annotation", "threshold",
	// "zero-tolerance". "annotation" preserves the legacy
	// BuildCoverageReportWithResults path. "threshold" and "zero-tolerance"
	// delegate to BuildCoverageReportStrict — matching `coverage --strictness`
	// exactly. The legacy boolean flag from CheckOpts.Strict is treated as
	// "zero-tolerance" when Strictness is not explicitly set.
	//
	// Stub: B 1/3 added the field; B 3/3 (implementation) wires it.
	Strictness string
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

	// spec-check C-09: opt-in test-annotation cross-reference pass.
	if input.CheckTestAnnotations {
		contents := make(map[string]string, len(input.TestFiles))
		for _, f := range input.TestFiles {
			contents[f.Path] = f.Content
		}
		taDiags := checker.CheckTestAnnotations(contents, specs)
		checkResult.Diagnostics = append(checkResult.Diagnostics, taDiags...)
		for _, d := range taDiags {
			switch d.Severity {
			case "error":
				checkResult.Summary.Errors++
			case "warning":
				checkResult.Summary.Warnings++
			case "info":
				checkResult.Summary.Info++
			}
		}
	}

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

	// spec-sync C-06/C-07: route the coverage phase through
	// BuildCoverageReportStrict under any strict mode (threshold or
	// zero-tolerance — which includes the manifest default threshold,
	// per spec-manifest C-24). Annotation mode preserves the legacy
	// BuildCoverageReportWithResults path.
	effectiveStrictness := input.Strictness
	if effectiveStrictness == "" && input.CheckOpts != nil && input.CheckOpts.Strict {
		// Legacy: --strict boolean without --strictness → zero-tolerance.
		effectiveStrictness = "zero-tolerance"
	}
	useStrictPath := effectiveStrictness == "threshold" || effectiveStrictness == "zero-tolerance"

	var coverageReport *coverage.CoverageReport
	if useStrictPath {
		report, strictErr := coverage.BuildCoverageReportStrict(specs, allAnnotations, thresholds, input.Results, true, nil)
		if strictErr != nil {
			// C-08: missing .specter-results.json under strict mode
			// fails the coverage phase. Surface a sync-specific message
			// that names the active strictness mode and offers both
			// remedies. Unlike `coverage --strict`, sync's strict mode
			// usually comes from the manifest default rather than an
			// explicit flag, so the message MUST NOT attribute the
			// requirement to `--strict` (coverage.ErrMissingResults's
			// wording, correct only where the operator passed --strict).
			msg := strictErr.Error()
			if errors.Is(strictErr, coverage.ErrMissingResults) {
				msg = fmt.Sprintf("strictness %q requires .specter-results.json — run 'specter ingest' first, or use --strictness annotation for structural coverage", effectiveStrictness)
			}
			result.Phases = append(result.Phases, PhaseResult{
				Phase:   "coverage",
				Passed:  false,
				Message: msg,
			})
			result.StoppedAt = "coverage"
			return result
		}
		coverageReport = report
	} else {
		coverageReport = coverage.BuildCoverageReportWithResults(specs, allAnnotations, thresholds, input.Results)
	}
	result.CoverageReport = coverageReport

	// spec-sync C-09 / spec-coverage GH #94: under zero-tolerance the
	// report demotes approval_gate violations so the report and the
	// exit signal agree — same demotion `coverage` applies.
	if effectiveStrictness == "zero-tolerance" {
		coverage.DemoteApprovalGateViolations(coverageReport, specs)
	}

	// Dependency coverage warnings (C-08 — spec-coverage, not spec-sync)
	var edges []coverage.DepEdge
	for _, e := range graph.Edges {
		edges = append(edges, coverage.DepEdge{From: e.From, To: e.To})
	}
	result.DepCoverageWarnings = coverage.CheckDependencyCoverage(edges, coverageReport)

	// spec-sync C-09: zero-tolerance enforces the same two gates as
	// `coverage` (spec-coverage C-25/C-26), in the same order, BEFORE
	// the tier-threshold check — a failing annotated AC fails the run
	// even when the demoted coverage still clears the tier threshold
	// (the pre-1.4.0 false-green: one passing + one failing AC on a
	// Tier 3 spec passed at 50%).
	if effectiveStrictness == "zero-tolerance" {
		if nonPassed := coverage.CountNonPassed(input.Results); nonPassed > 0 {
			result.ZeroToleranceNonPassed = nonPassed
			result.Phases = append(result.Phases, PhaseResult{
				Phase: "coverage", Passed: false,
				Message: fmt.Sprintf("zero-tolerance strictness — %d annotated AC(s) did not pass", nonPassed),
			})
			result.StoppedAt = "coverage"
			return result
		}
		if gateViolations := coverage.CountApprovalGateViolations(specs); gateViolations > 0 {
			result.ApprovalGateViolations = gateViolations
			result.Phases = append(result.Phases, PhaseResult{
				Phase: "coverage", Passed: false,
				Message: fmt.Sprintf("zero-tolerance strictness — %d AC(s) carry approval_gate=true with unset approval_date", gateViolations),
			})
			result.StoppedAt = "coverage"
			return result
		}
	}

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

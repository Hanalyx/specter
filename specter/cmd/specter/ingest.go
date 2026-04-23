// ingest.go — `specter ingest` CLI. Thin I/O wrapper around
// internal/ingest parsers. Reads a runner's output file, writes
// .specter-results.json for `specter coverage --strict` to consume.
//
// @spec spec-ingest
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hanalyx/specter/internal/ingest"
	"github.com/spf13/cobra"
)

func ingestCmd() *cobra.Command {
	var junitPaths []string
	var goTestPaths []string
	var outputPath string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Convert CI test results (JUnit XML, go test -json) into .specter-results.json",
		Long: `Consumes a test runner's output and writes .specter-results.json that
specter coverage --strict reads to determine pass/fail per AC.

Flavors:
  --junit <path>      JUnit XML (vitest, jest, pytest, playwright)
  --go-test <path>    go test -json newline-delimited output

Both flags accept glob patterns and can be repeated. All matched files
are merged into one output via the worst-status-wins rule.

Diagnostics:
  Emits a summary line to stderr on every run:
    Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation.

  --verbose adds a per-case drop reason for each dropped testcase so
  operators can see which tests need migration to Convention A/B.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(junitPaths) == 0 && len(goTestPaths) == 0 {
				fmt.Fprintln(os.Stderr, "error: at least one of --junit or --go-test is required")
				return errSilent
			}
			if outputPath == "" {
				outputPath = ".specter-results.json"
			}

			// C-11: expand each --junit / --go-test entry as a glob. A literal
			// path with no wildcard passes through. A pattern with no matches
			// is a hard failure — silently producing an empty result would
			// hide an operator typo.
			junitFiles, err := expandPaths(junitPaths, "--junit")
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return errSilent
			}
			goTestFiles, err := expandPaths(goTestPaths, "--go-test")
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return errSilent
			}

			var results []ingest.TestResult
			var totalScanned int
			var totalDropped []string

			for _, p := range junitFiles {
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", p, readErr)
					return errSilent
				}
				jResults, jScanned, jDropped, err := ingest.ParseJUnitStats(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse junit %s: %v\n", p, err)
					return errSilent
				}
				results = append(results, jResults...)
				totalScanned += jScanned
				totalDropped = append(totalDropped, jDropped...)
			}

			for _, p := range goTestFiles {
				data, readErr := os.ReadFile(p)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", p, readErr)
					return errSilent
				}
				gResults, gScanned, gDropped, err := ingest.ParseGoTestStats(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse go-test %s: %v\n", p, err)
					return errSilent
				}
				results = append(results, gResults...)
				totalScanned += gScanned
				totalDropped = append(totalDropped, gDropped...)
			}

			if err := ingest.WriteResultsFile(outputPath, results); err != nil {
				fmt.Fprintf(os.Stderr, "error: write %s: %v\n", outputPath, err)
				return errSilent
			}

			// C-09: default summary on stderr. Extracted count reflects pairs
			// after worst-status-wins merging (C-08).
			extracted := len(ingest.MergeResults(results))
			fmt.Fprintf(os.Stderr, "Scanned %d test cases; extracted %d (spec_id, ac_id) pairs; dropped %d with no runner-visible annotation.\n",
				totalScanned, extracted, len(totalDropped))

			if verbose {
				for _, name := range totalDropped {
					fmt.Fprintf(os.Stderr, "  dropped: %s — no (spec_id, ac_id) pair found in name, classname, or output\n", name)
				}
			}

			fmt.Printf("Wrote %d result entries to %s\n", extracted, outputPath)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&junitPaths, "junit", nil, "Path to JUnit XML file. Accepts glob patterns; may be repeated.")
	cmd.Flags().StringArrayVar(&goTestPaths, "go-test", nil, "Path to go test -json output file. Accepts glob patterns; may be repeated.")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path (default: .specter-results.json)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Emit one line per dropped testcase (testcases without a (spec_id, ac_id) annotation)")
	return cmd
}

// expandPaths resolves each input as a glob pattern. Literal paths (no
// wildcards) pass through. Patterns with no matches are a hard error —
// silent empty results would hide operator typos. C-11.
func expandPaths(inputs []string, flagName string) ([]string, error) {
	var out []string
	for _, in := range inputs {
		if !hasGlobMeta(in) {
			out = append(out, in)
			continue
		}
		matches, err := filepath.Glob(in)
		if err != nil {
			return nil, fmt.Errorf("%s %q: bad pattern: %w", flagName, in, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s %q: no files matched", flagName, in)
		}
		out = append(out, matches...)
	}
	return out, nil
}

func hasGlobMeta(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

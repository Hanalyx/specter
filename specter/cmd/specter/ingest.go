// ingest.go — `specter ingest` CLI. Thin I/O wrapper around
// internal/ingest parsers. Reads a runner's output file, writes
// .specter-results.json for `specter coverage --strict` to consume.
//
// @spec spec-ingest
package main

import (
	"fmt"
	"os"

	"github.com/Hanalyx/specter/internal/ingest"
	"github.com/spf13/cobra"
)

func ingestCmd() *cobra.Command {
	var junitPath string
	var goTestPath string
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

Diagnostics:
  Emits a summary line to stderr on every run:
    Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation.

  --verbose adds a per-case drop reason for each dropped testcase so
  operators can see which tests need migration to Convention A/B.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if junitPath == "" && goTestPath == "" {
				fmt.Fprintln(os.Stderr, "error: one of --junit or --go-test is required")
				return errSilent
			}
			if outputPath == "" {
				outputPath = ".specter-results.json"
			}

			var results []ingest.TestResult
			var totalScanned int
			var totalDropped []string

			if junitPath != "" {
				data, readErr := os.ReadFile(junitPath)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", junitPath, readErr)
					return errSilent
				}
				jResults, jScanned, jDropped, err := ingest.ParseJUnitStats(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse junit: %v\n", err)
					return errSilent
				}
				results = append(results, jResults...)
				totalScanned += jScanned
				totalDropped = append(totalDropped, jDropped...)
			}

			if goTestPath != "" {
				data, readErr := os.ReadFile(goTestPath)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", goTestPath, readErr)
					return errSilent
				}
				gResults, gScanned, gDropped, err := ingest.ParseGoTestStats(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse go-test: %v\n", err)
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

			// C-09: default summary on stderr. Extracted count uses the
			// post-merge length so it reflects (spec, AC) PAIRS, not raw
			// testcases — worst-status-wins merging can collapse duplicates.
			extracted := len(ingest.MergeResults(results))
			fmt.Fprintf(os.Stderr, "Scanned %d test cases; extracted %d (spec_id, ac_id) pairs; dropped %d with no runner-visible annotation.\n",
				totalScanned, extracted, len(totalDropped))

			// C-10: --verbose emits per-case drop reasons in input order.
			if verbose {
				for _, name := range totalDropped {
					fmt.Fprintf(os.Stderr, "  dropped: %s — no (spec_id, ac_id) pair found in name, classname, or output\n", name)
				}
			}

			fmt.Printf("Wrote %d result entries to %s\n", extracted, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&junitPath, "junit", "", "Path to JUnit XML file")
	cmd.Flags().StringVar(&goTestPath, "go-test", "", "Path to go test -json output file")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path (default: .specter-results.json)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Emit one line per dropped testcase (testcases without a (spec_id, ac_id) annotation)")
	return cmd
}

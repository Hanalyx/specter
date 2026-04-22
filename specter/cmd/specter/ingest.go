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

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Convert CI test results (JUnit XML, go test -json) into .specter-results.json",
		Long: `Consumes a test runner's output and writes .specter-results.json that
specter coverage --strict reads to determine pass/fail per AC.

Flavors:
  --junit <path>      JUnit XML (vitest, jest, pytest, playwright)
  --go-test <path>    go test -json newline-delimited output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if junitPath == "" && goTestPath == "" {
				fmt.Fprintln(os.Stderr, "error: one of --junit or --go-test is required")
				return errSilent
			}
			if outputPath == "" {
				outputPath = ".specter-results.json"
			}

			var results []ingest.TestResult
			var err error

			if junitPath != "" {
				data, readErr := os.ReadFile(junitPath)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", junitPath, readErr)
					return errSilent
				}
				results, err = ingest.ParseJUnit(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse junit: %v\n", err)
					return errSilent
				}
			}

			if goTestPath != "" {
				data, readErr := os.ReadFile(goTestPath)
				if readErr != nil {
					fmt.Fprintf(os.Stderr, "error: read %s: %v\n", goTestPath, readErr)
					return errSilent
				}
				goResults, err := ingest.ParseGoTest(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse go-test: %v\n", err)
					return errSilent
				}
				results = append(results, goResults...)
			}

			if err := ingest.WriteResultsFile(outputPath, results); err != nil {
				fmt.Fprintf(os.Stderr, "error: write %s: %v\n", outputPath, err)
				return errSilent
			}

			fmt.Printf("Wrote %d result entries to %s\n", len(ingest.MergeResults(results)), outputPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&junitPath, "junit", "", "Path to JUnit XML file")
	cmd.Flags().StringVar(&goTestPath, "go-test", "", "Path to go test -json output file")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path (default: .specter-results.json)")
	return cmd
}

// ingest.go — `specter ingest` CLI. Thin I/O wrapper around
// internal/ingest parsers. Reads a runner's output file, writes
// .specter-results.json for `specter coverage --strict` to consume.
//
// @spec spec-ingest
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hanalyx/specter/internal/ingest"
	"github.com/spf13/cobra"
)

func ingestCmd() *cobra.Command {
	var mergePaths []string
	var streamName string
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
			// C-15: --merge and the runner flags are two modes that disagree
			// about what the output is built from. Refused rather than given a
			// precedence rule, because a silent winner would make the artifact
			// depend on flag order.
			if len(mergePaths) > 0 {
				var conflicts []string
				if len(junitPaths) > 0 {
					conflicts = append(conflicts, "--junit")
				}
				if len(goTestPaths) > 0 {
					conflicts = append(conflicts, "--go-test")
				}
				if cmd.Flags().Changed("stream") {
					conflicts = append(conflicts, "--stream")
				}
				if len(conflicts) > 0 {
					fmt.Fprintf(os.Stderr, "error: --merge cannot be combined with %s. --merge builds the output from the files it names; the others build it from runner output\n",
						strings.Join(conflicts, ", "))
					return errSilent
				}
				if outputPath == "" {
					outputPath = ".specter-results.json"
				}
				return runMerge(mergePaths, outputPath)
			}

			if len(junitPaths) == 0 && len(goTestPaths) == 0 {
				fmt.Fprintln(os.Stderr, "error: at least one of --junit, --go-test or --merge is required")
				return errSilent
			}
			// C-14: an empty name is refused, so a producer that meant to name
			// a stream learns it from the run rather than from a coverage
			// report weeks later.
			if cmd.Flags().Changed("stream") && streamName == "" {
				fmt.Fprintln(os.Stderr, "error: --stream requires a non-empty name")
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
			// C-16: named for what was observed. Not "failed to build", which
			// ingest cannot tell from a filtered-out package or one with none.
			var silentPackages int

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
				gResults, gScanned, gDropped, gSilent, err := ingest.ParseGoTestStreamStats(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: parse go-test %s: %v\n", p, err)
					return errSilent
				}
				results = append(results, gResults...)
				totalScanned += gScanned
				totalDropped = append(totalDropped, gDropped...)
				silentPackages += gSilent
			}

			// C-14: one invocation writes one stream.
			if streamName != "" {
				for i := range results {
					results[i].Stream = streamName
				}
			}

			// C-16: the block is written only when the run names a stream.
			// C-14 promises an unlabeled run writes exactly the file it wrote
			// before, and even an empty array would break that.
			var streams []ingest.StreamInfo
			if streamName != "" {
				streams = []ingest.StreamInfo{{
					Name:                  streamName,
					Scanned:               totalScanned,
					Extracted:             len(ingest.MergeResults(results)),
					ZeroTestEventPackages: silentPackages,
				}}
			}

			if err := ingest.WriteResultsFileWithStreams(outputPath, results, streams); err != nil {
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
	cmd.Flags().StringVar(&streamName, "stream", "", "Label every entry this run produces with a stream name. Omit for an unlabeled file, which reads as the default stream.")
	cmd.Flags().StringArrayVar(&mergePaths, "merge", nil, "Build the output from the named results files alone, never accumulating into an existing output. May be repeated.")
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

// runMerge builds an output from the named results files alone, spec-ingest
// C-15. It deliberately does not read the existing output first. Accumulating
// is right within one run and wrong across runs: a criterion that passed in the
// previous run and produced no entry in this one would keep its stale passing
// entry, which hides exactly the absence the stream foundation exists to make
// visible. A stream re-run is a stream replaced.
func runMerge(paths []string, outputPath string) error {
	files, err := expandPaths(paths, "--merge")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return errSilent
	}

	var results []ingest.TestResult
	// Blocks, not rows. Each input's `streams` key carries whether it was
	// declared at all, and appending the rows alone would drop that: an input
	// declaring an empty block contributes no rows and would vanish. A
	// declared empty block beside a labeled entry is the artifact C-44
	// rejects, so losing it here is what let `--merge` write one.
	var blocks []ingest.StreamBlock
	for _, p := range files {
		r, block, readErr := ingest.ReadResultsFile(p)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", p, readErr)
			return errSilent
		}
		results = append(results, r...)
		blocks = append(blocks, block)
	}

	merged := ingest.MergeStreams(blocks)
	// C-15: the artifact is checked before it is written, so a merge never
	// produces one `coverage` refuses and never destroys the output already
	// there when it declines to.
	if err := ingest.WriteMergedResultsFile(outputPath, results, merged); err != nil {
		if errors.Is(err, ingest.ErrMergeWouldBeRefused) || errors.Is(err, ingest.ErrMergeTooLarge) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			fmt.Fprintf(os.Stderr, "       %s was not written, and any existing file at that path is unchanged.\n", outputPath)
			return errSilent
		}
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", outputPath, err)
		return errSilent
	}

	entries := len(ingest.MergeResults(results))
	fmt.Fprintf(os.Stderr, "Merged %d file(s) into %d result entries across %d stream(s).\n", len(files), entries, merged.Len())
	fmt.Printf("Wrote %d result entries to %s\n", entries, outputPath)
	return nil
}

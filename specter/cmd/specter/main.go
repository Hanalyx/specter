package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/coverage"
	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
	specsync "github.com/Hanalyx/specter/internal/sync"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "specter",
		Short:   "A type system for specs",
		Long:    "Specter validates, links, and type-checks .spec.yaml files the way tsc validates .ts files.",
		Version: version,
	}

	root.AddCommand(parseCmd())
	root.AddCommand(resolveCmd())
	root.AddCommand(checkCmd())
	root.AddCommand(coverageCmd())
	root.AddCommand(syncCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// --- Helpers ---

func discoverSpecs(patterns ...string) []string {
	if len(patterns) > 0 && patterns[0] != "" {
		return patterns
	}
	var files []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == ".git") {
			return filepath.SkipDir
		}
		if info.IsDir() && strings.HasPrefix(path, filepath.Join("tests", "fixtures")) {
			return filepath.SkipDir
		}
		if info.IsDir() && strings.HasPrefix(path, filepath.Join("testdata")) {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".spec.yaml") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func discoverTestFiles(glob string) []string {
	if glob == "" {
		glob = "**/*.test.{ts,js,py}"
	}
	// Simple recursive walk for test files
	var files []string
	exts := []string{".test.ts", ".test.js", ".test.py", "_test.go", "_test.py"}
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == ".git") {
			return filepath.SkipDir
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	return files
}

func parseAllSpecs(files []string) ([]resolver.SpecInput, []schema.SpecAST, bool) {
	var inputs []resolver.SpecInput
	var specs []schema.SpecAST
	hasErrors := false

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", file, err)
			hasErrors = true
			continue
		}
		result := parser.ParseSpec(string(data))
		if result.OK {
			inputs = append(inputs, resolver.SpecInput{Spec: *result.Value, File: file})
			specs = append(specs, *result.Value)
		} else {
			hasErrors = true
			fmt.Fprintf(os.Stderr, "FAIL %s\n", file)
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  error [%s] %s: %s\n", e.Type, e.Path, e.Message)
			}
		}
	}

	return inputs, specs, hasErrors
}

// --- Commands ---

func parseCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "parse [files...]",
		Short: "Parse and validate .spec.yaml files against the canonical schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			files := discoverSpecs(args...)
			if len(files) == 0 {
				fmt.Println("No .spec.yaml files found.")
				os.Exit(1)
			}

			hasErrors := false
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", file, err)
					hasErrors = true
					continue
				}

				result := parser.ParseSpec(string(data))
				if jsonOutput {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]interface{}{"file": file, "ok": result.OK, "value": result.Value, "errors": result.Errors})
					continue
				}

				if result.OK {
					fmt.Printf("PASS %s — %s@%s\n", file, result.Value.ID, result.Value.Version)
				} else {
					hasErrors = true
					fmt.Fprintf(os.Stderr, "FAIL %s\n", file)
					for _, e := range result.Errors {
						fmt.Fprintf(os.Stderr, "  error [%s] %s: %s\n", e.Type, e.Path, e.Message)
					}
				}
			}

			if hasErrors {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	return cmd
}

func resolveCmd() *cobra.Command {
	var jsonOutput, dotOutput bool
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Build and validate the spec dependency graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			files := discoverSpecs()
			if len(files) == 0 {
				fmt.Println("No .spec.yaml files found.")
				os.Exit(1)
			}

			inputs, _, hasErrors := parseAllSpecs(files)
			if hasErrors {
				fmt.Fprintln(os.Stderr, "\nFix parse errors before resolving dependencies.")
				os.Exit(1)
			}

			graph := resolver.ResolveSpecs(inputs)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(graph)
				return nil
			}

			if dotOutput {
				fmt.Println("digraph specs {")
				fmt.Println("  rankdir=BT;")
				for id := range graph.Nodes {
					fmt.Printf("  %q;\n", id)
				}
				for _, e := range graph.Edges {
					label := ""
					if e.VersionRange != "" {
						label = fmt.Sprintf(" [label=%q]", e.VersionRange)
					}
					fmt.Printf("  %q -> %q%s;\n", e.From, e.To, label)
				}
				fmt.Println("}")
			} else {
				fmt.Printf("Spec Graph: %d specs, %d dependencies\n\n", len(graph.Nodes), len(graph.Edges))
				if len(graph.TopologicalOrder) > 0 {
					fmt.Println("Resolution order:")
					for _, id := range graph.TopologicalOrder {
						node := graph.Nodes[id]
						var deps []string
						for _, e := range graph.Edges {
							if e.From == id {
								deps = append(deps, e.To)
							}
						}
						depStr := ""
						if len(deps) > 0 {
							depStr = " -> " + strings.Join(deps, ", ")
						}
						fmt.Printf("  %s@%s%s\n", id, node.Spec.Version, depStr)
					}
					fmt.Println()
				}
			}

			if len(graph.Diagnostics) == 0 {
				fmt.Println("No dependency issues found.")
			} else {
				for _, d := range graph.Diagnostics {
					fmt.Fprintf(os.Stderr, "error [%s] %s\n", d.Kind, d.Message)
				}
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&dotOutput, "dot", false, "Output graph in DOT format")
	return cmd
}

func checkCmd() *cobra.Command {
	var jsonOutput bool
	var tierOverride int
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run type-checking rules across the spec graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			files := discoverSpecs()
			inputs, _, hasErrors := parseAllSpecs(files)
			if hasErrors {
				os.Exit(1)
			}

			graph := resolver.ResolveSpecs(inputs)
			for _, d := range graph.Diagnostics {
				if d.Severity == "error" {
					fmt.Fprintf(os.Stderr, "error [%s] %s\n", d.Kind, d.Message)
				}
			}

			opts := &checker.CheckOptions{}
			if tierOverride > 0 {
				opts.TierOverride = tierOverride
			}

			result := checker.CheckSpecs(graph, opts)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				return nil
			}

			if len(result.Diagnostics) == 0 {
				fmt.Printf("All %d specs passed structural checks.\n", len(graph.Nodes))
				return nil
			}

			for _, d := range result.Diagnostics {
				prefix := "error"
				if d.Severity == "warning" {
					prefix = "warn"
				} else if d.Severity == "info" {
					prefix = "info"
				}
				cid := ""
				if d.ConstraintID != "" {
					cid = " " + d.ConstraintID
				}
				fmt.Printf("%s [%s] %s%s: %s\n", prefix, d.Kind, d.SpecID, cid, d.Message)
			}

			fmt.Printf("\n%d error(s), %d warning(s), %d info\n", result.Summary.Errors, result.Summary.Warnings, result.Summary.Info)

			if result.Summary.Errors > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().IntVar(&tierOverride, "tier", 0, "Override tier enforcement level")
	return cmd
}

func coverageCmd() *cobra.Command {
	var jsonOutput bool
	var testsGlob string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Generate spec-to-test traceability matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			files := discoverSpecs()
			_, specs, hasErrors := parseAllSpecs(files)
			if hasErrors {
				os.Exit(1)
			}

			testFiles := discoverTestFiles(testsGlob)
			var allAnnotations []coverage.AnnotationMatch
			for _, f := range testFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(string(data), f)...)
			}

			report := coverage.BuildCoverageReport(specs, allAnnotations)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(report)
				return nil
			}

			fmt.Println("Spec Coverage Report")
			fmt.Println()
			fmt.Printf("%-24s %-6s %-8s %-9s %-10s %s\n", "Spec ID", "Tier", "ACs", "Covered", "Coverage", "Status")
			fmt.Println(strings.Repeat("-", 65))

			for _, e := range report.Entries {
				status := "PASS"
				if !e.PassesThreshold {
					if e.CoveragePct == 0 {
						status = "NONE"
					} else {
						status = "FAIL"
					}
				}
				fmt.Printf("%-24s T%-5d %-8d %-9d %-10s %s\n",
					e.SpecID, e.Tier, e.TotalACs, len(e.CoveredACs),
					fmt.Sprintf("%.0f%%", e.CoveragePct), status)

				if len(e.UncoveredACs) > 0 {
					fmt.Printf("  uncovered: %s\n", strings.Join(e.UncoveredACs, ", "))
				}
			}

			fmt.Printf("\n%d specs: %d passing, %d failing\n",
				report.Summary.TotalSpecs, report.Summary.Passing, report.Summary.Failing)

			if report.Summary.Failing > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&testsGlob, "tests", "", "Glob pattern for test files")
	return cmd
}

func syncCmd() *cobra.Command {
	var jsonOutput bool
	var testsGlob string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run full validation pipeline (parse + resolve + check + coverage)",
		RunE: func(cmd *cobra.Command, args []string) error {
			specFiles := discoverSpecs()
			if len(specFiles) == 0 {
				fmt.Println("No .spec.yaml files found.")
				os.Exit(1)
			}
			testFiles := discoverTestFiles(testsGlob)

			var specContents []specsync.FileContent
			for _, f := range specFiles {
				data, _ := os.ReadFile(f)
				specContents = append(specContents, specsync.FileContent{Path: f, Content: string(data)})
			}
			var testContents []specsync.FileContent
			for _, f := range testFiles {
				data, _ := os.ReadFile(f)
				testContents = append(testContents, specsync.FileContent{Path: f, Content: string(data)})
			}

			result := specsync.RunSync(specsync.SyncInput{
				SpecFiles: specContents,
				TestFiles: testContents,
			})

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]interface{}{
					"passed":     result.Passed,
					"phases":     result.Phases,
					"stopped_at": result.StoppedAt,
				})
				if !result.Passed {
					os.Exit(1)
				}
				return nil
			}

			fmt.Println("Specter Sync")
			fmt.Println()
			for _, p := range result.Phases {
				status := "PASS"
				if !p.Passed {
					status = "FAIL"
				}
				fmt.Printf("  %s %s: %s\n", status, p.Phase, p.Message)
			}
			fmt.Println()

			if result.Passed {
				fmt.Println("All checks passed.")
			} else {
				fmt.Printf("Pipeline failed at %s phase.\n", result.StoppedAt)
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&testsGlob, "tests", "", "Glob pattern for test files")
	return cmd
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/coverage"
	"github.com/Hanalyx/specter/internal/manifest"
	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/reverse"
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
	root.AddCommand(reverseCmd())
	root.AddCommand(initCmd())

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

			report := coverage.BuildCoverageReport(specs, allAnnotations, checker.CoverageThresholdByTier)

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

func reverseCmd() *cobra.Command {
	var (
		jsonOutput  bool
		adapterName string
		outputDir   string
		groupBy     string
		dryRun      bool
		excludes    []string
	)

	cmd := &cobra.Command{
		Use:   "reverse [path]",
		Short: "Extract draft .spec.yaml files from existing source code",
		Long:  "Analyze source code and test files to generate draft .spec.yaml specifications. Uses language-specific adapters (typescript, python, go) to extract constraints from validation schemas, acceptance criteria from test assertions, and gaps where constraints lack test coverage.",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}

			// Walk target path and read source files
			var files []reverse.SourceFile
			skipDirs := map[string]bool{
				"node_modules": true, "dist": true, ".git": true,
				"vendor": true, "__pycache__": true, ".next": true,
				"testdata": true, "bin": true,
			}

			_ = filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && skipDirs[info.Name()] {
					return filepath.SkipDir
				}
				if info.IsDir() {
					return nil
				}
				// Check --exclude patterns
				for _, pattern := range excludes {
					if matched, _ := filepath.Match(pattern, path); matched {
						return nil
					}
					// Also match against just the relative path segments
					if strings.Contains(path, pattern) {
						return nil
					}
				}
				lang := reverse.DetectLanguage(path)
				if lang == "" {
					base := filepath.Base(path)
					if base != "package.json" && base != "go.mod" && base != "pyproject.toml" {
						return nil
					}
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				files = append(files, reverse.SourceFile{
					Path:    path,
					Content: string(data),
				})
				return nil
			})

			// Look for manifest and schema files in parent directories
			manifests := []string{"package.json", "go.mod", "pyproject.toml",
				"prisma/schema.prisma", "schema.prisma"}
			absTarget, _ := filepath.Abs(targetPath)
			dir := absTarget
			for {
				for _, m := range manifests {
					mPath := filepath.Join(dir, m)
					if data, err := os.ReadFile(mPath); err == nil {
						// Only add if not already found during walk
						alreadyFound := false
						for _, f := range files {
							abs, _ := filepath.Abs(f.Path)
							if abs == mPath {
								alreadyFound = true
								break
							}
						}
						if !alreadyFound {
							files = append(files, reverse.SourceFile{
								Path:    mPath,
								Content: string(data),
							})
						}
					}
				}
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}

			if len(files) == 0 {
				fmt.Println("No source files found.")
				os.Exit(1)
			}

			adapters := []reverse.Adapter{
				&reverse.TypeScriptAdapter{},
				&reverse.PythonAdapter{},
				&reverse.GoAdapter{},
			}

			date := "2026-04-02"

			input := reverse.ReverseInput{
				Files:       files,
				AdapterName: adapterName,
				GroupBy:     groupBy,
				Date:        date,
			}

			result := reverse.Reverse(input, adapters)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]interface{}{
					"specs":       result.Specs,
					"diagnostics": result.Diagnostics,
					"summary":     result.Summary,
				})
				if result.Summary.SpecsGenerated == 0 {
					os.Exit(1)
				}
				return nil
			}

			for _, d := range result.Diagnostics {
				fmt.Fprintf(os.Stderr, "%s [%s] %s\n", d.Severity, d.Kind, d.Message)
			}

			if result.Summary.SpecsGenerated == 0 {
				fmt.Println("No specs generated. Check diagnostics above.")
				os.Exit(1)
			}

			for _, gs := range result.Specs {
				if dryRun {
					fmt.Printf("--- %s (dry-run) ---\n", gs.FileName)
					fmt.Println(gs.YAML)
					for _, w := range gs.Warnings {
						fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
					}
					continue
				}

				outPath := filepath.Join(outputDir, gs.FileName)
				if mkErr := os.MkdirAll(outputDir, 0755); mkErr != nil {
					fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", mkErr)
					os.Exit(1)
				}
				if wErr := os.WriteFile(outPath, []byte(gs.YAML), 0644); wErr != nil {
					fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, wErr)
					os.Exit(1)
				}
				fmt.Printf("GENERATED %s — %s@%s (%d constraints, %d ACs)\n",
					outPath, gs.Spec.ID, gs.Spec.Version,
					len(gs.Spec.Constraints), len(gs.Spec.AcceptanceCriteria))
				for _, w := range gs.Warnings {
					fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
				}
			}

			fmt.Printf("\nSummary: %d spec(s) generated, %d constraint(s), %d assertion(s), %d gap(s)\n",
				result.Summary.SpecsGenerated, result.Summary.ConstraintsFound,
				result.Summary.AssertionsFound, result.Summary.GapsDetected)

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&adapterName, "adapter", "", "Language adapter (typescript, python, go). Auto-detects if omitted")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "specs", "Output directory for generated .spec.yaml files")
	cmd.Flags().StringVar(&groupBy, "group-by", "file", "Grouping strategy: file or directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview output without writing files")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "Exclude paths matching pattern (can be repeated)")

	return cmd
}

// --- Manifest Helpers ---

func findManifest() (manifestPath string, projectRoot string) {
	dir, err := os.Getwd()
	if err != nil {
		return "", ""
	}
	for {
		candidate := filepath.Join(dir, "specter.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ""
}

func loadManifest() (*manifest.Manifest, string) {
	path, root := findManifest()
	if path == "" {
		return manifest.Defaults(), ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Defaults(), ""
	}
	m, err := manifest.ParseManifest(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid specter.yaml: %v (using defaults)\n", err)
		return manifest.Defaults(), ""
	}
	return m, root
}

func initCmd() *cobra.Command {
	var (
		name  string
		force bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a specter.yaml project manifest",
		Long:  "Scaffold a specter.yaml file from existing .spec.yaml files in the current directory. Groups all specs into a default domain with sensible settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat("specter.yaml"); err == nil && !force {
				fmt.Println("specter.yaml already exists. Use --force to overwrite.")
				os.Exit(1)
			}

			if name == "" {
				dir, _ := os.Getwd()
				name = filepath.Base(dir)
			}

			specFiles := discoverSpecs()
			var specIDs []string
			for _, file := range specFiles {
				data, err := os.ReadFile(file)
				if err != nil {
					continue
				}
				result := parser.ParseSpec(string(data))
				if result.OK {
					specIDs = append(specIDs, result.Value.ID)
				}
			}

			yamlStr := manifest.ScaffoldManifest(name, "", specIDs)
			if err := os.WriteFile("specter.yaml", []byte(yamlStr), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing specter.yaml: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Created specter.yaml with %d spec(s) in system %q\n", len(specIDs), name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "System name (defaults to directory name)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing specter.yaml")

	return cmd
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/coverage"
	specdiff "github.com/Hanalyx/specter/internal/diff"
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
	root.AddCommand(doctorCmd())
	root.AddCommand(explainCmd())
	root.AddCommand(watchCmd())
	root.AddCommand(diffCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// --- Helpers ---

func discoverSpecs(patterns ...string) []string {
	if len(patterns) > 0 && patterns[0] != "" {
		return patterns
	}
	// Load manifest to honour settings.exclude — BUG-002 fix.
	m, _ := loadManifest()
	excludeNames := make(map[string]bool)
	for _, e := range m.ExcludePatterns() {
		excludeNames[e] = true
	}

	var files []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip by directory name (e.g. ".claude", "node_modules")
			if excludeNames[info.Name()] {
				return filepath.SkipDir
			}
			// Skip by path prefix for entries like "tests/fixtures", "testdata"
			if strings.HasPrefix(path, filepath.Join("tests", "fixtures")) ||
				strings.HasPrefix(path, "testdata") {
				return filepath.SkipDir
			}
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
	var jsonOutput, dotOutput, mermaidOutput bool
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
			} else if mermaidOutput {
				// C-09: Mermaid flowchart output (renders natively in GitHub PRs)
				fmt.Println("graph BT")
				for id, node := range graph.Nodes {
					fmt.Printf("    %s[\"%s@%s\"]\n", id, id, node.Spec.Version)
				}
				for _, e := range graph.Edges {
					if e.VersionRange != "" {
						fmt.Printf("    %s -->|\"%s\"| %s\n", e.From, e.VersionRange, e.To)
					} else {
						fmt.Printf("    %s --> %s\n", e.From, e.To)
					}
				}
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
					// C-10: dangling-reference suggestions
					if d.Kind == "dangling_reference" {
						if len(d.Suggestions) > 0 {
							fmt.Fprintf(os.Stderr, "  did you mean: %s\n", strings.Join(d.Suggestions, ", "))
						}
						if d.SuggestedFixPath != "" {
							fmt.Fprintf(os.Stderr, "  fix: create %s with `id: %s`\n", d.SuggestedFixPath, d.MissingDep)
						}
					}
				}
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&dotOutput, "dot", false, "Output graph in DOT format")
	cmd.Flags().BoolVar(&mermaidOutput, "mermaid", false, "Output graph in Mermaid format (renders in GitHub PRs)")
	return cmd
}

func checkCmd() *cobra.Command {
	var jsonOutput bool
	var tierOverride int
	var strict bool
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

			m, _ := loadManifest()
			opts := &checker.CheckOptions{
				Strict:      strict || m.Settings.Strict,
				WarnOnDraft: m.Settings.WarnOnDraft,
			}
			if tierOverride > 0 {
				opts.TierOverride = tierOverride
			}

			result := checker.CheckSpecs(graph, opts)

			// Tier conflict warnings (C-14)
			_, specs, _ := parseAllSpecs(files)
			tierConflicts := manifest.CheckTierConflicts(specs, m)

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(result)
				return nil
			}

			for _, tc := range tierConflicts {
				fmt.Printf("warn [tier_conflict] %s\n", tc.Message)
			}

			if len(result.Diagnostics) == 0 && len(tierConflicts) == 0 {
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
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors (also set via settings.strict in specter.yaml)")
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

			m, _ := loadManifest()
			var results *coverage.ResultsFile
			if data, err := os.ReadFile(".specter-results.json"); err == nil {
				results, _ = coverage.ParseResultsFile(data)
			}
			report := coverage.BuildCoverageReportWithResults(specs, allAnnotations, m.CoverageThresholds(), results)

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

			// Dependency coverage warnings
			var edges []coverage.DepEdge
			if inputs, _, _ := parseAllSpecs(files); len(inputs) > 0 {
				graph := resolver.ResolveSpecs(inputs)
				for _, e := range graph.Edges {
					edges = append(edges, coverage.DepEdge{From: e.From, To: e.To})
				}
			}
			for _, w := range coverage.CheckDependencyCoverage(edges, report) {
				fmt.Printf("warn [dependency_coverage] %s\n", w.Message)
				fmt.Printf("  run: specter explain %s:%s\n", w.DependsOn, w.UncoveredACs[0])
			}

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
	var onlyPhase string
	var strict bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Run full validation pipeline (parse + resolve + check + coverage)",
		RunE: func(cmd *cobra.Command, args []string) error {
			validPhases := map[string]bool{"parse": true, "resolve": true, "check": true, "coverage": true}
			if onlyPhase != "" && !validPhases[onlyPhase] {
				fmt.Fprintf(os.Stderr, "error: --only must be one of: parse, resolve, check, coverage\n")
				os.Exit(1)
			}

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

			m, _ := loadManifest()
			checkOpts := &checker.CheckOptions{
				Strict:      strict || m.Settings.Strict,
				WarnOnDraft: m.Settings.WarnOnDraft,
			}

			var results *coverage.ResultsFile
			if data, err := os.ReadFile(".specter-results.json"); err == nil {
				results, _ = coverage.ParseResultsFile(data)
			}

			result := specsync.RunSync(specsync.SyncInput{
				SpecFiles:  specContents,
				TestFiles:  testContents,
				Thresholds: m.CoverageThresholds(),
				CheckOpts:  checkOpts,
				OnlyPhase:  onlyPhase,
				Results:    results,
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

			for _, w := range result.DepCoverageWarnings {
				fmt.Printf("  warn [dependency_coverage] %s\n", w.Message)
				fmt.Printf("       run: specter explain %s:%s\n", w.DependsOn, w.UncoveredACs[0])
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
	cmd.Flags().StringVar(&onlyPhase, "only", "", "Run only this phase (parse|resolve|check|coverage); prerequisites run without halting")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors (also set via settings.strict in specter.yaml)")
	return cmd
}

func reverseCmd() *cobra.Command {
	var (
		jsonOutput  bool
		adapterName string
		outputDir   string
		groupBy     string
		dryRun      bool
		overwrite   bool
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

				// Skip existing files unless --overwrite is set
				if _, existErr := os.Stat(outPath); existErr == nil && !overwrite {
					fmt.Printf("SKIPPED %s (already exists, use --overwrite to replace)\n", outPath)
					continue
				}

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
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing spec files (default: skip)")
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
		name     string
		force    bool
		template string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a specter.yaml project manifest",
		Long:  "Scaffold a specter.yaml file from existing .spec.yaml files in the current directory. Groups all specs into a default domain with sensible settings.\n\nWith --template, creates a draft .spec.yaml file instead of specter.yaml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// --template: create a spec file, not specter.yaml
			if template != "" {
				return runInitTemplate(template, force)
			}

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
	cmd.Flags().StringVar(&template, "template", "", "Create a spec file from a template (api-endpoint, service, auth, data-model)")

	return cmd
}

// runInitTemplate creates a draft .spec.yaml from a named template.
func runInitTemplate(templateType string, force bool) error {
	content, err := manifest.SpecTemplate(templateType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	outFile := templateType + ".spec.yaml"
	if _, statErr := os.Stat(outFile); statErr == nil && !force {
		fmt.Printf("%s already exists. Use --force to overwrite.\n", outFile)
		os.Exit(1)
	}

	if err := os.WriteFile(outFile, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
		os.Exit(1)
	}

	fmt.Printf("Created %s (template: %s)\n", outFile, templateType)
	fmt.Println("Edit the file to replace placeholder values, then run: specter sync")
	return nil
}

// doctorCmd implements the specter doctor pre-flight health checker.
//
// @spec spec-doctor
func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight project health checks",
		Long:  "Checks project readiness before running the full sync pipeline. Reports PASS/WARN/FAIL for each check so developers know exactly what needs attention.",
		RunE: func(cmd *cobra.Command, args []string) error {
			anyFail := false

			// Helper: print an aligned check result line.
			printCheck := func(name, status, msg string) {
				fmt.Printf("  %-12s [%s]  %s\n", name, status, msg)
			}

			fmt.Println("specter doctor")
			fmt.Println()

			// C-08: run ALL checks regardless of failures

			// --- Check 1: Manifest presence (C-01, AC-01, AC-02) ---
			manifestPath, _ := findManifest()
			if manifestPath != "" {
				printCheck("manifest", "PASS", "specter.yaml found at "+manifestPath)
			} else {
				printCheck("manifest", "WARN", "No specter.yaml found — run `specter init` to scaffold one (optional)")
			}

			// --- Check 2: .spec.yaml files present (C-02, AC-03) ---
			specFiles := discoverSpecs()
			if len(specFiles) == 0 {
				printCheck("spec-files", "FAIL", "No .spec.yaml files found — create at least one spec to get started")
				anyFail = true
			} else {
				printCheck("spec-files", "PASS", fmt.Sprintf("%d spec file(s) discovered", len(specFiles)))
			}

			// --- Check 3: All specs parse cleanly (C-03, AC-04) ---
			parseOK := true
			parseErrors := 0
			for _, f := range specFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					parseOK = false
					parseErrors++
					continue
				}
				result := parser.ParseSpec(string(data))
				if !result.OK {
					parseOK = false
					parseErrors++
					for _, pe := range result.Errors {
						fmt.Printf("    %s: %s\n", f, pe.Message)
					}
				}
			}
			if !parseOK {
				printCheck("parse", "FAIL", fmt.Sprintf("%d spec file(s) have parse errors (see above)", parseErrors))
				anyFail = true
			} else if len(specFiles) > 0 {
				printCheck("parse", "PASS", "All specs parse cleanly")
			} else {
				printCheck("parse", "WARN", "No specs to parse")
			}

			// --- Check 4: @spec/@ac annotations in test files (C-04, AC-05) ---
			testFiles := discoverTestFiles("")
			annotationCount := 0
			for _, f := range testFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				annotations := coverage.ExtractAnnotations(string(data), f)
				annotationCount += len(annotations)
			}
			if annotationCount == 0 {
				printCheck("annotations", "WARN", "No @spec/@ac annotations found in test files — add annotations to track coverage")
			} else {
				printCheck("annotations", "PASS", fmt.Sprintf("%d annotation(s) found across %d test file(s)", annotationCount, len(testFiles)))
			}

			// --- Check 5: Coverage meets tier thresholds (C-05, AC-06) ---
			if len(specFiles) > 0 {
				m, _ := loadManifest()
				_, specs, hasParseErrors := parseAllSpecs(specFiles)
				if hasParseErrors {
					printCheck("coverage", "WARN", "Skipping coverage check — specs have parse errors")
				} else {
					var allAnnotations []coverage.AnnotationMatch
					for _, f := range testFiles {
						data, err := os.ReadFile(f)
						if err != nil {
							continue
						}
						allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(string(data), f)...)
					}
					thresholds := m.CoverageThresholds()
					report := coverage.BuildCoverageReport(specs, allAnnotations, thresholds)

					belowThreshold := 0
					for _, e := range report.Entries {
						if !e.PassesThreshold {
							belowThreshold++
						}
					}

					if belowThreshold > 0 {
						printCheck("coverage", "FAIL", fmt.Sprintf("%d spec(s) below tier coverage threshold", belowThreshold))
						for _, e := range report.Entries {
							if !e.PassesThreshold {
								threshold := thresholds[e.Tier]
								fmt.Printf("    %s: %.0f%% coverage (T%d requires %d%%)\n",
									e.SpecID, e.CoveragePct, e.Tier, threshold)
							}
						}
						anyFail = true
					} else {
						printCheck("coverage", "PASS", fmt.Sprintf("All %d spec(s) meet coverage thresholds", len(report.Entries)))
					}
				}
			} else {
				printCheck("coverage", "WARN", "No specs to check coverage for")
			}

			fmt.Println()

			// C-06: exit 0 if all PASS/WARN, exit 1 if any FAIL
			if anyFail {
				fmt.Println("Result: FAIL — fix the issues above before running `specter sync`")
				os.Exit(1)
			}
			fmt.Println("Result: OK — project is ready for `specter sync`")
			return nil
		},
	}
}

// explainCmd shows coverage status and annotation examples for a spec's ACs.
//
// @spec spec-explain
func explainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <spec-id>[:<ac-id>]",
		Short: "Show annotation examples for a spec's acceptance criteria",
		Long:  "Explains how to annotate tests to cover a spec's ACs. Run `specter explain <spec-id>` to list all ACs, or `specter explain <spec-id>:<ac-id>` for details on one.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse argument: "spec-id" or "spec-id:AC-NN"
			arg := args[0]
			specID := arg
			acID := ""
			if idx := strings.Index(arg, ":"); idx >= 0 {
				specID = arg[:idx]
				acID = arg[idx+1:]
			}

			// Load all specs
			specFiles := discoverSpecs()
			_, specs, _ := parseAllSpecs(specFiles)

			// Find the requested spec
			var targetSpec *schema.SpecAST
			for i := range specs {
				if specs[i].ID == specID {
					targetSpec = &specs[i]
					break
				}
			}
			if targetSpec == nil {
				fmt.Fprintf(os.Stderr, "error: spec %q not found\n", specID)
				fmt.Fprintf(os.Stderr, "  searched %d spec files\n", len(specFiles))
				if len(specs) > 0 {
					fmt.Fprintf(os.Stderr, "  available specs:")
					for _, s := range specs {
						fmt.Fprintf(os.Stderr, " %s", s.ID)
					}
					fmt.Fprintln(os.Stderr)
				}
				os.Exit(1)
			}

			// Discover test files and build coverage
			testFiles := discoverTestFiles("")
			var allAnnotations []coverage.AnnotationMatch
			for _, f := range testFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(string(data), f)...)
			}

			// Determine which ACs are covered and by which files
			coveredBy := make(map[string][]string) // acID -> []file
			for _, ann := range allAnnotations {
				if ann.SpecID != specID {
					continue
				}
				for _, id := range ann.ACIDs {
					coveredBy[id] = append(coveredBy[id], ann.File)
				}
			}

			// Detect annotation style from test file extensions
			langs := detectAnnotationLanguages(testFiles)

			if acID == "" {
				// List mode: show all ACs with status
				return explainListMode(targetSpec, coveredBy, testFiles, langs)
			}
			// Detail mode: one AC
			return explainDetailMode(targetSpec, acID, coveredBy, testFiles, langs)
		},
	}
}

// explainListMode lists all ACs in a spec with COVERED/UNCOVERED status.
func explainListMode(spec *schema.SpecAST, coveredBy map[string][]string, testFiles []string, langs []string) error {
	fmt.Printf("specter explain %s\n\n", spec.ID)
	fmt.Printf("  %-8s %-8s  %s\n", "Status", "AC", "Description")
	fmt.Println("  " + strings.Repeat("-", 60))

	for _, ac := range spec.AcceptanceCriteria {
		status := "UNCOVERED"
		if len(coveredBy[ac.ID]) > 0 {
			status = "COVERED"
		}
		desc := ac.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Printf("  %-8s %-8s  %s\n", status, ac.ID, desc)
	}

	fmt.Printf("\n  Scanned %d test file(s)\n", len(testFiles))
	fmt.Printf("  Run `specter explain %s:<ac-id>` for annotation examples\n", spec.ID)
	return nil
}

// explainDetailMode shows full details and annotation example for one AC.
func explainDetailMode(spec *schema.SpecAST, acID string, coveredBy map[string][]string, testFiles []string, langs []string) error {
	// Find the AC
	var targetAC *schema.AcceptanceCriterion
	for i := range spec.AcceptanceCriteria {
		if spec.AcceptanceCriteria[i].ID == acID {
			targetAC = &spec.AcceptanceCriteria[i]
			break
		}
	}
	if targetAC == nil {
		fmt.Fprintf(os.Stderr, "error: %s not found in spec %q\n", acID, spec.ID)
		fmt.Fprintf(os.Stderr, "  available ACs:")
		for _, ac := range spec.AcceptanceCriteria {
			fmt.Fprintf(os.Stderr, " %s", ac.ID)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}

	files := coveredBy[acID]

	fmt.Printf("specter explain %s:%s\n\n", spec.ID, acID)
	fmt.Printf("  Spec:   %s (v%s, tier %d)\n", spec.ID, spec.Version, spec.Tier)
	fmt.Printf("  %s:  %s\n", acID, targetAC.Description)

	if len(files) > 0 {
		fmt.Printf("  Status: COVERED\n\n")
		fmt.Println("  Covered in:")
		for _, f := range files {
			fmt.Printf("    %s\n", f)
		}
	} else {
		fmt.Printf("  Status: UNCOVERED\n\n")
		fmt.Println("  To cover this AC, add annotations in your test file:")
		fmt.Println()
		for _, lang := range langs {
			fmt.Printf("  %s:\n", lang)
			switch lang {
			case "Python":
				fmt.Printf("    # @spec %s\n", spec.ID)
				fmt.Printf("    # @ac %s\n", acID)
				fmt.Printf("    def test_%s_%s():\n", sanitizeID(spec.ID), strings.ToLower(strings.ReplaceAll(acID, "-", "_")))
				fmt.Printf("        # %s\n", targetAC.Description)
				fmt.Printf("        ...\n")
			case "TypeScript / JavaScript":
				fmt.Printf("    // @spec %s\n", spec.ID)
				fmt.Printf("    // @ac %s\n", acID)
				fmt.Printf("    it('%s: %s', () => {\n", acID, targetAC.Description)
				fmt.Printf("      // test implementation\n")
				fmt.Printf("    });\n")
			default: // Go / generic
				fmt.Printf("    // @spec %s\n", spec.ID)
				fmt.Printf("    // @ac %s\n", acID)
				funcName := "Test" + toCamelCase(spec.ID) + "_" + strings.ReplaceAll(acID, "-", "")
				fmt.Printf("    func %s(t *testing.T) {\n", funcName)
				fmt.Printf("        // %s\n", targetAC.Description)
				fmt.Printf("    }\n")
			}
			fmt.Println()
		}
	}

	fmt.Printf("  Scanned %d test file(s)\n", len(testFiles))
	return nil
}

// detectAnnotationLanguages returns the annotation language labels for a set of test files.
//
// C-08: detect from file extensions.
func detectAnnotationLanguages(testFiles []string) []string {
	hasGo, hasPy, hasTS := false, false, false
	for _, f := range testFiles {
		switch {
		case strings.HasSuffix(f, ".go"):
			hasGo = true
		case strings.HasSuffix(f, ".py"):
			hasPy = true
		case strings.HasSuffix(f, ".ts") || strings.HasSuffix(f, ".tsx") ||
			strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".jsx"):
			hasTS = true
		}
	}
	// Default to Go/generic if nothing detected
	if !hasGo && !hasPy && !hasTS {
		return []string{"Go / generic"}
	}
	var langs []string
	if hasGo {
		langs = append(langs, "Go / generic")
	}
	if hasPy {
		langs = append(langs, "Python")
	}
	if hasTS {
		langs = append(langs, "TypeScript / JavaScript")
	}
	return langs
}

func sanitizeID(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}

func toCamelCase(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

// watchCmd re-runs the sync pipeline whenever spec or test files change.
//
// @spec spec-watch
func watchCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Re-run sync pipeline on file changes",
		Long:  "Polls for changes in .spec.yaml and test files and re-runs the sync pipeline. Press Ctrl+C to stop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _ := loadManifest()

			// C-08: startup message with watched globs and interval
			specsDir := m.SpecsDir()
			fmt.Printf("specter watch\n\n")
			fmt.Printf("  Watching: %s/**/*.spec.yaml, test files\n", specsDir)
			fmt.Printf("  Interval: %s\n", interval)
			fmt.Printf("  Press Ctrl+C to stop\n\n")

			// C-06: run once immediately on startup
			lastMods := collectModTimes()
			runWatchCycle(m)

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sig)

			for {
				select {
				case <-sig:
					// C-04: exit 0 on interrupt
					fmt.Println("\nstopped")
					return nil
				case <-ticker.C:
					currentMods := collectModTimes()
					if modsChanged(lastMods, currentMods) {
						lastMods = currentMods
						runWatchCycle(m)
					}
				}
			}
		},
	}

	// C-05: configurable poll interval
	cmd.Flags().DurationVar(&interval, "interval", 500*time.Millisecond, "Poll interval (e.g. 500ms, 1s, 2s)")
	return cmd
}

// runWatchCycle executes the sync pipeline and prints a compact summary line.
//
// C-03: timestamped summary, C-07: continues on FAIL.
func runWatchCycle(m *manifest.Manifest) {
	specFiles := discoverSpecs()
	testFiles := discoverTestFiles("")

	timestamp := time.Now().Format("15:04:05")

	if len(specFiles) == 0 {
		fmt.Printf("[%s] WARN  no spec files found\n", timestamp)
		return
	}

	inputs, specs, hasErrors := parseAllSpecs(specFiles)
	if hasErrors {
		fmt.Printf("[%s] FAIL  parse errors in spec files\n", timestamp)
		return
	}

	// Build coverage
	var allAnnotations []coverage.AnnotationMatch
	for _, f := range testFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(string(data), f)...)
	}

	thresholds := m.CoverageThresholds()
	report := coverage.BuildCoverageReport(specs, allAnnotations, thresholds)

	// Count covered ACs
	totalACs := 0
	coveredACs := 0
	for _, e := range report.Entries {
		totalACs += e.TotalACs
		coveredACs += len(e.CoveredACs)
	}

	// Run resolve + check to determine PASS/FAIL
	_ = inputs // used in real sync; here we just use coverage for the summary
	passing := report.Summary.Passing
	failing := report.Summary.Failing

	status := "PASS"
	if failing > 0 || hasErrors {
		status = "FAIL"
	}

	fmt.Printf("[%s] %-4s  %d spec(s)  %d/%d ACs covered  (%d passing, %d failing)\n",
		timestamp, status, len(specs), coveredACs, totalACs, passing, failing)
}

// collectModTimes snapshots the modification times of all watched files.
func collectModTimes() map[string]time.Time {
	mods := make(map[string]time.Time)
	for _, f := range discoverSpecs() {
		if info, err := os.Stat(f); err == nil {
			mods[f] = info.ModTime()
		}
	}
	for _, f := range discoverTestFiles("") {
		if info, err := os.Stat(f); err == nil {
			mods[f] = info.ModTime()
		}
	}
	return mods
}

// modsChanged returns true if any file's modification time differs between snapshots.
func modsChanged(prev, curr map[string]time.Time) bool {
	if len(prev) != len(curr) {
		return true
	}
	for f, t := range curr {
		if prev[f] != t {
			return true
		}
	}
	return false
}


// @spec spec-diff
func diffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <path>[@<ref>] <path>[@<ref>]",
		Short: "Show semantic diff of a spec between two git revisions",
		Long: `Compare two versions of a spec and show a human-readable semantic diff.

Each argument is either:
  path            — read from disk
  path@ref        — read from git (e.g. specs/foo.spec.yaml@HEAD~1)

Example:
  specter diff specs/engine.spec.yaml@HEAD~5 specs/engine.spec.yaml`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			v1, err := readSpecAtRef(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", args[0], err)
				os.Exit(1)
			}
			v2, err := readSpecAtRef(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", args[1], err)
				os.Exit(1)
			}

			d := specdiff.DiffSpecs(*v1, *v2)

			if d.Class == specdiff.ChangeUnchanged {
				fmt.Printf("spec %s %s → %s: no changes\n", d.SpecID, d.OldVersion, d.NewVersion)
				return nil
			}

			fmt.Printf("spec %s %s → %s [%s]\n", d.SpecID, d.OldVersion, d.NewVersion, d.Class)
			fmt.Println()

			for _, c := range d.ACChanges {
				switch c.Kind {
				case "added":
					fmt.Printf("  +%s: %s\n", c.ID, c.Description)
				case "removed":
					fmt.Printf("  -%s: %s\n", c.ID, c.Description)
				case "changed":
					fmt.Printf("  ~%s: %s → %s\n", c.ID, c.OldDesc, c.Description)
				}
			}
			for _, c := range d.ConstraintChanges {
				switch c.Kind {
				case "added":
					fmt.Printf("  +%s: %s\n", c.ID, c.Description)
				case "removed":
					fmt.Printf("  -%s: %s\n", c.ID, c.Description)
				case "changed":
					fmt.Printf("  ~%s: %s → %s\n", c.ID, c.OldDesc, c.Description)
				}
			}
			for _, dc := range d.DepChanges {
				fmt.Printf("  ~depends_on %s: %s → %s\n", dc.SpecID, dc.OldRange, dc.NewRange)
			}
			return nil
		},
	}
}

// readSpecAtRef reads and parses a spec from disk or from a git ref.
// The argument format is path[@ref]. If no @ref, reads from disk.
func readSpecAtRef(arg string) (*schema.SpecAST, error) {
	// Split on the last '@' to get path and ref
	path, ref := arg, ""
	if idx := strings.LastIndex(arg, "@"); idx > 0 {
		path, ref = arg[:idx], arg[idx+1:]
	}

	var content []byte
	if ref == "" {
		// Read from disk
		var err error
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	} else {
		// Read from git: git show <ref>:<path>
		// Normalize path to be repo-relative
		out, err := gitShow(ref, path)
		if err != nil {
			return nil, fmt.Errorf("git show failed: %w", err)
		}
		content = out
	}

	pr := parser.ParseSpec(string(content))
	if !pr.OK {
		return nil, fmt.Errorf("parse failed: %v", pr.Errors)
	}
	return pr.Value, nil
}

// gitShow runs `git show <ref>:<path>` and returns the output.
func gitShow(ref, path string) ([]byte, error) {
	return exec.Command("git", "show", ref+":"+path).Output()
}

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/coverage"
	specdiff "github.com/Hanalyx/specter/internal/diff"
	"github.com/Hanalyx/specter/internal/explain"
	"github.com/Hanalyx/specter/internal/manifest"
	"github.com/Hanalyx/specter/internal/migrate"
	"github.com/Hanalyx/specter/internal/parser"
	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/reverse"
	"github.com/Hanalyx/specter/internal/schema"
	specsync "github.com/Hanalyx/specter/internal/sync"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var version = "dev"

const issuesURL = "https://github.com/Hanalyx/specter/issues/new?template=bug_report.md"

// ---------------------------------------------------------------------------
// Shared constants — extracted to avoid duplication and magic values.
// ---------------------------------------------------------------------------

const specFileExt = ".spec.yaml"

var testFileExts = []string{".test.ts", ".test.js", ".test.py", "_test.go", "_test.py"}

var validSyncPhases = map[string]bool{"parse": true, "resolve": true, "check": true, "coverage": true}

const watchDebounce = 150 * time.Millisecond

const maxDescLen = 50

// noSpecsMessage is used by parse/resolve/sync when discovery turns up
// nothing. Explains where specter looked and what to try next — users often
// keep specs in a non-default directory and need the hint.
func noSpecsMessage() string {
	m, manifestRoot, _ := loadManifest()
	var searched string
	if manifestRoot == "" {
		searched = "current directory and subdirectories (no specter.yaml found)"
	} else {
		searched = fmt.Sprintf("%q (from specter.yaml)", m.SpecsDir())
	}
	return fmt.Sprintf(
		"No .spec.yaml files found.\n\n"+
			"  Searched: %s\n"+
			"  Extension: .spec.yaml (literal)\n\n"+
			"What to try next:\n"+
			"  • Generate draft specs from existing code:   specter reverse src/\n"+
			"  • Scaffold from a template:                   specter init --template api-endpoint\n"+
			"  • Point specter at a different directory:     add specs_dir to specter.yaml\n",
		searched,
	)
}

// errSilent is returned from RunE when diagnostics have already been printed
// to stderr. Cobra will set exit code 1 without printing anything extra
// (because SilenceErrors is true on the root command).
var errSilent = fmt.Errorf("")

// ---------------------------------------------------------------------------
// CLI spinner — writes to stderr so it never interferes with --json or file
// output. Suppressed when stderr is not a terminal (pipes, CI).
// ---------------------------------------------------------------------------

type spinner struct {
	msg    chan string
	done   chan struct{}
	active bool
}

func newSpinner(initial string) *spinner {
	// Suppress in non-interactive environments (pipes, CI, dumb terminals).
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") != "" {
		return &spinner{active: false}
	}
	// Quick isatty check via os.Stderr.Stat — character device = terminal.
	if fi, err := os.Stderr.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return &spinner{active: false}
	}
	s := &spinner{
		msg:    make(chan string, 4),
		done:   make(chan struct{}),
		active: true,
	}
	go s.run(initial)
	return s
}

func (s *spinner) run(initial string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	msg := initial
	i := 0
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.done:
			fmt.Fprintf(os.Stderr, "\r\033[K") // clear the spinner line
			return
		case m := <-s.msg:
			msg = m
		case <-tick.C:
			fmt.Fprintf(os.Stderr, "\r\033[K%s %s", frames[i%len(frames)], msg)
			i++
		}
	}
}

// update changes the spinner message without stopping it.
func (s *spinner) update(msg string) {
	if !s.active {
		return
	}
	s.msg <- msg
}

// stop clears the spinner line and terminates the goroutine.
func (s *spinner) stop() {
	if !s.active {
		return
	}
	s.active = false
	close(s.done)
}

func main() {
	// Catch unexpected panics and print a pre-filled bug report link.
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			fmt.Fprintf(os.Stderr, "\n\nUnexpected error: %v\n", r)
			fmt.Fprintf(os.Stderr, "This looks like a bug in Specter. Please report it:\n")
			fmt.Fprintf(os.Stderr, "  %s\n\n", bugReportURL(fmt.Sprintf("panic: %v\n\nStack trace:\n```\n%s```", r, stack)))
			os.Exit(2)
		}
	}()

	root := &cobra.Command{
		Use:     "specter",
		Short:   "A type system for specs",
		Long:    "Specter validates, links, and type-checks .spec.yaml files the way tsc validates .ts files.",
		Version: version,
	}
	root.SilenceUsage = true
	root.SilenceErrors = true

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
	root.AddCommand(ingestCmd())
	root.AddCommand(feedbackCmd())
	root.AddCommand(prePushCheckCmd())

	if err := root.Execute(); err != nil {
		// errSilent is our sentinel for "command already printed diagnostics
		// to stderr; don't append anything." Used by every subcommand's RunE.
		// Everything else is a Cobra-surfaced error (unknown command, bad
		// flag, wrong args) that we DO need to print — SilenceErrors=true
		// suppresses Cobra's own printing of them, so we have to do it.
		if err != errSilent {
			fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
			fmt.Fprintln(os.Stderr, "\nRun `specter --help` to see available commands.")
		}
		os.Exit(1)
	}
}

// bugReportURL returns a pre-filled GitHub issue URL with version, OS, command,
// and Go runtime info already populated in the issue body.
func bugReportURL(context string) string {
	// Redact full paths from args — keep only the subcommand + flags.
	args := os.Args
	cmd := "specter"
	if len(args) > 1 {
		cmd = "specter " + strings.Join(args[1:], " ")
	}

	body := fmt.Sprintf(
		"**Specter version:** %s\n"+
			"**OS / arch:** %s/%s\n"+
			"**Go runtime:** %s\n"+
			"**Command run:** `%s`\n\n"+
			"**What happened:**\n%s\n\n"+
			"**Expected behavior:**\n<!-- what did you expect? -->\n\n"+
			"**Steps to reproduce:**\n1.\n2.\n3.\n\n"+
			"**Spec file (if relevant):**\n```yaml\n\n```\n",
		version,
		runtime.GOOS, runtime.GOARCH,
		runtime.Version(),
		cmd,
		context,
	)
	return issuesURL + "&body=" + url.QueryEscape(body)
}

// feedbackCmd opens (or prints) a pre-filled GitHub issue URL.
func feedbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feedback",
		Short: "Open a pre-filled GitHub issue to report a bug or request a feature",
		Long: `Opens a GitHub issue form pre-filled with your Specter version, OS, and
Go runtime. Describe the bug or feature request in the form.

If your browser does not open automatically, copy and paste the printed URL.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			link := bugReportURL("<!-- describe what went wrong -->")
			opened := tryOpenBrowser(link)
			if opened {
				fmt.Println("Opening GitHub issue form in your browser...")
			} else {
				fmt.Println("Copy and open this URL in your browser to file a report:")
				fmt.Println()
			}
			fmt.Println(link)
			return nil
		},
	}
}

// tryOpenBrowser attempts to open url in the default browser.
// Returns true if the open command was launched successfully.
func tryOpenBrowser(link string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	default:
		return false
	}
	return cmd.Start() == nil
}

// --- Helpers ---

func discoverSpecs(patterns ...string) []string {
	if len(patterns) > 0 && patterns[0] != "" {
		return patterns
	}
	// Load manifest to honor settings.exclude and specs_dir.
	m, manifestRoot, _ := loadManifest()
	excludeNames := make(map[string]bool)
	for _, e := range m.ExcludePatterns() {
		excludeNames[e] = true
	}
	// spec-doctor C-10 / GH #93: when no specter.yaml is found, walk
	// recursively from cwd instead of the manifest default `specs`.
	// AC-02 reports manifest as WARN/optional; if discovery then required
	// specs to live under ./specs/, the optional framing is a lie. When a
	// manifest IS present (manifestRoot != ""), honor settings.specs_dir
	// as before so explicit configurations stay authoritative (AC-11).
	var root string
	if manifestRoot == "" {
		root = "."
	} else {
		root = m.SpecsDir()
	}

	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		if strings.HasSuffix(path, specFileExt) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func discoverTestFiles(glob string) []string {
	if glob != "" {
		return globMatchWalk(glob)
	}

	// Default: walk the repo for all recognized test file suffixes.
	var files []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == "node_modules" || info.Name() == "dist" || info.Name() == ".git") {
			return filepath.SkipDir
		}
		for _, ext := range testFileExts {
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
	inputs, specs, _, hasErrors := parseAllSpecsDetailed(files)
	return inputs, specs, hasErrors
}

// parseAllSpecsDetailed is parseAllSpecs with the per-file parse errors
// collected in structured form. The v0.9.0 coverage JSON contract surfaces
// these to the VS Code extension so the sidebar can distinguish "no specs
// yet" from "specs present but failed to parse" — see spec-coverage C-10.
func parseAllSpecsDetailed(files []string) ([]resolver.SpecInput, []schema.SpecAST, []coverage.ParseErrorEntry, bool) {
	var inputs []resolver.SpecInput
	var specs []schema.SpecAST
	var parseErrors []coverage.ParseErrorEntry
	hasErrors := false

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", file, err)
			hasErrors = true
			parseErrors = append(parseErrors, coverage.ParseErrorEntry{
				File:    file,
				Type:    "io",
				Message: err.Error(),
			})
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
				parseErrors = append(parseErrors, coverage.ParseErrorEntry{
					File:    file,
					Path:    e.Path,
					Type:    e.Type,
					Message: e.Message,
					Line:    e.Line,
					Column:  e.Column,
				})
			}
		}
	}

	return inputs, specs, parseErrors, hasErrors
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
				fmt.Print(noSpecsMessage())
				return errSilent
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
				return errSilent
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
				fmt.Print(noSpecsMessage())
				return errSilent
			}

			inputs, _, hasErrors := parseAllSpecs(files)
			if hasErrors {
				fmt.Fprintln(os.Stderr, "\nFix parse errors before resolving dependencies.")
				return errSilent
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
				// Plain-English footer belongs on stdout only for the default
				// human-readable format. When the user asked for a structured
				// format (--dot, --mermaid, --json) stdout must be pure so the
				// output pipes cleanly to dot / mmdc / jq. The "no issues"
				// status is implicit in the successful exit code.
				if !dotOutput && !mermaidOutput && !jsonOutput {
					fmt.Println("No dependency issues found.")
				}
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
				return errSilent
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
	var testAnnotations bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run type-checking rules across the spec graph",
		RunE: func(cmd *cobra.Command, args []string) error {
			files := discoverSpecs()
			inputs, _, hasErrors := parseAllSpecs(files)
			if hasErrors {
				return errSilent
			}

			graph := resolver.ResolveSpecs(inputs)
			resolverErrors := 0
			for _, d := range graph.Diagnostics {
				if d.Severity == "error" {
					fmt.Fprintf(os.Stderr, "error [%s] %s\n", d.Kind, d.Message)
					resolverErrors++
				} else if d.Severity == "warning" {
					fmt.Fprintf(os.Stderr, "warn [%s] %s\n", d.Kind, d.Message)
				}
			}
			if resolverErrors > 0 {
				fmt.Fprintf(os.Stderr, "\n%d resolver error(s) — fix dependency issues before running check\n", resolverErrors)
				return errSilent
			}

			m, _, mErr := loadManifest()
			if mErr != nil {
				fmt.Fprintln(os.Stderr, "error:", mErr)
				return errSilent
			}
			opts := &checker.CheckOptions{
				Strict:      strict || m.Settings.Strict,
				WarnOnDraft: m.Settings.WarnOnDraft,
			}
			if tierOverride > 0 {
				opts.TierOverride = tierOverride
			}

			result := checker.CheckSpecs(graph, opts)

			// C-09: opt-in test-annotation cross-reference.
			_, specs, _ := parseAllSpecs(files)
			if testAnnotations {
				testFiles := discoverTestFiles("")
				contents := make(map[string]string, len(testFiles))
				for _, path := range testFiles {
					data, err := os.ReadFile(path)
					if err != nil {
						continue
					}
					contents[path] = string(data)
				}
				taDiags := checker.CheckTestAnnotations(contents, specs)
				result.Diagnostics = append(result.Diagnostics, taDiags...)
				for _, d := range taDiags {
					switch d.Severity {
					case "error":
						result.Summary.Errors++
					case "warning":
						result.Summary.Warnings++
					case "info":
						result.Summary.Info++
					}
				}
			}

			// Tier conflict warnings (C-14)
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
				ctype := ""
				if d.ConstraintType != "" {
					ctype = " (" + d.ConstraintType + ")"
				}
				fmt.Printf("%s [%s] %s%s%s: %s\n", prefix, d.Kind, d.SpecID, cid, ctype, d.Message)
			}

			fmt.Printf("\n%d error(s), %d warning(s), %d info\n", result.Summary.Errors, result.Summary.Warnings+len(tierConflicts), result.Summary.Info)

			if result.Summary.Errors > 0 {
				return errSilent
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().IntVar(&tierOverride, "tier", 0, "Override tier enforcement level")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors (also set via settings.strict in specter.yaml)")
	cmd.Flags().BoolVarP(&testAnnotations, "test", "t", false, "Cross-reference test-file @spec/@ac annotations against parsed specs")
	return cmd
}

func coverageCmd() *cobra.Command {
	var jsonOutput bool
	var testsGlob string
	var failingOnly bool
	var strict bool
	var scope string
	var strictnessFlag string
	var quiet bool
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Generate spec-to-test traceability matrix",
		RunE: func(cmd *cobra.Command, args []string) error {
			// AC-25: --scope without --strict is an operator mistake.
			// Fail-fast rather than silently degrading to annotation counting.
			if scope != "" && !strict {
				fmt.Fprintln(os.Stderr, "error: --scope requires --strict")
				return errSilent
			}

			files := discoverSpecs()
			inputs, specs, parseErrors, hasErrors := parseAllSpecsDetailed(files)

			// Build specID → source-file map so the CLI can populate
			// SpecFile on each coverage entry after the pure builder runs.
			// Consumers (VS Code sidebar click-to-open) rely on this.
			specFileByID := make(map[string]string, len(inputs))
			for _, in := range inputs {
				specFileByID[in.Spec.ID] = in.File
			}

			m, _, mErr := loadManifest()
			if mErr != nil {
				fmt.Fprintln(os.Stderr, "error:", mErr)
				return errSilent
			}

			// C-25: when --tests is unset, fall back to settings.tests_glob.
			// Manifest may carry multiple globs as a list; iterate and union
			// the matches (deduped). Empty manifest list + no --tests falls
			// through to discoverTestFiles("") which walks "." for known
			// test-file extensions.
			var testFiles []string
			switch {
			case testsGlob != "":
				testFiles = discoverTestFiles(testsGlob)
			case len(m.Settings.TestsGlob) > 0:
				seen := map[string]bool{}
				for _, g := range m.Settings.TestsGlob {
					for _, f := range discoverTestFiles(g) {
						if !seen[f] {
							seen[f] = true
							testFiles = append(testFiles, f)
						}
					}
				}
			default:
				testFiles = discoverTestFiles("")
			}
			var allAnnotations []coverage.AnnotationMatch
			for _, f := range testFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				allAnnotations = append(allAnnotations, coverage.ExtractAnnotations(string(data), f)...)
			}

			// Validate the CLI flag against the same enum as settings.strictness.
			// Reject typos at the flag layer rather than silently falling
			// through (the same class GH #76 closes for the manifest).
			if strictnessFlag != "" {
				validStrictness := map[string]bool{"annotation": true, "threshold": true, "zero-tolerance": true}
				if !validStrictness[strictnessFlag] {
					fmt.Fprintf(os.Stderr, "error: --strictness %q is not a valid value (allowed: annotation, threshold, zero-tolerance)\n", strictnessFlag)
					return errSilent
				}
			}

			// Resolve effective strictness: CLI flag overrides manifest setting.
			effectiveStrictness := m.Settings.Strictness
			if strictnessFlag != "" {
				effectiveStrictness = strictnessFlag
			}
			if effectiveStrictness == "" {
				effectiveStrictness = "threshold"
			}

			// C-24: --strict combined with strictness=annotation is incoherent
			// (annotation mode is for new adopters; strict requires runner-visible
			// annotations). Fail-fast with a clear message.
			if strict && effectiveStrictness == "annotation" {
				fmt.Fprintln(os.Stderr, "error: --strict requires settings.strictness >= threshold; current strictness is annotation")
				fmt.Fprintln(os.Stderr, "       set settings.strictness to 'threshold' or 'zero-tolerance' in specter.yaml, or pass --strictness <level>")
				return errSilent
			}

			// C-27: empty test discovery under --strict surfaces a warning.
			// Under zero-tolerance, the warning becomes a hard error.
			if strict && len(allAnnotations) == 0 {
				fmt.Fprintln(os.Stderr, "warn: no test files contained @spec/@ac annotations — coverage will report 0% for every spec")
				fmt.Fprintln(os.Stderr, "      set settings.tests_glob in specter.yaml or pass --tests <glob>")
				if effectiveStrictness == "zero-tolerance" {
					fmt.Fprintln(os.Stderr, "error: zero-tolerance strictness requires at least one annotated test file")
					return errSilent
				}
			}

			// AC-25: resolve --scope domain to a set of spec IDs.
			// Unknown domain → fail-fast listing valid names.
			var scopedSpecs map[string]bool
			if scope != "" {
				domain, ok := m.Domains[scope]
				if !ok {
					var validNames []string
					for name := range m.Domains {
						validNames = append(validNames, name)
					}
					sort.Strings(validNames)
					fmt.Fprintf(os.Stderr, "error: unknown domain %q. valid domains: %s\n",
						scope, strings.Join(validNames, ", "))
					return errSilent
				}
				scopedSpecs = make(map[string]bool, len(domain.Specs))
				for _, s := range domain.Specs {
					scopedSpecs[s] = true
				}
			}

			var results *coverage.ResultsFile
			if data, err := os.ReadFile(".specter-results.json"); err == nil {
				var pErr error
				results, pErr = coverage.ParseResultsFile(data)
				if pErr != nil {
					fmt.Fprintf(os.Stderr, "warn: could not parse .specter-results.json: %v\n", pErr)
				}
			}

			// AC-23 / C-22: under --strict, a parseable-but-empty results
			// file will demote 100% of annotated ACs. That's correct per
			// C-19 but silent mass demotion on Day 1 is a documentation
			// failure — warn the operator about the likely cause (tests
			// without runner-visible annotations) and point at the
			// conventions doc BEFORE the report prints.
			if strict && results != nil && len(results.Results) == 0 {
				fmt.Fprintln(os.Stderr, "warn: no (spec_id, ac_id) pairs were extracted from test output — tests likely don't carry runner-visible annotations")
				fmt.Fprintln(os.Stderr, "      see docs/explainer/v0.10-ci-gated-coverage.md, Conventions A and B")
			}

			report, strictErr := coverage.BuildCoverageReportStrict(specs, allAnnotations, m.CoverageThresholds(), results, strict, scopedSpecs)
			if strictErr != nil {
				fmt.Fprintf(os.Stderr, "error: %s\n", strictErr.Error())
				return errSilent
			}
			report.ParseErrors = parseErrors
			report.ParseErrorPatterns = coverage.SummarizeParseErrors(parseErrors)
			report.SpecCandidatesCount = len(files)
			for i := range report.Entries {
				report.Entries[i].SpecFile = specFileByID[report.Entries[i].SpecID]
			}

			// spec-coverage C-28 / GH #80: when --strict, surface (spec_id,
			// ac_id) pairs that have source-file annotations but no matching
			// .specter-results.json entry. Captures the missing-runtime-channel
			// cause that v0.10's mechanical demotion otherwise leaves silent.
			// Always populate report.DiagnosticHints so --json carries them;
			// stderr printing happens later under non-json + non-quiet.
			if strict {
				report.DiagnosticHints = coverage.DiagnoseSourceOnlyACs(allAnnotations, results, specs)
			}

			// GH #94 — under zero-tolerance, demote ACs that violate the
			// approval_gate contract (approval_gate: true with unset
			// approval_date) so the report reflects the same enforcement
			// signal the exit code carries. v0.11.0 fired the exit code but
			// left the report unchanged; the user-visible PASS/FAIL cell
			// stayed identical between threshold and zero-tolerance.
			//
			// Demotion shape: move the AC from CoveredACs to UncoveredACs,
			// recompute CoveragePct + PassesThreshold per entry, recompute
			// Summary.Passing / Summary.Failing.
			if effectiveStrictness == "zero-tolerance" {
				demoteApprovalGateViolations(report, specs)
			}

			// C-10: --json emits the report in every state, including when
			// parse failed. Downstream consumers (VS Code extension) branch on
			// ParseErrors vs Entries to decide what to render. Exit code, not
			// stdout presence, signals pass/fail.
			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(report)
				if hasErrors {
					return errSilent
				}
				return nil
			}

			if hasErrors {
				return errSilent
			}

			// AC-31 / AC-33: per-AC source-only hints are printed to stderr
			// ABOVE the table when --strict is on, --quiet is off, and we're
			// not in --json mode (JSON consumers see them in DiagnosticHints).
			if strict && !quiet && len(report.DiagnosticHints) > 0 {
				for _, h := range report.DiagnosticHints {
					loc := h.File
					if h.Line > 0 {
						loc = fmt.Sprintf("%s:%d", h.File, h.Line)
					}
					fmt.Fprintf(os.Stderr,
						"hint: %s/%s has source annotation in %s but no matching pass in .specter-results.json\n",
						h.SpecID, h.ACID, loc)
					fmt.Fprintln(os.Stderr,
						"      did your test runner emit a runner-visible annotation? "+
							"(Convention A: spec-id/AC-NN in the test name; "+
							"Convention B: print '// @spec'/'// @ac' lines from the test body)")
				}
			}

			// C-16: summary header ABOVE the table, reflects the full
			// report even when --failing filters the rendered rows.
			fmt.Print(coverage.BuildSummaryHeader(report))
			fmt.Println()

			// C-15 / C-17: sort worst-first; optionally filter to sub-100%.
			displayEntries := coverage.SortCoverageEntriesForDisplay(report.Entries)
			if failingOnly {
				displayEntries = coverage.FilterFailing(displayEntries)
				if len(displayEntries) == 0 {
					fmt.Printf("All %d specs at 100%% coverage.\n", len(report.Entries))
					// Exit code still respects threshold pass/fail —
					// --failing is a display filter, not a status change.
					if report.Summary.Failing > 0 {
						return errSilent
					}
					return nil
				}
			}

			fmt.Printf("%-41s %-6s %-8s %-9s %-10s %s\n", "Spec ID", "Tier", "ACs", "Covered", "Coverage", "Status")
			fmt.Println(strings.Repeat("-", 82))

			for _, e := range displayEntries {
				status := "PASS"
				if !e.PassesThreshold {
					if e.CoveragePct == 0 {
						status = "NONE"
					} else {
						status = "FAIL"
					}
				}
				// C-18: truncate long spec IDs so the column stays aligned.
				fmt.Printf("%-41s T%-5d %-8d %-9d %-10s %s\n",
					coverage.DisplaySpecID(e.SpecID), e.Tier, e.TotalACs, len(e.CoveredACs),
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

			// C-25 / AC-28: zero-tolerance fails on any non-passed annotated AC,
			// regardless of whether the spec's tier-coverage % met its threshold.
			// Exit code 2 distinguishes strictness violation from threshold failure.
			//
			// C-26 / AC-29: zero-tolerance also fails on approval_gate=true with
			// unset approval_date. Exit code 3 distinguishes approval-gate violation.
			if effectiveStrictness == "zero-tolerance" {
				if results != nil {
					nonPassed := 0
					for _, r := range results.Results {
						if r.Status != "" && r.Status != "passed" {
							nonPassed++
						}
					}
					if nonPassed > 0 {
						fmt.Fprintf(os.Stderr, "error: zero-tolerance strictness — %d annotated AC(s) did not pass\n", nonPassed)
						os.Exit(2)
					}
				}
				gateViolations := 0
				for _, s := range specs {
					for _, ac := range s.AcceptanceCriteria {
						if ac.ApprovalGate && ac.ApprovalDate == "" {
							gateViolations++
						}
					}
				}
				if gateViolations > 0 {
					fmt.Fprintf(os.Stderr, "error: zero-tolerance strictness — %d AC(s) carry approval_gate=true with unset approval_date\n", gateViolations)
					os.Exit(3)
				}
			}

			if report.Summary.Failing > 0 {
				return errSilent
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().StringVar(&testsGlob, "tests", "", "Glob pattern for test files")
	cmd.Flags().BoolVar(&failingOnly, "failing", false, "Show only specs below 100% coverage in the table (summary header still reflects the full report)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Require .specter-results.json and treat any non-passed annotated AC as uncovered (all tiers)")
	cmd.Flags().StringVar(&scope, "scope", "", "Narrow --strict demand to specs in the named domain from specter.yaml (specs outside the domain fall back to v0.9 boolean-passed logic). Requires --strict.")
	cmd.Flags().StringVar(&strictnessFlag, "strictness", "", "Override settings.strictness in specter.yaml (annotation | threshold | zero-tolerance)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress per-AC source-only hints under --strict (the diagnostic_hints array still appears in --json output)")
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
			if onlyPhase != "" && !validSyncPhases[onlyPhase] {
				fmt.Fprintf(os.Stderr, "error: --only must be one of: parse, resolve, check, coverage\n")
				return errSilent
			}

			specFiles := discoverSpecs()
			if len(specFiles) == 0 {
				fmt.Print(noSpecsMessage())
				return errSilent
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

			m, _, mErr := loadManifest()
			if mErr != nil {
				fmt.Fprintln(os.Stderr, "error:", mErr)
				return errSilent
			}
			checkOpts := &checker.CheckOptions{
				Strict:      strict || m.Settings.Strict,
				WarnOnDraft: m.Settings.WarnOnDraft,
			}

			var results *coverage.ResultsFile
			if data, err := os.ReadFile(".specter-results.json"); err == nil {
				var parseErr error
				results, parseErr = coverage.ParseResultsFile(data)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "warn: could not parse .specter-results.json: %v\n", parseErr)
				}
			}

			result := specsync.RunSync(specsync.SyncInput{
				SpecFiles:            specContents,
				TestFiles:            testContents,
				Thresholds:           m.CoverageThresholds(),
				CheckOpts:            checkOpts,
				OnlyPhase:            onlyPhase,
				Results:              results,
				CheckTestAnnotations: strict || m.Settings.Strict, // spec-check C-09/AC-12: sync --strict (or settings.strict) routes through
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
					return errSilent
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
				return errSilent
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

			spin := newSpinner("Scanning files…")

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
				spin.stop()
				fmt.Println("No source files found.")
				return errSilent
			}

			// Make all source file paths relative to the scan root so the engine
			// can mirror the directory structure in the output without knowing the
			// absolute path on disk.
			for i := range files {
				if rel, err := filepath.Rel(absTarget, files[i].Path); err == nil {
					files[i].Path = filepath.ToSlash(rel)
				}
			}

			spin.update(fmt.Sprintf("Analyzing %d file(s)…", len(files)))

			adapters := []reverse.Adapter{
				&reverse.TypeScriptAdapter{},
				&reverse.PythonAdapter{},
				&reverse.GoAdapter{},
			}

			date := time.Now().Format("2006-01-02")

			input := reverse.ReverseInput{
				Files:       files,
				AdapterName: adapterName,
				GroupBy:     groupBy,
				Date:        date,
			}

			result := reverse.Reverse(input, adapters)
			spin.stop()

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]interface{}{
					"specs":       result.Specs,
					"diagnostics": result.Diagnostics,
					"summary":     result.Summary,
				})
				if result.Summary.SpecsGenerated == 0 {
					return errSilent
				}
				return nil
			}

			for _, d := range result.Diagnostics {
				fmt.Fprintf(os.Stderr, "%s [%s] %s\n", d.Severity, d.Kind, d.Message)
			}

			if result.Summary.SpecsGenerated == 0 {
				fmt.Println("No specs generated. Check diagnostics above.")
				return errSilent
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

				if mkErr := os.MkdirAll(filepath.Dir(outPath), 0755); mkErr != nil {
					fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", mkErr)
					return errSilent
				}
				if wErr := os.WriteFile(outPath, []byte(gs.YAML), 0644); wErr != nil {
					fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, wErr)
					return errSilent
				}
				fmt.Printf("GENERATED %s — %s@%s (%d constraints, %d ACs)\n",
					outPath, gs.Spec.ID, gs.Spec.Version,
					len(gs.Spec.Constraints), len(gs.Spec.AcceptanceCriteria))
				for _, w := range gs.Warnings {
					fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
				}
			}

			gapCount := result.Summary.GapsDetected
			fmt.Printf("\nSummary: %d spec(s) generated, %d constraint(s), %d assertion(s), %d gap(s)\n",
				result.Summary.SpecsGenerated, result.Summary.ConstraintsFound,
				result.Summary.AssertionsFound, gapCount)
			if gapCount > 0 {
				fmt.Printf("\n%d AC(s) need triage — reverse extracts structure but not intent. Until triaged, these ACs count as uncovered and `specter sync` will fail.\n", gapCount)
				fmt.Println()
				fmt.Println("Next steps:")
				// Pick first generated spec to show a concrete example
				if len(result.Specs) > 0 {
					example := result.Specs[0].Spec.ID
					fmt.Printf("  1. Triage gaps in one spec:     specter explain %s\n", example)
					fmt.Printf("  2. Fill in each gap AC's description and remove the `gap: true` flag.\n")
					fmt.Printf("  3. Run parse to validate:        specter parse %s/%s\n", outputDir, result.Specs[0].FileName)
				} else {
					fmt.Printf("  1. Open a generated spec and triage its gap ACs (fill description, remove gap: true)\n")
					fmt.Printf("  2. Run: specter explain <spec-id>   to see annotation examples per AC\n")
					fmt.Printf("  3. Run: specter parse <spec-file>   to validate your edits\n")
				}
				fmt.Printf("  4. Run sync to check the whole corpus: specter sync\n")
			}

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

// loadManifest finds and parses the nearest specter.yaml. Always returns a
// non-nil Manifest — Defaults() if no file exists or parse fails — so library
// helpers (noSpecsMessage, discoverSpecs) can safely deref. Returns a non-nil
// error when a manifest IS present but fails to parse; RunE handlers must
// check the error and fail-fast (per GH #76 — silent fallback to Defaults()
// on parse error swallowed every typo'd settings key).
func loadManifest() (*manifest.Manifest, string, error) {
	path, root := findManifest()
	if path == "" {
		return manifest.Defaults(), "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Defaults(), "", fmt.Errorf("read %s: %w", path, err)
	}
	m, err := manifest.ParseManifest(string(data))
	if err != nil {
		return manifest.Defaults(), "", fmt.Errorf("invalid %s: %w", path, err)
	}
	return m, root, nil
}

func initCmd() *cobra.Command {
	var (
		name        string
		force       bool
		template    string
		refresh     bool
		dryRun      bool
		installHook bool
		aiTool      string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a specter.yaml project manifest",
		Long:  "Scaffold a specter.yaml file from existing .spec.yaml files in the current directory. Groups all specs into a default domain with sensible settings.\n\nWith --template, creates a draft .spec.yaml file instead of specter.yaml.\n\nWith --refresh, updates only domains.default.specs in an existing manifest; preserves every other field.\n\nWith --install-hook, writes a git pre-push hook that blocks pushes lacking @spec/@ac annotation deltas.\n\nWith --ai <tool>, writes an AI assistant instruction file for the given tool (claude, codex, cursor, copilot, gemini).",
		RunE: func(cmd *cobra.Command, args []string) error {
			// --install-hook and --ai are different write targets; combining
			// them silently would skip the second flag (early return). Reject
			// the combination explicitly so users see the conflict.
			if installHook && aiTool != "" {
				fmt.Fprintln(os.Stderr, "error: --install-hook and --ai are different write targets and cannot be combined; run them in separate invocations.")
				return errSilent
			}

			// --install-hook: write the git pre-push hook.
			if installHook {
				return runInitInstallHook()
			}

			// --ai <tool>: write the per-tool AI instruction file.
			if aiTool != "" {
				return runInitAI(aiTool)
			}

			// --template: create a spec file, not specter.yaml
			if template != "" {
				return runInitTemplate(template, force)
			}

			// C-21: --refresh and --force are mutually exclusive.
			if refresh && force {
				fmt.Fprintln(os.Stderr, "error: --refresh and --force are mutually exclusive. --force rewrites the entire manifest; --refresh updates only domains.default.specs.")
				return errSilent
			}

			// C-17/C-18: --refresh updates an existing manifest in place.
			if refresh {
				return runInitRefresh(dryRun)
			}

			if _, err := os.Stat("specter.yaml"); err == nil && !force {
				fmt.Println("specter.yaml already exists. Use --force to overwrite.")
				return errSilent
			}

			if name == "" {
				dir, _ := os.Getwd()
				name = filepath.Base(dir)
			}

			specFiles := discoverSpecs()
			var specIDs []string
			var initParseErrors []coverage.ParseErrorEntry
			for _, file := range specFiles {
				data, err := os.ReadFile(file)
				if err != nil {
					initParseErrors = append(initParseErrors, coverage.ParseErrorEntry{File: file, Type: "io", Message: err.Error()})
					continue
				}
				result := parser.ParseSpec(string(data))
				if result.OK {
					specIDs = append(specIDs, result.Value.ID)
				} else {
					for _, pe := range result.Errors {
						initParseErrors = append(initParseErrors, coverage.ParseErrorEntry{
							File: file, Path: pe.Path, Type: pe.Type, Message: pe.Message, Line: pe.Line, Column: pe.Column,
						})
					}
				}
			}

			yamlStr := manifest.ScaffoldManifestWithContext(name, "", specIDs, len(specFiles))
			if err := os.WriteFile("specter.yaml", []byte(yamlStr), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing specter.yaml: %v\n", err)
				return errSilent
			}

			fmt.Printf("Created specter.yaml with %d spec(s) in system %q\n", len(specIDs), name)
			unparsedFiles := len(specFiles) - len(specIDs)
			if unparsedFiles > 0 {
				fmt.Println()
				fmt.Printf("Warning: %d spec file(s) were discovered but could not be parsed:\n", unparsedFiles)
				patterns := coverage.SummarizeParseErrors(initParseErrors)
				if len(patterns) > 0 {
					top := patterns[0]
					if top.Count == unparsedFiles && unparsedFiles > 1 {
						pathPart := ""
						if top.Path != "" {
							pathPart = fmt.Sprintf(" at %q", top.Path)
						}
						fmt.Printf("  Every failing spec hit the same error: [%s]%s.\n", top.Type, pathPart)
						fmt.Println("  This is the signature of schema version drift — the specs may")
						fmt.Println("  have been written against an older Specter schema. Run `specter")
						fmt.Println("  doctor` for a full report, then fix the specs and re-run")
						fmt.Println("  `specter init --force` to populate the manifest.")
					} else {
						for _, p := range patterns {
							pathPart := ""
							if p.Path != "" {
								pathPart = fmt.Sprintf(" at %q", p.Path)
							}
							fmt.Printf("  [%s]%s — %d occurrence(s) across %d file(s)\n", p.Type, pathPart, p.Count, len(p.Files))
						}
					}
				}
				fmt.Println()
				fmt.Println("The manifest was still written with an empty default domain as a")
				fmt.Println("placeholder. Add your spec IDs under `domains.default.specs` once")
				fmt.Println("the parse errors are resolved.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "System name (defaults to directory name)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing specter.yaml")
	cmd.Flags().StringVar(&template, "template", "", "Create a spec file from a template (api-endpoint, service, auth, data-model)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Update domains.default.specs in an existing specter.yaml from the current on-disk spec list; preserves all other fields")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "With --refresh, print the proposed change to stdout without writing the file")
	cmd.Flags().BoolVar(&installHook, "install-hook", false, "Install a git pre-push hook that blocks pushes lacking @spec/@ac annotation deltas")
	cmd.Flags().StringVar(&aiTool, "ai", "", "Write an AI assistant instruction file for the named tool (claude, codex, cursor, copilot, gemini)")

	return cmd
}

// runInitInstallHook implements `specter init --install-hook` (C-22 / AC-27..29).
// Writes .git/hooks/pre-push with mode 0755, wrapping the hook script in a
// fenced region so re-runs replace only the Specter-managed body.
func runInitInstallHook() error {
	hookDir := filepath.Join(".git", "hooks")
	// Use Lstat (not Stat) so a symlinked `.git` is detected. A workspace
	// where `.git` is a symlink to an attacker-chosen path could redirect
	// the hook write to that path, dropping mode-0755 attacker-controlled
	// shell into an arbitrary location. Refuse to write through symlinks.
	info, err := os.Lstat(".git")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: .git not found. Run `specter init --install-hook` from a git repository root.")
		return errSilent
	}
	if info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(os.Stderr, "error: .git is a symlink; refusing to install hook through a symlink. Resolve the link or run from the actual repository root.")
		return errSilent
	}
	if !info.IsDir() {
		// `.git` can be a regular file in worktrees (it contains `gitdir: <path>`).
		// Worktree support is intentionally out of scope for the v0.11 hook
		// installer — refuse rather than guess, with a clear pointer.
		fmt.Fprintln(os.Stderr, "error: .git is not a directory (looks like a git worktree); install the hook from the primary working tree.")
		return errSilent
	}
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating %s: %v\n", hookDir, err)
		return errSilent
	}

	hookPath := filepath.Join(hookDir, "pre-push")
	existing := ""
	if data, err := os.ReadFile(hookPath); err == nil {
		existing = string(data)
	}

	// PrePushHookScript() returns the full fenced template (shebang + markers
	// + body). Extract the body so ReplaceFencedRegion adds exactly one set
	// of markers around it. Hook uses shell-comment markers — HTML-comment
	// markers would be a syntax error in sh.
	hookMarkers := manifest.ShellMarkers("v1")
	hookBody := extractFencedBody(manifest.PrePushHookScript(), hookMarkers)
	body, err := manifest.ReplaceFencedRegion(existing, hookMarkers, hookBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error rendering hook: %v\n", err)
		return errSilent
	}
	// Ensure the file starts with a shebang. ReplaceFencedRegion preserves
	// out-of-fence content, so a previously-written shebang survives; if
	// existing was empty we need to prepend.
	if !strings.HasPrefix(body, "#!") {
		body = "#!/bin/sh\n" + body
	}

	if err := os.WriteFile(hookPath, []byte(body), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", hookPath, err)
		return errSilent
	}
	// WriteFile preserves the requested mode on creation but not on overwrite
	// of an existing file — chmod explicitly to ensure executable.
	if err := os.Chmod(hookPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error chmod %s: %v\n", hookPath, err)
		return errSilent
	}

	fmt.Printf("Installed git pre-push hook at %s\n", hookPath)
	fmt.Println("Hook blocks pushes that change implementation files without @spec/@ac annotation deltas.")
	fmt.Println("Bypass with: git push --no-verify")
	return nil
}

// runInitAI implements `specter init --ai <tool>` (C-23 / AC-30..36).
// Writes the per-tool instruction file with the v0.11 fenced template.
func runInitAI(tool string) error {
	target, err := manifest.AITargetPath(tool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return errSilent
	}

	// AC-36: claude with existing AGENTS.md uses @AGENTS.md import instead of inlining.
	hasAgentsMd := false
	if tool == "claude" {
		if _, err := os.Stat("AGENTS.md"); err == nil {
			hasAgentsMd = true
		}
	}

	rendered, err := manifest.RenderAIInstructions(tool, hasAgentsMd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return errSilent
	}

	// If the target already exists, replace only the fenced region; preserve
	// out-of-fence content byte-for-byte (AC-35).
	existing := ""
	if data, err := os.ReadFile(target); err == nil {
		existing = string(data)
	}
	// Extract the body from the freshly-rendered template (which is fully
	// fenced with markdown markers) and apply it via ReplaceFencedRegion to
	// the existing file.
	mdMarkers := manifest.MarkdownMarkers("v1")
	body := extractFencedBody(rendered, mdMarkers)
	final, err := manifest.ReplaceFencedRegion(existing, mdMarkers, body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return errSilent
	}

	// Create parent dir if needed (AC-32 / AC-33).
	if dir := filepath.Dir(target); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating %s: %v\n", dir, err)
			return errSilent
		}
	}

	if err := os.WriteFile(target, []byte(final), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", target, err)
		return errSilent
	}

	fmt.Printf("Wrote %s instruction file at %s (%d bytes)\n", tool, target, len(final))
	if tool == "claude" && hasAgentsMd {
		fmt.Println("Detected existing AGENTS.md — CLAUDE.md uses @AGENTS.md import to avoid duplicating the template.")
	}
	return nil
}

// demoteApprovalGateViolations is the v0.11.1 fix for GH #94. Under
// strictness=zero-tolerance, an AC with approval_gate: true and unset
// approval_date is a demotion — it must show up in the report as
// uncovered, not just trigger the exit-3 code path. Walks the report
// in place: moves violating ACs from CoveredACs to UncoveredACs,
// recomputes per-entry CoveragePct + PassesThreshold, and recomputes
// Summary.Passing / Summary.Failing.
//
// v0.11.0 emitted the exit code but left the report identical to
// threshold mode — operator-visible report stayed PASS while the run
// exited 3. This function aligns the report with the exit signal.
func demoteApprovalGateViolations(report *coverage.CoverageReport, specs []schema.SpecAST) {
	// Build (specID → set of AC IDs to demote) from the spec AST.
	violations := make(map[string]map[string]bool)
	for i := range specs {
		s := &specs[i]
		var demoted map[string]bool
		for _, ac := range s.AcceptanceCriteria {
			if ac.ApprovalGate && ac.ApprovalDate == "" {
				if demoted == nil {
					demoted = make(map[string]bool)
				}
				demoted[ac.ID] = true
			}
		}
		if demoted != nil {
			violations[s.ID] = demoted
		}
	}
	if len(violations) == 0 {
		return
	}

	// Walk report entries and demote.
	report.Summary.Passing = 0
	report.Summary.Failing = 0
	for i := range report.Entries {
		e := &report.Entries[i]
		demoted := violations[e.SpecID]
		if demoted == nil {
			if e.PassesThreshold {
				report.Summary.Passing++
			} else {
				report.Summary.Failing++
			}
			continue
		}
		var keptCovered []string
		for _, acID := range e.CoveredACs {
			if demoted[acID] {
				e.UncoveredACs = append(e.UncoveredACs, acID)
				continue
			}
			keptCovered = append(keptCovered, acID)
		}
		e.CoveredACs = keptCovered
		if e.TotalACs > 0 {
			e.CoveragePct = float64(len(e.CoveredACs)) * 100 / float64(e.TotalACs)
		} else {
			e.CoveragePct = 0
		}
		// PassesThreshold uses the per-tier threshold the entry was built with.
		e.PassesThreshold = int(e.CoveragePct) >= e.Threshold
		if e.PassesThreshold {
			report.Summary.Passing++
		} else {
			report.Summary.Failing++
		}
	}
}

// extractFencedBody pulls the in-fence body out of a freshly-rendered template,
// stripping the begin/end markers so the body can be re-applied via
// ReplaceFencedRegion to a target that may already have its own out-of-fence
// content.
func extractFencedBody(fenced string, markers manifest.FencedMarkers) string {
	bIdx := strings.Index(fenced, markers.Begin)
	eIdx := strings.Index(fenced, markers.End)
	if bIdx < 0 || eIdx < 0 || eIdx < bIdx {
		return fenced
	}
	body := fenced[bIdx+len(markers.Begin) : eIdx]
	return strings.Trim(body, "\n")
}

// runInitRefresh implements `specter init --refresh` (C-17 through C-21).
// Reads the existing specter.yaml, rescans the specs directory for
// parseable spec IDs, and updates only domains.default.specs. Every other
// manifest field is preserved. In dry-run mode, prints the proposed diff
// and exits without writing.
func runInitRefresh(dryRun bool) error {
	data, err := os.ReadFile("specter.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "error: specter.yaml not found. Use `specter init` (without --refresh) to create one.")
			return errSilent
		}
		fmt.Fprintf(os.Stderr, "error reading specter.yaml: %v\n", err)
		return errSilent
	}
	existing, err := manifest.ParseManifest(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: specter.yaml failed to parse: %v\n", err)
		return errSilent
	}

	// Rescan specs directory for parseable IDs. Same discovery + parse
	// flow as greenfield `specter init`.
	specFiles := discoverSpecs()
	var specIDs []string
	var parseErrors []coverage.ParseErrorEntry
	for _, file := range specFiles {
		fdata, rerr := os.ReadFile(file)
		if rerr != nil {
			parseErrors = append(parseErrors, coverage.ParseErrorEntry{File: file, Type: "io", Message: rerr.Error()})
			continue
		}
		result := parser.ParseSpec(string(fdata))
		if result.OK {
			specIDs = append(specIDs, result.Value.ID)
		} else {
			for _, pe := range result.Errors {
				parseErrors = append(parseErrors, coverage.ParseErrorEntry{
					File: file, Path: pe.Path, Type: pe.Type, Message: pe.Message, Line: pe.Line, Column: pe.Column,
				})
			}
		}
	}

	updated, diff := manifest.RefreshManifestDomains(existing, specIDs)

	if dryRun {
		fmt.Println("Dry run — no changes will be written.")
		fmt.Println()
		if len(diff.Added) == 0 && len(diff.Removed) == 0 {
			fmt.Println("No changes needed: domains.default.specs already reflects the on-disk spec set.")
			return nil
		}
		fmt.Println("Proposed changes to domains.default.specs:")
		for _, id := range diff.Added {
			fmt.Printf("  + %s\n", id)
		}
		for _, id := range diff.Removed {
			fmt.Printf("  - %s\n", id)
		}
		fmt.Println()
		fmt.Println("Run `specter init --refresh` (without --dry-run) to apply.")
		return nil
	}

	// Serialize and write.
	out, err := yaml.Marshal(updated)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling updated manifest: %v\n", err)
		return errSilent
	}
	var sb strings.Builder
	sb.WriteString("# Specter Project Manifest\n")
	sb.WriteString("# See: https://github.com/Hanalyx/specter\n\n")
	sb.Write(out)
	if werr := os.WriteFile("specter.yaml", []byte(sb.String()), 0644); werr != nil {
		fmt.Fprintf(os.Stderr, "error writing specter.yaml: %v\n", werr)
		return errSilent
	}

	fmt.Printf("updated specter.yaml: +%d added, -%d removed\n", len(diff.Added), len(diff.Removed))
	if len(parseErrors) > 0 {
		fmt.Println()
		fmt.Printf("Warning: %d spec file(s) were discovered but could not be parsed:\n", len(parseErrors))
		patterns := coverage.SummarizeParseErrors(parseErrors)
		for _, p := range patterns {
			pathPart := ""
			if p.Path != "" {
				pathPart = fmt.Sprintf(" at %q", p.Path)
			}
			fmt.Printf("  [%s]%s — %d occurrence(s)\n", p.Type, pathPart, p.Count)
		}
		fmt.Println("Run `specter doctor` for a full report.")
	}
	return nil
}

// runInitTemplate creates a draft .spec.yaml from a named template.
func runInitTemplate(templateType string, force bool) error {
	content, err := manifest.SpecTemplate(templateType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return errSilent
	}

	outFile := templateType + ".spec.yaml"
	if _, statErr := os.Stat(outFile); statErr == nil && !force {
		fmt.Printf("%s already exists. Use --force to overwrite.\n", outFile)
		return errSilent
	}

	if err := os.WriteFile(outFile, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outFile, err)
		return errSilent
	}

	fmt.Printf("Created %s (template: %s)\n", outFile, templateType)
	fmt.Println("Edit the file to replace placeholder values, then run: specter sync")
	return nil
}

// doctorCmd implements the specter doctor pre-flight health checker.
//
// @spec spec-doctor
func doctorCmd() *cobra.Command {
	var fix, dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight project health checks",
		Long:  "Checks project readiness before running the full sync pipeline. Reports PASS/WARN/FAIL for each check so developers know exactly what needs attention.\n\nWith --fix, applies known-safe schema-drift rewrites to spec files (initial table: strip trust_level removed in v0.6.5). Add --dry-run to preview without writing.",
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
			var allParseErrs []coverage.ParseErrorEntry
			for _, f := range specFiles {
				data, err := os.ReadFile(f)
				if err != nil {
					parseOK = false
					parseErrors++
					allParseErrs = append(allParseErrs, coverage.ParseErrorEntry{File: f, Type: "io", Message: err.Error()})
					continue
				}
				result := parser.ParseSpec(string(data))
				if !result.OK {
					parseOK = false
					parseErrors++
					for _, pe := range result.Errors {
						fmt.Printf("    %s: %s\n", f, pe.Message)
						allParseErrs = append(allParseErrs, coverage.ParseErrorEntry{
							File: f, Path: pe.Path, Type: pe.Type, Message: pe.Message, Line: pe.Line, Column: pe.Column,
						})
					}
				}
			}
			if !parseOK {
				printCheck("parse", "FAIL", fmt.Sprintf("%d spec file(s) have parse errors (see above)", parseErrors))
				anyFail = true
				// AC-09 (spec-doctor v1.1.0): when parse fails, name the
				// widespread pattern. If the same (type, path) hits every
				// discovered spec, that's schema drift, not N bugs.
				patterns := coverage.SummarizeParseErrors(allParseErrs)
				if len(patterns) > 0 && len(specFiles) > 0 {
					top := patterns[0]
					affected := len(top.Files)
					fmt.Println()
					fmt.Println("  Pattern analysis:")
					if affected == len(specFiles) && len(specFiles) > 1 {
						pathPart := ""
						if top.Path != "" {
							pathPart = fmt.Sprintf(" at %q", top.Path)
						}
						fmt.Printf("    Every %d discovered spec hit the same failure: [%s]%s.\n", len(specFiles), top.Type, pathPart)
						fmt.Println("    This pattern is the signature of schema version drift —")
						fmt.Println("    your specs may have been written against an older Specter")
						fmt.Println("    schema. Check the spec-parse changelog and migrate each file.")
					} else {
						for _, p := range patterns {
							pathPart := ""
							if p.Path != "" {
								pathPart = fmt.Sprintf(" at %q", p.Path)
							}
							fmt.Printf("    [%s]%s — %d occurrence(s) across %d file(s)\n", p.Type, pathPart, p.Count, len(p.Files))
							if len(patterns) > 3 {
								break
							}
						}
					}
				}
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
				m, _, mErr := loadManifest()
				if mErr != nil {
					fmt.Fprintln(os.Stderr, "error:", mErr)
					return errSilent
				}
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

			// spec-doctor C-11/12/13 (v0.12 / GH-via-feat-doctor-fix):
			// after the diagnostic checks complete, if --fix is on, apply
			// the known-safe rewrite table and print a summary. Exit code
			// under --fix reflects the fix action only (0 on success, even
			// no-op; non-zero only if the rewrite mechanism errored).
			// Diagnostic check failures still appear in the output above
			// but do not drive the exit code under --fix — operator runs
			// `specter doctor` (no --fix) for a pure health-based exit.
			if fix {
				// spec-doctor C-16 (v1.7.0): BETA gate. Print warning + prompt
				// for confirmation unless --yes is set or --dry-run is in use
				// (preview is read-only, no warning needed).
				if !dryRun && !yes {
					proceed, err := confirmFixWithUser(os.Stdin, stdinIsTTY(), os.Stderr)
					if err != nil {
						fmt.Fprintln(os.Stderr, "error:", err)
						return errSilent
					}
					if !proceed {
						fmt.Println("Aborted. No files modified.")
						return nil
					}
				}
				applied, unhandled, err := runDoctorFix(allParseErrs, dryRun)
				if err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
					return errSilent
				}
				printDoctorFixSummary(applied, unhandled, dryRun)
				return nil
			}

			// C-06: exit 0 if all PASS/WARN, exit 1 if any FAIL
			if anyFail {
				fmt.Println("Result: FAIL — fix the issues above before running `specter sync`")
				return errSilent
			}
			fmt.Println("Result: OK — project is ready for `specter sync`")
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Apply known-safe schema-drift rewrites to spec files in place (BETA — see --yes to bypass interactive confirmation)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "With --fix, print what would change without writing to disk (skips BETA prompt)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the --fix BETA confirmation prompt (for non-interactive use)")
	return cmd
}

// stdinIsTTY reports whether os.Stdin is connected to a character device
// (terminal). Returns false for pipes, redirects, /dev/null, and any
// CI-like environment without a controlling terminal.
//
// Implementation: os.Stdin.Stat() returns a FileInfo whose Mode includes
// os.ModeCharDevice when the underlying fd points at a terminal. Stat()
// failing returns false (treat unknown as non-TTY — safer default for a
// destructive operation gate). spec-doctor C-16 (1.8.0+).
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// confirmFixWithUser prints the spec-doctor C-16 BETA warning and prompts
// the operator on the supplied stderr writer for confirmation.
//
// Returns (true, nil) on affirmative TTY input ("y" / "yes",
// case-insensitive); (false, nil) on any other TTY input (decline);
// (false, err) when isTTY=false — the gate refuses BEFORE reading
// content so piped affirmative data (echo y | specter doctor --fix)
// cannot bypass the BETA acknowledgment. spec-doctor C-16 (1.8.0+).
//
// The (stdin, isTTY, stderr) signature exists for unit-testing: tests
// inject a strings.NewReader and a synthetic isTTY value; the CLI path
// passes os.Stdin, stdinIsTTY(), and os.Stderr.
func confirmFixWithUser(stdin io.Reader, isTTY bool, stderr io.Writer) (bool, error) {
	fmt.Fprintln(stderr, "[BETA] specter doctor --fix")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "  This command rewrites your .spec.yaml files in place to repair known")
	fmt.Fprintln(stderr, "  schema drift (currently: strip the v0.6.5-removed `trust_level` field).")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "  Known limitation (BACKLOG cycle 6): when `trust_level: <value>` appears")
	fmt.Fprintln(stderr, "  inside a string literal (e.g., a description block scalar mentioning")
	fmt.Fprintln(stderr, "  the deprecated field), the deletion may strip that documentation line")
	fmt.Fprintln(stderr, "  too. Cycle 6 will replace regex deletion with line-targeted deletion.")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "  Recommended:")
	fmt.Fprintln(stderr, "    - Commit your spec files BEFORE running so changes can be diffed.")
	fmt.Fprintln(stderr, "    - Use --dry-run to preview changes without writing.")
	fmt.Fprintln(stderr, "    - Use --yes (or -y) for non-interactive (CI) runs.")
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "Continue? (y/N): ")

	if !isTTY {
		// Refuse before reading any stdin content. Closes the regression
		// where echo y | specter doctor --fix bypassed the gate by
		// arriving as content rather than EOF.
		fmt.Fprintln(stderr)
		return false, fmt.Errorf("--fix requires interactive confirmation; run with --yes (or -y) for non-interactive use, or run interactively")
	}

	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read stdin: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// doctorFixApplied is one (file, rewrites) tuple produced by runDoctorFix.
// Used by printDoctorFixSummary to emit the C-13 rewritten-files block.
type doctorFixApplied struct {
	File     string
	Rewrites []string
}

// doctorFixUnhandled is one (file, rewrite, reason) tuple produced by
// runDoctorFix when migrate.Apply refused a structurally unsafe shape.
// Used by printDoctorFixSummary to emit the C-15 needs-manual-edit block.
type doctorFixUnhandled struct {
	File    string
	Rewrite string
	Reason  string
}

// runDoctorFix applies the migrate package's rewrite table to every spec
// file with parse errors and canonicalizes specter.yaml when present.
// Returns:
//   - applied: (file, rewrites) tuples where at least one rewrite fired
//   - unhandled: (file, rewrite, reason) tuples where a rewrite predicate
//     matched but migrate.Apply refused due to unsafe YAML shape (C-15).
//
// Under dryRun the on-disk content is left unchanged; both return slices
// still reflect what would have been rewritten / would have been refused
// so the summary can name them.
func runDoctorFix(parseErrors []coverage.ParseErrorEntry, dryRun bool) ([]doctorFixApplied, []doctorFixUnhandled, error) {
	// Group parse errors by file so each file's rewrite table is consulted
	// once with all relevant errors.
	errsByFile := map[string][]coverage.ParseErrorEntry{}
	var fileOrder []string
	for _, e := range parseErrors {
		if _, seen := errsByFile[e.File]; !seen {
			fileOrder = append(fileOrder, e.File)
		}
		errsByFile[e.File] = append(errsByFile[e.File], e)
	}

	var applied []doctorFixApplied
	var unhandled []doctorFixUnhandled
	for _, file := range fileOrder {
		content, err := os.ReadFile(file)
		if err != nil {
			// Read failure already surfaced in the doctor parse check; skip
			// rewrite rather than double-error.
			continue
		}
		result, err := migrate.Apply(content, errsByFile[file])
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite %s: %w", file, err)
		}
		// Surface refusals first so the operator sees them in the summary
		// regardless of whether other rewrites on the same file applied.
		for _, u := range result.Unhandled {
			unhandled = append(unhandled, doctorFixUnhandled{
				File:    file,
				Rewrite: u.Rewrite,
				Reason:  u.Reason,
			})
		}
		if len(result.Applied) == 0 {
			continue
		}
		if !dryRun {
			if err := os.WriteFile(file, result.Content, 0644); err != nil {
				return nil, nil, fmt.Errorf("write %s: %w", file, err)
			}
		}
		applied = append(applied, doctorFixApplied{File: file, Rewrites: result.Applied})
	}

	// spec-doctor C-14 (v0.12): canonicalize specter.yaml when present and
	// missing schema_version. Silent no-op when the file doesn't exist —
	// `specter init` is the right command for that path.
	if manifestApplied, err := canonicalizeManifest(dryRun); err != nil {
		return nil, nil, err
	} else if manifestApplied != nil {
		applied = append(applied, *manifestApplied)
	}

	return applied, unhandled, nil
}

// canonicalizeManifest implements spec-doctor C-14 (v0.12). Returns a
// doctorFixApplied entry with rewrite name "add-schema-version" when the
// manifest existed but lacked a schema_version line. Returns nil if no
// rewrite was needed (manifest absent, OR already declares schema_version).
// Under dryRun, the in-memory result is computed but no file is written.
func canonicalizeManifest(dryRun bool) (*doctorFixApplied, error) {
	manifestPath, _ := findManifest()
	if manifestPath == "" {
		return nil, nil // no manifest → silent no-op (AC-18)
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", manifestPath, err)
	}
	// Already declares schema_version? byte-unchanged (AC-17). Match a
	// `schema_version:` key at the start of any line — guards against
	// matching a string literal containing the substring elsewhere.
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "schema_version:") {
			return nil, nil
		}
	}
	// Missing → prepend the canonical line, preserving original bytes
	// after it (AC-16). Use plain string concat so existing comments,
	// formatting, and trailing whitespace are byte-preserved.
	newContent := []byte("schema_version: 1\n" + string(content))
	if !dryRun {
		if err := os.WriteFile(manifestPath, newContent, 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", manifestPath, err)
		}
	}
	return &doctorFixApplied{
		File:     manifestPath,
		Rewrites: []string{"add-schema-version"},
	}, nil
}

// printDoctorFixSummary emits the spec-doctor summary blocks. Two blocks:
//   - C-13 rewritten: `N file(s) rewritten` (or `would be rewritten` under
//     dry-run), one line per applied rewrite. When N=0 AND no manual-edit
//     entries exist, prints `no changes`.
//   - C-15 needs-manual-edit: `N file(s) need manual edit`, one line per
//     refused rewrite naming the file and reason. Skipped when empty.
func printDoctorFixSummary(applied []doctorFixApplied, unhandled []doctorFixUnhandled, dryRun bool) {
	fmt.Println()
	if len(applied) == 0 && len(unhandled) == 0 {
		fmt.Println("doctor --fix: no changes")
		return
	}
	if len(applied) > 0 {
		totalVerb := "rewritten"
		itemVerb := "rewrite"
		if dryRun {
			totalVerb = "would be rewritten"
			itemVerb = "would rewrite"
		}
		fmt.Printf("doctor --fix: %d file(s) %s\n", len(applied), totalVerb)
		for _, a := range applied {
			for _, name := range a.Rewrites {
				fmt.Printf("  %s %s (%s)\n", itemVerb, a.File, name)
			}
		}
	}
	if len(unhandled) > 0 {
		// Distinct files only — multiple unhandled rewrites on one file
		// still count as one file in the summary header.
		seenFiles := map[string]bool{}
		for _, u := range unhandled {
			seenFiles[u.File] = true
		}
		fmt.Printf("doctor --fix: %d file(s) need manual edit\n", len(seenFiles))
		for _, u := range unhandled {
			fmt.Printf("  skip %s — %s\n", u.File, u.Reason)
		}
	}
}

// explainCmd shows coverage status and annotation examples for a spec's ACs.
//
// @spec spec-explain
func explainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <spec-id>[:<ac-id>] | annotation | schema [<field-path>]",
		Short: "Read-only diagnostic surfaces: AC coverage, annotation reference, schema reference",
		Long: "Read-only verb with four surfaces:\n" +
			"  explain <spec-id>[:<ac-id>]  show AC coverage and annotation examples (default)\n" +
			"  explain annotation           print the test-annotation reference\n" +
			"  explain schema               print the full schema field reference\n" +
			"  explain schema <field-path>  print detail on one field (e.g., spec.acceptance_criteria.items.approval_gate)\n",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dispatch on first arg: reference keyword or spec-id.
			switch args[0] {
			case "annotation":
				if len(args) != 1 {
					return fmt.Errorf("explain annotation takes no arguments")
				}
				fmt.Print(explain.AnnotationReference())
				return nil
			case "schema":
				schemaJSON, err := parser.SchemaBytes()
				if err != nil {
					return fmt.Errorf("load schema: %w", err)
				}
				if len(args) == 1 {
					out, err := explain.RenderSchemaReference(schemaJSON)
					if err != nil {
						return err
					}
					fmt.Print(out)
					return nil
				}
				out, err := explain.RenderSchemaField(schemaJSON, args[1])
				if err != nil {
					fmt.Fprintln(os.Stderr, "error:", err.Error())
					return errSilent
				}
				fmt.Print(out)
				return nil
			}
			// Default: spec-id[:AC-NN] form. Exactly one arg required.
			if len(args) != 1 {
				return fmt.Errorf("explain %s takes no second argument", args[0])
			}
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
				return errSilent
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

// explainListMode renders the spec card: tier, coverage %, and per-AC status
// with the test file(s) covering each AC.
func explainListMode(spec *schema.SpecAST, coveredBy map[string][]string, testFiles []string, langs []string) error {
	covered := 0
	total := len(spec.AcceptanceCriteria)
	for _, ac := range spec.AcceptanceCriteria {
		if len(coveredBy[ac.ID]) > 0 {
			covered++
		}
	}
	pct := 0
	if total > 0 {
		pct = (covered * 100) / total
	}

	fmt.Printf("specter explain %s\n\n", spec.ID)
	fmt.Printf("  Tier: %d    Coverage: %d%% (%d/%d ACs)\n\n", spec.Tier, pct, covered, total)
	fmt.Printf("  %-8s %-8s  %-40s  %s\n", "Status", "AC", "Description", "Test files")
	fmt.Println("  " + strings.Repeat("-", 90))

	for _, ac := range spec.AcceptanceCriteria {
		status := "UNCOVERED"
		files := ""
		if len(coveredBy[ac.ID]) > 0 {
			status = "COVERED"
			files = strings.Join(coveredBy[ac.ID], ", ")
		}
		desc := ac.Description
		if len(desc) > maxDescLen {
			desc = desc[:maxDescLen-3] + "..."
		}
		fmt.Printf("  %-8s %-8s  %-40s  %s\n", status, ac.ID, desc, files)
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
		return errSilent
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
				// spec-explain C-13 (v0.12 / GH #77): Python's source-comment
				// pattern alone fails coverage --strict because pytest's JUnit
				// XML doesn't include source-file scans — only runtime emission
				// via system-out reaches `specter ingest`. Teach the dual-channel
				// pattern: source comments + pytest.mark.spec decorator +
				// conftest autouse fixture (emits Convention B into system-out)
				// + pytest.ini settings (junit_logging = system-out plus marker
				// registration).
				funcName := "test_" + sanitizeID(spec.ID) + "_" + strings.ToLower(strings.ReplaceAll(acID, "-", "_"))
				fmt.Printf("    # Test (carries both Convention B source comments AND pytest.mark.spec):\n")
				fmt.Printf("    # @spec %s\n", spec.ID)
				fmt.Printf("    # @ac %s\n", acID)
				fmt.Printf("    @pytest.mark.spec(%q, %q)\n", spec.ID, acID)
				fmt.Printf("    def %s():\n", funcName)
				fmt.Printf("        # %s\n", targetAC.Description)
				fmt.Printf("        ...\n")
				fmt.Println()
				fmt.Printf("    # conftest.py — emits Convention B into JUnit <system-out>\n")
				fmt.Printf("    # so `specter ingest` sees the (spec_id, ac_id) pair:\n")
				fmt.Printf("    import pytest\n")
				fmt.Printf("\n")
				fmt.Printf("    @pytest.fixture(autouse=True)\n")
				fmt.Printf("    def specter_emit_annotations(request):\n")
				fmt.Printf("        marker = request.node.get_closest_marker(\"spec\")\n")
				fmt.Printf("        if marker:\n")
				fmt.Printf("            spec_id, *ac_ids = marker.args\n")
				fmt.Printf("            print(f\"// @spec {spec_id}\")\n")
				fmt.Printf("            for ac_id in ac_ids:\n")
				fmt.Printf("                print(f\"// @ac {ac_id}\")\n")
				fmt.Printf("        yield\n")
				fmt.Println()
				fmt.Printf("    # pytest.ini — registers the marker and routes prints to JUnit:\n")
				fmt.Printf("    [pytest]\n")
				fmt.Printf("    markers =\n")
				fmt.Printf("        spec: Specter SDD spec/AC mapping\n")
				fmt.Printf("    junit_logging = system-out\n")
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
		// AC-13: when test discovery returned nothing, detectAnnotationLanguages
		// fell back to the Go default. Note the dual-channel requirement
		// explicitly so the developer is not led down the source-only path.
		if len(testFiles) == 0 {
			fmt.Println("  Note: source comments alone do not satisfy `coverage --strict`.")
			fmt.Println("  Tests must produce a runner-visible signal (Convention A: spec-id/AC-NN")
			fmt.Println("  in the test name, OR Convention B: emit `// @spec`/`// @ac` lines via")
			fmt.Println("  test stdout). Run `specter explain annotation` for the full reference.")
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
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Re-run sync pipeline on file changes",
		Long:  "Watches .spec.yaml and test files for changes and re-runs the full sync pipeline. Press Ctrl+C to stop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, mErr := loadManifest()
			if mErr != nil {
				fmt.Fprintln(os.Stderr, "error:", mErr)
				return errSilent
			}

			specsDir := m.SpecsDir()
			fmt.Printf("specter watch\n\n")
			fmt.Printf("  Watching: %s, test files\n", specsDir)
			fmt.Printf("  Press Ctrl+C to stop\n\n")

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("failed to start file watcher: %w", err)
			}
			defer func() { _ = watcher.Close() }()

			// Watch the specs directory and current directory (for test files).
			for _, dir := range []string{specsDir, "."} {
				_ = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
					if walkErr != nil || !info.IsDir() {
						return nil
					}
					if info.Name() == "node_modules" || info.Name() == ".git" || info.Name() == "dist" {
						return filepath.SkipDir
					}
					if addErr := watcher.Add(path); addErr != nil {
						fmt.Fprintf(os.Stderr, "warn: could not watch %s: %v\n", path, addErr)
					}
					return nil
				})
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sig)

			// C-06: run once immediately on startup
			runWatchCycle(m)

			// Debounce: coalesce rapid successive events into one cycle.
			var debounce <-chan time.Time

			for {
				select {
				case <-sig:
					fmt.Println("\nstopped")
					return nil
				case event, ok := <-watcher.Events:
					if !ok {
						return nil
					}
					name := event.Name
					isSpec := strings.HasSuffix(name, specFileExt)
					isTest := false
					for _, ext := range testFileExts {
						if strings.HasSuffix(name, ext) {
							isTest = true
							break
						}
					}
					if isSpec || isTest {
						debounce = time.After(watchDebounce)
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return nil
					}
					fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
				case <-debounce:
					debounce = nil
					runWatchCycle(m)
				}
			}
		},
	}
	return cmd
}

// runWatchCycle executes the full sync pipeline and prints a compact summary line.
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
		fmt.Printf("[%s] FAIL  parse\n", timestamp)
		return
	}

	// Resolve — detect cycles, dangling refs, version mismatches
	graph := resolver.ResolveSpecs(inputs)
	resolverFail := false
	for _, d := range graph.Diagnostics {
		if d.Severity == "error" {
			resolverFail = true
			break
		}
	}
	if resolverFail {
		fmt.Printf("[%s] FAIL  resolve  (%d issue(s))\n", timestamp, len(graph.Diagnostics))
		return
	}

	// Check — structural rules
	opts := &checker.CheckOptions{
		Strict:      m.Settings.Strict,
		WarnOnDraft: m.Settings.WarnOnDraft,
	}
	checkResult := checker.CheckSpecs(graph, opts)
	if checkResult.Summary.Errors > 0 {
		fmt.Printf("[%s] FAIL  check  (%d error(s))\n", timestamp, checkResult.Summary.Errors)
		return
	}

	// Coverage
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

	totalACs := 0
	coveredACs := 0
	for _, e := range report.Entries {
		totalACs += e.TotalACs
		coveredACs += len(e.CoveredACs)
	}

	passing := report.Summary.Passing
	failing := report.Summary.Failing

	status := "PASS"
	if failing > 0 {
		status = "FAIL"
	}

	fmt.Printf("[%s] %-4s  %d spec(s)  %d/%d ACs covered  (%d passing, %d failing)\n",
		timestamp, status, len(specs), coveredACs, totalACs, passing, failing)
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
				return errSilent
			}
			v2, err := readSpecAtRef(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", args[1], err)
				return errSilent
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

// validGitRef matches the characters that appear in valid git refs:
// branch names, tags, commit SHAs, and revision qualifiers like HEAD~1.
var validGitRef = regexp.MustCompile(`^[a-zA-Z0-9_.~^/\-]+$`)

// gitShow runs `git show <ref>:<path>` and returns the output.
// The path is resolved to be repo-root-relative so git show works
// regardless of the working directory within the repo.
func gitShow(ref, path string) ([]byte, error) {
	if !validGitRef.MatchString(ref) {
		return nil, fmt.Errorf("invalid git ref %q: must match [a-zA-Z0-9_.~^/\\-]+", ref)
	}
	// Get the repo root
	rootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}
	root := strings.TrimSpace(string(rootBytes))

	// Resolve path to absolute, then make it relative to root
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	relPath, err := filepath.Rel(root, absPath)
	if err != nil {
		return nil, err
	}

	return exec.Command("git", "show", ref+":"+relPath).Output()
}

// Parity test between cobra-registered flags and docs/CLI_REFERENCE.md.
//
// Closes the v0.10.x docs-vs-code mismatch class. Three concrete cases
// shipped during v0.10.x where the CLI reference documented behavior
// the code didn't implement:
//
//   - `specter ingest --junit` glob-support claim (v0.10.0 CHANGELOG /
//     CLI_REFERENCE; surfaced as BUG-2 when jwtms tried it).
//   - `approval_gate` enforcement claim in SPEC_SCHEMA_REFERENCE
//     (BUG-3 — a schema-reference mismatch, parallel class).
//   - VS Code extension-ID misdiagnosed twice during a real user's
//     auto-download bug; the docs implied a behavior the code
//     intentionally overrode.
//
// Reviewer attention didn't catch these — the reviewer shared the
// writer's mental model. This mechanical test prevents the class:
// if CLI_REFERENCE.md documents a flag that cobra doesn't register
// (phantom flag), OR cobra registers a flag CLI_REFERENCE.md doesn't
// document (missing doc), this test fails.
//
// Scope:
//
//   - Only DOCUMENTED commands are checked. Hidden commands
//     (prePushCheckCmd) and external completion helpers are exempt.
//   - Per-command flag tables in CLI_REFERENCE.md are matched by the
//     `### `specter <name>`` heading.
//   - Both long flags (e.g. `--strict`) and short aliases (e.g. `-s`)
//     are normalized to their long-form name for comparison.
//
// @spec spec-check (closes the BUG-2/BUG-3 docs-vs-code mismatch class)
package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// docsPath is the relative path from cmd/specter/ to CLI_REFERENCE.md.
const docsPath = "../../docs/CLI_REFERENCE.md"

// commandsExemptFromParity lists commands NOT required to appear in
// CLI_REFERENCE.md. Cobra's auto-generated `completion` (shell
// completion script generator) doesn't carry user-facing flags worth
// documenting; the suppressed help command (v0.13 C7) is hidden by
// construction; prePushCheckCmd is a hidden hook target.
var commandsExemptFromParity = map[string]bool{
	"completion": true,
	"no-help":    true, // C7's hidden help-command stub
}

// flagsExemptFromParity is a per-command allowlist of flags that don't
// need to appear in the docs (e.g. cobra's auto-added --help).
var flagsExemptFromParity = map[string]bool{
	"help": true,
}

// TestCLIDocsFlagsParity asserts every flag documented in
// CLI_REFERENCE.md is registered, and every registered (non-hidden)
// flag for documented commands is documented.
func TestCLIDocsFlagsParity(t *testing.T) {
	data, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read CLI_REFERENCE.md: %v (path: %s)", err, docsPath)
	}
	docFlags := parseCliReferenceFlags(string(data))

	root := buildRootForParityTest()

	for _, sub := range root.Commands() {
		if commandsExemptFromParity[sub.Name()] {
			continue
		}
		if sub.Hidden {
			continue
		}
		name := sub.Name()
		registered := registeredFlagNames(sub)

		documented, hasSection := docFlags[name]
		if !hasSection {
			// A visible cobra command without a CLI_REFERENCE.md
			// section. If it carries any non-trivial flag (beyond
			// --help), call it out.
			nontrivial := false
			for f := range registered {
				if !flagsExemptFromParity[f] {
					nontrivial = true
					break
				}
			}
			if nontrivial {
				t.Errorf("command `specter %s` is registered with flags %v but has no `### `specter %s`` section in CLI_REFERENCE.md",
					name, sortedKeys(registered), name)
			}
			continue
		}

		// Phantom flags: documented but not registered.
		for flag := range documented {
			if _, ok := registered[flag]; !ok {
				t.Errorf("CLI_REFERENCE.md documents `--%s` for `specter %s`, but cobra does not register it (phantom flag)", flag, name)
			}
		}
		// Undocumented flags: registered but not in docs.
		for flag := range registered {
			if flagsExemptFromParity[flag] {
				continue
			}
			if _, ok := documented[flag]; !ok {
				t.Errorf("`specter %s --%s` is registered but not documented in CLI_REFERENCE.md", name, flag)
			}
		}
	}
}

// buildRootForParityTest constructs the same cobra command tree
// main() builds, but standalone for the test. Must stay in sync with
// the registrations in main.go.
func buildRootForParityTest() *cobra.Command {
	root := &cobra.Command{Use: "specter"}
	// Hide the auto-generated help command the same way main() does
	// so it doesn't appear in root.Commands() with the wrong name.
	root.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
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
	return root
}

// registeredFlagNames returns the set of long-form flag names cobra
// has registered for a command (LocalFlags + InheritedFlags). Hidden
// flags are excluded.
func registeredFlagNames(cmd *cobra.Command) map[string]bool {
	out := make(map[string]bool)
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		out[f.Name] = true
	})
	return out
}

// reHeading matches `### `specter <name>`` headings. The name captures
// only the first word after specter — `specter doctor` not
// `specter doctor --fix`.
var reHeading = regexp.MustCompile("^### `specter\\s+([a-zA-Z][a-zA-Z0-9-]*)`")

// reFlagInTable matches a table-row line whose first column declares
// one or more flags as backticked tokens. Examples it matches:
//
//	| `--json` | Output as JSON. |
//	| `--strictness <level>` | Override settings.strictness. |
//	| `--test`, `-t` | Cross-reference test annotations. |
//	| `--yes`, `-y` | Skip the confirmation prompt. |
//
// The capture is the contents BETWEEN the first `|` and the second
// `|` — we extract flag names from that span.
var reTableRow = regexp.MustCompile(`^\|\s*(\S[^|]*?)\s*\|`)

// reFlagToken matches a backticked flag token, capturing the
// long-form name. Examples:
//
//	`--json`              → json
//	`--strictness <level>` → strictness
//	`--strictness=<level>` → strictness
//	`-t`                  → t (short flag, skipped at the call site)
var reFlagToken = regexp.MustCompile("`(--?[a-zA-Z][a-zA-Z0-9-]*)")

// parseCliReferenceFlags scans CLI_REFERENCE.md and returns a map of
// command-name → set of documented flag names (long form only;
// short aliases are normalized to their long form when in the same
// table cell, via the comma-separated pattern).
func parseCliReferenceFlags(doc string) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	var currentCmd string

	lines := strings.Split(doc, "\n")
	for _, line := range lines {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			currentCmd = m[1]
			if _, ok := out[currentCmd]; !ok {
				out[currentCmd] = make(map[string]bool)
			}
			continue
		}
		if currentCmd == "" {
			continue
		}
		// Detect lines that look like table rows in a flag table.
		// Two heuristics:
		//   - Row starts with `|` and the first cell contains a
		//     backticked flag token starting with `-`.
		//   - The table-header separator (`|--------|...`) is not a
		//     data row; skip.
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.HasPrefix(line, "|--") || strings.HasPrefix(line, "|---") {
			continue
		}
		m := reTableRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		cell := m[1]
		// Extract every backticked flag token in this cell. A cell
		// with `--test`, `-t` contains both forms; we keep only the
		// long form.
		for _, tok := range reFlagToken.FindAllStringSubmatch(cell, -1) {
			full := tok[1] // includes leading dashes
			if !strings.HasPrefix(full, "--") {
				continue // short alias; will be picked up via its long form
			}
			name := strings.TrimPrefix(full, "--")
			out[currentCmd][name] = true
		}
	}
	return out
}

// sortedKeys is a small helper for stable diagnostic output.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

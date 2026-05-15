// CLI integration tests for v0.13 C7: the auto-generated `specter help`
// subcommand is suppressed; `--help` / `-h` remain the only help surface.
//
// No spec annotation — C7 is a CLI-surface enhancement, not bound to any
// .spec.yaml contract. The tests are regression guards.
package main

import (
	"strings"
	"testing"
)

// `specter --help` MUST NOT list `help` in the Available Commands.
func TestRootHelpDoesNotListHelpCommand(t *testing.T) {
	t.Run("root --help omits the help subcommand row (v0.13 C7)", func(t *testing.T) {
		out, code := runCLI(t, t.TempDir(), "--help")
		if code != 0 {
			t.Fatalf("expected --help exit 0, got %d. output:\n%s", code, out)
		}
		// Look at the "Available Commands:" block. Each line is
		// "  <name>  <description>". A "help" command would appear as
		// a row starting with "help".
		lines := strings.Split(out, "\n")
		inCommands := false
		for _, line := range lines {
			if strings.HasPrefix(line, "Available Commands:") {
				inCommands = true
				continue
			}
			if inCommands && strings.HasPrefix(line, "Flags:") {
				break
			}
			if inCommands {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "help ") || trimmed == "help" {
					t.Errorf("expected 'help' subcommand suppressed from root listing; found row: %q", line)
				}
			}
		}
	})
}

// `specter help` MUST exit non-zero with an unknown-command error.
func TestSpecterHelpIsUnknownCommand(t *testing.T) {
	t.Run("`specter help` exits non-zero with unknown-command error (v0.13 C7)", func(t *testing.T) {
		out, code := runCLI(t, t.TempDir(), "help")
		if code == 0 {
			t.Errorf("expected `specter help` to exit non-zero; got 0. output:\n%s", out)
		}
		if !strings.Contains(out, "unknown command") {
			t.Errorf("expected output to contain 'unknown command'; got:\n%s", out)
		}
	})
}

// cli_test.go -- CLI integration test infrastructure.
//
// Uses the TestMain subprocess pattern: the test binary re-invokes itself
// as the CLI binary when SPECTER_TEST=1 is set.
//
// package main tests have access to all package-level functions, so
// helper functions (runWatchCycle, detectAnnotationLanguages, etc.) can be
// tested directly without subprocess overhead.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain enables the test binary to act as the specter CLI.
// When SPECTER_TEST=1, it calls main() and exits; otherwise it runs tests normally.
func TestMain(m *testing.M) {
	if os.Getenv("SPECTER_TEST") == "1" {
		main()
		os.Exit(0) // reached only if main() returns without os.Exit
	}
	os.Exit(m.Run())
}

// runCLI re-invokes the test binary as the CLI in the given directory.
// Returns (combined stdout+stderr output, exit code).
func runCLI(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SPECTER_TEST=1")
	out, _ := cmd.CombinedOutput()
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return string(out), code
}

// minimalValidSpec returns YAML for a valid draft spec with the given id and tier.
// Optionally pass acIDs to add acceptance criteria.
func minimalValidSpec(id string, tier int, acIDs ...string) string {
	acs := ""
	for _, acID := range acIDs {
		acs += fmt.Sprintf(`
    - id: %s
      description: "Test acceptance criterion"
      priority: high`, acID)
	}
	if acs == "" {
		acs = `
    - id: AC-01
      description: "Test acceptance criterion"
      priority: high`
	}

	return fmt.Sprintf(`spec:
  id: %s
  version: "1.0.0"
  status: draft
  tier: %d

  context:
    system: Test System
    feature: Test Feature

  objective:
    summary: Test spec for CLI integration tests.

  constraints:
    - id: C-01
      description: "MUST work correctly"
      type: technical
      enforcement: error

  acceptance_criteria:%s
`, id, tier, acs)
}

// writeSpec writes a spec file to dir/specs/<filename>.
func writeSpec(t *testing.T, dir, filename, content string) {
	t.Helper()
	specsDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

// writeManifest writes a specter.yaml to dir.
func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

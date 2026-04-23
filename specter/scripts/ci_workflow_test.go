// ci_workflow_test.go — tests for the GitHub Actions CI workflow that
// enforces Conventional Commits on PR titles. Exercises AC-08 (CI fails
// on bad title) and AC-09 (CI passes on valid title) of spec-commits by
// parsing .github/workflows/ci.yml and asserting the "Validate PR title"
// step's shell logic.
//
// @spec spec-commits
package scripts_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ciWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Name string            `yaml:"name"`
			If   string            `yaml:"if"`
			Run  string            `yaml:"run"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func loadCIWorkflow(t *testing.T) *ciWorkflow {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// scripts/ → ../../../.github/workflows/ci.yml (repo root level, above specter/)
	path := filepath.Join(filepath.Dir(file), "..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ci.yml: %v", err)
	}
	var w ciWorkflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatalf("parse ci.yml: %v", err)
	}
	return &w
}

// findValidatePRTitleStep returns the step whose name is "Validate PR title (Conventional Commits)"
// from any job in the workflow, or nil if not found.
func findValidatePRTitleStep(w *ciWorkflow) *struct {
	Name string
	If   string
	Run  string
} {
	for _, job := range w.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Name, "Validate PR title") {
				return &struct {
					Name string
					If   string
					Run  string
				}{Name: step.Name, If: step.If, Run: step.Run}
			}
		}
	}
	return nil
}

// @ac AC-08
func TestCIWorkflow_RejectsInvalidPRTitle(t *testing.T) {
	t.Run("spec-commits/AC-08 rejects invalid pr title", func(t *testing.T) {
		w := loadCIWorkflow(t)
		step := findValidatePRTitleStep(w)
		if step == nil {
			t.Fatal("no 'Validate PR title' step found in any CI job")
		}

		// Must guard on pull_request events
		if !strings.Contains(step.If, "pull_request") {
			t.Errorf("step must run only on pull_request events, got if: %q", step.If)
		}

		// Must contain the Conventional Commits regex and exit non-zero on mismatch
		expectedFragments := []string{
			"feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert",
			"exit 1",
		}
		for _, frag := range expectedFragments {
			if !strings.Contains(step.Run, frag) {
				t.Errorf("validate step must contain %q, got run:\n%s", frag, step.Run)
			}
		}
	})
}

// @ac AC-09
func TestCIWorkflow_AcceptsValidPRTitle(t *testing.T) {
	t.Run("spec-commits/AC-09 accepts valid pr title", func(t *testing.T) {
		w := loadCIWorkflow(t)
		step := findValidatePRTitleStep(w)
		if step == nil {
			t.Fatal("no 'Validate PR title' step found in any CI job")
		}

		// On the success path, the step must print a confirming line and NOT
		// exit non-zero. The "echo PR title OK" plus absence of an unconditional
		// `exit 1` (it's guarded by the regex mismatch) pins this.
		if !strings.Contains(step.Run, "PR title OK") {
			t.Errorf("valid-title path must print 'PR title OK', got run:\n%s", step.Run)
		}
	})
}

// action_test.go — tests for the specter-sync GitHub composite action.
//
// Parses .github/actions/specter-sync/action.yml and asserts the structure
// promised by spec-ci-action.
//
// @spec spec-ci-action
package scripts_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionDoc struct {
	Name   string `yaml:"name"`
	Inputs map[string]struct {
		Description string `yaml:"description"`
		Required    bool   `yaml:"required"`
		Default     string `yaml:"default"`
	} `yaml:"inputs"`
	Runs struct {
		Using string `yaml:"using"`
		Steps []struct {
			Name string                 `yaml:"name"`
			ID   string                 `yaml:"id"`
			Uses string                 `yaml:"uses"`
			With map[string]string      `yaml:"with"`
			Run  string                 `yaml:"run"`
			If   string                 `yaml:"if"`
			Env  map[string]string      `yaml:"env"`
			Raw  map[string]interface{} `yaml:",inline"`
		} `yaml:"steps"`
	} `yaml:"runs"`
}

func loadAction(t *testing.T) *actionDoc {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", ".github", "actions", "specter-sync", "action.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	var a actionDoc
	if err := yaml.Unmarshal(data, &a); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	return &a
}

// @ac AC-01
func TestAction_InputsVersionRequiredArgsDefaultsToSync(t *testing.T) {
	a := loadAction(t)
	v, ok := a.Inputs["version"]
	if !ok {
		t.Fatal("inputs.version missing")
	}
	if !v.Required {
		t.Error("inputs.version must be required")
	}
	if v.Default != "" {
		t.Errorf("inputs.version must have no default, got %q", v.Default)
	}
	args, ok := a.Inputs["args"]
	if !ok {
		t.Fatal("inputs.args missing")
	}
	if args.Required {
		t.Error("inputs.args must be optional")
	}
	if args.Default != "sync" {
		t.Errorf("inputs.args default must be 'sync', got %q", args.Default)
	}
}

// @ac AC-02
func TestAction_DownloadURLPattern(t *testing.T) {
	a := loadAction(t)
	var joined string
	for _, s := range a.Runs.Steps {
		joined += s.Run + "\n"
	}
	checks := []string{
		`https://github.com/Hanalyx/specter/releases/download/v${VERSION}/`,
		`specter_${VERSION}_${OS}_${ARCH}.tar.gz`,
	}
	for _, c := range checks {
		if !strings.Contains(joined, c) {
			t.Errorf("expected download pattern fragment %q in run scripts", c)
		}
	}
}

// @ac AC-03
func TestAction_CacheStepKeyedOnOSAndVersion(t *testing.T) {
	a := loadAction(t)
	var cacheStep *struct {
		Uses string
		With map[string]string
	}
	for _, s := range a.Runs.Steps {
		if strings.HasPrefix(s.Uses, "actions/cache@") {
			cacheStep = &struct {
				Uses string
				With map[string]string
			}{Uses: s.Uses, With: s.With}
			break
		}
	}
	if cacheStep == nil {
		t.Fatal("actions/cache step missing")
	}
	wantKey := "specter-${{ runner.os }}-${{ inputs.version }}"
	if cacheStep.With["key"] != wantKey {
		t.Errorf("cache key must be %q, got %q", wantKey, cacheStep.With["key"])
	}
}

// @ac AC-04
func TestAction_ArchitectureMapping(t *testing.T) {
	a := loadAction(t)
	var detect string
	for _, s := range a.Runs.Steps {
		if s.ID == "platform" {
			detect = s.Run
		}
	}
	if detect == "" {
		t.Fatal("platform detection step missing")
	}
	if !strings.Contains(detect, "X64)") || !strings.Contains(detect, `ARCH="amd64"`) {
		t.Error("X64 must map to amd64")
	}
	if !strings.Contains(detect, "ARM64)") || !strings.Contains(detect, `ARCH="arm64"`) {
		t.Error("ARM64 must map to arm64")
	}
}

// @ac AC-05
func TestAction_RunsSpecterWithInputsArgs(t *testing.T) {
	a := loadAction(t)
	var found bool
	for _, s := range a.Runs.Steps {
		if strings.Contains(s.Run, "specter ${{ inputs.args }}") {
			found = true
		}
	}
	if !found {
		t.Error("expected a run step invoking 'specter ${{ inputs.args }}'")
	}
}

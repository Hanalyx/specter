// action_test.go: tests for the specter-sync GitHub composite action.
//
// Parses .github/actions/specter-sync/action.yml and asserts the structure
// promised by spec-ci-action.
//
// @spec spec-ci-action
package scripts_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	t.Run("spec-ci-action/AC-01 inputs version required args defaults to sync", func(t *testing.T) {
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
	})
}

// @ac AC-02
func TestAction_DownloadURLPattern(t *testing.T) {
	t.Run("spec-ci-action/AC-02 download url pattern", func(t *testing.T) {
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
	})
}

// @ac AC-03
func TestAction_CacheStepKeyedOnOSAndVersion(t *testing.T) {
	t.Run("spec-ci-action/AC-03 cache step keyed on os and version with the v2 suffix", func(t *testing.T) {
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
		// The -v2 suffix retires every entry the unverified action wrote
		// under the old key, so a cache hit can only restore a binary that
		// passed verify-checksum.sh (C-03, C-06).
		wantKey := "specter-${{ runner.os }}-${{ inputs.version }}-v2"
		if cacheStep.With["key"] != wantKey {
			t.Errorf("cache key must be %q, got %q", wantKey, cacheStep.With["key"])
		}
		if _, hasRestoreKeys := cacheStep.With["restore-keys"]; hasRestoreKeys {
			t.Error("cache step must not set restore-keys: a prefix match could restore an entry written under the old key")
		}
	})
}

// @ac AC-04
func TestAction_ArchitectureMapping(t *testing.T) {
	t.Run("spec-ci-action/AC-04 architecture mapping", func(t *testing.T) {
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
	})
}

// @ac AC-05
func TestAction_RunsSpecterWithInputsArgs(t *testing.T) {
	t.Run("spec-ci-action/AC-05 runs specter with inputs args", func(t *testing.T) {
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
	})
}

// downloadStep returns the run script of the step that fetches the release.
func downloadStep(t *testing.T, a *actionDoc) string {
	t.Helper()
	for _, s := range a.Runs.Steps {
		if strings.Contains(s.Run, "releases/download") {
			return s.Run
		}
	}
	t.Fatal("no step downloads from releases/download")
	return ""
}

// @ac AC-06
func TestAction_DownloadVerifiedBeforeExtract(t *testing.T) {
	t.Run("spec-ci-action/AC-06 archive is verified against checksums.txt before tar runs", func(t *testing.T) {
		// Join shell line continuations so a split command is read whole.
		run := strings.ReplaceAll(downloadStep(t, loadAction(t)), "\\\n", " ")

		base := `BASE_URL="https://github.com/Hanalyx/specter/releases/download/v${VERSION}/"`
		for _, want := range []string{
			base,
			`FILENAME="specter_${VERSION}_${OS}_${ARCH}.tar.gz"`,
			`"${BASE_URL}${FILENAME}"`,
			`"${BASE_URL}checksums.txt"`,
		} {
			if !strings.Contains(run, want) {
				t.Errorf("download step must contain %s, so the archive and checksums.txt come from the same release", want)
			}
		}
		if !strings.Contains(run, "set -euo pipefail") {
			t.Error("download step must run under set -euo pipefail so a failed download or check stops it")
		}
		for _, line := range strings.Split(run, "\n") {
			if strings.Contains(line, "verify-checksum.sh") && strings.Contains(line, "||") {
				t.Errorf("verify-checksum.sh failure must stop the step, got: %s", strings.TrimSpace(line))
			}
		}
		verifyAt := strings.Index(run, "verify-checksum.sh")
		tarAt := strings.Index(run, "tar ")
		if verifyAt < 0 {
			t.Fatal("download step must run verify-checksum.sh")
		}
		if tarAt < 0 {
			t.Fatal("download step must extract with tar")
		}
		if verifyAt > tarAt {
			t.Error("verify-checksum.sh must run before tar extracts the archive")
		}
		if regexp.MustCompile(`curl[^\n]*\|\s*tar`).MatchString(run) {
			t.Error("the archive must not be piped from curl into tar")
		}
	})
}

// verifyScript is the checksum verifier the action calls.
func verifyScript(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(file), "..", ".github", "actions", "specter-sync", "verify-checksum.sh")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("verify-checksum.sh: %v", err)
	}
	return p
}

// @ac AC-06
func TestVerifyChecksum_FailsClosed(t *testing.T) {
	t.Run("spec-ci-action/AC-06 verify-checksum.sh passes a match and fails every other case", func(t *testing.T) {
		script := verifyScript(t)
		dir := t.TempDir()
		const name = "specter_9.9.9_linux_amd64.tar.gz"
		archive := filepath.Join(dir, name)
		if err := os.WriteFile(archive, []byte("archive bytes"), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte("archive bytes"))
		good := hex.EncodeToString(sum[:])
		other := sha256.Sum256([]byte("other bytes"))
		bad := hex.EncodeToString(other[:])
		// The real checksums.txt lists an SBOM next to each archive, and its
		// name starts with the archive's name. A substring match finds two
		// entries and rejects every real release, so the fixture carries one.
		otherLine := bad + "  specter_9.9.9_darwin_arm64.tar.gz\n" + bad + "  " + name + ".sbom.json\n"

		run := func(sums string, write bool) (int, string) {
			p := filepath.Join(t.TempDir(), "checksums.txt")
			if write {
				if err := os.WriteFile(p, []byte(sums), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out, err := exec.Command("bash", script, p, archive).CombinedOutput()
			if err == nil {
				return 0, string(out)
			}
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode(), string(out)
			}
			t.Fatalf("run verify-checksum.sh: %v", err)
			return -1, ""
		}

		// Control: a correct entry among others must pass. Without this,
		// a script that always fails would satisfy every case below.
		if code, out := run(otherLine+good+"  "+name+"\n", true); code != 0 {
			t.Fatalf("matching entry rejected (exit %d):\n%s", code, out)
		}

		cases := []struct {
			label string
			sums  string
			write bool
		}{
			{"mismatched hash", otherLine + bad + "  " + name + "\n", true},
			{"no entry for the archive", otherLine, true},
			{"two entries for the archive", good + "  " + name + "\n" + good + "  " + name + "\n", true},
			{"malformed hash", "abc123  " + name + "\n", true},
			{"missing checksums file", "", false},
			{"empty checksums file", "", true},
		}
		for _, c := range cases {
			if code, out := run(c.sums, c.write); code == 0 {
				t.Errorf("%s: expected non-zero exit, got 0:\n%s", c.label, out)
			}
		}
	})
}

// runPlatformStep runs the action's platform step under bash, with the
// runner context values filled in, and returns its exit code, its combined
// output, and what it wrote to GITHUB_OUTPUT.
func runPlatformStep(t *testing.T, script, runnerOS, runnerArch string) (int, string, string) {
	t.Helper()
	script = strings.ReplaceAll(script, "${{ runner.os }}", runnerOS)
	script = strings.ReplaceAll(script, "${{ runner.arch }}", runnerArch)
	if strings.Contains(script, "${{") {
		t.Fatalf("platform step uses a context value this test does not fill in:\n%s", script)
	}
	outFile := filepath.Join(t.TempDir(), "github_output")
	cmd := exec.Command("bash", "-e", "-c", script)
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outFile)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run platform step: %v", err)
		}
		code = ee.ExitCode()
	}
	written, _ := os.ReadFile(outFile)
	return code, string(out), string(written)
}

// @ac AC-07
func TestAction_OSMappingMatchesReleaseNames(t *testing.T) {
	t.Run("spec-ci-action/AC-07 macOS maps to darwin, Linux to linux, other OS refused", func(t *testing.T) {
		var script string
		for _, s := range loadAction(t).Runs.Steps {
			if s.ID == "platform" {
				script = s.Run
			}
		}
		if script == "" {
			t.Fatal("platform detection step missing")
		}

		for runnerOS, want := range map[string]string{"Linux": "os=linux", "macOS": "os=darwin"} {
			code, out, written := runPlatformStep(t, script, runnerOS, "X64")
			if code != 0 {
				t.Errorf("%s: platform step exited %d:\n%s", runnerOS, code, out)
				continue
			}
			if !strings.Contains(written, want+"\n") {
				t.Errorf("%s: expected %q in GITHUB_OUTPUT, got:\n%s", runnerOS, want, written)
			}
		}

		for _, runnerOS := range []string{"Windows", "FreeBSD"} {
			code, out, written := runPlatformStep(t, script, runnerOS, "X64")
			if code == 0 {
				t.Errorf("%s: expected non-zero exit, got 0; GITHUB_OUTPUT:\n%s", runnerOS, written)
			}
			if !strings.Contains(out, runnerOS) || !strings.Contains(out, "Linux and macOS") {
				t.Errorf("%s: refusal must name the OS and say only Linux and macOS are supported, got:\n%s", runnerOS, out)
			}
			if strings.Contains(written, "os=") {
				t.Errorf("%s: refused OS must not write an os output, got:\n%s", runnerOS, written)
			}
		}
	})
}

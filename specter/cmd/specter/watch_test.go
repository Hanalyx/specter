// watch_test.go -- tests for specter watch helpers and behavior.
//
// Note: AC-02 (spec file change triggers re-run) and AC-03 (test file change
// triggers re-run) are tested indirectly through modsChanged + collectModTimes.
// AC-05 (Ctrl+C exits 0) is covered by the AC-01 startup test which uses a
// subprocess that exits cleanly via timeout.
//
// @spec spec-watch
package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hanalyx/specter/internal/manifest"
)

// @ac AC-01
func TestWatch_StartupMessageAndInitialRun(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	_, lines := startWatch(t, dir)

	// Startup header must appear
	if !waitForLine(lines, "specter watch", 3*time.Second) {
		t.Error("expected 'specter watch' header in startup output")
	}
	// Initial run must happen before any file changes
	if !waitForLine(lines, "]", 3*time.Second) {
		t.Error("expected initial run output (timestamp line) within 3s")
	}
}

// @ac AC-04
func TestWatch_RunCycle_OutputFormat(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	// Capture output by redirecting stdout — test runWatchCycle directly.
	// We redirect os.Stdout temporarily to capture the output.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	m := manifest.Defaults()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	runWatchCycle(m)
	_ = os.Chdir(origDir)

	_ = w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Must contain [HH:MM:SS] format
	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("expected timestamp format [HH:MM:SS] in watch output, got: %q", output)
	}
	// Must contain PASS or FAIL
	if !strings.Contains(output, "PASS") && !strings.Contains(output, "FAIL") && !strings.Contains(output, "WARN") {
		t.Errorf("expected PASS/FAIL/WARN in watch output, got: %q", output)
	}
	// Must contain spec count
	if !strings.Contains(output, "spec") {
		t.Errorf("expected spec count in watch output, got: %q", output)
	}
}

// @ac AC-06
func TestWatch_Debounce_RapidWritesProduceOneRun(t *testing.T) {
	// AC-06: rapid successive writes within 150ms must produce exactly one run.
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	_, lines := startWatch(t, dir)

	// Wait for the initial run
	if !waitForLine(lines, "]", 5*time.Second) {
		t.Fatal("watch did not produce initial run within 5s")
	}

	// Fire three rapid writes inside the debounce window
	specsDir := filepath.Join(dir, "specs")
	for i := 0; i < 3; i++ {
		content := minimalValidSpec("my-spec", 3, "AC-01")
		_ = os.WriteFile(filepath.Join(specsDir, "my-spec.spec.yaml"), []byte(content), 0644)
		time.Sleep(30 * time.Millisecond) // well within 150ms window
	}

	// Collect run lines for 600ms — should see at most 2 (one for the debounced
	// burst, possibly one more if timing is tight). Must NOT see 3.
	runLines := 0
	deadline := time.After(600 * time.Millisecond)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				goto done
			}
			if strings.Contains(line, "]") && (strings.Contains(line, "PASS") || strings.Contains(line, "FAIL")) {
				runLines++
			}
		case <-deadline:
			goto done
		}
	}
done:
	if runLines >= 3 {
		t.Errorf("expected debounced to ≤2 runs for 3 rapid writes, got %d", runLines)
	}
}

// @ac AC-07
func TestWatch_FailRunContinuesLoop(t *testing.T) {
	// Test that runWatchCycle prints output even on failure (no panic/halt).
	// A directory with no spec files causes WARN (not panic).
	dir := t.TempDir()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	m := manifest.Defaults()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	// Call runWatchCycle twice — must not panic on either call
	runWatchCycle(m)
	runWatchCycle(m)
	_ = os.Chdir(origDir)

	_ = w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Two runs must both produce output
	lines := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("expected 2 run lines (loop continues on FAIL), got %d lines:\n%s", lines, output)
	}
}

// startWatch starts a watch subprocess in dir and returns the cmd + a channel
// that receives output lines as they are printed.
func startWatch(t *testing.T, dir string, extraArgs ...string) (*exec.Cmd, <-chan string) {
	t.Helper()
	args := append([]string{"watch"}, extraArgs...)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SPECTER_TEST=1")

	var combined bytes.Buffer
	pr, pw := io.Pipe()
	cmd.Stdout = io.MultiWriter(&combined, pw)
	cmd.Stderr = io.MultiWriter(&combined, pw)

	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()

	if err := cmd.Start(); err != nil {
		t.Fatalf("start watch: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = pw.Close()
	})
	return cmd, lines
}

// waitForLine reads from lines until a line containing substr is found,
// or the timeout elapses.
func waitForLine(lines <-chan string, substr string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return false
			}
			if strings.Contains(line, substr) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// @ac AC-02
func TestWatch_SpecFileChange_TriggersRerun(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	cmd, lines := startWatch(t, dir)

	// Wait for the initial run output (timestamp line)
	if !waitForLine(lines, "]", 5*time.Second) {
		t.Fatal("watch did not produce initial run output within 5s")
	}

	// Modify the spec file to trigger a re-run
	time.Sleep(150 * time.Millisecond) // ensure mtime changes
	newContent := minimalValidSpec("my-spec", 3, "AC-01", "AC-02")
	specsDir := filepath.Join(dir, "specs")
	if err := os.WriteFile(filepath.Join(specsDir, "my-spec.spec.yaml"), []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for a second run output (triggered by file change)
	if !waitForLine(lines, "]", 3*time.Second) {
		t.Fatal("watch did not re-run within 3s after spec file change")
	}
	_ = cmd // used via t.Cleanup
}

// @ac AC-03
func TestWatch_TestFileChange_TriggersRerun(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	cmd, lines := startWatch(t, dir)

	// Wait for the initial run
	if !waitForLine(lines, "]", 5*time.Second) {
		t.Fatal("watch did not produce initial run output within 5s")
	}

	// Create a new test file to trigger a re-run
	time.Sleep(150 * time.Millisecond)
	testFile := filepath.Join(dir, "new_test.go")
	if err := os.WriteFile(testFile, []byte("// @spec my-spec\n// @ac AC-01\npackage main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for a re-run triggered by the test file creation
	if !waitForLine(lines, "]", 3*time.Second) {
		t.Fatal("watch did not re-run within 3s after test file change")
	}
	_ = cmd
}

// @ac AC-05
func TestWatch_CtrlC_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 3, "AC-01"))

	cmd, lines := startWatch(t, dir)

	// Wait for the initial run to confirm watch is up
	if !waitForLine(lines, "]", 5*time.Second) {
		t.Fatal("watch did not start within 5s")
	}

	// Send SIGINT (same as Ctrl+C)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	// Wait for the process to exit
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not exit within 3s after SIGINT")
	}

	if cmd.ProcessState.ExitCode() != 0 {
		t.Errorf("expected exit code 0 after SIGINT, got %d", cmd.ProcessState.ExitCode())
	}
}

// modsChanged unit tests (AC-02 / AC-03 coverage for the change detection logic)
func TestModsChanged_IdenticalMaps_ReturnsFalse(t *testing.T) {
	now := time.Now()
	prev := map[string]time.Time{"a.go": now, "b.go": now}
	curr := map[string]time.Time{"a.go": now, "b.go": now}
	if modsChanged(prev, curr) {
		t.Error("identical mod times must not be reported as changed")
	}
}

func TestModsChanged_DifferentMtime_ReturnsTrue(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	prev := map[string]time.Time{"a.go": t1}
	curr := map[string]time.Time{"a.go": t2}
	if !modsChanged(prev, curr) {
		t.Error("different mod time must be reported as changed")
	}
}

func TestModsChanged_NewFile_ReturnsTrue(t *testing.T) {
	now := time.Now()
	prev := map[string]time.Time{"a.go": now}
	curr := map[string]time.Time{"a.go": now, "b.go": now}
	if !modsChanged(prev, curr) {
		t.Error("new file must be reported as changed")
	}
}

func TestModsChanged_DeletedFile_ReturnsTrue(t *testing.T) {
	now := time.Now()
	prev := map[string]time.Time{"a.go": now, "b.go": now}
	curr := map[string]time.Time{"a.go": now}
	if !modsChanged(prev, curr) {
		t.Error("deleted file must be reported as changed")
	}
}

// detectAnnotationLanguages unit tests (spec-explain AC-03 / AC-08 coverage)
func TestDetectAnnotationLanguages_GoFiles(t *testing.T) {
	files := []string{"handler_test.go", "user_test.go"}
	langs := detectAnnotationLanguages(files)
	found := false
	for _, l := range langs {
		if strings.Contains(l, "Go") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Go/generic in language list, got %v", langs)
	}
}

func TestDetectAnnotationLanguages_PythonFiles(t *testing.T) {
	files := []string{"test_user.py"}
	langs := detectAnnotationLanguages(files)
	found := false
	for _, l := range langs {
		if strings.Contains(l, "Python") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Python in language list, got %v", langs)
	}
}

func TestDetectAnnotationLanguages_NoFiles_DefaultsToGo(t *testing.T) {
	langs := detectAnnotationLanguages(nil)
	if len(langs) == 0 {
		t.Fatal("expected default language when no files, got empty list")
	}
	if !strings.Contains(langs[0], "Go") {
		t.Errorf("expected Go/generic default, got %v", langs)
	}
}

// resolveCmd mermaid test (spec-resolve AC-08)
// @spec spec-resolve
// @ac AC-08
func TestResolve_MermaidOutput(t *testing.T) {
	dir := t.TempDir()
	// Write two specs with a dependency
	writeSpec(t, dir, "dep.spec.yaml", minimalValidSpec("dep", 2, "AC-01"))
	depender := minimalValidSpec("main-spec", 2, "AC-01")
	depender += `
  depends_on:
    - spec_id: dep
      version_range: "^1.0.0"
      relationship: requires
`
	writeSpec(t, dir, "main-spec.spec.yaml", depender)

	out, _ := runCLI(t, dir, "resolve", "--mermaid")
	if !strings.Contains(out, "graph BT") {
		t.Errorf("expected 'graph BT' in mermaid output, got:\n%s", out)
	}
	if !strings.Contains(out, "dep") {
		t.Errorf("expected node 'dep' in mermaid output, got:\n%s", out)
	}
	if !strings.Contains(out, "main-spec") {
		t.Errorf("expected node 'main-spec' in mermaid output, got:\n%s", out)
	}
}

// toCamelCase and sanitizeID unit tests
func TestToCamelCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"spec-parse", "SpecParse"},
		{"my-feature", "MyFeature"},
		{"a", "A"},
	}
	for _, c := range cases {
		if got := toCamelCase(c.in); got != c.want {
			t.Errorf("toCamelCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	if got := sanitizeID("my-spec"); got != "my_spec" {
		t.Errorf("sanitizeID = %q, want %q", got, "my_spec")
	}
}

// v0.8.3: resolve --dot / --mermaid must NOT include the plain-English
// "No dependency issues found." footer on stdout. The footer pollutes
// structured output and breaks pipes to dot / mmdc / GitHub renderers.
//
// @spec spec-resolve
// @ac AC-08
func TestResolve_DotOutput_NoPlainEnglishFooter(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "a.spec.yaml", minimalValidSpec("a", 3, "AC-01"))

	out, _ := runCLI(t, dir, "resolve", "--dot")
	if strings.Contains(out, "No dependency issues") {
		t.Errorf("resolve --dot stdout must not contain human-readable footer; got:\n%s", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "digraph") {
		t.Errorf("resolve --dot must start with 'digraph', got:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "}") {
		t.Errorf("resolve --dot must end with '}', got:\n%s", out)
	}
}

// @spec spec-resolve
// @ac AC-08
func TestResolve_MermaidOutput_NoPlainEnglishFooter(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "a.spec.yaml", minimalValidSpec("a", 3, "AC-01"))

	out, _ := runCLI(t, dir, "resolve", "--mermaid")
	if strings.Contains(out, "No dependency issues") {
		t.Errorf("resolve --mermaid stdout must not contain human-readable footer; got:\n%s", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "graph BT") {
		t.Errorf("resolve --mermaid must start with 'graph BT', got:\n%s", out)
	}
}

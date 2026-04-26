// CLI integration tests for `specter pre-push-check`.
//
// @spec spec-manifest
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFileAt(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
}

// runGitInDir runs a git command in dir and returns stdout. Fatals on error.
func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// setupGitRepo creates a fresh git repo in t.TempDir(), seeds it with one
// commit, and returns the repo path.
func setupBareGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGitInDir(t, dir, "init", "--initial-branch=main")
	runGitInDir(t, dir, "config", "user.email", "test@example.com")
	runGitInDir(t, dir, "config", "user.name", "Test User")
	runGitInDir(t, dir, "config", "commit.gpgsign", "false")

	// Initial commit so HEAD is valid.
	if err := writeFileAt(dir, "README.md", "# initial\n"); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, dir, "add", "README.md")
	runGitInDir(t, dir, "commit", "-m", "initial")

	return dir
}

// runCLIWithStdin runs the specter binary with the given stdin and args from
// the given dir. Returns stdout+stderr combined and exit code.
func runCLIWithStdin(t *testing.T, dir, stdin string, args ...string) (string, int) {
	t.Helper()
	bin, err := filepath.Abs(filepath.Join("..", "..", "bin", "specter"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected exec error: %v", err)
	}
	return string(out), code
}

// @ac AC-28
func TestPrePushCheck_ImplOnlyDiff_Blocks(t *testing.T) {
	t.Run("spec-manifest/AC-28 pre-push-check blocks push with impl-only diff", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		baseSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		// Add an impl change with no @spec/@ac annotation.
		if err := writeFileAt(dir, "foo.go", "package main\nfunc bar() {}\n"); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", "foo.go")
		runGitInDir(t, dir, "commit", "-m", "add foo")
		headSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		stdin := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", headSha, baseSha)
		out, code := runCLIWithStdin(t, dir, stdin, "pre-push-check")

		if code == 0 {
			t.Fatalf("expected nonzero exit for impl-only diff, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "push blocked") && !strings.Contains(out, "annotation") {
			t.Errorf("expected blocked-push diagnostic in output, got:\n%s", out)
		}
	})
}

// @ac AC-28
func TestPrePushCheck_AnnotationDelta_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 pre-push-check allows push when annotation delta present", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		baseSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		if err := writeFileAt(dir, "foo.go", "package main\nfunc bar() {}\n"); err != nil {
			t.Fatal(err)
		}
		if err := writeFileAt(dir, "foo_test.go", "package main\n\n// @spec spec-foo\n// @ac AC-01\nfunc TestBar(t *testing.T) {}\n"); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", ".")
		runGitInDir(t, dir, "commit", "-m", "add foo + test")
		headSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		stdin := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", headSha, baseSha)
		out, code := runCLIWithStdin(t, dir, stdin, "pre-push-check")

		if code != 0 {
			t.Fatalf("expected exit 0 for impl + annotation delta, got %d; output:\n%s", code, out)
		}
	})
}

// @ac AC-28
func TestPrePushCheck_DocsOnly_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 pre-push-check allows docs-only diff", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		baseSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		if err := writeFileAt(dir, "NOTES.md", "# notes\n"); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", "NOTES.md")
		runGitInDir(t, dir, "commit", "-m", "add notes")
		headSha := runGitInDir(t, dir, "rev-parse", "HEAD")

		stdin := fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", headSha, baseSha)
		_, code := runCLIWithStdin(t, dir, stdin, "pre-push-check")

		if code != 0 {
			t.Errorf("expected exit 0 for docs-only diff, got %d", code)
		}
	})
}

// @ac AC-28
func TestPrePushCheck_DeletedBranch_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 pre-push-check skips deleted-branch refs", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		// Deleted branch: local sha = ZeroSha. Nothing to inspect.
		stdin := "refs/heads/gone 0000000000000000000000000000000000000000 refs/heads/gone aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"
		_, code := runCLIWithStdin(t, dir, stdin, "pre-push-check")

		if code != 0 {
			t.Errorf("expected exit 0 for deleted-branch ref, got %d", code)
		}
	})
}

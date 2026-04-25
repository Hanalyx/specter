// CLI integration tests for `specter init --install-hook`.
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupHookDir creates a temp dir with a .git/hooks/ subdirectory so the hook
// installer has a target to write to. Mirrors the structure of a real git repo
// without spinning one up — sufficient for AC-27 file-level checks.
func setupHookDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// @ac AC-27
func TestInitInstallHook_WritesExecutableFile(t *testing.T) {
	t.Run("spec-manifest/AC-27 init --install-hook writes executable .git/hooks/pre-push", func(t *testing.T) {
		dir := setupHookDir(t)

		_, code := runCLI(t, dir, "init", "--install-hook")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		hookPath := filepath.Join(dir, ".git", "hooks", "pre-push")
		info, err := os.Stat(hookPath)
		if err != nil {
			t.Fatalf("expected hook at %s, stat err: %v", hookPath, err)
		}
		if info.Mode()&0111 == 0 {
			t.Errorf("expected hook to be executable (mode 0755), got mode %v", info.Mode())
		}
	})
}

// @ac AC-27
func TestInitInstallHook_FencedMarkersPresent(t *testing.T) {
	t.Run("spec-manifest/AC-27 hook content wrapped in specter:begin/end markers", func(t *testing.T) {
		dir := setupHookDir(t)

		_, code := runCLI(t, dir, "init", "--install-hook")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		hookPath := filepath.Join(dir, ".git", "hooks", "pre-push")
		data, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatal(err)
		}
		body := string(data)
		if !strings.Contains(body, "<!-- specter:begin v1 -->") {
			t.Errorf("expected begin marker, got:\n%s", body)
		}
		if !strings.Contains(body, "<!-- specter:end -->") {
			t.Errorf("expected end marker, got:\n%s", body)
		}
	})
}

// @ac AC-27
func TestInitInstallHook_IdempotentReRun_PreservesOutOfFence(t *testing.T) {
	t.Run("spec-manifest/AC-27 re-running --install-hook preserves user content outside fence", func(t *testing.T) {
		dir := setupHookDir(t)

		_, code := runCLI(t, dir, "init", "--install-hook")
		if code != 0 {
			t.Fatalf("first run expected exit 0, got %d", code)
		}
		hookPath := filepath.Join(dir, ".git", "hooks", "pre-push")

		// Append user-authored content after the fence.
		original, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatal(err)
		}
		userMarker := "\n# user-authored hook content\nrunning_extra_check"
		if err := os.WriteFile(hookPath, append(original, []byte(userMarker)...), 0755); err != nil {
			t.Fatal(err)
		}

		// Re-run.
		_, code = runCLI(t, dir, "init", "--install-hook")
		if code != 0 {
			t.Fatalf("re-run expected exit 0, got %d", code)
		}

		final, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(final), userMarker) {
			t.Errorf("expected user-authored content preserved across re-run, got:\n%s", string(final))
		}
	})
}

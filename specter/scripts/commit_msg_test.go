// commit_msg_test.go — tests for the commit-msg hook script.
//
// Exercises AC-01 through AC-07 of spec-commits by running the
// scripts/commit-msg shell script as a subprocess with controlled input.
//
// @spec spec-commits
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// hookPath returns the absolute path to the commit-msg script.
func hookPath(t *testing.T) string {
	t.Helper()
	// This file lives in specter/scripts/, so the hook is sibling to it.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "commit-msg")
}

// runHook writes msg to a temp file and invokes the hook script with it.
// Returns the exit code (0 = accept, 1 = reject) and combined output.
func runHook(t *testing.T, msg string) (int, string) {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "commit-msg-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := tmp.WriteString(msg); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmp.Close()

	cmd := exec.Command("sh", hookPath(t), tmp.Name())
	out, _ := cmd.CombinedOutput()
	return cmd.ProcessState.ExitCode(), string(out)
}

// @ac AC-01
func TestCommitMsg_RejectsPlainMessage(t *testing.T) {
	t.Run("spec-commits/AC-01 rejects plain message", func(t *testing.T) {
		code, out := runHook(t, "update readme")
		if code == 0 {
			t.Errorf("expected rejection of plain message, got exit 0\noutput: %s", out)
		}
		if out == "" {
			t.Error("expected error message on rejection, got empty output")
		}
	})
}

// @ac AC-02
func TestCommitMsg_AcceptsMinimalFeat(t *testing.T) {
	t.Run("spec-commits/AC-02 accepts minimal feat", func(t *testing.T) {
		code, out := runHook(t, "feat: add thing")
		if code != 0 {
			t.Errorf("expected acceptance of 'feat: add thing', got exit %d\noutput: %s", code, out)
		}
	})
}

// @ac AC-03
func TestCommitMsg_AcceptsScopedCommit(t *testing.T) {
	t.Run("spec-commits/AC-03 accepts scoped commit", func(t *testing.T) {
		code, out := runHook(t, "fix(coverage): resolve off-by-one in traceability")
		if code != 0 {
			t.Errorf("expected acceptance of scoped commit, got exit %d\noutput: %s", code, out)
		}
	})
}

// @ac AC-04
func TestCommitMsg_AcceptsBreakingChange(t *testing.T) {
	t.Run("spec-commits/AC-04 accepts breaking change", func(t *testing.T) {
		code, out := runHook(t, "feat!: rename @ac annotation to @criteria")
		if code != 0 {
			t.Errorf("expected acceptance of breaking change commit, got exit %d\noutput: %s", code, out)
		}
	})
}

// @ac AC-05
func TestCommitMsg_AllowsMergeCommit(t *testing.T) {
	t.Run("spec-commits/AC-05 allows merge commit", func(t *testing.T) {
		code, out := runHook(t, "Merge pull request #16 from feat/v0.5.0-roadmap")
		if code != 0 {
			t.Errorf("expected merge commit to be exempt, got exit %d\noutput: %s", code, out)
		}
	})
}

// @ac AC-06
func TestCommitMsg_RejectsInvalidType(t *testing.T) {
	t.Run("spec-commits/AC-06 rejects invalid type", func(t *testing.T) {
		code, out := runHook(t, "update: change some stuff")
		if code == 0 {
			t.Errorf("expected rejection of invalid type 'update', got exit 0\noutput: %s", out)
		}
	})
}

// @ac AC-07
func TestCommitMsg_RejectsLongSubject(t *testing.T) {
	t.Run("spec-commits/AC-07 rejects long subject", func(t *testing.T) {
		long := "feat: " + string(make([]byte, 96)) // 6 + 96 = 102 chars
		for i := range long[6:] {
			long = long[:6+i] + "a" + long[6+i+1:]
		}
		code, out := runHook(t, long)
		if code == 0 {
			t.Errorf("expected rejection of %d-char subject, got exit 0\noutput: %s", len(long), out)
		}
	})
}

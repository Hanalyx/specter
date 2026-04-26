// Pure-function tests for the pre-push hook script generation and
// diff-analysis logic used by `init --install-hook`.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// AC-28: hook blocks impl-only diffs; passes annotation-delta diffs and
// docs/spec/test-only diffs.
func TestShouldBlockPush_ImplOnly_Blocks(t *testing.T) {
	t.Run("spec-manifest/AC-28 impl-only diff blocks push", func(t *testing.T) {
		diff := PushDiffSummary{
			ImplFilesChanged: []string{"internal/foo/bar.go"},
			TestFilesChanged: nil,
			DocFilesChanged:  nil,
			SpecFilesChanged: nil,
			AnnotationDelta:  false,
		}
		if !ShouldBlockPush(diff) {
			t.Errorf("expected impl-only diff to block, did not")
		}
	})
}

func TestShouldBlockPush_AnnotationDelta_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 annotation-delta diff allows push", func(t *testing.T) {
		diff := PushDiffSummary{
			ImplFilesChanged: []string{"internal/foo/bar.go"},
			TestFilesChanged: []string{"internal/foo/bar_test.go"},
			AnnotationDelta:  true, // a test gained @ac
		}
		if ShouldBlockPush(diff) {
			t.Errorf("expected annotation-delta diff to allow push, did not")
		}
	})
}

func TestShouldBlockPush_DocsOrSpecsOnly_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 docs-or-specs-only diff allows push", func(t *testing.T) {
		diff := PushDiffSummary{
			DocFilesChanged:  []string{"README.md"},
			SpecFilesChanged: []string{"specs/spec-foo.spec.yaml"},
			AnnotationDelta:  false,
		}
		if ShouldBlockPush(diff) {
			t.Errorf("expected docs/specs-only diff to allow push, did not")
		}
	})
}

func TestShouldBlockPush_TestsOnly_Allows(t *testing.T) {
	t.Run("spec-manifest/AC-28 tests-only diff allows push", func(t *testing.T) {
		diff := PushDiffSummary{
			TestFilesChanged: []string{"internal/foo/bar_test.go"},
			AnnotationDelta:  true,
		}
		if ShouldBlockPush(diff) {
			t.Errorf("expected tests-only with annotation delta to allow push, did not")
		}
	})
}

// AC-27: hook script content shape — must contain shell-comment fenced
// markers (HTML-comment markers would be invalid shell syntax) and invoke
// the specter pre-push helper.
func TestPrePushHookScript_ContainsFencedMarkers(t *testing.T) {
	t.Run("spec-manifest/AC-27 hook script wrapped in shell-comment markers", func(t *testing.T) {
		script := PrePushHookScript()

		if !strings.HasPrefix(script, "#!") {
			t.Errorf("expected shebang at top of hook script, got:\n%s", script)
		}
		if !strings.Contains(script, "# specter:begin v1") {
			t.Errorf("expected shell-comment begin marker in hook, got:\n%s", script)
		}
		if !strings.Contains(script, "# specter:end") {
			t.Errorf("expected shell-comment end marker in hook, got:\n%s", script)
		}
		// HTML-comment markers must NOT be present — they would break sh parsing.
		if strings.Contains(script, "<!--") {
			t.Errorf("hook must not contain HTML-comment markers (invalid shell syntax), got:\n%s", script)
		}
		if !strings.Contains(script, "specter") {
			t.Errorf("expected hook to reference specter binary, got:\n%s", script)
		}
	})
}

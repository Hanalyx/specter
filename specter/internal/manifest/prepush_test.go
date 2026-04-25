// Pure-function tests for the pre-push hook helpers used by `specter pre-push-check`.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// AC-28: pre-push hook reads git's stdin format (one line per ref:
// "local_ref local_sha remote_ref remote_sha"). ParsePushSpecs must handle
// the common cases: single ref, multiple refs, new branch (remote sha is
// ZeroSha), deleted branch (local sha is ZeroSha), and trailing whitespace.
func TestParsePushSpecs_SingleRef(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs single ref", func(t *testing.T) {
		input := "refs/heads/main abc123 refs/heads/main def456\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].LocalRef != "refs/heads/main" || specs[0].LocalSha != "abc123" {
			t.Errorf("got %+v", specs[0])
		}
	})
}

func TestParsePushSpecs_MultipleRefs(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs multiple refs", func(t *testing.T) {
		input := "refs/heads/feat/a aaa refs/heads/feat/a 111\nrefs/heads/feat/b bbb refs/heads/feat/b 222\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(specs))
		}
	})
}

func TestParsePushSpecs_NewBranch_ZeroRemoteSha(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs new branch carries zero remote sha", func(t *testing.T) {
		input := "refs/heads/new abc123 refs/heads/new " + ZeroSha + "\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if specs[0].RemoteSha != ZeroSha {
			t.Errorf("expected remote sha = ZeroSha for new branch, got %q", specs[0].RemoteSha)
		}
	})
}

func TestParsePushSpecs_DeletedBranch_ZeroLocalSha(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs deleted branch carries zero local sha", func(t *testing.T) {
		input := "refs/heads/gone " + ZeroSha + " refs/heads/gone abc123\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if specs[0].LocalSha != ZeroSha {
			t.Errorf("expected local sha = ZeroSha for deleted branch, got %q", specs[0].LocalSha)
		}
	})
}

func TestParsePushSpecs_EmptyInput(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs empty stdin returns empty slice", func(t *testing.T) {
		specs, err := ParsePushSpecs(strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		if len(specs) != 0 {
			t.Errorf("expected no specs from empty input, got %d", len(specs))
		}
	})
}

func TestParsePushSpecs_MalformedLine(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs malformed line returns error", func(t *testing.T) {
		input := "only-three tokens here\n"
		_, err := ParsePushSpecs(strings.NewReader(input))
		if err == nil {
			t.Fatal("expected error on malformed line, got nil")
		}
	})
}

// AC-28: HasAnnotationDelta reports whether the unified-diff output contains
// any added line (starting with `+` but not `+++`) carrying `@spec ` or `@ac `.
func TestHasAnnotationDelta_AddedSpecLine(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta detects added @spec line", func(t *testing.T) {
		diff := "+++ b/foo_test.go\n+// @spec spec-foo\n+// @ac AC-01\n"
		if !HasAnnotationDelta(diff) {
			t.Errorf("expected delta detected, got false")
		}
	})
}

func TestHasAnnotationDelta_RemovedAnnotationOnly(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta ignores removed-only annotations", func(t *testing.T) {
		// Only `-` lines (deletion) — count as a delta? A removed annotation
		// is a code change that loses test coverage, but it's still a change
		// to the annotation set. Decision: count as delta. (Removing a test
		// without a replacement is something coverage --strict will catch
		// downstream; the hook just needs "annotation lines moved at all".)
		diff := "+++ b/foo_test.go\n-// @ac AC-01\n"
		if !HasAnnotationDelta(diff) {
			t.Errorf("expected removed-only annotation to count as delta, got false")
		}
	})
}

func TestHasAnnotationDelta_NoChange(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta returns false when no @spec/@ac lines added or removed", func(t *testing.T) {
		diff := "+++ b/foo.go\n+func bar() {}\n-func baz() {}\n"
		if HasAnnotationDelta(diff) {
			t.Errorf("expected no delta, got true")
		}
	})
}

func TestHasAnnotationDelta_ContextLineIgnored(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta ignores context lines mentioning @spec", func(t *testing.T) {
		// Context lines (no leading + or -) appear in unified diff but are
		// unchanged — not part of the delta.
		diff := " // @spec foo\n+func bar() {}\n"
		if HasAnnotationDelta(diff) {
			t.Errorf("expected context line not to count as delta, got true")
		}
	})
}

func TestHasAnnotationDelta_DiffHeaderIgnored(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta ignores +++/--- diff headers", func(t *testing.T) {
		// The +++ b/foo.go header isn't an added line of code; must not
		// match the @spec/@ac scan even though it begins with +.
		diff := "+++ b/foo.go\n--- a/foo.go\n+func x() {}\n"
		if HasAnnotationDelta(diff) {
			t.Errorf("expected diff headers not to count as delta, got true")
		}
	})
}

// SummarizePushDiff combines file categorization (existing IsImplFile etc.)
// with annotation-delta detection into one PushDiffSummary that ShouldBlockPush
// can consume directly.
func TestSummarizePushDiff_ImplOnlyNoAnnotation(t *testing.T) {
	t.Run("spec-manifest/AC-28 SummarizePushDiff impl-only no-annotation produces blocking summary", func(t *testing.T) {
		filenames := []string{"internal/foo/bar.go"}
		diff := "+++ b/internal/foo/bar.go\n+func bar() {}\n"
		s := SummarizePushDiff(filenames, diff)

		if len(s.ImplFilesChanged) != 1 || s.ImplFilesChanged[0] != "internal/foo/bar.go" {
			t.Errorf("expected impl files = [internal/foo/bar.go], got %+v", s.ImplFilesChanged)
		}
		if s.AnnotationDelta {
			t.Errorf("expected no annotation delta, got true")
		}
		if !ShouldBlockPush(s) {
			t.Errorf("expected ShouldBlockPush = true for impl-only no-annotation")
		}
	})
}

func TestSummarizePushDiff_ImplPlusAnnotation(t *testing.T) {
	t.Run("spec-manifest/AC-28 SummarizePushDiff impl + annotation delta allows push", func(t *testing.T) {
		filenames := []string{"internal/foo/bar.go", "internal/foo/bar_test.go"}
		diff := "+++ b/internal/foo/bar_test.go\n+// @spec spec-foo\n+// @ac AC-01\n"
		s := SummarizePushDiff(filenames, diff)

		if !s.AnnotationDelta {
			t.Errorf("expected annotation delta, got false")
		}
		if ShouldBlockPush(s) {
			t.Errorf("expected ShouldBlockPush = false for impl + annotation delta")
		}
	})
}

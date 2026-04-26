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
//
// Test inputs use 40-char hex shas matching git's canonical pre-push stdin
// format. Earlier tests used abbreviated forms (`abc123`, `aaa`); those are
// not what git actually emits and are now rejected by ParsePushSpecs's
// validShaRE guard (added to defeat flag-injection via crafted shas).
const (
	sha40A = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha40B = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sha40C = "cccccccccccccccccccccccccccccccccccccccc"
	sha40D = "dddddddddddddddddddddddddddddddddddddddd"
)

func TestParsePushSpecs_SingleRef(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs single ref", func(t *testing.T) {
		input := "refs/heads/main " + sha40A + " refs/heads/main " + sha40B + "\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].LocalRef != "refs/heads/main" || specs[0].LocalSha != sha40A {
			t.Errorf("got %+v", specs[0])
		}
	})
}

func TestParsePushSpecs_MultipleRefs(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs multiple refs", func(t *testing.T) {
		input := "refs/heads/feat/a " + sha40A + " refs/heads/feat/a " + sha40B + "\n" +
			"refs/heads/feat/b " + sha40C + " refs/heads/feat/b " + sha40D + "\n"
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
		input := "refs/heads/new " + sha40A + " refs/heads/new " + ZeroSha + "\n"
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
		input := "refs/heads/gone " + ZeroSha + " refs/heads/gone " + sha40A + "\n"
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

// Reject sha tokens that don't match git's canonical 40-char hex form.
// Without this guard, a sha-shaped token like "--upload-pack=evil" would
// flow into `git diff --name-only X..Y` and be parsed as a flag.
func TestParsePushSpecs_RejectsNonHexSha(t *testing.T) {
	t.Run("spec-manifest/AC-28 ParsePushSpecs rejects non-hex local_sha", func(t *testing.T) {
		// 40 chars, but contains uppercase + non-hex.
		input := "refs/heads/main --upload-pack=evil refs/heads/main " + ZeroSha + "\n"
		_, err := ParsePushSpecs(strings.NewReader(input))
		if err == nil {
			t.Fatal("expected error on non-hex local_sha, got nil")
		}
		if !strings.Contains(err.Error(), "local_sha") {
			t.Errorf("expected error to name local_sha, got: %v", err)
		}
	})
	t.Run("spec-manifest/AC-28 ParsePushSpecs rejects non-hex remote_sha", func(t *testing.T) {
		input := "refs/heads/main aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/main GARBAGE\n"
		_, err := ParsePushSpecs(strings.NewReader(input))
		if err == nil {
			t.Fatal("expected error on non-hex remote_sha, got nil")
		}
		if !strings.Contains(err.Error(), "remote_sha") {
			t.Errorf("expected error to name remote_sha, got: %v", err)
		}
	})
	t.Run("spec-manifest/AC-28 ParsePushSpecs rejects too-short sha", func(t *testing.T) {
		input := "refs/heads/main abc refs/heads/main " + ZeroSha + "\n"
		_, err := ParsePushSpecs(strings.NewReader(input))
		if err == nil {
			t.Fatal("expected error on short sha, got nil")
		}
	})
	t.Run("spec-manifest/AC-28 ParsePushSpecs accepts canonical 40-char hex sha", func(t *testing.T) {
		input := "refs/heads/main aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/main bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
		specs, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Fatalf("expected canonical sha to parse, got error: %v", err)
		}
		if len(specs) != 1 {
			t.Errorf("expected 1 spec, got %d", len(specs))
		}
	})
	t.Run("spec-manifest/AC-28 ParsePushSpecs accepts ZeroSha sentinel", func(t *testing.T) {
		// Already covered by TestParsePushSpecs_NewBranch_ZeroRemoteSha and
		// TestParsePushSpecs_DeletedBranch_ZeroLocalSha but verifying the new
		// regex doesn't reject the sentinel.
		input := "refs/heads/new aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/new " + ZeroSha + "\n"
		_, err := ParsePushSpecs(strings.NewReader(input))
		if err != nil {
			t.Errorf("expected ZeroSha to be valid hex, got error: %v", err)
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

// Prose mentions of @spec / @ac in non-comment-context (e.g., commit message
// text in a diff, doc prose) must NOT count as an annotation delta. Otherwise
// any commit whose message mentions an annotation could bypass the gate.
func TestHasAnnotationDelta_ProseMentionIgnored(t *testing.T) {
	t.Run("spec-manifest/AC-28 HasAnnotationDelta ignores prose mentions of @spec", func(t *testing.T) {
		// Added line is plain text (no comment marker before @spec).
		diff := "+++ b/CHANGELOG.md\n+fixes the @spec foo bug\n+see @ac AC-01 for details\n"
		if HasAnnotationDelta(diff) {
			t.Errorf("expected prose mentions not to count as annotation delta, got true")
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

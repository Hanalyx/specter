// Unit tables for the C-37 content rule. The CLI tests in cmd/specter bind
// the observable contract over real git repositories; these bind the pure
// classifier so a regression is named at the function that caused it.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

const canonBase = `package p

import (
	"example.com/x/compliance"
	"fmt"
	"sync"
	"z.example.com/y/other"
)

var _ = fmt.Sprint
var _ sync.Mutex
var _ = compliance.X
var _ = other.Y

// f is documented.
func f(err error) error {
	if err != nil {
		return err
	}
	return nil
}
`

const canonGrouped = `package p

import (
	"fmt"
	"sync"

	"example.com/x/compliance"
	"z.example.com/y/other"
)

var _ = fmt.Sprint
var _ sync.Mutex
var _ = compliance.X
var _ = other.Y

// f is documented.
func f(err error) error {
	if err != nil {
		return err
	}
	return nil
}
`

func sub(t *testing.T, src, old, new string) string {
	t.Helper()
	if !strings.Contains(src, old) {
		t.Fatalf("fixture substitution %q did not land", old)
	}
	return strings.Replace(src, old, new, 1)
}

// @ac AC-68
func TestCanonicalGo_ErasesImportGrouping(t *testing.T) {
	t.Run("spec-manifest/AC-68 goimports grouping has one canonical form", func(t *testing.T) {
		a, err := CanonicalGo([]byte(canonBase))
		if err != nil {
			t.Fatal(err)
		}
		b, err := CanonicalGo([]byte(canonGrouped))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("AC-68: the two groupings canonicalize differently:\n--- base ---\n%s\n--- grouped ---\n%s", a, b)
		}
		// The canonical form must still be the same program, not an empty
		// or truncated one. Every import path survives.
		for _, p := range []string{`"fmt"`, `"sync"`, `"example.com/x/compliance"`, `"z.example.com/y/other"`} {
			if !strings.Contains(string(a), p) {
				t.Errorf("AC-68: canonical form lost import %s", p)
			}
		}
		// Canonical form is idempotent, so it is a normal form and not a
		// one-way hash of the input.
		again, err := CanonicalGo(a)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(a) {
			t.Errorf("AC-68: CanonicalGo is not idempotent")
		}
	})

	t.Run("spec-manifest/AC-68 a format-only file leaves the summary, beside a test edit", func(t *testing.T) {
		changes := []FileChange{
			{Path: "internal/scheduler/persist.go", Status: 'M', Base: []byte(canonBase), Head: []byte(canonGrouped)},
			{Path: "internal/worker/x_test.go", Status: 'M'},
		}
		s := SummarizePushDiff(changes, "+++ b/internal/worker/x_test.go\n+func TestG(t *testing.T) {}\n")
		if len(s.ImplFilesChanged) != 0 {
			t.Errorf("AC-68: format-only file still counted: %v", s.ImplFilesChanged)
		}
		if len(s.CompareFailures) != 0 {
			t.Errorf("AC-68: a successful comparison recorded a failure: %v", s.CompareFailures)
		}
		if ShouldBlockPush(s) {
			t.Error("AC-68: ShouldBlockPush blocked a range with no implementation change")
		}
	})
}

// @ac AC-69
func TestFormatOnly_SemanticChangesAreNotFormatting(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"import added and used", sub(t, sub(t, canonBase,
			"\t\"sync\"\n", "\t\"strings\"\n\t\"sync\"\n"),
			"var _ sync.Mutex\n", "var _ sync.Mutex\nvar _ = strings.TrimSpace\n")},
		{"import removed", sub(t, sub(t, canonBase, "\t\"sync\"\n", ""), "var _ sync.Mutex\n", "")},
		{"condition flipped", sub(t, canonBase, "err != nil", "err == nil")},
		{"alias added", sub(t, sub(t, canonBase,
			"\t\"example.com/x/compliance\"\n", "\tc \"example.com/x/compliance\"\n"),
			"compliance.X", "c.X")},
		// Not in AC-69's list, but C-37 names both as still counting: a
		// formatter does not write comments or insert blank lines.
		{"comment edited", sub(t, canonBase, "// f is documented.", "// f is edited.")},
		{"blank line inserted", sub(t, canonBase, "\t\treturn err\n\t}\n", "\t\treturn err\n\t}\n\n")},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-69 "+tc.name+" still counts", func(t *testing.T) {
			same, err := formatOnly("impl.go", []byte(canonBase), []byte(tc.head))
			if err != nil {
				t.Fatalf("AC-69: %s did not parse: %v", tc.name, err)
			}
			if same {
				t.Errorf("AC-69: %s canonicalized equal to the base, so the rule would erase a real change", tc.name)
			}
			s := SummarizePushDiff([]FileChange{{Path: "impl.go", Status: 'M', Base: []byte(canonBase), Head: []byte(tc.head)}}, "")
			if len(s.ImplFilesChanged) != 1 {
				t.Errorf("AC-69: %s left the summary: %v", tc.name, s.ImplFilesChanged)
			}
		})
	}
}

// @ac AC-71
func TestSummarizePushDiff_FailsClosed(t *testing.T) {
	t.Run("spec-manifest/AC-71 a language with no canonical form counts without a failure", func(t *testing.T) {
		s := SummarizePushDiff([]FileChange{{Path: "impl.py", Status: 'M', Base: []byte("x = 1\n"), Head: []byte("x = 1 \n")}}, "")
		if len(s.ImplFilesChanged) != 1 {
			t.Errorf("AC-71: .py file left the summary")
		}
		if len(s.CompareFailures) != 0 {
			t.Errorf("AC-71: nothing was attempted for .py, yet a failure was recorded: %v", s.CompareFailures)
		}
	})

	t.Run("spec-manifest/AC-71 an unparseable side counts and names the file", func(t *testing.T) {
		for _, tc := range []struct{ name, base, head string }{
			{"base", "package p\n\nfunc (\n", canonBase},
			{"head", canonBase, "package p\n\nfunc (\n"},
		} {
			s := SummarizePushDiff([]FileChange{{Path: "impl.go", Status: 'M', Base: []byte(tc.base), Head: []byte(tc.head)}}, "")
			if len(s.ImplFilesChanged) != 1 {
				t.Errorf("AC-71: unparseable %s left the summary", tc.name)
			}
			if len(s.CompareFailures) != 1 || !strings.HasPrefix(s.CompareFailures[0], "impl.go: "+tc.name+" does not parse") {
				t.Errorf("AC-71: unparseable %s not reported as such: %v", tc.name, s.CompareFailures)
			}
		}
	})

	t.Run("spec-manifest/AC-71 an unreadable blob counts and carries its reason", func(t *testing.T) {
		s := SummarizePushDiff([]FileChange{{Path: "impl.go", Status: 'M', ReadError: "base blob unreadable: fatal: unable to read 1234"}}, "")
		if len(s.ImplFilesChanged) != 1 {
			t.Errorf("AC-71: unreadable file left the summary")
		}
		if len(s.CompareFailures) != 1 || s.CompareFailures[0] != "impl.go: base blob unreadable: fatal: unable to read 1234" {
			t.Errorf("AC-71: reason not carried through: %v", s.CompareFailures)
		}
		msg := FormatBlockedPushMessage(s)
		if !strings.Contains(msg, "could not compare impl.go: base blob unreadable") {
			t.Errorf("AC-71: block message does not name the file and reason:\n%s", msg)
		}
	})

	t.Run("spec-manifest/AC-71 added and deleted files count even with equal blobs", func(t *testing.T) {
		// Equal blobs on an A or D would only arise from a caller bug, but
		// if one ever did, the status must win: there is no base to
		// compare against on an add, and no head on a delete.
		for _, st := range []byte{'A', 'D', 0} {
			s := SummarizePushDiff([]FileChange{{Path: "impl.go", Status: st, Base: []byte(canonBase), Head: []byte(canonBase)}}, "")
			if len(s.ImplFilesChanged) != 1 {
				t.Errorf("AC-71: status %q with equal blobs left the summary", st)
			}
			if len(s.CompareFailures) != 0 {
				t.Errorf("AC-71: status %q recorded a failure though nothing was compared: %v", st, s.CompareFailures)
			}
		}
	})
}

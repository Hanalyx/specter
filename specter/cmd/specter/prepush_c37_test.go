// CLI integration tests for spec-manifest C-37: pre-push-check classifies a
// changed implementation file by content, not by name.
//
// Every case builds a real git repository, commits a base and a head, and
// feeds git's pre-push stdin line to the command, so the decision is read
// off the exit code the hook would see. Nothing here reaches into the
// classifier; the observable contract is the whole test.
//
// @spec spec-manifest
package main

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// c37Base is the pre-image every C-37 case starts from: gofmt-clean, every
// import in one sorted group, one condition to flip, one alias to add.
const c37Base = `package p

import (
	"example.com/x/compliance"
	"fmt"
	"sync"
)

var _ = fmt.Sprint
var _ sync.Mutex
var _ = compliance.X

func f(err error) error {
	if err != nil {
		return err
	}
	return nil
}
`

// c37FormatOnly is c37Base after goimports moves the third-party import into
// its own group. Same import set, same statements, two lines moved.
const c37FormatOnly = `package p

import (
	"fmt"
	"sync"

	"example.com/x/compliance"
)

var _ = fmt.Sprint
var _ sync.Mutex
var _ = compliance.X

func f(err error) error {
	if err != nil {
		return err
	}
	return nil
}
`

// c37TestFile carries the annotations in the base, so no range adds one.
const c37TestFile = "package p\n\nimport \"testing\"\n\n// @spec spec-foo\n// @ac AC-01\nfunc TestF(t *testing.T) {}\n"

// c37ExecChange flips the one condition. The import set is untouched.
var c37ExecChange = strings.Replace(c37Base, "err != nil", "err == nil", 1)

// setupC37Repo commits c37Base and the annotated test file and returns the
// repo directory and the base sha.
func setupC37Repo(t *testing.T) (string, string) {
	t.Helper()
	dir := setupBareGitRepo(t)
	if err := writeFileAt(dir, "impl.go", c37Base); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAt(dir, "impl_test.go", c37TestFile); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, dir, "add", "-A")
	runGitInDir(t, dir, "commit", "-q", "-m", "base")
	return dir, runGitInDir(t, dir, "rev-parse", "HEAD")
}

// commitC37 writes the named files, commits them with msg, and returns the
// new head sha.
func commitC37(t *testing.T, dir, msg string, files map[string]string) string {
	t.Helper()
	for name, body := range files {
		if err := writeFileAt(dir, name, body); err != nil {
			t.Fatal(err)
		}
	}
	runGitInDir(t, dir, "add", "-A")
	runGitInDir(t, dir, "commit", "-q", "-m", msg)
	return runGitInDir(t, dir, "rev-parse", "HEAD")
}

// pushLine is git's pre-push stdin for one ref moving from base to head.
func pushLine(head, base string) string {
	return "refs/heads/main " + head + " refs/heads/main " + base + "\n"
}

// @ac AC-68
func TestPrePushCheck_C37_FormatOnlyPasses(t *testing.T) {
	t.Run("spec-manifest/AC-68 the base is what gofmt would write", func(t *testing.T) {
		// Positive control on the fixture. If gofmt would rewrite the base,
		// the head is not a formatter's output over a clean file and the
		// case below tests something other than the recorded push.
		got, err := format.Source([]byte(c37Base))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c37Base {
			t.Fatalf("AC-68: c37Base is not gofmt-clean, so the fixture does not match the recorded push:\n%s", got)
		}
		if strings.Join(sortedImports(c37Base), ",") != strings.Join(sortedImports(c37FormatOnly), ",") {
			t.Fatal("AC-68: the two fixtures do not share an import set, so this is not a format-only case")
		}
	})

	t.Run("spec-manifest/AC-68 goimports grouping alone exits 0", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "goimports", map[string]string{"impl.go": c37FormatOnly})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code != 0 {
			t.Errorf("AC-68: a formatter-only range exited %d, want 0. The hook classified impl.go by name and blocked a push that changes no symbol.\n%s", code, out)
		}
	})

	t.Run("spec-manifest/AC-68 the same move beside a test edit with no annotation exits 0", func(t *testing.T) {
		// The shape of both recorded pushes: the implementation file moves
		// an import, and a test file changes without touching an
		// annotation. Classification has to cover the whole range.
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "goimports and a test edit", map[string]string{
			"impl.go":      c37FormatOnly,
			"impl_test.go": c37TestFile + "\nfunc TestG(t *testing.T) {}\n",
		})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code != 0 {
			t.Errorf("AC-68: format-only move plus an annotation-free test edit exited %d, want 0.\n%s", code, out)
		}
	})
}

// sortedImports returns the import paths of a Go source, sorted, so two
// fixtures can be compared on the set alone.
func sortedImports(src string) []string {
	var paths []string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"`) || strings.Contains(line, ` "`) {
			if i := strings.Index(line, `"`); i >= 0 {
				paths = append(paths, line[i:])
			}
		}
	}
	// Insertion sort; the lists are three entries long.
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && paths[j] < paths[j-1]; j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
	return paths
}

// @ac AC-69
func TestPrePushCheck_C37_SemanticChangesStillBlock(t *testing.T) {
	cases := []struct {
		name string
		head string
	}{
		{"import added and used", strings.Replace(strings.Replace(c37Base,
			"\t\"sync\"\n", "\t\"strings\"\n\t\"sync\"\n", 1),
			"var _ sync.Mutex\n", "var _ sync.Mutex\nvar _ = strings.TrimSpace\n", 1)},
		{"import removed", strings.Replace(strings.Replace(c37Base,
			"\t\"sync\"\n", "", 1),
			"var _ sync.Mutex\n", "", 1)},
		{"condition flipped", c37ExecChange},
		{"alias added", strings.Replace(strings.Replace(c37Base,
			"\t\"example.com/x/compliance\"\n", "\tc \"example.com/x/compliance\"\n", 1),
			"compliance.X", "c.X", 1)},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-69 "+tc.name+" exits non-zero", func(t *testing.T) {
			if tc.head == c37Base {
				t.Fatalf("AC-69: the %q fixture equals the base, so the substitution did not land", tc.name)
			}
			dir, base := setupC37Repo(t)
			head := commitC37(t, dir, tc.name, map[string]string{"impl.go": tc.head})
			out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
			if code == 0 {
				t.Errorf("AC-69: %s exited 0, want non-zero. Canonical formatting must not erase a change to the import set or the executable code.", tc.name)
			}
			if !strings.Contains(out, "impl.go") {
				t.Errorf("AC-69: %s blocked without naming impl.go:\n%s", tc.name, out)
			}
		})
	}
}

// @ac AC-70
func TestPrePushCheck_C37_ReadsPushedBlobsNotWorkingTree(t *testing.T) {
	t.Run("spec-manifest/AC-70 a format-only range passes despite an executable edit in the working tree and the index", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "goimports", map[string]string{"impl.go": c37FormatOnly})

		// Unstaged executable edit on top of the committed range.
		if err := writeFileAt(dir, "impl.go", c37ExecChange); err != nil {
			t.Fatal(err)
		}
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code != 0 {
			t.Errorf("AC-70: unstaged working-tree edit changed the verdict to %d, want 0. The classifier read the working tree, not the pushed head.\n%s", code, out)
		}

		// The same edit staged. Still not part of the pushed range.
		runGitInDir(t, dir, "add", "impl.go")
		out, code = runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code != 0 {
			t.Errorf("AC-70: staged edit changed the verdict to %d, want 0. The classifier read the index, not the pushed head.\n%s", code, out)
		}
	})

	t.Run("spec-manifest/AC-70 an executable range blocks despite the working tree holding the base content", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "flip", map[string]string{"impl.go": c37ExecChange})
		if err := writeFileAt(dir, "impl.go", c37Base); err != nil {
			t.Fatal(err)
		}
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-70: working tree restored to base made an executable range pass. The classifier read the working tree, not the pushed head.\n%s", out)
		}
	})
}

// @ac AC-71
func TestPrePushCheck_C37_FailsClosed(t *testing.T) {
	t.Run("spec-manifest/AC-71 a language with no canonical form still counts", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		if err := writeFileAt(dir, "impl.py", "x = 1\n"); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", "-A")
		runGitInDir(t, dir, "commit", "-q", "-m", "base")
		base := runGitInDir(t, dir, "rev-parse", "HEAD")
		head := commitC37(t, dir, "whitespace", map[string]string{"impl.py": "x = 1 \n"})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-71: a whitespace-only .py change exited 0. Python has no canonical form in this tool, so the file must count.\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-71 a base that does not parse still counts", func(t *testing.T) {
		dir := setupBareGitRepo(t)
		if err := writeFileAt(dir, "impl.go", "package p\n\nfunc (\n"); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", "-A")
		runGitInDir(t, dir, "commit", "-q", "-m", "broken base")
		base := runGitInDir(t, dir, "rev-parse", "HEAD")
		head := commitC37(t, dir, "fixed", map[string]string{"impl.go": c37Base})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-71: an unparseable base exited 0. A comparison that cannot be made must block.\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-71 an unreadable base blob blocks and is named", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "goimports", map[string]string{"impl.go": c37FormatOnly})

		// Remove the base blob from the object store, so git show cannot
		// read the pre-image. A fresh repository keeps objects loose.
		blob := runGitInDir(t, dir, "rev-parse", base+":impl.go")
		obj := filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])
		if err := os.Remove(obj); err != nil {
			t.Fatalf("AC-71: could not remove the base blob %s: %v", obj, err)
		}

		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-71: an unreadable base blob exited 0. The hook assumed formatting where it could not compare.\n%s", out)
		}
		if !strings.Contains(out, "could not compare impl.go") {
			t.Errorf("AC-71: the block message does not say which file could not be compared:\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-71 a file added in the range still counts", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "new file", map[string]string{"new.go": "package p\n\nvar Added = 1\n"})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-71: an added implementation file exited 0. A file with no base is a change, not a normalization.\n%s", out)
		}
	})
}

// @ac AC-72
func TestPrePushCheck_C37_NoDeclaredEscape(t *testing.T) {
	t.Run("spec-manifest/AC-72 a format-only flag does not exist", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		head := commitC37(t, dir, "flip", map[string]string{"impl.go": c37ExecChange})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check", "--format-only")
		if code == 0 {
			t.Errorf("AC-72: --format-only was accepted. No flag may declare a range format-only.\n%s", out)
		}
		if !strings.Contains(out, "unknown flag") {
			t.Errorf("AC-72: the rejection does not say the flag is unknown:\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-72 a commit trailer does not declare a range format-only", func(t *testing.T) {
		dir, base := setupC37Repo(t)
		if err := writeFileAt(dir, "impl.go", c37ExecChange); err != nil {
			t.Fatal(err)
		}
		runGitInDir(t, dir, "add", "-A")
		runGitInDir(t, dir, "commit", "-q", "-m", "flip\n\nFormat-Only: true")
		head := runGitInDir(t, dir, "rev-parse", "HEAD")
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-72: a Format-Only trailer made an executable change pass.\n%s", out)
		}
	})

	t.Run("spec-manifest/AC-72 the command reads no manifest", func(t *testing.T) {
		// A manifest that does not parse changes neither verdict, which is
		// only possible if no manifest key can reach the decision.
		dir, base := setupC37Repo(t)
		if err := writeFileAt(dir, "specter.yaml", ": : not yaml\n"); err != nil {
			t.Fatal(err)
		}
		head := commitC37(t, dir, "goimports", map[string]string{"impl.go": c37FormatOnly})
		out, code := runCLIWithStdin(t, dir, pushLine(head, base), "pre-push-check")
		if code != 0 {
			t.Errorf("AC-72: with an unparseable manifest the format-only range exited %d, want 0.\n%s", code, out)
		}

		head2 := commitC37(t, dir, "flip", map[string]string{"impl.go": c37ExecChange})
		out, code = runCLIWithStdin(t, dir, pushLine(head2, head), "pre-push-check")
		if code == 0 {
			t.Errorf("AC-72: with an unparseable manifest an executable change exited 0.\n%s", out)
		}
	})
}

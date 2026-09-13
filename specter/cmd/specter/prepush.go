// `specter pre-push-check` — internal subcommand invoked by the git
// pre-push hook installed via `specter init --install-hook`. Reads git's
// pre-push stdin format, runs `git diff` for each ref, and exits non-zero
// when ShouldBlockPush returns true.
//
// Hidden from `specter --help` because users don't invoke it directly.
//
// @spec spec-manifest
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Hanalyx/specter/internal/manifest"
	"github.com/spf13/cobra"
)

func prePushCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "pre-push-check",
		Short:  "Internal: read git pre-push stdin and decide whether to block",
		Long:   "Invoked by the git pre-push hook installed via `specter init --install-hook`. Not intended for direct use.",
		Hidden: true,
		Args:   cobra.ArbitraryArgs, // git passes hook args; we don't use them
		RunE: func(cmd *cobra.Command, args []string) error {
			specs, err := manifest.ParsePushSpecs(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "specter pre-push-check:", err)
				return errSilent
			}
			if len(specs) == 0 {
				return nil
			}

			for _, p := range specs {
				// Skip deleted-branch refs (no impl change to evaluate).
				if p.LocalSha == manifest.ZeroSha {
					continue
				}

				base := pickDiffBase(p)
				if base == "" {
					// Couldn't determine a base — skip rather than block.
					// Common case: brand-new branch with no merge-base
					// against any remote ref. The next push (after the
					// branch lands) will have a real base.
					continue
				}

				// C-37: a range that cannot be classified is not assumed
				// safe. Both failures below block; neither skips the ref.
				changes, err := gitDiffChanges(base, p.LocalSha)
				if err != nil {
					fmt.Fprintf(os.Stderr, "specter pre-push-check: cannot list %s..%s: %v\n", base, p.LocalSha, err)
					fmt.Fprint(os.Stderr, "specter pre-push: push blocked. A range that cannot be classified is not read as safe.\n")
					return errSilent
				}
				diff, err := gitDiffUnified(base, p.LocalSha)
				if err != nil {
					// The annotation delta is unknown, and unknown is
					// not "present". The per-file compare failures
					// already carry the readable reason.
					fmt.Fprintf(os.Stderr, "specter pre-push-check: git diff %s..%s: %v; reading the range as carrying no annotation delta\n", base, p.LocalSha, err)
					diff = ""
				}

				summary := manifest.SummarizePushDiff(changes, diff)
				if manifest.ShouldBlockPush(summary) {
					fmt.Fprint(os.Stderr, manifest.FormatBlockedPushMessage(summary))
					return errSilent
				}
			}
			return nil
		},
	}
}

// pickDiffBase chooses the commit-range base for one pushed ref. For an
// existing remote ref, base = remote sha. For a new branch (remote sha is
// ZeroSha), use the merge-base against `origin/HEAD` if available, else
// skip (return ""). Skipping is safer than blocking on first push — there's
// no "before" to compare against.
func pickDiffBase(p manifest.PushSpec) string {
	if p.RemoteSha != manifest.ZeroSha {
		return p.RemoteSha
	}
	// New branch — try merge-base against origin/HEAD.
	out, err := exec.Command("git", "merge-base", p.LocalSha, "origin/HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitDiffChanges lists the files changed between base and head with their
// status letter, and reads both blobs of every modified implementation file
// from the two commits. The working tree and the index are never consulted:
// what is compared is what is being pushed. --no-renames keeps the letters
// to A, M, and D, so a moved file reads as a delete and an add, both of
// which count.
func gitDiffChanges(base, head string) ([]manifest.FileChange, error) {
	out, err := exec.Command("git", "diff", "--name-status", "--no-renames", base+".."+head).Output()
	if err != nil {
		return nil, err
	}
	var changes []manifest.FileChange
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("unexpected --name-status line %q", line)
		}
		c := manifest.FileChange{Path: parts[1], Status: parts[0][0]}
		if c.Status == 'M' && manifest.IsImplFile(c.Path) {
			c.Base, c.Head, c.ReadError = readBlobPair(base, head, c.Path)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// readBlobPair reads one path at the base commit and at the head commit. A
// side that cannot be read yields a reason instead of contents, and the
// classifier counts the file.
func readBlobPair(base, head, path string) (b, h []byte, readErr string) {
	var err error
	if b, err = gitShowBlob(base, path); err != nil {
		return nil, nil, "base blob unreadable: " + err.Error()
	}
	if h, err = gitShowBlob(head, path); err != nil {
		return nil, nil, "head blob unreadable: " + err.Error()
	}
	return b, h, ""
}

// gitShowBlob returns the blob at rev:path, a path as git lists it, relative
// to the repository root. The git error text is kept, because
// "unable to read" and "does not exist" are different facts to a reader.
func gitShowBlob(rev, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", rev+":"+path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// gitDiffUnified returns the unified-diff output between base and head.
// `-U0` keeps the diff small (no surrounding context); we only care about
// added/removed annotation lines.
func gitDiffUnified(base, head string) (string, error) {
	out, err := exec.Command("git", "diff", "-U0", base+".."+head).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

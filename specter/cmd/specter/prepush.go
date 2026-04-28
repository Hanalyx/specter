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

				files, err := gitDiffFilenames(base, p.LocalSha)
				if err != nil {
					fmt.Fprintf(os.Stderr, "specter pre-push-check: git diff --name-only %s..%s: %v\n", base, p.LocalSha, err)
					continue
				}
				diff, err := gitDiffUnified(base, p.LocalSha)
				if err != nil {
					fmt.Fprintf(os.Stderr, "specter pre-push-check: git diff %s..%s: %v\n", base, p.LocalSha, err)
					continue
				}

				summary := manifest.SummarizePushDiff(files, diff)
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

// gitDiffFilenames returns the list of changed file paths between base and head.
func gitDiffFilenames(base, head string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", base+".."+head).Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
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

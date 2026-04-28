// Pre-push hook helpers: parse git's pre-push stdin format, detect
// annotation deltas in unified-diff output, and roll the result into a
// PushDiffSummary that ShouldBlockPush can consume.
//
// All functions here are pure (no I/O). The CLI subcommand
// `specter pre-push-check` shells out to git and feeds the results in.
//
// @spec spec-manifest
package manifest

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ZeroSha is git's "no commit" sentinel. Used as the remote sha for new
// branches and as the local sha for deleted branches.
const ZeroSha = "0000000000000000000000000000000000000000"

// validShaRE constrains LocalSha and RemoteSha to git's canonical 40-char
// lowercase hex form. Without this guard, a sha-shaped token from stdin
// could carry a leading `--` and flow into `git diff --name-only X..Y`
// as a flag rather than a ref. Refer to the pre-push hook contract: git
// emits 40-char hex (or all-zeros sentinel) on every line; anything else
// is malformed input and we reject it.
var validShaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

// PushSpec describes one ref being pushed, parsed from a single line of
// git's pre-push stdin format: `local_ref local_sha remote_ref remote_sha`.
type PushSpec struct {
	LocalRef  string
	LocalSha  string
	RemoteRef string
	RemoteSha string
}

// ParsePushSpecs reads git's pre-push stdin (one line per ref, four
// space-separated tokens) and returns the parsed specs in order.
// Empty input returns an empty slice with nil error. Malformed lines
// (wrong token count, or sha fields not matching the canonical 40-char
// hex form) return an error.
func ParsePushSpecs(r io.Reader) ([]PushSpec, error) {
	var specs []PushSpec
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 4 {
			return nil, fmt.Errorf("pre-push line must have 4 fields (local_ref local_sha remote_ref remote_sha), got %d: %q", len(tokens), line)
		}
		// Reject sha fields that don't match git's 40-char hex form.
		// This is the canonical input shape from `git push`'s stdin.
		if !validShaRE.MatchString(tokens[1]) {
			return nil, fmt.Errorf("pre-push line has malformed local_sha %q (expected 40-char hex)", tokens[1])
		}
		if !validShaRE.MatchString(tokens[3]) {
			return nil, fmt.Errorf("pre-push line has malformed remote_sha %q (expected 40-char hex)", tokens[3])
		}
		specs = append(specs, PushSpec{
			LocalRef:  tokens[0],
			LocalSha:  tokens[1],
			RemoteRef: tokens[2],
			RemoteSha: tokens[3],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pre-push stdin: %w", err)
	}
	return specs, nil
}

// annotationLineRE matches a real `@spec` or `@ac` annotation — i.e. one
// preceded by a source-comment marker (`//`, `#`, or `*`). Prose mentions
// like "fixes the @spec foo issue" no longer count, since the AC-28 gate
// is about test-annotation deltas, not arbitrary text containing @spec.
var annotationLineRE = regexp.MustCompile(`(?://|#|\*)\s*@(?:spec|ac)\s`)

// HasAnnotationDelta reports whether the unified-diff output contains any
// added or removed line carrying `@spec ` or `@ac `. Pure scan over the
// diff text. Headers (+++ / ---) and context lines (no leading + or -)
// are ignored.
//
// We count both additions and removals as "delta" — a removed @ac is
// still a change to the annotation set, and a code commit that drops a
// test annotation should not be invisible to the gate.
func HasAnnotationDelta(diff string) bool {
	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		// Skip diff file headers.
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if line[0] != '+' && line[0] != '-' {
			continue
		}
		// `+` or `-` followed by the line content.
		body := line[1:]
		if annotationLineRE.MatchString(body) {
			return true
		}
	}
	return false
}

// SummarizePushDiff combines file categorization (IsImplFile / isTestFile /
// isSpecFile / isDocFile) with HasAnnotationDelta into one PushDiffSummary
// that ShouldBlockPush can consume directly.
func SummarizePushDiff(filenames []string, diff string) PushDiffSummary {
	var s PushDiffSummary
	for _, f := range filenames {
		switch {
		case isTestFile(f):
			s.TestFilesChanged = append(s.TestFilesChanged, f)
		case isSpecFile(f):
			s.SpecFilesChanged = append(s.SpecFilesChanged, f)
		case isDocFile(f):
			s.DocFilesChanged = append(s.DocFilesChanged, f)
		case IsImplFile(f):
			s.ImplFilesChanged = append(s.ImplFilesChanged, f)
		}
	}
	s.AnnotationDelta = HasAnnotationDelta(diff)
	return s
}

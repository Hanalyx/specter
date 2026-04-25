// Fenced-region read/write helpers for `init --install-hook` and
// `init --ai <tool>`. Both commands write content into a file alongside
// user-authored content; the fenced markers let re-runs replace only the
// Specter-managed region byte-for-byte preserving everything outside.
//
// Marker format depends on target file syntax:
//   - Markdown / HTML: HTML-comment markers render as nothing in previews.
//     Use MarkdownMarkers.
//   - Shell scripts: HTML comments are not shell comments — `<!--` parses
//     as a redirection token (`<` plus `!--`), causing a syntax error on
//     the next newline. Use ShellMarkers (`#`-prefixed).
//
// The version tag (v1) lets future Specter releases migrate the format
// without ambiguity. Today only v1 is recognized.
//
// @spec spec-manifest
package manifest

import (
	"errors"
	"fmt"
	"strings"
)

// FencedMarkers names the begin/end strings that wrap the Specter-managed
// region in a generated file. Use MarkdownMarkers for AI instruction files;
// ShellMarkers for the pre-push hook.
type FencedMarkers struct {
	Begin string
	End   string
}

// MarkdownMarkers wraps content in HTML comments. Renders invisibly in
// rendered markdown; safe in any markdown-compatible file.
func MarkdownMarkers(version string) FencedMarkers {
	return FencedMarkers{
		Begin: fmt.Sprintf("<!-- specter:begin %s -->", version),
		End:   "<!-- specter:end -->",
	}
}

// ShellMarkers wraps content in shell-comment lines. Required for any file
// parsed by `sh` / `bash` / `zsh` (e.g., git hooks). HTML-comment markers
// would be a syntax error in shell.
func ShellMarkers(version string) FencedMarkers {
	return FencedMarkers{
		Begin: fmt.Sprintf("# specter:begin %s", version),
		End:   "# specter:end",
	}
}

// ReplaceFencedRegion replaces (or appends) the specter-managed region in
// `original`, leaving everything outside the fence byte-for-byte unchanged.
//
// Behavior:
//   - If both begin and end markers are present: replace just the in-fence body.
//   - If neither marker is present: append the fenced block (with a leading
//     blank line if `original` is non-empty and doesn't already end with \n).
//   - If only one marker is present: error rather than guess. Operator
//     intervention required to avoid corrupting hand-edited files.
func ReplaceFencedRegion(original string, markers FencedMarkers, body string) (string, error) {
	begin := markers.Begin
	end := markers.End

	beginIdx := strings.Index(original, begin)
	endIdx := strings.Index(original, end)

	switch {
	case beginIdx >= 0 && endIdx >= 0:
		if endIdx < beginIdx {
			return "", fmt.Errorf("malformed fenced region: end marker appears before begin marker")
		}
		// Cut from begin marker up to (and including) end marker.
		endStop := endIdx + len(end)
		var b strings.Builder
		b.WriteString(original[:beginIdx])
		b.WriteString(begin)
		b.WriteString("\n")
		b.WriteString(strings.TrimRight(body, "\n"))
		b.WriteString("\n")
		b.WriteString(end)
		b.WriteString(original[endStop:])
		return b.String(), nil

	case beginIdx >= 0 || endIdx >= 0:
		return "", errors.New("unterminated fenced region: only one of <!-- specter:begin --> / <!-- specter:end --> markers found; refusing to overwrite to avoid corruption")

	default:
		// Append a fresh fenced block.
		var b strings.Builder
		b.WriteString(original)
		if len(original) > 0 && !strings.HasSuffix(original, "\n") {
			b.WriteString("\n")
		}
		if len(original) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(begin)
		b.WriteString("\n")
		b.WriteString(strings.TrimRight(body, "\n"))
		b.WriteString("\n")
		b.WriteString(end)
		b.WriteString("\n")
		return b.String(), nil
	}
}

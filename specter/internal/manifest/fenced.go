// Fenced-region read/write helpers for `init --install-hook` and
// `init --ai <tool>`. Both commands write content into a file alongside
// user-authored content; the fenced markers let re-runs replace only the
// Specter-managed region byte-for-byte preserving everything outside.
//
// Marker format (HTML comments so they render as nothing in markdown
// previews; safely ignored by shells via the `# ...` line comments):
//
//	<!-- specter:begin v1 -->
//	... specter-managed content ...
//	<!-- specter:end -->
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

// ReplaceFencedRegion replaces (or appends) the specter-managed region in
// `original`, leaving everything outside the fence byte-for-byte unchanged.
//
// Behavior:
//   - If both begin and end markers are present: replace just the in-fence body.
//   - If neither marker is present: append the fenced block (with a leading
//     blank line if `original` is non-empty and doesn't already end with \n).
//   - If only one marker is present: error rather than guess. Operator
//     intervention required to avoid corrupting hand-edited files.
func ReplaceFencedRegion(original, version, body string) (string, error) {
	begin := fmt.Sprintf("<!-- specter:begin %s -->", version)
	end := "<!-- specter:end -->"

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

// Pure-function tests for the fenced-region read/write helpers used by
// `init --install-hook` and `init --ai <tool>`.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// AC-35 backbone: re-running init replaces only the in-fence content;
// out-of-fence content is preserved byte-for-byte.
func TestFencedRegion_ReplaceFenced_PreservesOutOfFence(t *testing.T) {
	t.Run("spec-manifest/AC-35 fenced replace preserves out-of-fence content", func(t *testing.T) {
		original := "# User intro\n\nSome notes the user wrote.\n" +
			"<!-- specter:begin v1 -->\nold body\n<!-- specter:end -->\n" +
			"# Trailing user content\nMore of the user's notes.\n"
		newBody := "fresh body line 1\nfresh body line 2"

		got, err := ReplaceFencedRegion(original, MarkdownMarkers("v1"), newBody)
		if err != nil {
			t.Fatalf("ReplaceFencedRegion: %v", err)
		}
		if !strings.Contains(got, "# User intro\n\nSome notes the user wrote.\n") {
			t.Errorf("expected pre-fence content preserved, got:\n%s", got)
		}
		if !strings.Contains(got, "# Trailing user content\nMore of the user's notes.\n") {
			t.Errorf("expected post-fence content preserved, got:\n%s", got)
		}
		if !strings.Contains(got, "fresh body line 1\nfresh body line 2") {
			t.Errorf("expected new body in fence, got:\n%s", got)
		}
		if strings.Contains(got, "old body") {
			t.Errorf("expected old body removed, got:\n%s", got)
		}
	})
}

// First-write case: input has no fence yet.
func TestFencedRegion_NoExistingFence_AppendsFenced(t *testing.T) {
	t.Run("spec-manifest/fenced first-write appends fenced region", func(t *testing.T) {
		original := "# Pre-existing user content\n"
		newBody := "specter body"

		got, err := ReplaceFencedRegion(original, MarkdownMarkers("v1"), newBody)
		if err != nil {
			t.Fatalf("ReplaceFencedRegion: %v", err)
		}
		if !strings.Contains(got, "# Pre-existing user content") {
			t.Errorf("expected user content preserved")
		}
		if !strings.Contains(got, "<!-- specter:begin v1 -->") {
			t.Errorf("expected begin marker added")
		}
		if !strings.Contains(got, "<!-- specter:end -->") {
			t.Errorf("expected end marker added")
		}
		if !strings.Contains(got, "specter body") {
			t.Errorf("expected new body present")
		}
	})
}

// Empty input: write the fenced region as-is.
func TestFencedRegion_EmptyInput_WritesFencedOnly(t *testing.T) {
	t.Run("spec-manifest/fenced empty input writes only fence", func(t *testing.T) {
		got, err := ReplaceFencedRegion("", MarkdownMarkers("v1"), "body")
		if err != nil {
			t.Fatalf("ReplaceFencedRegion: %v", err)
		}
		if !strings.Contains(got, "<!-- specter:begin v1 -->") || !strings.Contains(got, "<!-- specter:end -->") {
			t.Errorf("expected both markers, got:\n%s", got)
		}
		if !strings.Contains(got, "body") {
			t.Errorf("expected body present, got:\n%s", got)
		}
	})
}

// Mismatched markers: begin without end, or vice versa, must error rather than silently corrupting the file.
func TestFencedRegion_UnterminatedFence_ReturnsError(t *testing.T) {
	t.Run("spec-manifest/fenced unterminated marker errors out", func(t *testing.T) {
		original := "<!-- specter:begin v1 -->\nbody"
		_, err := ReplaceFencedRegion(original, MarkdownMarkers("v1"), "new")
		if err == nil {
			t.Fatal("expected error for unterminated fence, got nil")
		}
	})
}

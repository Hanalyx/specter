// streamblock_encapsulation_test.go -- StreamBlock makes the invalid state
// unrepresentable, spec-ingest 4.0.0 C-15/AC-18.
//
// Bundling presence beside the rows is directionally right and is not enough.
// With both fields exported, any package can write
//
//	ingest.StreamBlock{Streams: rows}
//
// which compiles, reads as complete, and silently declares nothing. That is
// the exact loss the 3.0.0 cycle set out to remove, and a comment claiming a
// caller "cannot" do it does not stop a caller doing it.
//
// Asserted structurally because no runtime observation can see it: every
// behavioral test in this package passes with the fields exported, since the
// production call site happens to construct the value correctly today.
//
// @spec spec-ingest
package ingest

import (
	"reflect"
	"testing"
)

// @spec spec-ingest
// @ac AC-18
//
// C-15: presence cannot be dropped while the rows are forwarded.
func TestStreamBlockKeepsPresenceInseparableFromRows(t *testing.T) {
	t.Run("spec-ingest/AC-18 no field of StreamBlock can be set from another package", func(t *testing.T) {
		rt := reflect.TypeOf(StreamBlock{})

		// Positive control. A guard over a struct with no fields passes
		// vacuously, and a rename would produce exactly that.
		if rt.NumField() == 0 {
			t.Fatalf("AC-18: StreamBlock has no fields, so the claim below is unobserved")
		}

		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			// PkgPath is empty for an exported field and holds the defining
			// package for an unexported one.
			if f.PkgPath == "" {
				t.Errorf("AC-18: StreamBlock.%s is exported, so another package can build a StreamBlock that carries rows and declares nothing. Presence and rows have to be inseparable, not merely adjacent", f.Name)
			}
		}
	})

	t.Run("spec-ingest/AC-18 the row count is reachable without exporting the slice", func(t *testing.T) {
		// The CLI prints "across N stream(s)". Unexporting the fields without
		// giving it a way to count would push the caller back to an exported
		// slice, so the accessor is part of the same decision.
		m, ok := reflect.TypeOf(StreamBlock{}).MethodByName("Len")
		if !ok {
			t.Fatalf("AC-18: StreamBlock has no Len method. The CLI summary needs a row count, and without one the fields cannot stay unexported")
		}
		if got := m.Type.NumOut(); got != 1 || m.Type.Out(0).Kind() != reflect.Int {
			t.Errorf("AC-18: Len returns %d value(s) of kind %v, want one int", got, m.Type.Out(0).Kind())
		}
	})
}

// @spec spec-ingest
// @ac AC-18
//
// The behavior the accessors have to preserve, so unexporting the fields
// cannot be satisfied by a Len that reports something else.
//
// Lands with the implementation rather than in the red commit: it names the
// constructor directly, and a test referencing a symbol that does not exist
// yet is a build failure, which takes down the package and reads as a kill.
func TestStreamBlockAccessorsReportTheBlock(t *testing.T) {
	t.Run("spec-ingest/AC-18 a declared empty block is declared and counts zero", func(t *testing.T) {
		// The case the whole cycle turns on. If these two ever collapse into
		// one another, the laundering path is back.
		empty := newStreamBlock(true, nil)
		if got := empty.Len(); got != 0 {
			t.Errorf("AC-18: a declared empty block reports Len %d, want 0", got)
		}
		if !empty.Declared() {
			t.Errorf("AC-18: a block constructed as declared reports Declared false, so presence did not survive construction")
		}
	})

	t.Run("spec-ingest/AC-18 an absent block is undeclared and counts zero", func(t *testing.T) {
		absent := newStreamBlock(false, nil)
		if absent.Declared() {
			t.Errorf("AC-18: a block constructed as undeclared reports Declared true. C-14's back-compat omission depends on this being false")
		}
		if got := absent.Len(); got != 0 {
			t.Errorf("AC-18: an absent block reports Len %d, want 0", got)
		}
	})

	t.Run("spec-ingest/AC-18 Len counts the rows it was given", func(t *testing.T) {
		// The positive control for Len. Without it, a Len hardcoded to 0
		// satisfies both cases above.
		two := newStreamBlock(true, []StreamInfo{{Name: "go"}, {Name: "js"}})
		if got := two.Len(); got != 2 {
			t.Errorf("AC-18: a block of two rows reports Len %d, want 2", got)
		}
	})
}

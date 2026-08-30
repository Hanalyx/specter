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

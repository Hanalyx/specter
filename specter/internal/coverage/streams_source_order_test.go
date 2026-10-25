package coverage

import "testing"

// @spec spec-coverage
// @ac AC-72
//
// C-44: every violation carries the position of the row it concerns, and no
// two share a full sort key. Both hold for a ResultsFile that was never
// parsed, which is the shape a caller assembling one by hand produces.
func TestStreamValidationIndexesAHandBuiltFile(t *testing.T) {
	t.Run("spec-coverage/AC-72 a hand-built file still carries distinct positions", func(t *testing.T) {
		// Never through ParseResultsFile, so no row carries a recorded source
		// position and every one reads as zero. The fallback has to supply
		// positions rather than pass the zero value through.
		rf := &ResultsFile{
			Streams: []StreamInfo{{Name: ""}, {Name: ""}, {Name: "go"}, {Name: "go"}},
		}

		got := ValidateStreams(rf)

		type key struct {
			kind   string
			stream string
			at     int
		}
		seen := map[key]int{}
		for _, v := range got {
			at := -1
			if v.StreamIndex != nil {
				at = *v.StreamIndex
			}
			k := key{v.Kind, v.Stream, at}
			seen[k]++
			if seen[k] > 1 {
				t.Errorf("C-44: two violations share the full sort key (%s, %q, %d). Every row has its own position and the total order cannot break a tie between equal keys",
					v.Kind, v.Stream, at)
			}
		}

		// Positions, not merely distinct ones. The empty rows sit at 0 and 1
		// and the repeated name at 3, which is the file as written.
		var emptyAt []int
		var dupAt []int
		for _, v := range got {
			switch v.Kind {
			case KindEmptyStreamName:
				emptyAt = append(emptyAt, *v.StreamIndex)
			case KindDuplicateStream:
				dupAt = append(dupAt, *v.StreamIndex)
			}
		}
		if len(emptyAt) != 2 || emptyAt[0] != 0 || emptyAt[1] != 1 {
			t.Errorf("C-44: empty-name rows reported at %v, want positions [0 1]", emptyAt)
		}
		if len(dupAt) != 1 || dupAt[0] != 3 {
			t.Errorf("C-44: the repeated name reported at %v, want position [3]", dupAt)
		}
	})
}

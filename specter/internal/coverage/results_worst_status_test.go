// results_worst_status_test.go -- unit tests for spec-coverage 1.16.0 C-33,
// worst-status resolution of the results lookup (AC-38).
//
// C-33 is a statement about two unexported methods on ResultsFile, so the
// test lives in the package that declares them. The CLI-visible half of the
// same constraint is in cmd/specter/coverage_results_order_test.go.
//
// @spec spec-coverage
package coverage

import "testing"

// AC-38 states four answers over two pairs and requires the same four answers
// from two row orderings of the same facts. The two orderings are the whole
// point: a first-match lookup answers differently depending on which row it
// reaches first, so a fixture with one entry per pair cannot see the defect.
//
// Both pairs carry a passing entry, and each pair's non-passing entry has a
// different status (failed for AC-01, errored for AC-02). The two pairs
// therefore vary independently: a lookup that returns a constant, or that
// keys on the wrong pair, cannot produce both answers.
//
// @ac AC-38
func TestResultsWorstStatus_ResolvesAcrossEveryMatchingEntry(t *testing.T) {
	const worstFirst = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"errored"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"passed"}]}`
	const bestFirst = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"errored"}]}`

	// answers holds the four values AC-38 names, in one comparable shape, so
	// the "identical across both orderings" claim is one comparison rather
	// than four repeated ones.
	type answers struct {
		statusAC01 string
		statusAC02 string
		passedAC01 bool
		passedAC02 bool
	}

	read := func(t *testing.T, body string) answers {
		t.Helper()
		rf, err := ParseResultsFile([]byte(body))
		if err != nil {
			t.Fatalf("ParseResultsFile: %v", err)
		}
		return answers{
			statusAC01: rf.status("demo-spec", "AC-01"),
			statusAC02: rf.status("demo-spec", "AC-02"),
			passedAC01: rf.passed("demo-spec", "AC-01"),
			passedAC02: rf.passed("demo-spec", "AC-02"),
		}
	}

	want := answers{statusAC01: "failed", statusAC02: "errored", passedAC01: false, passedAC02: false}

	cases := []struct {
		name string
		body string
	}{
		{"spec-coverage/AC-38 worst-status resolution with each pair's non-passing row first", worstFirst},
		{"spec-coverage/AC-38 worst-status resolution with each pair's passed row first", bestFirst},
	}

	got := make([]answers, len(cases))
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got[i] = read(t, c.body)
			if got[i] != want {
				t.Errorf("results lookup must resolve to the worst matching entry;\n got %+v\nwant %+v", got[i], want)
			}
		})
	}

	t.Run("spec-coverage/AC-38 both row orderings return the same four answers", func(t *testing.T) {
		if got[0] != got[1] {
			t.Errorf("row order must not change any answer;\nnon-passing first: %+v\npassed first:      %+v", got[0], got[1])
		}
	})
}

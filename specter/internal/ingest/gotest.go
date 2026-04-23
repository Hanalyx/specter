// gotest.go — parser for `go test -json` newline-delimited output.
// C-02, C-04.
//
// @spec spec-ingest
package ingest

import (
	"bufio"
	"bytes"
	"encoding/json"
)

// goTestEvent is the subset of `go test -json` event fields we care about.
// See https://pkg.go.dev/cmd/test2json for the full schema.
type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

// ParseGoTest consumes newline-delimited go-test-json output and returns one
// TestResult per completed (pass/fail/skip) test that carries a discoverable
// (spec, AC) annotation.
//
// Annotations can come from (in order of preference):
//  1. The test name itself: `TestXyz/spec-id/AC-NN`.
//  2. Output lines carrying `// @spec <id>` and `// @ac <AC-NN>` — absorbed as
//     context for the current in-flight test.
//
// Tests without annotations are silently dropped (C-04).
func ParseGoTest(data []byte) ([]TestResult, error) {
	results, _, _, err := ParseGoTestStats(data)
	return results, err
}

// ParseGoTestStats is like ParseGoTest but also returns the total number of
// completed testcases (pass/fail/skip actions) and the names of testcases
// dropped for lacking a (spec_id, ac_id) annotation. Powers the CLI summary
// (C-09) and --verbose per-case output (C-10).
func ParseGoTestStats(data []byte) (results []TestResult, scanned int, dropped []string, err error) {
	// Per-test annotation context accumulated from output-action lines.
	type pending struct {
		specFromOutput string
		acFromOutput   string
	}
	state := make(map[string]*pending) // key: Package+"\x00"+Test

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev goTestEvent
		if jErr := json.Unmarshal(line, &ev); jErr != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		key := ev.Package + "\x00" + ev.Test
		if _, ok := state[key]; !ok {
			state[key] = &pending{}
		}

		switch ev.Action {
		case "output":
			p := state[key]
			if p.specFromOutput == "" {
				if m := bodySpecAnnotation.FindStringSubmatch(ev.Output); m != nil {
					p.specFromOutput = m[1]
				}
			}
			if p.acFromOutput == "" {
				if m := bodyACAnnotation.FindStringSubmatch(ev.Output); m != nil {
					p.acFromOutput = m[1]
				}
			}

		case "pass", "fail", "skip":
			scanned++
			specID, acID := "", ""
			if m := testNameAnnotation.FindStringSubmatch(ev.Test); m != nil {
				specID = m[1]
				acID = m[2]
			} else if p := state[key]; p.specFromOutput != "" && p.acFromOutput != "" {
				specID = p.specFromOutput
				acID = p.acFromOutput
			}
			if specID == "" || acID == "" {
				dropped = append(dropped, ev.Test)
				delete(state, key)
				continue // C-04: silent drop
			}
			results = append(results, TestResult{
				SpecID: specID,
				ACID:   acID,
				Status: actionToStatus(ev.Action),
				Name:   ev.Test,
			})
			delete(state, key)
		}
	}

	return results, scanned, dropped, nil
}

func actionToStatus(action string) Status {
	switch action {
	case "pass":
		return StatusPassed
	case "fail":
		return StatusFailed
	case "skip":
		return StatusSkipped
	default:
		return StatusErrored
	}
}

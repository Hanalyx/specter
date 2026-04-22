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
	// Per-test annotation context accumulated from output-action lines.
	type pending struct {
		specFromOutput string
		acFromOutput   string
	}
	state := make(map[string]*pending) // key: Package+"\x00"+Test

	var results []TestResult

	scanner := bufio.NewScanner(bytes.NewReader(data))
	// go test -json can emit lines longer than the default 64KB buffer for
	// tests that dump large diagnostics; bump generously.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev goTestEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			// Malformed line: tolerate (go test -json occasionally leaks
			// non-JSON noise under certain build errors).
			continue
		}
		if ev.Test == "" {
			// Package-level events (build results, coverage summary) ignored.
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
			specID, acID := "", ""
			// Prefer annotation in test name, fall back to output context.
			if m := testNameAnnotation.FindStringSubmatch(ev.Test); m != nil {
				specID = m[1]
				acID = m[2]
			} else if p := state[key]; p.specFromOutput != "" && p.acFromOutput != "" {
				specID = p.specFromOutput
				acID = p.acFromOutput
			}
			if specID == "" || acID == "" {
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

	return results, nil
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

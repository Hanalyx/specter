// results.go — .specter-results.json support for pass-rate-aware coverage.
//
// v1.3.0 shipped pass-rate-aware Tier 1 via a boolean `passed` field.
// v1.9.0 extends the schema with an explicit `status` enum (passed | failed |
// skipped | errored) so `specter coverage --strict` can demote non-passing
// annotated ACs across all tiers. The boolean is preserved for back-compat.
//
// @spec spec-coverage
package coverage

import (
	"encoding/json"
	"fmt"
)

// ResultEntry records the outcome of a single AC in a specific spec.
// Status (v1.9.0+) is the canonical field; Passed is derived for back-compat.
type ResultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Status string `json:"status,omitempty"`
	Passed bool   `json:"passed"`
}

// ResultsFile is the parsed .specter-results.json structure.
type ResultsFile struct {
	Results []ResultEntry `json:"results"`
}

// ParseResultsFile parses .specter-results.json content. Normalizes the
// back-compat boolean and the status enum into a consistent pair so callers
// can use either field.
//
// C-21: accepts entries with only `passed`, only `status`, or both.
//
// MaxResultsFileBytes caps the input size before json.Unmarshal to prevent
// memory exhaustion when a malicious CI runner / PR commits a multi-GB
// .specter-results.json into the workspace. The structure is flat (one
// entry per (spec_id, ac_id) pair); 16 MiB is generous for ~100k entries.
const MaxResultsFileBytes = 16 << 20 // 16 MiB

func ParseResultsFile(data []byte) (*ResultsFile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > MaxResultsFileBytes {
		return nil, fmt.Errorf(".specter-results.json exceeds %d byte limit (got %d bytes)", MaxResultsFileBytes, len(data))
	}
	var rf ResultsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, err
	}
	for i := range rf.Results {
		r := &rf.Results[i]
		switch {
		case r.Status != "":
			// Status-first: derive Passed from Status.
			r.Passed = r.Status == "passed"
		case r.Passed:
			r.Status = "passed"
		default:
			// Explicit passed:false, no status → mark as failed.
			r.Status = "failed"
		}
	}
	return &rf, nil
}

// passed returns true if the given spec+AC has a passing result entry, or if
// no entry exists (absent means "not recorded yet", not "failed"). Used by
// pre-1.9 pass-rate-aware Tier 1 coverage; preserved verbatim.
func (rf *ResultsFile) passed(specID, acID string) bool {
	if rf == nil {
		return true
	}
	for _, r := range rf.Results {
		if r.SpecID == specID && r.ACID == acID {
			return r.Passed
		}
	}
	return true
}

// status returns the canonical status for a (spec, AC), or "unknown" if no
// entry exists. Under --strict (BuildCoverageReportStrict with strict=true),
// "unknown" is treated as uncovered — the point of --strict is that every
// annotated AC must have a verified passing result.
//
// C-22 (AC-22).
func (rf *ResultsFile) status(specID, acID string) string {
	if rf == nil {
		return "unknown"
	}
	for _, r := range rf.Results {
		if r.SpecID == specID && r.ACID == acID {
			if r.Status == "" {
				if r.Passed {
					return "passed"
				}
				return "failed"
			}
			return r.Status
		}
	}
	return "unknown"
}

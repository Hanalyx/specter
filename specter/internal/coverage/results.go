// results.go — .specter-results.json support for pass-rate-aware coverage.
//
// @spec spec-coverage
package coverage

import "encoding/json"

// ResultEntry records pass/fail for a single AC in a specific spec.
type ResultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Passed bool   `json:"passed"`
}

// ResultsFile is the parsed .specter-results.json structure.
type ResultsFile struct {
	Results []ResultEntry `json:"results"`
}

// ParseResultsFile parses .specter-results.json content.
// Returns nil, nil if data is empty.
func ParseResultsFile(data []byte) (*ResultsFile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var rf ResultsFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, err
	}
	return &rf, nil
}

// passed returns true if the given spec+AC has a passing result entry.
// Returns false if the entry is absent (missing entry = not passed).
func (rf *ResultsFile) passed(specID, acID string) bool {
	if rf == nil {
		return true // no results file = no restriction
	}
	for _, r := range rf.Results {
		if r.SpecID == specID && r.ACID == acID {
			return r.Passed
		}
	}
	return false // absent = not passed
}

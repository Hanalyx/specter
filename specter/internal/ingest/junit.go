// junit.go — JUnit XML parser (Surefire schema), supports vitest, jest,
// pytest, playwright outputs.
//
// @spec spec-ingest
package ingest

import (
	"encoding/xml"
	"fmt"
)

// junitRoot wraps the optional <testsuites> top-level and a possibly-bare
// <testsuite>. Surefire-style XMLs sometimes omit the <testsuites> envelope.
type junitRoot struct {
	XMLName    xml.Name
	Suites     []junitSuite `xml:"testsuite"`
	Nested     []junitRoot  `xml:"testsuites"` // rarely nested, but seen in the wild
	Standalone junitSuite   `xml:",chardata"`  // ignored; placeholder to silence decoder
}

type junitSuite struct {
	Name      string    `xml:"name,attr"`
	TestCase  []junitTC `xml:"testcase"`
	Nested    []junitTS `xml:"testsuite"` // nested suites are valid JUnit
	SystemOut string    `xml:"system-out"`
}

type junitTS struct {
	Name     string    `xml:"name,attr"`
	TestCase []junitTC `xml:"testcase"`
}

type junitTC struct {
	Name      string       `xml:"name,attr"`
	Classname string       `xml:"classname,attr"`
	Failure   *junitResult `xml:"failure"`
	Errored   *junitResult `xml:"error"`
	Skipped   *junitResult `xml:"skipped"`
	SystemOut string       `xml:"system-out"`
}

type junitResult struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// ParseJUnit parses a JUnit XML document and returns TestResults for every
// testcase that carries a recognizable (spec, AC) annotation. C-01, C-03, C-04.
func ParseJUnit(data []byte) ([]TestResult, error) {
	results, _, _, err := ParseJUnitStats(data)
	return results, err
}

// ParseJUnitStats is like ParseJUnit but also returns the total number of
// testcases scanned and the names of testcases dropped for lacking a
// (spec_id, ac_id) annotation. Powers `Scanned N; extracted M; dropped K`
// summary (C-09) and --verbose per-case output (C-10).
func ParseJUnitStats(data []byte) (results []TestResult, scanned int, dropped []string, err error) {
	var root struct {
		XMLName xml.Name
		Suites  []junitSuite `xml:"testsuite"`
	}

	if xmlErr := xml.Unmarshal(data, &root); xmlErr == nil && len(root.Suites) > 0 {
		results, scanned, dropped = collectFromSuitesStats(root.Suites)
		return results, scanned, dropped, nil
	}

	var single junitSuite
	if xmlErr := xml.Unmarshal(data, &single); xmlErr != nil {
		return nil, 0, nil, fmt.Errorf("parse junit: %w", xmlErr)
	}
	if len(single.TestCase) > 0 {
		results, scanned, dropped = collectFromSuitesStats([]junitSuite{single})
		return results, scanned, dropped, nil
	}

	return nil, 0, nil, nil
}

func collectFromSuites(suites []junitSuite) []TestResult {
	r, _, _ := collectFromSuitesStats(suites)
	return r
}

func collectFromSuitesStats(suites []junitSuite) (results []TestResult, scanned int, dropped []string) {
	for _, s := range suites {
		for _, tc := range s.TestCase {
			scanned++
			if r, ok := testResultFromCase(tc); ok {
				results = append(results, r)
			} else {
				dropped = append(dropped, tc.Name)
			}
		}
		for _, ns := range s.Nested {
			for _, tc := range ns.TestCase {
				scanned++
				if r, ok := testResultFromCase(tc); ok {
					results = append(results, r)
				} else {
					dropped = append(dropped, tc.Name)
				}
			}
		}
	}
	return results, scanned, dropped
}

func testResultFromCase(tc junitTC) (TestResult, bool) {
	specID, acID := extractAnnotations(tc.Name, tc.Classname, tc.SystemOut)
	if specID == "" || acID == "" {
		return TestResult{}, false // C-04: silent drop
	}

	status := StatusPassed
	switch {
	case tc.Errored != nil:
		status = StatusErrored
	case tc.Failure != nil:
		status = StatusFailed
	case tc.Skipped != nil:
		status = StatusSkipped
	}

	return TestResult{
		SpecID: specID,
		ACID:   acID,
		Status: status,
		Name:   tc.Name,
	}, true
}

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
	var root struct {
		XMLName xml.Name
		Suites  []junitSuite `xml:"testsuite"`
	}

	// Try as <testsuites> root first, then fall back to bare <testsuite>.
	if err := xml.Unmarshal(data, &root); err == nil && len(root.Suites) > 0 {
		return collectFromSuites(root.Suites), nil
	}

	// Fallback: single <testsuite> at the root.
	var single junitSuite
	if err := xml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("parse junit: %w", err)
	}
	if len(single.TestCase) > 0 {
		return collectFromSuites([]junitSuite{single}), nil
	}

	return nil, nil
}

func collectFromSuites(suites []junitSuite) []TestResult {
	var results []TestResult
	for _, s := range suites {
		for _, tc := range s.TestCase {
			if r, ok := testResultFromCase(tc); ok {
				results = append(results, r)
			}
		}
		// Nested suites.
		for _, ns := range s.Nested {
			for _, tc := range ns.TestCase {
				if r, ok := testResultFromCase(tc); ok {
					results = append(results, r)
				}
			}
		}
	}
	return results
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

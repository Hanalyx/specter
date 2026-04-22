// annotations.go — shared (spec, AC) extraction for JUnit test names and
// go test outputs. C-03.
//
// @spec spec-ingest
package ingest

import "regexp"

// spec-id/AC-NN or spec-id:AC-NN, embedded anywhere in a test name.
// Spec IDs are kebab-case, ACs are the canonical AC-NN form.
var testNameAnnotation = regexp.MustCompile(`([a-z][a-z0-9-]*[a-z0-9])[/:](AC-\d+)`)

// // @spec <id>  and  // @ac <id>  anywhere in a text body.
var bodySpecAnnotation = regexp.MustCompile(`//\s*@spec\s+([a-z][a-z0-9-]*[a-z0-9])`)
var bodyACAnnotation = regexp.MustCompile(`//\s*@ac\s+(AC-\d+)`)

// extractAnnotations returns (specID, acID) discovered from any of the three
// sources. First hit wins: test-name pattern → classname pattern → body text.
// Returns ("", "") when no annotation is present (C-04: caller silent-drops).
func extractAnnotations(name, classname, body string) (string, string) {
	if m := testNameAnnotation.FindStringSubmatch(name); m != nil {
		return m[1], m[2]
	}
	if m := testNameAnnotation.FindStringSubmatch(classname); m != nil {
		return m[1], m[2]
	}
	var specID, acID string
	if m := bodySpecAnnotation.FindStringSubmatch(body); m != nil {
		specID = m[1]
	}
	if m := bodyACAnnotation.FindStringSubmatch(body); m != nil {
		acID = m[1]
	}
	return specID, acID
}

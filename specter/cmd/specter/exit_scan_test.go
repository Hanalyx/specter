package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The exit-code scan behind spec-sync AC-16 and AC-17, extracted so both
// criteria read the same source of truth. It lives in a _test.go file on
// purpose: nothing in the shipped binary calls it, and a helper that only tests
// use should not be compiled into the binary.

var (
	// exitCallRe finds every os.Exit call site, whatever its argument.
	exitCallRe = regexp.MustCompile(`os\.Exit\(`)
	// exitLiteralRe finds the sites whose argument is an integer literal.
	exitLiteralRe = regexp.MustCompile(`os\.Exit\((\d+)\)`)
)

// exitScan reports the distinct exit codes the sources in dir can emit.
//
// calls counts every os.Exit site in scope. sites counts how many of those the
// scan resolved to a code. spec-sync AC-17 requires the two to agree: a scan
// that drops a site it cannot read reports the same result as a scan with
// nothing to drop, so the counts are the only thing that tells them apart.
//
// Scope, stated because this is a static scan: dir's own *.go files, excluding
// _test.go. A code emitted from internal/ would not be found. That limit is
// AC-16's, and it is recorded there.
func exitScan(dir string) (codes map[string]bool, sites, calls int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, 0, err
	}
	codes = map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, 0, 0, err
		}
		text := string(src)
		calls += len(exitCallRe.FindAllString(text, -1))
		for _, m := range exitLiteralRe.FindAllStringSubmatch(text, -1) {
			codes[m[1]] = true
			sites++
		}
	}
	return codes, sites, calls, nil
}

// exitScanFiles counts the files the scan would read, so a test can refuse to
// pass on an empty scope.
func exitScanFiles(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			n++
		}
	}
	return n, nil
}

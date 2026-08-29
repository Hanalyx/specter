package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Shared helper for the tests that read this package's own source.
//
// Three of them do: the exit-code scan, the exclusion-ownership guard, and the
// strictness-ownership guard. Two had grown their own parser.ParseDir call,
// which staticcheck rejects since Go 1.25 because ParseDir ignores build tags
// when associating files with packages. One helper here, built the way
// parseExitScanDir already was, keeps the lint clean and stops a third copy
// appearing next time a guard needs the AST.

// parseProductionFiles parses every non-test .go file in dir.
//
// Production sources only, and that is a property every caller wants rather
// than an accident: a guard asserting "this pattern appears in one place"
// would otherwise report the tests that exercise the pattern as extra copies.
func parseProductionFiles(t *testing.T, fset *token.FileSet, dir string) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	out := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out[path] = f
	}

	// The positive control every caller would otherwise write. A guard over an
	// empty file set passes every absence claim it makes.
	if len(out) == 0 {
		t.Fatalf("parsed no production files in %s, so any claim over them would pass vacuously", dir)
	}
	return out
}

// funcSitesMatching reports every node in files satisfying want, as
// "funcName:line" strings.
//
// Sites rather than function names. A guard that collects function names into
// a set cannot tell one promotion loop from two inside the same function, and
// "exactly one place applies it" is a claim about places.
func funcSitesMatching(fset *token.FileSet, files map[string]*ast.File, want func(ast.Node) bool) []string {
	var sites []string
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fd, func(n ast.Node) bool {
				if n != nil && want(n) {
					sites = append(sites, fmt.Sprintf("%s:%d", fd.Name.Name, fset.Position(n.Pos()).Line))
				}
				return true
			})
		}
	}
	return sites
}

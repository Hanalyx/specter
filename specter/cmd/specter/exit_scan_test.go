package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The exit-code scan behind spec-sync AC-16 and AC-17, shared so both criteria
// read the same source of truth. It lives in a _test.go file on purpose:
// nothing in the shipped binary calls it, and a helper that only tests use
// should not be compiled into the binary.
//
// The scan reads the syntax tree rather than matching text. A regex over source
// also matches os.Exit( inside a comment or a string literal, and it cannot
// tell a call from a mention. go/ast is stdlib, parses without type checking,
// and gives a position for every site, which is what an error has to name.

// exitScan reports the distinct exit codes the sources in dir can emit.
//
// calls counts every os.Exit call site in scope. sites counts how many of those
// the scan resolved to a code. spec-sync AC-17 requires the two to agree, and
// the scan returns an error rather than dropping a site it cannot read: a scan
// that skips what it cannot resolve reports the same result as a scan with
// nothing to skip, so the counts are the only thing that tells them apart.
//
// Scope, stated because this is a static scan: dir's own *.go files, excluding
// _test.go. A code emitted from internal/ would not be found. That limit is
// AC-16's, and it is recorded there.
func exitScan(dir string) (codes map[string]bool, sites, calls int, err error) {
	fset := token.NewFileSet()
	files, err := parseExitScanDir(fset, dir)
	if err != nil {
		return nil, 0, 0, err
	}

	// Constants first, across every file in scope, because a constant declared
	// in one file is used from another.
	consts := map[string]string{}
	for _, f := range files {
		collectIntConsts(f, consts)
	}

	codes = map[string]bool{}
	for _, f := range files {
		var scanErr error
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isOsExit(call) || scanErr != nil {
				return scanErr == nil
			}
			calls++
			code, ok := resolveExitArg(call, consts)
			if !ok {
				scanErr = fmt.Errorf("%s: cannot resolve the argument of os.Exit to an integer. "+
					"AC-17 refuses rather than skips, because a skipped site is invisible in the result",
					fset.Position(call.Pos()))
				return false
			}
			codes[code] = true
			sites++
			return false
		})
		if scanErr != nil {
			return nil, 0, 0, scanErr
		}
	}
	return codes, sites, calls, nil
}

func parseExitScanDir(fset *token.FileSet, dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*ast.File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// collectIntConsts records every `const name = <int literal>` in the file. A
// constant whose value is an expression is deliberately not recorded, so a site
// using it fails the scan instead of resolving to a guess.
func collectIntConsts(f *ast.File, into map[string]string) {
	for _, d := range f.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.INT {
					into[name.Name] = lit.Value
				}
			}
		}
	}
}

func isOsExit(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Exit" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "os"
}

// resolveExitArg turns an os.Exit argument into the integer it emits. An
// integer literal resolves to itself and a bare identifier resolves through the
// constants in scope. Anything else does not resolve, and the caller refuses.
func resolveExitArg(call *ast.CallExpr, consts map[string]string) (string, bool) {
	if len(call.Args) != 1 {
		return "", false
	}
	switch a := call.Args[0].(type) {
	case *ast.BasicLit:
		if a.Kind != token.INT {
			return "", false
		}
		return normalizeIntLit(a.Value)
	case *ast.Ident:
		v, ok := consts[a.Name]
		if !ok {
			return "", false
		}
		return normalizeIntLit(v)
	}
	return "", false
}

// normalizeIntLit renders a Go integer literal in the decimal form the registry
// rows use, so 0x0A and 10 do not read as two different codes.
func normalizeIntLit(lit string) (string, bool) {
	n, err := strconv.ParseInt(strings.ReplaceAll(lit, "_", ""), 0, 64)
	if err != nil {
		return "", false
	}
	return strconv.FormatInt(n, 10), true
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

// registryHasRow reports whether docs/EXIT_CODES.md carries a registry row for
// code. A row begins the line with the code cell. A cell holding the same code
// in some other column of some other table is not a row, and treating it as one
// is how a registry check passes over a registry that is missing an entry.
func registryHasRow(registry, code string) bool {
	want := "| `" + code + "` |"
	for _, line := range strings.Split(registry, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), want) {
			return true
		}
	}
	return false
}

// The registry-parity converse, spec-sync AC-18. AC-16 claims that every code
// the document marks Stable is reachable from some os.Exit, and the rules that
// claim needs are AC-18's, not this file's. They are restated here only so the
// code reads without the spec open.

// exitCodeSuccess is exempt from the converse. Success is returned by RunE
// returning nil and never through os.Exit, so no scan of exit calls can reach
// it. AC-18(c) requires the exemption to be guarded rather than assumed, and the
// guard lives in the assertion: if anything ever calls os.Exit(0), the exemption
// is wrong and the test says so.
const exitCodeSuccess = "0"

const registryHeader = "| Code | Commands | Condition | Standing |"

// registryStableCodes reads Section 1 of docs/EXIT_CODES.md and reports which
// codes carry at least one Stable row, along with how many rows it parsed.
//
// A code is required-reachable when ANY of its rows is Stable, per AC-18(b).
// Standing belongs to a row and the converse is a claim about a code: code 2 has
// three rows, two Stable and one an accidental collision, and requiring every
// row to be Stable would exempt a code that is emitted and must be required.
//
// Stable is an exact match, per AC-18(a). The other values in use are
// "Shipped, overloaded, frozen" and "Accidental collision", and a substring
// match over either would be a different assertion than the document makes.
func registryStableCodes(registry string) (stable map[string]bool, rows int, err error) {
	lines := strings.Split(registry, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == registryHeader {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, 0, fmt.Errorf("no Section 1 registry table: expected a line reading %q", registryHeader)
	}

	stable = map[string]bool{}
	for _, l := range lines[start+1:] {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "|") {
			break
		}
		cells := registryCells(t)
		if len(cells) != 4 {
			continue
		}
		code := strings.Trim(cells[0], "`")
		if code == "" || strings.Trim(code, "-") == "" {
			continue // the header separator row
		}
		rows++
		if cells[3] == "Stable" {
			stable[code] = true
		}
	}
	return stable, rows, nil
}

func registryCells(row string) []string {
	parts := strings.Split(strings.Trim(row, "|"), "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// unreachableStableCodes reports every code the registry marks Stable that
// nothing in emitted can produce. Empty is the passing answer.
func unreachableStableCodes(registry string, emitted map[string]bool) ([]string, error) {
	stable, rows, err := registryStableCodes(registry)
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("parsed the registry table and found no rows; an empty answer here would be vacuous")
	}
	var out []string
	for code := range stable {
		if code == exitCodeSuccess {
			continue
		}
		if !emitted[code] {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out, nil
}

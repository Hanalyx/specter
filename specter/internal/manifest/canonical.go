// Content classification for pre-push-check, spec-manifest C-37.
//
// A changed implementation file counts as an implementation change unless
// its base and head are the same file after canonical formatting. Everything
// here is pure: the CLI reads both blobs from the pushed range and passes
// them in, so nothing below can reach the working tree or the index.
//
// @spec spec-manifest
package manifest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
)

// FileChange is one path in a pushed range with what the comparison needs.
// Status is git's --name-status letter: A, M, or D. Base and Head are the
// blob contents at the pushed base and the pushed head, read only when
// Status is M. ReadError carries the reason a blob could not be read. A
// non-empty value makes the file count, and the reason is reported, because
// a comparison that cannot be made is never read as "no change".
type FileChange struct {
	Path      string
	Status    byte
	Base      []byte
	Head      []byte
	ReadError string
}

// CanonicalGo returns the canonical form of a Go source file: the standard
// library printer over the parsed file, with every import declaration
// merged into one group sorted by path and then by name. Two files with
// the same canonical form differ only in what gofmt reflow and goimports
// grouping change. Comments are kept, so a comment edit is not erased, and
// blank lines the printer preserves are kept, so a hand-inserted blank line
// is not erased either. Both of those still count, as C-37 says.
func CanonicalGo(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var specs []ast.Spec
	var rest []ast.Decl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			specs = append(specs, g.Specs...)
			continue
		}
		rest = append(rest, d)
	}
	if len(specs) > 0 {
		sort.SliceStable(specs, func(i, j int) bool {
			a, b := specs[i].(*ast.ImportSpec), specs[j].(*ast.ImportSpec)
			if a.Path.Value != b.Path.Value {
				return a.Path.Value < b.Path.Value
			}
			return importName(a) < importName(b)
		})
		// Positions are what the printer uses to reproduce group gaps.
		// Dropping them is what makes grouping disappear.
		for _, s := range specs {
			is := s.(*ast.ImportSpec)
			is.Path.ValuePos = token.NoPos
			if is.Name != nil {
				is.Name.NamePos = token.NoPos
			}
			is.EndPos = token.NoPos
		}
		merged := &ast.GenDecl{Tok: token.IMPORT, Lparen: token.Pos(1), Specs: specs, Rparen: token.Pos(1)}
		rest = append([]ast.Decl{merged}, rest...)
	}
	f.Decls = rest
	f.Imports = nil

	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func importName(s *ast.ImportSpec) string {
	if s.Name == nil {
		return ""
	}
	return s.Name.Name
}

// formatOnly reports whether base and head are the same file after
// canonical formatting. It answers only for Go. Any other language has no
// canonical form in this tool, and the answer is false with no error: the
// file counts, and nothing was attempted that could fail. A parse failure
// on either side is an error: the file counts, and the reason is reported.
func formatOnly(path string, base, head []byte) (bool, error) {
	if !strings.HasSuffix(path, ".go") {
		return false, nil
	}
	cb, err := CanonicalGo(base)
	if err != nil {
		return false, fmt.Errorf("base does not parse: %w", err)
	}
	ch, err := CanonicalGo(head)
	if err != nil {
		return false, fmt.Errorf("head does not parse: %w", err)
	}
	return bytes.Equal(cb, ch), nil
}

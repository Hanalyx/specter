// Go test-file reachability detection for unreachable_annotation (C-10).
//
// Uses go/parser + go/ast to identify test functions and inspect their
// bodies. AST-based detection eliminates the false-positive classes
// inherent to regex/brace-count source scanning:
//
//   - Multi-line t.Run( calls — AST walks the whole CallExpr regardless of layout.
//   - String-concatenated subtest names — non-literal arg routes to _unknown
//     (we can't statically determine the string value), avoiding false positives.
//   - Helper wrappers like runSpec(t, "foo/AC-01", ...) — AST inspects all
//     call expressions, not just t.Run by name.
//   - Brace counter corruption from braces inside strings/comments —
//     irrelevant under AST parsing.
//
// Supported test-function shapes:
//   - func TestXxx(t *testing.T) { ... } — the standard pattern
//
// Known limitations (routed to _unknown rather than false-positive
// unreachable_annotation):
//   - Table-driven tests where t.Run(tc.name, ...) uses a struct-field
//     identifier as the subtest name. The literal is in a CompositeLit
//     elsewhere in the function body; static linking is non-trivial. v0.13
//     routes to _unknown; v0.13.1+ may extend the walker to inspect
//     CompositeLit elements when an ambiguous .Run is observed.
//   - String concatenations involving constants (SpecID + "/AC-01") —
//     requires go/types for constant folding. Routed to _unknown.
//   - testify pointer-receiver methods with zero params (func (s *Suite)
//     TestFoo()) — testify drives t via suite.T() rather than the param.
//     Not currently recognized as a test function. v0.13.1+ may add a
//     receiver-aware path.
//   - Go 1.18+ generic test functions — parsed but not runner-visible
//     to `go test` (the runner won't instantiate them), so reachability
//     analysis is moot.
//
// Algorithm:
//  1. Parse the file with go/parser. If parsing fails (incomplete file
//     or syntax error), fall back to emitting _unknown for every @ac.
//  2. Collect every @spec / @ac source comment with its line number
//     using the file's comment list from go/ast.
//  3. Collect every Go test function (regular or pointer-receiver
//     methods that satisfy the testing.T signature shape).
//  4. For each @ac, attach to its enclosing or next-following test
//     function by line position. If none, emit _unknown.
//  5. Walk the function body's AST looking for reachability tokens
//     (Convention A: any CallExpr's first string-literal arg containing
//     spec-id/AC-NN; Convention B: any print-family call containing
//     @spec/<id> or @ac/<id> as a string-literal arg).
//  6. Emit diagnostics per the strictness routing rules.
//
// @spec spec-check
package checker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// scanGoFile implements the C-10 contract for Go test files using
// the Go AST. See package doc above for the algorithm.
func scanGoFile(path, content string, strictness string) []CheckDiagnostic {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		// File can't be parsed — fall back to _unknown for every @ac
		// rather than producing a false-positive unreachable_annotation.
		// Use the language-neutral comment scanner since the AST is unusable.
		return emitUnknownForAllAcs(path, content)
	}

	// Pass 1: collect @spec / @ac annotations from comments, with
	// their absolute line numbers from the token.FileSet. Block-comment
	// markers (e.g., `/*\n@ac AC-01\n*/`) get the line of the marker,
	// not the line of `/*`.
	comments := collectAstAnnotations(fset, file)

	// Pass 2: collect every Go test function declaration with its
	// AST node (we need the body later) and its line span.
	funcs := collectAstGoTestFuncs(fset, file)

	// Pass 3: build a comment-to-FuncDecl association via ast.CommentMap.
	// This is what closes the "free-floating @ac above a non-test func"
	// false-positive class — CommentMap respects Go's comment-attachment
	// rules (a CommentGroup belongs to the immediately following decl,
	// not to a decl line-distance away).
	cmap := ast.NewCommentMap(fset, file, file.Comments)
	// Build reverse index: for each test FuncDecl, the set of comment
	// lines its CommentGroups own.
	commentOwner := make(map[int]*astGoTestFunc) // line → test func
	for node, cgs := range cmap {
		fn, ok := node.(*ast.FuncDecl)
		if !ok {
			continue
		}
		var tf *astGoTestFunc
		for i := range funcs {
			if funcs[i].decl == fn {
				tf = &funcs[i]
				break
			}
		}
		if tf == nil {
			// CommentGroup belongs to a non-test FuncDecl (e.g.,
			// helper(), TestMain, or a method with the wrong signature).
			// Mark each comment line as "owned by non-test" so we route
			// to _unknown instead of attributing to the next test.
			for _, cg := range cgs {
				for _, c := range cg.List {
					emitCommentLines(fset, c, func(line int, _ string) {
						commentOwner[line] = nil // explicit non-test owner
					})
				}
			}
			continue
		}
		for _, cg := range cgs {
			for _, c := range cg.List {
				emitCommentLines(fset, c, func(line int, _ string) {
					commentOwner[line] = tf
				})
			}
		}
	}

	// Pass 4: for each @ac, decide reachability.
	var diags []CheckDiagnostic
	for _, ac := range comments.acs {
		specID := comments.activeSpecAt(ac.line)

		// Resolve which test function the @ac belongs to.
		// Priority:
		//   1. CommentMap attributes the @ac to a test FuncDecl → use it.
		//   2. CommentMap attributes the @ac to a non-test decl
		//      (helper, etc.) → _unknown (avoid the false-positive
		//      "attached to next test" trap).
		//   3. The @ac line is INSIDE a test function's body (a comment
		//      inside the body isn't owned by the FuncDecl's CommentGroup;
		//      check line containment).
		//   4. Otherwise → _unknown.
		var fn *astGoTestFunc
		if owner, mapped := commentOwner[ac.line]; mapped {
			if owner == nil {
				// Owned by a non-test decl — route to _unknown.
				diags = append(diags, CheckDiagnostic{
					Kind:     "unreachable_annotation_unknown",
					Severity: "warning",
					Message:  formatUnknownMessage(path, ac.line, ac.id),
				})
				continue
			}
			fn = owner
		} else {
			// Not in CommentMap — check inside-body containment.
			fn = funcs.containing(ac.line)
		}
		if fn == nil {
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation_unknown",
				Severity: "warning",
				Message:  formatUnknownMessage(path, ac.line, ac.id),
			})
			continue
		}

		reach := walkFunctionForReachability(fn.decl, specID, ac.id)
		switch reach {
		case reachable:
			continue
		case ambiguous:
			// Body contains a .Run-shaped call whose first arg is not
			// a string literal (variable, concatenation, struct-field
			// reference). We can't statically determine reachability;
			// emit _unknown rather than a false-positive unreachable.
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation_unknown",
				Severity: "warning",
				Message:  formatUnknownMessage(path, ac.line, ac.id),
			})
		case unreachable:
			sev, suppress := routeSeverity(strictness)
			if suppress {
				continue
			}
			diags = append(diags, CheckDiagnostic{
				Kind:     "unreachable_annotation",
				Severity: sev,
				Message:  formatUnreachableMessage(path, ac.line, ac.id, specID),
			})
		}
	}

	return diags
}

// emitCommentLines walks the lines of an *ast.Comment, calling visit
// for each line with its absolute line number. Block comments
// (`/* ... */`) may span multiple lines; line-comments (`//`) are
// single-line. This is what gets the correct line for an `@ac`
// marker inside a multi-line `/* ... */` block.
func emitCommentLines(fset *token.FileSet, c *ast.Comment, visit func(line int, text string)) {
	start := fset.Position(c.Pos()).Line
	text := c.Text
	switch {
	case strings.HasPrefix(text, "//"):
		visit(start, strings.TrimPrefix(text, "//"))
	case strings.HasPrefix(text, "/*"):
		body := strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
		for i, line := range strings.Split(body, "\n") {
			visit(start+i, line)
		}
	}
}

// containing returns the test function whose body line range contains
// the given line (strictly inside the func declaration → end-of-body).
// Returns nil if no test function contains the line.
func (fs astGoTestFuncs) containing(line int) *astGoTestFunc {
	for i := range fs {
		if line >= fs[i].startLine && line <= fs[i].endLine {
			return &fs[i]
		}
	}
	return nil
}

// reachVerdict is the per-(@ac, function) reachability outcome.
type reachVerdict int

const (
	unreachable reachVerdict = iota
	reachable
	// ambiguous means the function contains a subtest-shaped call
	// whose first argument is not a bare string literal. We cannot
	// statically determine whether the runner-visible name will
	// carry spec-id/AC-NN, so we route to _unknown rather than risk
	// a false positive.
	ambiguous
)

// astAnnotations holds @spec and @ac source-comment positions
// collected from a parsed Go file.
type astAnnotations struct {
	specs []specComment
	acs   []acComment
}

// activeSpecAt returns the most-recent @spec id declared at or
// before the given line.
func (a *astAnnotations) activeSpecAt(line int) string {
	var current string
	for _, s := range a.specs {
		if s.line > line {
			break
		}
		current = s.id
	}
	return current
}

// collectAstAnnotations walks the file's CommentGroups and pulls
// out every @spec / @ac marker, paired with the token.FileSet-resolved
// line number. Multi-line block comments (/* ... */) get the line of
// the marker, not the line of the opening /*. Multi-line string
// concerns don't apply: comments are already distinct from string
// literals in the AST.
func collectAstAnnotations(fset *token.FileSet, file *ast.File) astAnnotations {
	var out astAnnotations
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			emitCommentLines(fset, c, func(line int, text string) {
				trimmed := strings.TrimSpace(text)
				if strings.HasPrefix(trimmed, "@spec ") {
					id := strings.TrimSpace(strings.TrimPrefix(trimmed, "@spec "))
					if id != "" {
						out.specs = append(out.specs, specComment{line: line, id: id})
					}
				} else if strings.HasPrefix(trimmed, "@ac ") {
					id := strings.TrimSpace(strings.TrimPrefix(trimmed, "@ac "))
					if id != "" {
						out.acs = append(out.acs, acComment{line: line, id: id})
					}
				}
			})
		}
	}
	return out
}

// astGoTestFunc captures the AST node and line span of one test
// function declaration.
type astGoTestFunc struct {
	name      string
	decl      *ast.FuncDecl
	startLine int
	endLine   int
}

// astGoTestFuncs is a typed slice with a helper to attach an
// @ac comment to a test function by line position.
type astGoTestFuncs []astGoTestFunc

func (fs astGoTestFuncs) enclosingOrNext(line int) *astGoTestFunc {
	for i := range fs {
		if line >= fs[i].startLine && line <= fs[i].endLine {
			return &fs[i]
		}
		if fs[i].startLine >= line {
			return &fs[i]
		}
	}
	return nil
}

// collectAstGoTestFuncs returns every Go test function in the file.
// Accepts both regular tests (func TestXxx(t *testing.T)) and
// pointer-receiver method tests (e.g., testify suites: func (s *Suite)
// TestXxx(t *testing.T)). Generic test functions (Go 1.18+) work
// natively because go/ast handles type-parameter lists.
//
// TestMain(*testing.M) is excluded — it's the package test entrypoint,
// not a runtime-visible test function from a coverage perspective.
func collectAstGoTestFuncs(fset *token.FileSet, file *ast.File) astGoTestFuncs {
	var out astGoTestFuncs
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fn.Name.Name

		// Test functions start with "Test" followed by a non-lowercase
		// (uppercase letter or digit per Go convention) OR end-of-name.
		if !strings.HasPrefix(name, "Test") {
			continue
		}
		// Exclude TestMain explicitly — its first parameter is *testing.M.
		if name == "TestMain" {
			continue
		}

		// The first parameter must be *testing.T for Go's `go test`
		// to consider this a test function.
		if fn.Type == nil || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
			continue
		}
		firstParam := fn.Type.Params.List[0]
		if !isTestingTParam(firstParam) {
			continue
		}

		startLine := fset.Position(fn.Pos()).Line
		endLine := fset.Position(fn.End()).Line
		out = append(out, astGoTestFunc{
			name:      name,
			decl:      fn,
			startLine: startLine,
			endLine:   endLine,
		})
	}
	return out
}

// isTestingTParam reports whether a function parameter has type
// *testing.T (with any identifier name). Recognizes the form
// `name *testing.T` plus `*testing.T` (bare type, no parameter name).
func isTestingTParam(field *ast.Field) bool {
	if field == nil {
		return false
	}
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "testing" && sel.Sel != nil && sel.Sel.Name == "T"
}

// walkFunctionForReachability walks the function body looking for
// reachability tokens. Returns:
//   - reachable: at least one call argument carries the spec-id/AC-NN
//     pair (Convention A subtest-name OR Convention B print emission)
//   - ambiguous: a "subtest-shaped" call's first arg is not a string
//     literal — we can't statically determine whether the runner will
//     see the pair; emit _unknown rather than risk a false positive
//   - unreachable: neither reachable nor ambiguous applies
//
// A "subtest-shaped" call is one whose function selector ends in
// `.Run` (e.g., t.Run, s.Run, suite.T().Run). We accept any selector
// because helper-wrapped subtest patterns are common.
func walkFunctionForReachability(decl *ast.FuncDecl, specID, acID string) reachVerdict {
	if decl == nil || decl.Body == nil {
		return unreachable
	}
	pair := acID
	if specID != "" {
		pair = specID + "/" + acID
	}

	var (
		foundReachable    bool
		nonLiteralSubtest bool
	)

	ast.Inspect(decl.Body, func(n ast.Node) bool {
		if foundReachable {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Convention A — subtest-shaped call (any selector ending
		// in .Run). String-literal first arg matches; non-literal
		// first arg is "ambiguous" (route to _unknown later).
		if isSubtestShaped(call) {
			if literal, isLit := stringLiteralArg(call, 0); isLit {
				if pair != "" && strings.Contains(literal, pair) {
					foundReachable = true
					return false
				}
			} else {
				nonLiteralSubtest = true
			}
		}

		// Helper-wrapped subtest pattern — any function call whose
		// first or second string-literal arg contains the pair
		// counts as reachable. Catches patterns like
		// runSpec(t, "foo-spec/AC-01", ...) common in the
		// dogfood-strict migration.
		if pair != "" {
			if literal, isLit := stringLiteralArg(call, 0); isLit && strings.Contains(literal, pair) {
				foundReachable = true
				return false
			}
			if literal, isLit := stringLiteralArg(call, 1); isLit && strings.Contains(literal, pair) {
				foundReachable = true
				return false
			}
		}

		// Convention B — print-family call with @spec/<id> or @ac/<id>
		// as any string-literal arg.
		if isPrintFamilyCall(call) {
			if acID != "" && callHasLiteralContaining(call, "@ac "+acID) {
				foundReachable = true
				return false
			}
			if specID != "" && callHasLiteralContaining(call, "@spec "+specID) {
				foundReachable = true
				return false
			}
		}

		return true
	})

	switch {
	case foundReachable:
		return reachable
	case nonLiteralSubtest:
		return ambiguous
	default:
		return unreachable
	}
}

// isSubtestShaped reports whether a call expression has the shape
// "<receiver>.Run(...)" — common for t.Run, s.Run, and helper-style
// "runner.Run". A bare Run(...) (no selector) does not match.
func isSubtestShaped(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel != nil && sel.Sel.Name == "Run"
}

// isPrintFamilyCall reports whether the call invokes a print/log
// function from the standard library, the test framework, or a
// common log package. We over-include rather than under-include —
// false negatives here cause false positives upstream (a real
// reachable test misclassified as unreachable).
func isPrintFamilyCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		// Could be a method on `t` named Log — handled via the .Sel
		// check below.
		if sel.Sel == nil {
			return false
		}
		switch sel.Sel.Name {
		case "Log", "Logf", "Logln":
			return true
		}
		return false
	}
	if sel.Sel == nil {
		return false
	}
	switch pkg.Name {
	case "fmt":
		switch sel.Sel.Name {
		case "Print", "Println", "Printf",
			"Fprint", "Fprintln", "Fprintf",
			"Sprint", "Sprintln", "Sprintf":
			return true
		}
	case "log":
		switch sel.Sel.Name {
		case "Print", "Println", "Printf":
			return true
		}
	case "t":
		switch sel.Sel.Name {
		case "Log", "Logf", "Logln", "Error", "Errorf", "Fatal", "Fatalf":
			return true
		}
	}
	// Also accept any selector ending in Log/Logf/Logln/Println/Printf
	// regardless of package — covers logging helpers and method
	// receivers other than `t` (e.g., `suite.T().Log(...)`).
	switch sel.Sel.Name {
	case "Println", "Printf", "Print",
		"Log", "Logf", "Logln",
		"Fprintln", "Fprintf", "Fprint":
		return true
	}
	return false
}

// stringLiteralArg returns the unquoted value of the call's
// argument at index `idx` if it is a bare string literal. Returns
// ("", false) for any non-string-literal expression (including
// string concatenations like `"foo-" + id`, function calls, or
// non-string types).
func stringLiteralArg(call *ast.CallExpr, idx int) (string, bool) {
	if idx >= len(call.Args) {
		return "", false
	}
	lit, ok := call.Args[idx].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	// strconv.Unquote handles both interpreted and raw strings.
	if len(lit.Value) < 2 {
		return "", false
	}
	// Hand-unquote to avoid the strconv import in this hot path.
	v := lit.Value
	if v[0] == '`' && v[len(v)-1] == '`' {
		return v[1 : len(v)-1], true
	}
	if v[0] == '"' && v[len(v)-1] == '"' {
		// We accept the raw inner text without unescaping — for
		// reachability purposes, `\/` and `/` are equivalent
		// matches against the pair string.
		return v[1 : len(v)-1], true
	}
	return "", false
}

// callHasLiteralContaining reports whether any string-literal
// argument of the call contains the given substring.
func callHasLiteralContaining(call *ast.CallExpr, sub string) bool {
	for i := range call.Args {
		if literal, ok := stringLiteralArg(call, i); ok {
			if strings.Contains(literal, sub) {
				return true
			}
		}
	}
	return false
}

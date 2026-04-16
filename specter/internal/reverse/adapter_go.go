package reverse

import (
	"regexp"
	"strings"
)

// GoAdapter extracts spec data from Go source code.
type GoAdapter struct{}

func (a *GoAdapter) Name() string { return "go" }

func (a *GoAdapter) Detect(path, content string) bool {
	return strings.HasSuffix(path, ".go")
}

func (a *GoAdapter) IsTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// --- Route Extraction ---

var (
	httpHandleFuncRE = regexp.MustCompile(`http\.HandleFunc\s*\(\s*"([^"]+)"\s*,\s*(\w+)`)
	ginRouteRE       = regexp.MustCompile(`(?:r|router|group|g|e)\.(GET|POST|PUT|DELETE|PATCH)\s*\(\s*"([^"]+)"`)
	chiRouteRE       = regexp.MustCompile(`r\.(Get|Post|Put|Delete|Patch)\s*\(\s*"([^"]+)"`)
	muxHandleRE      = regexp.MustCompile(`(?:r|router)\.HandleFunc\s*\(\s*"([^"]+)"\s*\)\.Methods\s*\(\s*"([^"]+)"`)
)

func (a *GoAdapter) ExtractRoutes(path, content string) []ExtractedRoute {
	var routes []ExtractedRoute
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1

		if m := httpHandleFuncRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: "ANY", Path: m[1], Handler: m[2], File: path, Line: lineNum,
			})
		}
		if m := ginRouteRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: strings.ToUpper(m[1]), Path: m[2], Handler: "", File: path, Line: lineNum,
			})
		}
		if m := chiRouteRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: strings.ToUpper(m[1]), Path: m[2], Handler: "", File: path, Line: lineNum,
			})
		}
		if m := muxHandleRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: strings.ToUpper(m[2]), Path: m[1], Handler: "", File: path, Line: lineNum,
			})
		}
	}

	return routes
}

// --- Constraint Extraction ---

var (
	validateTagRE = regexp.MustCompile("(?s)`[^`]*validate:\"([^\"]+)\"[^`]*`")
	jsonTagRE     = regexp.MustCompile("(?s)`[^`]*json:\"([^\"]+)\"[^`]*`")
	structFieldRE = regexp.MustCompile(`(\w+)\s+\S+\s+` + "`")
)

func (a *GoAdapter) ExtractConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1

		// Extract field name from struct field line
		fieldName := ""
		if m := structFieldRE.FindStringSubmatch(line); len(m) > 1 {
			fieldName = m[1]
		}
		// Try json tag for field name if struct field didn't match
		if fieldName == "" {
			if m := jsonTagRE.FindStringSubmatch(line); len(m) > 1 {
				parts := strings.Split(m[1], ",")
				if parts[0] != "" && parts[0] != "-" {
					fieldName = parts[0]
				}
			}
		}

		// Extract validate tag rules
		if m := validateTagRE.FindStringSubmatch(line); len(m) > 1 {
			rules := strings.Split(m[1], ",")
			for _, rule := range rules {
				rule = strings.TrimSpace(rule)
				if rule == "" {
					continue
				}
				c := ExtractedConstraint{
					Field:      fieldName,
					SourceFile: path,
					Line:       lineNum,
				}
				if parts := strings.SplitN(rule, "=", 2); len(parts) == 2 {
					c.Rule = parts[0]
					c.Value = parts[1]
				} else {
					c.Rule = rule
				}
				constraints = append(constraints, c)
			}
		}
	}

	return constraints
}

// --- Assertion Extraction ---

var (
	testFuncRE   = regexp.MustCompile(`func\s+(Test\w+)\s*\(\s*t\s+\*testing\.T\s*\)`)
	subTestRE    = regexp.MustCompile(`t\.Run\s*\(\s*"([^"]+)"`)
	tableDriveRE = regexp.MustCompile(`\{\s*name:\s*"([^"]+)"`)
)

func (a *GoAdapter) ExtractAssertions(path, content string) []ExtractedAssertion {
	var assertions []ExtractedAssertion
	lines := strings.Split(content, "\n")

	currentTestFunc := ""
	for i, line := range lines {
		lineNum := i + 1

		// Top-level test function
		if m := testFuncRE.FindStringSubmatch(line); len(m) > 1 {
			currentTestFunc = m[1]
		}

		// Sub-tests via t.Run
		if m := subTestRE.FindStringSubmatch(line); len(m) > 1 {
			desc := m[1]
			assertions = append(assertions, ExtractedAssertion{
				TestName:    currentTestFunc + "/" + desc,
				Description: desc,
				IsError:     isErrorDescription(desc),
				SourceFile:  path,
				Line:        lineNum,
			})
		}

		// Table-driven test entries
		if m := tableDriveRE.FindStringSubmatch(line); len(m) > 1 {
			desc := m[1]
			assertions = append(assertions, ExtractedAssertion{
				TestName:    currentTestFunc + "/" + desc,
				Description: desc,
				IsError:     isErrorDescription(desc),
				SourceFile:  path,
				Line:        lineNum,
			})
		}
	}

	// If no sub-tests or table entries were found, use top-level test functions
	if len(assertions) == 0 {
		for i, line := range lines {
			if m := testFuncRE.FindStringSubmatch(line); len(m) > 1 {
				name := m[1]
				desc := testNameToDescription(name)
				assertions = append(assertions, ExtractedAssertion{
					TestName:    name,
					Description: desc,
					IsError:     isErrorDescription(desc),
					SourceFile:  path,
					Line:        i + 1,
				})
			}
		}
	}

	return assertions
}

// --- Import Extraction ---

var importLineRE = regexp.MustCompile(`^\s*"([^"]+)"`)

func (a *GoAdapter) ExtractImports(path, content string) []ExtractedImport {
	var imports []ExtractedImport
	lines := strings.Split(content, "\n")
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock {
			if m := importLineRE.FindStringSubmatch(trimmed); len(m) > 1 {
				imports = append(imports, ExtractedImport{Module: m[1], File: path})
			}
		}

		// Single-line import
		if strings.HasPrefix(trimmed, "import \"") {
			start := strings.Index(trimmed, "\"") + 1
			end := strings.LastIndex(trimmed, "\"")
			if start > 0 && end > start {
				imports = append(imports, ExtractedImport{Module: trimmed[start:end], File: path})
			}
		}
	}

	return imports
}

// --- System Name Inference ---

var goModuleRE = regexp.MustCompile(`module\s+(\S+)`)

func (a *GoAdapter) InferSystemName(files []SourceFile) string {
	for _, f := range files {
		if strings.HasSuffix(f.Path, "go.mod") || strings.Contains(f.Path, "/go.mod") {
			if m := goModuleRE.FindStringSubmatch(f.Content); len(m) > 1 {
				parts := strings.Split(m[1], "/")
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

// --- Helpers ---

func isErrorDescription(desc string) bool {
	lower := strings.ToLower(desc)
	errorKeywords := []string{"error", "fail", "invalid", "reject", "deny", "denied", "unauthorized", "forbidden", "bad", "missing", "empty", "null", "nil"}
	for _, kw := range errorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func testNameToDescription(name string) string {
	// Strip "Test" prefix
	name = strings.TrimPrefix(name, "Test")
	// Insert spaces before capitals
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(name[i-1])
			if prev >= 'a' && prev <= 'z' {
				result.WriteRune(' ')
			}
		}
		result.WriteRune(r)
	}
	desc := strings.TrimSpace(result.String())
	// Convert underscores to spaces
	desc = strings.ReplaceAll(desc, "_", " ")
	return strings.ToLower(desc)
}

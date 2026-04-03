package reverse

import (
	"regexp"
	"strings"
)

// PythonAdapter extracts spec data from Python source code (FastAPI, Django, Flask).
type PythonAdapter struct{}

func (a *PythonAdapter) Name() string { return "python" }

func (a *PythonAdapter) Detect(path, content string) bool {
	return strings.HasSuffix(path, ".py")
}

func (a *PythonAdapter) IsTestFile(path string) bool {
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	if strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") {
		return true
	}
	if strings.HasSuffix(base, "_test.py") {
		return true
	}
	if strings.Contains(path, "/tests/") {
		return true
	}
	return false
}

// --- Route Extraction ---

var (
	fastapiRouteRE = regexp.MustCompile(`@(?:app|router)\.(get|post|put|delete|patch)\(\s*['"]([^'"]+)['"]`)
	djangoPathRE   = regexp.MustCompile(`path\(\s*['"]([^'"]+)['"]`)
	flaskRouteRE   = regexp.MustCompile(`@(?:app|bp)\.route\(\s*['"]([^'"]+)['"]`)
	flaskMethodsRE = regexp.MustCompile(`methods\s*=\s*\[([^\]]+)\]`)
)

func (a *PythonAdapter) ExtractRoutes(path, content string) []ExtractedRoute {
	var routes []ExtractedRoute
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1

		// FastAPI: @app.get("/path") or @router.post("/path")
		if m := fastapiRouteRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: strings.ToUpper(m[1]), Path: m[2], Handler: "", File: path, Line: lineNum,
			})
		}

		// Django: path("route", ...)
		if m := djangoPathRE.FindStringSubmatch(line); len(m) > 1 {
			routes = append(routes, ExtractedRoute{
				Method: "ANY", Path: m[1], Handler: "", File: path, Line: lineNum,
			})
		}

		// Flask: @app.route("/path", methods=["GET", "POST"])
		if m := flaskRouteRE.FindStringSubmatch(line); len(m) > 1 {
			method := "ANY"
			if mm := flaskMethodsRE.FindStringSubmatch(line); len(mm) > 1 {
				// Parse first method from the list
				methods := strings.Split(mm[1], ",")
				if len(methods) > 0 {
					method = strings.ToUpper(strings.Trim(strings.TrimSpace(methods[0]), "\"'"))
				}
			}
			routes = append(routes, ExtractedRoute{
				Method: method, Path: m[1], Handler: "", File: path, Line: lineNum,
			})
		}
	}

	return routes
}

// --- Constraint Extraction ---

var (
	pydanticFieldRE    = regexp.MustCompile(`(\w+)\s*:\s*\w+\s*=\s*Field\(([^)]+)\)`)
	fieldMinLengthRE   = regexp.MustCompile(`min_length\s*=\s*(\d+)`)
	fieldMaxLengthRE   = regexp.MustCompile(`max_length\s*=\s*(\d+)`)
	fieldGeRE          = regexp.MustCompile(`ge\s*=\s*(\d+)`)
	fieldLeRE          = regexp.MustCompile(`le\s*=\s*(\d+)`)
	fieldRegexRE       = regexp.MustCompile(`regex\s*=\s*['"]([^'"]+)['"]`)
	requiredFieldRE    = regexp.MustCompile(`(\w+)\s*:\s*(\w+)\s*$`)
	sqlaNullableRE     = regexp.MustCompile(`(\w+)\s*=\s*Column\(.*nullable\s*=\s*False`)
)

func (a *PythonAdapter) ExtractConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip comment-only lines and common directives
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") ||
			strings.Contains(trimmed, "# noqa") || strings.Contains(trimmed, "# isort") ||
			strings.Contains(trimmed, "# type:") || strings.Contains(trimmed, "# pragma") ||
			strings.Contains(trimmed, "# fmt:") || strings.Contains(trimmed, "# pylint") ||
			strings.Contains(trimmed, "# noinspection") {
			continue
		}

		// Pydantic Field() with validators
		if m := pydanticFieldRE.FindStringSubmatch(trimmed); len(m) > 2 {
			fieldName := m[1]
			args := m[2]

			if mm := fieldMinLengthRE.FindStringSubmatch(args); len(mm) > 1 {
				constraints = append(constraints, ExtractedConstraint{
					Field: fieldName, Rule: "min_length", Value: mm[1],
					SourceFile: path, Line: lineNum,
				})
			}
			if mm := fieldMaxLengthRE.FindStringSubmatch(args); len(mm) > 1 {
				constraints = append(constraints, ExtractedConstraint{
					Field: fieldName, Rule: "max_length", Value: mm[1],
					SourceFile: path, Line: lineNum,
				})
			}
			if mm := fieldGeRE.FindStringSubmatch(args); len(mm) > 1 {
				constraints = append(constraints, ExtractedConstraint{
					Field: fieldName, Rule: "ge", Value: mm[1],
					SourceFile: path, Line: lineNum,
				})
			}
			if mm := fieldLeRE.FindStringSubmatch(args); len(mm) > 1 {
				constraints = append(constraints, ExtractedConstraint{
					Field: fieldName, Rule: "le", Value: mm[1],
					SourceFile: path, Line: lineNum,
				})
			}
			if mm := fieldRegexRE.FindStringSubmatch(args); len(mm) > 1 {
				constraints = append(constraints, ExtractedConstraint{
					Field: fieldName, Rule: "pattern", Value: mm[1],
					SourceFile: path, Line: lineNum,
				})
			}
			continue
		}

		// Required field: type annotation without Optional or default value
		if m := requiredFieldRE.FindStringSubmatch(trimmed); len(m) > 2 {
			typeName := m[2]
			if typeName != "Optional" && !strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "def ") && !strings.HasPrefix(trimmed, "class ") {
				constraints = append(constraints, ExtractedConstraint{
					Field: m[1], Rule: "required",
					SourceFile: path, Line: lineNum,
				})
			}
		}

		// SQLAlchemy Column(... nullable=False)
		if m := sqlaNullableRE.FindStringSubmatch(trimmed); len(m) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: m[1], Rule: "required",
				SourceFile: path, Line: lineNum,
			})
		}
	}

	return constraints
}

// --- Assertion Extraction ---

var (
	pytestFuncRE       = regexp.MustCompile(`def\s+test_(\w+)`)
	pytestRaisesRE     = regexp.MustCompile(`pytest\.raises\((\w+)\)`)
	assertStatusCodeRE = regexp.MustCompile(`assert\s+response\.status_code\s*==\s*(\d+)`)
	pytestClassRE      = regexp.MustCompile(`class\s+Test(\w+)\s*:`)
)

func (a *PythonAdapter) ExtractAssertions(path, content string) []ExtractedAssertion {
	var assertions []ExtractedAssertion
	lines := strings.Split(content, "\n")

	currentClass := ""
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Test class
		if m := pytestClassRE.FindStringSubmatch(trimmed); len(m) > 1 {
			currentClass = m[1]
		}

		// Test function
		if m := pytestFuncRE.FindStringSubmatch(trimmed); len(m) > 1 {
			funcName := m[1]
			desc := strings.ReplaceAll(funcName, "_", " ")
			testName := "test_" + funcName
			if currentClass != "" {
				testName = "Test" + currentClass + "::" + testName
			}

			isError := isErrorDescription(desc)

			// Look ahead for pytest.raises and status code assertions
			errorDesc := ""
			for j := i + 1; j < len(lines) && j < i+20; j++ {
				aheadLine := strings.TrimSpace(lines[j])
				// Stop at next function or class definition
				if strings.HasPrefix(aheadLine, "def ") || strings.HasPrefix(aheadLine, "class ") {
					break
				}
				if rm := pytestRaisesRE.FindStringSubmatch(aheadLine); len(rm) > 1 {
					isError = true
					errorDesc = rm[1]
				}
				if sm := assertStatusCodeRE.FindStringSubmatch(aheadLine); len(sm) > 1 {
					code := sm[1]
					if code >= "400" {
						isError = true
					}
				}
			}

			assertions = append(assertions, ExtractedAssertion{
				TestName:    testName,
				Description: desc,
				IsError:     isError,
				ErrorDesc:   errorDesc,
				SourceFile:  path,
				Line:        lineNum,
			})
		}
	}

	return assertions
}

// --- Import Extraction ---

var (
	pyFromImportRE = regexp.MustCompile(`^from\s+(\S+)\s+import`)
	pyImportRE     = regexp.MustCompile(`^import\s+(\S+)`)
)

func (a *PythonAdapter) ExtractImports(path, content string) []ExtractedImport {
	var imports []ExtractedImport
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// from X import ... (takes priority, check first)
		if m := pyFromImportRE.FindStringSubmatch(trimmed); len(m) > 1 {
			imports = append(imports, ExtractedImport{Module: m[1], File: path})
			continue
		}

		// import X (only if not a "from ... import")
		if m := pyImportRE.FindStringSubmatch(trimmed); len(m) > 1 {
			imports = append(imports, ExtractedImport{Module: m[1], File: path})
		}
	}

	return imports
}

// --- System Name Inference ---

var pyProjectNameRE = regexp.MustCompile(`name\s*=\s*"([^"]+)"`)

func (a *PythonAdapter) InferSystemName(files []SourceFile) string {
	for _, f := range files {
		if strings.HasSuffix(f.Path, "pyproject.toml") || strings.Contains(f.Path, "/pyproject.toml") {
			if m := pyProjectNameRE.FindStringSubmatch(f.Content); len(m) > 1 {
				return m[1]
			}
		}
	}
	return ""
}

package reverse

import (
	"regexp"
	"strings"
)

// TypeScriptAdapter extracts spec data from TypeScript/Next.js/Express source code.
type TypeScriptAdapter struct{}

func (a *TypeScriptAdapter) Name() string { return "typescript" }

func (a *TypeScriptAdapter) Detect(path, content string) bool {
	return strings.HasSuffix(path, ".ts") ||
		strings.HasSuffix(path, ".tsx") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".jsx")
}

func (a *TypeScriptAdapter) IsTestFile(path string) bool {
	if strings.Contains(path, "__tests__/") {
		return true
	}
	for _, suffix := range []string{".test.ts", ".test.tsx", ".spec.ts", ".test.js", ".test.jsx"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// --- Route Extraction ---

var (
	// Next.js App Router: export async function GET|POST|PUT|DELETE|PATCH
	nextjsRouteRE = regexp.MustCompile(`export\s+async\s+function\s+(GET|POST|PUT|DELETE|PATCH)`)
	// Express: app.get("/path", ...) or router.post("/path", ...)
	expressRouteRE = regexp.MustCompile(`(?:app|router)\.(get|post|put|delete|patch)\(\s*['"]([^'"]+)['"]`)
)

func (a *TypeScriptAdapter) ExtractRoutes(path, content string) []ExtractedRoute {
	var routes []ExtractedRoute
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1

		// Next.js App Router
		if m := nextjsRouteRE.FindStringSubmatch(line); len(m) > 1 {
			method := strings.ToUpper(m[1])
			apiPath := inferNextJSRoutePath(path)
			routes = append(routes, ExtractedRoute{
				Method: method, Path: apiPath, Handler: m[1], File: path, Line: lineNum,
			})
		}

		// Express routes
		if m := expressRouteRE.FindStringSubmatch(line); len(m) > 2 {
			routes = append(routes, ExtractedRoute{
				Method: strings.ToUpper(m[1]), Path: m[2], Handler: "", File: path, Line: lineNum,
			})
		}
	}

	return routes
}

// inferNextJSRoutePath converts a file path containing app/api/ to an API route path.
// e.g. "src/app/api/users/route.ts" -> "/api/users"
func inferNextJSRoutePath(filePath string) string {
	idx := strings.Index(filePath, "app/api/")
	if idx < 0 {
		return "/unknown"
	}
	// Strip everything before "app/" and remove the filename
	routePart := filePath[idx+len("app"):]
	// Remove trailing /route.ts or /route.js etc.
	if lastSlash := strings.LastIndex(routePart, "/"); lastSlash >= 0 {
		routePart = routePart[:lastSlash]
	}
	if routePart == "" {
		routePart = "/"
	}
	return routePart
}

// --- Constraint Extraction (Zod) ---

var (
	zodFieldRE     = regexp.MustCompile(`(\w+)\s*:\s*z\.`)
	zodStringMinRE = regexp.MustCompile(`z\.string\(\).*\.min\((\d+)\)`)
	zodStringMaxRE = regexp.MustCompile(`z\.string\(\).*\.max\((\d+)\)`)
	zodEmailRE     = regexp.MustCompile(`z\.string\(\).*\.email\(\)`)
	zodNumberMinRE = regexp.MustCompile(`z\.number\(\).*\.min\((\d+)\)`)
	zodNumberMaxRE = regexp.MustCompile(`z\.number\(\).*\.max\((\d+)\)`)
	zodEnumRE      = regexp.MustCompile(`z\.enum\(`)
	zodOptionalRE  = regexp.MustCompile(`\.optional\(\)`)
)

func (a *TypeScriptAdapter) ExtractConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Must be a zod field line
		fieldMatch := zodFieldRE.FindStringSubmatch(trimmed)
		if len(fieldMatch) < 2 {
			continue
		}
		fieldName := fieldMatch[1]

		// Check if field is required (no .optional())
		if !zodOptionalRE.MatchString(trimmed) {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "required", SourceFile: path, Line: lineNum,
			})
		}

		// z.string().min(N)
		if m := zodStringMinRE.FindStringSubmatch(trimmed); len(m) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "min", Value: m[1], SourceFile: path, Line: lineNum,
			})
		}

		// z.string().max(N)
		if m := zodStringMaxRE.FindStringSubmatch(trimmed); len(m) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "max", Value: m[1], SourceFile: path, Line: lineNum,
			})
		}

		// z.string().email()
		if zodEmailRE.MatchString(trimmed) {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "format", Value: "email", SourceFile: path, Line: lineNum,
			})
		}

		// z.number().min(N)
		if m := zodNumberMinRE.FindStringSubmatch(trimmed); len(m) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "min", Value: m[1], SourceFile: path, Line: lineNum,
			})
		}

		// z.number().max(N)
		if m := zodNumberMaxRE.FindStringSubmatch(trimmed); len(m) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "max", Value: m[1], SourceFile: path, Line: lineNum,
			})
		}

		// z.enum([...])
		if zodEnumRE.MatchString(trimmed) {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "enum", SourceFile: path, Line: lineNum,
			})
		}
	}

	return constraints
}

// --- Assertion Extraction (Jest/Vitest) ---

var (
	describeBlockRE = regexp.MustCompile(`describe\(\s*['"]([^'"]+)['"]`)
	itTestRE        = regexp.MustCompile(`(?:it|test)\(\s*['"]([^'"]+)['"]`)
)

func (a *TypeScriptAdapter) ExtractAssertions(path, content string) []ExtractedAssertion {
	var assertions []ExtractedAssertion
	lines := strings.Split(content, "\n")

	currentDescribe := ""
	for i, line := range lines {
		lineNum := i + 1

		// Track describe block context
		if m := describeBlockRE.FindStringSubmatch(line); len(m) > 1 {
			currentDescribe = m[1]
		}

		// Extract it/test assertions
		if m := itTestRE.FindStringSubmatch(line); len(m) > 1 {
			desc := m[1]
			testName := desc
			if currentDescribe != "" {
				testName = currentDescribe + " > " + desc
			}
			assertions = append(assertions, ExtractedAssertion{
				TestName:    testName,
				Description: desc,
				IsError:     isErrorDescription(desc),
				SourceFile:  path,
				Line:        lineNum,
			})
		}
	}

	return assertions
}

// --- Import Extraction ---

var tsImportRE = regexp.MustCompile(`import\s+.*\s+from\s+['"]([^'"]+)['"]`)

func (a *TypeScriptAdapter) ExtractImports(path, content string) []ExtractedImport {
	var imports []ExtractedImport
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := tsImportRE.FindStringSubmatch(trimmed); len(m) > 1 {
			imports = append(imports, ExtractedImport{Module: m[1], File: path})
		}
	}

	return imports
}

// --- System Name Inference ---

var packageNameRE = regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)

func (a *TypeScriptAdapter) InferSystemName(files []SourceFile) string {
	for _, f := range files {
		if strings.HasSuffix(f.Path, "package.json") || strings.Contains(f.Path, "/package.json") {
			if m := packageNameRE.FindStringSubmatch(f.Content); len(m) > 1 {
				return m[1]
			}
		}
	}
	return ""
}

package reverse

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// TypeScriptAdapter extracts spec data from TypeScript/Next.js/Express source code.
type TypeScriptAdapter struct{}

func (a *TypeScriptAdapter) Name() string { return "typescript" }

func (a *TypeScriptAdapter) Detect(path, content string) bool {
	return strings.HasSuffix(path, ".ts") ||
		strings.HasSuffix(path, ".tsx") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".jsx") ||
		strings.HasSuffix(path, ".prisma")
}

func (a *TypeScriptAdapter) IsTestFile(path string) bool {
	if strings.Contains(path, "__tests__/") {
		return true
	}
	for _, suffix := range []string{".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx", ".test.js", ".test.jsx", ".spec.js", ".spec.jsx"} {
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

// --- Constraint Extraction (Zod + TypeScript patterns + Prisma) ---

var (
	zodFieldRE     = regexp.MustCompile(`(\w+)\s*:\s*z\.`)
	zodStringMinRE = regexp.MustCompile(`z\.string\(\).*\.min\((\d+)`)
	zodStringMaxRE = regexp.MustCompile(`z\.string\(\).*\.max\((\d+)`)
	zodEmailRE     = regexp.MustCompile(`z\.string\(\).*\.email\(`)
	zodUrlRE       = regexp.MustCompile(`z\.string\(\).*\.url\(`)
	zodNumberMinRE = regexp.MustCompile(`z\.number\(\).*\.min\((\d+)`)
	zodNumberMaxRE = regexp.MustCompile(`z\.number\(\).*\.max\((\d+)`)
	zodEnumRE      = regexp.MustCompile(`z\.enum\(\[([^\]]*)\]`)
	zodOptionalRE  = regexp.MustCompile(`\.optional\(\)`)
	zodBooleanRE   = regexp.MustCompile(`z\.boolean\(\)`)
	zodArrayRE     = regexp.MustCompile(`z\.array\(`)
	// TypeScript enums: enum Role { ADMIN = "ADMIN", USER = "USER" }
	tsEnumRE = regexp.MustCompile(`enum\s+(\w+)\s*\{([^}]+)\}`)
	// TypeScript union types: type Status = "active" | "inactive" | "pending"
	tsUnionTypeRE = regexp.MustCompile(`type\s+(\w+)\s*=\s*((?:['"][^'"]+['"]\s*\|\s*)*['"][^'"]+['"])`)
	// TypeScript const assertions: const ROLES = ["admin", "user"] as const
	tsConstArrayRE = regexp.MustCompile(`const\s+(\w+)\s*=\s*\[([^\]]+)\]\s*as\s+const`)

	// Prisma model fields: name String @unique @db.VarChar(255)
	prismaFieldRE   = regexp.MustCompile(`^\s+(\w+)\s+(String|Int|Float|Boolean|DateTime|Json|BigInt|Decimal|Bytes)(\?)?(.*)$`)
	prismaUniqueRE  = regexp.MustCompile(`@unique`)
	prismaVarCharRE = regexp.MustCompile(`@db\.VarChar\((\d+)\)`)
	prismaRelRE     = regexp.MustCompile(`@relation`)

	// Role/auth checks: if (role !== "ADMIN") or session?.user?.role === "ADMIN"
	roleCheckRE = regexp.MustCompile(`(?:role|user\.role|session\.user\.role)\s*(?:===?|!==?)\s*['"](\w+)['"]`)
	// Status checks: if (status === "active") or status !== "deleted"
	statusCheckRE = regexp.MustCompile(`(?:\.status|status)\s*(?:===?|!==?)\s*['"](\w+)['"]`)
)

func (a *TypeScriptAdapter) ExtractConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// --- Zod schema fields ---
		if fieldMatch := zodFieldRE.FindStringSubmatch(trimmed); len(fieldMatch) >= 2 {
			fieldName := fieldMatch[1]
			constraints = append(constraints, a.extractZodConstraints(fieldName, trimmed, path, lineNum)...)
		}

		// --- TypeScript enums ---
		if m := tsEnumRE.FindStringSubmatch(trimmed); len(m) > 2 {
			enumName := m[1]
			constraints = append(constraints, ExtractedConstraint{
				Field:       strings.ToLower(enumName[:1]) + enumName[1:],
				Rule:        "enum",
				Value:       strings.TrimSpace(m[2]),
				Description: fmt.Sprintf("%s MUST be one of the enum values defined in %s", enumName, enumName),
				SourceFile:  path,
				Line:        lineNum,
			})
		}

		// --- TypeScript union types ---
		if m := tsUnionTypeRE.FindStringSubmatch(trimmed); len(m) > 2 {
			typeName := m[1]
			values := m[2]
			constraints = append(constraints, ExtractedConstraint{
				Field:       strings.ToLower(typeName[:1]) + typeName[1:],
				Rule:        "enum",
				Value:       values,
				Description: fmt.Sprintf("%s MUST be one of: %s", typeName, values),
				SourceFile:  path,
				Line:        lineNum,
			})
		}

		// --- TypeScript const arrays (as const) ---
		if m := tsConstArrayRE.FindStringSubmatch(trimmed); len(m) > 2 {
			constName := m[1]
			constraints = append(constraints, ExtractedConstraint{
				Field:       strings.ToLower(constName[:1]) + constName[1:],
				Rule:        "enum",
				Value:       strings.TrimSpace(m[2]),
				Description: fmt.Sprintf("%s MUST be one of the values in %s", constName, constName),
				SourceFile:  path,
				Line:        lineNum,
			})
		}
	}

	// --- Prisma schema extraction ---
	if strings.HasSuffix(path, ".prisma") {
		constraints = append(constraints, a.extractPrismaConstraints(path, content)...)
	}

	// --- Role/auth/status pattern extraction ---
	constraints = append(constraints, a.extractPatternConstraints(path, content)...)

	return constraints
}

func (a *TypeScriptAdapter) extractZodConstraints(fieldName, line, path string, lineNum int) []ExtractedConstraint {
	var constraints []ExtractedConstraint

	// Required (no .optional())
	if !zodOptionalRE.MatchString(line) {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "required", SourceFile: path, Line: lineNum,
		})
	}

	// z.string().min(N)
	if m := zodStringMinRE.FindStringSubmatch(line); len(m) > 1 {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "min", Value: m[1], SourceFile: path, Line: lineNum,
		})
	}

	// z.string().max(N)
	if m := zodStringMaxRE.FindStringSubmatch(line); len(m) > 1 {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "max", Value: m[1], SourceFile: path, Line: lineNum,
		})
	}

	// z.string().email()
	if zodEmailRE.MatchString(line) {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "format", Value: "email", SourceFile: path, Line: lineNum,
		})
	}

	// z.string().url()
	if zodUrlRE.MatchString(line) {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "format", Value: "url", SourceFile: path, Line: lineNum,
		})
	}

	// z.number().min(N)
	if m := zodNumberMinRE.FindStringSubmatch(line); len(m) > 1 {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "min", Value: m[1], SourceFile: path, Line: lineNum,
		})
	}

	// z.number().max(N)
	if m := zodNumberMaxRE.FindStringSubmatch(line); len(m) > 1 {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "max", Value: m[1], SourceFile: path, Line: lineNum,
		})
	}

	// z.enum([...]) — extract values
	if m := zodEnumRE.FindStringSubmatch(line); len(m) > 1 {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "enum", Value: strings.TrimSpace(m[1]), SourceFile: path, Line: lineNum,
		})
	}

	// z.boolean()
	if zodBooleanRE.MatchString(line) && !zodArrayRE.MatchString(line) {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "type", Value: "boolean", SourceFile: path, Line: lineNum,
		})
	}

	// z.array(...)
	if zodArrayRE.MatchString(line) {
		constraints = append(constraints, ExtractedConstraint{
			Field: fieldName, Rule: "type", Value: "array", SourceFile: path, Line: lineNum,
		})
	}

	return constraints
}

func (a *TypeScriptAdapter) extractPrismaConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		m := prismaFieldRE.FindStringSubmatch(line)
		if len(m) < 5 {
			continue
		}

		fieldName := m[1]
		fieldType := m[2]
		isOptional := m[3] == "?"
		attrs := m[4]

		// Skip relation fields
		if prismaRelRE.MatchString(attrs) {
			continue
		}

		// Type constraint
		constraints = append(constraints, ExtractedConstraint{
			Field:       fieldName,
			Rule:        "type",
			Value:       fieldType,
			Description: fmt.Sprintf("%s MUST be of type %s", fieldName, fieldType),
			SourceFile:  path,
			Line:        lineNum,
		})

		// Required (not optional)
		if !isOptional {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "required", SourceFile: path, Line: lineNum,
			})
		}

		// @unique
		if prismaUniqueRE.MatchString(attrs) {
			constraints = append(constraints, ExtractedConstraint{
				Field:       fieldName,
				Rule:        "unique",
				Value:       "true",
				Description: fmt.Sprintf("%s MUST be unique", fieldName),
				SourceFile:  path,
				Line:        lineNum,
			})
		}

		// @db.VarChar(N)
		if vm := prismaVarCharRE.FindStringSubmatch(attrs); len(vm) > 1 {
			constraints = append(constraints, ExtractedConstraint{
				Field: fieldName, Rule: "max", Value: vm[1], SourceFile: path, Line: lineNum,
			})
		}
	}

	return constraints
}

func (a *TypeScriptAdapter) extractPatternConstraints(path, content string) []ExtractedConstraint {
	var constraints []ExtractedConstraint
	lines := strings.Split(content, "\n")

	// Collect unique role and status values
	roles := make(map[string]int) // value -> first line
	statuses := make(map[string]int)

	for i, line := range lines {
		lineNum := i + 1

		for _, m := range roleCheckRE.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				if _, exists := roles[m[1]]; !exists {
					roles[m[1]] = lineNum
				}
			}
		}

		for _, m := range statusCheckRE.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				if _, exists := statuses[m[1]]; !exists {
					statuses[m[1]] = lineNum
				}
			}
		}
	}

	// Emit role constraint if roles found
	if len(roles) > 0 {
		var roleValues []string
		firstLine := 0
		for v, line := range roles {
			roleValues = append(roleValues, v)
			if firstLine == 0 || line < firstLine {
				firstLine = line
			}
		}
		sort.Strings(roleValues)
		constraints = append(constraints, ExtractedConstraint{
			Field:       "role",
			Rule:        "enum",
			Value:       strings.Join(roleValues, ", "),
			Description: fmt.Sprintf("role MUST be one of: %s", strings.Join(roleValues, ", ")),
			SourceFile:  path,
			Line:        firstLine,
		})
	}

	// Emit status constraint if statuses found
	if len(statuses) > 0 {
		var statusValues []string
		firstLine := 0
		for v, line := range statuses {
			statusValues = append(statusValues, v)
			if firstLine == 0 || line < firstLine {
				firstLine = line
			}
		}
		sort.Strings(statusValues)
		constraints = append(constraints, ExtractedConstraint{
			Field:       "status",
			Rule:        "enum",
			Value:       strings.Join(statusValues, ", "),
			Description: fmt.Sprintf("status MUST be one of: %s", strings.Join(statusValues, ", ")),
			SourceFile:  path,
			Line:        firstLine,
		})
	}

	return constraints
}

// --- Assertion Extraction (Jest/Vitest) ---

var (
	// C-11: patterns handle escaped quotes via (?:[^"\\]|\\.)*
	describeBlockDQ = regexp.MustCompile(`describe\(\s*"((?:[^"\\]|\\.)*)"`)
	describeBlockSQ = regexp.MustCompile(`describe\(\s*'((?:[^'\\]|\\.)*)'`)
	itTestDQ        = regexp.MustCompile(`(?:it|test)\(\s*"((?:[^"\\]|\\.)*)"`)
	itTestSQ        = regexp.MustCompile(`(?:it|test)\(\s*'((?:[^'\\]|\\.)*)'`)
)

func (a *TypeScriptAdapter) ExtractAssertions(path, content string) []ExtractedAssertion {
	var assertions []ExtractedAssertion
	lines := strings.Split(content, "\n")

	currentDescribe := ""
	for i, line := range lines {
		lineNum := i + 1

		// Track describe block context (try double-quoted, then single-quoted)
		if m := describeBlockDQ.FindStringSubmatch(line); len(m) > 1 {
			currentDescribe = m[1]
		} else if m := describeBlockSQ.FindStringSubmatch(line); len(m) > 1 {
			currentDescribe = m[1]
		}

		// Extract it/test assertions (try double-quoted, then single-quoted)
		m := itTestDQ.FindStringSubmatch(line)
		if len(m) < 2 {
			m = itTestSQ.FindStringSubmatch(line)
		}
		if len(m) > 1 {
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

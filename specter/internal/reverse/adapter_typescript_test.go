// @spec spec-reverse
package reverse

import (
	"strings"
	"testing"
)

var tsAdapter = &TypeScriptAdapter{}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_ZodSchema(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints zod schema", func(t *testing.T) {
		content := `import { z } from 'zod';

const UserSchema = z.object({
  name: z.string().min(1).max(100),
  email: z.string().email(),
  age: z.number().min(18).max(120),
  bio: z.string().optional(),
});
`
		constraints := tsAdapter.ExtractConstraints("schema.ts", content)
		if len(constraints) == 0 {
			t.Fatal("expected constraints, got none")
		}

		// Collect rules by field
		type fieldRule struct {
			field string
			rule  string
		}
		found := make(map[fieldRule]bool)
		for _, c := range constraints {
			found[fieldRule{c.Field, c.Rule}] = true
		}

		// name: required, min=1, max=100
		for _, expected := range []fieldRule{
			{"name", "required"},
			{"name", "min"},
			{"name", "max"},
			{"email", "required"},
			{"email", "format"},
			{"age", "required"},
			{"age", "min"},
			{"age", "max"},
		} {
			if !found[expected] {
				t.Errorf("expected field=%q rule=%q, not found", expected.field, expected.rule)
			}
		}

		// bio has .optional(), so it should NOT have required
		if found[fieldRule{"bio", "required"}] {
			t.Error("bio should not have required rule (it has .optional())")
		}
	})
}

// @ac AC-04
func TestTypeScriptAdapter_ExtractAssertions_JestDescribeIt(t *testing.T) {
	t.Run("spec-reverse/AC-04 typescript adapter extract assertions jest describe it", func(t *testing.T) {
		content := `import { describe, it, expect } from 'vitest';

describe('UserService', () => {
  it('should create a valid user', () => {
    expect(true).toBe(true);
  });

  it('should reject invalid email', () => {
    expect(false).toBe(false);
  });

  it('should return error for missing name', () => {
    expect(false).toBe(false);
  });
});
`
		assertions := tsAdapter.ExtractAssertions("user.test.ts", content)
		if len(assertions) != 3 {
			t.Fatalf("expected 3 assertions, got %d", len(assertions))
		}

		if assertions[0].Description != "should create a valid user" {
			t.Errorf("first assertion description = %q, want %q", assertions[0].Description, "should create a valid user")
		}
		if assertions[0].TestName != "UserService > should create a valid user" {
			t.Errorf("first assertion test name = %q, want %q", assertions[0].TestName, "UserService > should create a valid user")
		}
		if assertions[0].IsError {
			t.Error("first assertion should NOT be marked as error")
		}

		if assertions[1].Description != "should reject invalid email" {
			t.Errorf("second assertion description = %q, want %q", assertions[1].Description, "should reject invalid email")
		}
		if !assertions[1].IsError {
			t.Error("second assertion should be marked as error (contains 'invalid')")
		}

		if !assertions[2].IsError {
			t.Error("third assertion should be marked as error (contains 'error')")
		}
	})
}

// @ac AC-15
func TestTypeScriptAdapter_ExtractAssertions_EmbeddedQuotes(t *testing.T) {
	t.Run("spec-reverse/AC-15 typescript adapter extract assertions embedded quotes", func(t *testing.T) {
		content := `import { it, describe } from 'vitest';

describe("user's profile", () => {
  it("user's token is valid", () => {});
  it('reject "admin" role for guests', () => {});
  it("handles \"quoted\" field names", () => {});
});
`
		assertions := tsAdapter.ExtractAssertions("auth.test.ts", content)
		if len(assertions) != 3 {
			t.Fatalf("expected 3 assertions, got %d", len(assertions))
		}

		cases := []struct {
			want string
		}{
			{"user's token is valid"},
			{`reject "admin" role for guests`},
			{`handles \"quoted\" field names`}, // raw JS source; backslash escape not unescaped
		}
		for i, c := range cases {
			if assertions[i].Description != c.want {
				t.Errorf("assertion[%d] description = %q, want %q", i, assertions[i].Description, c.want)
			}
		}

		// describe block with apostrophe should also be captured correctly
		if assertions[0].TestName != "user's profile > user's token is valid" {
			t.Errorf("test name = %q, want %q", assertions[0].TestName, "user's profile > user's token is valid")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractRoutes_Express(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract routes express", func(t *testing.T) {
		content := `import express from 'express';

const app = express();

app.get('/api/users', listUsers);
app.post('/api/users', createUser);
router.delete('/api/users/:id', deleteUser);
`
		routes := tsAdapter.ExtractRoutes("routes.ts", content)
		if len(routes) != 3 {
			t.Fatalf("expected 3 routes, got %d", len(routes))
		}
		if routes[0].Method != "GET" || routes[0].Path != "/api/users" {
			t.Errorf("first route = %s %s, want GET /api/users", routes[0].Method, routes[0].Path)
		}
		if routes[1].Method != "POST" || routes[1].Path != "/api/users" {
			t.Errorf("second route = %s %s, want POST /api/users", routes[1].Method, routes[1].Path)
		}
		if routes[2].Method != "DELETE" || routes[2].Path != "/api/users/:id" {
			t.Errorf("third route = %s %s, want DELETE /api/users/:id", routes[2].Method, routes[2].Path)
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractRoutes_NextJS(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract routes nextjs", func(t *testing.T) {
		content := `import { NextResponse } from 'next/server';

export async function GET(request: Request) {
  return NextResponse.json({ users: [] });
}

export async function POST(request: Request) {
  return NextResponse.json({ created: true });
}
`
		routes := tsAdapter.ExtractRoutes("src/app/api/users/route.ts", content)
		if len(routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(routes))
		}
		if routes[0].Method != "GET" || routes[0].Path != "/api/users" {
			t.Errorf("first route = %s %s, want GET /api/users", routes[0].Method, routes[0].Path)
		}
		if routes[1].Method != "POST" || routes[1].Path != "/api/users" {
			t.Errorf("second route = %s %s, want POST /api/users", routes[1].Method, routes[1].Path)
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractImports(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract imports", func(t *testing.T) {
		content := `import { z } from 'zod';
import express from 'express';
import { PrismaClient } from '@prisma/client';
import { authMiddleware } from '../middleware/auth';
`
		imports := tsAdapter.ExtractImports("index.ts", content)
		if len(imports) != 4 {
			t.Fatalf("expected 4 imports, got %d", len(imports))
		}
		modules := make(map[string]bool)
		for _, imp := range imports {
			modules[imp.Module] = true
		}
		if !modules["zod"] {
			t.Error("expected zod import")
		}
		if !modules["express"] {
			t.Error("expected express import")
		}
		if !modules["@prisma/client"] {
			t.Error("expected @prisma/client import")
		}
		if !modules["../middleware/auth"] {
			t.Error("expected ../middleware/auth import")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_InferSystemName(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter infer system name", func(t *testing.T) {
		files := []SourceFile{
			{Path: "package.json", Content: `{"name": "my-nextjs-app", "version": "1.0.0"}`},
			{Path: "index.ts", Content: "import express from 'express'"},
		}
		name := tsAdapter.InferSystemName(files)
		if name != "my-nextjs-app" {
			t.Errorf("InferSystemName = %q, want %q", name, "my-nextjs-app")
		}
	})
}

// @ac AC-12
func TestTypeScriptAdapter_Detect(t *testing.T) {
	t.Run("spec-reverse/AC-12 typescript adapter detect", func(t *testing.T) {
		for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
			if !tsAdapter.Detect("file"+ext, "") {
				t.Errorf("expected Detect to return true for %s file", ext)
			}
		}
		if tsAdapter.Detect("main.go", "") {
			t.Error("expected Detect to return false for .go file")
		}
		if tsAdapter.Detect("main.py", "") {
			t.Error("expected Detect to return false for .py file")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_TypeScriptEnum(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints typescript enum", func(t *testing.T) {
		content := `enum Role { ADMIN = "ADMIN", USER = "USER", MODERATOR = "MODERATOR" }
`
		constraints := tsAdapter.ExtractConstraints("types.ts", content)
		found := false
		for _, c := range constraints {
			if c.Rule == "enum" && strings.Contains(c.Field, "role") {
				found = true
			}
		}
		if !found {
			t.Error("expected enum constraint from TypeScript enum")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_UnionType(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints union type", func(t *testing.T) {
		content := `type Status = "active" | "inactive" | "pending"
`
		constraints := tsAdapter.ExtractConstraints("types.ts", content)
		found := false
		for _, c := range constraints {
			if c.Rule == "enum" && strings.Contains(c.Field, "status") {
				found = true
				if c.Value == nil {
					t.Error("expected union type values in Value field")
				}
			}
		}
		if !found {
			t.Error("expected enum constraint from union type")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_Prisma(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints prisma", func(t *testing.T) {
		content := `model User {
  id        String   @id @default(cuid())
  email     String   @unique @db.VarChar(255)
  name      String
  bio       String?
  role      String   @default("USER")
  createdAt DateTime @default(now())
}
`
		constraints := tsAdapter.ExtractConstraints("schema.prisma", content)
		if len(constraints) == 0 {
			t.Fatal("expected constraints from Prisma schema, got none")
		}

		type fieldRule struct {
			field string
			rule  string
		}
		found := make(map[fieldRule]bool)
		for _, c := range constraints {
			found[fieldRule{c.Field, c.Rule}] = true
		}

		if !found[fieldRule{"email", "unique"}] {
			t.Error("expected email unique constraint")
		}
		if !found[fieldRule{"email", "max"}] {
			t.Error("expected email max constraint from @db.VarChar(255)")
		}
		if !found[fieldRule{"name", "required"}] {
			t.Error("expected name required constraint")
		}
		if found[fieldRule{"bio", "required"}] {
			t.Error("bio should not be required (it has ?)")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_RoleChecks(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints role checks", func(t *testing.T) {
		content := `export function requireRole(session: Session) {
  if (session.user.role === "ADMIN") {
    return true;
  }
  if (session.user.role === "MODERATOR") {
    return true;
  }
  return false;
}
`
		constraints := tsAdapter.ExtractConstraints("auth.ts", content)
		found := false
		for _, c := range constraints {
			if c.Field == "role" && c.Rule == "enum" {
				found = true
				val, ok := c.Value.(string)
				if !ok {
					t.Error("expected string value for role enum")
				} else if !strings.Contains(val, "ADMIN") || !strings.Contains(val, "MODERATOR") {
					t.Errorf("expected role enum to contain ADMIN and MODERATOR, got %q", val)
				}
			}
		}
		if !found {
			t.Error("expected role enum constraint from role checks")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_ZodUrl(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints zod url", func(t *testing.T) {
		content := `const schema = z.object({
  website: z.string().url(),
});
`
		constraints := tsAdapter.ExtractConstraints("schema.ts", content)
		found := false
		for _, c := range constraints {
			if c.Field == "website" && c.Rule == "format" && c.Value == "url" {
				found = true
			}
		}
		if !found {
			t.Error("expected format=url constraint for z.string().url()")
		}
	})
}

// @ac AC-03
func TestTypeScriptAdapter_ExtractConstraints_ZodBooleanArray(t *testing.T) {
	t.Run("spec-reverse/AC-03 typescript adapter extract constraints zod boolean array", func(t *testing.T) {
		content := `const schema = z.object({
  active: z.boolean(),
  tags: z.array(z.string()),
});
`
		constraints := tsAdapter.ExtractConstraints("schema.ts", content)
		type fieldRule struct {
			field string
			rule  string
		}
		found := make(map[fieldRule]bool)
		for _, c := range constraints {
			found[fieldRule{c.Field, c.Rule}] = true
		}
		if !found[fieldRule{"active", "type"}] {
			t.Error("expected type=boolean constraint for z.boolean()")
		}
		if !found[fieldRule{"tags", "type"}] {
			t.Error("expected type=array constraint for z.array()")
		}
	})
}

// @ac AC-12
func TestTypeScriptAdapter_Detect_Prisma(t *testing.T) {
	t.Run("spec-reverse/AC-12 typescript adapter detect prisma", func(t *testing.T) {
		if !tsAdapter.Detect("schema.prisma", "") {
			t.Error("expected Detect to return true for .prisma file")
		}
	})
}

// @ac AC-12
func TestTypeScriptAdapter_IsTestFile(t *testing.T) {
	t.Run("spec-reverse/AC-12 typescript adapter is test file", func(t *testing.T) {
		testFiles := []string{
			"handler.test.ts",
			"handler.test.tsx",
			"handler.spec.ts",
			"handler.test.js",
			"handler.test.jsx",
			"__tests__/handler.ts",
			"src/__tests__/deep/util.ts",
		}
		for _, f := range testFiles {
			if !tsAdapter.IsTestFile(f) {
				t.Errorf("expected IsTestFile true for %q", f)
			}
		}

		nonTestFiles := []string{
			"handler.ts",
			"handler.tsx",
			"index.js",
			"schema.ts",
		}
		for _, f := range nonTestFiles {
			if tsAdapter.IsTestFile(f) {
				t.Errorf("expected IsTestFile false for %q", f)
			}
		}
	})
}

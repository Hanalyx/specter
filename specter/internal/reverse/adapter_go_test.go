// @spec spec-reverse
package reverse

import "testing"

var goAdapter = &GoAdapter{}

// @ac AC-01
func TestGoAdapter_ExtractConstraints_StructTags(t *testing.T) {
	t.Run("spec-reverse/AC-01 extract constraints from struct tags", func(t *testing.T) {
		content := `package main

type CreateUserRequest struct {
	Name  string ` + "`" + `json:"name" validate:"required,min=1,max=100"` + "`" + `
	Email string ` + "`" + `json:"email" validate:"required,email"` + "`" + `
	Age   int    ` + "`" + `json:"age" validate:"min=18,max=120"` + "`" + `
}
`
		constraints := goAdapter.ExtractConstraints("handler.go", content)
		if len(constraints) == 0 {
			t.Fatal("expected constraints, got none")
		}

		// Check we found required, min, max, email rules
		rules := make(map[string]bool)
		for _, c := range constraints {
			rules[c.Rule] = true
		}

		for _, expected := range []string{"required", "min", "max", "email"} {
			if !rules[expected] {
				t.Errorf("expected rule %q, not found in extracted constraints", expected)
			}
		}
	})
}

// @ac AC-02
func TestGoAdapter_ExtractAssertions_SubTests(t *testing.T) {
	t.Run("spec-reverse/AC-02 extract assertions from subtests", func(t *testing.T) {
		content := `package main

import "testing"

func TestCreateUser(t *testing.T) {
	t.Run("returns 200 on valid input", func(t *testing.T) {
		// ...
	})
	t.Run("returns 400 when name is empty", func(t *testing.T) {
		// ...
	})
}
`
		assertions := goAdapter.ExtractAssertions("handler_test.go", content)
		if len(assertions) != 2 {
			t.Fatalf("expected 2 assertions, got %d", len(assertions))
		}
		if assertions[0].Description != "returns 200 on valid input" {
			t.Errorf("first assertion description = %q, want %q", assertions[0].Description, "returns 200 on valid input")
		}
		if assertions[1].Description != "returns 400 when name is empty" {
			t.Errorf("second assertion description = %q, want %q", assertions[1].Description, "returns 400 when name is empty")
		}
		if !assertions[1].IsError {
			t.Error("second assertion should be marked as error case")
		}
	})
}

// @ac AC-02
func TestGoAdapter_ExtractAssertions_TableDriven(t *testing.T) {
	t.Run("spec-reverse/AC-02 extract assertions from table-driven", func(t *testing.T) {
		content := `package main

import "testing"

func TestValidateAge(t *testing.T) {
	tests := []struct {
		name string
		age  int
		want bool
	}{
		{name: "valid age", age: 25, want: true},
		{name: "invalid age below minimum", age: 10, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {})
	}
}
`
		assertions := goAdapter.ExtractAssertions("validate_test.go", content)
		if len(assertions) != 2 {
			t.Fatalf("expected 2 assertions, got %d", len(assertions))
		}
		if assertions[0].Description != "valid age" {
			t.Errorf("got %q, want %q", assertions[0].Description, "valid age")
		}
		if assertions[1].Description != "invalid age below minimum" {
			t.Errorf("got %q, want %q", assertions[1].Description, "invalid age below minimum")
		}
	})
}

// @ac AC-01
func TestGoAdapter_ExtractRoutes_HttpHandleFunc(t *testing.T) {
	t.Run("spec-reverse/AC-01 extract routes from http.HandleFunc", func(t *testing.T) {
		content := `package main

import "net/http"

func main() {
	http.HandleFunc("/api/users", handleUsers)
	http.HandleFunc("/api/health", handleHealth)
}
`
		routes := goAdapter.ExtractRoutes("main.go", content)
		if len(routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(routes))
		}
		if routes[0].Path != "/api/users" {
			t.Errorf("first route path = %q, want %q", routes[0].Path, "/api/users")
		}
		if routes[0].Handler != "handleUsers" {
			t.Errorf("first route handler = %q, want %q", routes[0].Handler, "handleUsers")
		}
	})
}

// @ac AC-01
func TestGoAdapter_ExtractRoutes_Gin(t *testing.T) {
	t.Run("spec-reverse/AC-01 extract routes from Gin", func(t *testing.T) {
		content := `package main

func setupRoutes(r *gin.Engine) {
	r.GET("/api/users", listUsers)
	r.POST("/api/users", createUser)
	r.DELETE("/api/users/:id", deleteUser)
}
`
		routes := goAdapter.ExtractRoutes("routes.go", content)
		if len(routes) != 3 {
			t.Fatalf("expected 3 routes, got %d", len(routes))
		}
		if routes[0].Method != "GET" || routes[0].Path != "/api/users" {
			t.Errorf("first route = %s %s, want GET /api/users", routes[0].Method, routes[0].Path)
		}
		if routes[2].Method != "DELETE" || routes[2].Path != "/api/users/:id" {
			t.Errorf("third route = %s %s, want DELETE /api/users/:id", routes[2].Method, routes[2].Path)
		}
	})
}

// @ac AC-01
func TestGoAdapter_ExtractImports(t *testing.T) {
	t.Run("spec-reverse/AC-01 extract imports", func(t *testing.T) {
		content := `package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/myorg/mylib/auth"
)
`
		imports := goAdapter.ExtractImports("main.go", content)
		if len(imports) != 4 {
			t.Fatalf("expected 4 imports, got %d", len(imports))
		}
		modules := make(map[string]bool)
		for _, imp := range imports {
			modules[imp.Module] = true
		}
		if !modules["github.com/gin-gonic/gin"] {
			t.Error("expected gin import")
		}
		if !modules["net/http"] {
			t.Error("expected net/http import")
		}
	})
}

// @ac AC-01
func TestGoAdapter_InferSystemName(t *testing.T) {
	t.Run("spec-reverse/AC-01 infer system name from go.mod", func(t *testing.T) {
		files := []SourceFile{
			{Path: "go.mod", Content: "module github.com/hanalyx/specter\n\ngo 1.22\n"},
			{Path: "main.go", Content: "package main"},
		}
		name := goAdapter.InferSystemName(files)
		if name != "specter" {
			t.Errorf("InferSystemName = %q, want %q", name, "specter")
		}
	})
}

// @ac AC-11
func TestGoAdapter_Detect(t *testing.T) {
	t.Run("spec-reverse/AC-11 detect go files", func(t *testing.T) {
		if !goAdapter.Detect("main.go", "") {
			t.Error("expected Detect to return true for .go file")
		}
		if goAdapter.Detect("main.py", "") {
			t.Error("expected Detect to return false for .py file")
		}
	})
}

// @ac AC-11
func TestGoAdapter_IsTestFile(t *testing.T) {
	t.Run("spec-reverse/AC-11 identify _test.go files", func(t *testing.T) {
		if !goAdapter.IsTestFile("handler_test.go") {
			t.Error("expected IsTestFile true for _test.go")
		}
		if goAdapter.IsTestFile("handler.go") {
			t.Error("expected IsTestFile false for .go without _test")
		}
	})
}

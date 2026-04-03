// @spec spec-reverse
package reverse

import "testing"

var pythonAdapter = &PythonAdapter{}

// @ac AC-05
func TestPythonAdapter_ExtractConstraints_PydanticField(t *testing.T) {
	content := `from pydantic import BaseModel, Field

class CreateUserRequest(BaseModel):
    name: str = Field(min_length=1, max_length=100)
    age: int = Field(ge=18, le=120)
    email: str = Field(regex="^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+$")
`
	constraints := pythonAdapter.ExtractConstraints("models.py", content)
	if len(constraints) == 0 {
		t.Fatal("expected constraints, got none")
	}

	// Collect all (field, rule) pairs
	type fieldRule struct{ field, rule string }
	found := make(map[fieldRule]bool)
	for _, c := range constraints {
		found[fieldRule{c.Field, c.Rule}] = true
	}

	expected := []fieldRule{
		{"name", "min_length"},
		{"name", "max_length"},
		{"age", "ge"},
		{"age", "le"},
		{"email", "pattern"},
	}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected constraint field=%q rule=%q, not found", e.field, e.rule)
		}
	}

	// Check specific values
	for _, c := range constraints {
		if c.Field == "name" && c.Rule == "min_length" {
			if c.Value != "1" {
				t.Errorf("name min_length value = %v, want %q", c.Value, "1")
			}
		}
		if c.Field == "name" && c.Rule == "max_length" {
			if c.Value != "100" {
				t.Errorf("name max_length value = %v, want %q", c.Value, "100")
			}
		}
	}
}

// @ac AC-05
func TestPythonAdapter_ExtractConstraints_SQLAlchemy(t *testing.T) {
	content := `from sqlalchemy import Column, String, Integer

class User(Base):
    username = Column(String(50), nullable=False)
    bio = Column(String(500), nullable=True)
`
	constraints := pythonAdapter.ExtractConstraints("models.py", content)

	foundRequired := false
	for _, c := range constraints {
		if c.Field == "username" && c.Rule == "required" {
			foundRequired = true
		}
	}
	if !foundRequired {
		t.Error("expected required constraint for username (nullable=False)")
	}

	// bio should NOT be required
	for _, c := range constraints {
		if c.Field == "bio" && c.Rule == "required" {
			t.Error("bio should not be required (nullable=True)")
		}
	}
}

// @ac AC-06
func TestPythonAdapter_ExtractAssertions_PytestFunctions(t *testing.T) {
	content := `import pytest

def test_create_user_success():
    response = client.post("/users", json={"name": "Alice"})
    assert response.status_code == 201

def test_create_user_missing_name():
    response = client.post("/users", json={})
    assert response.status_code == 422
`
	assertions := pythonAdapter.ExtractAssertions("test_users.py", content)
	if len(assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(assertions))
	}

	if assertions[0].Description != "create user success" {
		t.Errorf("first assertion description = %q, want %q", assertions[0].Description, "create user success")
	}
	if assertions[0].IsError {
		t.Error("first assertion should not be error case")
	}

	if assertions[1].Description != "create user missing name" {
		t.Errorf("second assertion description = %q, want %q", assertions[1].Description, "create user missing name")
	}
	if !assertions[1].IsError {
		t.Error("second assertion should be marked as error case (missing)")
	}
}

// @ac AC-06
func TestPythonAdapter_ExtractAssertions_PytestRaises(t *testing.T) {
	content := `import pytest

def test_invalid_age_raises_error():
    with pytest.raises(ValueError):
        validate_age(-1)
`
	assertions := pythonAdapter.ExtractAssertions("test_validation.py", content)
	if len(assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(assertions))
	}
	if !assertions[0].IsError {
		t.Error("expected error case for pytest.raises")
	}
	if assertions[0].ErrorDesc != "ValueError" {
		t.Errorf("ErrorDesc = %q, want %q", assertions[0].ErrorDesc, "ValueError")
	}
}

// @ac AC-06
func TestPythonAdapter_ExtractAssertions_ClassBased(t *testing.T) {
	content := `import pytest

class TestUserAPI:
    def test_list_users(self):
        response = client.get("/users")
        assert response.status_code == 200

    def test_delete_user_not_found(self):
        response = client.delete("/users/999")
        assert response.status_code == 404
`
	assertions := pythonAdapter.ExtractAssertions("test_api.py", content)
	if len(assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(assertions))
	}
	if assertions[0].TestName != "TestUserAPI::test_list_users" {
		t.Errorf("first test name = %q, want %q", assertions[0].TestName, "TestUserAPI::test_list_users")
	}
	if assertions[1].TestName != "TestUserAPI::test_delete_user_not_found" {
		t.Errorf("second test name = %q, want %q", assertions[1].TestName, "TestUserAPI::test_delete_user_not_found")
	}
}

func TestPythonAdapter_ExtractRoutes_FastAPI(t *testing.T) {
	content := `from fastapi import FastAPI

app = FastAPI()

@app.get("/api/users")
async def list_users():
    pass

@app.post("/api/users")
async def create_user():
    pass

@router.delete("/api/users/{user_id}")
async def delete_user(user_id: int):
    pass
`
	routes := pythonAdapter.ExtractRoutes("main.py", content)
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}
	if routes[0].Method != "GET" || routes[0].Path != "/api/users" {
		t.Errorf("first route = %s %s, want GET /api/users", routes[0].Method, routes[0].Path)
	}
	if routes[1].Method != "POST" || routes[1].Path != "/api/users" {
		t.Errorf("second route = %s %s, want POST /api/users", routes[1].Method, routes[1].Path)
	}
	if routes[2].Method != "DELETE" || routes[2].Path != "/api/users/{user_id}" {
		t.Errorf("third route = %s %s, want DELETE /api/users/{user_id}", routes[2].Method, routes[2].Path)
	}
}

func TestPythonAdapter_ExtractRoutes_Flask(t *testing.T) {
	content := `from flask import Flask

app = Flask(__name__)

@app.route("/health")
def health():
    return "ok"

@app.route("/api/items", methods=["POST", "PUT"])
def create_item():
    pass
`
	routes := pythonAdapter.ExtractRoutes("app.py", content)
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Method != "ANY" || routes[0].Path != "/health" {
		t.Errorf("first route = %s %s, want ANY /health", routes[0].Method, routes[0].Path)
	}
	if routes[1].Method != "POST" || routes[1].Path != "/api/items" {
		t.Errorf("second route = %s %s, want POST /api/items", routes[1].Method, routes[1].Path)
	}
}

func TestPythonAdapter_ExtractRoutes_Django(t *testing.T) {
	content := `from django.urls import path

urlpatterns = [
    path("api/users/", views.user_list),
    path("api/users/<int:pk>/", views.user_detail),
]
`
	routes := pythonAdapter.ExtractRoutes("urls.py", content)
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Path != "api/users/" {
		t.Errorf("first route path = %q, want %q", routes[0].Path, "api/users/")
	}
	if routes[0].Method != "ANY" {
		t.Errorf("django route method = %q, want %q", routes[0].Method, "ANY")
	}
}

func TestPythonAdapter_ExtractImports(t *testing.T) {
	content := `from fastapi import FastAPI
from pydantic import BaseModel
import os
import sys
from myapp.models import User
`
	imports := pythonAdapter.ExtractImports("main.py", content)
	if len(imports) != 5 {
		t.Fatalf("expected 5 imports, got %d", len(imports))
	}

	modules := make(map[string]bool)
	for _, imp := range imports {
		modules[imp.Module] = true
	}

	for _, expected := range []string{"fastapi", "pydantic", "os", "sys", "myapp.models"} {
		if !modules[expected] {
			t.Errorf("expected import %q, not found", expected)
		}
	}
}

func TestPythonAdapter_InferSystemName(t *testing.T) {
	files := []SourceFile{
		{Path: "pyproject.toml", Content: `[project]
name = "my-fastapi-app"
version = "0.1.0"
`},
		{Path: "main.py", Content: "from fastapi import FastAPI"},
	}
	name := pythonAdapter.InferSystemName(files)
	if name != "my-fastapi-app" {
		t.Errorf("InferSystemName = %q, want %q", name, "my-fastapi-app")
	}
}

func TestPythonAdapter_InferSystemName_NoToml(t *testing.T) {
	files := []SourceFile{
		{Path: "main.py", Content: "from fastapi import FastAPI"},
	}
	name := pythonAdapter.InferSystemName(files)
	if name != "" {
		t.Errorf("InferSystemName = %q, want empty string", name)
	}
}

func TestPythonAdapter_Detect(t *testing.T) {
	if !pythonAdapter.Detect("main.py", "") {
		t.Error("expected Detect to return true for .py file")
	}
	if pythonAdapter.Detect("main.go", "") {
		t.Error("expected Detect to return false for .go file")
	}
}

func TestPythonAdapter_IsTestFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test_users.py", true},
		{"users_test.py", true},
		{"app/tests/test_api.py", true},
		{"app/tests/conftest.py", true},
		{"models.py", false},
		{"main.py", false},
	}
	for _, tt := range tests {
		got := pythonAdapter.IsTestFile(tt.path)
		if got != tt.want {
			t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// @spec spec-reverse
package reverse

import "testing"

// @ac AC-08
func TestGenerateSpecID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"src/UserRegistration.ts", "user-registration"},
		{"handler.go", "handler"},
		{"handler_test.go", "handler"},
		{"auth.test.ts", "auth"},
		{"CreatePayment.tsx", "create-payment"},
		{"src/api/users/route.ts", "users-route"},
		{"simple-zod.ts", "simple-zod"},
		{"models.py", "models"},
		{"src/lib/auth/index.ts", "auth-index"},
		{"examples/rest/main.go", "rest-main"},
		{"src/utils.py", "src-utils"},
		{"test_auth.py", "test-auth"},
		{"pydantic-model.py", "pydantic-model"},
		{"MyService.handler.ts", "my-service"},
		{"123file.go", "spec-123file"},
		{"", "unknown-spec"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GenerateSpecID(tt.input)
			if got != tt.want {
				t.Errorf("GenerateSpecID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSpecIDFromRoute(t *testing.T) {
	tests := []struct {
		route string
		want  string
	}{
		{"/api/webhooks/stripe", "webhooks-stripe"},
		{"/api/blog/[slug]", "blog-slug"},
		{"/api/onboarding", "onboarding"},
		{"/api/auth/register", "auth-register"},
		{"/api/blog/categories", "blog-categories"},
		{"/unknown", "unknown"},
		{"/api/", "api-root"},
		{"/", "api-root"},
	}
	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			got := GenerateSpecIDFromRoute(tt.route)
			if got != tt.want {
				t.Errorf("GenerateSpecIDFromRoute(%q) = %q, want %q", tt.route, got, tt.want)
			}
		})
	}
}

func TestGenerateSpecID_KebabCasePattern(t *testing.T) {
	inputs := []string{
		"UserRegistration.ts",
		"createPaymentIntent.js",
		"APIHandler.go",
		"simple.py",
	}

	for _, input := range inputs {
		id := GenerateSpecID(input)
		// Must start with lowercase letter
		if len(id) == 0 || id[0] < 'a' || id[0] > 'z' {
			t.Errorf("GenerateSpecID(%q) = %q, must start with lowercase letter", input, id)
		}
		// Must only contain lowercase letters, digits, hyphens
		for _, r := range id {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Errorf("GenerateSpecID(%q) = %q, contains invalid character %q", input, id, string(r))
			}
		}
	}
}

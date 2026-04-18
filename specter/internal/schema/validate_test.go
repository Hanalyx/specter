// @spec spec-parse
package schema

import (
	"strings"
	"testing"
)

func validSpec() SpecAST {
	return SpecAST{
		ID: "test-spec", Version: "1.0.0", Status: StatusApproved, Tier: 2,
		Context:     SpecContext{System: "test"},
		Objective:   SpecObjective{Summary: "test"},
		Constraints: []Constraint{{ID: "C-01", Description: "test"}},
		AcceptanceCriteria: []AcceptanceCriterion{
			{ID: "AC-01", Description: "test", ReferencesConstraints: []string{"C-01"}},
		},
	}
}

// @ac AC-18 (v0.7.0 — internal enum validators)
func TestValidateEnums_ValidSpec(t *testing.T) {
	s := validSpec()
	if err := s.ValidateEnums(); err != nil {
		t.Fatalf("valid spec should pass ValidateEnums, got: %v", err)
	}
}

func TestValidateEnums_InvalidStatus(t *testing.T) {
	s := validSpec()
	s.Status = "banana"
	err := s.ValidateEnums()
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Errorf("expected error mentioning status, got: %v", err)
	}
}

func TestValidateEnums_InvalidTier(t *testing.T) {
	s := validSpec()
	s.Tier = 42
	err := s.ValidateEnums()
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Errorf("expected tier error, got: %v", err)
	}
}

func TestValidateEnums_InvalidEnforcement(t *testing.T) {
	s := validSpec()
	s.Constraints[0].Enforcement = "critical" // valid priority, NOT valid enforcement
	err := s.ValidateEnums()
	if err == nil || !strings.Contains(err.Error(), "enforcement") {
		t.Errorf("expected enforcement error, got: %v", err)
	}
}

func TestValidateEnums_InvalidPriority(t *testing.T) {
	s := validSpec()
	s.AcceptanceCriteria[0].Priority = "urgent"
	err := s.ValidateEnums()
	if err == nil || !strings.Contains(err.Error(), "priority") {
		t.Errorf("expected priority error, got: %v", err)
	}
}

func TestValidateEnums_EmptyOptionalFields(t *testing.T) {
	// Empty optional enum fields should be allowed (treated as unset).
	s := validSpec()
	s.Constraints[0].Type = ""
	s.Constraints[0].Enforcement = ""
	s.AcceptanceCriteria[0].Priority = ""
	if err := s.ValidateEnums(); err != nil {
		t.Errorf("empty optional enums should be allowed, got: %v", err)
	}
}

func TestValidateEnums_ConstantsMatchSchema(t *testing.T) {
	// Guard: if someone changes a constant value, this test surfaces the drift.
	// Pairs the Go constants to the canonical values that appear in the JSON Schema.
	cases := map[string]string{
		EnforcementError:   "error",
		StatusApproved:     "approved",
		PriorityCritical:   "critical",
		RelationshipRequires: "requires",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant drift: expected %q, got %q", want, got)
		}
	}
}

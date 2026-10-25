// templates.go -- C-13: spec file templates for specter init --template.
//
// Provides ready-to-use draft .spec.yaml templates that pass ParseSpec() and,
// per C-41, `specter check --concrete`: every constraint is referenced by a
// criterion, and every criterion carries placeholder inputs and
// expected_output that show the shape a real criterion needs.
//
// @spec spec-manifest
package manifest

import "fmt"

// SpecTemplate returns a valid draft .spec.yaml for the given template type.
// Supported types: api-endpoint, service, auth, data-model.
//
// C-13: all templates must pass ParseSpec() and have status: draft.
func SpecTemplate(templateType string) (string, error) {
	switch templateType {
	case "api-endpoint":
		return apiEndpointTemplate, nil
	case "service":
		return serviceTemplate, nil
	case "auth":
		return authTemplate, nil
	case "data-model":
		return dataModelTemplate, nil
	default:
		return "", fmt.Errorf("unknown template %q: use one of api-endpoint, service, auth, data-model", templateType)
	}
}

const apiEndpointTemplate = `spec:
  id: my-endpoint
  version: "1.0.0"
  status: draft
  tier: 2

  context:
    system: My System
    feature: REST API Endpoint
    description: >
      Describe what this endpoint does and why it exists.

  objective:
    summary: >
      Handle HTTP requests to this endpoint and return the appropriate response.
    scope:
      includes:
        - "Request validation"
        - "Business logic execution"
        - "Response formatting"
      excludes:
        - "Authentication (handled by middleware)"

  constraints:
    - id: C-01
      description: "MUST validate all required request fields before processing"
      type: technical
      enforcement: error

    - id: C-02
      description: "MUST return 400 Bad Request for invalid input"
      type: technical
      enforcement: error

    - id: C-03
      description: "MUST return 200 OK with the expected response schema on success"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Valid request returns 200 with expected response body"
      inputs:
        request: "<a valid request body>"
      expected_output:
        status: 200
        body: "<the response schema C-03 names>"
      references_constraints: ["C-03"]
      priority: critical

    - id: AC-02
      description: "Invalid request returns 400 with an actionable error message"
      inputs:
        request: "<a request with one field set to an invalid value>"
      expected_output:
        status: 400
        error_message: "<names the invalid field and the rule it broke>"
      error_cases:
        - condition: "A field holds a value of the wrong type or out of range"
          expected_behavior: "Return 400 and do not run the business logic"
      references_constraints: ["C-02"]
      priority: high

    - id: AC-03
      description: "Missing required fields return 400 listing the missing fields"
      inputs:
        request: "<a request with one required field removed>"
      expected_output:
        status: 400
        missing_fields: ["<name of the removed field>"]
      error_cases:
        - condition: "A required field is absent"
          expected_behavior: "Return 400 listing every missing field"
      references_constraints: ["C-01", "C-02"]
      priority: high
`

const serviceTemplate = `spec:
  id: my-service
  version: "1.0.0"
  status: draft
  tier: 2

  context:
    system: My System
    feature: Business Logic Service
    description: >
      Describe the domain service and the business problem it solves.

  objective:
    summary: >
      Encapsulate business logic for this domain and expose it as a pure function interface.
    scope:
      includes:
        - "Core business rules"
        - "Input validation"
        - "Output transformation"
      excludes:
        - "Persistence (handled by repository layer)"
        - "HTTP concerns"

  constraints:
    - id: C-01
      description: "MUST be a pure function with no side effects on invalid input"
      type: technical
      enforcement: error

    - id: C-02
      description: "MUST return a structured error on validation failure"
      type: technical
      enforcement: error

    - id: C-03
      description: "MUST NOT depend on any external services directly"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Valid input produces the expected output"
      inputs:
        input: "<a valid input value>"
      expected_output:
        result: "<the output that input must produce>"
      references_constraints: ["C-01"]
      priority: critical

    - id: AC-02
      description: "Invalid input returns a structured error with a descriptive message"
      inputs:
        input: "<an input that breaks one validation rule>"
      expected_output:
        error_code: "<the structured error code>"
        error_message: "<names the rule the input broke>"
      error_cases:
        - condition: "The input fails validation"
          expected_behavior: "Return a structured error and change no state"
      references_constraints: ["C-02"]
      priority: high

    - id: AC-03
      description: "Identical inputs always produce identical outputs (pure function)"
      inputs:
        input: "<any valid input, passed twice>"
      expected_output:
        outputs_identical: true
      references_constraints: ["C-01"]
      priority: high

    - id: AC-04
      description: "The service runs with every external dependency replaced by a test double"
      inputs:
        dependencies: "<each external dependency, replaced by a test double>"
      expected_output:
        result: "<the same output as AC-01>"
        external_calls: 0
      references_constraints: ["C-03"]
      priority: high
`

const authTemplate = `spec:
  id: my-auth
  version: "1.0.0"
  status: draft
  tier: 1

  context:
    system: My System
    feature: Authentication and Authorization
    description: >
      Describe the authentication mechanism and what resources it protects.

  objective:
    summary: >
      Verify user identity and enforce access control for protected resources.
    scope:
      includes:
        - "Token validation"
        - "Permission enforcement"
        - "Session management"
      excludes:
        - "User registration"
        - "Password reset flows"

  constraints:
    - id: C-01
      description: "MUST reject requests with missing or expired tokens"
      type: security
      enforcement: error

    - id: C-02
      description: "MUST NOT expose sensitive credential details in error responses"
      type: security
      enforcement: error

    - id: C-03
      description: "MUST enforce role-based access control for all protected endpoints"
      type: security
      enforcement: error

    - id: C-04
      description: "MUST log all authentication failures for audit purposes"
      type: security
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Valid token grants access to protected resource"
      inputs:
        token: "<a valid, unexpired token for a user with the required role>"
        resource: "<a protected resource>"
      expected_output:
        status: 200
      references_constraints: ["C-03"]
      priority: critical

    - id: AC-02
      description: "Expired token is rejected with 401 Unauthorized"
      inputs:
        token: "<a token whose expiry is in the past>"
      expected_output:
        status: 401
      error_cases:
        - condition: "The token has expired"
          expected_behavior: "Return 401 and grant no access"
      references_constraints: ["C-01"]
      priority: critical

    - id: AC-03
      description: "Missing token is rejected with 401 Unauthorized"
      inputs:
        token: ""
      expected_output:
        status: 401
      error_cases:
        - condition: "The request carries no token"
          expected_behavior: "Return 401 and grant no access"
      references_constraints: ["C-01"]
      priority: critical

    - id: AC-04
      description: "Insufficient role returns 403 Forbidden, not 401"
      inputs:
        token: "<a valid token for a user without the required role>"
      expected_output:
        status: 403
      error_cases:
        - condition: "The user is authenticated but lacks the role"
          expected_behavior: "Return 403, not 401"
      references_constraints: ["C-03"]
      priority: critical

    - id: AC-05
      description: "Error response does not reveal token contents or internal state"
      inputs:
        token: "<an invalid token>"
      expected_output:
        body_contains_token: false
        body_contains_stack_trace: false
      error_cases:
        - condition: "Any authentication failure"
          expected_behavior: "Return a generic error body with no credential or internal detail"
      references_constraints: ["C-02"]
      priority: high

    - id: AC-06
      description: "Every rejected request writes one audit log entry naming the reason"
      inputs:
        token: "<an expired token>"
      expected_output:
        audit_entries_written: 1
        audit_reason: "<names the reason, such as token expired>"
      error_cases:
        - condition: "A request is rejected for any authentication reason"
          expected_behavior: "Write exactly one audit entry before responding"
      references_constraints: ["C-04"]
      priority: high
`

const dataModelTemplate = `spec:
  id: my-data-model
  version: "1.0.0"
  status: draft
  tier: 2

  context:
    system: My System
    feature: Data Model
    description: >
      Describe the entity this model represents and its role in the domain.

  objective:
    summary: >
      Define the canonical shape, validation rules, and invariants for this data entity.
    scope:
      includes:
        - "Field definitions and types"
        - "Validation constraints"
        - "Uniqueness requirements"
      excludes:
        - "Persistence implementation"
        - "API serialization format"

  constraints:
    - id: C-01
      description: "MUST require a unique identifier field"
      type: technical
      enforcement: error

    - id: C-02
      description: "MUST validate all required fields are non-empty before persistence"
      type: technical
      enforcement: error

    - id: C-03
      description: "MUST enforce field length limits to prevent database overflow"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Valid entity with all required fields passes validation"
      inputs:
        entity: "<an entity with every required field set to a valid value>"
      expected_output:
        valid: true
      references_constraints: ["C-02"]
      priority: critical

    - id: AC-02
      description: "Entity with missing required field fails validation with a descriptive error"
      inputs:
        entity: "<an entity with one required field empty>"
      expected_output:
        valid: false
        error_field: "<name of the empty field>"
      error_cases:
        - condition: "A required field is empty"
          expected_behavior: "Fail validation and name the field"
      references_constraints: ["C-02"]
      priority: high

    - id: AC-03
      description: "Entity with field exceeding length limit fails validation"
      inputs:
        entity: "<an entity with one field one character over its limit>"
      expected_output:
        valid: false
        error_field: "<name of the long field>"
      error_cases:
        - condition: "A field is longer than its limit"
          expected_behavior: "Fail validation before persistence"
      references_constraints: ["C-03"]
      priority: high

    - id: AC-04
      description: "Entity with no identifier fails validation, and a second entity with the same identifier is rejected"
      inputs:
        first_entity: "<a valid entity>"
        second_entity: "<a valid entity with the same identifier>"
      expected_output:
        first_saved: true
        second_rejected: true
      error_cases:
        - condition: "The identifier is missing"
          expected_behavior: "Fail validation"
        - condition: "The identifier is already in use"
          expected_behavior: "Reject the second entity"
      references_constraints: ["C-01"]
      priority: high
`

// templates.go -- C-13: spec file templates for specter init --template.
//
// Provides ready-to-use draft .spec.yaml templates that pass ParseSpec().
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
      priority: critical

    - id: AC-02
      description: "Invalid request returns 400 with an actionable error message"
      priority: high

    - id: AC-03
      description: "Missing required fields return 400 listing the missing fields"
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
      priority: critical

    - id: AC-02
      description: "Invalid input returns a structured error with a descriptive message"
      priority: high

    - id: AC-03
      description: "Identical inputs always produce identical outputs (pure function)"
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
      priority: critical

    - id: AC-02
      description: "Expired token is rejected with 401 Unauthorized"
      priority: critical

    - id: AC-03
      description: "Missing token is rejected with 401 Unauthorized"
      priority: critical

    - id: AC-04
      description: "Insufficient role returns 403 Forbidden, not 401"
      priority: critical

    - id: AC-05
      description: "Error response does not reveal token contents or internal state"
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
      priority: critical

    - id: AC-02
      description: "Entity with missing required field fails validation with a descriptive error"
      priority: high

    - id: AC-03
      description: "Entity with field exceeding length limit fails validation"
      priority: high
`

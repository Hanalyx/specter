# Chapter 3: API Blueprinting

## MODULE 02 — Defining the Architecture (The "How") | Intermediate Level

---

## Lecture Preamble

*The professor opens two browser tabs: one showing the Anthropic API documentation, the other showing the OpenAI API reference. Both are meticulously structured, with every endpoint, parameter, and error code documented.*

Take a good look at these two pages. The Anthropic API docs and the OpenAI API reference are among the finest examples of API specification in the industry. Every request body is defined with exact types. Every response shape is documented. Every error code has a description and a recommended action. Every edge case — rate limits, token limits, malformed inputs — is accounted for.

These docs are not written after the fact. They are the **blueprint** from which the API is built. Anthropic's engineers do not code an endpoint and then write docs about it. They define the contract first — the request shape, the response shape, the error codes, the constraints — and then they implement to that contract. OpenAI does the same thing.

This is **API Blueprinting**, and it is the third pillar of architectural specification in SDD. If schema-first design gives you the data shapes, and component contracts give you the UI behaviors, API blueprints give you the connections between them — the request/response cycles that move data through your system.

Today we are going to learn how to write API blueprints that are so precise, an AI can implement them faithfully on the first try.

---

## 3.1 What Is an API Blueprint?

An API blueprint is a formal specification of an API endpoint that defines:

1. **The endpoint** — HTTP method, URL path, path parameters
2. **Authentication** — what credentials are required
3. **Request** — headers, query parameters, request body (with exact types)
4. **Response** — status codes, response bodies for each status code
5. **Errors** — specific error codes, their meanings, and recommended actions
6. **Constraints** — rate limits, size limits, pagination rules
7. **Edge cases** — what happens with invalid input, missing data, concurrent requests

A well-written API blueprint reads like a contract between the client and the server. The client agrees to send data in the specified format. The server agrees to respond in the specified format. If either side violates the contract, the system should fail in a predictable, documented way.

```yaml
# API Blueprint: Create a Task
# This is what we are going to learn to write in this chapter.

endpoint: POST /api/v1/projects/{projectId}/tasks
authentication: Bearer token (JWT)
authorization: User must be a member of the project

request:
  path_parameters:
    projectId: UUID (required)
  headers:
    Content-Type: application/json
    Authorization: Bearer {token}
    Idempotency-Key: string (optional, for retry safety)
  body:
    title: string (1-500 chars, required)
    description: string (0-50000 chars, optional, default "")
    priority: enum [urgent, high, medium, low, none] (optional, default "none")
    assigneeId: UUID | null (optional, default null)
    labelIds: UUID[] (optional, default [], max 20 items)
    dueDate: ISO 8601 date | null (optional, default null)
    parentTaskId: UUID | null (optional, default null)

responses:
  201 Created:
    body: Task object (full schema)
    headers:
      Location: /api/v1/tasks/{taskId}
  400 Bad Request:
    body: error with code VALIDATION_ERROR
  401 Unauthorized:
    body: error with code AUTHENTICATION_REQUIRED
  403 Forbidden:
    body: error with code INSUFFICIENT_PERMISSIONS
  404 Not Found:
    body: error with code PROJECT_NOT_FOUND
  409 Conflict:
    body: error with code DUPLICATE_IDEMPOTENCY_KEY
  422 Unprocessable Entity:
    body: error with code INVALID_REFERENCE (assigneeId or parentTaskId does not exist)
  429 Too Many Requests:
    body: error with code RATE_LIMIT_EXCEEDED
    headers:
      Retry-After: seconds (integer)

rate_limit: 100 requests per minute per user
```

That is a blueprint. It is precise, complete, and implementable. Let us learn how to write these systematically.

---

## 3.2 OpenAPI/Swagger: The Industry Standard

OpenAPI Specification (formerly Swagger) is the industry standard for describing REST APIs. It is used by virtually every major tech company, including Google, Microsoft, Amazon, Stripe, Twilio, and — importantly for this course — both OpenAI and Anthropic use OpenAPI-style specifications for their own APIs.

### Why OpenAPI Matters for SDD

OpenAPI matters because it is **machine-readable**. An OpenAPI specification can be consumed by:

- **Code generators** — generate client SDKs in any language
- **Documentation generators** — Swagger UI, Redoc, Stoplight
- **Mock servers** — generate fake APIs for frontend development
- **Test generators** — generate integration tests from the spec
- **AI models** — Claude and GPT can read and implement from OpenAPI specs
- **Validation middleware** — automatically validate requests and responses

When you write an OpenAPI spec, you are writing a document that serves humans, machines, and AI simultaneously.

### A Complete OpenAPI Example

Here is the "Create Task" endpoint from our blueprint, written as a full OpenAPI 3.1 specification:

```yaml
openapi: 3.1.0
info:
  title: Task Management API
  version: 1.0.0
  description: |
    API for managing tasks within projects.
    Authentication uses JWT Bearer tokens.
  contact:
    name: API Team
    email: api-team@example.com

servers:
  - url: https://api.example.com/v1
    description: Production
  - url: https://staging-api.example.com/v1
    description: Staging

security:
  - BearerAuth: []

paths:
  /projects/{projectId}/tasks:
    post:
      operationId: createTask
      summary: Create a new task in a project
      description: |
        Creates a new task within the specified project.
        The authenticated user must be a member of the project.
        The task will be created with status "backlog" and assigned
        a sequential display ID (e.g., "PROJ-42").
      tags:
        - Tasks
      parameters:
        - name: projectId
          in: path
          required: true
          description: The UUID of the project to create the task in
          schema:
            type: string
            format: uuid
        - name: Idempotency-Key
          in: header
          required: false
          description: |
            Unique key for idempotent requests. If a request with the
            same key was already processed, the original response is
            returned without creating a duplicate task.
          schema:
            type: string
            maxLength: 255
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateTaskRequest'
            examples:
              minimal:
                summary: Minimal task (title only)
                value:
                  title: "Fix login page bug"
              complete:
                summary: Fully specified task
                value:
                  title: "Implement user profile page"
                  description: "Build the user profile page with avatar upload and bio editing"
                  priority: "high"
                  assigneeId: "880e8400-e29b-41d4-a716-446655440003"
                  labelIds:
                    - "990e8400-e29b-41d4-a716-446655440004"
                    - "aa0e8400-e29b-41d4-a716-446655440005"
                  dueDate: "2026-03-15"
      responses:
        '201':
          description: Task created successfully
          headers:
            Location:
              description: URL of the newly created task
              schema:
                type: string
                format: uri
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiResponse_Task'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '403':
          $ref: '#/components/responses/Forbidden'
        '404':
          description: Project not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiErrorResponse'
              example:
                success: false
                data: null
                error:
                  code: "PROJECT_NOT_FOUND"
                  message: "Project with ID '550e8400...' does not exist"
                  details: null
                metadata:
                  requestId: "req_abc123"
                  timestamp: "2026-02-24T10:30:00Z"
                  processingTimeMs: 12
        '409':
          description: Duplicate idempotency key
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiErrorResponse'
        '422':
          description: Referenced entity does not exist
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiErrorResponse'
              example:
                success: false
                data: null
                error:
                  code: "INVALID_REFERENCE"
                  message: "Assignee with ID '880e8400...' is not a member of this project"
                  details:
                    field: "assigneeId"
                    value: "880e8400-e29b-41d4-a716-446655440003"
                metadata:
                  requestId: "req_def456"
                  timestamp: "2026-02-24T10:30:01Z"
                  processingTimeMs: 28
        '429':
          $ref: '#/components/responses/RateLimited'

    get:
      operationId: listTasks
      summary: List tasks in a project
      description: |
        Returns a paginated list of tasks in the specified project.
        Supports filtering by status, priority, assignee, and labels.
        Results are sorted by creation date (newest first) by default.
      tags:
        - Tasks
      parameters:
        - name: projectId
          in: path
          required: true
          schema:
            type: string
            format: uuid
        - name: status
          in: query
          required: false
          description: Filter by task status (comma-separated for multiple)
          schema:
            type: string
            example: "todo,in_progress"
        - name: priority
          in: query
          required: false
          schema:
            type: string
            enum: [urgent, high, medium, low, none]
        - name: assigneeId
          in: query
          required: false
          schema:
            type: string
            format: uuid
        - name: labelIds
          in: query
          required: false
          description: Filter by label IDs (comma-separated, tasks must have ALL specified labels)
          schema:
            type: string
        - name: search
          in: query
          required: false
          description: Full-text search on title and description
          schema:
            type: string
            maxLength: 200
        - name: sortBy
          in: query
          required: false
          schema:
            type: string
            enum: [created_at, updated_at, priority, due_date]
            default: created_at
        - name: sortOrder
          in: query
          required: false
          schema:
            type: string
            enum: [asc, desc]
            default: desc
        - name: cursor
          in: query
          required: false
          description: Cursor for pagination (opaque string from previous response)
          schema:
            type: string
        - name: limit
          in: query
          required: false
          description: Number of tasks to return (1-100)
          schema:
            type: integer
            minimum: 1
            maximum: 100
            default: 20
      responses:
        '200':
          description: List of tasks
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiResponse_TaskList'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '403':
          $ref: '#/components/responses/Forbidden'
        '404':
          description: Project not found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ApiErrorResponse'

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: |
        JWT token obtained from POST /auth/login.
        Include in Authorization header: Bearer {token}

  schemas:
    # --- Request Schemas ---

    CreateTaskRequest:
      type: object
      required:
        - title
      properties:
        title:
          type: string
          minLength: 1
          maxLength: 500
          description: The title of the task
        description:
          type: string
          maxLength: 50000
          default: ""
          description: Detailed description (Markdown supported)
        priority:
          type: string
          enum: [urgent, high, medium, low, none]
          default: none
        assigneeId:
          type: string
          format: uuid
          nullable: true
          default: null
          description: User ID of the assignee (must be project member)
        labelIds:
          type: array
          items:
            type: string
            format: uuid
          maxItems: 20
          uniqueItems: true
          default: []
          description: Label IDs to apply (must exist in workspace)
        dueDate:
          type: string
          format: date
          nullable: true
          default: null
          description: Due date in YYYY-MM-DD format
        parentTaskId:
          type: string
          format: uuid
          nullable: true
          default: null
          description: Parent task ID (for sub-tasks, must be in same project)
      additionalProperties: false

    UpdateTaskRequest:
      type: object
      minProperties: 1
      properties:
        title:
          type: string
          minLength: 1
          maxLength: 500
        description:
          type: string
          maxLength: 50000
        status:
          type: string
          enum: [backlog, todo, in_progress, in_review, done, cancelled]
        priority:
          type: string
          enum: [urgent, high, medium, low, none]
        assigneeId:
          type: string
          format: uuid
          nullable: true
        labelIds:
          type: array
          items:
            type: string
            format: uuid
          maxItems: 20
          uniqueItems: true
        dueDate:
          type: string
          format: date
          nullable: true
        parentTaskId:
          type: string
          format: uuid
          nullable: true
        estimatePoints:
          type: number
          minimum: 0
          nullable: true
      additionalProperties: false

    # --- Response Schemas ---

    Task:
      type: object
      required:
        - id
        - projectId
        - sequenceNumber
        - displayId
        - title
        - description
        - status
        - priority
        - creatorId
        - labelIds
        - commentCount
        - attachmentCount
        - createdAt
        - updatedAt
      properties:
        id:
          type: string
          format: uuid
        projectId:
          type: string
          format: uuid
        sequenceNumber:
          type: integer
          minimum: 1
        displayId:
          type: string
          pattern: '^[A-Z]{2,5}-[0-9]+$'
        title:
          type: string
        description:
          type: string
        status:
          type: string
          enum: [backlog, todo, in_progress, in_review, done, cancelled]
        priority:
          type: string
          enum: [urgent, high, medium, low, none]
        creatorId:
          type: string
          format: uuid
        assigneeId:
          type: string
          format: uuid
          nullable: true
        labelIds:
          type: array
          items:
            type: string
            format: uuid
        parentTaskId:
          type: string
          format: uuid
          nullable: true
        dueDate:
          type: string
          format: date
          nullable: true
        estimatePoints:
          type: number
          nullable: true
        commentCount:
          type: integer
          minimum: 0
        attachmentCount:
          type: integer
          minimum: 0
        createdAt:
          type: string
          format: date-time
        updatedAt:
          type: string
          format: date-time
        completedAt:
          type: string
          format: date-time
          nullable: true
      additionalProperties: false

    TaskList:
      type: object
      required:
        - items
        - nextCursor
        - totalCount
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/Task'
        nextCursor:
          type: string
          nullable: true
          description: Cursor for next page, null if no more results
        totalCount:
          type: integer
          minimum: 0
          description: Total number of tasks matching the filters

    # --- Envelope Schemas ---

    ApiResponse_Task:
      type: object
      required: [success, data, error, metadata]
      properties:
        success:
          type: boolean
          const: true
        data:
          $ref: '#/components/schemas/Task'
        error:
          type: 'null'
        metadata:
          $ref: '#/components/schemas/ResponseMetadata'

    ApiResponse_TaskList:
      type: object
      required: [success, data, error, metadata]
      properties:
        success:
          type: boolean
          const: true
        data:
          $ref: '#/components/schemas/TaskList'
        error:
          type: 'null'
        metadata:
          $ref: '#/components/schemas/ResponseMetadata'

    ApiErrorResponse:
      type: object
      required: [success, data, error, metadata]
      properties:
        success:
          type: boolean
          const: false
        data:
          type: 'null'
        error:
          $ref: '#/components/schemas/ApiError'
        metadata:
          $ref: '#/components/schemas/ResponseMetadata'

    ApiError:
      type: object
      required: [code, message]
      properties:
        code:
          type: string
          description: Machine-readable error code
        message:
          type: string
          description: Human-readable error message
        details:
          type: object
          nullable: true
          description: Additional context about the error
          additionalProperties: true

    ResponseMetadata:
      type: object
      required: [requestId, timestamp, processingTimeMs]
      properties:
        requestId:
          type: string
          description: Unique identifier for this request (for support)
        timestamp:
          type: string
          format: date-time
        processingTimeMs:
          type: integer
          minimum: 0

  responses:
    BadRequest:
      description: Invalid request format or validation error
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ApiErrorResponse'
          example:
            success: false
            data: null
            error:
              code: "VALIDATION_ERROR"
              message: "Request validation failed"
              details:
                errors:
                  - field: "title"
                    code: "REQUIRED"
                    message: "Title is required"
                  - field: "priority"
                    code: "INVALID_ENUM"
                    message: "Priority must be one of: urgent, high, medium, low, none"
            metadata:
              requestId: "req_xyz789"
              timestamp: "2026-02-24T10:30:00Z"
              processingTimeMs: 5

    Unauthorized:
      description: Missing or invalid authentication token
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ApiErrorResponse'
          example:
            success: false
            data: null
            error:
              code: "AUTHENTICATION_REQUIRED"
              message: "A valid authentication token is required"
              details: null
            metadata:
              requestId: "req_auth001"
              timestamp: "2026-02-24T10:30:00Z"
              processingTimeMs: 2

    Forbidden:
      description: User does not have permission for this action
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ApiErrorResponse'
          example:
            success: false
            data: null
            error:
              code: "INSUFFICIENT_PERMISSIONS"
              message: "You do not have permission to create tasks in this project"
              details:
                requiredRole: "member"
                currentRole: "guest"
            metadata:
              requestId: "req_perm002"
              timestamp: "2026-02-24T10:30:00Z"
              processingTimeMs: 8

    RateLimited:
      description: Too many requests
      headers:
        Retry-After:
          description: Seconds to wait before retrying
          schema:
            type: integer
        X-RateLimit-Limit:
          description: Request limit per minute
          schema:
            type: integer
        X-RateLimit-Remaining:
          description: Remaining requests in current window
          schema:
            type: integer
        X-RateLimit-Reset:
          description: Unix timestamp when the rate limit resets
          schema:
            type: integer
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ApiErrorResponse'
          example:
            success: false
            data: null
            error:
              code: "RATE_LIMIT_EXCEEDED"
              message: "Rate limit exceeded. Try again in 30 seconds."
              details:
                limit: 100
                window: "1 minute"
                retryAfter: 30
            metadata:
              requestId: "req_rl003"
              timestamp: "2026-02-24T10:30:00Z"
              processingTimeMs: 1
```

> **Professor's Aside:** That was a long specification. But look at what we have now: a machine-readable, human-readable, AI-readable document that completely defines two API endpoints. A code generator can produce a client SDK from this. A test generator can produce integration tests. An AI can implement the backend handlers. A frontend developer can build against mock data that matches these exact shapes. This one document replaces weeks of back-and-forth between teams.

---

## 3.3 How to Write an API Spec That an AI Can Implement Faithfully

Not all API specs are equally useful for AI-assisted implementation. Here are the principles that make the difference between a spec that an AI nails on the first try and one that requires three rounds of revision.

### Principle 1: Be Explicit About Every Decision

AI models are excellent at following instructions but poor at reading your mind. Every time you leave something ambiguous, the AI will fill the gap with its training data — which may or may not match your intent.

```yaml
# BAD: Ambiguous
paths:
  /tasks:
    get:
      summary: Get tasks
      responses:
        200:
          description: List of tasks

# GOOD: Explicit
paths:
  /projects/{projectId}/tasks:
    get:
      summary: List tasks in a project with filtering and pagination
      description: |
        Returns tasks matching the provided filters.
        Results are paginated using cursor-based pagination.
        Default sort: created_at descending.
        Maximum page size: 100.
        Empty filter values are ignored (not treated as "match empty").
      parameters:
        # [every parameter defined with types, defaults, and constraints]
      responses:
        200:
          description: |
            Paginated list of tasks.
            nextCursor is null when there are no more results.
            totalCount reflects the total matching tasks, not just this page.
```

### Principle 2: Define Error Responses as Thoroughly as Success Responses

This is where most API specs fail. They carefully define the happy path and then hand-wave at errors with "400: Bad Request" and nothing else.

```yaml
# BAD: Error hand-waving
responses:
  400:
    description: Bad request
  500:
    description: Server error

# GOOD: Specific error contracts
responses:
  400:
    description: Validation error
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/ApiErrorResponse'
        examples:
          missing_title:
            summary: Title is required
            value:
              success: false
              data: null
              error:
                code: "VALIDATION_ERROR"
                message: "Request validation failed"
                details:
                  errors:
                    - field: "title"
                      code: "REQUIRED"
                      message: "Title is required"
          title_too_long:
            summary: Title exceeds maximum length
            value:
              success: false
              data: null
              error:
                code: "VALIDATION_ERROR"
                message: "Request validation failed"
                details:
                  errors:
                    - field: "title"
                      code: "TOO_LONG"
                      message: "Title must be 500 characters or fewer"
                      limit: 500
                      actual: 523
```

### Principle 3: Include Examples for Every Endpoint

Examples serve dual purposes: they illustrate the spec for human readers and they provide concrete test cases for AI implementation.

```yaml
requestBody:
  content:
    application/json:
      schema:
        $ref: '#/components/schemas/CreateTaskRequest'
      examples:
        minimal:
          summary: Minimal task creation
          description: Only the required title field
          value:
            title: "Fix login bug"
        with_assignment:
          summary: Task with assignee and priority
          value:
            title: "Implement search feature"
            description: "Add full-text search to the task list"
            priority: "high"
            assigneeId: "880e8400-e29b-41d4-a716-446655440003"
        with_all_fields:
          summary: Task with all optional fields
          value:
            title: "Design system audit"
            description: "Review all components for accessibility compliance"
            priority: "medium"
            assigneeId: "880e8400-e29b-41d4-a716-446655440003"
            labelIds:
              - "990e8400-e29b-41d4-a716-446655440004"
            dueDate: "2026-04-01"
            parentTaskId: "aa0e8400-e29b-41d4-a716-446655440005"
```

### Principle 4: Specify Behavior, Not Just Shape

The OpenAPI spec defines data shapes. But for AI implementation, you also need to describe *behavior* — what the endpoint actually does.

```yaml
post:
  operationId: createTask
  description: |
    Creates a new task within the specified project.

    BEHAVIOR:
    1. Validate the request body against the schema.
    2. Verify the authenticated user is a member of the project.
    3. If assigneeId is provided, verify the assignee is a project member.
    4. If parentTaskId is provided, verify the parent task exists in the same project.
    5. If labelIds are provided, verify all labels exist in the workspace.
    6. Generate the next sequential display ID for the project.
    7. Create the task with status "backlog" and the current timestamp.
    8. Return the full task object.

    IDEMPOTENCY:
    If Idempotency-Key header is provided:
    - Check if a task was already created with this key.
    - If yes, return the existing task with 201 (not 409).
    - If no, create the task and store the idempotency key.
    - Idempotency keys expire after 24 hours.

    SIDE EFFECTS:
    - Increment project.taskCount
    - Send notification to assignee (if assigneeId provided)
    - Log audit event: "task.created"
```

### Principle 5: Define What Does NOT Happen

Negative specifications are as important as positive ones:

```yaml
description: |
  DOES NOT:
  - Auto-assign the task to the creator if no assigneeId provided
  - Send notifications to all project members (only assignee)
  - Allow creating tasks in archived projects
  - Allow setting status to anything other than "backlog" on creation
  - Allow duplicate label IDs in the labelIds array
  - Cascade delete child tasks when a parent task is deleted
```

> **Professor's Aside:** That last point — "does NOT" — is something I wish I had learned earlier in my career. When I started writing API specs, I only described what the endpoint does. But AI models are creative. They will add behaviors they think are helpful. "Oh, the user probably wants a notification to go to all project members." "Oh, I should auto-assign the task to the creator." By explicitly stating what does NOT happen, you constrain the AI's creativity to where it belongs: in the implementation details, not in the business logic.

---

## 3.4 Authentication and Authorization in API Specs

Authentication (who are you?) and authorization (what can you do?) are among the most critical parts of an API blueprint. They are also among the most commonly under-specified.

### Authentication Spec

```yaml
# Authentication specification
# Include this in your API blueprint

authentication:
  method: JWT Bearer Token
  header: "Authorization: Bearer {token}"

  token_structure:
    type: JWT (RS256)
    payload:
      sub: "User ID (UUID)"
      email: "User's email address"
      role: "User's global role: admin | user"
      workspaceRoles: "Map of workspaceId -> role"
      iat: "Issued at (Unix timestamp)"
      exp: "Expiration (Unix timestamp, 15 minutes after iat)"

  token_lifecycle:
    access_token:
      duration: 15 minutes
      refresh: "Use refresh token to obtain new access token"
    refresh_token:
      duration: 30 days
      rotation: "Each use generates a new refresh token and invalidates the old one"
      revocation: "POST /auth/logout revokes all refresh tokens for the user"

  error_responses:
    missing_token:
      status: 401
      code: "AUTHENTICATION_REQUIRED"
      message: "Authentication token is required"
    expired_token:
      status: 401
      code: "TOKEN_EXPIRED"
      message: "Authentication token has expired"
      action: "Client should refresh the token using the refresh token"
    invalid_token:
      status: 401
      code: "INVALID_TOKEN"
      message: "Authentication token is malformed or has been tampered with"
      action: "Client should re-authenticate"
    revoked_token:
      status: 401
      code: "TOKEN_REVOKED"
      message: "Authentication token has been revoked"
      action: "Client should re-authenticate"
```

### Authorization Spec

```yaml
authorization:
  model: Role-Based Access Control (RBAC) at workspace level

  roles:
    owner:
      description: "Created the workspace. Full control."
      permissions: [all]
    admin:
      description: "Can manage members and settings."
      permissions:
        - manage_members
        - manage_projects
        - manage_labels
        - create_tasks
        - update_any_task
        - delete_any_task
        - view_all_tasks
    member:
      description: "Can create and manage their own tasks."
      permissions:
        - create_tasks
        - update_own_tasks
        - update_assigned_tasks
        - view_all_tasks
    guest:
      description: "Read-only access to assigned tasks."
      permissions:
        - view_assigned_tasks

  endpoint_permissions:
    POST /tasks:
      required: create_tasks
      additional: "assigneeId must reference a member of the same project"
    GET /tasks:
      required: view_all_tasks OR view_assigned_tasks
      behavior: "If view_assigned_tasks only, filter results to assigned tasks"
    PUT /tasks/{id}:
      required: update_any_task OR (update_own_tasks AND task.creatorId == user.id) OR (update_assigned_tasks AND task.assigneeId == user.id)
    DELETE /tasks/{id}:
      required: delete_any_task
      note: "Members cannot delete tasks, even their own"

  error_responses:
    insufficient_permissions:
      status: 403
      code: "INSUFFICIENT_PERMISSIONS"
      message: "You do not have permission to {action} in this {resource}"
      details:
        requiredPermission: "The permission that was checked"
        currentRole: "The user's role in this workspace"
```

This level of detail in the auth spec prevents several classes of bugs:

- The AI will not accidentally allow guests to create tasks
- The AI will correctly filter task lists based on role
- The AI will return the right error code (401 vs 403) in each case
- The AI will include role information in error details for debugging

---

## 3.5 Error Handling as a First-Class Spec Concern

Let me say this plainly: **error handling is not an afterthought. It is a first-class concern that should receive as much specification effort as the happy path.**

In my experience reviewing AI-generated APIs, the single most common failure is inadequate error handling. The AI generates a handler that returns the right data on success and throws a generic 500 error on everything else. This happens because the spec did not define error cases.

### The Error Taxonomy

Define a complete taxonomy of errors for your API:

```typescript
// Error code taxonomy for the Task Management API
// Every API handler must use these codes — no ad-hoc error strings.

type ErrorCode =
  // --- Authentication Errors (401) ---
  | "AUTHENTICATION_REQUIRED"     // No token provided
  | "TOKEN_EXPIRED"               // Token has expired
  | "INVALID_TOKEN"               // Token is malformed
  | "TOKEN_REVOKED"               // Token has been revoked

  // --- Authorization Errors (403) ---
  | "INSUFFICIENT_PERMISSIONS"    // User lacks required permission
  | "ACCOUNT_SUSPENDED"           // User's account is suspended
  | "WORKSPACE_LOCKED"            // Workspace is in locked state (billing)

  // --- Validation Errors (400) ---
  | "VALIDATION_ERROR"            // Request body validation failed
  | "INVALID_QUERY_PARAMETER"     // Query parameter is malformed
  | "MISSING_REQUIRED_FIELD"      // A required field is missing
  | "INVALID_FIELD_VALUE"         // A field value is invalid

  // --- Not Found Errors (404) ---
  | "RESOURCE_NOT_FOUND"          // Generic not found
  | "PROJECT_NOT_FOUND"           // Specific: project does not exist
  | "TASK_NOT_FOUND"              // Specific: task does not exist
  | "USER_NOT_FOUND"              // Specific: user does not exist

  // --- Conflict Errors (409) ---
  | "DUPLICATE_RESOURCE"          // Trying to create something that already exists
  | "DUPLICATE_IDEMPOTENCY_KEY"   // Idempotency key already used
  | "CONCURRENT_MODIFICATION"     // Resource was modified by another request

  // --- Unprocessable Errors (422) ---
  | "INVALID_REFERENCE"           // Referenced entity does not exist
  | "CIRCULAR_REFERENCE"          // e.g., task is its own parent
  | "BUSINESS_RULE_VIOLATION"     // Domain-specific rule violated

  // --- Rate Limiting (429) ---
  | "RATE_LIMIT_EXCEEDED"         // Too many requests

  // --- Server Errors (500) ---
  | "INTERNAL_ERROR"              // Unexpected server error
  | "SERVICE_UNAVAILABLE"         // Dependency is down
  | "DATABASE_ERROR";             // Database operation failed

// Every error response must include:
interface ApiError {
  code: ErrorCode;              // Machine-readable, used for programmatic handling
  message: string;              // Human-readable, safe to show to end users
  details: Record<string, unknown> | null;  // Additional context
}
```

### Mapping Errors to HTTP Status Codes

```typescript
const ERROR_STATUS_MAP: Record<ErrorCode, number> = {
  // 401
  AUTHENTICATION_REQUIRED: 401,
  TOKEN_EXPIRED: 401,
  INVALID_TOKEN: 401,
  TOKEN_REVOKED: 401,

  // 403
  INSUFFICIENT_PERMISSIONS: 403,
  ACCOUNT_SUSPENDED: 403,
  WORKSPACE_LOCKED: 403,

  // 400
  VALIDATION_ERROR: 400,
  INVALID_QUERY_PARAMETER: 400,
  MISSING_REQUIRED_FIELD: 400,
  INVALID_FIELD_VALUE: 400,

  // 404
  RESOURCE_NOT_FOUND: 404,
  PROJECT_NOT_FOUND: 404,
  TASK_NOT_FOUND: 404,
  USER_NOT_FOUND: 404,

  // 409
  DUPLICATE_RESOURCE: 409,
  DUPLICATE_IDEMPOTENCY_KEY: 409,
  CONCURRENT_MODIFICATION: 409,

  // 422
  INVALID_REFERENCE: 422,
  CIRCULAR_REFERENCE: 422,
  BUSINESS_RULE_VIOLATION: 422,

  // 429
  RATE_LIMIT_EXCEEDED: 429,

  // 500
  INTERNAL_ERROR: 500,
  SERVICE_UNAVAILABLE: 503,
  DATABASE_ERROR: 500,
};
```

### Per-Endpoint Error Specification

For each endpoint, list every error that can occur and how it is triggered:

```yaml
# POST /projects/{projectId}/tasks — Error Specification

errors:
  - status: 400
    code: VALIDATION_ERROR
    triggers:
      - "title is missing"
      - "title is empty string or whitespace only"
      - "title exceeds 500 characters"
      - "description exceeds 50000 characters"
      - "priority is not a valid enum value"
      - "assigneeId is not a valid UUID format"
      - "labelIds contains duplicate values"
      - "labelIds exceeds 20 items"
      - "dueDate is not in YYYY-MM-DD format"
      - "dueDate is in the past"
      - "request body contains unknown fields (additionalProperties: false)"

  - status: 401
    code: AUTHENTICATION_REQUIRED
    triggers:
      - "Authorization header is missing"
      - "Authorization header does not start with 'Bearer '"
    code: TOKEN_EXPIRED
    triggers:
      - "JWT exp claim is in the past"
    code: INVALID_TOKEN
    triggers:
      - "JWT signature verification fails"
      - "JWT payload is malformed"

  - status: 403
    code: INSUFFICIENT_PERMISSIONS
    triggers:
      - "User is not a member of the project"
      - "User is a guest (guests cannot create tasks)"

  - status: 404
    code: PROJECT_NOT_FOUND
    triggers:
      - "No project exists with the given projectId"
      - "Project exists but is in a different workspace"

  - status: 409
    code: DUPLICATE_IDEMPOTENCY_KEY
    triggers:
      - "Idempotency-Key header matches a key used in the last 24 hours AND the original request had different body content"

  - status: 422
    code: INVALID_REFERENCE
    triggers:
      - "assigneeId references a user who is not a member of the project"
      - "parentTaskId references a task that does not exist"
      - "parentTaskId references a task in a different project"
      - "Any labelId references a label that does not exist in the workspace"
    code: CIRCULAR_REFERENCE
    triggers:
      - "parentTaskId would create a circular task hierarchy"

  - status: 429
    code: RATE_LIMIT_EXCEEDED
    triggers:
      - "User has exceeded 100 requests per minute"
    headers:
      Retry-After: "Seconds until the rate limit resets"
```

> **Professor's Aside:** Count the error triggers in that list. There are over twenty distinct error cases for a single endpoint. If your API spec only says "400: Bad Request," an AI will implement maybe three of them. With this level of specificity, the AI implements all twenty. That is the difference between a prototype and a production API.

---

## 3.6 How AI API Providers Practice Spec-Driven Design

It is worth pausing to observe that the companies building AI models are themselves practitioners of spec-driven API design.

### Anthropic's API

Anthropic's Claude API is one of the cleanest spec-driven APIs in the industry. Consider their Messages API:

- Every request parameter has an explicit type and constraint
- The response uses a discriminated union (`type: "message"` vs `type: "error"`)
- Error codes are enumerated and documented
- Rate limits are documented with specific headers
- Streaming responses have a well-defined event format with typed events
- Tool use requires JSON Schema definitions (schema-first at the API level)

When Anthropic adds a new feature (like extended thinking or computer use), they define the API contract first, document it, and then ship the implementation. The documentation is not an afterthought — it is the specification.

### OpenAI's API

OpenAI's API reference follows similar principles. Their endpoints are documented with:

- Full JSON Schema for request and response bodies
- Enumerated error codes with descriptions
- Rate limit documentation per model and tier
- Deprecation policies with timelines
- Versioning strategy (date-based API versions)

Their Structured Outputs feature, which we discussed in Chapter 1, is itself an exercise in spec-driven design: the user provides a schema, and the API guarantees conformance.

### The Lesson

If the companies building the most advanced AI systems in the world use spec-driven API design, it is not because they have nothing better to do. It is because they have learned — at enormous scale — that unclear API contracts lead to integration failures, customer frustration, and wasted engineering time.

---

## 3.7 Rate Limiting, Pagination, and Edge Cases

These three topics are the most commonly under-specified parts of API blueprints. Let us address each one.

### Rate Limiting Specification

```yaml
rate_limiting:
  strategy: Token bucket per user per endpoint group

  tiers:
    free:
      tasks_read: 60 requests/minute
      tasks_write: 20 requests/minute
      search: 10 requests/minute
    pro:
      tasks_read: 300 requests/minute
      tasks_write: 100 requests/minute
      search: 60 requests/minute
    enterprise:
      tasks_read: 1000 requests/minute
      tasks_write: 500 requests/minute
      search: 300 requests/minute

  response_headers:
    X-RateLimit-Limit: "Maximum requests allowed in the window"
    X-RateLimit-Remaining: "Remaining requests in current window"
    X-RateLimit-Reset: "Unix timestamp when the window resets"
    Retry-After: "Seconds to wait (only on 429 responses)"

  behavior_on_limit:
    status: 429
    body:
      code: "RATE_LIMIT_EXCEEDED"
      message: "Rate limit exceeded for {endpoint_group}. Try again in {retryAfter} seconds."
      details:
        limit: 60
        window: "1 minute"
        retryAfter: 23
    note: "Requests that are rate-limited still count toward the limit"

  special_cases:
    - "Webhook deliveries are not rate-limited"
    - "Health check endpoint (/health) is not rate-limited"
    - "Authentication endpoints have their own limits: 10 login attempts per minute per IP"
```

### Pagination Specification

```yaml
pagination:
  strategy: Cursor-based (NOT offset-based)

  reasoning: |
    Cursor-based pagination is preferred because:
    1. Stable results even when items are added/removed between pages
    2. Better performance for large datasets (no OFFSET scanning)
    3. Simpler for real-time data where new items appear frequently

  parameters:
    cursor:
      type: string
      required: false
      description: |
        Opaque string from the previous response's nextCursor field.
        Do not construct or modify cursor values — treat as opaque.
      encoding: "Base64-encoded JSON: {id, sortField, sortValue}"
    limit:
      type: integer
      required: false
      default: 20
      minimum: 1
      maximum: 100
      description: "Number of items to return per page"

  response_fields:
    items: "Array of results (length <= limit)"
    nextCursor: "String | null. Null means no more results."
    totalCount: "Total items matching filters (not just this page)"

  behavior:
    first_page: "Omit cursor parameter"
    next_page: "Pass nextCursor from previous response as cursor"
    last_page: "nextCursor is null"
    empty_results: "items is empty array, nextCursor is null, totalCount is 0"
    invalid_cursor: "Return 400 with code INVALID_CURSOR"
    expired_cursor: |
      Cursors expire after 1 hour. Return 400 with code EXPIRED_CURSOR.
      Client should restart pagination from the first page.

  example_flow:
    - request: "GET /tasks?limit=2"
      response:
        items: ["task1", "task2"]
        nextCursor: "eyJpZCI6InRhc2syIn0="
        totalCount: 5
    - request: "GET /tasks?limit=2&cursor=eyJpZCI6InRhc2syIn0="
      response:
        items: ["task3", "task4"]
        nextCursor: "eyJpZCI6InRhc2s0In0="
        totalCount: 5
    - request: "GET /tasks?limit=2&cursor=eyJpZCI6InRhc2s0In0="
      response:
        items: ["task5"]
        nextCursor: null
        totalCount: 5
```

### Edge Cases Specification

```yaml
edge_cases:
  concurrent_requests:
    scenario: "Two requests try to create a task with the same Idempotency-Key simultaneously"
    behavior: "One succeeds with 201. The other waits and returns the same 201 response."
    implementation: "Database-level lock on idempotency key"

  large_payloads:
    max_request_body: "1 MB"
    behavior: "Return 413 Payload Too Large if exceeded"
    note: "Description field (50000 chars) can be ~50KB, well under the limit"

  unicode:
    behavior: "All string fields support full Unicode (including emoji)"
    normalization: "NFC normalization applied to title and description"
    search: "Full-text search is Unicode-aware (accent-insensitive)"

  timezone:
    behavior: "All timestamps in responses are UTC (ISO 8601 with Z suffix)"
    dueDate: "Date only (YYYY-MM-DD), timezone is determined by the user's workspace settings"

  soft_delete:
    behavior: "Deleted tasks are soft-deleted (marked as deleted, not removed from database)"
    api_impact: "Deleted tasks do not appear in list endpoints"
    get_endpoint: "GET /tasks/{id} for a deleted task returns 404"
    restore: "Not supported via API (admin tool only)"

  null_vs_missing:
    behavior: |
      In request bodies:
      - Missing field: use the default value (or keep current value for updates)
      - Null value: explicitly set the field to null
      Example: To unassign a task, send { "assigneeId": null }
      Example: To keep the current assignee, omit "assigneeId" from the request

  empty_arrays:
    behavior: |
      In request bodies:
      - labelIds: [] means "remove all labels"
      - Missing labelIds means "keep current labels" (for updates)
      In responses:
      - labelIds is always present, even if empty (never null)

  special_characters_in_paths:
    behavior: "Path parameters (UUIDs) do not contain special characters"
    query_parameters: "Search query is URL-encoded. Server decodes before processing."
```

---

## 3.8 Practical Walkthrough: Full REST API Spec for a Resource

Let us bring everything together. Here is a complete API specification for the Task resource, covering all CRUD operations plus search. I will present this in a concise, implementation-ready format.

```typescript
// ===================================================
// TASK API — COMPLETE SPECIFICATION
// ===================================================

// --- Types ---

interface Task {
  id: string;                 // UUID v4
  projectId: string;          // UUID v4
  sequenceNumber: number;     // Auto-increment per project
  displayId: string;          // "{prefix}-{sequenceNumber}"
  title: string;              // 1-500 chars
  description: string;        // 0-50000 chars, Markdown
  status: TaskStatus;
  priority: TaskPriority;
  creatorId: string;          // UUID v4
  assigneeId: string | null;  // UUID v4 or null
  labelIds: string[];         // UUID v4 array, max 20
  parentTaskId: string | null;
  dueDate: string | null;     // YYYY-MM-DD
  estimatePoints: number | null;
  commentCount: number;
  attachmentCount: number;
  createdAt: string;          // ISO 8601
  updatedAt: string;          // ISO 8601
  completedAt: string | null; // ISO 8601
}

type TaskStatus = "backlog" | "todo" | "in_progress" | "in_review" | "done" | "cancelled";
type TaskPriority = "urgent" | "high" | "medium" | "low" | "none";

// --- Endpoints ---

/**
 * CREATE TASK
 * POST /api/v1/projects/{projectId}/tasks
 *
 * Auth: Bearer token. User must have "create_tasks" permission in project.
 * Idempotency: Supports Idempotency-Key header.
 *
 * Request body:
 *   title: string (required, 1-500 chars)
 *   description?: string (default "", max 50000 chars)
 *   priority?: TaskPriority (default "none")
 *   assigneeId?: string | null (default null, must be project member)
 *   labelIds?: string[] (default [], max 20, must exist in workspace)
 *   dueDate?: string | null (default null, YYYY-MM-DD, must be today or future)
 *   parentTaskId?: string | null (default null, must be in same project)
 *
 * Responses:
 *   201: { success: true, data: Task, error: null }
 *        Header: Location: /api/v1/tasks/{id}
 *   400: Validation error
 *   401: Not authenticated
 *   403: Not authorized (not a project member, or guest role)
 *   404: Project not found
 *   422: Invalid reference (assignee not in project, label not in workspace, etc.)
 *   429: Rate limited
 *
 * Side effects:
 *   - Increments project.taskCount
 *   - Sends notification to assignee if assigneeId provided
 *   - Creates audit log entry: "task.created"
 *   - Sets status to "backlog" (cannot be overridden on creation)
 */

/**
 * GET TASK
 * GET /api/v1/tasks/{taskId}
 *
 * Auth: Bearer token. User must have "view" permission for this task.
 *
 * Path params:
 *   taskId: UUID (required)
 *
 * Query params:
 *   include?: string (comma-separated: "comments", "labels", "assignee", "creator")
 *     - "comments": include latest 10 comments as task.recentComments
 *     - "labels": include full label objects as task.labelDetails (not just IDs)
 *     - "assignee": include user object as task.assigneeDetails
 *     - "creator": include user object as task.creatorDetails
 *
 * Responses:
 *   200: { success: true, data: Task (with optional includes), error: null }
 *   401: Not authenticated
 *   403: Not authorized
 *   404: Task not found (also returned for soft-deleted tasks)
 *
 * Caching:
 *   ETag header included. Support If-None-Match for 304 responses.
 */

/**
 * LIST TASKS
 * GET /api/v1/projects/{projectId}/tasks
 *
 * Auth: Bearer token. User must have view permission.
 *       Guests see only tasks assigned to them.
 *
 * Path params:
 *   projectId: UUID (required)
 *
 * Query params:
 *   status?: string (comma-separated TaskStatus values)
 *   priority?: string (comma-separated TaskPriority values)
 *   assigneeId?: UUID (filter by assignee)
 *   creatorId?: UUID (filter by creator)
 *   labelIds?: string (comma-separated UUIDs, AND logic)
 *   hasAssignee?: boolean (true = assigned, false = unassigned)
 *   search?: string (full-text search on title and description, max 200 chars)
 *   dueBefore?: string (YYYY-MM-DD, inclusive)
 *   dueAfter?: string (YYYY-MM-DD, inclusive)
 *   parentTaskId?: UUID | "null" ("null" string means top-level tasks only)
 *   sortBy?: "created_at" | "updated_at" | "priority" | "due_date" | "sequence_number"
 *            (default: "created_at")
 *   sortOrder?: "asc" | "desc" (default: "desc")
 *   cursor?: string (opaque pagination cursor)
 *   limit?: integer (1-100, default 20)
 *
 * Responses:
 *   200: {
 *     success: true,
 *     data: {
 *       items: Task[],
 *       nextCursor: string | null,
 *       totalCount: number
 *     },
 *     error: null
 *   }
 *   400: Invalid query parameters
 *   401: Not authenticated
 *   403: Not authorized
 *   404: Project not found
 *
 * Notes:
 *   - Empty filters are ignored (not "match nothing")
 *   - labelIds uses AND logic: task must have ALL specified labels
 *   - search uses case-insensitive, accent-insensitive full-text matching
 *   - sortBy "priority" sorts: urgent > high > medium > low > none
 */

/**
 * UPDATE TASK
 * PATCH /api/v1/tasks/{taskId}
 *
 * Auth: Bearer token. Permission depends on what is being updated:
 *   - Status change: "update_own_tasks" (if creator) or "update_assigned_tasks" (if assignee) or "update_any_task"
 *   - Reassignment: "update_any_task" only
 *   - Other fields: "update_own_tasks" or "update_assigned_tasks" or "update_any_task"
 *
 * Path params:
 *   taskId: UUID (required)
 *
 * Headers:
 *   If-Match: ETag (optional, for optimistic concurrency control)
 *
 * Request body (at least one field required):
 *   title?: string (1-500 chars)
 *   description?: string (0-50000 chars)
 *   status?: TaskStatus
 *   priority?: TaskPriority
 *   assigneeId?: string | null (null to unassign)
 *   labelIds?: string[] (replaces all labels; empty array removes all)
 *   dueDate?: string | null (null to remove due date)
 *   parentTaskId?: string | null (null to make top-level)
 *   estimatePoints?: number | null (null to clear estimate)
 *
 * Responses:
 *   200: { success: true, data: Task (updated), error: null }
 *   400: Validation error or empty body
 *   401: Not authenticated
 *   403: Not authorized for this update
 *   404: Task not found
 *   409: Conflict (If-Match ETag does not match current version)
 *   422: Invalid reference
 *
 * Side effects:
 *   - Updates task.updatedAt to current timestamp
 *   - If status changed to "done" or "cancelled": set task.completedAt
 *   - If status changed FROM "done"/"cancelled" to anything else: clear task.completedAt
 *   - If assigneeId changed: notify new assignee, notify old assignee
 *   - If status changed: create audit log entry "task.status_changed"
 *   - If assigneeId changed: create audit log entry "task.reassigned"
 *
 * Status transition rules:
 *   backlog -> todo, cancelled
 *   todo -> in_progress, backlog, cancelled
 *   in_progress -> in_review, todo, cancelled
 *   in_review -> done, in_progress, cancelled
 *   done -> (no transitions allowed — reopen creates a new task)
 *   cancelled -> backlog (re-open)
 */

/**
 * DELETE TASK
 * DELETE /api/v1/tasks/{taskId}
 *
 * Auth: Bearer token. Requires "delete_any_task" permission.
 *       Members cannot delete tasks, even their own.
 *
 * Path params:
 *   taskId: UUID (required)
 *
 * Responses:
 *   204: No content (successful deletion)
 *   401: Not authenticated
 *   403: Not authorized (not admin/owner)
 *   404: Task not found
 *
 * Behavior:
 *   - Soft delete: sets deletedAt timestamp, excluded from all queries
 *   - Does NOT delete child tasks (they become top-level)
 *   - Does NOT delete comments (preserved for audit)
 *   - Decrements project.taskCount
 *   - Creates audit log entry: "task.deleted"
 *
 * DOES NOT:
 *   - Cascade delete to child tasks
 *   - Send notifications (deletion is silent)
 *   - Allow bulk deletion (one task at a time)
 */

/**
 * SEARCH TASKS (cross-project)
 * GET /api/v1/workspaces/{workspaceId}/tasks/search
 *
 * Auth: Bearer token. User must be a workspace member.
 *       Results filtered to tasks the user can see.
 *
 * Path params:
 *   workspaceId: UUID (required)
 *
 * Query params:
 *   q: string (required, 1-200 chars, the search query)
 *   projectIds?: string (comma-separated UUIDs, filter to specific projects)
 *   status?: string (comma-separated TaskStatus values)
 *   assigneeId?: UUID
 *   sortBy?: "relevance" | "created_at" | "updated_at" (default: "relevance")
 *   cursor?: string
 *   limit?: integer (1-50, default 20) — lower max than list endpoint
 *
 * Responses:
 *   200: {
 *     success: true,
 *     data: {
 *       items: SearchResult[],
 *       nextCursor: string | null,
 *       totalCount: number,
 *       queryTimeMs: number
 *     },
 *     error: null
 *   }
 *   400: Missing or invalid query
 *   401: Not authenticated
 *   403: Not a workspace member
 *
 * SearchResult extends Task with:
 *   relevanceScore: number (0-1, how well the task matches the query)
 *   matchHighlights: {
 *     title?: string (title with <mark> tags around matching text)
 *     description?: string (description snippet with <mark> tags)
 *   }
 *
 * Notes:
 *   - Search covers title and description fields
 *   - Uses case-insensitive, accent-insensitive matching
 *   - Supports quoted phrases: "exact match"
 *   - Supports field-specific search: title:"bug fix"
 *   - Results from projects the user cannot access are excluded
 *   - Rate limit: 10 requests/minute (free), 60/min (pro), 300/min (enterprise)
 */
```

---

## 3.9 From API Spec to Implementation

Here is how you hand this spec to an AI for implementation:

```markdown
## Implementation Request

Implement the Task API based on the specification below.

### Technology Stack:
- Runtime: Node.js 22 with TypeScript 5.6
- Framework: Hono (lightweight, Edge-compatible)
- Database: PostgreSQL 16 via Drizzle ORM
- Validation: Zod for request/response validation
- Auth: JWT validation middleware (assume it exists, types provided)
- Testing: Vitest for unit tests, Supertest for integration tests

### File Structure:
```
src/
  api/
    tasks/
      tasks.routes.ts      — Route definitions
      tasks.handlers.ts    — Request handlers
      tasks.service.ts     — Business logic
      tasks.schemas.ts     — Zod schemas for request/response validation
      tasks.errors.ts      — Error code definitions
      tasks.test.ts        — Integration tests
  middleware/
    auth.ts                — Auth middleware (already exists)
    rate-limit.ts          — Rate limiting middleware (already exists)
    validate.ts            — Request validation middleware (already exists)
```

### Requirements:
- Every endpoint must validate request against Zod schemas
- Every error case from the spec must be handled explicitly
- No try/catch swallowing errors — use a global error handler
- All database queries must use parameterized queries (no SQL injection)
- Cursor pagination using the specified strategy
- ETag support for GET single task and optimistic locking for PATCH
- All timestamps in UTC

### API Specification:
[paste the complete specification here]
```

The AI now has unambiguous instructions for every endpoint, every error case, every edge case, and every side effect. The implementation will match the spec.

---

## 3.10 Exercises

### Exercise 1: Spec a User Management API

Write a complete API blueprint for user management:

- `POST /api/v1/auth/register` — Register a new user
- `POST /api/v1/auth/login` — Log in with email and password
- `POST /api/v1/auth/refresh` — Refresh an access token
- `POST /api/v1/auth/logout` — Log out (revoke refresh token)
- `POST /api/v1/auth/forgot-password` — Request password reset
- `POST /api/v1/auth/reset-password` — Complete password reset
- `GET /api/v1/users/me` — Get current user profile
- `PATCH /api/v1/users/me` — Update current user profile

For each endpoint, define: request shape, all response shapes, all error codes with triggers, rate limits, and side effects. Pay special attention to security: brute-force protection on login, rate limiting on password reset, and token rotation on refresh.

### Exercise 2: Spec Error Handling for an Existing API

Take any public API you have used (Stripe, GitHub, Twilio, Discord) and study their error handling patterns. Then write an error taxonomy for the Task Management API that follows their patterns. Compare the quality of error information in your spec vs. the original API.

### Exercise 3: Pagination Comparison

Write the same "list tasks" endpoint using three different pagination strategies:

1. Offset-based (`?page=3&perPage=20`)
2. Cursor-based (`?cursor=abc123&limit=20`)
3. Keyset-based (`?after=task_id_xyz&limit=20`)

For each strategy, document the trade-offs: performance, stability during concurrent writes, implementation complexity, and client complexity. Which strategy would you recommend for an API that an AI will both implement and consume?

### Exercise 4: Full API Review

Take the complete Task API spec from section 3.8 and hand it to an AI (Claude, GPT, Gemini). Ask the AI to review the spec for:

1. Missing error cases
2. Security vulnerabilities
3. Performance concerns
4. Consistency issues

Document the AI's feedback. Were there genuine issues the spec missed? This exercise demonstrates that AI is not just a consumer of specs — it can also be a reviewer.

---

## 3.11 Key Takeaways

1. **API Blueprints** define the full contract between client and server: endpoints, requests, responses, errors, and constraints.

2. **OpenAPI/Swagger** is the industry standard for API specification. It is machine-readable, generates code and docs, and AI models understand it natively.

3. **Error handling is a first-class concern.** Define a complete error taxonomy with specific codes, HTTP status mappings, and per-endpoint trigger lists.

4. **AI companies practice what they preach.** Anthropic and OpenAI's own APIs are among the best examples of spec-driven API design.

5. **Spec behavior, not just shape.** Include what the endpoint does, what side effects it has, and what it does NOT do.

6. **Rate limiting and pagination** are not optional — they are core parts of the API contract that must be specified before implementation.

7. **The more specific your spec, the better your AI-generated implementation.** Twenty specific error cases produce twenty correct error handlers.

8. **Include examples** for every endpoint. Examples serve as documentation for humans and test cases for AI.

---

## Looking Ahead

In the next chapter, we complete the architectural specification picture with **State Management Specs**. You have learned how to define data shapes (schemas), UI behaviors (component contracts), and API connections (blueprints). Now we will specify how data flows through your application — the stores, actions, selectors, and side effects that connect everything together.

---

*End of Chapter 3 — API Blueprinting*

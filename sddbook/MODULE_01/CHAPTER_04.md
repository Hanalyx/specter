# Chapter 4: Practice — From Vibe to Spec

## MODULE 01 — Foundations: The "Contract" Mindset

---

### Lecture Preamble

*Alright. Enough theory. Today is workshop day.*

*For the past three chapters, I have been telling you why specs matter, what makes a good spec, and how specs fit into the development workflow. Today, you are going to do it. We are going to take real-world "vibe-coded" prompts — the kind of thing developers actually type into AI assistants every day — and transform them into structured specs.*

*This is where the rubber meets the road. Knowing the theory of specs is like knowing the theory of swimming: useful, but you are still going to swallow some water the first time you get in the pool. So let us get in the pool.*

*I am going to walk you through three exercises, each one more complex than the last. For each exercise, I will show you the vibe prompt, analyze why it is problematic, and then build the spec step by step with you. After the exercises, we will look at common anti-patterns and build a quality checklist you can use on your own specs.*

*Open your editors. This is a hands-on chapter.*

---

## 4.1 The Anatomy of a Vibe Prompt

Before we start refactoring, let us understand what we are refactoring *from.* A "vibe prompt" has recognizable characteristics:

1. **Conversational tone.** It reads like something you would say to a colleague, not something you would put in a technical document.

2. **Implicit assumptions.** It assumes the AI knows your project, your tech stack, your conventions, and your preferences.

3. **Vague scope.** It describes the general area of work but not the precise boundaries.

4. **Missing constraints.** It says what to build but not what to avoid.

5. **No success criteria.** There is no way to objectively determine if the output is correct.

6. **Ephemeral.** It exists in a chat window and will be lost after the session.

Here is a classic vibe prompt:

```
Hey, can you build me a todo list app with React? I want to be able to
add, edit, and delete todos. Make it look nice.
```

This prompt contains roughly 25 words and roughly 200 implicit decisions. Let us count some of them:

- "React" — which version? With TypeScript? What build tool?
- "todo list app" — full app or component? What routing?
- "add" — what fields? Title only? Title + description? Due date? Priority?
- "edit" — inline editing? Modal? Separate page?
- "delete" — soft delete? Hard delete? Confirmation dialog?
- "look nice" — what design system? What color scheme? Responsive?
- (Not mentioned) — persistence? localStorage? Database? API?
- (Not mentioned) — authentication? Multi-user?
- (Not mentioned) — testing? Accessibility? Error handling?

The developer knows the answers to most of these questions. They just did not type them. And the AI will answer every single one of them silently, probabilistically, based on its training data.

---

## 4.2 Exercise 1: A Simple CRUD Feature

### The Vibe Prompt

```
I need a user management page. It should show all users in a table,
let me add new users, edit existing ones, and delete them. We're using
React and TypeScript. The API is at /api/users.
```

This is actually a better-than-average vibe prompt. It mentions the tech stack and the API. But it still has significant gaps. Let us analyze them.

### Gap Analysis

| What is Stated | What is Missing |
|---|---|
| "user management page" | What user fields exist? What is the data model? |
| "show all users in a table" | Which columns? Pagination? Sorting? Filtering? |
| "add new users" | What form fields? Validation rules? Modal or page? |
| "edit existing ones" | Inline or separate form? Which fields are editable? |
| "delete them" | Soft or hard delete? Confirmation? Authorization? |
| "React and TypeScript" | Which versions? What other libraries? |
| "API at /api/users" | What HTTP methods? What request/response shapes? |
| (Not mentioned) | Loading states, error states, empty states |
| (Not mentioned) | Accessibility, responsive design |
| (Not mentioned) | Authorization (who can CRUD users?) |
| (Not mentioned) | Existing codebase patterns and conventions |

### Building the Spec

Let us transform this into a proper micro-spec. I will build it section by section.

**Step 1: Metadata**

```yaml
kind: Feature
metadata:
  name: UserManagement
  module: admin
  version: 1.0.0
  system_spec: system.spec.yaml@3.1.0
```

Already we have made a decision that the vibe prompt left implicit: this is an *admin* feature. That has implications for authorization, placement in the app, and UI design.

**Step 2: Context**

```yaml
context:
  description: >
    The admin section of the application currently has a dashboard
    and a settings page but no user management. Administrators need
    to view, create, edit, and deactivate user accounts. The backend
    API for user CRUD operations has been built and deployed.

  technical_context: >
    User API:
      GET    /api/users         → { users: User[], total: number, page: number }
      GET    /api/users/:id     → User
      POST   /api/users         → User (created)
      PUT    /api/users/:id     → User (updated)
      DELETE /api/users/:id     → { success: boolean } (soft-delete)

    User type:
      {
        id: string (UUID),
        email: string,
        name: string,
        role: "admin" | "editor" | "viewer",
        status: "active" | "inactive",
        createdAt: string (ISO 8601),
        lastLoginAt: string | null (ISO 8601)
      }

    Pagination: API supports ?page=N&limit=N (default limit 20, max 100)
    Sorting: API supports ?sort=field&order=asc|desc
    Filtering: API supports ?status=active|inactive&role=admin|editor|viewer

    Existing admin layout: AdminLayout component with sidebar navigation.
    Existing components: DataTable, Modal, FormField, ErrorBanner, Skeleton.

  related_specs:
    - auth.spec.yaml  # Role-based access control
    - admin-layout.spec.yaml  # Admin section layout and navigation
```

Notice how much richer this is than the vibe prompt. We have specified:

- The exact API contract (endpoints, methods, request/response shapes)
- The complete data model with types
- Pagination, sorting, and filtering capabilities
- Existing components that should be reused
- The layout context
- Related specs for cross-cutting concerns

**Step 3: Objective**

```yaml
objective:
  summary: >
    Create a user management interface in the admin section that allows
    administrators to list, create, edit, and deactivate user accounts.

  acceptance_criteria:
    # List View
    - User table displays columns: Name, Email, Role, Status, Last Login
    - Table is paginated with 20 users per page
    - Table supports sorting by any column (click column header)
    - Table supports filtering by Status and Role (dropdown filters)
    - Table shows total user count and current page
    - Empty state shows "No users found" message with appropriate context

    # Create User
    - "Create User" button opens a modal form
    - Form fields: Name (required), Email (required), Role (required, dropdown)
    - Email field validates email format
    - Name field requires minimum 2 characters
    - Form shows validation errors inline
    - Successful creation closes modal and refreshes table
    - Duplicate email shows specific error message from API

    # Edit User
    - Clicking a table row opens an edit modal pre-filled with user data
    - Edit form has same fields as create, plus Status toggle
    - Email field is read-only in edit mode (cannot change email)
    - Changes are saved on form submission
    - Successful save closes modal and refreshes table row

    # Deactivate User
    - Each row has a "Deactivate" action (visible on hover or in menu)
    - Deactivating shows a confirmation dialog with user's name
    - Confirmation dialog requires typing "DEACTIVATE" to proceed
    - Deactivation calls DELETE endpoint (soft-delete)
    - Deactivated users show "Inactive" status in table
    - Already inactive users show a "Reactivate" action instead

    # States
    - Loading state shows Skeleton component in table area
    - Error state shows ErrorBanner with retry button
    - Optimistic updates for status changes

  scope:
    includes:
      - User list table with pagination, sorting, filtering
      - Create user modal form
      - Edit user modal form
      - Deactivate/reactivate user action
      - Input validation (client-side)
    excludes:
      - User profile detail page (separate spec)
      - Password reset for users
      - Bulk operations (multi-select, bulk delete)
      - CSV import/export of users
      - User activity log / audit trail
      - Email notifications on user changes
```

Compare this to the vibe prompt's "add, edit, and delete." The acceptance criteria make every behavior explicit and testable. The scope section prevents the AI from building features like CSV export or bulk delete that a "user management page" might reasonably include.

**Step 4: Constraints**

```yaml
constraints:
  # Technical
  - MUST use existing DataTable component for the table
  - MUST use existing Modal component for create/edit forms
  - MUST use existing FormField component for form inputs
  - MUST use React Query for all API interactions
  - MUST NOT introduce new dependencies
  - MUST use Zod for form validation schema

  # Security
  - MUST require "admin" role to access this page (redirect others)
  - MUST NOT display user passwords or password hashes
  - MUST NOT allow an admin to deactivate their own account
  - MUST NOT allow changing a user's email (immutable after creation)
  - MUST sanitize all user-provided text before rendering

  # Performance
  - MUST NOT fetch all users at once (use pagination)
  - MUST debounce filter changes at 300ms
  - MUST cancel in-flight requests when filters change
  - Table MUST use virtualization if more than 100 visible rows

  # Accessibility
  - Table MUST have proper ARIA table roles
  - Sort buttons MUST announce sort direction to screen readers
  - Modal focus MUST be trapped when open
  - Form errors MUST be associated with inputs via aria-describedby
  - Confirmation dialog MUST be announced to screen readers
```

**Step 5: Testing**

```yaml
testing:
  unit:
    # List
    - Table renders with correct columns
    - Pagination navigates between pages
    - Sorting toggles on column header click
    - Filters update the query parameters
    - Empty state renders when no users match
    - Loading state renders Skeleton component
    - Error state renders ErrorBanner with retry

    # Create
    - Create modal opens on button click
    - Form validates required fields
    - Form validates email format
    - Form validates name minimum length
    - Successful creation closes modal
    - API error shows error message

    # Edit
    - Edit modal opens with pre-filled data
    - Email field is read-only
    - Changes are submitted on save
    - Successful save closes modal

    # Deactivate
    - Confirmation dialog appears on deactivate click
    - Typing "DEACTIVATE" enables confirm button
    - Confirmation calls DELETE endpoint
    - Status updates in table after deactivation

  integration:
    - Full create → appear in list cycle
    - Full edit → see changes in list cycle
    - Deactivate → status change in list cycle
    - Filter and sort persist across pagination

  accessibility:
    - Page is navigable by keyboard alone
    - Screen reader announces table content correctly
    - Modal trap focus works correctly
    - Form validation errors are announced
```

### The Complete Transformation

Let us put the original vibe prompt next to the spec and appreciate the difference:

**Vibe prompt:** 32 words, ~200 implicit decisions, ephemeral, no validation criteria.

**Spec:** ~150 lines, every significant decision explicit, version-controlled, testable, reviewable.

The spec took perhaps 20 minutes to write. The vibe prompt took 15 seconds. But the spec will save hours of rework, re-prompting, and debugging. It will produce consistent code regardless of which AI model generates it. It will serve as documentation for future developers. And it will prevent the AI from making silent decisions about security, accessibility, scope, and behavior that might not match your intent.

> **Professor's Aside:** I want to be honest about the economics here. Twenty minutes to write a spec for a feature that might take a few hours to implement seems like a lot. But consider: without the spec, you might spend 30 minutes re-prompting, 20 minutes fixing the AI's security decisions, 15 minutes adjusting the scope, and 30 minutes figuring out why the generated code does not match your existing patterns. The spec is not additional time. It is *front-loaded* time that eliminates back-loaded pain.

---

## 4.3 Exercise 2: A UI Component Request

### The Vibe Prompt

```
Build me a date picker component. It should look modern and work well on mobile.
```

This is a deceptively simple request that hides enormous complexity. Date pickers are one of the most complex UI components in web development, touching on internationalization, time zones, accessibility, browser compatibility, and user experience.

### Gap Analysis

| What is Stated | What is Missing |
|---|---|
| "date picker component" | Single date? Range? Multiple? |
| "look modern" | Completely undefined aesthetic criterion |
| "work well on mobile" | Touch targets? Native input fallback? Responsive layout? |
| (Not mentioned) | Date format (ISO, US, European?) |
| (Not mentioned) | Min/max date constraints |
| (Not mentioned) | Disabled dates |
| (Not mentioned) | Time selection? Just date? |
| (Not mentioned) | Locale/internationalization |
| (Not mentioned) | Integration (form library? standalone?) |
| (Not mentioned) | Accessibility (keyboard, screen reader) |
| (Not mentioned) | Existing date libraries in the project |

"Look modern" is particularly problematic. It means nothing to an AI. The AI's concept of "modern" is whatever was most common in its training data, which is a statistical average of designs from 2020-2025. Your concept of "modern" is whatever your design team has decided, which might be very specific.

### Building the Spec

```yaml
kind: Component
metadata:
  name: DatePicker
  module: shared
  version: 1.0.0
  system_spec: system.spec.yaml@3.1.0

context:
  description: >
    The application has several forms that require date input
    (event scheduling, report filtering, user profile birthday).
    Currently, these use native HTML date inputs, which have
    inconsistent UX across browsers and do not support our design
    system. We need a custom date picker that integrates with
    our existing form library and design tokens.

  technical_context: >
    Form handling: React Hook Form v7 with Controller component.
    Date library: date-fns v3 is already installed and used for
    date formatting throughout the app.
    Design tokens: Tailwind CSS with custom theme colors defined
    in tailwind.config.ts (colors.primary, colors.surface, etc.).
    Localization: The app supports en-US and es-MX locales via
    the existing i18n system (react-intl).
    Existing components:
      - Popover component at /shared/components/Popover.tsx
      - IconButton component at /shared/components/IconButton.tsx
      - FormField component at /shared/components/FormField.tsx
        (wraps inputs with label, error message, description)

  assumptions:
    - Only date selection needed (no time picker for v1)
    - Only single date selection (no range picker for v1)
    - Calendar starts on Sunday for en-US, Monday for es-MX
    - Minimum supported date: 1900-01-01
    - Maximum supported date: 2100-12-31

objective:
  summary: >
    Create a reusable DatePicker component that integrates with
    React Hook Form and the existing design system, replacing
    native HTML date inputs across the application.

  acceptance_criteria:
    # Core Interaction
    - Text input field displays the selected date in locale-appropriate format
    - Clicking the input (or calendar icon) opens a calendar popover
    - Calendar shows a month view with selectable day cells
    - Clicking a day selects it and closes the popover
    - Month/year navigation via previous/next arrow buttons
    - Year can be jumped to via a year dropdown (range: 1900-2100)
    - Selected date is highlighted in the calendar
    - Today's date has a distinct visual indicator
    - Clicking outside the popover closes it

    # Text Input
    - User can type a date directly in the input field
    - Typed dates are parsed on blur using date-fns parse
    - Invalid date input shows validation error
    - Input placeholder shows expected format (e.g., "MM/DD/YYYY")

    # Integration
    - Works as a controlled component with React Hook Form Controller
    - Exposes value as ISO 8601 date string (YYYY-MM-DD)
    - Supports min and max date props
    - Supports disabled dates via a predicate function
    - Supports required, disabled, and read-only states
    - Triggers onChange and onBlur callbacks

    # Mobile
    - On screens < 640px, calendar renders as a bottom sheet (not popover)
    - Touch targets are minimum 44x44px
    - Swipe gestures for month navigation on touch devices
    - No hover-dependent interactions

    # Locale
    - Date format adapts to current locale (MM/DD/YYYY vs DD/MM/YYYY)
    - Day names and month names are localized
    - Calendar start day adapts to locale

  scope:
    includes:
      - DatePicker component with calendar popover
      - Text input with date parsing
      - Month/year navigation
      - React Hook Form integration
      - Locale support (en-US, es-MX)
      - Mobile-responsive layout
    excludes:
      - Time picker (separate component for v2)
      - Date range picker (separate component for v2)
      - Multi-date selection
      - Week number display
      - Holiday/event indicators on calendar
      - Custom day cell rendering

constraints:
  # Technical
  - MUST use date-fns for all date operations (no moment.js, no dayjs)
  - MUST use existing Popover component for desktop calendar display
  - MUST NOT introduce new npm dependencies
  - MUST work with React Hook Form Controller pattern
  - MUST expose a ref for form registration
  - Value MUST always be ISO 8601 format (YYYY-MM-DD) or null

  # Performance
  - MUST lazy-load the calendar popover (not in initial bundle)
  - Calendar render MUST NOT cause layout shift in the form
  - MUST NOT re-render parent form on internal state changes
  - MUST memoize calendar day cells to prevent unnecessary re-renders

  # Accessibility
  - MUST implement ARIA date picker pattern (role="dialog" for calendar)
  - Calendar MUST be navigable by keyboard (arrow keys for days)
  - Month navigation MUST be accessible via keyboard (Page Up/Page Down)
  - Selected date MUST be announced to screen readers
  - Disabled dates MUST have aria-disabled and not be focusable
  - MUST support Escape to close calendar and return focus to input
  - MUST NOT trap focus when opened as popover (trap only in bottom sheet)
  - Input MUST have associated label (via FormField wrapper)
  - Error messages MUST be linked via aria-describedby

  # UX
  - MUST NOT allow selecting dates outside min/max range
  - MUST visually distinguish disabled dates from selectable dates
  - MUST show current month on open (or month containing selected date)
  - Calendar MUST NOT flash or shift when navigating months
  - MUST provide clear visual feedback for hover, focus, and selected states

testing:
  unit:
    - Renders with no selected date (shows placeholder)
    - Renders with pre-selected date (shows formatted date)
    - Calendar opens on input click
    - Calendar opens on icon button click
    - Day selection updates the input value
    - Day selection closes the calendar
    - Month navigation shows previous/next month
    - Year dropdown changes the displayed year
    - Today's date has visual indicator
    - Min date prevents selecting earlier dates
    - Max date prevents selecting later dates
    - Disabled dates cannot be selected
    - Invalid typed input shows error
    - Valid typed input updates the value
    - ESC closes the calendar
    - Outside click closes the calendar

  integration:
    - Works with React Hook Form validation
    - Form submission includes the date value
    - Form reset clears the date value
    - Locale change updates date format and calendar

  accessibility:
    - Calendar is keyboard navigable (arrow keys)
    - Month navigation works with Page Up/Down
    - Selected date is announced by screen reader
    - Disabled dates are announced as disabled
    - Focus returns to input after calendar closes
    - Error messages are associated with input

  responsive:
    - Desktop: calendar renders as popover
    - Mobile (< 640px): calendar renders as bottom sheet
    - Touch targets meet 44x44px minimum
```

### What Changed From Vibe to Spec

The vibe prompt was 15 words. The spec is roughly 170 lines. Here is what the spec adds:

1. **Concrete design decisions** instead of "look modern": specific interaction patterns, visual states, and mobile behavior.

2. **Integration points** instead of a standalone component: React Hook Form, date-fns, existing Popover and FormField components.

3. **Data contract** instead of ambiguity: ISO 8601 values, locale-specific formatting, min/max constraints.

4. **Accessibility** instead of nothing: full keyboard navigation, screen reader support, ARIA patterns.

5. **Mobile strategy** instead of "work well on mobile": bottom sheet on small screens, minimum touch targets, swipe gestures.

6. **Explicit scope** instead of open-ended: single date only, no time, no range, no custom rendering.

> **Professor's Aside:** The date picker is my favorite example for teaching SDD because it is a component that *everyone* thinks is simple and *no one* gets right on the first try. The number of date picker implementations I have seen that are inaccessible, locale-unaware, or broken on mobile is staggering. A good spec forces you to confront the complexity upfront, before you are knee-deep in code.

---

## 4.4 Exercise 3: An API Endpoint Request

### The Vibe Prompt

```
Create an API endpoint for uploading files. Users should be able to
upload documents and images. Store them in S3.
```

This prompt is particularly dangerous because file upload is a feature with serious security implications. An AI generating a file upload endpoint from this prompt might produce something that works but is vulnerable to malicious file uploads, path traversal attacks, denial-of-service via large files, or server-side request forgery.

### Gap Analysis

| What is Stated | What is Missing |
|---|---|
| "uploading files" | Upload mechanism (multipart? presigned URL? chunked?) |
| "documents and images" | Specific file types? Size limits? |
| "Store them in S3" | Which bucket? What path structure? Lifecycle rules? |
| (Not mentioned) | Authentication and authorization |
| (Not mentioned) | File validation (type, size, content) |
| (Not mentioned) | Malware scanning |
| (Not mentioned) | Rate limiting |
| (Not mentioned) | Metadata storage (database record for each file?) |
| (Not mentioned) | Access control (who can view uploaded files?) |
| (Not mentioned) | Error handling (partial upload, network failure) |
| (Not mentioned) | Thumbnails or previews |
| (Not mentioned) | File naming strategy (original name? UUID?) |

### Building the Spec

```yaml
kind: Endpoint
metadata:
  name: FileUpload
  module: files
  version: 1.0.0
  system_spec: system.spec.yaml@3.1.0

context:
  description: >
    The application needs to allow authenticated users to upload
    documents and images as attachments to projects. Currently
    there is no file upload capability. Files will be stored in
    AWS S3 and metadata will be stored in the application database.

  technical_context: >
    Backend: Express.js with TypeScript.
    Database: PostgreSQL with Prisma ORM.
    Storage: AWS S3, bucket "acme-app-uploads-prod".
    Auth: JWT middleware (req.user.id, req.user.role available).
    File processing: sharp library is available for image processing.
    Virus scanning: ClamAV service running at clamav:3310.

    Existing database tables:
      - projects (id, name, ownerId, ...)
      - project_members (projectId, userId, role)

    New table needed:
      - files (id, projectId, uploadedBy, originalName, storagePath,
              mimeType, sizeBytes, status, createdAt, deletedAt)

    S3 path convention: {projectId}/{fileId}/{filename}
    Max project storage: 5 GB per project
    Presigned URL strategy: Upload directly from client to S3
    using presigned POST URL (server never handles file bytes).

  assumptions:
    - ClamAV service is available and healthy
    - S3 bucket exists with proper IAM permissions
    - Client will handle the actual S3 upload using the presigned URL

objective:
  summary: >
    Create a two-step file upload flow: (1) server generates a
    presigned S3 URL and creates a pending file record, (2) client
    uploads directly to S3 and confirms completion.

  endpoints:
    - name: Request Upload URL
      method: POST
      path: /api/projects/:projectId/files/upload-url
      description: >
        Validates the upload request and returns a presigned S3 URL
        for direct client-to-S3 upload.
      request:
        params:
          - name: projectId
            type: string
            required: true
            description: Project to attach the file to
        body:
          type: object
          properties:
            fileName:
              type: string
              required: true
              description: Original file name
              validation: "Max 255 chars, no path separators"
            mimeType:
              type: string
              required: true
              description: MIME type of the file
              validation: "Must be in allowed types list"
            sizeBytes:
              type: number
              required: true
              description: File size in bytes
              validation: "Must be > 0 and <= 50MB"
      response:
        success:
          status: 201
          body:
            fileId: string
            uploadUrl: string (presigned S3 POST URL)
            fields: object (form fields for S3 POST)
            expiresAt: string (ISO 8601, 15 min from now)
        errors:
          - status: 400
            conditions:
              - Invalid MIME type
              - File too large
              - File name invalid
          - status: 401
            condition: Not authenticated
          - status: 403
            conditions:
              - Not a member of the project
              - Project storage quota exceeded
          - status: 404
            condition: Project not found
          - status: 429
            condition: Upload rate limit exceeded

    - name: Confirm Upload
      method: POST
      path: /api/projects/:projectId/files/:fileId/confirm
      description: >
        Client calls this after successful S3 upload. Server
        verifies the file exists in S3, triggers virus scan,
        and marks the file as active.
      request:
        params:
          - name: projectId
            type: string
            required: true
          - name: fileId
            type: string
            required: true
      response:
        success:
          status: 200
          body:
            file:
              id: string
              name: string
              mimeType: string
              sizeBytes: number
              url: string (CDN URL for access)
              status: "active"
              createdAt: string
        errors:
          - status: 400
            condition: File not found in S3
          - status: 401
            condition: Not authenticated
          - status: 403
            condition: Not the user who requested the upload
          - status: 404
            condition: File record not found
          - status: 409
            condition: File already confirmed

  acceptance_criteria:
    # Upload URL Generation
    - Validates user is authenticated and is a project member
    - Validates file name (no path traversal characters)
    - Validates MIME type against allowed list
    - Validates file size against 50MB maximum
    - Checks project storage quota before generating URL
    - Creates a "pending" file record in database
    - Generates presigned S3 POST URL with 15-minute expiration
    - S3 path follows convention: {projectId}/{fileId}/{sanitized-name}

    # Upload Confirmation
    - Verifies file exists at expected S3 path
    - Verifies file size in S3 matches declared size (within 1%)
    - Triggers asynchronous virus scan via ClamAV
    - Updates file status from "pending" to "active"
    - Returns CDN URL for file access

    # Background Processing
    - Pending files not confirmed within 1 hour are cleaned up (cron job)
    - Files flagged by ClamAV are quarantined (status: "quarantined")
    - Quarantined files are deleted from S3 within 24 hours

    # Rate Limiting
    - Maximum 10 upload URL requests per user per minute
    - Maximum 100 uploads per project per day

  scope:
    includes:
      - Presigned URL generation endpoint
      - Upload confirmation endpoint
      - File metadata storage in database
      - MIME type validation
      - File size validation
      - Project storage quota check
      - Virus scan trigger
      - Pending file cleanup logic
    excludes:
      - File download/access endpoint (separate spec)
      - File deletion endpoint (separate spec)
      - Image thumbnail generation (separate spec)
      - File versioning
      - Bulk upload
      - Resumable/chunked uploads
      - File sharing outside the project

constraints:
  # Security (CRITICAL)
  - MUST validate MIME type against allowlist, not blocklist
  - Allowed MIME types: image/jpeg, image/png, image/gif, image/webp,
    application/pdf, application/msword,
    application/vnd.openxmlformats-officedocument.wordprocessingml.document,
    text/plain, text/csv
  - MUST NOT trust client-declared MIME type alone (verify via file magic bytes after upload)
  - MUST sanitize file names (remove path separators, null bytes, special characters)
  - MUST NOT include user input in S3 key without sanitization
  - MUST use a UUID for the file ID (not sequential, not guessable)
  - Presigned URL MUST expire in 15 minutes maximum
  - MUST NOT return the raw S3 URL (return CDN URL only)
  - MUST verify that the confirming user is the same user who requested the URL
  - MUST NOT allow overwriting existing files (unique path per upload)
  - MUST run ClamAV scan on every uploaded file without exception
  - Quarantined files MUST NOT be accessible via CDN

  # Performance
  - Server MUST NOT receive or buffer file bytes (presigned URL only)
  - Database queries MUST use indexes on projectId and uploadedBy
  - Presigned URL generation MUST complete in < 200ms
  - Storage quota check MUST use a cached/aggregated value (not SUM query)

  # Data Integrity
  - File record MUST be created before presigned URL is returned
  - MUST use database transaction for file record creation
  - Pending cleanup job MUST NOT delete files that are being confirmed
  - MUST log all file operations for audit trail

  # Error Handling
  - MUST return specific, actionable error messages
  - MUST NOT expose S3 bucket name or internal paths in error messages
  - MUST NOT expose ClamAV details in error messages
  - Failed virus scans MUST NOT silently pass (fail closed)

testing:
  unit:
    # Upload URL
    - Returns 401 for unauthenticated requests
    - Returns 403 for non-project-members
    - Returns 400 for disallowed MIME types
    - Returns 400 for files exceeding 50MB
    - Returns 400 for file names with path traversal characters
    - Returns 403 when project storage quota is exceeded
    - Returns 429 when rate limit is exceeded
    - Returns 201 with valid presigned URL for valid requests
    - File record is created with "pending" status
    - S3 path follows naming convention
    - Presigned URL expires in 15 minutes

    # Upload Confirmation
    - Returns 404 for non-existent file records
    - Returns 403 if confirming user is not the uploader
    - Returns 400 if file not found in S3
    - Returns 409 if file already confirmed
    - Returns 200 and updates status to "active"
    - Triggers ClamAV scan
    - Returns CDN URL (not S3 URL)

    # File Name Sanitization
    - Removes path separators (/, \\)
    - Removes null bytes
    - Truncates to 255 characters
    - Preserves file extension
    - Handles unicode characters correctly

  integration:
    - Full upload flow: request URL → upload to S3 → confirm
    - Quota enforcement across multiple uploads
    - Rate limiting across multiple requests
    - Pending file cleanup after expiration

  security:
    - Path traversal in file name does not affect S3 key
    - Disallowed MIME type is rejected even if extension is allowed
    - Expired presigned URL is rejected by S3
    - Quarantined file is not accessible via CDN
    - File ID is not sequential or predictable
```

### Why This Spec Is Critical

The file upload vibe prompt was 22 words. The spec is roughly 230 lines. But look at what the spec captures that the vibe prompt does not:

1. **A presigned URL architecture** that keeps file bytes off the server entirely. The vibe prompt would likely produce a multipart upload handler that buffers files in server memory — a performance and security liability.

2. **Fifteen security constraints** that prevent real-world attack vectors: path traversal, MIME type spoofing, file overwriting, rate limiting, virus scanning.

3. **A two-step flow** (request URL, confirm upload) that handles the presigned URL pattern correctly with proper state management.

4. **Error handling for every failure mode**, with specific error codes and conditions.

5. **Background processing** for pending file cleanup and virus scanning.

Without this spec, an AI would almost certainly generate a simpler, less secure implementation. It would probably:

- Accept the file directly (multipart upload to the server)
- Store it in S3 without virus scanning
- Use the original filename without sanitization
- Not check storage quotas
- Not implement rate limiting
- Return the raw S3 URL
- Not handle the confirmation step
- Not clean up abandoned uploads

Every one of these omissions is a real-world vulnerability or operational issue. The spec prevents them all.

> **Professor's Aside:** File upload is one of the features where specs matter most, because the security surface area is enormous and the consequences of getting it wrong are severe. If a student takes only one thing from this chapter, I hope it is this: the more security-sensitive the feature, the more detailed the constraints section of your spec must be. The constraints section is your security posture for that feature.

---

## 4.5 Common Anti-Patterns in Prompting (and How Specs Fix Them)

Now that we have worked through three exercises, let us catalog the anti-patterns we see repeatedly in vibe prompts and map each one to the spec element that fixes it.

### Anti-Pattern 1: The Magic Word

```
Make it look good.
Make it modern.
Make it fast.
Make it secure.
```

These are words that feel meaningful but communicate nothing concrete. "Good," "modern," "fast," and "secure" are relative terms that mean different things to different people and different AI models.

**Spec Fix:** Replace magic words with measurable criteria.

```yaml
# Instead of "make it fast":
constraints:
  - Page load MUST complete in under 2 seconds on 3G connection
  - API response time MUST be under 200ms at p95
  - Component MUST NOT cause layout shift (CLS < 0.1)

# Instead of "make it secure":
constraints:
  - MUST sanitize all user input with DOMPurify
  - MUST use parameterized queries (no string concatenation in SQL)
  - MUST validate CSRF token on all mutating requests
  - MUST NOT log sensitive data (passwords, tokens, PII)
```

### Anti-Pattern 2: The Implied Stack

```
Build me a login page.
```

This tells the AI nothing about the technology stack. It will choose one based on its training data distribution, which might be React, Vue, Angular, Svelte, vanilla JS, or something else entirely.

**Spec Fix:** Explicit context section with technology stack.

```yaml
context:
  technical_context: >
    React 19, TypeScript 5.4, Tailwind CSS 4.0,
    React Hook Form v7, React Router v7.
    Auth API: POST /api/auth/login
```

### Anti-Pattern 3: The Absent Boundary

```
Add a notification system.
```

No scope boundary means the AI decides what "a notification system" includes. As we saw in Chapter 1, it might generate email, SMS, push, in-app, analytics, preferences, templates, and scheduling.

**Spec Fix:** Explicit scope with includes AND excludes.

```yaml
scope:
  includes:
    - In-app toast notification component
    - Integration with background task completion events
  excludes:
    - Email notifications
    - Push notifications
    - SMS notifications
    - Notification preferences
    - Notification history
```

### Anti-Pattern 4: The Missing Negative

```
Build a comment system for blog posts.
```

This says what to build but not what to avoid. The AI might generate comments with no moderation, no spam protection, no rate limiting, no XSS prevention, no length limits, and no nested reply limits.

**Spec Fix:** Constraints section with MUST NOT rules.

```yaml
constraints:
  - MUST NOT render user-generated HTML (sanitize all content)
  - MUST NOT allow more than 3 levels of nesting for replies
  - MUST NOT allow comments longer than 5000 characters
  - MUST NOT allow more than 5 comments per user per minute
  - MUST NOT display comments flagged as spam
  - MUST NOT allow editing comments older than 15 minutes
```

### Anti-Pattern 5: The Context Vacuum

```
Add a "Mark as Complete" button to the task card.
```

This assumes the AI knows what a "task card" is in your application, what "complete" means in your data model, what the existing button styles look like, what state management the task card uses, and what API endpoint handles task completion.

**Spec Fix:** Context section with technical details.

```yaml
context:
  technical_context: >
    TaskCard component at /features/tasks/components/TaskCard.tsx.
    Task type: { id: string, title: string, status: "todo" | "in_progress" | "done" }
    Update API: PUT /api/tasks/:id with { status: "done" }
    Existing button style: Use the existing SecondaryButton component.
    State: Task list is managed by React Query, key ["tasks", projectId].
    Optimistic updates are used for all task mutations.
```

### Anti-Pattern 6: The Testless Request

```
Build the feature I described and make sure it works.
```

"Make sure it works" is not a test strategy. It does not define what "works" means. It does not specify edge cases. It does not describe failure modes.

**Spec Fix:** Testing section with specific test cases.

```yaml
testing:
  unit:
    - Button renders in "todo" and "in_progress" states
    - Button does NOT render in "done" state
    - Clicking button calls PUT /api/tasks/:id with { status: "done" }
    - Optimistic update shows "done" state immediately
    - Failed API call reverts to previous state
    - Button is disabled while API call is in flight
```

### Anti-Pattern 7: The Conversational Debug Loop

This is not a single prompt but a pattern of interaction:

```
User: Build me a user profile page.
AI: [generates code]
User: No, I meant with an avatar.
AI: [regenerates with avatar]
User: The avatar should be editable.
AI: [adds upload functionality]
User: It should use our S3 bucket.
AI: [adds S3 integration, but now the form validation is gone]
User: Wait, where did the validation go?
AI: [adds it back but breaks the avatar upload]
```

Each iteration fixes one thing and potentially breaks another. The developer is building the spec incrementally through a chat conversation, but the AI does not have a persistent, complete picture of the requirements.

**Spec Fix:** Write the complete spec upfront. The spec captures all requirements in one place. The AI generates from the complete picture, not from a evolving conversation.

```yaml
# One document captures everything:
# - Avatar upload with S3 integration
# - Form validation
# - Profile fields
# - Accessibility
# - Constraints
# No iterative degradation. No lost context.
```

---

## 4.6 The Spec Quality Checklist

Here is a practical checklist you can use to evaluate any spec before feeding it to an AI. Print this out. Tape it to your monitor. Use it until it becomes second nature.

### Context Checklist

```
[ ] Technology stack is specified (language, framework, key libraries)
[ ] Relevant library versions are specified
[ ] Existing patterns and conventions are described
[ ] Integration points are identified (APIs, components, services)
[ ] Data models/types are defined with field names and types
[ ] Related features or specs are referenced
[ ] Current state of the system is described (what exists now)
```

### Objective Checklist

```
[ ] Summary is a clear, one-sentence description of the delta
[ ] Summary starts with an action verb (Create, Add, Modify, Remove)
[ ] Every acceptance criterion is testable (binary yes/no)
[ ] Every acceptance criterion is unambiguous (one interpretation)
[ ] Acceptance criteria cover the happy path
[ ] Acceptance criteria cover error/failure states
[ ] Acceptance criteria cover edge cases
[ ] Scope includes section lists what IS in scope
[ ] Scope excludes section lists what is NOT in scope
[ ] No "magic words" (good, modern, fast, clean, nice)
```

### Constraints Checklist

```
[ ] Technical constraints specify what libraries/patterns to use
[ ] Technical constraints specify what NOT to use
[ ] Security constraints address input validation
[ ] Security constraints address data exposure
[ ] Security constraints address authentication/authorization
[ ] Performance constraints specify measurable limits
[ ] Accessibility constraints specify keyboard support
[ ] Accessibility constraints specify screen reader support
[ ] All constraints use MUST or MUST NOT vocabulary
[ ] The "malicious compliance" test has been applied
```

### Testing Checklist

```
[ ] Every acceptance criterion has at least one test
[ ] Every MUST constraint has at least one test
[ ] Every MUST NOT constraint has at least one test
[ ] Error/failure paths have tests
[ ] Edge cases have tests
[ ] Accessibility requirements have tests
[ ] Tests are organized by type (unit, integration, accessibility)
```

### Meta Checklist

```
[ ] Spec has a version number
[ ] Spec references the system spec
[ ] Spec includes metadata (name, module, owner)
[ ] Spec is shorter than the expected code
[ ] Spec focuses on WHAT and WHY, not HOW
[ ] A developer unfamiliar with the codebase could implement from this spec
[ ] The spec has been reviewed by at least one other person
```

---

## 4.7 The Spec Writing Process

Let me formalize the process of writing a spec into a repeatable workflow:

### Phase 1: Gather (5-10 minutes)

Before writing a single line of spec, gather the information you need:

1. **Read the feature request.** Understand what is being asked for.
2. **Explore the codebase.** Understand the current state — what exists, what patterns are used, what components/APIs are available.
3. **Identify the data model.** What types, schemas, and database tables are involved?
4. **Identify the integration points.** What APIs, components, and services will this feature interact with?
5. **Identify the stakeholders.** Who cares about this feature? PM, security, design, QA?

### Phase 2: Draft (10-15 minutes)

Write the first draft of the spec:

1. **Start with context.** This is where most of the gathered information goes.
2. **Write the objective.** Focus on the acceptance criteria — they are the most important part.
3. **Define the scope.** Be explicit about what is excluded.
4. **Write the constraints.** Apply the "malicious compliance" test.
5. **Define the tests.** One test per acceptance criterion, one per constraint.

### Phase 3: Review (5-10 minutes)

Review the spec before using it:

1. **Read it as an outsider.** Could someone unfamiliar with the project implement this?
2. **Apply the checklist.** Go through the quality checklist item by item.
3. **Ask a colleague.** Have someone else read the acceptance criteria and tell you what they think the feature does. If their understanding does not match yours, the spec is ambiguous.
4. **Check for magic words.** Search for vague terms and replace them with measurable criteria.

### Phase 4: Generate and Validate (varies)

Use the spec to generate code, then validate:

1. **Feed the spec to the AI.** Include both the micro-spec and the system spec.
2. **Walk through the acceptance criteria.** For each criterion, verify the generated code addresses it.
3. **Walk through the constraints.** For each constraint, verify the code respects it.
4. **Run the generated tests.** Do they pass? Do they actually test what they claim to test?
5. **If anything fails, refine the spec.** Do not patch the code. Fix the spec and regenerate.

> **Professor's Aside:** That last point is crucial and counterintuitive. When the generated code is wrong, the instinct is to fix the code. Resist this instinct. Instead, ask: "What was unclear or missing in the spec that led the AI to generate incorrect code?" Fix the spec, regenerate, and verify. This creates a virtuous cycle where your specs get better over time, and each generation is more accurate than the last. If you fix the code without fixing the spec, you will hit the same problem again next time you generate from that spec.

---

## 4.8 From Spec Back to Prompt: The Delivery Format

One question students often ask at this point: "I have a YAML spec. How do I actually give it to the AI?"

There are several approaches:

### Approach 1: Direct Spec Inclusion

Simply include the spec as-is in your prompt to the AI:

```
Generate the implementation for the following spec:

---
kind: Component
metadata:
  name: DatePicker
  module: shared
...
```

Most modern AI models (Claude, GPT-4, Gemini) handle YAML natively and understand the structure. The structured format actually helps the model organize its output.

### Approach 2: Spec + System Context

Include both the micro-spec and the system spec:

```
System context:
[contents of system.spec.yaml]

Generate the implementation for the following feature spec:
[contents of feature.spec.yaml]
```

This gives the AI the complete picture: global conventions plus feature-specific requirements.

### Approach 3: Spec + Existing Code

When modifying an existing feature, include the spec AND the current code:

```
Here is the current implementation:
[contents of existing Component.tsx]

Here is the updated spec:
[contents of updated spec.yaml]

Update the implementation to match the new spec. The changes
from the previous version are:
[diff of spec changes]
```

This approach is powerful for incremental changes because the AI can see both where the code currently is and where it needs to go.

### Approach 4: Automated Pipeline

In mature SDD workflows, spec-to-code generation is automated. A CI/CD pipeline detects spec changes, feeds them to the AI via API, generates code, runs tests, and creates a PR:

```typescript
// Simplified SDD pipeline (conceptual)

import { readSpec, readSystemSpec } from "./spec-reader";
import { generateCode } from "./ai-client";
import { runTests } from "./test-runner";
import { createPullRequest } from "./git-client";

async function processSpecChange(specPath: string) {
  const spec = await readSpec(specPath);
  const systemSpec = await readSystemSpec(spec.metadata.system_spec);

  const generatedCode = await generateCode({
    spec,
    systemSpec,
    model: "claude-opus-4-6",
    temperature: 0,
  });

  const testResults = await runTests(generatedCode.testFiles);

  if (testResults.allPassed) {
    await createPullRequest({
      title: `feat(${spec.metadata.module}): implement ${spec.metadata.name} v${spec.metadata.version}`,
      files: generatedCode.allFiles,
      description: `Generated from ${specPath} v${spec.metadata.version}`,
    });
  } else {
    // Log failures for human review
    console.error("Generated code did not pass spec-derived tests.");
    console.error(testResults.failures);
  }
}
```

This is the endgame of SDD: the human writes the spec, the machine writes the code, and the tests (also derived from the spec) verify the result. The human's job shifts from *writing code* to *writing intent* — and verifying that the intent is correctly executed.

---

## 4.9 Homework: Convert Your Own Prompt

Here is your homework assignment. This is not optional. You will get more out of doing this one exercise than from reading all four chapters.

### The Assignment

1. **Find a past prompt.** Go through your chat history with whatever AI coding assistant you use. Find a prompt where the result was not quite right — where you had to re-prompt, fix the code manually, or got something you did not expect.

2. **Analyze the gaps.** Using the gap analysis technique from this chapter, identify what was missing from your prompt. What decisions did the AI make that you did not specify? What assumptions did it make?

3. **Write a micro-spec.** Transform your prompt into a full micro-spec using the three-pillar structure (Context, Objective, Constraints). Use the quality checklist to evaluate your spec.

4. **Regenerate.** Feed your micro-spec to the same AI model you originally used. Compare the output to what you got from the original vibe prompt.

5. **Reflect.** Write a brief reflection (3-5 sentences) on:
   - What decisions did the spec make explicit that the prompt left implicit?
   - How did the spec-generated output compare to the prompt-generated output?
   - What did you learn about your own assumptions by writing the spec?

### Evaluation Criteria

Your spec will be evaluated on:

- **Completeness:** Does it cover all three pillars with sufficient detail?
- **Clarity:** Is every acceptance criterion testable and unambiguous?
- **Appropriate detail:** Does it specify what and why without specifying how?
- **Constraints:** Does it include meaningful constraints (not just technical, but security and accessibility)?
- **Scope:** Does it have explicit includes AND excludes?

---

## 4.10 What Comes Next

You have now completed Module 1: Foundations. You understand:

1. **Why** specs matter (Chapter 1: natural language is too ambiguous for production software)
2. **What** the spec's role is (Chapter 2: the single source of truth that governs code)
3. **How** specs are structured (Chapter 3: Context, Objective, Constraints)
4. **How** to write specs (Chapter 4: practical transformation from vibe prompts)

In Module 2, we will build on these foundations with the **Specification Language** — the formal syntax and semantics of SDD specs. You will learn about spec types (Component, Feature, Endpoint, Migration, Integration), spec composition (how specs reference and build on each other), and spec validation (how to mechanically verify that a spec is well-formed).

But before we get there, let me leave you with a thought.

The shift from vibe coding to spec-driven development is not just a workflow change. It is a *mindset* change. It is the shift from "I am writing code with AI help" to "I am writing specifications that happen to be implemented by AI." The human's job is to express intent clearly. The AI's job is to translate that intent into working code. And the spec is the contract between them.

When you write a spec, you are not doing busywork. You are not adding overhead. You are doing the *hard part* of software development — deciding what to build and what not to build, defining the boundaries of the system, encoding security and accessibility requirements, and creating a testable definition of success.

The code is just the easy part. Anyone (or anything) can write code. Defining what the code should do — with precision, completeness, and clarity — is the craft.

Welcome to Spec-Driven Development. The hard, satisfying, valuable work starts here.

---

## Chapter Summary

| Concept | Key Takeaway |
|---|---|
| Vibe Prompt Anatomy | Vibe prompts are conversational, implicit, vague, unconstrained, untestable, and ephemeral. |
| Gap Analysis | The first step in spec writing is identifying what the vibe prompt leaves unsaid. |
| CRUD Spec | Even "simple" CRUD features hide dozens of decisions about data models, validation, authorization, states, and scope. |
| UI Component Spec | Complex components (date pickers, etc.) require detailed specs for interaction, accessibility, internationalization, and mobile. |
| API Endpoint Spec | Security-sensitive features need extensive constraints to prevent vulnerabilities the AI would otherwise ignore. |
| Anti-Patterns | Magic words, implied stacks, absent boundaries, missing negatives, context vacuums, testless requests, and conversational debug loops. |
| Quality Checklist | A systematic checklist for evaluating spec completeness, clarity, constraints, testing, and meta information. |
| Writing Process | Gather, Draft, Review, Generate-and-Validate — a repeatable four-phase workflow. |

---

## Discussion Questions

1. Look at the three exercise specs in this chapter. Which one do you think would produce the most different output between the vibe prompt version and the spec version? Why?

2. The chapter argues that "fixing the spec is better than fixing the code." Can you think of a situation where this is not true — where directly fixing the code is the better approach?

3. Consider the file upload spec (Exercise 3). There are fifteen security constraints. If you had to pick the three most critical ones, which would they be and why?

4. The spec writing process is described as a 20-30 minute investment. Do you think this time investment is justified for a feature that takes 2 hours to implement? What about a feature that takes 20 minutes?

5. The homework asks you to convert one of your own past prompts into a spec. Before you do it, predict: how many implicit decisions do you think your original prompt contained?

---

## Module 1 Recap

Congratulations. You have completed the first module of Mastering Spec-Driven Development. Here is a final summary of what you have learned:

| Chapter | Core Lesson |
|---|---|
| Chapter 1: From Prose to Protocol | Natural language is too ambiguous for AI-assisted development. The industry has evolved from vibe coding to structured prompting to spec-driven development. More capable AI requires more structure, not less. |
| Chapter 2: The Single Source of Truth | The spec is the ultimate authority over the code. Spec = what + why. Code = how. When they diverge, the spec wins. |
| Chapter 3: The Anatomy of a Micro-Spec | Every spec has three pillars: Context (what exists), Objective (what should change), and Constraints (what must not happen). |
| Chapter 4: From Vibe to Spec | Transform vibe prompts into specs by analyzing gaps, writing structured context/objectives/constraints, and validating with a quality checklist. |

You now have the conceptual foundation for SDD. In Module 2, we will turn these concepts into a rigorous practice with formal spec languages, composition patterns, and automated validation.

See you in the next module.

---

*End of Module 01: Foundations — The "Contract" Mindset*

*Next: Module 02 — The Specification Language*

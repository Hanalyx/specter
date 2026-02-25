# Chapter 2: Evolutionary Specs

## MODULE 04 — Advanced Orchestration & Agents (Advanced Level)

---

### Lecture Preamble

*Let me tell you a story that every experienced developer has lived through. You ship version 1.0 of a feature. It works. Users are happy. Then the product manager walks over and says, "Can we just add one more thing?" And then another thing. And then a client wants a slightly different behavior. And then regulations change. And then the mobile team needs a different response format. And before you know it, your beautiful clean feature is an archaeological dig site -- layers of changes stacked on top of each other, each one making sense at the time, and together forming something nobody fully understands.*

*This is the problem of evolution. Software is never done. Specifications are never done. The question is not whether your specs will change -- they will, I promise you that -- but how you manage that change without losing your mind or your system's integrity.*

*In the last chapter, we learned how multiple agents coordinate around a spec. Today, we learn what happens when that spec needs to change. And I do not mean "throw it away and write a new one." I mean evolve it -- carefully, traceeably, and safely -- so that every agent, every developer, and every downstream system can understand what changed, why it changed, and what they need to do about it.*

*This is one of those topics where the AI development world has learned directly from decades of hard-won lessons in API design. Google's API versioning strategy, Stripe's legendary API evolution, Anthropic's model versioning across Claude releases -- these are all relevant case studies. Let us learn from them.*

---

## 2.1 The Fundamental Problem: Change Is Inevitable

Every spec you write will change. This is not a bug in the process. It is a feature of building real software. Requirements evolve because:

- **Users discover new needs** once they start using the software
- **Business priorities shift** as markets change
- **Technical constraints change** as infrastructure evolves
- **Regulations update** (GDPR, SOC2, accessibility standards)
- **Integration partners** change their APIs
- **Performance requirements** shift as data volumes grow
- **Security threats** emerge and demand new protections

The question is: when a spec changes, what happens to the code built from the previous version of that spec? What happens to other specs that depend on it? What happens to the agents that consumed it?

> **Professor's aside:** I have seen teams handle spec changes in exactly two ways. The bad way: edit the spec in place, tell the Builder to "figure out what changed," and hope for the best. The good way: treat spec changes with the same rigor you treat database migrations or API version bumps. We are going to learn the good way.

---

## 2.2 Version Control for Specs: Semantic Versioning Applied to Specifications

The first discipline of evolutionary specs is versioning. Every spec gets a version number, and that version number follows **semantic versioning** (semver), adapted for specifications.

### Semver for Specs

In traditional software semver, versions follow the pattern `MAJOR.MINOR.PATCH`:

- **MAJOR** -- Breaking changes that require consumer updates
- **MINOR** -- Additive changes that are backward compatible
- **PATCH** -- Fixes that do not change the spec's interface

For specifications, we adapt this:

| Version Bump | Meaning | Example | Consumer Impact |
|-------------|---------|---------|-----------------|
| **MAJOR** (1.0 -> 2.0) | Breaking change: existing implementations must be updated | Removing a required input, changing output format, altering error behavior | All consumers must migrate |
| **MINOR** (1.0 -> 1.1) | Additive change: new capabilities without breaking existing ones | Adding a new optional input, adding a new error case, adding a new output field | Consumers can upgrade at their pace |
| **PATCH** (1.0.0 -> 1.0.1) | Clarification or correction that does not change behavior | Fixing a typo in a description, clarifying an ambiguous constraint, adding an example | No consumer impact |

### Spec Versioning in Practice

```yaml
# Version 1.0.0 -- Initial spec
spec_version: "1.0.0"
feature: "notification-preferences"
status: "approved"
created: "2026-01-15"
last_modified: "2026-01-15"

changelog:
  - version: "1.0.0"
    date: "2026-01-15"
    author: "architect-agent"
    type: "initial"
    description: "Initial specification for notification preferences"

purpose: |
  Allow users to manage their notification preferences including
  email and push notification toggles, quiet hours, and frequency.

inputs:
  - name: "user_id"
    type: "string"
    required: true
    description: "The authenticated user's ID"
  - name: "preferences"
    type: "NotificationPreferences"
    required: true
    description: "The preference settings to save"

outputs:
  - name: "saved_preferences"
    type: "NotificationPreferences"
    description: "The saved preferences, confirming what was persisted"

constraints:
  - "Quiet hours are specified as a start time and end time in the user's local timezone"
  - "Frequency options: immediate, hourly, daily"
  - "All preferences are user-scoped (no global overrides)"

error_cases:
  - condition: "Invalid time format for quiet hours"
    behavior: "Return 400 with validation error details"
  - condition: "User not authenticated"
    behavior: "Return 401"
```

```yaml
# Version 1.1.0 -- Additive change (MINOR bump)
spec_version: "1.1.0"
feature: "notification-preferences"
status: "approved"
created: "2026-01-15"
last_modified: "2026-02-01"

changelog:
  - version: "1.1.0"
    date: "2026-02-01"
    author: "architect-agent"
    type: "minor"
    description: "Added SMS notification channel and category-level preferences"
    changes:
      - type: "addition"
        section: "inputs"
        detail: "Added optional 'sms_enabled' field to NotificationPreferences"
      - type: "addition"
        section: "inputs"
        detail: "Added optional 'category_overrides' field for per-category settings"
      - type: "addition"
        section: "error_cases"
        detail: "Added error case for invalid phone number when SMS is enabled"
  - version: "1.0.0"
    date: "2026-01-15"
    author: "architect-agent"
    type: "initial"
    description: "Initial specification for notification preferences"

# ... (all v1.0.0 content preserved, new content added)

inputs:
  - name: "user_id"
    type: "string"
    required: true
    description: "The authenticated user's ID"
  - name: "preferences"
    type: "NotificationPreferences"
    required: true
    description: "The preference settings to save"
    properties:
      email_enabled:
        type: "boolean"
        required: true     # Required since v1.0.0
      push_enabled:
        type: "boolean"
        required: true     # Required since v1.0.0
      sms_enabled:
        type: "boolean"
        required: false    # NEW in v1.1.0 -- optional for backward compat
        since: "1.1.0"
        default: false
      quiet_hours:
        type: "TimeRange"
        required: false
      frequency:
        type: "enum"
        values: ["immediate", "hourly", "daily"]
        required: true
      category_overrides:
        type: "Map<Category, ChannelPreferences>"
        required: false    # NEW in v1.1.0 -- optional for backward compat
        since: "1.1.0"
        default: null
        description: "Per-category notification settings that override global defaults"
```

Notice the key design decisions:
- New fields are **optional** with sensible defaults
- The `since` annotation marks when each field was introduced
- The changelog is cumulative -- every version's changes are preserved
- No existing fields changed their type, name, or required status

---

## 2.3 Additive Changes (Safe) vs. Breaking Changes (Dangerous)

This is one of the most important distinctions in spec evolution. Understanding it will save you from cascading failures across your system.

### Additive Changes (MINOR version bump)

Additive changes are safe because they do not invalidate existing implementations. An implementation built against v1.0 of the spec will continue to work correctly when the spec is bumped to v1.1.

**Safe additive changes include:**

```yaml
# These are all safe MINOR changes:

# 1. Adding a new OPTIONAL input
inputs_added:
  - name: "theme_preference"
    type: "string"
    required: false       # MUST be optional
    default: "system"     # MUST have a default
    since: "1.1.0"

# 2. Adding a new output field
outputs_added:
  - name: "last_synced_at"
    type: "ISO8601DateTime"
    since: "1.1.0"
    description: "Timestamp of last cross-device sync"

# 3. Adding a new error case
error_cases_added:
  - condition: "SMS enabled but no verified phone number"
    behavior: "Return 400 with message 'Please verify your phone number'"
    since: "1.1.0"

# 4. Adding a new acceptance criterion
acceptance_criteria_added:
  - criterion: "Given a user with SMS enabled, when they save preferences, their phone number is validated"
    since: "1.1.0"

# 5. Relaxing a constraint (making it less restrictive)
constraint_relaxed:
  before: "Frequency options: immediate, hourly, daily"
  after: "Frequency options: immediate, hourly, daily, weekly"
  # Adding options is safe -- existing options still work
```

### Breaking Changes (MAJOR version bump)

Breaking changes invalidate existing implementations. Code built against v1.x will not work correctly against v2.0.

**Dangerous breaking changes include:**

```yaml
# These are all BREAKING changes requiring a MAJOR bump:

# 1. Removing an existing field
field_removed:
  name: "push_enabled"
  was_in: "1.x"
  removed_in: "2.0.0"
  impact: "All code referencing push_enabled will fail"

# 2. Changing a field's type
type_changed:
  field: "frequency"
  was: "enum ['immediate', 'hourly', 'daily']"
  now: "object { type: string, interval_minutes: number }"
  impact: "All existing frequency values are invalid"

# 3. Making an optional field required
requirement_changed:
  field: "quiet_hours"
  was: "optional"
  now: "required"
  impact: "Existing implementations that don't set quiet_hours will fail validation"

# 4. Changing error codes or messages
error_changed:
  condition: "Invalid time format"
  was: "Return 400"
  now: "Return 422"
  impact: "Client code catching 400 will miss this error"

# 5. Changing output format
output_changed:
  field: "saved_preferences"
  was: "flat object"
  now: "nested object with channel sub-objects"
  impact: "All consumers parsing the output will break"

# 6. Tightening a constraint (making it more restrictive)
constraint_tightened:
  before: "Max 50,000 rows"
  after: "Max 10,000 rows"
  impact: "Exports that worked before may now fail"
```

> **Professor's aside:** There is a simple test for whether a change is breaking: take the existing test suite (built from v1 acceptance criteria), run it against the v2 spec, and see if any tests should fail. If they do, it is a breaking change. If they all still pass, it is additive.

### The Decision Matrix

```
Is the change:                                    Version bump:
─────────────────────────────────────────────────────────────────
Adding a new OPTIONAL field?                      MINOR (1.1)
Adding a new error case?                          MINOR (1.1)
Clarifying a description?                         PATCH (1.0.1)
Fixing a typo?                                    PATCH (1.0.1)
Removing a field?                                 MAJOR (2.0)
Changing a field's type?                          MAJOR (2.0)
Making an optional field required?                MAJOR (2.0)
Changing error codes?                             MAJOR (2.0)
Changing output format?                           MAJOR (2.0)
Adding a new enum value?                          MINOR (1.1)
Removing an enum value?                           MAJOR (2.0)
Relaxing a constraint?                            MINOR (1.1)
Tightening a constraint?                          MAJOR (2.0)
```

---

## 2.4 Migration Specs: Specifying the Transition from v1 to v2

When a breaking change is unavoidable (and sometimes it is), you need a **migration spec** -- a specification that describes how to move from the old version to the new version.

### What a Migration Spec Contains

A migration spec is its own document, separate from both v1 and v2 of the feature spec. It specifies:

1. **What changed** -- A precise diff between v1 and v2
2. **Why it changed** -- The motivation for the breaking change
3. **How to migrate** -- Step-by-step transformation instructions
4. **The migration timeline** -- When v1 will be deprecated and removed
5. **Rollback plan** -- How to revert if the migration fails

```yaml
# spec/migrations/notification-preferences-v1-to-v2.migration.yaml

migration:
  from_spec: "notification-preferences.spec.yaml"
  from_version: "1.1.0"
  to_version: "2.0.0"
  created: "2026-03-01"
  author: "architect-agent"
  status: "approved"

motivation: |
  The v1 flat preferences structure doesn't scale as we add more
  notification channels (SMS, Slack, Teams). V2 restructures
  preferences into a channel-based model where each channel has
  its own sub-configuration.

timeline:
  v2_available: "2026-03-15"
  v1_deprecated: "2026-04-15"
  v1_removed: "2026-06-15"
  migration_window: "90 days"

changes:
  - id: "MC-001"
    type: "restructure"
    description: "Flatten preferences -> nested channel model"
    before:
      schema: |
        {
          email_enabled: boolean,
          push_enabled: boolean,
          sms_enabled: boolean,
          frequency: "immediate" | "hourly" | "daily",
          quiet_hours: { start: string, end: string }
        }
    after:
      schema: |
        {
          channels: {
            email: { enabled: boolean, frequency: string, quiet_hours: TimeRange },
            push: { enabled: boolean, frequency: string, quiet_hours: TimeRange },
            sms: { enabled: boolean, frequency: string, quiet_hours: TimeRange }
          },
          global_quiet_hours: TimeRange | null
        }

  - id: "MC-002"
    type: "field_move"
    description: "frequency moves from top-level to per-channel"
    before: "preferences.frequency"
    after: "preferences.channels.{channel}.frequency"
    migration_logic: |
      For each channel, set channel.frequency = old_frequency.
      This preserves the existing behavior where all channels
      shared the same frequency setting.

  - id: "MC-003"
    type: "field_move"
    description: "quiet_hours becomes per-channel with global fallback"
    before: "preferences.quiet_hours"
    after: "preferences.global_quiet_hours AND per-channel quiet_hours"
    migration_logic: |
      Set global_quiet_hours = old quiet_hours.
      Set each channel.quiet_hours = null (will inherit from global).

data_migration:
  strategy: "lazy"
  description: |
    Existing user preferences are migrated on first access.
    When a user's preferences are read:
    1. Check if preferences are in v1 format
    2. If v1, transform to v2 format using migration_logic above
    3. Save the v2 format
    4. Return v2 format to caller

    This avoids a big-bang migration of all users at once.

  migration_function:
    name: "migratePreferencesV1ToV2"
    input: "V1NotificationPreferences"
    output: "V2NotificationPreferences"
    constraints:
      - "Must be idempotent -- running migration twice produces same result"
      - "Must preserve all user settings -- no data loss"
      - "Must complete within 100ms per user record"

api_compatibility:
  strategy: "dual_support"
  description: |
    During the migration window, the API accepts both v1 and v2 formats.
    v1 requests are silently migrated to v2 on the server side.
    v1 responses are still available via Accept-Version: v1 header.
    After the migration window, v1 is removed.

  endpoints:
    - path: "/api/preferences"
      v1_behavior: "Accepts flat format, returns flat format"
      v2_behavior: "Accepts channel format, returns channel format"
      during_migration: "Accepts both, returns based on Accept-Version header"

rollback_plan:
  trigger: "Migration causes >1% error rate increase"
  steps:
    - "Revert API to v1-only mode"
    - "v2 preferences are back-converted to v1 format"
    - "Investigate root cause before re-attempting"
  constraints:
    - "Rollback must complete within 30 minutes"
    - "No user data may be lost during rollback"

acceptance_criteria:
  - "Given a user with v1 preferences, when they access their preferences, they receive v2 format"
  - "Given a user with v1 preferences, the migration preserves all existing settings"
  - "Given a user sending v1 format during migration window, the API accepts and migrates"
  - "Given the migration is complete, v1 format returns 410 Gone with migration instructions"
  - "Given a migration failure, rollback restores v1 behavior within 30 minutes"
```

### The Migration Function Implementation

The Builder Agent, given the migration spec, produces:

```typescript
// src/migrations/notification-preferences-v1-to-v2.ts

/**
 * Migration: notification-preferences v1.1.0 -> v2.0.0
 * Spec: spec/migrations/notification-preferences-v1-to-v2.migration.yaml
 *
 * Transforms flat preference structure to channel-based model.
 * Idempotent: safe to run multiple times on the same record.
 */

interface V1NotificationPreferences {
  email_enabled: boolean;
  push_enabled: boolean;
  sms_enabled?: boolean;        // Added in v1.1.0
  frequency: 'immediate' | 'hourly' | 'daily';
  quiet_hours?: {
    start: string;              // HH:MM format
    end: string;                // HH:MM format
  };
  category_overrides?: Record<string, unknown>;  // Added in v1.1.0
}

interface V2ChannelPreferences {
  enabled: boolean;
  frequency: 'immediate' | 'hourly' | 'daily' | 'weekly';
  quiet_hours: { start: string; end: string } | null;
}

interface V2NotificationPreferences {
  schema_version: '2.0.0';
  channels: {
    email: V2ChannelPreferences;
    push: V2ChannelPreferences;
    sms: V2ChannelPreferences;
  };
  global_quiet_hours: { start: string; end: string } | null;
  category_overrides: Record<string, unknown> | null;
}

/**
 * Detects whether a preferences object is in v1 format.
 * Used for lazy migration on first access.
 */
export function isV1Format(
  prefs: unknown,
): prefs is V1NotificationPreferences {
  if (typeof prefs !== 'object' || prefs === null) return false;
  const p = prefs as Record<string, unknown>;
  return (
    'email_enabled' in p &&
    typeof p.email_enabled === 'boolean' &&
    !('schema_version' in p)
  );
}

/**
 * Detects whether a preferences object is already in v2 format.
 */
export function isV2Format(
  prefs: unknown,
): prefs is V2NotificationPreferences {
  if (typeof prefs !== 'object' || prefs === null) return false;
  const p = prefs as Record<string, unknown>;
  return p.schema_version === '2.0.0' && 'channels' in p;
}

/**
 * Migrate v1 preferences to v2 format.
 *
 * Idempotent: if already v2, returns as-is.
 * Preserves all user settings -- no data loss.
 *
 * Migration rules (from migration spec):
 * - MC-001: Flat -> nested channel model
 * - MC-002: frequency moves to per-channel (same value for all)
 * - MC-003: quiet_hours becomes global_quiet_hours, per-channel set to null
 */
export function migratePreferencesV1ToV2(
  prefs: V1NotificationPreferences | V2NotificationPreferences,
): V2NotificationPreferences {
  // Idempotency check -- already v2, return as-is
  if (isV2Format(prefs)) {
    return prefs;
  }

  const v1 = prefs as V1NotificationPreferences;

  return {
    schema_version: '2.0.0',

    // MC-001: Restructure into channel-based model
    channels: {
      email: {
        enabled: v1.email_enabled,
        frequency: v1.frequency,   // MC-002: Copy global frequency to each channel
        quiet_hours: null,         // MC-003: Inherit from global
      },
      push: {
        enabled: v1.push_enabled,
        frequency: v1.frequency,
        quiet_hours: null,
      },
      sms: {
        enabled: v1.sms_enabled ?? false,  // Default for pre-v1.1 users
        frequency: v1.frequency,
        quiet_hours: null,
      },
    },

    // MC-003: Old quiet_hours becomes global_quiet_hours
    global_quiet_hours: v1.quiet_hours ?? null,

    // Preserve v1.1 category overrides
    category_overrides: v1.category_overrides ?? null,
  };
}

/**
 * Reverse migration (for rollback).
 * Converts v2 back to v1 format.
 *
 * Note: Some data loss is possible (per-channel settings collapse to global).
 * This is acceptable per the rollback plan in the migration spec.
 */
export function rollbackPreferencesV2ToV1(
  prefs: V2NotificationPreferences,
): V1NotificationPreferences {
  return {
    email_enabled: prefs.channels.email.enabled,
    push_enabled: prefs.channels.push.enabled,
    sms_enabled: prefs.channels.sms.enabled,
    // Take frequency from email channel as the "global" frequency
    frequency: prefs.channels.email.frequency as 'immediate' | 'hourly' | 'daily',
    quiet_hours: prefs.global_quiet_hours ?? undefined,
    category_overrides: prefs.category_overrides ?? undefined,
  };
}
```

---

## 2.5 Deprecation Patterns: Marking Spec Sections as Deprecated Before Removal

Never remove a spec feature without warning. Deprecation is the buffer zone between "this exists" and "this is gone."

### The Deprecation Lifecycle

```
ACTIVE          ->    DEPRECATED       ->    REMOVED
(v1.0 - v1.x)       (v1.x - v1.y)          (v2.0+)

Feature works        Feature works           Feature is gone
normally.            but warns consumers     Requests using it
                     that it will be         return errors.
                     removed.
```

### Deprecation in Spec YAML

```yaml
# Spec with deprecated fields (v1.2.0)
spec_version: "1.2.0"
feature: "notification-preferences"

inputs:
  - name: "user_id"
    type: "string"
    required: true

  - name: "preferences"
    type: "NotificationPreferences"
    required: true
    properties:
      email_enabled:
        type: "boolean"
        required: true
      push_enabled:
        type: "boolean"
        required: true
      sms_enabled:
        type: "boolean"
        required: false
        since: "1.1.0"
        default: false

      # DEPRECATED FIELDS -- still functional but will be removed in v2.0
      frequency:
        type: "enum"
        values: ["immediate", "hourly", "daily"]
        required: true
        deprecated:
          since: "1.2.0"
          removal_version: "2.0.0"
          reason: "Replaced by per-channel frequency settings"
          migration: "Use channel_preferences.{channel}.frequency instead"
          sunset_date: "2026-06-15"

      quiet_hours:
        type: "TimeRange"
        required: false
        deprecated:
          since: "1.2.0"
          removal_version: "2.0.0"
          reason: "Replaced by global_quiet_hours and per-channel quiet_hours"
          migration: "Use global_quiet_hours for system-wide settings"
          sunset_date: "2026-06-15"

      # NEW FIELDS (replacements for deprecated ones)
      channel_preferences:
        type: "Map<Channel, ChannelConfig>"
        required: false   # Optional during deprecation period
        since: "1.2.0"
        description: "Per-channel notification configuration (replaces global frequency)"

      global_quiet_hours:
        type: "TimeRange"
        required: false
        since: "1.2.0"
        description: "System-wide quiet hours (replaces quiet_hours)"

deprecation_notices:
  - field: "frequency"
    message: "The 'frequency' field is deprecated and will be removed in v2.0.0. Use channel_preferences.{channel}.frequency instead."
    behavior_during_deprecation: |
      When 'frequency' is provided:
      1. Apply it as the default for all channels
      2. Include a deprecation warning in the response headers
      3. Log a deprecation metric for monitoring migration progress

  - field: "quiet_hours"
    message: "The 'quiet_hours' field is deprecated and will be removed in v2.0.0. Use global_quiet_hours instead."
    behavior_during_deprecation: |
      When 'quiet_hours' is provided:
      1. Treat it as global_quiet_hours
      2. Include a deprecation warning in the response headers
      3. Log a deprecation metric
```

### Deprecation Warning Implementation

```typescript
// src/middleware/deprecation-warning.middleware.ts

/**
 * Middleware that detects deprecated field usage and adds warning headers.
 * Implements deprecation_notices from the spec.
 */

interface DeprecationNotice {
  field: string;
  message: string;
  removalVersion: string;
  sunsetDate: string;
}

const DEPRECATION_NOTICES: DeprecationNotice[] = [
  {
    field: 'frequency',
    message: "The 'frequency' field is deprecated. Use channel_preferences.{channel}.frequency.",
    removalVersion: '2.0.0',
    sunsetDate: '2026-06-15',
  },
  {
    field: 'quiet_hours',
    message: "The 'quiet_hours' field is deprecated. Use global_quiet_hours.",
    removalVersion: '2.0.0',
    sunsetDate: '2026-06-15',
  },
];

export function checkDeprecatedFields(
  body: Record<string, unknown>,
): DeprecationNotice[] {
  const warnings: DeprecationNotice[] = [];

  for (const notice of DEPRECATION_NOTICES) {
    if (notice.field in body || hasNestedField(body, notice.field)) {
      warnings.push(notice);
    }
  }

  return warnings;
}

export function addDeprecationHeaders(
  headers: Record<string, string>,
  warnings: DeprecationNotice[],
): void {
  if (warnings.length === 0) return;

  // Standard HTTP Deprecation header (RFC 8594)
  headers['Deprecation'] = 'true';

  // Sunset header -- when the deprecated feature will be removed
  const earliestSunset = warnings
    .map((w) => new Date(w.sunsetDate))
    .sort((a, b) => a.getTime() - b.getTime())[0];
  headers['Sunset'] = earliestSunset.toUTCString();

  // Custom warning messages
  headers['X-Deprecation-Warnings'] = JSON.stringify(
    warnings.map((w) => w.message),
  );
}

function hasNestedField(
  obj: Record<string, unknown>,
  fieldPath: string,
): boolean {
  const parts = fieldPath.split('.');
  let current: unknown = obj;
  for (const part of parts) {
    if (current === null || current === undefined) return false;
    if (typeof current !== 'object') return false;
    current = (current as Record<string, unknown>)[part];
  }
  return current !== undefined;
}
```

---

## 2.6 How This Mirrors API Versioning Strategies

The practices we are describing for spec versioning are not new inventions. They come directly from decades of API versioning experience.

### Google's API Versioning Strategy

Google maintains one of the world's largest API surfaces -- thousands of APIs across Google Cloud, Maps, YouTube, Gmail, and more. Their approach to API versioning has several key principles that map directly to spec versioning:

**Stability levels:** Google categorizes APIs into stability tiers (Alpha, Beta, GA), each with different change guarantees. For specs, we can adopt the same pattern:

```yaml
# Spec stability levels (inspired by Google's API stability tiers)
spec_stability: "ga"  # alpha | beta | ga

# Alpha: Breaking changes may happen at any time. No migration support.
# Beta: Breaking changes require 30-day notice. Migration specs provided.
# GA: Breaking changes only in major versions. Full migration support.
```

**Version in the path:** Google puts the API version in the URL path (`/v1/`, `/v2/`). For specs, we can put the version in the filename or directory:

```
specs/
  notification-preferences/
    v1/
      notification-preferences.spec.yaml     # v1.x
    v2/
      notification-preferences.spec.yaml     # v2.x
    migrations/
      v1-to-v2.migration.yaml
```

### Stripe's API Evolution

Stripe is famous for maintaining backward compatibility across years of API changes. Their approach is instructive for spec management:

**Stripe's "API version" concept:** Every Stripe API request can specify a version via a header. Old versions continue to work. Stripe maintains compatibility shims that translate between versions internally.

This maps to a powerful SDD pattern:

```yaml
# Spec with version compatibility layer
compatibility:
  supported_versions: ["1.0", "1.1", "1.2", "2.0"]
  default_version: "2.0"
  version_shims:
    "1.0_to_2.0":
      description: "Translates v1.0 flat format to v2.0 channel format"
      function: "migratePreferencesV1ToV2"
    "1.1_to_2.0":
      description: "Translates v1.1 format (with SMS) to v2.0"
      function: "migratePreferencesV1ToV2"  # Same function handles both
    "1.2_to_2.0":
      description: "Translates v1.2 (deprecated fields) to v2.0"
      function: "migratePreferencesV1ToV2"
```

> **Professor's aside:** Stripe manages to support API versions that are years old. They do this by treating each version as a series of transformations: request comes in as v1, gets transformed to v2 format internally, processed, then the response is transformed back to v1 format before returning. It is expensive to maintain, but it is how you earn trust with developers. The same principle applies to specs: always provide a migration path.

### How This Relates to AI Model Versioning

The major AI companies face this same challenge with model versioning. When Anthropic releases a new Claude model, or OpenAI releases a new GPT version, they face the same spec evolution questions:

- What behaviors changed between versions?
- Are existing prompts (specs for model behavior) still valid?
- How do you migrate from the old model to the new one?

We will look at these in detail in sections 2.10 and 2.11.

---

## 2.7 Changelog-Driven Development

Every spec change, no matter how small, gets a changelog entry. This is not optional. The changelog is how you track the full history of a spec's evolution and is essential for debugging, auditing, and migration planning.

### Changelog Format

```yaml
# The changelog lives inside the spec file
changelog:
  - version: "2.0.0"
    date: "2026-03-15"
    author: "architect-agent"
    type: "major"
    breaking: true
    description: "Restructured to channel-based preference model"
    changes:
      - type: "breaking"
        id: "MC-001"
        description: "Flat preferences restructured to nested channel model"
        migration: "See migration spec v1-to-v2.migration.yaml"
      - type: "breaking"
        id: "MC-002"
        description: "frequency field moved from top-level to per-channel"
      - type: "breaking"
        id: "MC-003"
        description: "quiet_hours replaced by global_quiet_hours + per-channel"
      - type: "addition"
        description: "Added 'weekly' as a frequency option"

  - version: "1.2.0"
    date: "2026-02-15"
    author: "architect-agent"
    type: "minor"
    breaking: false
    description: "Deprecated flat fields in preparation for v2.0"
    changes:
      - type: "deprecation"
        description: "Deprecated 'frequency' field -- use per-channel settings"
      - type: "deprecation"
        description: "Deprecated 'quiet_hours' -- use global_quiet_hours"
      - type: "addition"
        description: "Added channel_preferences for per-channel configuration"
      - type: "addition"
        description: "Added global_quiet_hours as replacement for quiet_hours"

  - version: "1.1.0"
    date: "2026-02-01"
    author: "architect-agent"
    type: "minor"
    breaking: false
    description: "Added SMS channel and category-level preferences"
    changes:
      - type: "addition"
        description: "Added sms_enabled field (optional, defaults to false)"
      - type: "addition"
        description: "Added category_overrides field (optional)"
      - type: "addition"
        description: "Added error case for invalid phone number"

  - version: "1.0.0"
    date: "2026-01-15"
    author: "architect-agent"
    type: "initial"
    breaking: false
    description: "Initial specification for notification preferences"
    changes:
      - type: "initial"
        description: "Full specification: email, push, quiet hours, frequency"
```

### Changelog as Agent Context

The changelog serves a dual purpose. It is documentation for humans, but it is also **context for agents**. When the Architect Agent needs to update a spec, the changelog tells it:

- What has been tried before
- Why certain decisions were made
- What was deprecated and when
- The velocity of change (is this a stable spec or a rapidly evolving one?)

```python
# How an Architect Agent uses changelog context

architect_prompt = f"""
You need to update the notification-preferences spec.

New requirement: Add Slack as a notification channel.

Current spec version: 2.0.0
Changelog summary:
- v1.0: Initial flat structure
- v1.1: Added SMS
- v1.2: Deprecated flat fields, introduced per-channel model
- v2.0: Full migration to channel-based model

Given this history, adding Slack should be a MINOR change (v2.1.0)
since the v2.0 channel model was specifically designed to accommodate
new channels without breaking changes.

Add Slack as a new channel in the channels map, following the same
pattern as email, push, and sms.
"""
```

---

## 2.8 The "Diff Spec": Specifying Only What Changed

For large specs, sending the entire document through the agent pipeline every time a small change is made is wasteful and error-prone. The "diff spec" pattern addresses this by specifying only the delta.

### Diff Spec Format

```yaml
# spec/diffs/notification-preferences-v2.0-to-v2.1.diff.yaml

diff_spec:
  base_spec: "notification-preferences.spec.yaml"
  base_version: "2.0.0"
  target_version: "2.1.0"
  type: "minor"
  date: "2026-04-01"
  author: "architect-agent"

additions:
  - path: "inputs.preferences.channels.slack"
    value:
      enabled:
        type: "boolean"
        required: false
        default: false
      frequency:
        type: "enum"
        values: ["immediate", "hourly", "daily", "weekly"]
        required: false
        default: "immediate"
      quiet_hours:
        type: "TimeRange"
        required: false
        default: null
      webhook_url:
        type: "string"
        required: false  # Required only when slack.enabled = true
        description: "Slack webhook URL for delivering notifications"

  - path: "error_cases"
    append:
      - condition: "Slack enabled but no webhook URL provided"
        behavior: "Return 400 with message 'Slack webhook URL is required when Slack notifications are enabled'"
      - condition: "Slack webhook URL is invalid or unreachable"
        behavior: "Return 400 with message 'Invalid Slack webhook URL'"

  - path: "acceptance_criteria"
    append:
      - "Given a user enables Slack notifications with a valid webhook URL, notifications are delivered to that Slack channel"
      - "Given a user enables Slack without providing a webhook URL, a 400 error is returned"

modifications: []

removals: []

# The Builder Agent can apply this diff to the base spec
# to produce the full v2.1.0 spec
```

### Applying Diff Specs Programmatically

```typescript
// sdd-tools/src/diff-spec-applicator.ts

import * as yaml from 'js-yaml';
import * as fs from 'fs';
import * as path from 'path';
import { set, get } from 'lodash';

interface DiffSpec {
  diff_spec: {
    base_spec: string;
    base_version: string;
    target_version: string;
    type: 'major' | 'minor' | 'patch';
  };
  additions: Array<{
    path: string;
    value?: unknown;
    append?: unknown;
  }>;
  modifications: Array<{
    path: string;
    old_value: unknown;
    new_value: unknown;
  }>;
  removals: Array<{
    path: string;
  }>;
}

/**
 * Apply a diff spec to a base spec to produce the new version.
 *
 * This is the spec equivalent of applying a git patch.
 */
export function applyDiffSpec(
  baseSpec: Record<string, unknown>,
  diffSpec: DiffSpec,
): Record<string, unknown> {
  // Deep clone to avoid mutating the original
  const result = JSON.parse(JSON.stringify(baseSpec));

  // Apply additions
  for (const addition of diffSpec.additions) {
    if (addition.value !== undefined) {
      set(result, addition.path, addition.value);
    }
    if (addition.append !== undefined) {
      const existing = get(result, addition.path);
      if (Array.isArray(existing) && Array.isArray(addition.append)) {
        set(result, addition.path, [...existing, ...addition.append]);
      }
    }
  }

  // Apply modifications
  for (const mod of diffSpec.modifications) {
    const current = get(result, mod.path);
    if (JSON.stringify(current) !== JSON.stringify(mod.old_value)) {
      throw new Error(
        `Modification conflict at ${mod.path}: expected ${JSON.stringify(mod.old_value)}, found ${JSON.stringify(current)}`,
      );
    }
    set(result, mod.path, mod.new_value);
  }

  // Apply removals (in reverse order to avoid index shifts)
  for (const removal of [...diffSpec.removals].reverse()) {
    const parts = removal.path.split('.');
    const parent = get(result, parts.slice(0, -1).join('.'));
    const key = parts[parts.length - 1];

    if (Array.isArray(parent)) {
      parent.splice(Number(key), 1);
    } else if (typeof parent === 'object' && parent !== null) {
      delete (parent as Record<string, unknown>)[key];
    }
  }

  // Update version
  result.spec_version = diffSpec.diff_spec.target_version;

  return result;
}
```

---

## 2.9 Backward Compatibility as a Spec Constraint

Backward compatibility is not just a nice-to-have. It is a constraint that should be written directly into your spec. This makes it enforceable by the Critic Agent.

```yaml
# spec/constraints/backward-compatibility.constraint.yaml

compatibility_constraints:
  name: "backward-compatibility-policy"
  applies_to: "all specs with status: 'ga'"

  rules:
    - id: "BC-001"
      rule: "No required fields may be removed in minor versions"
      enforcement: "Critic Agent must check every spec diff for removed required fields"

    - id: "BC-002"
      rule: "No field types may be changed in minor versions"
      enforcement: "Critic Agent must verify field types match between versions"

    - id: "BC-003"
      rule: "New required fields are only allowed in major versions"
      enforcement: "Critic Agent must verify all new fields in minor versions are optional"

    - id: "BC-004"
      rule: "Enum values may be added but not removed in minor versions"
      enforcement: "Critic Agent must verify no enum values were removed"

    - id: "BC-005"
      rule: "Error codes may be added but not changed or removed in minor versions"
      enforcement: "Critic Agent must verify existing error codes are preserved"

    - id: "BC-006"
      rule: "Constraints may be relaxed but not tightened in minor versions"
      enforcement: "Critic Agent must verify constraints are equal or less restrictive"

    - id: "BC-007"
      rule: "All breaking changes require a migration spec"
      enforcement: "Major version bumps without a migration spec are rejected"

    - id: "BC-008"
      rule: "Deprecated fields must remain functional for at least 2 minor versions"
      enforcement: "Fields deprecated in v1.2 cannot be removed before v1.4"
```

### Critic Agent Backward Compatibility Check

```typescript
// sdd-tools/src/backward-compatibility-checker.ts

interface CompatibilityViolation {
  rule: string;
  field: string;
  description: string;
  severity: 'critical' | 'high';
}

/**
 * Check that a spec change respects backward compatibility constraints.
 * Used by the Critic Agent to validate spec evolutions.
 */
export function checkBackwardCompatibility(
  oldSpec: Record<string, unknown>,
  newSpec: Record<string, unknown>,
  versionBump: 'major' | 'minor' | 'patch',
): CompatibilityViolation[] {
  const violations: CompatibilityViolation[] = [];

  if (versionBump === 'major') {
    // Major versions allow breaking changes, but still
    // check for migration spec
    return violations;
  }

  // BC-001: No required fields removed
  const oldRequiredFields = getRequiredFields(oldSpec);
  const newRequiredFields = getRequiredFields(newSpec);
  for (const field of oldRequiredFields) {
    if (!newRequiredFields.includes(field)) {
      violations.push({
        rule: 'BC-001',
        field,
        description: `Required field '${field}' was removed in a ${versionBump} version`,
        severity: 'critical',
      });
    }
  }

  // BC-003: New required fields only in major versions
  for (const field of newRequiredFields) {
    if (!oldRequiredFields.includes(field)) {
      violations.push({
        rule: 'BC-003',
        field,
        description: `New required field '${field}' added in a ${versionBump} version (must be optional or wait for major)`,
        severity: 'critical',
      });
    }
  }

  // BC-004: Enum values not removed
  const oldEnums = getEnumFields(oldSpec);
  const newEnums = getEnumFields(newSpec);
  for (const [fieldName, oldValues] of Object.entries(oldEnums)) {
    const newValues = newEnums[fieldName];
    if (newValues) {
      for (const val of oldValues) {
        if (!newValues.includes(val)) {
          violations.push({
            rule: 'BC-004',
            field: fieldName,
            description: `Enum value '${val}' removed from '${fieldName}' in a ${versionBump} version`,
            severity: 'critical',
          });
        }
      }
    }
  }

  return violations;
}

function getRequiredFields(spec: Record<string, unknown>): string[] {
  // Implementation: traverse spec inputs and collect required field names
  const inputs = (spec as any).inputs ?? [];
  return inputs
    .filter((input: any) => input.required)
    .map((input: any) => input.name);
}

function getEnumFields(
  spec: Record<string, unknown>,
): Record<string, string[]> {
  // Implementation: find all enum-type fields and their values
  const result: Record<string, string[]> = {};
  const inputs = (spec as any).inputs ?? [];
  for (const input of inputs) {
    if (input.type === 'enum' && input.values) {
      result[input.name] = input.values;
    }
  }
  return result;
}
```

---

## 2.10 How Anthropic Versions Claude's Capabilities Across Model Releases

Anthropic's approach to versioning Claude's capabilities is a fascinating case study in spec evolution at scale.

When Anthropic releases a new Claude model (e.g., Claude 3 Opus, Claude 3.5 Sonnet, Claude 4 Sonnet, Claude 4 Opus), they face a fundamental spec evolution challenge: the new model may behave differently from the old one, even with the same prompt.

### The Model Card as a Spec

Anthropic publishes model cards for each Claude release. These are, in essence, specifications of model behavior:

- **Capabilities:** What the model can and cannot do
- **Behavioral changes:** How the model differs from previous versions
- **Safety improvements:** What new guardrails were added
- **Performance characteristics:** Latency, context window, token limits

From an SDD perspective, each model card is a spec, and model upgrades are spec evolutions:

```yaml
# Conceptual model spec (inspired by Anthropic's approach)
model_spec:
  name: "claude-4-opus"
  version: "4.0"
  previous_version: "3.5-sonnet"

  capabilities_added:
    - "Extended thinking mode for complex reasoning"
    - "Improved code generation accuracy"
    - "Better instruction following for structured outputs"

  capabilities_changed:
    - capability: "JSON output formatting"
      before: "Sometimes includes trailing commas"
      after: "Strictly valid JSON in all cases"
      impact: "Code relying on lenient JSON parsing may behave differently"

  behavioral_changes:
    - behavior: "Refusal patterns"
      description: "Model is less likely to refuse benign requests"
      migration: "Prompts designed to work around over-refusal may now produce different results"

  deprecations:
    - feature: "XML-based tool calling format"
      status: "deprecated"
      replacement: "JSON-based tool calling"
      removal: "Next major version"
```

### What This Means for SDD Practitioners

When you upgrade the model powering your agents, you are performing a spec evolution on the agent's capabilities. Your Architect Agent prompt that works with Claude 3.5 Sonnet may produce different specs when run on Claude 4 Opus.

This is why **pinning model versions** in your SDD pipeline is important:

```yaml
# sdd-pipeline-config.yaml
agents:
  architect:
    model: "claude-sonnet-4-20250514"      # Pinned version
    temperature: 0.3
  builder:
    model: "claude-sonnet-4-20250514"
    temperature: 0.2
  critic:
    model: "claude-sonnet-4-20250514"
    temperature: 0.1

# Model upgrade is treated as a spec change:
# 1. Update model versions
# 2. Run regression tests
# 3. Validate outputs match expectations
# 4. Document behavioral changes in changelog
```

---

## 2.11 How OpenAI Manages Breaking Changes Across GPT Versions

OpenAI's approach to model versioning provides additional lessons. When OpenAI deprecates a model (e.g., sunsetting `gpt-3.5-turbo-0613` in favor of newer versions), they follow a pattern similar to API deprecation:

1. **Announce the deprecation** with a timeline
2. **Provide a migration window** (typically 3-6 months)
3. **Maintain the old model** during the window
4. **Redirect to a default model** after the window closes

For SDD pipelines, this means:

```yaml
# Agent model dependency spec
model_dependencies:
  - agent: "architect"
    current_model: "gpt-4o-2024-08-06"
    fallback_model: "gpt-4o"         # Latest stable
    pinned: true
    last_validated: "2026-02-01"
    next_validation_due: "2026-03-01"
    notes: "Architect output was validated against 50 test cases"

  - agent: "builder"
    current_model: "gpt-4o-2024-08-06"
    fallback_model: "gpt-4o"
    pinned: true
    last_validated: "2026-02-01"
    notes: "Builder code quality was validated against acceptance criteria"

model_upgrade_process:
  steps:
    - "Run all agent regression tests with new model"
    - "Compare output quality metrics (completeness, correctness)"
    - "Check for behavioral changes in structured output format"
    - "Update model version in config"
    - "Document any prompt adjustments needed"
    - "Add changelog entry for model upgrade"
```

> **Professor's aside:** This is one of the hidden costs of AI-assisted development that people do not talk about enough. When you build a pipeline that depends on a specific model, you are taking on a dependency that will need maintenance. The model provider will update, deprecate, and eventually remove that model version. Your SDD pipeline needs a plan for that, just like it needs a plan for any other dependency update.

---

## 2.12 Practical Exercise: Evolving a Spec Through Three Versions

Let us walk through a complete evolution exercise. We will build a user profile feature through three versions, demonstrating additive changes, deprecation, and breaking changes.

### Version 1.0.0: The Initial Spec

```yaml
# spec/features/user-profile.spec.yaml -- v1.0.0

spec_version: "1.0.0"
feature: "user-profile"
status: "ga"
created: "2026-01-01"

changelog:
  - version: "1.0.0"
    date: "2026-01-01"
    type: "initial"
    description: "Initial user profile specification"

purpose: |
  Allow users to view and update their profile information.

inputs:
  - name: "user_id"
    type: "string"
    required: true
  - name: "profile_data"
    type: "UserProfile"
    required: true
    properties:
      display_name:
        type: "string"
        required: true
        max_length: 100
      email:
        type: "string"
        required: true
        format: "email"
      bio:
        type: "string"
        required: false
        max_length: 500
      avatar_url:
        type: "string"
        required: false
        format: "url"

outputs:
  - name: "updated_profile"
    type: "UserProfile"
    description: "The saved profile data"

constraints:
  - "display_name must be between 1 and 100 characters"
  - "email must be a valid email format"
  - "bio must not exceed 500 characters"
  - "avatar_url must be a valid HTTPS URL"

error_cases:
  - condition: "display_name exceeds 100 characters"
    behavior: "Return 400 with field-level validation error"
  - condition: "email is invalid format"
    behavior: "Return 400 with field-level validation error"
  - condition: "User not authenticated"
    behavior: "Return 401"

acceptance_criteria:
  - "Given valid profile data, the profile is saved and returned"
  - "Given an invalid email, a 400 error with field details is returned"
  - "Given a display_name over 100 chars, a 400 error is returned"
```

### Version 1.1.0: Additive Change (MINOR)

New requirement: Users want to add social media links and set a timezone.

```yaml
# spec/features/user-profile.spec.yaml -- v1.1.0

spec_version: "1.1.0"
feature: "user-profile"
status: "ga"
created: "2026-01-01"
last_modified: "2026-02-01"

changelog:
  - version: "1.1.0"
    date: "2026-02-01"
    type: "minor"
    breaking: false
    description: "Added social links and timezone support"
    changes:
      - type: "addition"
        description: "Added optional social_links field (map of platform -> URL)"
      - type: "addition"
        description: "Added optional timezone field (IANA timezone string)"
      - type: "addition"
        description: "Added error case for invalid timezone"
      - type: "addition"
        description: "Added acceptance criteria for social links and timezone"
  - version: "1.0.0"
    date: "2026-01-01"
    type: "initial"
    description: "Initial user profile specification"

purpose: |
  Allow users to view and update their profile information,
  including social media links and timezone preferences.

inputs:
  - name: "user_id"
    type: "string"
    required: true
  - name: "profile_data"
    type: "UserProfile"
    required: true
    properties:
      display_name:
        type: "string"
        required: true
        max_length: 100
      email:
        type: "string"
        required: true
        format: "email"
      bio:
        type: "string"
        required: false
        max_length: 500
      avatar_url:
        type: "string"
        required: false
        format: "url"
      # NEW in v1.1.0
      social_links:
        type: "Map<string, string>"
        required: false               # Optional -- backward compatible
        since: "1.1.0"
        description: "Map of platform name to profile URL"
        constraints:
          - "Keys must be lowercase alphanumeric (e.g., 'github', 'twitter', 'linkedin')"
          - "Values must be valid HTTPS URLs"
          - "Maximum 10 social links"
      # NEW in v1.1.0
      timezone:
        type: "string"
        required: false               # Optional -- backward compatible
        since: "1.1.0"
        format: "iana-timezone"
        default: "UTC"
        description: "User's preferred timezone (IANA format, e.g., 'America/New_York')"

outputs:
  - name: "updated_profile"
    type: "UserProfile"
    description: "The saved profile data (now includes social_links and timezone if set)"

constraints:
  - "display_name must be between 1 and 100 characters"
  - "email must be a valid email format"
  - "bio must not exceed 500 characters"
  - "avatar_url must be a valid HTTPS URL"
  - "social_links values must be valid HTTPS URLs"          # NEW
  - "Maximum 10 social links per user"                      # NEW
  - "timezone must be a valid IANA timezone identifier"     # NEW

error_cases:
  - condition: "display_name exceeds 100 characters"
    behavior: "Return 400 with field-level validation error"
  - condition: "email is invalid format"
    behavior: "Return 400 with field-level validation error"
  - condition: "User not authenticated"
    behavior: "Return 401"
  # NEW in v1.1.0
  - condition: "Social link URL is not valid HTTPS"
    behavior: "Return 400 with field-level validation error identifying the invalid link"
    since: "1.1.0"
  - condition: "More than 10 social links provided"
    behavior: "Return 400 with message 'Maximum 10 social links allowed'"
    since: "1.1.0"
  - condition: "Invalid timezone identifier"
    behavior: "Return 400 with message 'Invalid timezone. Use IANA format (e.g., America/New_York)'"
    since: "1.1.0"

acceptance_criteria:
  - "Given valid profile data, the profile is saved and returned"
  - "Given an invalid email, a 400 error with field details is returned"
  - "Given a display_name over 100 chars, a 400 error is returned"
  # NEW in v1.1.0
  - "Given valid social links, they are saved and returned in the profile"
  - "Given a social link with HTTP (not HTTPS) URL, a 400 error is returned"
  - "Given 11 social links, a 400 error is returned"
  - "Given a valid IANA timezone, it is saved and returned"
  - "Given an invalid timezone string, a 400 error is returned"
  - "Given no timezone provided, the profile defaults to UTC"
```

Notice how every new element is:
- Marked with `since: "1.1.0"`
- Made optional with a sensible default
- Accompanied by new error cases and acceptance criteria
- Documented in the changelog

### Version 2.0.0: Breaking Change (MAJOR)

New requirement: The company is internationalizing. The profile needs to support multiple languages, and the flat `display_name` / `bio` structure needs to become a localized content model.

```yaml
# spec/features/user-profile.spec.yaml -- v2.0.0

spec_version: "2.0.0"
feature: "user-profile"
status: "ga"
created: "2026-01-01"
last_modified: "2026-03-15"

changelog:
  - version: "2.0.0"
    date: "2026-03-15"
    type: "major"
    breaking: true
    description: "Internationalization: localized profile content model"
    migration_spec: "migrations/user-profile-v1-to-v2.migration.yaml"
    changes:
      - type: "breaking"
        id: "UP-001"
        description: "display_name changed from string to LocalizedString"
      - type: "breaking"
        id: "UP-002"
        description: "bio changed from string to LocalizedString"
      - type: "addition"
        description: "Added primary_language field (required)"
      - type: "addition"
        description: "Added supported_languages field"
      - type: "removal"
        description: "Removed avatar_url (replaced by avatar object in v1.2, now mandatory)"
  - version: "1.1.0"
    date: "2026-02-01"
    type: "minor"
    breaking: false
    description: "Added social links and timezone support"
  - version: "1.0.0"
    date: "2026-01-01"
    type: "initial"
    description: "Initial user profile specification"

purpose: |
  Allow users to view and update their profile information with
  support for multiple languages. Profile content (display name,
  bio) can be provided in multiple languages with a primary language
  designation.

types:
  LocalizedString:
    description: "A string value with translations"
    properties:
      default:
        type: "string"
        required: true
        description: "The value in the user's primary language"
      translations:
        type: "Map<LanguageCode, string>"
        required: false
        description: "Translations keyed by ISO 639-1 language code"

inputs:
  - name: "user_id"
    type: "string"
    required: true
  - name: "profile_data"
    type: "UserProfile"
    required: true
    properties:
      display_name:
        type: "LocalizedString"      # CHANGED from string (breaking)
        required: true
        constraints:
          - "default value must be 1-100 characters"
          - "each translation must be 1-100 characters"
      email:
        type: "string"
        required: true
        format: "email"
      bio:
        type: "LocalizedString"      # CHANGED from string (breaking)
        required: false
        constraints:
          - "default value must not exceed 500 characters"
          - "each translation must not exceed 500 characters"
      primary_language:
        type: "string"
        required: true               # NEW required field (breaking)
        format: "iso-639-1"
        description: "The user's primary language (e.g., 'en', 'fr', 'ja')"
      supported_languages:
        type: "string[]"
        required: false
        description: "Additional languages the user has provided translations for"
        default: []
      social_links:
        type: "Map<string, string>"
        required: false
      timezone:
        type: "string"
        required: false
        format: "iana-timezone"
        default: "UTC"
      avatar:
        type: "AvatarConfig"         # CHANGED from avatar_url (breaking)
        required: false
        properties:
          url:
            type: "string"
            format: "url"
          alt_text:
            type: "LocalizedString"
          crop_area:
            type: "Rectangle"
            required: false

# ... constraints, error_cases, and acceptance_criteria updated accordingly
```

And the accompanying migration spec:

```yaml
# spec/migrations/user-profile-v1-to-v2.migration.yaml

migration:
  from_spec: "user-profile.spec.yaml"
  from_version: "1.1.0"
  to_version: "2.0.0"

changes:
  - id: "UP-001"
    type: "type_change"
    field: "display_name"
    from: "string"
    to: "LocalizedString"
    migration_logic: |
      Convert: "John Doe" ->
      { default: "John Doe", translations: {} }
      Set primary_language to user's browser locale or "en" as fallback.

  - id: "UP-002"
    type: "type_change"
    field: "bio"
    from: "string"
    to: "LocalizedString"
    migration_logic: |
      Convert: "I am a developer" ->
      { default: "I am a developer", translations: {} }

  - id: "UP-003"
    type: "field_replacement"
    old_field: "avatar_url"
    new_field: "avatar"
    migration_logic: |
      Convert: "https://example.com/avatar.jpg" ->
      { url: "https://example.com/avatar.jpg", alt_text: { default: "" }, crop_area: null }

data_migration:
  strategy: "lazy"
  description: "Migrate on first access. Detect v1 format by absence of primary_language field."
```

---

## 2.13 Git Strategies for Spec Management

Your specs live in version control alongside your code. How you manage them in Git has a significant impact on your team's ability to evolve specs safely.

### Directory Structure

```
project/
  src/
    features/
      user-profile/
        ...
      notifications/
        ...
  specs/
    features/
      user-profile/
        user-profile.spec.yaml            # Current version
        CHANGELOG.md                       # Human-readable changelog
      notifications/
        notification-preferences.spec.yaml
        CHANGELOG.md
    migrations/
      user-profile-v1-to-v2.migration.yaml
      notification-prefs-v1-to-v2.migration.yaml
    schemas/
      spec-schema.yaml                     # The meta-schema all specs follow
      migration-schema.yaml                # Schema for migration specs
    constraints/
      backward-compatibility.constraint.yaml
  tests/
    spec-validation/
      backward-compat.test.ts              # Automated BC checks
      schema-conformance.test.ts           # All specs match schema
```

### Branching Strategy for Spec Changes

```
main
  |
  |--- feature/add-slack-channel
  |      |
  |      |-- specs/features/notifications/notification-preferences.spec.yaml  (v2.0 -> v2.1)
  |      |-- src/features/notifications/slack-channel.ts                       (new)
  |      |-- tests/features/notifications/slack-channel.test.ts               (new)
  |      |
  |      PR: "Add Slack notification channel (spec v2.1.0)"
  |      CI: runs spec schema validation, backward compat check
  |
  |--- feature/i18n-profiles
         |
         |-- specs/features/user-profile/user-profile.spec.yaml              (v1.1 -> v2.0)
         |-- specs/migrations/user-profile-v1-to-v2.migration.yaml           (new)
         |-- src/features/user-profile/...                                    (updated)
         |-- src/migrations/user-profile-v1-to-v2.ts                         (new)
         |-- tests/migrations/user-profile-v1-to-v2.test.ts                  (new)
         |
         PR: "Internationalize user profiles (spec v2.0.0, migration included)"
         CI: runs spec validation + migration spec validation + BC violation check (expected: MAJOR)
```

### CI Checks for Spec Changes

```yaml
# .github/workflows/spec-validation.yml

name: Spec Validation

on:
  pull_request:
    paths:
      - 'specs/**'

jobs:
  validate-specs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Need full history for diff

      - name: Install dependencies
        run: npm ci

      - name: Schema Validation
        run: |
          # Validate all specs against the meta-schema
          npx ts-node scripts/validate-spec-schema.ts

      - name: Backward Compatibility Check
        run: |
          # Compare changed specs against their previous versions
          # Uses git diff to find changed spec files
          CHANGED_SPECS=$(git diff --name-only origin/main -- 'specs/features/**/*.spec.yaml')
          for spec in $CHANGED_SPECS; do
            echo "Checking backward compatibility: $spec"
            npx ts-node scripts/check-backward-compat.ts \
              --old "$(git show origin/main:$spec)" \
              --new "$spec"
          done

      - name: Migration Spec Required Check
        run: |
          # If a spec has a major version bump, ensure a migration spec exists
          npx ts-node scripts/check-migration-required.ts

      - name: Changelog Entry Check
        run: |
          # Every spec change must have a changelog entry
          npx ts-node scripts/check-changelog.ts
```

### Handling Spec Merge Conflicts

Spec merge conflicts are particularly dangerous because a bad merge can silently change the contract. Here are strategies:

**Strategy 1: Spec files are atomic.** Treat the entire spec file as a unit. If two branches modify the same spec, the merge requires manual review.

```yaml
# .gitattributes
specs/**/*.spec.yaml merge=spec-merge
```

```bash
# .git/config -- custom merge driver that always conflicts on spec files
[merge "spec-merge"]
  name = Spec merge driver (always conflicts for safety)
  driver = false
```

**Strategy 2: Version number as conflict detector.** If two branches both bump the version, Git will naturally conflict on the version field, forcing manual resolution.

**Strategy 3: Automated merge validation.** After every merge, run the backward compatibility checker to ensure the merged spec is valid.

```python
# scripts/post-merge-spec-check.py
"""
Git post-merge hook that validates all spec files.
"""

import subprocess
import sys
import yaml

def get_changed_specs():
    """Find spec files changed in the merge."""
    result = subprocess.run(
        ["git", "diff", "--name-only", "HEAD~1", "HEAD", "--",
         "specs/features/**/*.spec.yaml"],
        capture_output=True, text=True,
    )
    return result.stdout.strip().split('\n') if result.stdout.strip() else []

def validate_spec(path: str) -> list[str]:
    """Validate a spec file for schema conformance and internal consistency."""
    errors = []
    with open(path) as f:
        spec = yaml.safe_load(f)

    # Check version is present
    if 'spec_version' not in spec:
        errors.append(f"{path}: missing spec_version")

    # Check changelog matches version
    if 'changelog' in spec:
        latest_changelog = spec['changelog'][0]
        if latest_changelog.get('version') != spec.get('spec_version'):
            errors.append(
                f"{path}: changelog latest version "
                f"({latest_changelog.get('version')}) does not match "
                f"spec_version ({spec.get('spec_version')})"
            )

    return errors

def main():
    changed_specs = get_changed_specs()
    if not changed_specs:
        return 0

    all_errors = []
    for spec_path in changed_specs:
        errors = validate_spec(spec_path)
        all_errors.extend(errors)

    if all_errors:
        print("SPEC VALIDATION ERRORS:")
        for error in all_errors:
            print(f"  - {error}")
        return 1

    print(f"All {len(changed_specs)} changed specs validated successfully.")
    return 0

if __name__ == '__main__':
    sys.exit(main())
```

---

## 2.14 Key Takeaways

1. **Every spec gets a semantic version.** MAJOR for breaking changes, MINOR for additive changes, PATCH for clarifications. No exceptions.

2. **Additive changes are safe; breaking changes are dangerous.** Learn to distinguish them instinctively. When in doubt, it is breaking.

3. **Deprecation before removal.** Never remove a spec feature without a deprecation period. Mark it deprecated, provide the migration path, give consumers time to adapt.

4. **Migration specs are first-class artifacts.** When a breaking change is needed, the migration spec is as important as the new version of the feature spec.

5. **The changelog is mandatory.** Every change, every version, every reason. The changelog is how agents and humans understand the history of a spec.

6. **Backward compatibility is a testable constraint.** Automate it. Put it in CI. Make it impossible to accidentally ship a breaking change in a minor version.

7. **Git strategies matter.** Spec files need careful merge conflict handling, CI validation, and branch management.

8. **AI model versions are spec dependencies.** When the model powering your agents changes, treat it with the same rigor as any other spec evolution.

> **Professor's aside:** The companies that do this well -- Google, Stripe, Anthropic -- have one thing in common: they invest heavily in the tooling and processes around spec evolution. It is not glamorous work. It is not the kind of thing that makes headlines. But it is the difference between a system that degrades gracefully over years of change and one that collapses under the weight of its own history.

---

### Exercises

**Exercise 1:** Take the user profile spec v2.0.0 above and write a v2.1.0 that adds a "pronouns" field. Determine whether this is a MINOR or MAJOR change and justify your answer. Write the changelog entry.

**Exercise 2:** Write a backward compatibility checker test that validates the transition from user profile v1.0.0 to v1.1.0 passes (no violations), and from v1.1.0 to v2.0.0 correctly identifies the breaking changes.

**Exercise 3:** Design a "canary deployment" pattern for spec changes: how would you roll out a v2.0.0 spec to 5% of traffic first, validate it works, and then gradually increase to 100%? Write the spec for this rollout process itself.

---

*Next chapter: "Environment-Aware Specs" -- writing specifications that account for where and how the software will be deployed.*

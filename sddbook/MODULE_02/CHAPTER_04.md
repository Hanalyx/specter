# Chapter 4: State Management Specs

## MODULE 02 — Defining the Architecture (The "How") | Intermediate Level

---

## Lecture Preamble

*The professor draws a diagram on the whiteboard: a user clicks a button, an API call fires, a loading spinner appears, data arrives, the UI updates, a notification pops up. Arrows connect these events in a web of dependencies.*

Look at this diagram. It is a simplified version of what happens when a user clicks "Assign task to me" in our task management app. A single click triggers a cascade: an optimistic UI update, an API call, a loading state, a success or failure response, a notification, a cache invalidation, and possibly a real-time update to other users viewing the same task.

If you do not spec this cascade before you build it, chaos ensues. The optimistic update shows the wrong user. The loading spinner appears in the wrong place. The error state does not revert the optimistic update. The cache invalidation misses a related query. The real-time update conflicts with the local state.

This is the state management problem, and it is the last piece of our architectural specification puzzle. In this chapter, you will learn how to **specify the flow of data through your application** — the stores, actions, selectors, and side effects that turn user intentions into UI changes.

When you hand an AI a state management spec, you are telling it exactly how every piece of data moves, transforms, and triggers reactions in your app. Without this spec, the AI will invent its own state architecture — and it will be different in every component.

---

## 4.1 Why State Management Needs a Spec

State management is the most commonly "figured out as we go" part of application development. Teams start with a few useState hooks, then add a context provider, then realize they need a global store, then bolt on a caching layer, then add optimistic updates, and before long they have a Frankenstein's monster of state scattered across five different mechanisms.

AI-assisted development makes this worse, not better. Each time you prompt an AI to build a feature, it chooses whatever state management approach seems reasonable for that feature in isolation. Over ten features, you end up with ten different state patterns.

The solution is the same as it was for schemas, components, and APIs: **spec it first.**

A state management spec defines:

1. **Store shape** — what data is stored and in what structure
2. **Actions** — what operations can modify the state
3. **Selectors** — how components read derived data from the store
4. **Side effects** — what happens outside the store when actions are dispatched
5. **Data flow** — how API responses, user events, and real-time updates interact

### The Redux Insight

Even if you never use Redux, its core insight is foundational to state management specification: **state changes are described by actions, and actions are contracts.**

When Redux introduced the concept of "actions" in 2015, it was really introducing the concept of **state change specifications**. An action says: "This specific thing happened, and here is the data associated with it." A reducer says: "Given the current state and this action, here is the new state."

This is pure specification thinking:

```typescript
// This is a SPEC, not just code:
// "When a task is assigned, these are the inputs and this is the result"

type Action =
  | { type: "TASK_ASSIGNED"; taskId: string; assigneeId: string }
  | { type: "TASK_UNASSIGNED"; taskId: string }
  | { type: "TASK_STATUS_CHANGED"; taskId: string; newStatus: TaskStatus }
  | { type: "TASK_CREATED"; task: Task }
  | { type: "TASK_DELETED"; taskId: string };

// The reducer is a STATE TRANSITION FUNCTION — a formalization of "given this state and this event, produce this state"
function taskReducer(state: TaskState, action: Action): TaskState {
  switch (action.type) {
    case "TASK_ASSIGNED":
      // Spec: set task.assigneeId to the new value; update task.updatedAt
      return {
        ...state,
        tasks: state.tasks.map((t) =>
          t.id === action.taskId
            ? { ...t, assigneeId: action.assigneeId, updatedAt: new Date().toISOString() }
            : t
        ),
      };
    // ... other cases
  }
}
```

> **Professor's Aside:** I want to be clear: I am not telling you to use Redux. Redux has its place, but modern React applications have many options — Zustand, Jotai, Valtio, MobX, React Context, and the increasingly powerful combination of React Server Components with server state. What I am telling you is that Redux's *conceptual model* — state + action = new state — is the right way to *spec* state management, regardless of which library you actually use.

---

## 4.2 Defining Store Shapes

The first step in a state management spec is defining the shape of your stores. A store shape is a schema (Chapter 1) applied to application state.

### Principle: Normalize Your State

Before we write store shapes, let us establish a critical principle: **normalize your state**. Just as a relational database normalizes data to avoid duplication, your store should normalize entities.

```typescript
// BAD: Denormalized (nested) state
interface BadTaskState {
  projects: Array<{
    id: string;
    name: string;
    tasks: Array<{           // Tasks are nested inside projects
      id: string;
      title: string;
      assignee: {            // User data is duplicated in every task
        id: string;
        name: string;
        email: string;
      } | null;
      comments: Array<{     // Comments nested inside tasks
        id: string;
        content: string;
        author: {            // User data duplicated AGAIN
          id: string;
          name: string;
        };
      }>;
    }>;
  }>;
}
// Problem: If a user changes their name, you must update it
// in every task and every comment where they appear.
```

```typescript
// GOOD: Normalized state
interface NormalizedState {
  // Each entity type gets its own flat collection
  users: {
    byId: Record<string, User>;
    allIds: string[];
  };
  projects: {
    byId: Record<string, Project>;
    allIds: string[];
  };
  tasks: {
    byId: Record<string, Task>;
    // Index by project for fast lookups
    idsByProject: Record<string, string[]>;
  };
  comments: {
    byId: Record<string, Comment>;
    // Index by task for fast lookups
    idsByTask: Record<string, string[]>;
  };
}
```

### Complete Store Shape Specification

Here is a complete store shape spec for our task management application:

```typescript
// ===================================================
// TASK MANAGEMENT APP — STATE MANAGEMENT SPECIFICATION
// ===================================================

// --- Root Store Shape ---

interface RootStore {
  auth: AuthStore;
  workspace: WorkspaceStore;
  projects: ProjectsStore;
  tasks: TasksStore;
  ui: UIStore;
}

// --- Auth Store ---

interface AuthStore {
  /** The authenticated user, or null if not logged in */
  currentUser: User | null;

  /** Authentication status */
  status: "idle" | "authenticating" | "authenticated" | "error";

  /** Auth error message, if any */
  error: string | null;

  /** JWT access token (stored in memory, NOT localStorage) */
  accessToken: string | null;

  /** When the access token expires (for proactive refresh) */
  tokenExpiresAt: number | null;  // Unix timestamp
}

// --- Workspace Store ---

interface WorkspaceStore {
  /** The active workspace */
  activeWorkspace: Workspace | null;

  /** All workspaces the user belongs to */
  workspaces: Workspace[];

  /** Members of the active workspace */
  members: {
    byId: Record<string, WorkspaceMember>;
    allIds: string[];
    loading: boolean;
    error: string | null;
  };

  /** Labels in the active workspace */
  labels: {
    byId: Record<string, Label>;
    allIds: string[];
    loading: boolean;
    error: string | null;
  };
}

// --- Projects Store ---

interface ProjectsStore {
  /** Normalized project entities */
  byId: Record<string, Project>;

  /** Ordered list of project IDs */
  allIds: string[];

  /** Currently selected project ID */
  activeProjectId: string | null;

  /** Loading state for the project list */
  loading: boolean;

  /** Error from the last project operation */
  error: string | null;
}

// --- Tasks Store ---

interface TasksStore {
  /** Normalized task entities */
  byId: Record<string, Task>;

  /** Task IDs indexed by project */
  idsByProject: Record<string, string[]>;

  /** Task IDs indexed by current view/filter.
   *  This is the "active query result" — the task IDs
   *  that match the current filter/sort/search criteria. */
  activeQueryResult: {
    taskIds: string[];
    totalCount: number;
    nextCursor: string | null;
    loading: boolean;
    error: string | null;
  };

  /** Active filters for the task list */
  filters: TaskFilters;

  /** Sort configuration */
  sort: {
    field: "created_at" | "updated_at" | "priority" | "due_date";
    order: "asc" | "desc";
  };

  /** Optimistic updates in flight.
   *  Maps taskId -> the pending changes.
   *  Used to revert on failure. */
  optimisticUpdates: Record<string, {
    previousState: Task;
    pendingAction: string;
    timestamp: number;
  }>;

  /** Task currently being edited (for detail view) */
  activeTaskId: string | null;
}

interface TaskFilters {
  status: TaskStatus[] | null;        // null = all statuses
  priority: TaskPriority[] | null;    // null = all priorities
  assigneeId: string | null;          // null = all assignees
  labelIds: string[] | null;          // null = all labels
  search: string;                     // empty string = no search
  dueBefore: string | null;           // YYYY-MM-DD
  dueAfter: string | null;            // YYYY-MM-DD
}

// --- UI Store ---

interface UIStore {
  /** Sidebar state */
  sidebar: {
    isOpen: boolean;
    width: number;         // In pixels, for resize support
  };

  /** Modal state */
  modal: {
    type: ModalType | null;
    props: Record<string, unknown>;
  };

  /** Command palette state */
  commandPalette: {
    isOpen: boolean;
    query: string;
  };

  /** Toast notifications */
  toasts: Toast[];

  /** Theme preference */
  theme: "light" | "dark" | "system";

  /** Whether the app is in offline mode */
  isOffline: boolean;
}

type ModalType =
  | "createTask"
  | "editTask"
  | "deleteTask"
  | "createProject"
  | "editProject"
  | "inviteMember"
  | "settings";

interface Toast {
  id: string;
  type: "success" | "error" | "warning" | "info";
  title: string;
  message?: string;
  duration: number;        // Milliseconds, 0 = persistent
  action?: {
    label: string;
    onClick: () => void;
  };
  createdAt: number;       // Unix timestamp
}
```

> **Professor's Aside:** Look at the `optimisticUpdates` field in the Tasks store. That is one of the most important design decisions in any state management spec. Optimistic updates make the UI feel instant — the user sees the change immediately, and the API call happens in the background. But if the API call fails, you need to revert to the previous state. By including `previousState` in the spec, we ensure the AI implements proper rollback. Without this in the spec, nine out of ten AI implementations will optimistically update but never revert on failure.

---

## 4.3 Defining Actions

Actions are the events that cause state changes. In a state management spec, every action must define:

1. **A name** — what happened
2. **A payload** — the data associated with the event
3. **The state changes** — exactly what fields change and how
4. **Side effects** — any external operations triggered by the action

### Action Specification Format

```typescript
// ===================================================
// TASK ACTIONS — COMPLETE SPECIFICATION
// ===================================================

// --- Action Types ---

type TaskAction =
  // --- CRUD Actions ---
  | { type: "tasks/fetchList"; payload: FetchTasksPayload }
  | { type: "tasks/fetchListSuccess"; payload: FetchTasksSuccessPayload }
  | { type: "tasks/fetchListError"; payload: { error: string } }

  | { type: "tasks/fetchOne"; payload: { taskId: string } }
  | { type: "tasks/fetchOneSuccess"; payload: { task: Task } }
  | { type: "tasks/fetchOneError"; payload: { taskId: string; error: string } }

  | { type: "tasks/create"; payload: CreateTaskPayload }
  | { type: "tasks/createSuccess"; payload: { task: Task } }
  | { type: "tasks/createError"; payload: { error: string; tempId: string } }

  | { type: "tasks/update"; payload: UpdateTaskPayload }
  | { type: "tasks/updateSuccess"; payload: { task: Task } }
  | { type: "tasks/updateError"; payload: { taskId: string; error: string } }

  | { type: "tasks/delete"; payload: { taskId: string } }
  | { type: "tasks/deleteSuccess"; payload: { taskId: string } }
  | { type: "tasks/deleteError"; payload: { taskId: string; error: string } }

  // --- Filter/Sort Actions ---
  | { type: "tasks/setFilters"; payload: Partial<TaskFilters> }
  | { type: "tasks/clearFilters" }
  | { type: "tasks/setSort"; payload: { field: string; order: "asc" | "desc" } }

  // --- Pagination Actions ---
  | { type: "tasks/loadMore" }
  | { type: "tasks/loadMoreSuccess"; payload: FetchTasksSuccessPayload }
  | { type: "tasks/loadMoreError"; payload: { error: string } }

  // --- Selection Actions ---
  | { type: "tasks/setActiveTask"; payload: { taskId: string | null } }

  // --- Optimistic Actions ---
  | { type: "tasks/optimisticUpdate"; payload: { taskId: string; changes: Partial<Task> } }
  | { type: "tasks/revertOptimistic"; payload: { taskId: string } }

  // --- Real-time Actions ---
  | { type: "tasks/realtime/created"; payload: { task: Task } }
  | { type: "tasks/realtime/updated"; payload: { task: Task } }
  | { type: "tasks/realtime/deleted"; payload: { taskId: string } };

// --- Payload Types ---

interface FetchTasksPayload {
  projectId: string;
  filters?: Partial<TaskFilters>;
  sort?: { field: string; order: "asc" | "desc" };
  cursor?: string;
  limit?: number;
}

interface FetchTasksSuccessPayload {
  tasks: Task[];
  nextCursor: string | null;
  totalCount: number;
  append: boolean;  // true for "load more", false for fresh fetch
}

interface CreateTaskPayload {
  projectId: string;
  title: string;
  description?: string;
  priority?: TaskPriority;
  assigneeId?: string | null;
  labelIds?: string[];
  dueDate?: string | null;
  parentTaskId?: string | null;
  tempId: string;  // Temporary ID for optimistic UI
}

interface UpdateTaskPayload {
  taskId: string;
  changes: Partial<{
    title: string;
    description: string;
    status: TaskStatus;
    priority: TaskPriority;
    assigneeId: string | null;
    labelIds: string[];
    dueDate: string | null;
    estimatePoints: number | null;
  }>;
}
```

### Action-by-Action State Change Specification

For each action, document exactly how the state changes:

```typescript
// ===================================================
// STATE CHANGE SPECIFICATIONS
// ===================================================

const stateChangeSpecs = {
  "tasks/fetchList": {
    stateChanges: {
      "activeQueryResult.loading": "set to true",
      "activeQueryResult.error": "set to null",
    },
    sideEffects: [
      "Make GET /api/v1/projects/{projectId}/tasks with current filters and sort",
      "On success: dispatch tasks/fetchListSuccess",
      "On failure: dispatch tasks/fetchListError",
    ],
  },

  "tasks/fetchListSuccess": {
    stateChanges: {
      "byId": "merge tasks into the normalized map (add or update)",
      "idsByProject[projectId]": "update to reflect returned task IDs",
      "activeQueryResult.taskIds": "payload.append ? append : replace",
      "activeQueryResult.totalCount": "set to payload.totalCount",
      "activeQueryResult.nextCursor": "set to payload.nextCursor",
      "activeQueryResult.loading": "set to false",
      "activeQueryResult.error": "set to null",
    },
    sideEffects: [],
  },

  "tasks/fetchListError": {
    stateChanges: {
      "activeQueryResult.loading": "set to false",
      "activeQueryResult.error": "set to payload.error",
    },
    sideEffects: [
      "Dispatch ui/showToast with type 'error' and message from payload",
    ],
  },

  "tasks/create": {
    stateChanges: {
      // Optimistic: add a temporary task to the store immediately
      "byId[tempId]": "create temporary task with tempId, status 'backlog', provided fields",
      "idsByProject[projectId]": "prepend tempId to the list",
      "activeQueryResult.taskIds": "prepend tempId if it matches current filters",
      "activeQueryResult.totalCount": "increment by 1",
    },
    sideEffects: [
      "Make POST /api/v1/projects/{projectId}/tasks",
      "On success: dispatch tasks/createSuccess",
      "On failure: dispatch tasks/createError",
    ],
  },

  "tasks/createSuccess": {
    stateChanges: {
      // Replace temporary task with real task from server
      "byId": "remove entry for tempId; add entry for real task.id",
      "idsByProject[projectId]": "replace tempId with real task.id",
      "activeQueryResult.taskIds": "replace tempId with real task.id",
    },
    sideEffects: [
      "Dispatch ui/showToast with type 'success': 'Task created'",
      "Close 'createTask' modal if open",
    ],
  },

  "tasks/createError": {
    stateChanges: {
      // Remove the optimistic temporary task
      "byId": "remove entry for tempId",
      "idsByProject[projectId]": "remove tempId from list",
      "activeQueryResult.taskIds": "remove tempId from list",
      "activeQueryResult.totalCount": "decrement by 1",
    },
    sideEffects: [
      "Dispatch ui/showToast with type 'error': payload.error",
    ],
  },

  "tasks/update": {
    stateChanges: {
      // Optimistic: apply changes immediately
      "optimisticUpdates[taskId]": "store { previousState: current task, pendingAction: 'update', timestamp: now }",
      "byId[taskId]": "merge payload.changes into existing task; update updatedAt",
    },
    sideEffects: [
      "Make PATCH /api/v1/tasks/{taskId} with payload.changes",
      "On success: dispatch tasks/updateSuccess",
      "On failure: dispatch tasks/updateError",
    ],
  },

  "tasks/updateSuccess": {
    stateChanges: {
      "byId[taskId]": "replace with server response (authoritative state)",
      "optimisticUpdates[taskId]": "remove entry",
    },
    sideEffects: [],
  },

  "tasks/updateError": {
    stateChanges: {
      // Revert to previous state
      "byId[taskId]": "restore from optimisticUpdates[taskId].previousState",
      "optimisticUpdates[taskId]": "remove entry",
    },
    sideEffects: [
      "Dispatch ui/showToast with type 'error': payload.error",
      "Dispatch ui/showToast with action: { label: 'Retry', onClick: re-dispatch original update }",
    ],
  },

  "tasks/setFilters": {
    stateChanges: {
      "filters": "merge payload into existing filters",
      "activeQueryResult.taskIds": "clear (will be repopulated by fetch)",
      "activeQueryResult.nextCursor": "set to null",
    },
    sideEffects: [
      "Debounce 300ms, then dispatch tasks/fetchList with new filters",
    ],
  },

  "tasks/realtime/updated": {
    stateChanges: {
      "byId[task.id]": "update if task.updatedAt > current task.updatedAt (last-write-wins)",
    },
    sideEffects: [
      "If task.id === activeTaskId, check if another user modified fields the current user is editing — if so, show conflict notification",
    ],
    note: "Do NOT apply if there is a pending optimistic update for this task",
  },
};
```

---

## 4.4 Defining Selectors

Selectors are functions that derive data from the store. They answer the question: "What does this component actually need from the state?"

### Why Selectors Need Specs

Without selector specs, every component reaches directly into the store and computes its own derived data. This leads to:

- Duplicated logic across components
- Inconsistent computations (one component filters differently than another)
- Performance issues (recomputing on every render)
- AI confusion (the AI cannot predict what data shape a component expects)

### Selector Specification

```typescript
// ===================================================
// SELECTORS — SPECIFICATION
// ===================================================

/**
 * selectActiveProject
 *
 * Returns: Project | null
 * Depends on: projects.byId, projects.activeProjectId
 * Used by: ProjectHeader, TaskList, TaskCreateModal
 *
 * Logic:
 *   If activeProjectId is null, return null.
 *   Otherwise, return projects.byId[activeProjectId].
 *   If the project is not in the store (should not happen), return null.
 */

/**
 * selectFilteredTasks
 *
 * Returns: Task[]
 * Depends on: tasks.byId, tasks.activeQueryResult.taskIds
 * Used by: TaskList, TaskBoard
 *
 * Logic:
 *   Map activeQueryResult.taskIds to Task objects from tasks.byId.
 *   Filter out any IDs that no longer exist in byId
 *   (handles race conditions with deletions).
 *   Apply sort from tasks.sort (client-side re-sort of server results).
 *
 * Memoization: Memoize on taskIds array reference and byId reference.
 */

/**
 * selectTaskById
 *
 * Input: taskId: string
 * Returns: Task | undefined
 * Depends on: tasks.byId
 * Used by: TaskDetailView, TaskCard, TaskRow
 *
 * Logic: Direct lookup: tasks.byId[taskId]
 * Memoization: Memoize per taskId (returns same reference if task unchanged).
 */

/**
 * selectTaskCounts
 *
 * Returns: { total: number, byStatus: Record<TaskStatus, number>, byPriority: Record<TaskPriority, number> }
 * Depends on: tasks.byId, tasks.idsByProject, projects.activeProjectId
 * Used by: ProjectSidebar, TaskFilterBar
 *
 * Logic:
 *   Get all task IDs for the active project.
 *   Count tasks by status and priority.
 *   Return the counts object.
 *
 * Memoization: Recompute only when tasks in the active project change.
 */

/**
 * selectOverdueTasks
 *
 * Returns: Task[]
 * Depends on: tasks.byId, tasks.idsByProject, projects.activeProjectId
 * Used by: ProjectDashboard, OverdueWidget
 *
 * Logic:
 *   Get all tasks for the active project.
 *   Filter to tasks where:
 *     - dueDate is not null
 *     - dueDate < today
 *     - status is NOT "done" or "cancelled"
 *   Sort by dueDate ascending (most overdue first).
 *
 * Edge cases:
 *   - "today" is determined by the workspace timezone setting
 *   - Tasks with due date of today are NOT overdue (they are due today)
 */

/**
 * selectAssigneesForActiveProject
 *
 * Returns: Array<{ user: User, taskCount: number }>
 * Depends on: workspace.members, tasks.byId, tasks.idsByProject
 * Used by: AssigneeFilter, TaskAssigneeDropdown
 *
 * Logic:
 *   Get all members of the workspace.
 *   For each member, count tasks assigned to them in the active project.
 *   Sort by task count descending (busiest first).
 *   Include members with 0 tasks.
 */

/**
 * selectIsTaskListLoading
 *
 * Returns: boolean
 * Depends on: tasks.activeQueryResult.loading
 * Used by: TaskList, TaskBoard
 *
 * Note: Simple selector, but specified to ensure consistent loading
 * behavior across views that show the same data.
 */

/**
 * selectCanLoadMore
 *
 * Returns: boolean
 * Depends on: tasks.activeQueryResult.nextCursor, tasks.activeQueryResult.loading
 * Used by: TaskList (infinite scroll trigger)
 *
 * Logic: nextCursor is not null AND not currently loading.
 */

/**
 * selectHasOptimisticUpdates
 *
 * Input: taskId: string
 * Returns: boolean
 * Depends on: tasks.optimisticUpdates
 * Used by: TaskCard (to show "saving..." indicator)
 *
 * Logic: taskId exists in optimisticUpdates map.
 */
```

### Selector Implementation Example (Zustand)

Here is how these selector specs translate to a Zustand implementation:

```typescript
import { create } from "zustand";
import { createSelector } from "reselect";

// The store (simplified)
const useTaskStore = create<TasksStore>((set, get) => ({
  byId: {},
  idsByProject: {},
  activeQueryResult: {
    taskIds: [],
    totalCount: 0,
    nextCursor: null,
    loading: false,
    error: null,
  },
  filters: {
    status: null,
    priority: null,
    assigneeId: null,
    labelIds: null,
    search: "",
    dueBefore: null,
    dueAfter: null,
  },
  sort: { field: "created_at", order: "desc" },
  optimisticUpdates: {},
  activeTaskId: null,

  // Actions (implemented per action specs above)
  // ...
}));

// Selectors (implemented per selector specs above)

const selectFilteredTasks = createSelector(
  (state: TasksStore) => state.activeQueryResult.taskIds,
  (state: TasksStore) => state.byId,
  (taskIds, byId) =>
    taskIds
      .map((id) => byId[id])
      .filter((task): task is Task => task !== undefined)
);

const selectTaskById = (taskId: string) =>
  createSelector(
    (state: TasksStore) => state.byId,
    (byId) => byId[taskId]
  );

const selectCanLoadMore = createSelector(
  (state: TasksStore) => state.activeQueryResult.nextCursor,
  (state: TasksStore) => state.activeQueryResult.loading,
  (nextCursor, loading) => nextCursor !== null && !loading
);
```

---

## 4.5 Server State vs. Client State

One of the most important distinctions in modern frontend architecture is between **server state** and **client state**. This distinction should be explicit in your state management spec.

### What Is Server State?

Server state is data that:
- Originates from and is owned by the server
- Can be stale (another user or process may have changed it)
- Needs to be fetched, cached, and synchronized
- Is shared between multiple clients

Examples: user data, tasks, projects, comments, notifications.

### What Is Client State?

Client state is data that:
- Originates from and is owned by the client
- Is always current (by definition)
- Does not need synchronization
- Is specific to this user's session

Examples: sidebar open/closed, current filters, modal visibility, form input values, theme preference.

### Specifying the Boundary

In your state management spec, explicitly categorize every piece of state:

```typescript
// ===================================================
// STATE OWNERSHIP SPECIFICATION
// ===================================================

/**
 * SERVER STATE (managed by TanStack Query / React Query)
 *
 * These are fetched from the API, cached, and automatically
 * refetched when stale. The query cache is the source of truth.
 *
 * Entity queries:
 *   - tasks: GET /api/v1/projects/{projectId}/tasks
 *   - task detail: GET /api/v1/tasks/{taskId}
 *   - projects: GET /api/v1/workspaces/{workspaceId}/projects
 *   - workspace members: GET /api/v1/workspaces/{workspaceId}/members
 *   - labels: GET /api/v1/workspaces/{workspaceId}/labels
 *   - comments: GET /api/v1/tasks/{taskId}/comments
 *   - notifications: GET /api/v1/notifications
 *   - current user: GET /api/v1/users/me
 *
 * Cache configuration:
 *   - staleTime: 30 seconds (data is considered fresh for 30s)
 *   - gcTime: 5 minutes (unused cache entries removed after 5min)
 *   - refetchOnWindowFocus: true (refetch when user returns to tab)
 *   - refetchOnReconnect: true (refetch when network reconnects)
 *
 * Mutations:
 *   - All write operations (create, update, delete) go through mutations
 *   - Mutations use optimistic updates (see section 4.3)
 *   - On mutation success: invalidate related queries
 *   - On mutation failure: revert optimistic update + show error toast
 */

/**
 * CLIENT STATE (managed by Zustand)
 *
 * This state exists only in the client and is not persisted
 * to the server (unless explicitly noted).
 *
 * UI state:
 *   - sidebar.isOpen: boolean (persisted to localStorage)
 *   - sidebar.width: number (persisted to localStorage)
 *   - theme: string (persisted to localStorage)
 *   - commandPalette.isOpen: boolean
 *   - commandPalette.query: string
 *   - modal.type: ModalType | null
 *   - modal.props: Record<string, unknown>
 *   - toasts: Toast[]
 *
 * Navigation state:
 *   - activeProjectId: string | null (derived from URL params)
 *   - activeTaskId: string | null (derived from URL params)
 *
 * Filter/sort state:
 *   - filters: TaskFilters (persisted to URL query params)
 *   - sort: { field, order } (persisted to URL query params)
 *
 * Ephemeral state:
 *   - isOffline: boolean (derived from navigator.onLine)
 *   - optimisticUpdates: Record<string, ...> (cleared on page reload)
 */
```

### TanStack Query / React Query Pattern

Here is how the server state spec translates to TanStack Query:

```typescript
// ===================================================
// SERVER STATE — TANSTACK QUERY SPECIFICATION
// ===================================================

// --- Query Keys ---
// Every query key is a tuple that uniquely identifies the data.
// This spec prevents key collisions and enables precise cache invalidation.

const queryKeys = {
  tasks: {
    all: ["tasks"] as const,
    lists: () => [...queryKeys.tasks.all, "list"] as const,
    list: (projectId: string, filters: TaskFilters, sort: SortConfig) =>
      [...queryKeys.tasks.lists(), projectId, filters, sort] as const,
    details: () => [...queryKeys.tasks.all, "detail"] as const,
    detail: (taskId: string) =>
      [...queryKeys.tasks.details(), taskId] as const,
  },

  projects: {
    all: ["projects"] as const,
    lists: () => [...queryKeys.projects.all, "list"] as const,
    list: (workspaceId: string) =>
      [...queryKeys.projects.lists(), workspaceId] as const,
    details: () => [...queryKeys.projects.all, "detail"] as const,
    detail: (projectId: string) =>
      [...queryKeys.projects.details(), projectId] as const,
  },

  comments: {
    all: ["comments"] as const,
    list: (taskId: string) =>
      [...queryKeys.comments.all, "list", taskId] as const,
  },

  notifications: {
    all: ["notifications"] as const,
    list: () => [...queryKeys.notifications.all, "list"] as const,
    unreadCount: () =>
      [...queryKeys.notifications.all, "unread-count"] as const,
  },
};

// --- Cache Invalidation Rules ---
// When a mutation succeeds, these queries must be invalidated.

const invalidationRules = {
  "tasks/create": [
    "queryKeys.tasks.lists()",    // Refetch task list
    "queryKeys.tasks.all",        // Invalidate all task queries
  ],

  "tasks/update": (taskId: string) => [
    "queryKeys.tasks.detail(taskId)",  // Refetch updated task
    "queryKeys.tasks.lists()",          // Refetch task list (status/priority may have changed filter results)
  ],

  "tasks/delete": (taskId: string) => [
    "queryKeys.tasks.detail(taskId)",  // Remove from cache
    "queryKeys.tasks.lists()",          // Refetch task list
    "queryKeys.comments.list(taskId)", // Remove associated comments from cache
  ],

  "comments/create": (taskId: string) => [
    "queryKeys.comments.list(taskId)",  // Refetch comment list
    "queryKeys.tasks.detail(taskId)",   // Refetch task (commentCount changed)
  ],
};

// --- Optimistic Update Spec for Mutations ---

/**
 * updateTask mutation — optimistic update specification
 *
 * BEFORE API call:
 *   1. Cancel any in-flight queries for queryKeys.tasks.detail(taskId)
 *   2. Snapshot current task data from cache
 *   3. Optimistically update cache with new values
 *   4. Return snapshot for rollback
 *
 * ON SUCCESS:
 *   1. Replace cached task with server response (authoritative)
 *   2. Invalidate queryKeys.tasks.lists() (to update list views)
 *
 * ON ERROR:
 *   1. Restore snapshot to cache (rollback)
 *   2. Show error toast
 *
 * ON SETTLE (success or error):
 *   1. Refetch queryKeys.tasks.detail(taskId) to ensure consistency
 */
```

```typescript
// Implementation of the optimistic update spec:

function useUpdateTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: UpdateTaskPayload) =>
      api.patch(`/tasks/${payload.taskId}`, payload.changes),

    onMutate: async (payload) => {
      // Cancel in-flight queries
      await queryClient.cancelQueries({
        queryKey: queryKeys.tasks.detail(payload.taskId),
      });

      // Snapshot previous value
      const previousTask = queryClient.getQueryData<Task>(
        queryKeys.tasks.detail(payload.taskId)
      );

      // Optimistically update
      queryClient.setQueryData(
        queryKeys.tasks.detail(payload.taskId),
        (old: Task | undefined) =>
          old ? { ...old, ...payload.changes, updatedAt: new Date().toISOString() } : old
      );

      return { previousTask };
    },

    onError: (_error, payload, context) => {
      // Rollback
      if (context?.previousTask) {
        queryClient.setQueryData(
          queryKeys.tasks.detail(payload.taskId),
          context.previousTask
        );
      }
    },

    onSettled: (_data, _error, payload) => {
      // Always refetch to ensure consistency
      queryClient.invalidateQueries({
        queryKey: queryKeys.tasks.detail(payload.taskId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.tasks.lists(),
      });
    },
  });
}
```

> **Professor's Aside:** The TanStack Query pattern solves one of the hardest problems in frontend development: keeping server state in sync with the UI. But notice how much specification it requires. The query keys, the cache invalidation rules, the optimistic update lifecycle — all of this needs to be defined before implementation. When you hand an AI "use React Query for data fetching" without specifying cache invalidation rules, the AI will get the happy path right and miss every edge case. When you hand it the full spec, it gets everything right.

---

## 4.6 How to Spec Real-Time Data Flows

Real-time features (WebSocket events, Server-Sent Events) are among the hardest things to spec correctly. They introduce concurrency, ordering, and conflict resolution problems that do not exist in request-response architectures.

### WebSocket Events as Contracts

```typescript
// ===================================================
// REAL-TIME EVENT SPECIFICATION
// ===================================================

// --- Connection Lifecycle ---

interface WebSocketSpec {
  endpoint: "wss://api.example.com/v1/ws";
  authentication: "JWT token sent as query parameter: ?token={jwt}";
  reconnection: {
    strategy: "Exponential backoff with jitter";
    initialDelay: 1000;     // 1 second
    maxDelay: 30000;        // 30 seconds
    maxAttempts: "unlimited (keep trying until connection restored)";
    jitter: "Add random 0-500ms to each delay";
  };
  heartbeat: {
    clientSendsEvery: 30000;  // Client pings every 30 seconds
    serverTimeout: 10000;     // If no pong in 10 seconds, reconnect
  };
  subscriptions: {
    method: "Send subscription messages after connection established";
    format: '{ "type": "subscribe", "channel": "project:{projectId}" }';
    unsubscribe: '{ "type": "unsubscribe", "channel": "project:{projectId}" }';
  };
}

// --- Event Types ---

type ServerEvent =
  | TaskCreatedEvent
  | TaskUpdatedEvent
  | TaskDeletedEvent
  | CommentCreatedEvent
  | UserPresenceEvent
  | TypingIndicatorEvent;

interface TaskCreatedEvent {
  type: "task.created";
  timestamp: string;        // ISO 8601
  channel: string;          // "project:{projectId}"
  payload: {
    task: Task;             // Full task object
    actorId: string;        // Who created it
  };
}

interface TaskUpdatedEvent {
  type: "task.updated";
  timestamp: string;
  channel: string;
  payload: {
    task: Task;             // Full updated task
    actorId: string;        // Who updated it
    changedFields: string[];// Which fields changed: ["status", "assigneeId"]
  };
}

interface TaskDeletedEvent {
  type: "task.deleted";
  timestamp: string;
  channel: string;
  payload: {
    taskId: string;
    actorId: string;
  };
}

interface CommentCreatedEvent {
  type: "comment.created";
  timestamp: string;
  channel: string;
  payload: {
    comment: Comment;
    taskId: string;
    actorId: string;
  };
}

interface UserPresenceEvent {
  type: "presence.update";
  timestamp: string;
  channel: string;
  payload: {
    userId: string;
    status: "online" | "away" | "offline";
    lastActiveAt: string;
    currentView: {
      type: "task" | "project" | "dashboard";
      entityId: string | null;
    } | null;
  };
}

interface TypingIndicatorEvent {
  type: "typing.indicator";
  timestamp: string;
  channel: string;
  payload: {
    userId: string;
    taskId: string;
    field: "title" | "description" | "comment";
    isTyping: boolean;
  };
}

// --- Event Handling Specification ---

interface EventHandlingSpec {
  "task.created": {
    action: "Add task to local cache if it belongs to the active project";
    conflict: "Check if task with this ID already exists (from optimistic create)";
    conflictResolution: "Server version wins — replace optimistic version";
    uiUpdate: "Append to task list if matches current filters; update totalCount";
    notification: "Show toast 'New task: {title}' if actorId !== currentUser.id";
    selfEvent: "If actorId === currentUser.id, ignore (already handled optimistically)";
  };

  "task.updated": {
    action: "Update task in local cache";
    conflict: "Check if we have a pending optimistic update for this task";
    conflictResolution: {
      noPendingUpdate: "Apply server version directly";
      pendingUpdateSameFields: "Server wins — discard optimistic, apply server version";
      pendingUpdateDifferentFields: "Merge — keep optimistic fields, apply server's other fields";
    };
    uiUpdate: "Re-evaluate filter membership (status change might add/remove from list)";
    notification: "Show toast if assigneeId changed to currentUser.id: 'You were assigned to {title}'";
    selfEvent: "If actorId === currentUser.id, only apply if server version is newer than our optimistic";
  };

  "task.deleted": {
    action: "Remove task from local cache";
    conflict: "If task is currently being edited, show 'This task was deleted' warning";
    uiUpdate: "Remove from task list; decrement totalCount";
    notification: "Show toast if task was assigned to currentUser: 'Task {displayId} was deleted'";
  };

  "presence.update": {
    action: "Update user presence in workspace members store";
    uiUpdate: "Update avatar indicators (green dot = online, yellow = away)";
    debounce: "Batch presence updates — process at most once per second";
  };

  "typing.indicator": {
    action: "Update typing indicators for the specified task/field";
    uiUpdate: "Show '{userName} is typing...' below the relevant field";
    timeout: "Clear typing indicator after 5 seconds of no typing events";
    selfEvent: "Ignore typing events from currentUser";
  };
}
```

### Conflict Resolution Spec

The most critical part of real-time specs is conflict resolution. When a user makes a change locally and a real-time update arrives for the same entity, what happens?

```typescript
// ===================================================
// CONFLICT RESOLUTION SPECIFICATION
// ===================================================

interface ConflictResolutionSpec {
  strategy: "Last Write Wins with Optimistic Priority";

  rules: {
    rule1: {
      name: "Self-events are ignored";
      description: "If event.actorId === currentUser.id, the event is from our own action. We already have the optimistic state; ignore the echo.";
      exception: "Unless we received an error for our action — then the server event takes precedence.";
    };

    rule2: {
      name: "No pending optimistic update";
      description: "If there is no pending optimistic update for this entity, apply the server event directly.";
    };

    rule3: {
      name: "Optimistic update is pending, same fields changed";
      description: "If the server event changes the same fields as our pending optimistic update, this likely means another user modified the same thing. Show a conflict notification: 'Someone else changed the status of this task. Your change may be overwritten.'";
      resolution: "Keep our optimistic state for now. When our API response arrives, it will be the final arbiter.";
    };

    rule4: {
      name: "Optimistic update is pending, different fields changed";
      description: "If the server event changes different fields than our optimistic update, merge them. Example: we changed the title (optimistic), and someone else changed the status (server event). Apply the status change without reverting our title change.";
    };

    rule5: {
      name: "Stale event detection";
      description: "Compare event.timestamp with the entity's updatedAt in our cache. If the event is older than our cached version, discard it.";
    };
  };

  edgeCases: {
    "rapid successive updates": "Buffer events for 100ms and process only the latest per entity";
    "reconnection flood": "After reconnection, request full state refresh for subscribed channels instead of replaying missed events";
    "out of order events": "Always compare timestamps; never assume events arrive in order";
    "duplicate events": "Deduplicate by event ID (each event must have a unique ID)";
  };
}
```

---

## 4.7 The Relationship Between API Specs and State Management Specs

API specs (Chapter 3) and state management specs are two views of the same data flow. Here is how they connect:

```
User Action
    |
    v
[State Management: Dispatch Action]
    |
    ├── Optimistic Update (immediate)
    |       |
    |       v
    |   [Store Updated] --> [UI Re-renders]
    |
    └── Side Effect: API Call
            |
            v
        [API Blueprint: Request]
            |
            v
        [Server Processes]
            |
            v
        [API Blueprint: Response]
            |
            ├── Success
            |       |
            |       v
            |   [State Management: Success Action]
            |       |
            |       v
            |   [Store Updated with Server Data] --> [UI Re-renders]
            |   [Cache Invalidation]
            |
            └── Error
                    |
                    v
                [State Management: Error Action]
                    |
                    v
                [Revert Optimistic Update] --> [UI Re-renders]
                [Show Error Toast]
```

### Tracing a Flow: "Assign Task to Me"

Let us trace a complete flow through all our specs:

```
1. USER clicks "Assign to me" button on TaskCard

2. COMPONENT CONTRACT (Chapter 2)
   - TaskCard.props.onAssign(taskId, currentUser.id)
   - Button shows loading state

3. STATE MANAGEMENT (this chapter)
   - Dispatch: { type: "tasks/update", payload: { taskId, changes: { assigneeId: currentUser.id } } }
   - Optimistic update: task.assigneeId = currentUser.id in store
   - UI immediately shows current user as assignee

4. API BLUEPRINT (Chapter 3)
   - PATCH /api/v1/tasks/{taskId}
   - Body: { "assigneeId": "current-user-uuid" }
   - Headers: Authorization: Bearer {token}

5a. SUCCESS PATH
   - API returns 200 with updated Task
   - Dispatch: { type: "tasks/updateSuccess", payload: { task: serverTask } }
   - Store updates with server response (authoritative)
   - Cache invalidation: tasks.detail(taskId), tasks.lists()
   - UI shows confirmed assignment

5b. ERROR PATH
   - API returns 422: "User is not a member of this project"
   - Dispatch: { type: "tasks/updateError", payload: { taskId, error: "..." } }
   - Store reverts to previous assigneeId
   - Toast: "Could not assign task: User is not a member of this project"
   - Button returns to original state

6. REAL-TIME (this chapter)
   - Server broadcasts: { type: "task.updated", payload: { task, changedFields: ["assigneeId"] } }
   - Other users viewing the same task see the assignment change
   - If actorId === currentUser.id, event is ignored (already handled optimistically)
```

That is one user action. When you trace it through all four spec layers (schema, component, API, state), you see exactly how data flows from click to final UI state. There is no ambiguity for a human developer or an AI.

---

## 4.8 Zustand Store Specification: A Complete Example

Let us bring everything together with a concrete Zustand store specification that an AI can implement directly.

```typescript
// ===================================================
// ZUSTAND STORE SPECIFICATION — TASK MANAGEMENT
// ===================================================

/**
 * STORE: useTaskStore
 *
 * This store manages client-side state for the task management views.
 * Server state (task entities) is managed by TanStack Query.
 * This store handles: UI state, filters, sort, optimistic updates,
 * and coordination between components.
 *
 * PERSISTENCE: None (ephemeral — resets on page reload)
 * DEVTOOLS: Enable Zustand devtools in development mode
 */

interface TaskStore {
  // === STATE ===

  /** Currently active project (derived from URL, set by router) */
  activeProjectId: string | null;

  /** Currently active task (for detail view) */
  activeTaskId: string | null;

  /** Current filter configuration */
  filters: TaskFilters;

  /** Current sort configuration */
  sort: {
    field: "created_at" | "updated_at" | "priority" | "due_date";
    order: "asc" | "desc";
  };

  /** Task IDs selected for bulk operations */
  selectedTaskIds: Set<string>;

  /** Whether bulk selection mode is active */
  isBulkSelectMode: boolean;

  /** Optimistic updates in flight */
  optimisticUpdates: Map<string, {
    previousData: unknown;
    action: string;
    timestamp: number;
  }>;

  // === ACTIONS ===

  /** Set the active project (called by router integration) */
  setActiveProject: (projectId: string | null) => void;

  /** Set the active task (for detail view) */
  setActiveTask: (taskId: string | null) => void;

  /** Update filters (merges with existing filters) */
  setFilters: (filters: Partial<TaskFilters>) => void;

  /** Clear all filters to defaults */
  clearFilters: () => void;

  /** Set sort configuration */
  setSort: (field: string, order: "asc" | "desc") => void;

  /** Toggle selection of a single task */
  toggleTaskSelection: (taskId: string) => void;

  /** Select all tasks in the current view */
  selectAllTasks: (taskIds: string[]) => void;

  /** Clear all selections */
  clearSelection: () => void;

  /** Toggle bulk select mode */
  toggleBulkSelectMode: () => void;

  /** Register an optimistic update */
  registerOptimistic: (entityId: string, previousData: unknown, action: string) => void;

  /** Clear an optimistic update (on success or after revert) */
  clearOptimistic: (entityId: string) => void;

  /** Check if an entity has a pending optimistic update */
  hasOptimisticUpdate: (entityId: string) => boolean;
}

/**
 * STORE: useUIStore
 *
 * This store manages global UI state that is shared across
 * multiple components and is not tied to a specific feature.
 *
 * PERSISTENCE: sidebar and theme are persisted to localStorage
 * DEVTOOLS: Enable Zustand devtools in development mode
 */

interface UIStore {
  // === STATE ===

  sidebar: {
    isOpen: boolean;
    width: number;  // 200-400px, default 260
  };

  modal: {
    type: ModalType | null;
    props: Record<string, unknown>;
  };

  commandPalette: {
    isOpen: boolean;
    query: string;
  };

  toasts: Toast[];

  theme: "light" | "dark" | "system";

  isOffline: boolean;

  // === ACTIONS ===

  /** Toggle sidebar open/closed */
  toggleSidebar: () => void;

  /** Set sidebar width (for drag-resize) */
  setSidebarWidth: (width: number) => void;

  /** Open a modal with props */
  openModal: (type: ModalType, props?: Record<string, unknown>) => void;

  /** Close the current modal */
  closeModal: () => void;

  /** Toggle command palette */
  toggleCommandPalette: () => void;

  /** Set command palette query */
  setCommandPaletteQuery: (query: string) => void;

  /** Show a toast notification */
  showToast: (toast: Omit<Toast, "id" | "createdAt">) => void;

  /** Dismiss a toast notification */
  dismissToast: (toastId: string) => void;

  /** Set theme preference */
  setTheme: (theme: "light" | "dark" | "system") => void;

  /** Set offline status (called by network listener) */
  setOffline: (isOffline: boolean) => void;
}

// === ACTION BEHAVIOR SPECIFICATIONS ===

const actionBehaviors = {
  "useUIStore.showToast": {
    behavior: [
      "Generate a unique ID (uuid or nanoid)",
      "Set createdAt to Date.now()",
      "Add toast to the beginning of the toasts array",
      "If toast.duration > 0, set a timeout to auto-dismiss",
      "Maximum 5 toasts visible at once — if adding a 6th, dismiss the oldest",
    ],
  },

  "useUIStore.openModal": {
    behavior: [
      "If a modal is already open, close it first (no stacking)",
      "Set modal.type and modal.props",
      "Add 'overflow: hidden' to document.body (prevent background scroll)",
      "Trap focus within the modal",
    ],
  },

  "useUIStore.closeModal": {
    behavior: [
      "Set modal.type to null and modal.props to {}",
      "Remove 'overflow: hidden' from document.body",
      "Return focus to the element that was focused before the modal opened",
    ],
  },

  "useTaskStore.setFilters": {
    behavior: [
      "Merge provided filters with existing filters (shallow merge)",
      "Reset selectedTaskIds (filter change invalidates selection)",
      "Serialize filters to URL query parameters",
      "Trigger TanStack Query refetch with new filters (via query key change)",
    ],
    note: "Do NOT debounce filter changes — let TanStack Query handle deduplication",
  },

  "useTaskStore.clearFilters": {
    behavior: [
      "Reset all filter fields to defaults",
      "Clear URL query parameters",
      "Reset selectedTaskIds",
    ],
    defaults: {
      status: null,
      priority: null,
      assigneeId: null,
      labelIds: null,
      search: "",
      dueBefore: null,
      dueAfter: null,
    },
  },
};
```

---

## 4.9 Exercise: Complete State Management Spec for a Chat Application

This is the capstone exercise for Module 2. You will write a complete state management specification for a real-time chat application. This exercise brings together schemas, component contracts, API specs, and state management.

### The Application

You are building a Slack-like chat application with the following features:

1. Multiple channels (public and private)
2. Direct messages between users
3. Message sending with optimistic updates
4. Message editing and deletion
5. Typing indicators
6. Unread message counts
7. Message reactions (emoji reactions)
8. File attachments
9. User presence (online, away, offline)
10. Message search

### Your Task

Write the complete state management spec, including:

#### Part 1: Store Shapes

Define the store shapes for:

- **MessagesStore** — normalized messages with indexes by channel
- **ChannelsStore** — channels with unread counts
- **UsersStore** — users with presence information
- **UIStore** — chat-specific UI state (active channel, message input, scroll position)

For each store, document:
- Every field with its type
- Whether it is server state or client state
- How it is initialized
- How it is persisted (if at all)

#### Part 2: Actions

Define all actions for the chat system:

- Sending a message (with optimistic update)
- Receiving a message (via WebSocket)
- Editing a message
- Deleting a message
- Switching channels
- Marking channel as read
- Adding/removing reactions
- Starting/stopping typing
- Uploading a file attachment

For each action, document:
- The action type and payload
- The state changes (which store fields change and how)
- The side effects (API calls, WebSocket messages, notifications)
- Error handling (what to revert, what to show)

#### Part 3: Selectors

Define selectors for:

- Messages in the active channel (sorted by timestamp)
- Unread count for each channel
- Total unread count across all channels
- Users currently typing in the active channel
- Online users in the active channel
- Search results
- Active channel details

For each selector, document:
- Input dependencies (which store fields)
- Output shape
- Memoization strategy
- Which components use this selector

#### Part 4: Real-Time Events

Define the WebSocket event contracts for:

- `message.created` — new message in a channel
- `message.updated` — message was edited
- `message.deleted` — message was deleted
- `reaction.added` / `reaction.removed` — reaction changes
- `typing.start` / `typing.stop` — typing indicators
- `presence.update` — user presence changes
- `channel.updated` — channel metadata changes

For each event, document:
- The event shape (TypeScript interface)
- How it updates the local state
- Conflict resolution with pending optimistic updates
- Whether it triggers a notification

#### Part 5: Data Flow Diagram

Trace the complete data flow for "User sends a message":

1. User types in the message input and presses Enter
2. Optimistic update adds message to local state
3. API call sends the message to the server
4. Server broadcasts the message via WebSocket
5. Other clients receive and display the message
6. Original client receives the echo and reconciles

Document every state change, every side effect, and every edge case (network failure, duplicate message, out-of-order delivery).

> **Professor's Aside:** This exercise is intentionally demanding. A complete state management spec for a chat application is a real-world task that takes an experienced engineer several hours. But when you hand this spec to an AI, the AI will produce a working chat application with correct real-time behavior, proper optimistic updates, and solid conflict resolution. Without the spec, the AI will produce something that looks like a chat app but breaks under real-world conditions — messages appear twice, typing indicators stick forever, unread counts are wrong after reconnection. The spec is the difference between a demo and a product.

---

## 4.10 Key Takeaways

1. **State management specs** define how data flows through your application: stores, actions, selectors, and side effects.

2. **Redux-style thinking** — state + action = new state — is the right mental model for specifying state changes, regardless of which library you use.

3. **Normalize your state.** Flat, indexed collections prevent data duplication and simplify updates.

4. **Separate server state from client state.** Server state belongs in a query cache (TanStack Query). Client state belongs in a store (Zustand, Jotai, etc.).

5. **Optimistic updates require explicit specs.** Define: what changes immediately, what to revert on failure, and how to reconcile with server responses.

6. **Selectors are part of the spec.** Define what derived data each component needs, how it is computed, and how it is memoized.

7. **Real-time events are contracts.** Define every event type, its payload, how it updates state, and how it resolves conflicts with local state.

8. **API specs and state management specs are two views of the same data flow.** Trace user actions through both to ensure consistency.

9. **Cache invalidation rules must be explicit.** When a mutation succeeds, which queries need to be refetched? This is not a detail to leave to the AI's imagination.

10. **The spec prevents state architecture drift.** Without a spec, each AI-generated feature uses a different state pattern. With a spec, every feature follows the same architecture.

---

## Module 2 Conclusion

You have now completed Module 2: Defining the Architecture. Over four chapters, you have learned to specify the four pillars of application architecture:

| Chapter | Pillar | What It Specifies |
|---------|--------|-------------------|
| 1. Schema-First Design | Data | The shapes and constraints of your data |
| 2. Component Contracts | UI | The behavior of your UI components |
| 3. API Blueprinting | Connections | The request/response contracts between systems |
| 4. State Management Specs | Flow | How data moves through your application |

Together, these four specs give an AI (or a human team) everything needed to build a complete, correct, consistent application. The schema defines what data exists. The component contracts define how users interact with it. The API blueprints define how systems exchange it. The state management spec defines how it flows.

In Module 3, we will move from architecture to process: how to write implementation prompts that leverage these specs, how to review AI-generated code against specs, and how to iterate toward production quality.

---

*End of Chapter 4 — State Management Specs*

*End of Module 2 — Defining the Architecture (The "How")*

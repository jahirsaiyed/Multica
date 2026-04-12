# State Management

## Philosophy

Two clear owners, strict separation:

| What | Owner | Location |
|------|-------|----------|
| Data from the API (issues, users, inbox, agents, etc.) | TanStack Query | `packages/core/api/` hooks |
| Client UI state (selections, filters, drafts, modal state) | Zustand | `packages/core/` stores |

**Never duplicate server data into Zustand.** If it came from the API, it belongs in the Query cache. Copying it into a store creates two sources of truth and they will drift.

---

## TanStack Query (Server State)

### Setup
`CoreProvider` wraps all apps with `QueryProvider`. The `QueryClient` is configured with sensible defaults — no polling, no `staleTime` workarounds. WS events drive freshness via invalidation.

### Cache Key Convention
All workspace-scoped queries include `wsId` in the key:
```ts
// Issues list
queryKey: ['issues', wsId, filters]

// Single issue
queryKey: ['issue', wsId, issueId]

// Agents
queryKey: ['agents', wsId]
```

This makes workspace switching automatic — the cache key changes, new queries fire, old data is not reused.

### WS → Query Invalidation
`packages/core/realtime/use-realtime-sync.ts` listens to all WS events and maps them to invalidations:
```
issue:created  → invalidate ['issues', wsId]
issue:updated  → invalidate ['issue', wsId, id] + list
comment:created → invalidate ['comments', wsId, issueId]
inbox:new      → update inbox cache directly (payload included)
agent:status   → invalidate ['agents', wsId]
```

Events are debounced per-prefix to prevent thundering herd from bulk updates.

### Optimistic Mutations Pattern
```ts
useMutation({
  mutationFn: (data) => api.issues.update(data),
  onMutate: async (data) => {
    await queryClient.cancelQueries({ queryKey: ['issue', wsId, data.id] })
    const prev = queryClient.getQueryData(['issue', wsId, data.id])
    queryClient.setQueryData(['issue', wsId, data.id], (old) => ({ ...old, ...data }))
    return { prev }
  },
  onError: (err, data, ctx) => {
    queryClient.setQueryData(['issue', wsId, data.id], ctx.prev)
  },
  onSettled: () => {
    queryClient.invalidateQueries({ queryKey: ['issue', wsId, data.id] })
  },
})
```

---

## Zustand Stores

All stores live in `packages/core/`. Two patterns are used:

### Pattern 1: Module-Level Singleton (auth, workspace, chat)
Created once at app boot via `CoreProvider`, registered to a module-level variable, accessed via a proxy hook:
```ts
// packages/core/auth/store.ts
let _store: AuthStore | null = null

export function registerAuthStore(store: AuthStore) { _store = store }

export const useAuthStore: UseBoundStore<AuthStore> = new Proxy(
  (() => {}) as unknown as UseBoundStore<AuthStore>,
  {
    apply: (_, __, args) => _store!(...args),
    get: (_, prop) => (_store as any)[prop],
  }
)
```

### Pattern 2: Vanilla Store (my-issues, scope filters)
Created at module level, not a hook — accessed with `useStore(myIssuesViewStore, selector)`:
```ts
export const myIssuesViewStore = createStore<MyIssuesViewState>()(
  persist(...)
)
// Usage:
const viewMode = useStore(myIssuesViewStore, (s) => s.viewMode)
```

---

## Store Reference

### Auth Store (`packages/core/auth/store.ts`)
```ts
State:
  user: User | null
  isLoading: boolean

Actions:
  initialize()                    // hydrate from storage token
  sendCode(email)                 // POST /auth/send-code
  verifyCode(email, code)         // POST /auth/verify-code → JWT
  loginWithGoogle(code, redirectUri)
  logout()
  setUser(user)
```
Persistence: token stored in `StorageAdapter` as `multica_token`.

### Workspace Store (`packages/core/workspace/store.ts`)
```ts
State:
  workspace: Workspace | null
  workspaces: Workspace[]

Actions:
  hydrateWorkspace(list, preferredId)  // called after auth
  switchWorkspace(wsId)                // updates API header, clears cache, rehydrates
  refreshWorkspaces()
  updateWorkspace(ws)
  createWorkspace(data)
  leaveWorkspace(id)
  deleteWorkspace(id)
  clearWorkspace()
```
Side effect of `switchWorkspace`:
1. Sets `X-Workspace-ID` header on the API client
2. Clears TanStack Query cache
3. Re-hydrates workspace-aware Zustand stores (calls registered rehydration callbacks)

### Chat Store (`packages/core/chat/store.ts`)
```ts
State:
  isOpen: boolean
  isFullscreen: boolean
  activeSessionId: string | null
  pendingTaskId: string | null
  selectedAgentId: string | null
  showHistory: boolean
  timelineItems: ChatTimelineItem[]

Actions:
  setOpen(), toggle(), toggleFullscreen()
  setActiveSession(id)
  setPendingTask(id)
  setSelectedAgentId(id)
  setShowHistory(bool)
  addTimelineItem(item)
  clearTimeline()
```

### Issue View Store (`packages/core/issues/stores/view-store.ts`)
```ts
State (persisted, workspace-aware):
  viewMode: "board" | "list"
  statusFilters: IssueStatus[]
  priorityFilters: IssuePriority[]
  assigneeFilters: ActorFilterValue[]   // { type: "member" | "agent", id }
  includeNoAssignee: boolean
  creatorFilters: ActorFilterValue[]
  projectFilters: string[]
  includeNoProject: boolean
  sortBy: "position" | "priority" | "due_date" | "created_at" | "title"
  sortDirection: "asc" | "desc"
  cardProperties: { priority, description, assignee, dueDate }
  listCollapsedStatuses: IssueStatus[]

Actions:
  toggleStatusFilter(status)
  togglePriorityFilter(priority)
  toggleAssigneeFilter(value)
  toggleCreatorFilter(value)
  setViewMode(mode)
  setSortBy(field)
  setSortDirection(dir)
  setCardProperty(key, value)
  toggleCollapseStatus(status)
  clearFilters()
```
Auto-clears filters on workspace switch.

### My Issues View Store (`packages/core/issues/stores/my-issues-view-store.ts`)
Extends IssueViewState with:
```ts
scope: "assigned" | "created" | "agents"
setScope(scope)
```
Vanilla store — use `useStore(myIssuesViewStore, selector)`.

### Issue Draft Store (`packages/core/issues/stores/draft-store.ts`)
```ts
State (persisted):
  draft: {
    title: string
    description: string
    status: IssueStatus
    priority: IssuePriority
    assigneeType?: "member" | "agent"
    assigneeId?: string
    dueDate?: string
  }

Actions:
  setDraft(patch)
  clearDraft()
  hasDraft(): boolean
```

### Issue Selection Store (`packages/core/issues/stores/selection-store.ts`)
```ts
State:
  selectedIds: Set<string>

Actions:
  toggle(id)
  select(ids[])
  deselect(ids[])
  clear()
```

### Issues Scope Store (`packages/core/issues/stores/issues-scope-store.ts`)
```ts
State:
  scope: "all" | "members" | "agents"
Actions:
  setScope(scope)
```

### Recent Issues Store (`packages/core/issues/stores/recent-issues-store.ts`)
```ts
State (persisted):
  items: RecentIssueEntry[]   // max 20, with id, identifier, title, status, visitedAt
Actions:
  recordVisit(entry)
```

### Modal Store (`packages/core/modals/store.ts`)
```ts
State:
  modal: "create-workspace" | "create-issue" | null
  data: Record<string, unknown> | null
Actions:
  open(modal, data?)
  close()
```

### Navigation Store (`packages/core/navigation/store.ts`)
```ts
State (persisted, excludes /login and /pair/):
  lastPath: string
Actions:
  onPathChange(path)
```

---

## Storage Adapters

**Problem:** `packages/core` must not call `localStorage` directly (breaks SSR and Electron).

**Solution:** `StorageAdapter` interface injected into `CoreProvider`:
```ts
interface StorageAdapter {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}
```

- **Web**: `defaultStorage` — SSR-safe wrapper (checks for `window` first)
- **Desktop**: Same adapter works (Electron renderer has `localStorage`)
- **Workspace-aware storage**: Keys are prefixed with `wsId` so stores re-hydrate when workspace switches

---

## Common Footguns

### Unstable selector references
```ts
// BAD — new object on every render → infinite re-renders
const state = useStore((s) => ({ a: s.a, b: s.b }))

// GOOD — select primitives separately
const a = useStore((s) => s.a)
const b = useStore((s) => s.b)

// GOOD — use shallow comparison
const { a, b } = useStore((s) => ({ a: s.a, b: s.b }), shallow)
```

### Hooks and workspace context
```ts
// BAD — only works inside WorkspaceIdProvider
function useMyHook() {
  const wsId = useWorkspaceId()  // throws outside
  ...
}

// GOOD — accepts wsId as param, works everywhere
function useMyHook(wsId: string) {
  ...
}
```

### Persisting server data
```ts
// BAD — copies server data into Zustand
const issueStore = create(persist((set) => ({
  issues: [] as Issue[],
  setIssues: (issues) => set({ issues }),
}), ...))

// GOOD — server data lives in Query cache
const { data: issues } = useQuery({ queryKey: ['issues', wsId], ... })
```

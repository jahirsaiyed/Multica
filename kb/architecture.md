# Architecture

## System Overview

Multica is a Go backend + TypeScript monorepo. The backend serves a REST API and WebSocket connection. Two frontend apps (web and desktop) share almost all business logic via shared packages.

```
┌─────────────────────────────────────────────────────────┐
│                    Client Layer                          │
│  apps/web/ (Next.js 15)   apps/desktop/ (Electron)      │
│       ↓  ↓                       ↓  ↓                   │
│  packages/views/  ←→  packages/core/  ←→  packages/ui/  │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP + WS
┌────────────────────────▼────────────────────────────────┐
│                    Go Backend                            │
│  server/cmd/server  →  Chi router                        │
│  server/internal/handler/  (REST endpoints)              │
│  server/internal/realtime/ (WebSocket hub)               │
│  server/internal/events/   (in-process pub/sub)          │
│  server/pkg/db/            (sqlc + pgx PostgreSQL)       │
└─────────────────────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│               PostgreSQL (pgvector/pg17)                 │
└─────────────────────────────────────────────────────────┘
                         ↑
┌────────────────────────┴────────────────────────────────┐
│                 Local Daemon (CLI)                        │
│  server/cmd/multica  →  daemon polls claim endpoint      │
│  Executes Claude Code locally, reports via REST          │
└─────────────────────────────────────────────────────────┘
```

## Directory Guide

```
server/
  cmd/
    server/         Main server entry point + router setup
    multica/        CLI entry point (also runs local daemon)
    migrate/        Database migration runner
  internal/
    handler/        HTTP handlers (one file per domain)
    middleware/     Auth, workspace membership, rate limiting
    realtime/       WebSocket hub + client management
    events/         In-process pub/sub bus
    daemon/         Daemon client + task execution logic
    taskservice/    Task creation and dispatch logic
  pkg/
    db/
      generated/    sqlc-generated type-safe query methods
      queries/      Raw SQL query files (edit these, then run make sqlc)
    protocol/       WebSocket event type constants + message structs

apps/
  web/
    app/
      (auth)/       Login page group
      (dashboard)/  Protected routes: issues, projects, agents, inbox, etc.
      (landing)/    Public marketing pages
    platform/       Next.js-specific: navigation adapter, auth cookies
    components/     web-providers.tsx (CoreProvider wiring)
  desktop/
    src/renderer/src/
      App.tsx        Root: CoreProvider + conditional login/shell
      routes.tsx     Memory router route definitions
      platform/      Desktop navigation adapter (DesktopNavigationProvider)
      stores/        Tab store (tab-per-route memory routers)

packages/
  core/             Headless business logic (zero react-dom, zero next/*)
    api/            ApiClient + WSClient
    auth/           Auth store (user, login, logout)
    workspace/      Workspace store (current ws, list, switch)
    chat/           Chat panel store
    issues/         Issue stores (view, draft, selection, recent)
    modals/         Global modal store
    navigation/     Navigation store (last path)
    realtime/       WSProvider, useRealtimeSync, useWSEvent
    runtimes/       Runtime hooks (update detection)
    platform/       CoreProvider, StorageAdapter, AuthInitializer
  ui/               Atomic components (shadcn/Base UI, zero business logic)
    components/ui/  55 shadcn components
    components/common/  8 domain-aware shared components
    styles/         Shared Tailwind CSS foundation
  views/            Shared business pages (zero next/*, zero react-router)
    layout/         DashboardLayout, DashboardGuard, AppSidebar
    issues/         IssuesPage, IssueDetailPage, BoardView, ListView
    my-issues/      MyIssuesPage
    inbox/          InboxPage
    agents/         AgentsPage, AgentDetailPage
    projects/       ProjectsPage, ProjectDetailPage
    settings/       SettingsPage (account + workspace tabs)
    runtimes/       RuntimesPage
    skills/         SkillsPage
    navigation/     NavigationAdapter interface + NavigationProvider
```

## Key Architectural Decisions

### Internal Packages Pattern
All `packages/*` export raw `.ts`/`.tsx` files — no pre-compilation. The consuming app's bundler compiles them directly. This gives zero-config HMR and instant go-to-definition.

### Dependency Direction
```
apps/web  ──→  packages/views  ──→  packages/core
              packages/ui      ──→  (nothing internal)
apps/desktop ─→ packages/views
```
`ui/` and `core/` are independent of each other. No circular imports.

### Platform Bridge
`packages/core/platform/` provides `CoreProvider` — initializes API client, auth/workspace stores, WS connection, and QueryClient. Each app wraps its root with `<CoreProvider>` and provides its own `NavigationAdapter` for routing.

### Navigation Abstraction
Views use `useNavigation().push(path)` and `<AppLink>` — never `next/link` or `react-router-dom` directly. Each app injects its own `NavigationAdapter` implementation.

## Data Flows

### Auth Flow
```
User enters email
  → POST /api/auth/send-code  (rate-limited, sends 6-digit code)
  → POST /api/auth/verify-code  → JWT returned
  → AuthStore.setUser(user) + token saved to storage
  → WorkspaceStore.hydrateWorkspace([workspaces])
  → API client sets Authorization: Bearer {token} header
  → onLogin() callback (sets cookie in web app)
```

### Workspace Switching
```
User selects workspace
  → WorkspaceStore.switchWorkspace(wsId)
  → API client sets X-Workspace-ID header
  → TanStack Query cache cleared (keys include wsId → new queries fire)
  → Workspace-aware Zustand stores re-hydrated (storage prefix changes)
  → WS reconnects with new workspace_id query param
```

### Real-Time Update Flow
```
Daemon/user action → REST endpoint
  → Handler publishes event to events.Bus
  → Bus dispatches to 3 listener chains:
      1. subscriber_listeners.go  → write inbox items
      2. activity_listeners.go    → create timeline entries
      3. notification_listeners.go → send email, push inbox notifications
  → Hub.Broadcast(workspaceID, message)
  → All WS clients in workspace room receive message
  → WSClient.on(eventType, handler) fires
  → useRealtimeSync maps event → TanStack Query invalidation
  → Components re-render with fresh data
```

### Daemon Task Execution
```
Issue assigned to agent
  → TaskService.CreateTask(issueId, agentId)  →  agent_task_queue row (status=queued)
  → WS: task:dispatch event to daemon

Daemon poll loop (every 2s):
  → GET /api/runtimes/{runtimeId}/claim-task  (FOR UPDATE SKIP LOCKED)
  → POST /api/tasks/{taskId}/start
  → Execute Claude Code locally
  → POST /api/tasks/{taskId}/progress  (streaming updates)
  → POST /api/tasks/{taskId}/messages  (tool calls, output)
  → POST /api/tasks/{taskId}/complete | /fail

Frontend receives task:progress + task:message WS events
  → Invalidates issue + task queries
  → Live task panel shows streaming output
```

### Desktop Tab System
```
apps/desktop uses multiple memory routers (one per tab)
  → TabStore: tabs[], activeTabId
  → Each tab has its own RouterProvider (isolated history)
  → DesktopNavigationProvider (root): reads active tab's router, proxies push/back
  → TabNavigationProvider (per-tab): provides pathname/searchParams for that tab
  → Shared views use useNavigation() → routes to active tab's router
```

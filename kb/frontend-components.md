# Frontend Components

## Package Overview

```
packages/ui/     Atomic UI (shadcn + Base UI, zero business logic)
packages/views/  Shared business pages (zero next/*, zero react-router)
packages/core/   Headless logic: stores, hooks, API client, WS client
```

---

## packages/ui — Atomic Components

All components use `@base-ui/react` primitives (not Radix), Tailwind with semantic design tokens, and live in `packages/ui/components/ui/`.

### Form & Input (8)
| Component | File |
|-----------|------|
| Button | `button.tsx` |
| Input | `input.tsx` |
| Label | `label.tsx` |
| Checkbox | `checkbox.tsx` |
| RadioGroup | `radio-group.tsx` |
| Select | `select.tsx` |
| NativeSelect | `native-select.tsx` |
| ToggleGroup | `toggle-group.tsx` |

### Containers & Layout (9)
| Component | File |
|-----------|------|
| Card | `card.tsx` |
| Accordion | `accordion.tsx` |
| Collapsible | `collapsible.tsx` |
| Tabs | `tabs.tsx` |
| Resizable | `resizable.tsx` |
| Sidebar | `sidebar.tsx` |
| Drawer | `drawer.tsx` |
| Sheet | `sheet.tsx` |
| ScrollArea | `scroll-area.tsx` |

### Dialogs & Popovers (5)
| Component | File |
|-----------|------|
| Dialog | `dialog.tsx` |
| Popover | `popover.tsx` |
| DropdownMenu | `dropdown-menu.tsx` |
| HoverCard | `hover-card.tsx` |
| AlertDialog | `alert-dialog.tsx` |

### Navigation & Lists (4)
| Component | File |
|-----------|------|
| NavigationMenu | `navigation-menu.tsx` |
| Breadcrumb | `breadcrumb.tsx` |
| Menubar | `menubar.tsx` |
| Pagination | `pagination.tsx` |

### Feedback & Status (6)
| Component | File |
|-----------|------|
| Alert | `alert.tsx` |
| Skeleton | `skeleton.tsx` |
| Progress | `progress.tsx` |
| Spinner | `spinner.tsx` |
| Badge | `badge.tsx` |
| Empty | `empty.tsx` |

### Data Display (3)
| Component | File |
|-----------|------|
| Table | `table.tsx` |
| Carousel | `carousel.tsx` |
| Chart | `chart.tsx` |

### Utilities (11)
`field.tsx`, `input-group.tsx`, `button-group.tsx`, `aspect-ratio.tsx`, `item.tsx`, `kbd.tsx`, `direction.tsx`, `command.tsx`, `combobox.tsx`, `toggle.tsx`, `separator.tsx`

### Common Components (`packages/ui/components/common/`)
Domain-aware but still dependency-free from `@multica/core`:

| Component | Description |
|-----------|-------------|
| `actor-avatar.tsx` | Avatar with fallback initials; handles member/agent variants |
| `emoji-picker.tsx` | Full emoji picker popover |
| `quick-emoji-picker.tsx` | Compact emoji reaction bar |
| `reaction-bar.tsx` | Grouped reactions with count |
| `file-upload-button.tsx` | File input trigger button |
| `mention-hover-card.tsx` | @mention hover card (user/agent info) |
| `multica-icon.tsx` | Brand icon component |
| `theme-provider.tsx` | Light/dark/system theme provider |

### Adding Components
```bash
pnpm ui:add <component-name>
# Adds to packages/ui/components/ui/
# Config: packages/ui/components.json (Base UI variant, base-nova style)
```

---

## packages/views — Shared Business Pages

All pages are framework-agnostic. They:
- Import from `@multica/core` and `@multica/ui`
- Use `useNavigation()` (never `next/link` or `<Link>`)
- Live in `packages/views/<domain>/`

### Layout Components

#### `DashboardGuard` (`layout/dashboard-guard.tsx`)
Auth + workspace check. Used by both apps as the root layout wrapper.
- Checks auth state; redirects to login if not authenticated
- Provides `WorkspaceIdProvider` to children
- Shows workspace-loading skeleton during hydration

#### `DashboardLayout` (`layout/dashboard-layout.tsx`)
```tsx
<DashboardLayout
  sidebar={<AppSidebar ... />}
  extra={<PlatformSpecificSlot />}   // optional, web/desktop inject their own
>
  {children}
</DashboardLayout>
```

#### `AppSidebar` (`layout/app-sidebar.tsx`)
- Personal nav: Inbox (with unread badge), My Issues
- Workspace nav: Issues, Projects, Agents
- Configure nav: Runtimes, Skills, Settings
- Pinned items with drag-reorder (dnd-kit)
- Global `C` shortcut to open create-issue modal
- Workspace switcher at top

### Page Components

#### `IssuesPage` (`issues/components/issues-page.tsx`)
- Fetches all workspace issues
- Filters by scope (all/members/agents) + view filters from `issueViewStore`
- Board view (kanban by status) or list view
- Drag-and-drop reordering (switches to manual sort)
- Batch actions on selected issues

#### `IssueDetailPage` (`issues/components/issue-detail-page.tsx`)
- Full issue detail with description editor (tiptap)
- Timeline (comments + activity merged, chronological)
- Assignee, status, priority, project, due date pickers
- Subscriber list
- Active task panel (shows live streaming output)
- Reactions

#### `MyIssuesPage` (`my-issues/components/my-issues-page.tsx`)
- Scoped to user's assigned/created/agent-owned issues
- Scope selector tab (assigned / created / agents)
- Load-more for completed issues

#### `InboxPage` (`inbox/components/inbox-page.tsx`)
- Two-panel resizable (list + detail)
- Click to read auto-marks as read
- Batch: mark all read, archive all, archive completed, archive read

#### `AgentsPage` (`agents/components/agents-page.tsx`)
- Two-panel resizable (list + detail)
- Create/archive/restore agents
- Agent detail: instructions editor, skills list, task history, runtime status

#### `ProjectsPage` (`projects/components/projects-page.tsx`)
- Table with inline lead picker, status, priority dropdowns
- Create project dialog with emoji icon picker + description editor
- Progress bar (done/total issue counts per project)

#### `SettingsPage` (`settings/components/settings-page.tsx`)
- Two tab groups:
  - Account: Profile, Appearance, API Tokens
  - Workspace: General, Repositories (GitHub), Members

#### `RuntimesPage` (`runtimes/components/runtimes-page.tsx`)
- List of connected local/cloud runtimes
- Status (online/offline), last seen
- Update available indicator

#### `SkillsPage` (`skills/components/skills-page.tsx`)
- Skill library with create/edit/delete
- Skill detail: content editor, file attachments, agent assignments

---

## packages/core — Hooks

### `useActorName` (`workspace/hooks.ts`)
```ts
const { getActorName, getActorInitials, getActorAvatarUrl } = useActorName(wsId)
getActorName('member', userId)   // → "Jane Doe"
getActorName('agent', agentId)   // → "Code Agent"
```

### `useWSEvent` (`realtime/hooks.ts`)
```ts
useWSEvent('issue:updated', (payload) => { ... })
```

### `useWSReconnect` (`realtime/hooks.ts`)
```ts
useWSReconnect(() => { /* refetch on reconnect */ })
```

### `useMyRuntimesNeedUpdate` (`runtimes/hooks.ts`)
```ts
const needsUpdate = useMyRuntimesNeedUpdate(wsId)
// accepts wsId as param — safe outside WorkspaceIdProvider
```

### `useWorkspaceId` (`hooks.tsx`)
```ts
const wsId = useWorkspaceId()  // throws outside WorkspaceIdProvider
```

---

## Navigation

### `NavigationAdapter` interface (`views/navigation/`)
```ts
interface NavigationAdapter {
  push(path: string): void
  replace(path: string): void
  back(): void
  pathname: string
  searchParams: URLSearchParams
  openInNewTab?(path: string, title?: string): void  // desktop only
  getShareableUrl?(path: string): string | null       // desktop only
}
```

Web implementation: wraps `next/router` + `usePathname` + `useSearchParams`
Desktop implementation: wraps per-tab memory router

### Usage in views
```tsx
import { useNavigation } from "@multica/views/navigation"
const nav = useNavigation()
nav.push(`/issues/${issueId}`)
```

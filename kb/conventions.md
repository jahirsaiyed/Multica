# Conventions

Distilled from `CLAUDE.md`. Read `CLAUDE.md` for the authoritative, full version.

---

## Package Boundary Rules (Hard Constraints)

| Package | Zero imports of |
|---------|----------------|
| `packages/core/` | `react-dom`, `localStorage` (use StorageAdapter), `process.env`, UI libs, `next/*`, `react-router-dom` |
| `packages/ui/` | `@multica/core`, `@multica/views`, any business logic |
| `packages/views/` | `next/*`, `react-router-dom`, Zustand stores (use hooks from core) |
| `apps/web/platform/` | Only place for `next/navigation` wiring |
| `apps/desktop/src/renderer/src/platform/` | Only place for `react-router-dom` navigation wiring |

**All shared Zustand stores live in `packages/core/`** — even view-related ones (filters, view modes). Stores are pure state, not UI.

---

## The No-Duplication Rule

If the same logic exists in both apps, it must go to a shared package.

Decision process:
1. Depends on Next.js or Electron APIs? → Keep in the respective app
2. Depends on `react-router-dom` or `next/navigation`? → Keep in app's `platform/` layer
3. Everything else → `packages/core/` (headless logic) or `packages/views/` (UI components)

When apps need different behavior for the same concept, extract shared logic into a component with props/slots for the differences. Never duplicate logic.

---

## Cross-Platform Feature Checklist

When adding a new page or feature:

1. **New page component** → `packages/views/<domain>/`
   - No framework-specific imports
   - Use `useNavigation().push()` for routing
2. **Wire in both apps**:
   - Web: add route in `apps/web/app/(dashboard)/`
   - Desktop: add route to `apps/desktop/src/renderer/src/routes.tsx`
3. **Navigation**: always use `useNavigation()` / `<AppLink>` — never framework-specific
4. **Shared guards/providers**: use `DashboardGuard` from `packages/views/layout/`
5. **Platform-specific UI**: inject via props slots (`extra`, `topSlot`) on shared layout components
6. **New hooks needing workspace context**: accept `wsId` as parameter, not read from `useWorkspaceId()` Context

---

## State Management Rules

1. **Never duplicate server data into Zustand.** If from API → Query cache only.
2. **Workspace-scoped queries must key on `wsId`.** Workspace switching = automatic.
3. **Mutations are optimistic by default.** Apply locally → send → roll back on error → invalidate on settle.
4. **WS events invalidate queries — never write to stores directly.** Single source of truth.
5. **Persist what's worth preserving** (preferences, drafts, tab layout). **Don't persist ephemeral state** (modal open/close, transient selections) or server data.

---

## CSS Architecture

- **Design tokens only.** Use semantic tokens (`bg-background`, `text-muted-foreground`). Never hardcoded Tailwind colors (`text-red-500`, `bg-gray-100`).
- **Shared styles** in `packages/ui/styles/`. Never duplicate scrollbar styling, keyframes, or base layer rules in app CSS.
- **`@source` directives** in both apps scan shared packages so Tailwind sees all class names.

---

## Go Conventions

- Standard Go formatting: `gofmt`, `go vet`
- One handler file per domain: `issue.go`, `agent.go`, `comment.go`, etc.
- All queries filter by `workspace_id` first
- Return JSON errors: `{"error": "message"}`
- Polymorphic actors: `actor_type` + `actor_id`; resolve via `resolveActor()`

---

## TypeScript Conventions

- Strict mode enabled; keep types explicit
- No `any` without comment explaining why
- Avoid `as` casts; prefer type guards

---

## Coding Rules

- **English only** in code comments
- **Prefer existing patterns** over introducing parallel abstractions
- **No backwards compatibility layers** unless explicitly requested — remove old paths instead of preserving both
- **No over-engineering** — minimum complexity needed. Three similar lines > premature abstraction.
- **No error handling for impossible scenarios** — only validate at system boundaries
- **No feature flags** or compatibility shims when you can just change the code

---

## Testing Rules

### Where tests live
| What | Where | Why |
|------|-------|-----|
| Shared business logic (stores, hooks) | `packages/core/*.test.ts` | No DOM needed |
| Shared UI components (pages, forms) | `packages/views/*.test.tsx` | jsdom, no framework mocks |
| Platform-specific wiring (cookies, redirects) | `apps/web/*.test.tsx` | Needs framework mocks |
| End-to-end flows | `e2e/*.spec.ts` | Real browser + backend |

Never test shared component behavior in an app test file. If a test mocks `next/navigation` to test a `@multica/views` component, it's in the wrong place.

### Mock patterns (Vitest)
```ts
// Zustand store mock — stores are both callable and have .getState()
const mockStore = vi.hoisted(() => {
  const fn = vi.fn((selector) => selector(state))
  Object.assign(fn, { getState: () => state })
  return fn
})
vi.mock('@multica/core', () => ({ useAuthStore: mockStore }))
```

### Go tests
Standard `go test`. Tests create their own fixture data in the test database.

### E2E tests
Self-contained. Use `TestApiClient` fixture for setup/teardown:
```ts
test.beforeEach(async ({ page }) => {
  api = await createTestApi()
  await loginAsDefault(page)
})
test.afterEach(async () => { await api.cleanup() })
```

---

## Commit Format

```
feat(scope): description
fix(scope): description
refactor(scope): description
docs: description
test(scope): description
chore(scope): description
```

Atomic commits grouped by logical intent.

---

## Verification Before Push

```bash
make check   # typecheck + unit tests + Go tests + E2E
```

For quick iteration:
```bash
pnpm typecheck   # TS errors only
pnpm test        # Vitest only
make test        # Go tests only
```

---

## pnpm Catalog

All shared dependencies (including test deps) are version-pinned in `pnpm-workspace.yaml` under `catalog:`. When adding a new shared dep, add to catalog first, then reference with `catalog:` in `package.json`.

```yaml
# pnpm-workspace.yaml
catalog:
  some-package: "^1.0.0"
```
```json
// package.json
{ "dependencies": { "some-package": "catalog:" } }
```

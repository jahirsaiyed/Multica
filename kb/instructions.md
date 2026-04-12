# Multica Codebase — AI Assistant Instructions

You are reading a packed representation of the Multica codebase. Multica is an **AI-native task management platform** — like Linear, but with AI agents as first-class citizens.

## What This Codebase Is

- Agents can be assigned issues, create issues, comment, and change status
- Supports local (daemon) and cloud agent runtimes
- Built for 2–10 person AI-native teams
- Go backend + monorepo frontend (pnpm workspaces + Turborepo)

## Repo Layout

```
server/          Go backend (Chi router, sqlc, gorilla/websocket)
apps/web/        Next.js 15 App Router frontend
apps/desktop/    Electron desktop app (electron-vite)
packages/core/   Headless business logic — all shared Zustand stores and hooks
packages/ui/     Atomic UI components (Base UI / shadcn, no business logic)
packages/views/  Shared business pages/components (no next/* or react-router imports)
```

## Key Constraints to Respect

1. **Package boundaries are hard rules.** `core/` has zero react-dom and zero localStorage. `ui/` has zero `@multica/core`. `views/` has zero `next/*` or `react-router-dom`.
2. **Server state lives in TanStack Query.** Never copy API data into Zustand stores.
3. **Mutations are optimistic.** Apply locally → send request → roll back on failure → invalidate on settle.
4. **WS events invalidate queries — never write to stores directly.**
5. **All queries key on `wsId`.** Workspace switching is automatic cache-key rotation.
6. **New shared pages go in `packages/views/`** and must be wired in both `apps/web/` and `apps/desktop/`.

## Reference

See `kb/` directory for detailed documentation:
- `architecture.md` — system map, data flow, auth/WS/daemon flows
- `api-reference.md` — all REST routes and WS event types
- `data-models.md` — PostgreSQL schema and sqlc patterns
- `state-management.md` — Zustand stores and TanStack Query patterns
- `frontend-components.md` — component inventory
- `agent-runtime.md` — agent lifecycle and daemon protocol
- `conventions.md` — coding rules distilled from CLAUDE.md

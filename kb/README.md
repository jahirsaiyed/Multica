# Knowledge Base

Human- and AI-readable documentation for the Multica codebase. Use these files for onboarding, code reviews, and as context when working with AI assistants.

## Files

| File | Contents |
|------|----------|
| `architecture.md` | System map, directory guide, auth/WS/daemon data flows |
| `api-reference.md` | All REST endpoints and WebSocket event types |
| `data-models.md` | PostgreSQL schema, table relationships, sqlc patterns |
| `frontend-components.md` | Component inventory: ui/, views/, core/ hooks |
| `state-management.md` | Zustand stores, TanStack Query patterns, rules |
| `agent-runtime.md` | Agent lifecycle, daemon protocol, task queue, skill system |
| `conventions.md` | Coding rules, package boundaries, CSS architecture |
| `instructions.md` | Prepended to every repomix XML pack as AI context |

## Repomix XML Packs

Generated XML files (git-ignored) for feeding the full codebase to AI assistants:

| Pack | Config | Contents |
|------|--------|----------|
| `kb/full.xml` | `repomix.config.json` | Entire codebase (all Go + TS) |
| `kb/frontend.xml` | `repomix.config.frontend.json` | packages/ + apps/ TS only |
| `kb/backend.xml` | `repomix.config.backend.json` | server/ Go + SQL queries |
| `kb/core.xml` | `repomix.config.core.json` | packages/core/ only |

## Regenerating

```bash
# Regenerate all XML packs
make kb
# or
pnpm repomix:all

# Individual packs
pnpm repomix:full
pnpm repomix:frontend
pnpm repomix:backend
pnpm repomix:core
```

## Keeping KB Up to Date

Update the relevant markdown files in `kb/` when:
- Adding or removing API routes (`api-reference.md`)
- Changing the database schema (`data-models.md`)
- Adding new Zustand stores or query hooks (`state-management.md`)
- Adding shared packages or views (`frontend-components.md`, `architecture.md`)
- Changing agent runtime behavior (`agent-runtime.md`)

The PR checklist in `.github/PULL_REQUEST_TEMPLATE.md` includes a reminder.

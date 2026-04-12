# Agent Runtime

## Overview

Agents are AI actors that can be assigned issues, create issues, post comments, and change statuses. Every agent is backed by an **agent_runtime** — the execution environment. Two runtime modes exist:

- **local** — a daemon process running on a developer's machine, polling the server
- **cloud** — a server-managed runtime (future)

---

## Agent Data Model

```
agent
  ├── id, workspace_id
  ├── name, description, avatar_url
  ├── runtime_mode: "local" | "cloud"
  ├── runtime_id → agent_runtime
  ├── visibility: "workspace" | "private"
  ├── status: "idle" | "working" | "blocked" | "error" | "offline"
  ├── max_concurrent_tasks: int (default 1)
  ├── instructions: TEXT           ← system prompt prepended to every task
  ├── tools: JSONB []              ← tool configuration
  ├── triggers: JSONB []           ← event triggers (on_comment, on_mention, etc.)
  ├── owner_id → user              ← for private agents
  └── archived_at / archived_by    ← soft delete
```

Agents have **skills** — reusable instruction sets attached via the `agent_skill` junction table.

---

## Agent Lifecycle

```
Created → Archived (soft delete)
              ↓
           Restored
```

On archive: all `queued` and `dispatched` tasks for that agent are cancelled.
On restore: agent returns to `offline` status; daemon re-registers to bring it online.

---

## Daemon Protocol

The local daemon is the `multica daemon` CLI command (`server/cmd/multica`).

### Registration
```
POST /api/daemons/register
Body: { daemon_id, device_name, runtimes: [{ name, runtime_id, provider }] }
→ Creates/updates agent_runtime rows (status = 'online')
→ Returns workspace repos for context
```

### Heartbeat
```
POST /api/runtimes/{runtimeId}/heartbeat
→ Updates last_seen_at
→ Returns: pending ping requests, update availability flags
```
The server sweeper marks runtimes as `offline` if no heartbeat for >60s.

### Pairing Flow (first-time setup)
```
multica setup --local
  → Daemon generates pairing token
  → POST /api/daemons/pairing-sessions (status=pending)
  → User approves in web UI → POST /api/daemons/pairing-sessions/{token}/approve
  → Daemon polls until approved → POST /api/daemons/pairing-sessions/{token}/claim
  → Claims workspace + receives daemon_token (mdt_{token})
  → Token stored in ~/.multica/config
```

---

## Task Queue

### Task States
```
queued → dispatched → running → completed
                             → failed
                   → cancelled (any time before completed/failed)
```

### Serialization Constraint
At most one `queued` or `dispatched` task per issue (enforced by partial unique index). New tasks for the same issue queue only after the current one completes.

### Task Claim
Daemon polls:
```
GET /api/runtimes/{runtimeId}/claim-task
→ Atomically claims next queued task (FOR UPDATE SKIP LOCKED)
→ Returns: task + agent skills + workspace repos + session_id (for resumption)
```

### Session Resumption
Tasks store `session_id` (Claude Code session) and `work_dir`. On the next task for the same issue:
- Daemon checks if session_id is set in the claimed task
- If yes, resumes the existing Claude Code session in the same working directory
- Preserves conversation history and file context

### Chat Tasks
Chat sessions use the same `agent_task_queue` table. Differences:
- `issue_id = NULL`, `chat_session_id` is set instead
- Triggered by `POST /api/chat-sessions/{id}/messages`
- No per-issue deduplication constraint

---

## Task Execution Flow

```
1. Issue assigned to agent (via UI or agent API call)
   → TaskService.CreateTask(issueId, agentId, runtime_id)
   → INSERT INTO agent_task_queue (status='queued')
   → WS broadcast: task:dispatch

2. Daemon claim loop (every 2s):
   GET /api/runtimes/{runtimeId}/claim-task
   → Returns task with: instructions, issue context, agent skills, repos, session_id

3. Daemon executes:
   POST /api/tasks/{taskId}/start    ← marks running

   [Claude Code execution loop]
   POST /api/tasks/{taskId}/progress  ← { summary, step, total }
   POST /api/tasks/{taskId}/messages  ← streaming { type, content, tool, input, output }
   POST /api/tasks/{taskId}/usage     ← token counts per model

   POST /api/tasks/{taskId}/complete  ← { output, pr_url, session_id, work_dir }
   OR
   POST /api/tasks/{taskId}/fail      ← { error }

4. Each report event:
   → Handler publishes to events.Bus
   → Bus dispatches to listener chains
   → Hub broadcasts WS events to workspace clients
   → Frontend: useRealtimeSync invalidates query cache
   → Task panel shows streaming output in real time
```

---

## Task Message Types

Messages are stored in `task_message` table and streamed via `task:message` WS events:

| Type | Description | Fields Used |
|------|-------------|-------------|
| `text` | Text output from agent | `content` |
| `tool_use` | Agent invoking a tool | `tool`, `input` (JSONB) |
| `tool_result` | Result from tool execution | `tool`, `output` |
| `error` | Error during execution | `content` |

Messages have a `seq` field for ordering and deduplication. Frontend stores them in a timeline, collapsible by tool name.

---

## Skills

Skills are reusable instruction modules attached to agents.

```
skill
  ├── id, workspace_id
  ├── name (unique per workspace)
  ├── description
  ├── content: TEXT          ← main instructions/prompt content
  ├── config: JSONB          ← tool/model config overrides
  └── skill_file[]           ← supporting files (path + content)

agent_skill (junction)
  ├── agent_id
  └── skill_id
```

When the daemon claims a task, all skills for the agent are included in the context snapshot. The daemon injects skill content into the Claude Code context.

---

## Triggers

Agents can define **triggers** — event conditions that automatically create tasks:

```json
// triggers JSONB field on agent
[
  { "type": "on_comment", "conditions": { ... } },
  { "type": "on_mention", "conditions": { ... } },
  { "type": "on_status_change", "conditions": { "status": "in_progress" } }
]
```

Trigger evaluation happens in the server-side comment/issue handler:
1. Comment created on an issue with an assigned agent
2. Handler checks agent triggers
3. If trigger condition matches → `TaskService.CreateTask()`

---

## Agent as Assignee (Polymorphic Pattern)

Issues have:
```sql
assignee_type TEXT CHECK('member', 'agent')
assignee_id   UUID
```

The frontend renders agents with:
- Purple background avatar
- Robot icon indicator
- Distinct styling in assignee picker

The server resolves the actor via `resolveActor()` in handler context:
- `X-Agent-ID` header present → `actor_type = 'agent'`
- Otherwise → `actor_type = 'member'`

---

## Token Usage Tracking

```
task_usage: per-task token counts per (provider, model)
runtime_usage: per-runtime daily aggregates per (provider, model)
```

Usage reported via `POST /api/tasks/{taskId}/usage` after each Claude API call.
Aggregated usage visible on issue detail page via `GET /api/issues/{id}/usage`.

---

## Frontend: Agent Runtime UI

- **RuntimesPage** (`packages/views/runtimes/`): lists runtimes, status, last-seen
- **AgentsPage** (`packages/views/agents/`): agent list + detail panel
  - Instructions editor
  - Skills assignment
  - Task history
  - Archive/restore controls
- **Task panel on IssueDetailPage**: shows live streaming task output
  - Tool calls expandable
  - Progress bar from `task:progress` events
  - Status badge (queued/running/completed/failed/cancelled)
- **Chat panel** (`packages/core/chat/store.ts`): side panel for direct agent chat
  - Persists session across navigation
  - Timeline of tool calls + text output

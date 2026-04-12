# API Reference

All handlers live in `server/internal/handler/`. Routes registered in `server/cmd/server/router.go`.

## Authentication

All endpoints (except the auth group below) require:
- `Authorization: Bearer {jwt}` — JWT issued by verify-code or google-login
- `X-Workspace-ID: {uuid}` — required for all workspace-scoped operations

Optional:
- `X-Agent-ID: {uuid}` — when request originates from an agent
- `X-Task-ID: {uuid}` — when agent request is tied to a specific task

Daemon endpoints use `Authorization: Bearer mdt_{token}` (daemon tokens) or PAT fallback.

## Error Format

```json
{ "error": "descriptive message" }
```

Common status codes: `200`, `201`, `204`, `400`, `401`, `403`, `404`, `409`, `500`.

---

## Auth (`auth.go`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/auth/send-code` | Send email verification code (rate-limited: 1/10s per email) |
| POST | `/api/auth/verify-code` | Verify code → issue JWT; creates user + workspace if new |
| POST | `/api/auth/google-login` | OAuth2 Google login with code exchange |
| GET | `/api/auth/me` | Get current authenticated user |
| PATCH | `/api/auth/me` | Update user name or avatar URL |

---

## Workspaces (`workspace.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/workspaces` | List all workspaces for current user |
| POST | `/api/workspaces` | Create workspace with auto-generated issue prefix |
| GET | `/api/workspaces/{id}` | Get workspace by ID or slug |
| PATCH | `/api/workspaces/{id}` | Update name, description, settings, repos, issue prefix |
| DELETE | `/api/workspaces/{id}` | Delete workspace |
| GET | `/api/workspaces/{id}/members` | List members (basic info) |
| GET | `/api/workspaces/{id}/members-with-user` | List members with full user details |
| POST | `/api/workspaces/{id}/members` | Add member by email (auto-creates user if needed) |
| PATCH | `/api/workspaces/{id}/members/{memberId}` | Change member role (owner/admin/member) |
| DELETE | `/api/workspaces/{id}/members/{memberId}` | Remove member |
| POST | `/api/workspaces/{id}/leave` | Current user leaves workspace |

---

## Issues (`issue.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/issues` | List issues; filters: status, priority, assignee, search, project |
| POST | `/api/issues` | Create issue with optional assignee and attachments |
| GET | `/api/issues/{id}` | Get issue by UUID or identifier (e.g., MUL-42) |
| PATCH | `/api/issues/{id}` | Update title, description, status, priority, assignee, project, due date |
| DELETE | `/api/issues/{id}` | Delete issue (with S3 attachment cleanup) |
| POST | `/api/issues/{id}/subscribe` | Subscribe to issue |
| DELETE | `/api/issues/{id}/subscribe` | Unsubscribe from issue |
| GET | `/api/issues/{id}/subscribers` | List subscribers |
| GET | `/api/issues/{id}/timeline` | Merged timeline of activities + comments (chronological) |
| GET | `/api/issues/{id}/attachments` | List attachments |
| POST | `/api/issues/{id}/reactions` | Add emoji reaction |
| DELETE | `/api/issues/{id}/reactions` | Remove emoji reaction |
| GET | `/api/issues/assignee-frequency` | Frequency of assignees used by current user |
| GET | `/api/issues/search` | Full-text search (title + description, ranked) |
| GET | `/api/issues/{id}/usage` | Aggregated token usage for all tasks on issue |
| GET | `/api/issues/{id}/active-tasks` | Get active tasks for issue |
| GET | `/api/issues/{id}/tasks` | Full task history for issue |

---

## Comments (`comment.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/issues/{id}/comments` | List comments; supports `since` (RFC3339), `limit`, `offset` |
| POST | `/api/issues/{id}/comments` | Create comment; parses @mentions, auto-triggers on_comment/on_mention |
| PATCH | `/api/comments/{commentId}` | Edit comment (author or admin only) |
| DELETE | `/api/comments/{commentId}` | Delete comment + S3 cleanup |
| POST | `/api/comments/{commentId}/reactions` | Add emoji reaction |
| DELETE | `/api/comments/{commentId}/reactions` | Remove emoji reaction |

---

## Files (`file.go`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/upload-file` | Upload file (max 100MB); sniffs MIME; creates S3 object + DB record |
| GET | `/api/attachments/{id}` | Get attachment metadata with signed CloudFront URL |
| DELETE | `/api/attachments/{id}` | Delete attachment + S3 object |

---

## Projects (`project.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects` | List projects with optional status/priority filters; includes issue counts |
| POST | `/api/projects` | Create project with optional lead and description |
| GET | `/api/projects/{id}` | Get project with issue stats |
| PATCH | `/api/projects/{id}` | Update project fields |
| DELETE | `/api/projects/{id}` | Delete project |
| GET | `/api/projects/search` | Full-text search with snippet extraction |

---

## Agents (`agent.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/agents` | List agents; optional `archived` filter; batch-loads skills |
| POST | `/api/agents` | Create agent with runtime, instructions, visibility |
| GET | `/api/agents/{id}` | Get agent with full skill list |
| PATCH | `/api/agents/{id}` | Update agent (owner/admin only) |
| POST | `/api/agents/{id}/archive` | Archive agent; cancels all pending/active tasks |
| POST | `/api/agents/{id}/restore` | Restore archived agent |
| GET | `/api/agents/{id}/tasks` | All tasks (any status) for agent |

---

## Chat (`chat.go`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/chat-sessions` | Create chat session with agent |
| GET | `/api/chat-sessions` | List sessions by creator; optional status filter |
| GET | `/api/chat-sessions/{sessionId}` | Get session (creator only) |
| POST | `/api/chat-sessions/{sessionId}/archive` | Archive session |
| POST | `/api/chat-sessions/{sessionId}/messages` | Send user message; enqueues chat task |
| GET | `/api/chat-sessions/{sessionId}/messages` | List all messages in session |

---

## Inbox (`inbox.go`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/inbox` | List inbox items (unread + archived) |
| GET | `/api/inbox/unread-count` | Count of unread items |
| POST | `/api/inbox/{id}/read` | Mark item read |
| POST | `/api/inbox/{id}/archive` | Archive item (and all siblings for same issue) |
| POST | `/api/inbox/mark-all-read` | Mark all read |
| POST | `/api/inbox/archive-all` | Archive all |
| POST | `/api/inbox/archive-all-read` | Archive all read items |
| POST | `/api/inbox/archive-completed` | Archive items for completed issues |

---

## Daemon / Runtime (`daemon.go`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/daemons/register` | Daemon registration; returns workspace repos |
| POST | `/api/daemons/deregister` | Mark runtimes offline on shutdown |
| POST | `/api/runtimes/{runtimeId}/heartbeat` | Heartbeat; includes pending ping/update requests |
| GET | `/api/runtimes/{runtimeId}/claim-task` | Atomically claim next queued task (FOR UPDATE SKIP LOCKED) |
| GET | `/api/runtimes/{runtimeId}/pending-tasks` | List queued/dispatched tasks for runtime |
| POST | `/api/tasks/{taskId}/start` | Mark task as running |
| POST | `/api/tasks/{taskId}/complete` | Mark complete with output, PR URL, session ID, work dir |
| POST | `/api/tasks/{taskId}/fail` | Mark failed with error message |
| POST | `/api/tasks/{taskId}/progress` | Broadcast progress update (summary, step, total) |
| POST | `/api/tasks/{taskId}/usage` | Record token usage (provider, model, input/output/cache) |
| POST | `/api/tasks/{taskId}/messages` | Store + broadcast batch of agent execution messages |
| GET | `/api/tasks/{taskId}/messages` | Get persisted messages; optional `since` query |
| GET | `/api/tasks/{taskId}/status` | Get task status (used by daemon to check for cancellation) |
| POST | `/api/tasks/{taskId}/cancel` | User cancels running/queued task |

---

## Personal Access Tokens (`personal_access_token.go`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/personal-access-tokens` | Create PAT with optional expiry in days |
| GET | `/api/personal-access-tokens` | List all PATs for current user |
| DELETE | `/api/personal-access-tokens/{id}` | Revoke PAT |

---

## WebSocket Events (`server/pkg/protocol/events.go`)

Connect: `GET /ws?token={jwt}&workspace_id={uuid}` → upgrade to WS.

All messages have the envelope:
```json
{ "type": "event:type", "payload": { ... } }
```

### Issue Events
| Event | Payload |
|-------|---------|
| `issue:created` | Issue object |
| `issue:updated` | Updated issue fields |
| `issue:deleted` | `{ id }` |

### Comment Events
| Event | Payload |
|-------|---------|
| `comment:created` | Comment + issue context |
| `comment:updated` | Updated comment |
| `comment:deleted` | `{ id, issue_id }` |
| `reaction:added` | Reaction + comment context |
| `reaction:removed` | Reaction + comment context |
| `issue_reaction:added` | Reaction + issue context |
| `issue_reaction:removed` | Reaction + issue context |

### Task Events
| Event | Payload |
|-------|---------|
| `task:dispatch` | `{ task_id, issue_id, title, description }` |
| `task:progress` | `{ task_id, summary, step?, total? }` |
| `task:completed` | `{ task_id, pr_url?, output? }` |
| `task:failed` | `{ task_id, error }` |
| `task:cancelled` | `{ task_id }` |
| `task:message` | `{ task_id, issue_id?, seq, type, tool?, content?, input?, output? }` |

`task:message` types: `"text"`, `"tool_use"`, `"tool_result"`, `"error"`

### Agent Events
| Event | Payload |
|-------|---------|
| `agent:status` | `{ agent_id, status, task_count }` |
| `agent:created` | Agent object |
| `agent:archived` | `{ agent_id }` |
| `agent:restored` | `{ agent_id }` |

### Inbox Events
| Event | Payload |
|-------|---------|
| `inbox:new` | InboxItem object |
| `inbox:read` | `{ id }` |
| `inbox:archived` | `{ id }` |
| `inbox:batch-read` | `{ ids[] }` |
| `inbox:batch-archived` | `{ ids[] }` |

### Chat Events
| Event | Payload |
|-------|---------|
| `chat:message` | `{ chat_session_id, message_id, role, content, task_id?, created_at }` |
| `chat:done` | `{ chat_session_id, task_id, content }` |

### Other Events
| Event | Description |
|-------|-------------|
| `workspace:updated` | Workspace settings changed |
| `workspace:deleted` | Workspace deleted |
| `member:added` | Member added to workspace |
| `member:updated` | Member role changed |
| `member:removed` | Member removed |
| `subscriber:added` | User subscribed to issue |
| `subscriber:removed` | User unsubscribed |
| `activity:created` | Activity log entry created |
| `skill:created` / `skill:updated` / `skill:deleted` | Skill CRUD |
| `project:created` / `project:updated` / `project:deleted` | Project CRUD |
| `pin:created` / `pin:deleted` | Issue pinned/unpinned |
| `daemon:register` | Daemon connected |
| `daemon:heartbeat` | Daemon heartbeat |

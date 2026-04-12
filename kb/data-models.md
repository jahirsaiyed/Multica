# Data Models

Schema managed via migration files in `server/migrations/`. Queries in `server/pkg/db/queries/`, compiled to `server/pkg/db/generated/` via `make sqlc`.

## Core Tables

### `user`
```sql
id         UUID PK
name       TEXT NOT NULL
email      TEXT UNIQUE NOT NULL
avatar_url TEXT
created_at TIMESTAMPTZ DEFAULT now()
updated_at TIMESTAMPTZ DEFAULT now()
```

### `workspace`
```sql
id             UUID PK
name           TEXT NOT NULL
slug           TEXT UNIQUE NOT NULL
description    TEXT
settings       JSONB DEFAULT '{}'    -- UI/feature settings
context        TEXT                  -- workspace-level AI instructions
repos          JSONB DEFAULT '[]'    -- linked GitHub repos [{owner, name, default_branch}]
issue_prefix   TEXT DEFAULT ''       -- e.g. "MUL"
issue_counter  INT DEFAULT 0         -- auto-incremented per workspace
created_at     TIMESTAMPTZ
updated_at     TIMESTAMPTZ
```
Index: `idx_workspace_slug`

### `member`
```sql
id           UUID PK
workspace_id UUID FK → workspace CASCADE
user_id      UUID FK → user CASCADE
role         TEXT CHECK('owner','admin','member')
created_at   TIMESTAMPTZ
UNIQUE(workspace_id, user_id)
```
Index: `idx_member_workspace`

---

## Agent & Runtime Tables

### `agent`
```sql
id                   UUID PK
workspace_id         UUID FK → workspace CASCADE
name                 TEXT NOT NULL
description          TEXT DEFAULT ''
avatar_url           TEXT
runtime_mode         TEXT CHECK('local','cloud')
runtime_config       JSONB DEFAULT '{}'   -- legacy, being phased out
runtime_id           UUID FK → agent_runtime RESTRICT
visibility           TEXT DEFAULT 'workspace' CHECK('workspace','private')
status               TEXT DEFAULT 'offline' CHECK('idle','working','blocked','error','offline')
max_concurrent_tasks INT DEFAULT 1
owner_id             UUID FK → user NULLABLE
instructions         TEXT DEFAULT ''       -- system prompt injected into every task
tools                JSONB DEFAULT '[]'    -- tool config
triggers             JSONB DEFAULT '[]'    -- trigger definitions (on_comment, on_mention, etc.)
archived_at          TIMESTAMPTZ NULLABLE  -- soft delete
archived_by          UUID FK → user NULLABLE
created_at           TIMESTAMPTZ
updated_at           TIMESTAMPTZ
```
Index: `idx_agent_workspace`

### `agent_runtime`
```sql
id           UUID PK
workspace_id UUID FK → workspace CASCADE
daemon_id    TEXT NULLABLE          -- unique string identifier for the daemon process
name         TEXT NOT NULL
runtime_mode TEXT CHECK('local','cloud')
provider     TEXT NOT NULL          -- 'multica_agent', 'legacy_local', etc.
status       TEXT DEFAULT 'offline' CHECK('online','offline')
device_info  TEXT DEFAULT ''
metadata     JSONB DEFAULT '{}'
owner_id     UUID FK → user NULLABLE
last_seen_at TIMESTAMPTZ NULLABLE
created_at   TIMESTAMPTZ
updated_at   TIMESTAMPTZ
UNIQUE(workspace_id, daemon_id, provider)
```
Indexes: `idx_agent_runtime_workspace`, `idx_agent_runtime_status(workspace_id, status)`

---

## Issue Management

### `issue`
```sql
id                 UUID PK
workspace_id       UUID FK → workspace CASCADE
number             INT DEFAULT 0           -- per-workspace sequential number
title              TEXT NOT NULL
description        TEXT
status             TEXT DEFAULT 'backlog'
                   CHECK('backlog','todo','in_progress','in_review','done','blocked','cancelled')
priority           TEXT DEFAULT 'none'
                   CHECK('urgent','high','medium','low','none')
assignee_type      TEXT CHECK('member','agent')   -- polymorphic
assignee_id        UUID
creator_type       TEXT CHECK('member','agent')
creator_id         UUID NOT NULL
parent_issue_id    UUID FK → issue SET NULL        -- sub-tasks
project_id         UUID FK → project SET NULL
position           FLOAT DEFAULT 0                 -- drag-and-drop order
due_date           TIMESTAMPTZ NULLABLE
acceptance_criteria JSONB DEFAULT '[]'
context_refs       JSONB DEFAULT '[]'
created_at         TIMESTAMPTZ
updated_at         TIMESTAMPTZ
UNIQUE(workspace_id, number)
```
Indexes: `idx_issue_workspace`, `idx_issue_assignee(assignee_type, assignee_id)`, `idx_issue_status(workspace_id, status)`, `idx_issue_parent`, `idx_issue_project`, `idx_issue_workspace_number`, bigram indexes for full-text search

### `project`
```sql
id           UUID PK
workspace_id UUID FK → workspace CASCADE
title        TEXT NOT NULL
description  TEXT
icon         TEXT                      -- emoji icon
status       TEXT DEFAULT 'planned'
             CHECK('planned','in_progress','paused','completed','cancelled')
priority     TEXT DEFAULT 'none'
lead_type    TEXT CHECK('member','agent')  -- polymorphic lead
lead_id      UUID
created_at   TIMESTAMPTZ
updated_at   TIMESTAMPTZ
```

### `issue_subscriber`
```sql
issue_id  UUID FK → issue CASCADE   -- PK
user_type TEXT CHECK('member','agent')  -- PK
user_id   UUID                       -- PK
reason    TEXT CHECK('creator','assignee','commenter','mentioned','manual')
created_at TIMESTAMPTZ
```
Index: `idx_issue_subscriber_user(user_type, user_id)`

### `issue_reaction` / `comment_reaction`
```sql
id           UUID PK
issue_id / comment_id   UUID FK → issue/comment CASCADE
workspace_id UUID FK
actor_type   TEXT CHECK('member','agent')
actor_id     UUID NOT NULL
emoji        TEXT NOT NULL
created_at   TIMESTAMPTZ
UNIQUE(issue_id/comment_id, actor_type, actor_id, emoji)
```

---

## Comments

### `comment`
```sql
id           UUID PK
workspace_id UUID FK → workspace CASCADE
issue_id     UUID FK → issue CASCADE
author_type  TEXT CHECK('member','agent')
author_id    UUID NOT NULL
content      TEXT NOT NULL
type         TEXT DEFAULT 'comment'
             CHECK('comment','status_change','progress_update','system')
parent_id    UUID FK → comment SET NULL   -- reply threading
created_at   TIMESTAMPTZ
updated_at   TIMESTAMPTZ
```
Indexes: `idx_comment_issue`, `idx_comment_workspace`

### `attachment`
```sql
id            UUID PK
workspace_id  UUID FK
issue_id      UUID FK → issue CASCADE NULLABLE
comment_id    UUID FK → comment CASCADE NULLABLE
uploader_type TEXT CHECK('member','agent')
uploader_id   UUID NOT NULL
filename      TEXT NOT NULL
url           TEXT NOT NULL              -- S3 key
content_type  TEXT NOT NULL
size_bytes    BIGINT NOT NULL
created_at    TIMESTAMPTZ
```

---

## Task Queue

### `agent_task_queue`
```sql
id                UUID PK
agent_id          UUID FK → agent CASCADE
runtime_id        UUID FK → agent_runtime CASCADE
issue_id          UUID FK → issue CASCADE NULLABLE   -- NULL for chat tasks
chat_session_id   UUID FK → chat_session SET NULL    -- set for chat tasks
status            TEXT DEFAULT 'queued'
                  CHECK('queued','dispatched','running','completed','failed','cancelled')
priority          INT DEFAULT 0
trigger_comment_id UUID FK → comment SET NULL        -- comment that triggered this task
dispatched_at     TIMESTAMPTZ NULLABLE
started_at        TIMESTAMPTZ NULLABLE
completed_at      TIMESTAMPTZ NULLABLE
result            JSONB NULLABLE
error             TEXT
context           JSONB          -- snapshot passed to daemon at claim time
session_id        TEXT           -- Claude Code session ID for resumption
work_dir          TEXT           -- working directory for resumption
created_at        TIMESTAMPTZ
-- At most one pending task per issue (partial unique index):
UNIQUE(issue_id) WHERE status IN ('queued','dispatched')
```
Indexes:
- `idx_agent_task_queue_agent(agent_id, status)`
- `idx_agent_task_queue_pending(agent_id, priority DESC, created_at ASC)` WHERE pending
- `idx_agent_task_queue_runtime_pending(runtime_id, ...)` WHERE pending

### `task_message`
```sql
id       UUID PK
task_id  UUID FK → agent_task_queue CASCADE
seq      INTEGER NOT NULL    -- message sequence number
type     TEXT NOT NULL       -- 'text','tool_use','tool_result','error'
tool     TEXT NULLABLE
content  TEXT NULLABLE
input    JSONB NULLABLE      -- tool_use input
output   TEXT NULLABLE       -- tool_result output
created_at TIMESTAMPTZ
```
Index: `idx_task_message_task_id_seq(task_id, seq)`

### `task_usage`
```sql
id                UUID PK
task_id           UUID FK → agent_task_queue CASCADE
provider          TEXT DEFAULT ''
model             TEXT NOT NULL
input_tokens      BIGINT DEFAULT 0
output_tokens     BIGINT DEFAULT 0
cache_read_tokens BIGINT DEFAULT 0
cache_write_tokens BIGINT DEFAULT 0
created_at        TIMESTAMPTZ
UNIQUE(task_id, provider, model)
```

---

## Chat

### `chat_session`
```sql
id          UUID PK
workspace_id UUID FK
agent_id    UUID FK → agent CASCADE
creator_id  UUID FK → user CASCADE
title       TEXT DEFAULT ''
session_id  TEXT NULLABLE    -- Claude Code session ID for resumption
work_dir    TEXT NULLABLE
status      TEXT DEFAULT 'active' CHECK('active','archived')
created_at  TIMESTAMPTZ
updated_at  TIMESTAMPTZ
```

### `chat_message`
```sql
id               UUID PK
chat_session_id  UUID FK → chat_session CASCADE
role             TEXT CHECK('user','assistant')
content          TEXT NOT NULL
task_id          UUID NULLABLE    -- link to agent task if triggered one
created_at       TIMESTAMPTZ
```

---

## Skills

### `skill`
```sql
id           UUID PK
workspace_id UUID FK
name         TEXT NOT NULL
description  TEXT DEFAULT ''
content      TEXT DEFAULT ''   -- main skill content/instructions
config       JSONB DEFAULT '{}'
created_by   UUID FK → user NULLABLE
created_at   TIMESTAMPTZ
updated_at   TIMESTAMPTZ
UNIQUE(workspace_id, name)
```

### `skill_file`
```sql
id         UUID PK
skill_id   UUID FK → skill CASCADE
path       TEXT NOT NULL         -- relative file path
content    TEXT NOT NULL
created_at TIMESTAMPTZ
updated_at TIMESTAMPTZ
UNIQUE(skill_id, path)
```

### `agent_skill` (junction)
```sql
agent_id   UUID FK → agent CASCADE   -- PK
skill_id   UUID FK → skill CASCADE   -- PK
created_at TIMESTAMPTZ
```

---

## Inbox

### `inbox_item`
```sql
id             UUID PK
workspace_id   UUID FK
recipient_type TEXT CHECK('member','agent')
recipient_id   UUID NOT NULL
actor_type     TEXT NULLABLE
actor_id       UUID NULLABLE
type           TEXT NOT NULL
severity       TEXT DEFAULT 'info' CHECK('action_required','attention','info')
issue_id       UUID FK → issue CASCADE NULLABLE
title          TEXT NOT NULL
body           TEXT NULLABLE
details        JSONB DEFAULT '{}'
read           BOOLEAN DEFAULT FALSE
archived       BOOLEAN DEFAULT FALSE
created_at     TIMESTAMPTZ
```
Index: `idx_inbox_recipient(recipient_type, recipient_id, read)`

---

## Auth Tables

### `verification_code`
```sql
id         UUID PK
email      TEXT NOT NULL
code       TEXT NOT NULL
expires_at TIMESTAMPTZ NOT NULL
used       BOOLEAN DEFAULT FALSE
created_at TIMESTAMPTZ
```

### `personal_access_token`
```sql
id           UUID PK
user_id      UUID FK → user CASCADE
name         TEXT NOT NULL
token_hash   TEXT NOT NULL UNIQUE   -- SHA-256 of the token
token_prefix TEXT NOT NULL          -- first 8 chars for display
expires_at   TIMESTAMPTZ NULLABLE
last_used_at TIMESTAMPTZ NULLABLE
revoked      BOOLEAN DEFAULT FALSE
created_at   TIMESTAMPTZ
```

### `daemon_token`
```sql
id           UUID PK
token_hash   TEXT NOT NULL UNIQUE
workspace_id UUID FK
daemon_id    TEXT NOT NULL
expires_at   TIMESTAMPTZ NOT NULL
created_at   TIMESTAMPTZ
```

### `daemon_pairing_session`
```sql
id            UUID PK
token         TEXT NOT NULL UNIQUE
daemon_id     TEXT NOT NULL
device_name   TEXT NOT NULL
runtime_name  TEXT NOT NULL
runtime_type  TEXT NOT NULL
runtime_version TEXT DEFAULT ''
workspace_id  UUID FK NULLABLE
approved_by   UUID FK → user SET NULL
status        TEXT DEFAULT 'pending' CHECK('pending','approved','claimed','expired')
approved_at   TIMESTAMPTZ NULLABLE
claimed_at    TIMESTAMPTZ NULLABLE
expires_at    TIMESTAMPTZ NOT NULL
created_at    TIMESTAMPTZ
updated_at    TIMESTAMPTZ
```

---

## Other Tables

### `pinned_item`
```sql
id           UUID PK
workspace_id UUID FK
user_id      UUID FK → user CASCADE
item_type    TEXT CHECK('issue','project')
item_id      UUID NOT NULL
position     FLOAT DEFAULT 0
created_at   TIMESTAMPTZ
UNIQUE(workspace_id, user_id, item_type, item_id)
```

### `activity_log`
```sql
id           UUID PK
workspace_id UUID FK
issue_id     UUID FK → issue CASCADE NULLABLE
actor_type   TEXT CHECK('member','agent','system')
actor_id     UUID NULLABLE
action       TEXT NOT NULL
details      JSONB DEFAULT '{}'
created_at   TIMESTAMPTZ
```

### `runtime_usage`
```sql
id                 UUID PK
runtime_id         UUID FK
date               DATE NOT NULL
provider           TEXT NOT NULL
model              TEXT DEFAULT ''
input_tokens       BIGINT DEFAULT 0
output_tokens      BIGINT DEFAULT 0
cache_read_tokens  BIGINT DEFAULT 0
cache_write_tokens BIGINT DEFAULT 0
UNIQUE(runtime_id, date, provider, model)
```

---

## sqlc Query Patterns

Query files: `server/pkg/db/queries/*.sql`. Regenerate: `make sqlc`.

### Return annotations
```sql
-- name: GetIssue :one      → returns single row
-- name: ListIssues :many   → returns slice
-- name: DeleteIssue :exec  → no return value
-- name: CreateIssue :execrows  → returns row count
```

### Optional filter pattern
```sql
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
```

### Partial update pattern
```sql
SET title = COALESCE(sqlc.narg('title'), title),
    updated_at = now()
```

### Atomic task claim (FOR UPDATE SKIP LOCKED)
```sql
UPDATE agent_task_queue
SET status = 'dispatched', dispatched_at = now()
WHERE id = (
    SELECT id FROM agent_task_queue
    WHERE runtime_id = $1 AND status = 'queued'
    ORDER BY priority DESC, created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *
```

### Array membership
```sql
WHERE id = ANY($1::uuid[])
```

### Upsert
```sql
INSERT INTO table (...) VALUES (...)
ON CONFLICT (unique_key)
DO UPDATE SET ... RETURNING *
```

## Polymorphic Pattern

Used throughout for members vs. agents acting as creators, assignees, authors, subscribers, actors:
- Always two columns: `{role}_type TEXT CHECK('member','agent')` + `{role}_id UUID`
- Always indexed together: `(type_col, id_col)`
- System actors use `actor_type = 'system'` in activity logs

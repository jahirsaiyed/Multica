CREATE TABLE mcp_server (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  transport    TEXT NOT NULL CHECK (transport IN ('stdio', 'sse')),
  -- stdio transport fields
  command      TEXT,
  args         JSONB NOT NULL DEFAULT '[]',
  env          JSONB NOT NULL DEFAULT '{}',
  -- sse transport fields
  url          TEXT,
  headers      JSONB NOT NULL DEFAULT '{}',
  created_by   UUID REFERENCES "user"(id) ON DELETE SET NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX mcp_server_workspace_name_idx ON mcp_server(workspace_id, name);

CREATE TABLE agent_mcp_server (
  agent_id      UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
  mcp_server_id UUID NOT NULL REFERENCES mcp_server(id) ON DELETE CASCADE,
  PRIMARY KEY (agent_id, mcp_server_id)
);

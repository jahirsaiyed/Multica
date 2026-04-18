-- MCP Server CRUD

-- name: ListMCPServersByWorkspace :many
SELECT * FROM mcp_server
WHERE workspace_id = $1
ORDER BY name ASC;

-- name: GetMCPServerInWorkspace :one
SELECT * FROM mcp_server
WHERE id = $1 AND workspace_id = $2;

-- name: CreateMCPServer :one
INSERT INTO mcp_server (workspace_id, name, description, transport, command, args, env, url, headers, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateMCPServer :one
UPDATE mcp_server SET
    name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    transport   = COALESCE(sqlc.narg('transport'), transport),
    command     = COALESCE(sqlc.narg('command'), command),
    args        = COALESCE(sqlc.narg('args'), args),
    env         = COALESCE(sqlc.narg('env'), env),
    url         = COALESCE(sqlc.narg('url'), url),
    headers     = COALESCE(sqlc.narg('headers'), headers),
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMCPServer :exec
DELETE FROM mcp_server WHERE id = $1;

-- Agent-MCP Server junction

-- name: ListAgentMCPServersByWorkspace :many
SELECT ams.agent_id, ms.*
FROM mcp_server ms
JOIN agent_mcp_server ams ON ams.mcp_server_id = ms.id
WHERE ms.workspace_id = $1
ORDER BY ms.name ASC;

-- name: ListAgentMCPServers :many
SELECT ms.* FROM mcp_server ms
JOIN agent_mcp_server ams ON ams.mcp_server_id = ms.id
WHERE ams.agent_id = $1
ORDER BY ms.name ASC;

-- name: AddAgentMCPServer :exec
INSERT INTO agent_mcp_server (agent_id, mcp_server_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAllAgentMCPServers :exec
DELETE FROM agent_mcp_server WHERE agent_id = $1;

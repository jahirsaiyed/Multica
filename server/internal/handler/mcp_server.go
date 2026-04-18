package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// --- Response structs ---

type MCPServerResponse struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     *string           `json:"command,omitempty"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	URL         *string           `json:"url,omitempty"`
	Headers     map[string]string `json:"headers"`
	CreatedBy   *string           `json:"created_by"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

func mcpServerToResponse(m db.McpServer) MCPServerResponse {
	var args []string
	if m.Args != nil {
		json.Unmarshal(m.Args, &args)
	}
	if args == nil {
		args = []string{}
	}

	var env map[string]string
	if m.Env != nil {
		json.Unmarshal(m.Env, &env)
	}
	if env == nil {
		env = map[string]string{}
	}

	var headers map[string]string
	if m.Headers != nil {
		json.Unmarshal(m.Headers, &headers)
	}
	if headers == nil {
		headers = map[string]string{}
	}

	var command *string
	if m.Command.Valid {
		command = &m.Command.String
	}

	var url *string
	if m.Url.Valid {
		url = &m.Url.String
	}

	return MCPServerResponse{
		ID:          uuidToString(m.ID),
		WorkspaceID: uuidToString(m.WorkspaceID),
		Name:        m.Name,
		Description: m.Description,
		Transport:   m.Transport,
		Command:     command,
		Args:        args,
		Env:         env,
		URL:         url,
		Headers:     headers,
		CreatedBy:   uuidToPtr(m.CreatedBy),
		CreatedAt:   timestampToString(m.CreatedAt),
		UpdatedAt:   timestampToString(m.UpdatedAt),
	}
}

// --- Request structs ---

type CreateMCPServerRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     *string           `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	URL         *string           `json:"url"`
	Headers     map[string]string `json:"headers"`
}

type UpdateMCPServerRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	Transport   *string           `json:"transport"`
	Command     *string           `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	URL         *string           `json:"url"`
	Headers     map[string]string `json:"headers"`
}

type SetAgentMCPServersRequest struct {
	MCPServerIDs []string `json:"mcp_server_ids"`
}

// --- Helpers ---

func (h *Handler) loadMCPServerForUser(w http.ResponseWriter, r *http.Request, id string) (db.McpServer, bool) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return db.McpServer{}, false
	}

	server, err := h.Queries.GetMCPServerInWorkspace(r.Context(), db.GetMCPServerInWorkspaceParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "mcp server not found")
		return server, false
	}
	return server, true
}

// --- MCP Server CRUD ---

func (h *Handler) ListMCPServers(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	servers, err := h.Queries.ListMCPServersByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list mcp servers")
		return
	}

	resp := make([]MCPServerResponse, len(servers))
	for i, s := range servers {
		resp[i] = mcpServerToResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, ok := h.loadMCPServerForUser(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, mcpServerToResponse(server))
}

func (h *Handler) CreateMCPServer(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)

	creatorID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req CreateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Transport != "stdio" && req.Transport != "sse" {
		writeError(w, http.StatusBadRequest, "transport must be 'stdio' or 'sse'")
		return
	}

	args, _ := json.Marshal(req.Args)
	if req.Args == nil {
		args = []byte("[]")
	}
	env, _ := json.Marshal(req.Env)
	if req.Env == nil {
		env = []byte("{}")
	}
	headers, _ := json.Marshal(req.Headers)
	if req.Headers == nil {
		headers = []byte("{}")
	}

	var command pgtype.Text
	if req.Command != nil {
		command = pgtype.Text{String: *req.Command, Valid: true}
	}
	var url pgtype.Text
	if req.URL != nil {
		url = pgtype.Text{String: *req.URL, Valid: true}
	}

	server, err := h.Queries.CreateMCPServer(r.Context(), db.CreateMCPServerParams{
		WorkspaceID: parseUUID(workspaceID),
		Name:        req.Name,
		Description: req.Description,
		Transport:   req.Transport,
		Command:     command,
		Args:        args,
		Env:         env,
		Url:         url,
		Headers:     headers,
		CreatedBy:   parseUUID(creatorID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "an mcp server with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create mcp server: "+err.Error())
		return
	}

	resp := mcpServerToResponse(server)
	actorType, actorID := h.resolveActor(r, creatorID, workspaceID)
	h.publish(protocol.EventMCPServerCreated, workspaceID, actorType, actorID, map[string]any{"mcp_server": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, ok := h.loadMCPServerForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(server.WorkspaceID), "mcp server not found", "owner", "admin"); !ok {
		return
	}

	var req UpdateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateMCPServerParams{
		ID: parseUUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Transport != nil {
		if *req.Transport != "stdio" && *req.Transport != "sse" {
			writeError(w, http.StatusBadRequest, "transport must be 'stdio' or 'sse'")
			return
		}
		params.Transport = pgtype.Text{String: *req.Transport, Valid: true}
	}
	if req.Command != nil {
		params.Command = pgtype.Text{String: *req.Command, Valid: true}
	}
	if req.Args != nil {
		params.Args, _ = json.Marshal(req.Args)
	}
	if req.Env != nil {
		params.Env, _ = json.Marshal(req.Env)
	}
	if req.URL != nil {
		params.Url = pgtype.Text{String: *req.URL, Valid: true}
	}
	if req.Headers != nil {
		params.Headers, _ = json.Marshal(req.Headers)
	}

	updated, err := h.Queries.UpdateMCPServer(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "an mcp server with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update mcp server: "+err.Error())
		return
	}

	resp := mcpServerToResponse(updated)
	wsID := resolveWorkspaceID(r)
	actorType, actorID := h.resolveActor(r, requestUserID(r), wsID)
	h.publish(protocol.EventMCPServerUpdated, wsID, actorType, actorID, map[string]any{"mcp_server": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	server, ok := h.loadMCPServerForUser(w, r, id)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(server.WorkspaceID), "mcp server not found", "owner", "admin"); !ok {
		return
	}

	if err := h.Queries.DeleteMCPServer(r.Context(), parseUUID(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mcp server")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(server.WorkspaceID))
	h.publish(protocol.EventMCPServerDeleted, uuidToString(server.WorkspaceID), actorType, actorID, map[string]any{"mcp_server_id": id})
	w.WriteHeader(http.StatusNoContent)
}

// --- Agent-MCP Server junction ---

func (h *Handler) ListAgentMCPServers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}

	servers, err := h.Queries.ListAgentMCPServers(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent mcp servers")
		return
	}

	resp := make([]MCPServerResponse, len(servers))
	for i, s := range servers {
		resp[i] = mcpServerToResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) SetAgentMCPServers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, id)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	var req SetAgentMCPServersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	qtx := h.Queries.WithTx(tx)

	if err := qtx.RemoveAllAgentMCPServers(r.Context(), agent.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear agent mcp servers")
		return
	}

	for _, serverID := range req.MCPServerIDs {
		if err := qtx.AddAgentMCPServer(r.Context(), db.AddAgentMCPServerParams{
			AgentID:     agent.ID,
			McpServerID: parseUUID(serverID),
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add agent mcp server: "+err.Error())
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	servers, err := h.Queries.ListAgentMCPServers(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent mcp servers")
		return
	}

	resp := make([]MCPServerResponse, len(servers))
	for i, s := range servers {
		resp[i] = mcpServerToResponse(s)
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(agent.WorkspaceID))
	h.publish(protocol.EventAgentStatus, uuidToString(agent.WorkspaceID), actorType, actorID, map[string]any{"agent_id": uuidToString(agent.ID), "mcp_servers": resp})
	writeJSON(w, http.StatusOK, resp)
}

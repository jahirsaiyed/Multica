"use client";

import { useState } from "react";
import { Plus, Plug, Trash2 } from "lucide-react";
import type { Agent } from "@multica/core/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { mcpServerListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import { useQuery, useQueryClient } from "@tanstack/react-query";

export function MCPTab({
  agent,
}: {
  agent: Agent;
}) {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const { data: workspaceMCPServers = [] } = useQuery(mcpServerListOptions(wsId));
  const [saving, setSaving] = useState(false);
  const [showPicker, setShowPicker] = useState(false);

  const agentMCPIds = new Set(agent.mcp_servers.map((s) => s.id));
  const availableServers = workspaceMCPServers.filter((s) => !agentMCPIds.has(s.id));

  const handleAdd = async (serverId: string) => {
    setSaving(true);
    try {
      const newIds = [...agent.mcp_servers.map((s) => s.id), serverId];
      await api.setAgentMCPServers(agent.id, { mcp_server_ids: newIds });
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add MCP server");
    } finally {
      setSaving(false);
      setShowPicker(false);
    }
  };

  const handleRemove = async (serverId: string) => {
    setSaving(true);
    try {
      const newIds = agent.mcp_servers.filter((s) => s.id !== serverId).map((s) => s.id);
      await api.setAgentMCPServers(agent.id, { mcp_server_ids: newIds });
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove MCP server");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">MCP Servers</h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            MCP servers assigned to this agent. Manage servers on the MCP Servers page.
          </p>
        </div>
        <Button
          variant="outline"
          size="xs"
          onClick={() => setShowPicker(true)}
          disabled={saving || availableServers.length === 0}
        >
          <Plus className="h-3 w-3" />
          Add Server
        </Button>
      </div>

      {agent.mcp_servers.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-12">
          <Plug className="h-8 w-8 text-muted-foreground/40" />
          <p className="mt-3 text-sm text-muted-foreground">No MCP servers assigned</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Add MCP servers from the workspace to this agent.
          </p>
          {availableServers.length > 0 && (
            <Button
              onClick={() => setShowPicker(true)}
              size="xs"
              className="mt-3"
              disabled={saving}
            >
              <Plus className="h-3 w-3" />
              Add Server
            </Button>
          )}
        </div>
      ) : (
        <div className="space-y-2">
          {agent.mcp_servers.map((server) => (
            <div
              key={server.id}
              className="flex items-center gap-3 rounded-lg border px-4 py-3"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <Plug className="h-4 w-4 text-muted-foreground" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">{server.name}</span>
                  <Badge variant="secondary" className="text-xs px-1.5 py-0">
                    {server.transport}
                  </Badge>
                </div>
                {server.description && (
                  <div className="text-xs text-muted-foreground truncate">
                    {server.description}
                  </div>
                )}
              </div>
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => handleRemove(server.id)}
                disabled={saving}
                className="text-muted-foreground hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {/* MCP Server Picker Dialog */}
      {showPicker && (
        <Dialog open onOpenChange={(v) => { if (!v) setShowPicker(false); }}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="text-sm">Add MCP Server</DialogTitle>
              <DialogDescription className="text-xs">
                Select an MCP server to assign to this agent.
              </DialogDescription>
            </DialogHeader>
            <div className="max-h-64 overflow-y-auto space-y-1">
              {availableServers.map((server) => (
                <button
                  key={server.id}
                  onClick={() => handleAdd(server.id)}
                  disabled={saving}
                  className="flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm transition-colors hover:bg-accent/50"
                >
                  <Plug className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{server.name}</span>
                      <Badge variant="secondary" className="text-xs px-1.5 py-0">
                        {server.transport}
                      </Badge>
                    </div>
                    {server.description && (
                      <div className="text-xs text-muted-foreground truncate">
                        {server.description}
                      </div>
                    )}
                  </div>
                </button>
              ))}
              {availableServers.length === 0 && (
                <p className="py-6 text-center text-xs text-muted-foreground">
                  All workspace MCP servers are already assigned.
                </p>
              )}
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setShowPicker(false)}>
                Cancel
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

"use client";

import { useState, useEffect } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import { Plug, Plus, Trash2, Save, AlertCircle } from "lucide-react";
import type { MCPServer, CreateMCPServerRequest, UpdateMCPServerRequest, MCPTransport } from "@multica/core/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@multica/ui/components/ui/resizable";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { toast } from "sonner";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { api } from "@multica/core/api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { mcpServerListOptions, workspaceKeys } from "@multica/core/workspace/queries";

// ---------------------------------------------------------------------------
// Key-Value Editor
// ---------------------------------------------------------------------------

function KVEditor({
  label,
  value,
  onChange,
  keyPlaceholder = "KEY",
  valuePlaceholder = "value",
}: {
  label: string;
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}) {
  const entries = Object.entries(value);
  const [newKey, setNewKey] = useState("");
  const [newVal, setNewVal] = useState("");

  const handleAdd = () => {
    if (!newKey.trim()) return;
    onChange({ ...value, [newKey.trim()]: newVal });
    setNewKey("");
    setNewVal("");
  };

  const handleRemove = (k: string) => {
    const next = { ...value };
    delete next[k];
    onChange(next);
  };

  return (
    <div className="space-y-2">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      {entries.length > 0 && (
        <div className="space-y-1">
          {entries.map(([k, v]) => (
            <div key={k} className="flex items-center gap-2">
              <Input value={k} readOnly className="h-7 text-xs font-mono flex-1" />
              <span className="text-muted-foreground text-xs">=</span>
              <Input value={v} readOnly className="h-7 text-xs font-mono flex-1" />
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={() => handleRemove(k)}
                className="text-muted-foreground hover:text-destructive shrink-0"
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-center gap-2">
        <Input
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          placeholder={keyPlaceholder}
          className="h-7 text-xs font-mono flex-1"
          onKeyDown={(e) => e.key === "Enter" && handleAdd()}
        />
        <span className="text-muted-foreground text-xs">=</span>
        <Input
          value={newVal}
          onChange={(e) => setNewVal(e.target.value)}
          placeholder={valuePlaceholder}
          className="h-7 text-xs font-mono flex-1"
          onKeyDown={(e) => e.key === "Enter" && handleAdd()}
        />
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={handleAdd}
          disabled={!newKey.trim()}
          className="text-muted-foreground shrink-0"
        >
          <Plus className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Create MCP Server Dialog
// ---------------------------------------------------------------------------

function CreateMCPServerDialog({
  onClose,
  onCreate,
}: {
  onClose: () => void;
  onCreate: (data: CreateMCPServerRequest) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [transport, setTransport] = useState<MCPTransport>("stdio");
  const [command, setCommand] = useState("");
  const [args, setArgs] = useState("");
  const [env, setEnv] = useState<Record<string, string>>({});
  const [url, setUrl] = useState("");
  const [headers, setHeaders] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);

  const handleCreate = async () => {
    if (!name.trim()) return;
    setLoading(true);
    try {
      const parsedArgs = args.trim()
        ? args.split(/\s+/).filter(Boolean)
        : [];
      await onCreate({
        name: name.trim(),
        description: description.trim(),
        transport,
        command: transport === "stdio" ? command.trim() || undefined : undefined,
        args: transport === "stdio" ? parsedArgs : [],
        env: transport === "stdio" ? env : {},
        url: transport === "sse" ? url.trim() || undefined : undefined,
        headers: transport === "sse" ? headers : {},
      });
      onClose();
    } catch {
      setLoading(false);
    }
  };

  return (
    <Dialog open onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Add MCP Server</DialogTitle>
          <DialogDescription>
            Configure a Model Context Protocol server for your agents.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs text-muted-foreground">Name</Label>
              <Input
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Filesystem Tools"
                className="mt-1"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Transport</Label>
              <div className="mt-1 flex gap-2">
                {(["stdio", "sse"] as MCPTransport[]).map((t) => (
                  <button
                    key={t}
                    onClick={() => setTransport(t)}
                    className={`flex-1 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
                      transport === t
                        ? "border-primary bg-primary/10 text-primary"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {t === "stdio" ? "stdio" : "HTTP/SSE"}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div>
            <Label className="text-xs text-muted-foreground">Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What tools does this server provide?"
              className="mt-1"
            />
          </div>

          {transport === "stdio" ? (
            <>
              <div>
                <Label className="text-xs text-muted-foreground">Command</Label>
                <Input
                  value={command}
                  onChange={(e) => setCommand(e.target.value)}
                  placeholder="e.g. npx"
                  className="mt-1 font-mono text-sm"
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground">Arguments (space-separated)</Label>
                <Input
                  value={args}
                  onChange={(e) => setArgs(e.target.value)}
                  placeholder="e.g. @modelcontextprotocol/server-filesystem /tmp"
                  className="mt-1 font-mono text-sm"
                />
              </div>
              <KVEditor label="Environment Variables" value={env} onChange={setEnv} />
            </>
          ) : (
            <>
              <div>
                <Label className="text-xs text-muted-foreground">URL</Label>
                <Input
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://mcp.example.com/sse"
                  className="mt-1 font-mono text-sm"
                />
              </div>
              <KVEditor label="Headers" value={headers} onChange={setHeaders} keyPlaceholder="Header-Name" valuePlaceholder="value" />
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={handleCreate} disabled={loading || !name.trim()}>
            {loading ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// MCP Server List Item
// ---------------------------------------------------------------------------

function MCPServerListItem({
  server,
  isSelected,
  onClick,
}: {
  server: MCPServer;
  isSelected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors ${
        isSelected ? "bg-accent" : "hover:bg-accent/50"
      }`}
    >
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted">
        <Plug className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{server.name}</div>
        {server.description && (
          <div className="mt-0.5 truncate text-xs text-muted-foreground">
            {server.description}
          </div>
        )}
      </div>
      <Badge variant={server.transport === "stdio" ? "secondary" : "outline"} className="shrink-0 text-[10px]">
        {server.transport}
      </Badge>
    </button>
  );
}

// ---------------------------------------------------------------------------
// MCP Server Detail
// ---------------------------------------------------------------------------

function MCPServerDetail({
  server,
  onUpdate,
  onDelete,
}: {
  server: MCPServer;
  onUpdate: (id: string, data: UpdateMCPServerRequest) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}) {
  const [name, setName] = useState(server.name);
  const [description, setDescription] = useState(server.description);
  const [transport, setTransport] = useState<MCPTransport>(server.transport);
  const [command, setCommand] = useState(server.command ?? "");
  const [argsText, setArgsText] = useState((server.args ?? []).join(" "));
  const [env, setEnv] = useState<Record<string, string>>(server.env ?? {});
  const [url, setUrl] = useState(server.url ?? "");
  const [headers, setHeaders] = useState<Record<string, string>>(server.headers ?? {});
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    setName(server.name);
    setDescription(server.description);
    setTransport(server.transport);
    setCommand(server.command ?? "");
    setArgsText((server.args ?? []).join(" "));
    setEnv(server.env ?? {});
    setUrl(server.url ?? "");
    setHeaders(server.headers ?? {});
  }, [server.id]);

  const isDirty =
    name !== server.name ||
    description !== server.description ||
    transport !== server.transport ||
    command !== (server.command ?? "") ||
    argsText !== (server.args ?? []).join(" ") ||
    JSON.stringify(env) !== JSON.stringify(server.env ?? {}) ||
    url !== (server.url ?? "") ||
    JSON.stringify(headers) !== JSON.stringify(server.headers ?? {});

  const handleSave = async () => {
    setSaving(true);
    try {
      const parsedArgs = argsText.trim() ? argsText.split(/\s+/).filter(Boolean) : [];
      await onUpdate(server.id, {
        name: name.trim(),
        description: description.trim(),
        transport,
        command: transport === "stdio" ? command.trim() || undefined : undefined,
        args: transport === "stdio" ? parsedArgs : [],
        env: transport === "stdio" ? env : {},
        url: transport === "sse" ? url.trim() || undefined : undefined,
        headers: transport === "sse" ? headers : {},
      });
    } catch {
      // toast handled by parent
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Plug className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="grid grid-cols-2 gap-3 flex-1 min-w-0">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="h-8 text-sm font-medium"
              placeholder="Server name"
            />
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="h-8 text-sm"
              placeholder="Description"
            />
          </div>
        </div>
        <div className="flex items-center gap-2 ml-3">
          {isDirty && (
            <Button onClick={handleSave} disabled={saving || !name.trim()} size="xs">
              <Save className="h-3 w-3" />
              {saving ? "Saving..." : "Save"}
            </Button>
          )}
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => setConfirmDelete(true)}
                  className="text-muted-foreground hover:text-destructive"
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              }
            />
            <TooltipContent>Delete server</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* Form */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {/* Transport selector */}
        <div>
          <Label className="text-xs text-muted-foreground">Transport</Label>
          <div className="mt-1.5 flex gap-2 w-48">
            {(["stdio", "sse"] as MCPTransport[]).map((t) => (
              <button
                key={t}
                onClick={() => setTransport(t)}
                className={`flex-1 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
                  transport === t
                    ? "border-primary bg-primary/10 text-primary"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {t === "stdio" ? "stdio" : "HTTP/SSE"}
              </button>
            ))}
          </div>
        </div>

        {transport === "stdio" ? (
          <>
            <div>
              <Label className="text-xs text-muted-foreground">Command</Label>
              <Input
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder="e.g. npx"
                className="mt-1 font-mono text-sm"
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground">Arguments (space-separated)</Label>
              <Textarea
                value={argsText}
                onChange={(e) => setArgsText(e.target.value)}
                placeholder="e.g. @modelcontextprotocol/server-filesystem /tmp"
                className="mt-1 font-mono text-sm min-h-[60px]"
                rows={2}
              />
            </div>
            <KVEditor label="Environment Variables" value={env} onChange={setEnv} />
          </>
        ) : (
          <>
            <div>
              <Label className="text-xs text-muted-foreground">URL</Label>
              <Input
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://mcp.example.com/sse"
                className="mt-1 font-mono text-sm"
              />
            </div>
            <KVEditor label="Headers" value={headers} onChange={setHeaders} keyPlaceholder="Header-Name" valuePlaceholder="value" />
          </>
        )}
      </div>

      {/* Delete Confirmation */}
      {confirmDelete && (
        <Dialog open onOpenChange={(v) => { if (!v) setConfirmDelete(false); }}>
          <DialogContent className="max-w-sm" showCloseButton={false}>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-destructive/10">
                <AlertCircle className="h-5 w-5 text-destructive" />
              </div>
              <DialogHeader className="flex-1 gap-1">
                <DialogTitle className="text-sm font-semibold">Delete MCP server?</DialogTitle>
                <DialogDescription className="text-xs">
                  This will permanently delete &quot;{server.name}&quot; and remove it from all agents.
                </DialogDescription>
              </DialogHeader>
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={() => setConfirmDelete(false)}>
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => {
                  setConfirmDelete(false);
                  onDelete(server.id);
                }}
              >
                Delete
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function MCPServersPage() {
  const isLoading = useAuthStore((s) => s.isLoading);
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const { data: servers = [] } = useQuery(mcpServerListOptions(wsId));
  const [selectedId, setSelectedId] = useState<string>("");
  const [showCreate, setShowCreate] = useState(false);
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({
    id: "multica_mcp_servers_layout",
  });

  useEffect(() => {
    if (servers.length > 0 && !selectedId) {
      setSelectedId(servers[0]!.id);
    }
  }, [servers, selectedId]);

  const handleCreate = async (data: CreateMCPServerRequest) => {
    const server = await api.createMCPServer(data);
    qc.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) });
    setSelectedId(server.id);
    toast.success("MCP server created");
  };

  const handleUpdate = async (id: string, data: UpdateMCPServerRequest) => {
    try {
      await api.updateMCPServer(id, data);
      qc.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) });
      toast.success("MCP server saved");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to save MCP server");
      throw e;
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteMCPServer(id);
      if (selectedId === id) {
        const remaining = servers.filter((s) => s.id !== id);
        setSelectedId(remaining[0]?.id ?? "");
      }
      qc.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) });
      toast.success("MCP server deleted");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete MCP server");
    }
  };

  const selected = servers.find((s) => s.id === selectedId) ?? null;

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0">
        <div className="w-72 border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-6 w-6 rounded" />
          </div>
          <div className="divide-y">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 px-4 py-3">
                <Skeleton className="h-8 w-8 rounded-lg" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-3 w-40" />
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex-1 p-6 space-y-4">
          <Skeleton className="h-8 w-48" />
          <Skeleton className="h-4 w-64" />
          <Skeleton className="h-4 w-56" />
        </div>
      </div>
    );
  }

  return (
    <ResizablePanelGroup
      orientation="horizontal"
      className="flex-1 min-h-0"
      defaultLayout={defaultLayout}
      onLayoutChanged={onLayoutChanged}
    >
      <ResizablePanel id="list" defaultSize={280} minSize={240} maxSize={400} groupResizeBehavior="preserve-pixel-size">
        <div className="overflow-y-auto h-full border-r">
          <div className="flex h-12 items-center justify-between border-b px-4">
            <h1 className="text-sm font-semibold">MCP Servers</h1>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => setShowCreate(true)}
                  >
                    <Plus className="h-4 w-4 text-muted-foreground" />
                  </Button>
                }
              />
              <TooltipContent side="bottom">Add MCP server</TooltipContent>
            </Tooltip>
          </div>
          {servers.length === 0 ? (
            <div className="flex flex-col items-center justify-center px-4 py-12">
              <Plug className="h-8 w-8 text-muted-foreground/40" />
              <p className="mt-3 text-sm text-muted-foreground">No MCP servers yet</p>
              <p className="mt-1 text-xs text-muted-foreground text-center">
                MCP servers provide tools to your agents at runtime.
              </p>
              <Button
                onClick={() => setShowCreate(true)}
                size="xs"
                className="mt-3"
              >
                <Plus className="h-3 w-3" />
                Add MCP Server
              </Button>
            </div>
          ) : (
            <div className="divide-y">
              {servers.map((server) => (
                <MCPServerListItem
                  key={server.id}
                  server={server}
                  isSelected={server.id === selectedId}
                  onClick={() => setSelectedId(server.id)}
                />
              ))}
            </div>
          )}
        </div>
      </ResizablePanel>

      <ResizableHandle />

      <ResizablePanel id="detail" minSize="50%">
        <div className="flex-1 overflow-hidden h-full">
          {selected ? (
            <MCPServerDetail
              key={selected.id}
              server={selected}
              onUpdate={handleUpdate}
              onDelete={handleDelete}
            />
          ) : (
            <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
              <Plug className="h-10 w-10 text-muted-foreground/30" />
              <p className="mt-3 text-sm">Select an MCP server to view details</p>
              <Button
                onClick={() => setShowCreate(true)}
                size="xs"
                className="mt-3"
              >
                <Plus className="h-3 w-3" />
                Add MCP Server
              </Button>
            </div>
          )}
        </div>
      </ResizablePanel>

      {showCreate && (
        <CreateMCPServerDialog
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </ResizablePanelGroup>
  );
}

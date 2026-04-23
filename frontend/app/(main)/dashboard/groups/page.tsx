"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  FolderTree,
  Trash2,
  Pencil,
  Check,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import type { components } from "@/lib/openapi-types";

type MonitorGroup = components["schemas"]["MonitorGroup"];

export default function GroupsDashboardPage() {
  const [groups, setGroups] = useState<MonitorGroup[] | null>(null);
  const [name, setName] = useState("");
  const [ordering, setOrdering] = useState<number | "">(0);
  const [busy, setBusy] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editName, setEditName] = useState("");
  const [editOrdering, setEditOrdering] = useState(0);

  async function load() {
    const res = await fetch("/api/monitor-groups", { credentials: "include" });
    if (res.ok) setGroups((await res.json()) as MonitorGroup[]);
  }

  useEffect(() => {
    load();
  }, []);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await fetch("/api/monitor-groups", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, ordering: Number(ordering) || 0 }),
        credentials: "include",
      });
      if (res.status === 402) {
        toast.error("Monitor groups are a Pro feature.");
        return;
      }
      if (!res.ok) {
        toast.error(`Create failed (HTTP ${res.status}).`);
        return;
      }
      setName("");
      setOrdering(0);
      toast.success("Group created.");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function save(id: number) {
    const res = await fetch(`/api/monitor-groups/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: editName, ordering: editOrdering }),
      credentials: "include",
    });
    if (res.ok) {
      toast.success("Saved.");
      setEditingId(null);
      await load();
    } else {
      toast.error(`Save failed (HTTP ${res.status}).`);
    }
  }

  async function remove(id: number) {
    const res = await fetch(`/api/monitor-groups/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (res.ok) {
      toast.success("Group deleted.");
      await load();
    } else {
      toast.error(`Delete failed (HTTP ${res.status}).`);
    }
  }

  return (
    <div className="container mx-auto px-4 py-12 max-w-3xl">
      <Link
        href="/dashboard"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-6"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to dashboard
      </Link>

      <div className="flex items-center gap-3">
        <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary">
          <FolderTree className="h-5 w-5" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight">Monitor groups</h1>
      </div>
      <p className="mt-3 text-sm text-muted-foreground max-w-xl">
        Group monitors into collapsible sections on your public status
        page. Pro feature. Deleting a group unparents its monitors (they
        move to the ungrouped default section, not deleted).
      </p>

      <form
        onSubmit={create}
        className="mt-8 rounded-lg border border-border/60 bg-card p-5 space-y-4"
      >
        <div className="grid sm:grid-cols-[1fr_160px] gap-3">
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              placeholder="Public API"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              disabled={busy}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="ordering">Order</Label>
            <Input
              id="ordering"
              type="number"
              min={0}
              max={999}
              value={ordering}
              onChange={(e) =>
                setOrdering(e.target.value === "" ? "" : Number(e.target.value))
              }
              disabled={busy}
            />
          </div>
        </div>
        <Button type="submit" disabled={busy || !name}>
          {busy ? "Creating…" : "Create group"}
        </Button>
      </form>

      <section className="mt-10">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-3">
          Your groups
        </h2>
        {groups === null ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : groups.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No groups yet. Create one above to start organising monitors.
          </p>
        ) : (
          <ul className="space-y-3">
            {groups.map((g) => {
              const editing = editingId === g.id;
              return (
                <li
                  key={g.id}
                  className="rounded-lg border border-border/60 bg-card p-4 flex items-start justify-between gap-4 flex-wrap"
                >
                  {editing ? (
                    <div className="flex-1 grid sm:grid-cols-[1fr_120px] gap-2 min-w-0">
                      <Input
                        value={editName}
                        onChange={(e) => setEditName(e.target.value)}
                      />
                      <Input
                        type="number"
                        value={editOrdering}
                        onChange={(e) => setEditOrdering(Number(e.target.value))}
                      />
                    </div>
                  ) : (
                    <div className="min-w-0">
                      <div className="font-medium">{g.name}</div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        Order {g.ordering} · created{" "}
                        {new Date(g.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  )}

                  <div className="flex items-center gap-2 shrink-0">
                    {editing ? (
                      <>
                        <button
                          onClick={() => save(g.id)}
                          className="text-primary hover:text-primary/80"
                          aria-label="Save"
                        >
                          <Check className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => setEditingId(null)}
                          className="text-muted-foreground hover:text-foreground"
                          aria-label="Cancel"
                        >
                          <X className="h-4 w-4" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button
                          onClick={() => {
                            setEditingId(g.id);
                            setEditName(g.name);
                            setEditOrdering(g.ordering);
                          }}
                          className="text-muted-foreground hover:text-foreground"
                          aria-label={`Edit ${g.name}`}
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => remove(g.id)}
                          className="text-muted-foreground hover:text-destructive"
                          aria-label={`Delete ${g.name}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}

"use client";

import { useEffect, useState, useTransition } from "react";
import { FolderTree } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import type { components } from "@/lib/openapi-types";

type MonitorGroup = components["schemas"]["MonitorGroup"];

// Special sentinel — the Select can't hold null, so "unassigned" is a
// string the component knows to translate back to null on submit.
const UNASSIGNED = "__unassigned__";

export function MonitorGroupPicker({
  monitorId,
  initialGroupId,
}: {
  monitorId: string;
  initialGroupId: number | null;
}) {
  const [groups, setGroups] = useState<MonitorGroup[] | null>(null);
  const [value, setValue] = useState<string>(
    initialGroupId ? String(initialGroupId) : UNASSIGNED,
  );
  const [isPending, start] = useTransition();

  useEffect(() => {
    fetch("/api/monitor-groups", { credentials: "include" })
      .then((r) => (r.ok ? r.json() : []))
      .then((g: MonitorGroup[]) => setGroups(g))
      .catch(() => setGroups([]));
  }, []);

  async function save(next: string) {
    const prev = value;
    setValue(next);
    const groupId = next === UNASSIGNED ? null : Number(next);
    const res = await fetch(`/api/monitors/${monitorId}/group`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group_id: groupId }),
      credentials: "include",
    });
    if (res.status === 402) {
      setValue(prev);
      toast.error("Monitor groups are a Pro feature.");
      return;
    }
    if (!res.ok) {
      setValue(prev);
      toast.error(`Save failed (HTTP ${res.status}).`);
      return;
    }
    toast.success(next === UNASSIGNED ? "Monitor unassigned." : "Moved to group.");
  }

  if (!groups || groups.length === 0) return null; // Nothing to pick

  return (
    <div className="flex items-center gap-2">
      <FolderTree className="h-4 w-4 text-muted-foreground" />
      <Select
        value={value}
        onValueChange={(v) => v && start(() => save(v))}
        disabled={isPending}
      >
        <SelectTrigger className="w-52">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={UNASSIGNED}>Ungrouped</SelectItem>
          {groups.map((g) => (
            <SelectItem key={g.id} value={String(g.id)}>
              {g.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";

const STATES = [
  { value: "investigating", label: "Investigating" },
  { value: "identified", label: "Identified" },
  { value: "monitoring", label: "Monitoring" },
  { value: "resolved", label: "Resolved" },
] as const;

export function IncidentUpdateForm({
  incidentId,
  currentState,
}: {
  incidentId: number;
  currentState: string;
}) {
  const [state, setState] = useState<string>(currentState);
  const [body, setBody] = useState("");
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!body.trim()) return;

    const res = await fetch(`/api/incidents/${incidentId}/state`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ state, body }),
      credentials: "include",
    });

    if (res.status === 402) {
      toast.error("Posting incident updates requires a Pro subscription.");
      return;
    }
    if (res.status === 409) {
      toast.error(`Invalid state transition (current state is ${currentState}).`);
      return;
    }
    if (!res.ok) {
      toast.error(`Failed to post update (HTTP ${res.status}).`);
      return;
    }

    setBody("");
    toast.success("Update posted.");
    startTransition(() => router.refresh());
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <div>
        <label
          htmlFor="state"
          className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground"
        >
          New state
        </label>
        <Select value={state} onValueChange={setState}>
          <SelectTrigger id="state" className="w-60">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {STATES.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                {s.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div>
        <label
          htmlFor="body"
          className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground"
        >
          Update text
        </label>
        <Textarea
          id="body"
          placeholder="What's happening now?"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={3}
          required
        />
      </div>

      <Button type="submit" disabled={isPending || !body.trim()}>
        {isPending ? "Posting…" : "Post update"}
      </Button>
    </form>
  );
}

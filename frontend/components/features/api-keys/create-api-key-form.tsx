"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useCreateAPIKey } from "@/lib/mutations";
import type { components } from "@/lib/openapi-types";

type Scope =
  components["schemas"]["CreateAPIKeyRequest"]["scopes"][number];

const SCOPES: { value: Scope; label: string; description: string }[] = [
  { value: "monitors:read", label: "monitors:read", description: "List and view monitors" },
  { value: "monitors:write", label: "monitors:write", description: "Create, edit, delete monitors" },
  { value: "channels:read", label: "channels:read", description: "List and view channels" },
  { value: "channels:write", label: "channels:write", description: "Create, edit, delete channels" },
];

export function CreateAPIKeyForm({
  onCreated,
}: {
  onCreated: (rawKey: string) => void;
}) {
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<Scope[]>(["monitors:read"]);
  const [expiresInDays, setExpiresInDays] = useState("0");
  const create = useCreateAPIKey();

  const canSubmit = name.trim().length > 0 && scopes.length > 0;

  const toggleScope = (s: Scope) =>
    setScopes((prev) =>
      prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s],
    );

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const res = await create.mutateAsync({
      name: name.trim(),
      scopes,
      expires_in_days: Number(expiresInDays),
    });
    if (res.raw_key) onCreated(res.raw_key);
    setName("");
    setScopes(["monitors:read"]);
    setExpiresInDays("0");
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Create API key</CardTitle>
        <CardDescription>
          Programmatic access with scoped permissions. The key value is shown only once.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Deploy bot, CI pipeline, …"
              required
            />
          </div>

          <div className="space-y-2">
            <Label>Scopes</Label>
            <div className="space-y-2">
              {SCOPES.map((s) => (
                <label
                  key={s.value}
                  className="flex items-start gap-3 rounded-md border border-border/60 p-3 cursor-pointer hover:bg-accent/30 transition-colors"
                >
                  <Checkbox
                    checked={scopes.includes(s.value)}
                    onCheckedChange={() => toggleScope(s.value)}
                  />
                  <div className="min-w-0">
                    <div className="text-sm font-mono">{s.label}</div>
                    <div className="text-xs text-muted-foreground">{s.description}</div>
                  </div>
                </label>
              ))}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="expires">Expires in</Label>
            <Select value={expiresInDays} onValueChange={(v) => setExpiresInDays(v ?? "0")}>
              <SelectTrigger id="expires">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">Never</SelectItem>
                <SelectItem value="7">7 days</SelectItem>
                <SelectItem value="30">30 days</SelectItem>
                <SelectItem value="90">90 days</SelectItem>
                <SelectItem value="365">1 year</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Button type="submit" disabled={!canSubmit || create.isPending}>
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

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
import { useLocale } from "@/components/i18n/locale-provider";

type Scope =
  components["schemas"]["CreateAPIKeyRequest"]["scopes"][number];

export function CreateAPIKeyForm({
  onCreated,
}: {
  onCreated: (rawKey: string) => void;
}) {
  const { dict } = useLocale();
  const t = dict.api_keys;
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<Scope[]>(["monitors:read"]);
  const [expiresInDays, setExpiresInDays] = useState("0");
  const create = useCreateAPIKey();

  const SCOPES: { value: Scope; label: string; description: string }[] = [
    { value: "monitors:read", label: "monitors:read", description: t.scope_monitors_read_desc },
    { value: "monitors:write", label: "monitors:write", description: t.scope_monitors_write_desc },
    { value: "channels:read", label: "channels:read", description: t.scope_channels_read_desc },
    { value: "channels:write", label: "channels:write", description: t.scope_channels_write_desc },
  ];

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
        <CardTitle className="text-lg">{t.form_card_title}</CardTitle>
        <CardDescription>{t.form_card_desc}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">{t.field_name}</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t.field_name_placeholder}
              required
            />
          </div>

          <div className="space-y-2">
            <Label>{t.field_scopes}</Label>
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
            <Label htmlFor="expires">{t.field_expires}</Label>
            <Select value={expiresInDays} onValueChange={(v) => setExpiresInDays(v ?? "0")}>
              <SelectTrigger id="expires">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">{t.expires_never}</SelectItem>
                <SelectItem value="7">{t.expires_7d}</SelectItem>
                <SelectItem value="30">{t.expires_30d}</SelectItem>
                <SelectItem value="90">{t.expires_90d}</SelectItem>
                <SelectItem value="365">{t.expires_365d}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Button type="submit" disabled={!canSubmit || create.isPending}>
            {create.isPending ? t.creating : t.create}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

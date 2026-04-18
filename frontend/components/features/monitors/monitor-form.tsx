"use client";

import { useRouter } from "next/navigation";
import Link from "next/link";
import { useForm, FormProvider } from "react-hook-form";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
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
import { useCreateMonitor, useUpdateMonitor } from "@/lib/mutations";
import {
  useMonitorTypes,
  useChannels,
  type MonitorDetail,
} from "@/lib/queries";
import { ConfigFields } from "./config-fields";
import { ArrowLeft } from "lucide-react";

type FormValues = {
  name: string;
  type: string;
  check_config: Record<string, unknown>;
  interval_seconds: number;
  alert_after_failures: number;
  is_public: boolean;
  is_paused?: boolean;
  channel_ids?: string[];
};

interface Props {
  mode: "create" | "edit";
  initial?: MonitorDetail;
}

export function MonitorForm({ mode, initial }: Props) {
  const router = useRouter();
  const { data: monitorTypes } = useMonitorTypes();
  const { data: channels } = useChannels();
  const create = useCreateMonitor();
  const update = useUpdateMonitor(initial?.id ?? "");

  const methods = useForm<FormValues>({
    defaultValues: {
      name: initial?.name ?? "",
      type: initial?.type ?? "",
      check_config: (initial?.check_config as Record<string, unknown>) ?? {},
      interval_seconds: initial?.interval_seconds ?? 300,
      alert_after_failures: initial?.alert_after_failures ?? 3,
      is_public: initial?.is_public ?? false,
      is_paused: initial?.is_paused ?? false,
      channel_ids: [],
    },
  });

  const selectedType = methods.watch("type");
  const typeInfo = monitorTypes?.find((t) => t.type === selectedType);

  const onSubmit = methods.handleSubmit(async (values) => {
    if (mode === "create") {
      const created = await create.mutateAsync({
        name: values.name,
        type: values.type,
        check_config: values.check_config,
        interval_seconds: values.interval_seconds,
        alert_after_failures: values.alert_after_failures,
        is_public: values.is_public,
      });
      if (created.id) router.push(`/monitors/${created.id}`);
    } else {
      await update.mutateAsync({
        name: values.name,
        check_config: values.check_config,
        interval_seconds: values.interval_seconds,
        alert_after_failures: values.alert_after_failures,
        is_paused: values.is_paused,
        is_public: values.is_public,
      });
      if (initial?.id) router.push(`/monitors/${initial.id}`);
    }
  });

  const pending = create.isPending || update.isPending;

  return (
    <div className="container mx-auto px-4 py-8 max-w-xl">
      <Link
        href={initial?.id ? `/monitors/${initial.id}` : "/dashboard"}
        className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="mr-1 h-4 w-4" />
        {initial?.id ? "Back to monitor" : "Back to dashboard"}
      </Link>

      <Card>
        <CardHeader>
          <CardTitle>{mode === "create" ? "New monitor" : `Edit ${initial?.name ?? "monitor"}`}</CardTitle>
          <CardDescription>
            Configure the check that will run against your endpoint.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FormProvider {...methods}>
            <form onSubmit={onSubmit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="My API"
                  required
                  {...methods.register("name", { required: true })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="type">Monitor type</Label>
                {mode === "create" ? (
                  <Select
                    value={selectedType}
                    onValueChange={(v) => methods.setValue("type", v ?? "")}
                  >
                    <SelectTrigger id="type">
                      <SelectValue placeholder="Select type…" />
                    </SelectTrigger>
                    <SelectContent>
                      {monitorTypes?.map((t) => (
                        <SelectItem key={t.type} value={t.type ?? ""}>
                          {t.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Input value={initial?.type ?? ""} disabled />
                )}
              </div>

              <div className="rounded-md border border-border/60 bg-muted/20 p-4">
                <ConfigFields typeInfo={typeInfo} />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="interval_seconds">Check interval</Label>
                  <Select
                    value={String(methods.watch("interval_seconds"))}
                    onValueChange={(v) =>
                      methods.setValue("interval_seconds", Number(v ?? 300))
                    }
                  >
                    <SelectTrigger id="interval_seconds">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="60">1 minute</SelectItem>
                      <SelectItem value="300">5 minutes</SelectItem>
                      <SelectItem value="900">15 minutes</SelectItem>
                      <SelectItem value="3600">1 hour</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="alert_after_failures">
                    Alert after N failures
                  </Label>
                  <Input
                    id="alert_after_failures"
                    type="number"
                    min={1}
                    max={10}
                    {...methods.register("alert_after_failures", {
                      valueAsNumber: true,
                      min: 1,
                      max: 10,
                    })}
                  />
                </div>
              </div>

              <div className="flex items-center justify-between rounded-md border border-border/60 p-4">
                <div>
                  <div className="font-medium text-sm">Show on public status page</div>
                  <div className="text-xs text-muted-foreground">
                    Visible at /status/your-slug
                  </div>
                </div>
                <Switch
                  aria-label="Show on public status page"
                  checked={methods.watch("is_public")}
                  onCheckedChange={(v) => methods.setValue("is_public", v)}
                />
              </div>

              {channels && channels.length > 0 ? (
                <div className="space-y-2">
                  <Label>Notification channels</Label>
                  <p className="text-xs text-muted-foreground">
                    Leave empty to alert on all channels. Pick specific channels
                    to override.
                  </p>
                  <div className="space-y-2">
                    {channels.map((ch) => (
                      <label
                        key={ch.id}
                        className="flex items-center gap-2 text-sm"
                      >
                        <Checkbox
                          onCheckedChange={(checked) => {
                            const cur = methods.getValues("channel_ids") ?? [];
                            methods.setValue(
                              "channel_ids",
                              checked
                                ? [...cur, ch.id ?? ""]
                                : cur.filter((id) => id !== ch.id),
                            );
                          }}
                        />
                        <span>{ch.name}</span>
                        <span className="text-xs text-muted-foreground">
                          ({ch.type})
                        </span>
                      </label>
                    ))}
                  </div>
                </div>
              ) : null}

              <div className="flex items-center gap-2 pt-2">
                <Button type="submit" disabled={pending}>
                  {pending
                    ? "Saving…"
                    : mode === "create"
                      ? "Create monitor"
                      : "Save changes"}
                </Button>
                <Link
                  href={initial?.id ? `/monitors/${initial.id}` : "/dashboard"}
                  className={buttonVariants({ variant: "ghost" })}
                >
                  Cancel
                </Link>
              </div>
            </form>
          </FormProvider>
        </CardContent>
      </Card>
    </div>
  );
}

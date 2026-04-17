"use client";

import { useRouter } from "next/navigation";
import Link from "next/link";
import { useForm, FormProvider } from "react-hook-form";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
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
import { useCreateChannel, useUpdateChannel } from "@/lib/mutations";
import { useChannelTypes, type Channel } from "@/lib/queries";
import { DynamicConfigFields } from "@/components/features/common/dynamic-config-fields";
import { ArrowLeft } from "lucide-react";

type FormValues = {
  name: string;
  type: string;
  config: Record<string, unknown>;
  is_enabled: boolean;
};

interface Props {
  mode: "create" | "edit";
  initial?: Channel;
}

export function ChannelForm({ mode, initial }: Props) {
  const router = useRouter();
  const { data: types } = useChannelTypes();
  const create = useCreateChannel();
  const update = useUpdateChannel(initial?.id ?? "");

  const methods = useForm<FormValues>({
    defaultValues: {
      name: initial?.name ?? "",
      type: initial?.type ?? "",
      config: (initial?.config as Record<string, unknown>) ?? {},
      is_enabled: initial?.is_enabled ?? true,
    },
  });

  const selectedType = methods.watch("type");
  const typeInfo = types?.find((t) => t.type === selectedType);

  const onSubmit = methods.handleSubmit(async (values) => {
    if (mode === "create") {
      await create.mutateAsync({
        name: values.name,
        type: values.type,
        config: values.config,
      });
      router.push("/channels");
    } else {
      await update.mutateAsync({
        name: values.name,
        config: values.config,
        is_enabled: values.is_enabled,
      });
      router.push("/channels");
    }
  });

  const pending = create.isPending || update.isPending;

  return (
    <div className="container mx-auto px-4 py-8 max-w-xl">
      <Link
        href="/channels"
        className="inline-flex items-center text-sm text-muted-foreground hover:text-foreground mb-4"
      >
        <ArrowLeft className="mr-1 h-4 w-4" /> Back to channels
      </Link>

      <Card>
        <CardHeader>
          <CardTitle>
            {mode === "create" ? "New channel" : `Edit ${initial?.name ?? "channel"}`}
          </CardTitle>
          <CardDescription>
            Destinations receive alerts when a monitor changes state.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FormProvider {...methods}>
            <form onSubmit={onSubmit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="Ops Telegram"
                  required
                  {...methods.register("name", { required: true })}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="type">Channel type</Label>
                {mode === "create" ? (
                  <Select
                    value={selectedType}
                    onValueChange={(v) => methods.setValue("type", v ?? "")}
                  >
                    <SelectTrigger id="type">
                      <SelectValue placeholder="Select type…" />
                    </SelectTrigger>
                    <SelectContent>
                      {types?.map((t) => (
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
                <DynamicConfigFields
                  fields={typeInfo?.schema?.fields}
                  namespace="config"
                />
              </div>

              {mode === "edit" ? (
                <div className="flex items-center justify-between rounded-md border border-border/60 p-4">
                  <div>
                    <div className="font-medium text-sm">Enabled</div>
                    <div className="text-xs text-muted-foreground">
                      When off, alerts are not delivered to this channel.
                    </div>
                  </div>
                  <Switch
                    checked={methods.watch("is_enabled")}
                    onCheckedChange={(v) => methods.setValue("is_enabled", v)}
                  />
                </div>
              ) : null}

              <div className="flex items-center gap-2 pt-2">
                <Button type="submit" disabled={pending}>
                  {pending
                    ? "Saving…"
                    : mode === "create"
                      ? "Create channel"
                      : "Save changes"}
                </Button>
                <Link
                  href="/channels"
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

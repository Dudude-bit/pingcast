"use client";

import { use } from "react";
import { useChannels } from "@/lib/queries";
import { ChannelForm } from "@/components/features/channels/channel-form";
import { Skeleton } from "@/components/ui/skeleton";
import { useLocale } from "@/components/i18n/locale-provider";

export default function EditChannelPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { dict } = useLocale();
  const { data, isLoading, error } = useChannels();

  if (isLoading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-xl space-y-4">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-80 w-full" />
      </div>
    );
  }

  const channel = data?.find((c) => c.id === id);

  if (error || !channel) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-xl">
        <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
          {error?.message ?? dict.channels.not_found}
        </div>
      </div>
    );
  }

  return <ChannelForm mode="edit" initial={channel} />;
}

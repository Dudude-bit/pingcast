"use client";

import Link from "next/link";
import { useChannels } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";
import { buttonVariants } from "@/components/ui/button";
import { ChannelRow } from "./channel-row";
import { Bell } from "lucide-react";

export function ChannelList() {
  const { data, isLoading, error } = useChannels();

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 2 }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-6 text-sm text-red-700 dark:text-red-400">
        Failed to load channels: {error.message}
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border/60 bg-card py-16 px-6 text-center">
        <Bell className="mx-auto h-10 w-10 text-muted-foreground/60" />
        <h3 className="mt-4 text-base font-semibold">No channels yet</h3>
        <p className="mt-1 text-sm text-muted-foreground">
          Add a notification channel to receive alerts when monitors go down.
        </p>
        <Link href="/channels/new" className={`${buttonVariants()} mt-6`}>
          Create channel
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {data.map((ch) => (
        <ChannelRow key={ch.id} ch={ch} />
      ))}
    </div>
  );
}

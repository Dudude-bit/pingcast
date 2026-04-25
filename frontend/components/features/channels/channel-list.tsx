"use client";

import Link from "next/link";
import { useChannels } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";
import { buttonVariants } from "@/components/ui/button";
import { ChannelRow } from "./channel-row";
import { Bell } from "lucide-react";
import { useLocale } from "@/components/i18n/locale-provider";

export function ChannelList() {
  const { dict, locale } = useLocale();
  const t = dict.channels;
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
        {dict.common.load_failed}: {error.message}
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border/60 bg-card py-16 px-6 text-center">
        <Bell className="mx-auto h-10 w-10 text-muted-foreground/60" />
        <h3 className="mt-4 text-base font-semibold">{t.empty}</h3>
        <Link
          href={`/${locale}/channels/new`}
          className={`${buttonVariants()} mt-6`}
        >
          {t.submit_create}
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

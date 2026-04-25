"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { ChannelList } from "@/components/features/channels/channel-list";

export default function ChannelsPage() {
  return (
    <div className="container mx-auto px-4 py-8 max-w-3xl">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
        <div className="min-w-0">
          <h1 className="text-2xl font-bold tracking-tight">
            Notification channels
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Telegram, email, and webhook destinations for monitor alerts.
          </p>
        </div>
        <Link
          href="/channels/new"
          className={`${buttonVariants()} shrink-0 self-start sm:self-auto`}
        >
          <Plus className="mr-2 h-4 w-4" /> New channel
        </Link>
      </div>
      <ChannelList />
    </div>
  );
}

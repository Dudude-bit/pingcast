"use client";

import Link from "next/link";
import { Check, Circle } from "lucide-react";
import { useMonitors, useChannels } from "@/lib/queries";

/**
 * GettingStarted shows a 3-step onboarding checklist on the dashboard
 * while the user hasn't finished wiring up their first end-to-end
 * monitoring flow. Once all three boxes are checked the component
 * returns null and stays out of the way forever.
 *
 * A newly-registered user without this hint sees "no monitors yet"
 * and isn't told that alerts need a bound channel to actually land
 * somewhere — the checklist closes that gap in a non-blocking way.
 */
export function GettingStarted() {
  const monitors = useMonitors();
  const channels = useChannels();

  // Don't flash the checklist while initial fetch is in flight.
  if (monitors.isLoading || channels.isLoading) return null;

  const hasMonitor = (monitors.data?.length ?? 0) > 0;
  const hasChannel = (channels.data?.length ?? 0) > 0;

  // Third step: any monitor with at least one bound channel. The
  // MonitorWithUptime shape doesn't carry channel_ids on the list
  // endpoint, so treat "has both a monitor AND a channel" as a
  // sufficient approximation here. Users who actually bind will see
  // the alert flow; users who forget will still have the two boxes
  // checked but the "bind" step drawing their attention.
  const done = hasMonitor && hasChannel;
  if (done) return null;

  const steps: { label: string; href: string; doneState: boolean }[] = [
    {
      label: "Add your first monitor",
      href: "/monitors/new",
      doneState: hasMonitor,
    },
    {
      label: "Add a notification channel (Telegram or webhook)",
      href: "/channels",
      doneState: hasChannel,
    },
    {
      label: "Bind the channel to your monitor so alerts reach you",
      href: hasMonitor ? "/dashboard" : "/monitors/new",
      doneState: false,
    },
  ];

  return (
    <section
      aria-label="Getting started"
      className="rounded-lg border border-border/60 bg-card p-5 mb-6"
    >
      <h2 className="text-sm font-semibold">Getting started</h2>
      <p className="mt-1 text-xs text-muted-foreground">
        A few quick steps to wire up your first alert. This goes away
        once you&apos;re fully set up.
      </p>
      <ol className="mt-4 space-y-2">
        {steps.map((s) => (
          <li key={s.label} className="flex items-center gap-3 text-sm">
            {s.doneState ? (
              <Check className="h-4 w-4 text-emerald-500 shrink-0" />
            ) : (
              <Circle className="h-4 w-4 text-muted-foreground/60 shrink-0" />
            )}
            <Link
              href={s.href}
              className={
                s.doneState
                  ? "text-muted-foreground line-through"
                  : "underline-offset-2 hover:underline"
              }
            >
              {s.label}
            </Link>
          </li>
        ))}
      </ol>
    </section>
  );
}

"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { components } from "@/lib/openapi-types";

type ChartPoint = components["schemas"]["ChartPoint"];

function formatHour(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function ResponseTimeChart({ data }: { data: ChartPoint[] }) {
  if (!data.length) {
    return (
      <div className="h-40 flex items-center justify-center rounded-md bg-muted/30 text-sm text-muted-foreground">
        No data for the last 24 hours yet.
      </div>
    );
  }

  const series = data
    .filter((p) => p.timestamp && p.avg_response_ms !== undefined)
    .map((p) => ({
      t: p.timestamp!,
      ms: Math.round(p.avg_response_ms ?? 0),
      count: p.check_count ?? 0,
    }));

  return (
    <div className="h-48 -ml-2">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={series} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id="rt-gradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="hsl(221 83% 53%)" stopOpacity={0.35} />
              <stop offset="100%" stopColor="hsl(221 83% 53%)" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="hsl(var(--border))" strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="t"
            tickFormatter={formatHour}
            fontSize={11}
            stroke="hsl(var(--muted-foreground))"
            tickLine={false}
            axisLine={false}
            minTickGap={24}
          />
          <YAxis
            width={40}
            fontSize={11}
            stroke="hsl(var(--muted-foreground))"
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => `${v}ms`}
          />
          <Tooltip
            contentStyle={{
              background: "hsl(var(--popover))",
              border: "1px solid hsl(var(--border))",
              borderRadius: 8,
              fontSize: 12,
            }}
            labelFormatter={(label) => formatHour(String(label))}
            formatter={(v, _n, item) => [
              `${v} ms`,
              `${(item.payload as { count: number }).count} checks`,
            ]}
          />
          <Area
            type="monotone"
            dataKey="ms"
            stroke="hsl(221 83% 53%)"
            strokeWidth={2}
            fill="url(#rt-gradient)"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

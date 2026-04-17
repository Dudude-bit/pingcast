"use client";

import { motion } from "framer-motion";
import { CheckCircle2, Zap } from "lucide-react";

const DEMO_MONITORS = [
  { name: "api.pingcast.io", target: "GET /health", uptime: 99.98, status: "up" as const },
  { name: "Customer dashboard", target: "GET app.example.com", uptime: 99.92, status: "up" as const },
  { name: "Checkout", target: "POST /api/pay", uptime: 99.81, status: "up" as const },
  { name: "Legacy SFTP", target: "TCP sftp.example.com:22", uptime: 97.4, status: "down" as const },
];

function uptimeColor(u: number) {
  if (u >= 99.5) return "text-emerald-500";
  if (u >= 95) return "text-amber-500";
  return "text-red-500";
}

export function LandingDemo() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.5, duration: 0.6, ease: "easeOut" }}
      className="relative mx-auto max-w-2xl"
    >
      <div className="absolute -inset-px rounded-2xl bg-gradient-to-tr from-blue-500/20 via-cyan-500/10 to-teal-500/20 blur-xl" />
      <div className="relative rounded-2xl border border-border/60 bg-card/80 backdrop-blur p-4 shadow-xl">
        <div className="flex items-center justify-between px-2 py-1 mb-3 text-xs text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-red-400" />
            <span className="h-2 w-2 rounded-full bg-amber-400" />
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
          </div>
          <span className="font-mono">pingcast.io/dashboard</span>
          <span />
        </div>
        <ul className="divide-y divide-border/40">
          {DEMO_MONITORS.map((m, i) => (
            <motion.li
              key={m.name}
              initial={{ opacity: 0, x: -8 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.7 + i * 0.08, duration: 0.35 }}
              className="flex items-center gap-3 py-3 px-2"
            >
              <span
                className={`relative inline-block h-2.5 w-2.5 rounded-full shrink-0 ${
                  m.status === "up" ? "bg-emerald-500" : "bg-red-500"
                }`}
              >
                <span
                  className={`absolute inset-0 rounded-full animate-ping ${
                    m.status === "up" ? "bg-emerald-500" : "bg-red-500"
                  } opacity-60`}
                />
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate">{m.name}</div>
                <div className="text-xs text-muted-foreground font-mono truncate">
                  {m.target}
                </div>
              </div>
              <span
                className={`text-sm font-semibold tabular-nums ${uptimeColor(m.uptime)}`}
              >
                {m.uptime.toFixed(2)}%
              </span>
            </motion.li>
          ))}
        </ul>
        <div className="mt-3 flex items-center gap-2 rounded-md bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 px-3 py-2 text-xs">
          <CheckCircle2 className="h-3.5 w-3.5" />
          <span>Instant Telegram alert sent when Legacy SFTP went down</span>
          <Zap className="ml-auto h-3.5 w-3.5" />
        </div>
      </div>
    </motion.div>
  );
}

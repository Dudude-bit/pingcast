import { defineConfig } from "vitest/config";
import path from "node:path";

// Vitest config for unit tests on lib/* utilities. Playwright owns
// browser-level E2E (frontend/tests/) — Vitest is for pure-function /
// browser-API-stub tests like cookie bucketing and locale resolution.
export default defineConfig({
  test: {
    environment: "jsdom",
    include: ["lib/**/*.test.{ts,tsx}"],
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
});

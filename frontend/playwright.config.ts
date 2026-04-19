import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  globalSetup: "./tests/global-setup.ts",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [["html"], ["list"]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3001",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      // Mobile-only specs opt out via grepInvert on the project side.
      grepInvert: /@mobile/,
    },
    {
      name: "mobile-chromium",
      use: { ...devices["iPhone 14"] },
      // Only run the mobile-tagged specs on this project.
      grep: /@mobile/,
    },
  ],
});

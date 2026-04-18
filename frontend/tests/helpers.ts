import { Page, expect } from "@playwright/test";
import { execSync } from "node:child_process";

/**
 * Clears Redis so rate-limit state from previous tests doesn't bleed in.
 * Called in every test's beforeEach — the suite would otherwise blow past
 * the 5-attempts/15min per-IP register limit inside a few tests.
 */
export function flushRedis(): void {
  try {
    execSync("docker compose exec -T redis redis-cli FLUSHDB", {
      stdio: "pipe",
    });
  } catch {
    // ignore — only matters if Redis is down, in which case the API
    // will surface a clearer error to the test anyway.
  }
}

/** Generates a collision-free email for a test run. */
export function uniqueEmail(): string {
  return `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

/** Generates a valid slug (lowercase, alphanumeric + hyphens, 3-30 chars). */
export function uniqueSlug(): string {
  return `e2e${Date.now().toString(36).slice(-6)}${Math.random().toString(36).slice(2, 6)}`;
}

/**
 * Registers a new user via the /register page and asserts arrival at
 * /dashboard. Returns the credentials so callers can log in again.
 */
export async function registerFreshUser(
  page: Page,
  overrides: Partial<{ email: string; slug: string; password: string }> = {},
): Promise<{ email: string; slug: string; password: string }> {
  const email = overrides.email ?? uniqueEmail();
  const slug = overrides.slug ?? uniqueSlug();
  const password = overrides.password ?? "password123";

  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/dashboard/);

  return { email, slug, password };
}

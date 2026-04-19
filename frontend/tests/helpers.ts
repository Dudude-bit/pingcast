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

/** UI helper: navigate to monitor create, fill form, submit, expect redirect. */
export async function uiCreateMonitor(
  page: Page,
  opts: { name: string; url: string },
): Promise<void> {
  await page.goto("/monitors/new");
  await page.getByLabel(/name/i).fill(opts.name);
  await page.getByLabel(/url/i).fill(opts.url);
  await page.getByRole("button", { name: /create/i }).click();
  await expect(page).toHaveURL(/\/monitors/);
}

/**
 * UI helper: create a channel via the /channels page. `config` is a
 * record of field-label → value pairs the helper fills in the dialog.
 */
export async function uiCreateChannel(
  page: Page,
  opts: {
    name: string;
    type: "telegram" | "webhook";
    config: Record<string, string>;
  },
): Promise<void> {
  await page.goto("/channels");
  await page.getByRole("button", { name: /(create|add).*channel/i }).click();
  await page.getByLabel(/name/i).fill(opts.name);
  await page.getByRole("combobox", { name: /type/i }).click();
  await page.getByRole("option", { name: new RegExp(opts.type, "i") }).click();
  for (const [label, value] of Object.entries(opts.config)) {
    await page.getByLabel(new RegExp(label, "i")).fill(value);
  }
  await page.getByRole("button", { name: /^(create|save)$/i }).click();
  await expect(page.getByText(opts.name)).toBeVisible();
}

/**
 * UI helper: on a monitor-detail page, open the "bind channel" control
 * and attach the named channel. Assumes the caller has already navigated
 * to the monitor detail.
 */
export async function uiBindChannelOnMonitor(
  page: Page,
  channelName: string,
): Promise<void> {
  await page.getByRole("button", { name: /(bind|add).*channel/i }).click();
  await page.getByRole("option", { name: channelName }).click();
  await expect(page.getByText(channelName)).toBeVisible();
}

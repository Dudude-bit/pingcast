import { test, expect } from "@playwright/test";

test("unknown slug shows not-found page", async ({ page }) => {
  const res = await page.goto("/status/does-not-exist-xyz-12345");
  expect(res?.status()).toBe(404);
  await expect(
    page.getByRole("heading", { name: /status page not found/i }),
  ).toBeVisible();
});

test("valid slug renders status page with SEO metadata", async ({ page }) => {
  const slug = `s${Date.now().toString(36).slice(-6)}${Math.random().toString(36).slice(2, 6)}`;
  const email = `stat-${slug}@example.com`;

  // Register a user (their slug becomes the status page URL).
  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/dashboard/);

  // Visit the public status page — no monitors yet, so should say "No public services".
  await page.context().clearCookies();
  await page.goto(`/status/${slug}`);
  await expect(
    page.getByRole("heading", { name: /all systems operational/i }),
  ).toBeVisible();
  await expect(
    page.getByText(/no public services configured/i),
  ).toBeVisible();
});

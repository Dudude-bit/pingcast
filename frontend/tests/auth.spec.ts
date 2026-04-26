import { test, expect } from "@playwright/test";
import { flushRedis, uniqueEmail, uniqueSlug, mainEmail, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test("landing page renders with hero + primary CTA", async ({ page }) => {
  await page.goto(`${locPrefix}`);
  // New tagline since the status-page pivot — match the headline-2
  // line which is the more specific marker.
  await expect(
    page.getByRole("heading", { name: /at a third of Atlassian/i }),
  ).toBeVisible();
  // Primary CTA is "Spin up a status page" (was "Start monitoring").
  await expect(
    page.getByRole("link", { name: /spin up a status page/i }),
  ).toBeVisible();
});

test("register → dashboard round-trip", async ({ page }) => {
  const email = uniqueEmail();
  const slug = uniqueSlug();
  const password = "password123";

  await page.goto(`${locPrefix}/register`);
  await mainEmail(page).fill(email);
  await page.getByLabel("Status page slug").fill(slug);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: /create account/i }).click();

  await expect(page).toHaveURL(/\/dashboard/);
});

test("login with wrong password shows generic error", async ({ page }) => {
  await page.goto(`${locPrefix}/login`);
  await mainEmail(page).fill("nonexistent@example.com");
  await page.getByLabel("Password").fill("wrongpassword");
  await page.getByRole("button", { name: /sign in/i }).click();
  await expect(page.getByText(/invalid email or password/i)).toBeVisible();
});

test("register with duplicate email shows generic error (enumeration-safe)", async ({
  page,
}) => {
  const email = uniqueEmail();
  const slug1 = uniqueSlug();
  const slug2 = uniqueSlug();

  await page.goto(`${locPrefix}/register`);
  await mainEmail(page).fill(email);
  await page.getByLabel("Status page slug").fill(slug1);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/dashboard/);

  await page.context().clearCookies();
  await page.goto(`${locPrefix}/register`);
  await mainEmail(page).fill(email);
  await page.getByLabel("Status page slug").fill(slug2);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();

  await expect(page.getByText(/registration failed/i)).toBeVisible();
  await expect(page.getByText(/email already/i)).not.toBeVisible();
});

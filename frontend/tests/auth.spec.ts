import { test, expect } from "@playwright/test";
import { flushRedis, uniqueEmail, uniqueSlug } from "./helpers";

test.beforeEach(flushRedis);

test("landing page renders with CTA", async ({ page }) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: /Know when it breaks/i }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: /start monitoring/i }),
  ).toBeVisible();
});

test("register → dashboard round-trip", async ({ page }) => {
  const email = uniqueEmail();
  const slug = uniqueSlug();
  const password = "password123";

  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: /create account/i }).click();

  await expect(page).toHaveURL(/\/dashboard/);
});

test("login with wrong password shows generic error", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("Email").fill("nonexistent@example.com");
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

  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug1);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/dashboard/);

  await page.context().clearCookies();
  await page.goto("/register");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Status page slug").fill(slug2);
  await page.getByLabel("Password").fill("password123");
  await page.getByRole("button", { name: /create account/i }).click();

  await expect(page.getByText(/registration failed/i)).toBeVisible();
  await expect(page.getByText(/email already/i)).not.toBeVisible();
});

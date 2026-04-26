import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("monitors CRUD", () => {
  test("empty dashboard shows create CTA", async ({ page }) => {
    await registerFreshUser(page);
    await expect(page.getByRole("heading", { name: /no monitors yet/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /create monitor/i })).toBeVisible();
  });

  test("create → detail → edit → delete round-trip", async ({ page }) => {
    await registerFreshUser(page);

    // Create
    await page.getByRole("link", { name: /new monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/new/);

    await page.getByLabel("Name").fill("Example API");
    await page.getByLabel("Monitor type").click();
    await page.getByRole("option", { name: /^http$/i }).click();

    // Dynamic HTTP config fields render; fill URL
    await page.getByLabel("URL").fill("https://example.com/health");

    await page.getByRole("button", { name: /create monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/[0-9a-f-]+$/);
    await expect(page.getByRole("heading", { name: /example api/i })).toBeVisible();

    // Back to dashboard shows the row
    await page.goto(`${locPrefix}/dashboard`);
    await expect(page.getByText(/example api/i)).toBeVisible();

    // Edit
    await page.getByText(/example api/i).click();
    await expect(page).toHaveURL(/\/monitors\/[0-9a-f-]+$/);
    await page.getByRole("link", { name: /edit/i }).click();
    await expect(page).toHaveURL(/\/edit$/);

    await page.getByLabel("Name").fill("Renamed monitor");
    await page.getByRole("button", { name: /save changes/i }).click();
    await expect(page.getByRole("heading", { name: /renamed monitor/i })).toBeVisible();

    // Delete (opens confirm dialog)
    await page.getByRole("button", { name: /delete/i }).first().click();
    await page.getByRole("button", { name: /^delete$/i }).click();
    await expect(page).toHaveURL(/\/dashboard/);
    await expect(page.getByRole("heading", { name: /no monitors yet/i })).toBeVisible();
  });

  test("protected route redirects to /login without session", async ({ page }) => {
    // Fresh browser — no cookies.
    await page.context().clearCookies();
    await page.goto(`${locPrefix}/dashboard`);
    await expect(page).toHaveURL(/\/login/);
  });
});

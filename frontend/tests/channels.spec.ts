import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("channels CRUD", () => {
  test("empty channels page shows create CTA", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto(`${locPrefix}/channels`);
    await expect(
      page.getByRole("heading", { name: /no channels yet/i }),
    ).toBeVisible();
  });

  test("create telegram channel, edit, delete", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto(`${locPrefix}/channels`);

    await page.getByRole("link", { name: /add channel/i }).click();
    await expect(page).toHaveURL(/\/channels\/new/);

    await page.getByLabel("Name").fill("Ops Telegram");
    await page.getByLabel("Type").click();
    await page.getByRole("option", { name: /telegram/i }).click();

    // Telegram channel type exposes a chat_id field (from Go channel registry)
    await page.getByLabel(/chat id/i).fill("123456789");

    await page.getByRole("button", { name: /create channel/i }).click();
    await expect(page).toHaveURL(/\/channels$/);
    await expect(page.getByText(/ops telegram/i)).toBeVisible();

    // Edit: change the name via dropdown → edit
    await page.getByRole("button", { name: /row actions/i }).click();
    await page.getByRole("menuitem", { name: /edit/i }).click();
    await expect(page).toHaveURL(/\/channels\/[0-9a-f-]+\/edit$/);

    await page.getByLabel("Name").fill("Renamed channel");
    await page.getByRole("button", { name: /save changes/i }).click();
    await expect(page).toHaveURL(/\/channels$/);
    await expect(page.getByText(/renamed channel/i)).toBeVisible();

    // Delete
    await page.getByRole("button", { name: /row actions/i }).click();
    await page.getByRole("menuitem", { name: /delete/i }).click();
    await page.getByRole("button", { name: /^delete$/i }).click();
    await expect(
      page.getByRole("heading", { name: /no channels yet/i }),
    ).toBeVisible();
  });
});

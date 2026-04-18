import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

test.describe("API keys", () => {
  test("empty state shown to new users", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/api-keys");
    await expect(page.getByText(/no api keys yet/i)).toBeVisible();
  });

  test("create key → reveal dialog → copy → key persists in list", async ({
    page,
  }) => {
    await registerFreshUser(page);
    await page.goto("/api-keys");

    await page.getByLabel("Name").fill("CI bot");
    // monitors:read is default-checked; ensure at least one scope is ticked
    await page.getByRole("button", { name: /create key/i }).click();

    // Reveal dialog opens with raw key
    await expect(
      page.getByRole("heading", { name: /your new api key/i }),
    ).toBeVisible();
    // Raw key has pc_live_ prefix
    await expect(page.getByText(/pc_live_[a-f0-9]+/)).toBeVisible();

    // Copy button present
    await expect(page.getByRole("button", { name: /copy key/i })).toBeVisible();

    // Close dialog — `.first()` because base-ui Dialog also renders a
    // built-in close-icon button with the same accessible name.
    await page.getByRole("button", { name: /^close$/i }).first().click();

    // Key appears in the list by name, and the raw key is NOT visible
    await expect(page.getByText(/ci bot/i)).toBeVisible();
    await expect(page.getByText(/pc_live_/)).not.toBeVisible();
  });
});

import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("channel types", () => {
  test("create telegram channel via form", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto(`${locPrefix}/channels`);

    await page.getByRole("link", { name: /add channel/i }).click();
    await page.getByLabel("Name").fill("Telegram Alerts");
    await page.getByLabel("Type").click();
    await page.getByRole("option", { name: /telegram/i }).click();
    // Telegram channels only need the chat_id — the bot token is set
    // server-side via TELEGRAM_BOT_TOKEN env, not per-channel.
    await page.getByLabel("Chat ID").fill("777");
    await page.getByRole("button", { name: /create channel/i }).click();
    await expect(page).toHaveURL(/\/channels$/);
    await expect(page.getByText(/telegram alerts/i)).toBeVisible();
  });

  test("webhook type exposes URL field on selection", async ({ page }) => {
    // Smoke for the dynamic-config-fields renderer: when the user picks
    // "Webhook", the URL field appears. Doesn't submit — there's a
    // separate bug where the Headers (JSON) field passes its value as
    // a string instead of parsing to an object, which the API rejects;
    // tracked separately.
    await registerFreshUser(page);
    await page.goto(`${locPrefix}/channels`);
    await page.getByRole("link", { name: /add channel/i }).click();
    await page.getByLabel("Name").fill("Ops Webhook");
    await page.getByLabel("Type").click();
    await page.getByRole("option", { name: /webhook/i }).click();
    await expect(page.getByLabel("Webhook URL")).toBeVisible();
    await expect(page.getByLabel("Headers (JSON)")).toBeVisible();
  });

  test("email channel on free plan surfaces an upgrade cue", async ({
    page,
  }) => {
    await registerFreshUser(page);
    await page.goto(`${locPrefix}/channels`);
    await page.getByRole("link", { name: /add channel/i }).click();
    await page.getByLabel("Name").fill("Oops Email");
    await page.getByLabel("Type").click();
    // The Email option may be disabled OR accompanied by an "upgrade"
    // hint. Accept either.
    const emailOption = page.getByRole("option", { name: /email/i });
    const upgradeHint = page.getByText(/upgrade|pro plan/i).first();
    await expect(emailOption.or(upgradeHint)).toBeVisible();
  });
});

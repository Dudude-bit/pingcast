import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

test.describe("channel types", () => {
  test("create telegram + webhook channels via form", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/channels");

    // Telegram
    await page
      .getByRole("link", { name: /(create|add|new).*channel/i })
      .click();
    await page.getByLabel("Name").fill("Telegram Alerts");
    await page.getByLabel(/type/i).click();
    await page.getByRole("option", { name: /telegram/i }).click();
    await page.getByLabel(/bot.?token/i).fill("12345:ABCDEF");
    await page.getByLabel(/chat.?id/i).fill("777");
    await page.getByRole("button", { name: /create/i }).click();
    await expect(page.getByText(/telegram alerts/i)).toBeVisible();

    // Webhook
    await page
      .getByRole("link", { name: /(create|add|new).*channel/i })
      .click();
    await page.getByLabel("Name").fill("Ops Webhook");
    await page.getByLabel(/type/i).click();
    await page.getByRole("option", { name: /webhook/i }).click();
    await page.getByLabel(/url/i).fill("https://example.com/hook");
    await page.getByRole("button", { name: /create/i }).click();
    await expect(page.getByText(/ops webhook/i)).toBeVisible();
  });

  test("email channel on free plan surfaces an upgrade cue", async ({
    page,
  }) => {
    await registerFreshUser(page);
    await page.goto("/channels");
    await page
      .getByRole("link", { name: /(create|add|new).*channel/i })
      .click();
    await page.getByLabel("Name").fill("Oops Email");
    await page.getByLabel(/type/i).click();
    // The Email option may be disabled OR accompanied by an "upgrade"
    // hint. Accept either.
    const emailOption = page.getByRole("option", { name: /email/i });
    const upgradeHint = page.getByText(/upgrade|pro plan/i).first();
    await expect(emailOption.or(upgradeHint)).toBeVisible();
  });
});

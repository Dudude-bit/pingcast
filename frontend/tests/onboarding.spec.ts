import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

test.describe("onboarding journey", () => {
  test("signup → monitor → channel → bind → visible on detail", async ({
    page,
  }) => {
    await registerFreshUser(page);

    // Monitor
    await page.getByRole("link", { name: /new monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/new/);
    await page.getByLabel("Name").fill("Onboarding Monitor");
    await page.getByLabel("Monitor type").click();
    await page.getByRole("option", { name: /^http$/i }).click();
    await page.getByLabel("URL").fill("https://example.com/health");
    await page.getByRole("button", { name: /create monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/[0-9a-f-]+$/);

    // Channel
    await page.goto("/channels");
    await page.getByRole("link", { name: /(create|add|new).*channel/i }).click();
    await page.getByLabel("Name").fill("Onboarding Webhook");
    await page.getByLabel(/type/i).click();
    await page.getByRole("option", { name: /webhook/i }).click();
    await page.getByLabel(/url/i).fill("https://example.com/hook");
    await page.getByRole("button", { name: /create/i }).click();
    await expect(page.getByText(/onboarding webhook/i)).toBeVisible();

    // Bind on monitor detail — return to dashboard and open the monitor
    await page.goto("/dashboard");
    await page.getByText(/onboarding monitor/i).click();
    await expect(page).toHaveURL(/\/monitors\/[0-9a-f-]+$/);

    await page
      .getByRole("button", { name: /(add|bind).*channel/i })
      .click();
    await page
      .getByRole("option", { name: /onboarding webhook/i })
      .click();

    await expect(page.getByText(/onboarding webhook/i)).toBeVisible();
  });
});

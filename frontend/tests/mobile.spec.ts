import { test, expect } from "@playwright/test";
import { flushRedis, registerViaAPI, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("mobile viewport @mobile", () => {
  test("landing renders and CTA is reachable", async ({ page }) => {
    await page.goto(`${locPrefix}`);
    // Primary landing CTA after the status-page pivot.
    await expect(
      page.getByRole("link", { name: /spin up a status page|sign up|create account/i }).first(),
    ).toBeVisible();
  });

  test("dashboard layout doesn't overflow horizontally", async ({ page }) => {
    await registerViaAPI(page);
    await page.goto(`${locPrefix}/dashboard`);
    await expect(page).toHaveURL(/\/dashboard/);

    // Body scroll width should not exceed viewport width on mobile.
    const { scrollWidth, clientWidth } = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });
});

import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

test.describe("mobile viewport @mobile", () => {
  test("landing renders and CTA is reachable", async ({ page }) => {
    await page.goto("/");
    // Landing CTA button (e.g. "Get started" or "Sign up")
    await expect(
      page.getByRole("link", { name: /get started|sign up|create account/i }).first(),
    ).toBeVisible();
  });

  test("dashboard layout doesn't overflow horizontally", async ({ page }) => {
    await registerFreshUser(page);
    await expect(page).toHaveURL(/\/dashboard/);

    // Body scroll width should not exceed viewport width on mobile.
    const { scrollWidth, clientWidth } = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });
});

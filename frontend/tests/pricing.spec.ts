import { test, expect } from "@playwright/test";

test.describe("pricing page", () => {
  test("renders both plans and a clear CTA", async ({ page }) => {
    await page.goto("/pricing");
    await expect(page.getByRole("heading", { name: /^pricing$/i })).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /hosted.*free/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /self-hosted/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", { name: /start monitoring/i }),
    ).toBeVisible();
  });

  test("has the right title + meta description for search", async ({
    page,
  }) => {
    await page.goto("/pricing");
    await expect(page).toHaveTitle(/Pricing.*PingCast/i);
    const desc = await page
      .locator('meta[name="description"]')
      .getAttribute("content");
    expect(desc).toMatch(/free|self-host/i);
  });
});

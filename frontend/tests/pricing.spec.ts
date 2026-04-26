import { test, expect } from "@playwright/test";
import { locPrefix } from "./helpers";

test.describe("pricing page", () => {
  test("renders all three tiers and a working signup CTA", async ({ page }) => {
    await page.goto(`${locPrefix}/pricing`);
    await expect(page.getByRole("heading", { name: /^pricing$/i })).toBeVisible();
    // Three plan headings — Free, Pro, Self-host (pivot copy).
    await expect(
      page.getByRole("heading", { name: /^free$/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /^pro$/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /^self-host$/i }),
    ).toBeVisible();
    // Free CTA goes to /register.
    await expect(
      page.getByRole("link", { name: /start free/i }),
    ).toBeVisible();
  });

  test("has the right title + meta description for search", async ({
    page,
  }) => {
    await page.goto(`${locPrefix}/pricing`);
    await expect(page).toHaveTitle(/Pricing.*PingCast/i);
    const desc = await page
      .locator('meta[name="description"]')
      .getAttribute("content");
    expect(desc).toMatch(/free|self-host|pro/i);
  });
});

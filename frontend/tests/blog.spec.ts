import { test, expect } from "@playwright/test";
import { locPrefix } from "./helpers";

// /[lang]/blog renders the MDX-backed post index + per-post pages.
// Sprint 4 shipped 3 posts; this smoke verifies the index lists ≥ 1
// and each listed post resolves to a 200 page with rendered content.

test.describe("blog index + posts", () => {
  test("index lists posts and links resolve", async ({ page }) => {
    await page.goto(`${locPrefix}/blog`);

    // h1 = "Blog"
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/blog/i);

    // Subscribe form is mounted.
    await expect(
      page.getByLabel(/email address for newsletter/i).first(),
    ).toBeVisible();

    // At least one post link exists. After i18n every post URL is
    // prefixed with /en or /ru — match the locale-prefixed shape so a
    // future un-prefixed regression would surface as a failure here.
    const links = page.locator('a[href^="/en/blog/"]');
    expect(await links.count()).toBeGreaterThan(0);

    // Click the first post link → ends up on /en/blog/<slug>, h1 visible.
    await links.first().click();
    await expect(page).toHaveURL(/\/en\/blog\/[a-z0-9-]+$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("each listed post has a working back-to-blog link", async ({ page }) => {
    await page.goto(`${locPrefix}/blog`);
    const link = page.locator('a[href^="/en/blog/"]').first();
    await link.click();
    await expect(page).toHaveURL(/\/en\/blog\/[a-z0-9-]+$/);

    // Post-page CTA back to the index reads "All posts".
    await page.getByRole("link", { name: /all posts/i }).first().click();
    await expect(page).toHaveURL(/\/en\/blog$/);
  });
});

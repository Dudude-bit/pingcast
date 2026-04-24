import { test, expect } from "@playwright/test";

// /blog renders the MDX-backed post index + per-post pages. Sprint 4
// shipped 3 posts; this smoke verifies the index lists ≥ 1 and each
// listed post resolves to a 200 page with rendered content.

test.describe("blog index + posts", () => {
  test("index lists posts and links resolve", async ({ page }) => {
    await page.goto("/blog");

    // h1 = "Blog"
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/blog/i);

    // Subscribe form is mounted.
    await expect(
      page.getByLabel(/email address for newsletter/i).first(),
    ).toBeVisible();

    // At least one post link of the form /blog/<slug> exists.
    const links = page.locator('a[href^="/blog/"]');
    const count = await links.count();
    expect(count).toBeGreaterThan(0);

    // Click the first post link → ends up on /blog/<slug>, h1 visible.
    await links.first().click();
    await expect(page).toHaveURL(/\/blog\/[a-z0-9-]+$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  });

  test("each listed post has a working back-to-blog link", async ({ page }) => {
    await page.goto("/blog");
    const link = page.locator('a[href^="/blog/"]').first();
    await link.click();
    await expect(page).toHaveURL(/\/blog\/[a-z0-9-]+$/);

    await page.getByRole("link", { name: /all posts/i }).click();
    await expect(page).toHaveURL(/\/blog$/);
  });
});

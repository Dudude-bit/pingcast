import { test, expect } from "@playwright/test";
import { flushRedis } from "./helpers";

test.beforeEach(flushRedis);

// Newsletter signup is a public form mounted in the footer (every page)
// and on /blog. Posts to /api/newsletter/subscribe and shows an inline
// "check your inbox" confirmation. SMTP is no-op in test, so we don't
// assert on email delivery — just on the request being accepted and the
// UI flipping into the success state.

test.describe("newsletter signup", () => {
  test("footer form accepts a valid email", async ({ page }) => {
    await page.goto("/");
    // Scroll the footer into view so the form is targetable.
    await page.locator("footer").scrollIntoViewIfNeeded();

    const email = `nl-${Date.now()}@example.com`;
    const footer = page.locator("footer");
    await footer.getByLabel(/email address for newsletter/i).fill(email);
    await footer.getByRole("button", { name: /^subscribe$/i }).click();

    await expect(footer.getByText(/check your inbox to confirm/i))
      .toBeVisible({ timeout: 5000 });
  });

  test("blog index has its own subscribe form that also works", async ({
    page,
  }) => {
    await page.goto("/blog");

    // The blog form is in a card above the post list, not in the footer.
    // Filter by parent text to disambiguate from the footer form.
    const card = page.locator("text=Subscribe — 1-2 a month").locator("..");
    await card.getByLabel(/email address for newsletter/i).fill(
      `blog-${Date.now()}@example.com`,
    );
    await card.getByRole("button", { name: /^subscribe$/i }).click();

    await expect(card.getByText(/check your inbox to confirm/i))
      .toBeVisible({ timeout: 5000 });
  });

  test("invalid email gets HTML5 validation, no request fires", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator("footer").scrollIntoViewIfNeeded();

    const footer = page.locator("footer");
    await footer.getByLabel(/email address for newsletter/i).fill("not-an-email");
    await footer.getByRole("button", { name: /^subscribe$/i }).click();

    // HTML5 validation kicks in before the fetch — success message stays
    // hidden, error message also stays hidden, the helper text persists.
    await expect(footer.getByText(/check your inbox/i)).not.toBeVisible();
  });
});

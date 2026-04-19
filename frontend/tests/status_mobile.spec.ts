import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, uniqueSlug } from "./helpers";

test.beforeEach(flushRedis);

test.describe("public status page — mobile @mobile", () => {
  test("renders cleanly on iPhone viewport", async ({ page }) => {
    const slug = uniqueSlug();
    await registerFreshUser(page, { slug });

    await page.goto(`/status/${slug}`);

    // Page headline contains the slug or "status"
    await expect(
      page.getByRole("heading", { name: new RegExp(`(${slug}|status)`, "i") }),
    ).toBeVisible();

    // No horizontal overflow on mobile.
    const { scrollWidth, clientWidth } = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });
});

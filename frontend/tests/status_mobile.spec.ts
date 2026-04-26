import { test, expect } from "@playwright/test";
import { flushRedis, registerViaAPI, uniqueSlug } from "./helpers";

test.beforeEach(flushRedis);

test.describe("public status page — mobile @mobile", () => {
  test("renders cleanly on iPhone viewport", async ({ page }) => {
    const slug = uniqueSlug();
    await registerViaAPI(page, { slug });

    await page.goto(`/status/${slug}`);

    // Empty status page renders the all-systems-operational headline.
    await expect(
      page.getByRole("heading", { name: /all systems operational/i }),
    ).toBeVisible();

    // No horizontal overflow on mobile.
    const { scrollWidth, clientWidth } = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });
});

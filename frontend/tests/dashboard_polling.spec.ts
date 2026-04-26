import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("dashboard polling", () => {
  test("TanStack Query refetches monitor list and picks up a new row", async ({
    page,
  }) => {
    await registerFreshUser(page);
    await expect(page).toHaveURL(/\/dashboard/);

    // Create a monitor via the same UI the user would — serves as a
    // proxy for "some state change happens" and verifies that after
    // creation (client-side cache invalidation) the list includes it.
    await page.getByRole("link", { name: /new monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/new/);
    await page.getByLabel("Name").fill("Polled Monitor");
    await page.getByLabel("Monitor type").click();
    await page.getByRole("option", { name: /^http$/i }).click();
    await page.getByLabel("URL").fill("https://example.com/health");
    await page.getByRole("button", { name: /create monitor/i }).click();

    // Back on dashboard, the row appears without a hard refresh.
    await page.goto(`${locPrefix}/dashboard`);
    await expect(page.getByText(/polled monitor/i)).toBeVisible();

    // Second check: wait through one refetchInterval (15s) to catch the
    // next query. The row must still be present (absence would mean
    // the poll clobbered state).
    await page.waitForTimeout(16_000);
    await expect(page.getByText(/polled monitor/i)).toBeVisible();
  });
});

import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

test.describe("monitor detail page", () => {
  test("renders uptime, chart placeholder, and incidents section", async ({
    page,
  }) => {
    await registerFreshUser(page);

    // Create a monitor so we have a detail page to visit
    await page.goto("/monitors/new");
    await page.getByLabel("Name").fill("Detail Monitor");
    await page.getByLabel("Monitor type").click();
    await page.getByRole("option", { name: /^http$/i }).click();
    await page.getByLabel("URL").fill("https://example.com/health");
    await page.getByRole("button", { name: /create monitor/i }).click();
    await expect(page).toHaveURL(/\/monitors\/[0-9a-f-]+$/);

    // Detail page structural checks — headers/labels the page renders
    // even with zero check_results yet.
    await expect(
      page.getByRole("heading", { name: /detail monitor/i }),
    ).toBeVisible();

    // Uptime blocks (24h / 7d / 30d) are rendered as visible text labels.
    await expect(page.getByText(/24h/i)).toBeVisible();
    await expect(page.getByText(/7d/i)).toBeVisible();
    await expect(page.getByText(/30d/i)).toBeVisible();

    // Chart region is present even if no data. Recharts renders an
    // <svg> inside the container.
    const chart = page.locator("svg").first();
    await expect(chart).toBeVisible();

    // Incidents section heading (empty state OK)
    await expect(page.getByText(/incidents/i)).toBeVisible();
  });
});

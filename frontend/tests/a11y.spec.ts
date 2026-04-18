import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

/**
 * Runs axe-core WCAG 2.1 AA scan on the key pages. Fails on any violation
 * with a serious or critical impact — moderate/minor are noisy on a fresh
 * shadcn install and would drown real regressions in noise.
 *
 * When axe finds an issue, inspect the JSON in the failure output — each
 * violation includes the rule id (e.g., `color-contrast`), the affected
 * nodes' selectors, and a link to deque-docs explaining the fix.
 */
async function scan(page: import("@playwright/test").Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  const blocking = results.violations.filter(
    (v) => v.impact === "serious" || v.impact === "critical",
  );
  expect(blocking, JSON.stringify(blocking, null, 2)).toEqual([]);
}

test.describe("accessibility", () => {
  test("landing page has no serious/critical a11y violations", async ({
    page,
  }) => {
    await page.goto("/");
    await scan(page);
  });

  test("login page", async ({ page }) => {
    await page.goto("/login");
    await scan(page);
  });

  test("register page", async ({ page }) => {
    await page.goto("/register");
    await scan(page);
  });

  test("empty dashboard", async ({ page }) => {
    await registerFreshUser(page);
    await scan(page);
  });

  test("monitor create form", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/monitors/new");
    await scan(page);
  });

  test("api keys empty state", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/api-keys");
    await scan(page);
  });

  test("public status page", async ({ page }) => {
    const { slug } = await registerFreshUser(page);
    await page.context().clearCookies();
    await page.goto(`/status/${slug}`);
    await scan(page);
  });
});

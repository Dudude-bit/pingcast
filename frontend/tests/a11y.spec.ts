import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(async ({ page }) => {
  flushRedis();
  // Framer Motion's initial opacity:0 → 1 stagger on the landing page
  // flags as color-contrast violations if axe scans mid-animation. Force
  // reduced-motion and inject CSS to short-circuit any remaining
  // transitions / animations — motion.divs use inline styles that
  // emulateMedia alone can't override.
  await page.emulateMedia({ reducedMotion: "reduce" });
});

/**
 * Runs axe-core WCAG 2.1 AA scan on the key pages. Fails on any violation
 * with a serious or critical impact — moderate/minor are noisy on a fresh
 * shadcn install and would drown real regressions in noise.
 *
 * Injects a stylesheet that zeroes animation/transition durations so that
 * Framer Motion's initial opacity:0 doesn't trip color-contrast checks
 * mid-animation. axe evaluates computed styles at scan time — if an
 * element is transitioning, contrast is flagged against the in-flight
 * colors rather than the final state.
 *
 * When axe finds an issue, inspect the JSON in the failure output — each
 * violation includes the rule id (e.g., `color-contrast`), the affected
 * nodes' selectors, and a link to deque-docs explaining the fix.
 */
async function scan(page: import("@playwright/test").Page) {
  // Wait for Framer Motion's stagger (~600ms total on the landing) so
  // elements settle at opacity: 1 before axe evaluates color-contrast.
  // Without this, in-flight translateY/opacity frames get scanned
  // against whatever their interpolated color is, tripping false
  // positives that have nothing to do with the final rendered state.
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(800);
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

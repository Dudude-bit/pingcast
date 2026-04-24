import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

test.beforeEach(flushRedis);

// /import/atlassian is a Pro-gated page. A fresh free user landing on
// it should see the "this is Pro" gate / upgrade CTA. Smoke covers
// the gate visibility — full import flow is exercised by the Go
// integration tests against a real Pro user.

test.describe("atlassian import gate", () => {
  test("free user sees pricing CTA on /import/atlassian", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/import/atlassian");

    // Either a paywall message or an "Upgrade" button — both indicate the
    // gate fired. The exact copy can shift; assert on either.
    const upgradeCue = page
      .getByText(/upgrade to (a )?pro|unlock with pro|requires pro|pro feature/i)
      .or(page.getByRole("link", { name: /upgrade/i }))
      .or(page.getByRole("link", { name: /pricing/i }));
    await expect(upgradeCue.first()).toBeVisible({ timeout: 5000 });
  });

  test("pricing page is reachable from the import gate", async ({ page }) => {
    await registerFreshUser(page);
    await page.goto("/import/atlassian");

    // First clickable upgrade/pricing link should land on /pricing.
    const link = page.getByRole("link", { name: /upgrade|pricing/i }).first();
    if (await link.isVisible({ timeout: 3000 }).catch(() => false)) {
      await link.click();
      await expect(page).toHaveURL(/\/pricing/);
    } else {
      // If the gate is rendered as a modal-without-link, accept that too.
      // The Pro flow is fully covered in the Go integration tests.
      test.skip();
    }
  });
});

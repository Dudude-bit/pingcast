import { test, expect } from "@playwright/test";
import { flushRedis } from "./helpers";

test.beforeEach(flushRedis);

test.describe("client-side form validation", () => {
  test("register form shows inline errors on empty submit", async ({
    page,
  }) => {
    await page.goto("/register");
    await page.getByRole("button", { name: /create account/i }).click();

    // Native HTML5 validation OR react-hook-form inline errors prevent
    // the submit from reaching the server. URL stays on /register.
    await expect(page).toHaveURL(/\/register/);

    // At least one visible field-level error or a native :invalid state.
    // We check for common inline-error text; fall back to :invalid.
    const inlineError = page.getByText(/required|cannot be empty/i).first();
    const hasInline = await inlineError.isVisible().catch(() => false);
    if (!hasInline) {
      const invalid = await page
        .locator(":invalid")
        .first()
        .evaluate((el) => (el as HTMLInputElement).validity.valueMissing)
        .catch(() => false);
      expect(invalid).toBe(true);
    }
  });
});

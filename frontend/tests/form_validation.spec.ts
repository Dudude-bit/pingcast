import { test, expect } from "@playwright/test";
import { flushRedis, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("client-side form validation", () => {
  test("register form shows inline errors on empty submit", async ({
    page,
  }) => {
    await page.goto(`${locPrefix}/register`);
    await page.getByRole("button", { name: /create account/i }).click();

    // Native HTML5 validation OR react-hook-form inline errors prevent
    // the submit from reaching the server. URL stays on /register.
    await expect(page).toHaveURL(/\/register/);

    // The native :required validity is on the form's email/password
    // inputs. Scope to the registration form (the first <form> on the
    // page — the newsletter form lives in the footer further down).
    const firstForm = page.locator("form").first();
    const emailInvalid = await firstForm
      .locator("input[type='email']")
      .first()
      .evaluate((el) => (el as HTMLInputElement).validity.valueMissing);
    expect(emailInvalid).toBe(true);
  });
});

import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser, locPrefix } from "./helpers";

test.beforeEach(flushRedis);

test.describe("session expiry", () => {
  test("clearing session cookie mid-session redirects next click to /login", async ({
    page,
  }) => {
    await registerFreshUser(page);
    await expect(page).toHaveURL(/\/dashboard/);

    // Simulate session expiry by removing the cookie the server set.
    await page.context().clearCookies({ name: "session_id" });

    // Any attempt to navigate to a guarded route must redirect to login.
    // Dashboard is a Pro-gated server-rendered route; with the new
    // auth gate, hitting it without a cookie now 307→/<lang>/login.
    await page.goto(`${locPrefix}/dashboard`);
    await expect(page).toHaveURL(/\/login/);
  });
});

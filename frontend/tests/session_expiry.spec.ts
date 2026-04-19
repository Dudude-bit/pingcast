import { test, expect } from "@playwright/test";
import { flushRedis, registerFreshUser } from "./helpers";

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
    await page.goto("/monitors");
    await expect(page).toHaveURL(/\/login/);
  });
});

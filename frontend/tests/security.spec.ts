import { test, expect } from "@playwright/test";

test.describe("security headers", () => {
  test("landing emits baseline security headers", async ({ request }) => {
    const res = await request.get("/");
    const h = res.headers();
    expect(h["strict-transport-security"]).toMatch(/max-age=\d+/);
    expect(h["x-content-type-options"]).toBe("nosniff");
    expect(h["referrer-policy"]).toBe("strict-origin-when-cross-origin");
    expect(h["permissions-policy"]).toContain("camera=()");
    expect(h["x-frame-options"]).toBe("SAMEORIGIN");
  });

  test("robots.txt picks up the same headers", async ({ request }) => {
    const res = await request.get("/robots.txt");
    expect(res.headers()["x-content-type-options"]).toBe("nosniff");
  });
});

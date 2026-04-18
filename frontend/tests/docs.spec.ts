import { test, expect } from "@playwright/test";

test.describe("API docs", () => {
  test("/openapi.yaml is served with the spec body", async ({ request }) => {
    const res = await request.get("/openapi.yaml");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("openapi:");
    expect(body).toContain("PingCast API");
  });

  test("/docs/api renders the Scalar reference", async ({ page }) => {
    await page.goto("/docs/api");
    // Scalar renders operation sections that include tag/operation text
    // from the spec — look for a known endpoint label.
    await expect(
      page.getByText(/monitors/i).first(),
    ).toBeVisible({ timeout: 15000 });
  });
});

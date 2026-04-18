import { test, expect } from "@playwright/test";

test.describe("SEO surface", () => {
  test("robots.txt disallows authenticated routes", async ({ request }) => {
    const res = await request.get("/robots.txt");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("User-Agent: *");
    expect(body).toContain("Disallow: /dashboard");
    expect(body).toContain("Sitemap:");
  });

  test("sitemap.xml includes public pages", async ({ request }) => {
    const res = await request.get("/sitemap.xml");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("<loc>");
    expect(body).toContain("/register");
    expect(body).toContain("/login");
  });

  test("landing embeds SoftwareApplication JSON-LD", async ({ page }) => {
    await page.goto("/");
    const json = await page
      .locator('script[type="application/ld+json"]')
      .textContent();
    expect(json).toBeTruthy();
    const parsed = JSON.parse(json ?? "{}");
    expect(parsed["@type"]).toBe("SoftwareApplication");
    expect(parsed.name).toBe("PingCast");
  });

  test("register page has a tailored title + description", async ({ page }) => {
    await page.goto("/register");
    await expect(page).toHaveTitle(/Create account.*PingCast/i);
    const desc = await page
      .locator('meta[name="description"]')
      .getAttribute("content");
    expect(desc).toMatch(/monitoring/i);
  });
});

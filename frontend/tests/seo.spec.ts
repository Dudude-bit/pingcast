import { test, expect } from "@playwright/test";
import { locPrefix } from "./helpers";

test.describe("SEO surface", () => {
  test("robots.txt disallows authenticated routes", async ({ request }) => {
    const res = await request.get("/robots.txt");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("User-Agent: *");
    expect(body).toContain("Disallow: /dashboard");
    expect(body).toContain("Sitemap:");
  });

  test("sitemap.xml includes locale-prefixed public pages", async ({
    request,
  }) => {
    const res = await request.get("/sitemap.xml");
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain("<loc>");
    // After i18n, every page is enumerated under both /en and /ru.
    expect(body).toContain("/en/register");
    expect(body).toContain("/en/login");
    expect(body).toContain("/en/pricing");
    expect(body).toContain("/en/docs/api");
    expect(body).toContain("/ru/pricing");
  });

  test("landing embeds SoftwareApplication JSON-LD", async ({ page }) => {
    await page.goto(`${locPrefix}`);
    // Multiple JSON-LD scripts can co-exist (FAQ, Breadcrumb, Org…).
    // Find the SoftwareApplication block specifically.
    const scripts = await page
      .locator('script[type="application/ld+json"]')
      .allTextContents();
    expect(scripts.length).toBeGreaterThan(0);
    const apps = scripts
      .map((raw) => {
        try {
          return JSON.parse(raw) as Record<string, unknown>;
        } catch {
          return null;
        }
      })
      .filter(
        (obj): obj is Record<string, unknown> =>
          obj !== null && obj["@type"] === "SoftwareApplication",
      );
    expect(apps.length).toBeGreaterThan(0);
    expect(apps[0]!.name).toBe("PingCast");
  });

  test("register page has a tailored title + description", async ({ page }) => {
    await page.goto(`${locPrefix}/register`);
    await expect(page).toHaveTitle(/Create account.*PingCast/i);
    const desc = await page
      .locator('meta[name="description"]')
      .getAttribute("content");
    expect(desc).toMatch(/status page|monitor|sign up/i);
  });

  test("landing exposes OG + Twitter image tags", async ({ page }) => {
    await page.goto(`${locPrefix}`);
    const ogImage = await page
      .locator('meta[property="og:image"]')
      .first()
      .getAttribute("content");
    const twImage = await page
      .locator('meta[name="twitter:image"]')
      .first()
      .getAttribute("content");
    expect(ogImage).toMatch(/opengraph-image/);
    expect(twImage).toMatch(/twitter-image/);
  });

  test("opengraph-image endpoint renders a PNG", async ({ request }) => {
    const res = await request.get("/opengraph-image");
    expect(res.status()).toBe(200);
    expect(res.headers()["content-type"]).toBe("image/png");
    // PNG signature: 89 50 4E 47 0D 0A 1A 0A
    const buf = await res.body();
    expect(buf.subarray(0, 4).toString("hex")).toBe("89504e47");
  });
});

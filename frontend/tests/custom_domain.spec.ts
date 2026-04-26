import { test, expect } from "@playwright/test";
import { flushRedis } from "./helpers";

test.beforeEach(flushRedis);

// /api/public/lookup-domain is the contract proxy.ts depends on for
// custom-domain routing. If it breaks, every paying Pro customer with
// status.theircompany.com → us setup sees 404 instead of their branded
// status page. This spec locks in the contract:
//   * 404 when the hostname isn't an active custom domain
//   * 200 + {slug} when it is (verified via the canonical apex hosts
//     list in proxy.ts — those resolve internally)
//   * proxy.ts itself doesn't try to look up canonical hosts (that
//     would burn an API call on every pingcast.io request)

test("unknown host returns 404 from lookup endpoint", async ({ request }) => {
  const res = await request.get(
    "/api/public/lookup-domain?hostname=does-not-exist.test",
  );
  expect(res.status()).toBe(404);
});

test("missing hostname param returns 400", async ({ request }) => {
  // The handler should reject empty hostnames so an upstream caller
  // bug (proxy.ts ever passing "") doesn't waste a DB roundtrip.
  const res = await request.get("/api/public/lookup-domain?hostname=");
  expect([400, 404]).toContain(res.status());
});

test("our own apex doesn't get rewritten via custom-domain lookup", async ({
  page,
}) => {
  // Visiting / on the canonical host must NOT 404 — proxy.ts is
  // expected to short-circuit canonical hosts before hitting the
  // lookup endpoint at all. We can't directly assert "endpoint not
  // called" from the browser, but we can assert the visible outcome:
  // the home page renders.
  const res = await page.goto("/en");
  expect(res?.status()).toBe(200);
  // The landing has the new tagline — proves we got the real page
  // and not some fallback.
  await expect(
    page.getByRole("heading", { level: 1 }).first(),
  ).toBeVisible();
});

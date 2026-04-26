import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

// Stable env reset between tests so one variant config doesn't leak
// into the next.
const ORIGINAL_ENV = { ...process.env };

beforeEach(() => {
  process.env = { ...ORIGINAL_ENV };
  // Clear cookies — the bucket cookie is the source of stability we
  // want to prove. Carrying it across tests would make persistence
  // tests pass for the wrong reason.
  document.cookie.split("; ").forEach((c) => {
    const eq = c.indexOf("=");
    const name = eq > -1 ? c.slice(0, eq) : c;
    if (name) {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
    }
  });
  vi.resetModules();
});

afterEach(() => {
  process.env = { ...ORIGINAL_ENV };
});

describe("getBucket", () => {
  test("persists assignment across calls within a session", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    const { getBucket } = await import("./abtest");

    const first = getBucket();
    const second = getBucket();
    const third = getBucket();

    expect(second).toBe(first);
    expect(third).toBe(first);
  });

  test("writes the bucket to a cookie so it survives reload", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    const { getBucket } = await import("./abtest");

    const assigned = getBucket();

    expect(document.cookie).toContain("pc_ab_pricing=");
    expect(document.cookie).toContain(`pc_ab_pricing=${assigned}`);
  });

  test("reads an existing cookie instead of re-rolling", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    document.cookie = "pc_ab_pricing=B; path=/";
    const { getBucket } = await import("./abtest");

    expect(getBucket()).toBe("B");
  });

  test("never returns a variant the operator hasn't enabled", async () => {
    // Soft-launch shape: only A and B live, C is dark. A previously-
    // assigned C cookie must not stick once the operator disables it.
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B";
    document.cookie = "pc_ab_pricing=C; path=/";
    const { getBucket } = await import("./abtest");

    const got = getBucket();
    expect(["A", "B"]).toContain(got);
    expect(got).not.toBe("C");
  });

  test("malformed env falls back to A so the page never blanks", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "X,Y,Z";
    const { getBucket } = await import("./abtest");

    expect(getBucket()).toBe("A");
  });

  test("missing env defaults to A", async () => {
    delete process.env.NEXT_PUBLIC_AB_VARIANTS;
    const { getBucket } = await import("./abtest");

    expect(getBucket()).toBe("A");
  });

  test("distributes across all enabled variants over many calls", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    const { getBucket } = await import("./abtest");

    // Seed 300 fresh assignments by clearing the cookie each time. We
    // don't need uniform distribution — just proof that more than one
    // bucket gets returned (catches the "always picks A" regression).
    const seen = new Set<string>();
    for (let i = 0; i < 300; i++) {
      document.cookie =
        "pc_ab_pricing=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/";
      seen.add(getBucket());
    }
    expect(seen.size).toBeGreaterThanOrEqual(2);
  });
});

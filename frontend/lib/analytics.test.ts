import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

const ORIGINAL_ENV = { ...process.env };

type CapturedCall = {
  event: string;
  options?: { props?: Record<string, string | number | boolean> };
};

let captured: CapturedCall[];

beforeEach(() => {
  process.env = { ...ORIGINAL_ENV };
  captured = [];
  // Stub Plausible — every track() call writes here for inspection.
  (window as unknown as { plausible: unknown }).plausible = (
    event: string,
    options?: { props?: Record<string, string | number | boolean> },
  ) => {
    captured.push({ event, options });
  };
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
  delete (window as unknown as { plausible?: unknown }).plausible;
});

describe("track", () => {
  test("auto-tags every event with the visitor's A/B bucket", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    const { track } = await import("./analytics");

    track("pro_checkout_clicked", { variant: "founder" });

    expect(captured).toHaveLength(1);
    const props = captured[0]!.options?.props;
    expect(props).toBeDefined();
    expect(props!.bucket).toMatch(/^[ABC]$/);
    expect(props!.variant).toBe("founder");
  });

  test("sends the same bucket on subsequent events in the same session", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B,C";
    const { track } = await import("./analytics");

    track("pricing_page_view");
    track("pro_checkout_clicked");
    track("register_completed");

    const buckets = captured.map((c) => c.options?.props?.bucket);
    expect(buckets[0]).toBeDefined();
    expect(buckets[1]).toBe(buckets[0]);
    expect(buckets[2]).toBe(buckets[0]);
  });

  test("does nothing when Plausible is not loaded (no ad-blocker crashes)", async () => {
    delete (window as unknown as { plausible?: unknown }).plausible;
    const { track } = await import("./analytics");

    expect(() => track("pricing_page_view")).not.toThrow();
    expect(captured).toHaveLength(0);
  });

  test("caller props are not lost when bucket auto-merges", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A";
    const { track } = await import("./analytics");

    track("custom_event", { lang: "ru", source: "blog_index" });

    const props = captured[0]!.options?.props;
    expect(props).toEqual(expect.objectContaining({
      bucket: "A",
      lang: "ru",
      source: "blog_index",
    }));
  });

  test("caller props can override the auto-bucket if needed (explicit > implicit)", async () => {
    process.env.NEXT_PUBLIC_AB_VARIANTS = "A,B";
    const { track } = await import("./analytics");

    // Edge case: a one-off event that needs to claim a different
    // bucket (e.g. when you hand-instrument a server-side event).
    // Caller-provided `bucket` must win over the auto-merge.
    track("custom_event", { bucket: "Z" });

    expect(captured[0]!.options?.props?.bucket).toBe("Z");
  });
});

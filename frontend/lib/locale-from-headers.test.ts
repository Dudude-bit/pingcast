import { describe, expect, test } from "vitest";
import { pickLocale } from "./locale-from-headers";

describe("pickLocale", () => {
  // The most important rule: a visitor who navigated to /ru/foo with an
  // English browser must see Russian. The not-found-locale regression
  // we fixed in commit 11257d4 lives or dies on this assertion.
  test("URL prefix beats Accept-Language", () => {
    expect(pickLocale("/ru/blog", "en-US,en;q=0.9")).toBe("ru");
    expect(pickLocale("/en/blog", "ru-RU,ru;q=0.9")).toBe("en");
  });

  test("URL prefix is exact-match or path-prefix only — random substrings don't trigger", () => {
    // "/ruby/foo" must NOT pick RU just because the path starts with /ru.
    expect(pickLocale("/ruby/foo", "")).toBe("en");
    // "/enroll" must NOT pick EN via the prefix path.
    expect(pickLocale("/enroll", "ru-RU,ru;q=0.9")).toBe("ru");
  });

  test("falls back to Accept-Language when path is locale-less", () => {
    expect(pickLocale("/status/acme", "ru-RU,ru;q=0.9,en;q=0.8")).toBe("ru");
    expect(pickLocale("/status/acme", "en-US,en;q=0.9")).toBe("en");
  });

  test("handles weighted Accept-Language with mixed regions", () => {
    expect(pickLocale("", "fr-FR,fr;q=0.9,ru;q=0.5,en;q=0.4")).toBe("ru");
  });

  test("falls back to DEFAULT_LOCALE when both inputs are empty", () => {
    expect(pickLocale("", "")).toBe("en");
  });

  test("falls back to DEFAULT_LOCALE when Accept-Language has no supported locales", () => {
    expect(pickLocale("/status/acme", "fr-FR,fr;q=0.9")).toBe("en");
  });

  test("malformed Accept-Language doesn't crash", () => {
    expect(() => pickLocale("", ";;,,;q=garbage")).not.toThrow();
    expect(pickLocale("", ";;,,;q=garbage")).toBe("en");
  });

  test("bare /<lang> path (no trailing slash) still resolves", () => {
    expect(pickLocale("/ru", "en-US")).toBe("ru");
    expect(pickLocale("/en", "ru-RU")).toBe("en");
  });
});

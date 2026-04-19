import { test, expect } from "@playwright/test";

test.describe("theme toggle", () => {
  test("dark mode persists across reload via localStorage", async ({
    page,
  }) => {
    await page.goto("/");

    // Open the theme toggle and switch to dark mode. next-themes stores
    // the choice in localStorage under the key "theme".
    const toggle = page.getByRole("button", { name: /theme|dark|light/i }).first();
    await toggle.click();

    // Menu item that switches the app to dark
    await page.getByRole("menuitem", { name: /^dark$/i }).click();

    // html tag picks up .dark class
    await expect(page.locator("html")).toHaveClass(/dark/);

    // Reload — the class must still be present
    await page.reload();
    await expect(page.locator("html")).toHaveClass(/dark/);

    const stored = await page.evaluate(() => localStorage.getItem("theme"));
    expect(stored).toBe("dark");
  });
});

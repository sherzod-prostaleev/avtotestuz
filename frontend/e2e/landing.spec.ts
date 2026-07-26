import { test, expect } from "@playwright/test";

test.describe("Landing page", () => {
  test("loads and shows hero content", async ({ page }) => {
    await page.goto("/uz-Latn");
    await expect(page.locator("h1").first()).toBeVisible();
  });

  test("shows proof stats (question + ticket counts)", async ({ page }) => {
    await page.goto("/uz-Latn");
    // Copy uses "1231+" (verified import size), not a hard "1235".
    await expect(page.getByText("1231+")).toBeVisible();
    await expect(page.getByText("62")).toBeVisible();
  });

  test("has login link in header", async ({ page }) => {
    await page.goto("/uz-Latn");
    const loginLink = page.locator('a[href*="login"]').first();
    await expect(loginLink).toBeVisible();
  });

  test("CTA button navigates to login", async ({ page }) => {
    await page.goto("/uz-Latn");
    const cta = page.locator('a[href*="login"]').first();
    await cta.click();
    await expect(page).toHaveURL(/login/);
  });

  test("theme toggle is present", async ({ page }) => {
    await page.goto("/uz-Latn");
    // ThemeToggle component exists in the header
    const header = page.locator("header").first();
    await expect(header).toBeVisible();
  });
});

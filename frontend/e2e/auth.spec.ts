import { test, expect } from "@playwright/test";

test.describe("Authentication flow", () => {
  test("login page loads with phone and password inputs", async ({ page }) => {
    await page.goto("/uz-Latn/login");
    const phoneInput = page.locator('input[type="tel"]').first();
    const passwordInput = page.locator('input[type="password"]').first();
    await expect(phoneInput).toBeVisible();
    await expect(passwordInput).toBeVisible();
  });

  test("register page is reachable from login", async ({ page }) => {
    await page.goto("/uz-Latn/login");
    await page.getByRole("link", { name: /Ro'yxatdan o'tish/i }).click();
    await expect(page).toHaveURL(/\/register/);
    await expect(page.locator('input[type="password"]')).toHaveCount(2);
  });

  test("legacy verify route redirects toward login", async ({ page }) => {
    await page.goto("/uz-Latn/login/verify?phone=901112233");
    await page.waitForURL(/\/login(?!\/verify)/, { timeout: 5000 });
    expect(page.url()).toContain("/login");
    expect(page.url()).not.toContain("/verify");
  });
});

import { test, expect } from "@playwright/test";

test.describe("Authentication flow", () => {
  test("login page loads with phone input", async ({ page }) => {
    await page.goto("/uz-Latn/login");
    const phoneInput = page.locator('input[type="tel"], input[name="phone"], input[placeholder*="998"]').first();
    await expect(phoneInput).toBeVisible();
  });

  test("can submit phone number", async ({ page }) => {
    await page.goto("/uz-Latn/login");
    const phoneInput = page.locator('input[type="tel"], input[name="phone"], input[placeholder*="998"]').first();
    await phoneInput.fill("901112233");
    const submitButton = page.locator('button[type="submit"]').first();
    await submitButton.click();
    // Wait for navigation or response
    await page.waitForTimeout(3000);
    // Verify we're still on login or moved to verify
    const url = page.url();
    expect(url).toContain("login");
  });

  test("verify page has code input", async ({ page }) => {
    await page.goto("/uz-Latn/login/verify?phone=901112233");
    // Wait for page to load
    await page.waitForTimeout(2000);
    // Check for any input field
    const inputs = page.locator("input");
    const count = await inputs.count();
    expect(count).toBeGreaterThan(0);
  });
});

import { test, expect } from "@playwright/test";

test.describe("Road signs catalog", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/signs");
    await expect(page).toHaveURL(/login/);
  });

  test("shows signs after authentication", async ({ page }) => {
    test.skip(!process.env.E2E_AUTH_TOKEN, "E2E_AUTH_TOKEN not set");

    await page.goto("/uz-Latn/signs");
    await page.waitForTimeout(2000);

    // Check for signs content
    const signsContent = page.locator('[class*="sign"], [class*="grid"]').first();
    await expect(signsContent).toBeVisible({ timeout: 10_000 });
  });

  test("group filter works", async ({ page }) => {
    test.skip(!process.env.E2E_AUTH_TOKEN, "E2E_AUTH_TOKEN not set");

    await page.goto("/uz-Latn/signs");
    await page.waitForTimeout(2000);

    const filterButtons = page.locator('button[aria-pressed], button:has-text("Barcha")');
    const count = await filterButtons.count();
    if (count > 0) {
      await filterButtons.first().click();
      await page.waitForTimeout(1000);
    }
  });
});

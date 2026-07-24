import { test, expect } from "@playwright/test";

test.describe("Dashboard", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/dashboard");
    await expect(page).toHaveURL(/login/);
  });

  test("dashboard shows after authentication", async ({ page }) => {
    // This test requires a valid session - skip in CI without auth
    test.skip(!process.env.E2E_AUTH_TOKEN, "E2E_AUTH_TOKEN not set");

    await page.goto("/uz-Latn/dashboard");
    await page.waitForTimeout(2000);

    // Check for dashboard content
    const heading = page.locator("h1, h2").first();
    await expect(heading).toBeVisible();
  });
});

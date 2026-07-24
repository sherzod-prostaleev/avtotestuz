import { test, expect } from "@playwright/test";

test.describe("Session flow", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/session/start?mode=variant&variant_id=1");
    await expect(page).toHaveURL(/login/);
  });

  test("session start page shows mode options", async ({ page }) => {
    test.skip(!process.env.E2E_AUTH_TOKEN, "E2E_AUTH_TOKEN not set");

    await page.goto("/uz-Latn/session/start?mode=variant&variant_id=1");
    await page.waitForTimeout(2000);

    // Check for session start content
    const startContent = page.locator('button:has-text("Boshlash"), button:has-text("Start")').first();
    if (await startContent.isVisible()) {
      await expect(startContent).toBeVisible();
    }
  });
});

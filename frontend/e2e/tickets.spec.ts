import { test, expect } from "@playwright/test";

test.describe("Tickets page", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/tickets");
    await expect(page).toHaveURL(/login/);
  });

  test("shows ticket grid after authentication", async ({ page }) => {
    // This test requires authentication - skip if not available
    test.skip(!process.env.E2E_AUTH_TOKEN, "E2E_AUTH_TOKEN not set");

    await page.goto("/uz-Latn/tickets");
    await page.waitForTimeout(2000);

    // Check for ticket cards
    const ticketCards = page.locator('[role="button"], [class*="ticket"]');
    const count = await ticketCards.count();
    expect(count).toBeGreaterThan(0);
  });
});

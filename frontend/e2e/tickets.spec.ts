import { test, expect } from "@playwright/test";
import { applyE2EAuthCookies, hasE2EAuthToken } from "./helpers/auth";

test.describe("Tickets page", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/tickets");
    await expect(page).toHaveURL(/login/);
  });

  test("authenticated session reaches tickets shell", async ({ page }) => {
    test.skip(!hasE2EAuthToken(), "E2E_AUTH_TOKEN not set");

    await applyE2EAuthCookies(page);
    await page.goto("/uz-Latn/tickets");
    await expect(page).not.toHaveURL(/login/);
    await expect(page).toHaveURL(/\/uz-Latn\/tickets/);
  });
});

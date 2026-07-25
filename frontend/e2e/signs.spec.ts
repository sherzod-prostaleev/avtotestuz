import { test, expect } from "@playwright/test";
import { applyE2EAuthCookies, hasE2EAuthToken } from "./helpers/auth";

test.describe("Road signs catalog", () => {
  test("redirects to login when not authenticated", async ({ page }) => {
    await page.goto("/uz-Latn/signs");
    await expect(page).toHaveURL(/login/);
  });

  test("authenticated session reaches signs shell", async ({ page }) => {
    test.skip(!hasE2EAuthToken(), "E2E_AUTH_TOKEN not set");

    await applyE2EAuthCookies(page);
    await page.goto("/uz-Latn/signs");
    await expect(page).not.toHaveURL(/login/);
    await expect(page).toHaveURL(/\/uz-Latn\/signs/);
  });
});

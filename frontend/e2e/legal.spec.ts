import { test, expect } from "@playwright/test";

test.describe("Public legal & pricing pages", () => {
  test("oferta loads without auth", async ({ page }) => {
    await page.goto("/uz-Latn/oferta");
    await expect(page).not.toHaveURL(/login/);
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/oferta/i);
  });

  test("privacy loads without auth", async ({ page }) => {
    await page.goto("/uz-Latn/privacy");
    await expect(page).not.toHaveURL(/login/);
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/maxfiylik|privacy|конфиденц/i);
  });

  test("narxlar loads without auth", async ({ page }) => {
    await page.goto("/uz-Latn/narxlar");
    await expect(page).not.toHaveURL(/login/);
    await expect(page.getByRole("heading", { level: 1 })).toContainText(/tarif/i);
  });

  test("landing footer links to legal pages", async ({ page }) => {
    await page.goto("/uz-Latn");
    await expect(page.locator('footer a[href*="/oferta"]').first()).toBeVisible();
    await expect(page.locator('footer a[href*="/privacy"]').first()).toBeVisible();
    await expect(page.locator('footer a[href*="/narxlar"]').first()).toBeVisible();
  });
});

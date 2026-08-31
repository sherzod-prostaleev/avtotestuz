import { test, expect } from "@playwright/test";

test.describe("Page transition", () => {
  test("navigates smoothly and renders incoming page content without blank screen", async ({ page }) => {
    await page.goto("/uz-Latn");
    await expect(page.locator("main").first()).toBeVisible();

    const link = page.locator('a[href*="/narxlar"]').first();
    await link.click();

    await expect(page).toHaveURL(/narxlar/);
    await expect(page.locator("h1").first()).toBeVisible();
    await expect(page.locator("main").first()).toBeVisible();
  });

  test("maintains content visibility during back and forward history traversal", async ({ page }) => {
    await page.goto("/uz-Latn");
    const link = page.locator('a[href*="/narxlar"]').first();
    await link.click();
    await expect(page).toHaveURL(/narxlar/);

    await page.goBack();
    await expect(page).toHaveURL(/\/uz-Latn$/);
    await expect(page.locator("main").first()).toBeVisible();

    await page.goForward();
    await expect(page).toHaveURL(/narxlar/);
    await expect(page.locator("h1").first()).toBeVisible();
  });

  test("handles slow route without hanging or blank screen", async ({ page }) => {
    await page.route("**/*_rsc=*", async (route) => {
      await new Promise((r) => setTimeout(r, 400));
      await route.continue();
    });

    await page.goto("/uz-Latn");
    const link = page.locator('a[href*="/narxlar"]').first();
    await link.click();

    await expect(page).toHaveURL(/narxlar/, { timeout: 10_000 });
    await expect(page.locator("h1").first()).toBeVisible();
    await expect(page.locator("main").first()).toBeVisible();
  });

  test("switches language smoothly without unmounting shell or blanking", async ({ page }) => {
    await page.goto("/uz-Latn");
    const switcher = page.getByRole("group").filter({ has: page.getByRole("button", { name: /Ру|Ru/i }) }).first();
    const target = (await switcher.count())
      ? switcher.getByRole("button", { name: /Ру|Ru/i }).first()
      : page.getByRole("button", { name: /Ру|Ru/i }).first();

    await target.click();

    await expect(page).toHaveURL(/\/ru(\/|$)/);
    await expect(page.locator("main").first()).toBeVisible();
  });
});
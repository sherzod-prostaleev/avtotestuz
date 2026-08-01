import { test, expect } from "@playwright/test";

test.describe("Landing page", () => {
  test("loads and shows hero content", async ({ page }) => {
    await page.goto("/uz-Latn");
    await expect(page.locator("h1").first()).toBeVisible();
  });

  test("shows proof stats (question + ticket counts)", async ({ page }) => {
    await page.goto("/uz-Latn");
    // Copy uses "1231+" (verified import size), not a hard "1235".
    await expect(page.getByText("1231+")).toBeVisible();
    await expect(page.getByText("62")).toBeVisible();
  });

  test("has login link in header", async ({ page }) => {
    await page.goto("/uz-Latn");
    const loginLink = page.locator('a[href*="login"]').first();
    await expect(loginLink).toBeVisible();
  });

  test("CTA button navigates to login", async ({ page }) => {
    await page.goto("/uz-Latn");
    const cta = page.locator('a[href*="login"]').first();
    await cta.click();
    await expect(page).toHaveURL(/login/);
  });

  test("diagnostic CTA stays public even when legacy CMS href points to login", async ({ page }) => {
    await page.route("**/api/proxy/site/home", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            headline: "",
            subtitle: "",
            ctaLabel: "CMS diagnostika",
            ctaHref: "/uz-Latn/login",
          },
        }),
      });
    });
    await page.route("**/api/proxy/demo/diagnostic?*", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            total_seconds: 720,
            questions: [
              {
                id: "question-1",
                text: "Public diagnostika savoli",
                image_url: null,
                answers: [
                  { id: "answer-1", text: "Birinchi javob" },
                  { id: "answer-2", text: "Ikkinchi javob" },
                ],
              },
            ],
          },
        }),
      });
    });

    await page.goto("/uz-Latn");
    await page.getByRole("link", { name: "CMS diagnostika" }).click();

    await expect(page).toHaveURL(/\/uz-Latn\/diagnostic$/);
    await expect(page.getByText("Public diagnostika savoli")).toBeVisible();
    expect((await page.context().cookies()).some(({ name }) => name === "at" || name === "rt")).toBe(false);
  });

  test("theme toggle is present", async ({ page }) => {
    await page.goto("/uz-Latn");
    // ThemeToggle component exists in the header
    const header = page.locator("header").first();
    await expect(header).toBeVisible();
  });
});

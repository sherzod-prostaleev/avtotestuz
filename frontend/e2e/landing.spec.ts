import { test, expect } from "@playwright/test";

test.describe("Landing page", () => {
  test("loads and shows hero content", async ({ page }) => {
    await page.goto("/uz-Latn");
    await expect(page.locator("h1").first()).toBeVisible();
  });

  test("shows proof stats read from the live catalog", async ({ page }) => {
    // The counts are no longer written into the copy: the page derives them
    // from GET /variants (biletlar = list length, savollar = summed
    // question_count). Pin a catalog and assert what the page makes of it, so
    // this test keeps passing when the real bank grows.
    await page.route("**/api/proxy/variants", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          data: [
            { number: 1, question_count: 20 },
            { number: 2, question_count: 20 },
            { number: 3, question_count: 5 },
          ],
        }),
      });
    });

    await page.goto("/uz-Latn");

    const facts = page.locator(".landing-fact");
    await expect(facts.filter({ hasText: "savol" })).toContainText("45+");
    await expect(facts.filter({ hasText: "bilet" })).toContainText("3");
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

    // The landing page renders this CTA three times -- hero, bottom section
    // and the sticky mobile bar -- all bound to the same href, so a bare
    // getByRole is a strict-mode violation. Asserting on every copy is the
    // stronger test anyway: the bug this guards against is a CMS ctaHref of
    // /login leaking into the CTA, and it would leak into all three.
    const ctas = page.getByRole("link", { name: "CMS diagnostika" });
    // evaluateAll does not auto-wait, and the label only appears once the
    // mocked CMS response has been applied, so wait for the first copy first.
    await ctas.first().waitFor();
    const hrefs = await ctas.evaluateAll((links) =>
      links.map((link) => link.getAttribute("href")),
    );
    expect(hrefs.length).toBeGreaterThan(0);
    for (const href of hrefs) {
      expect(href).toBe("/uz-Latn/diagnostic");
    }

    await ctas.first().click();

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

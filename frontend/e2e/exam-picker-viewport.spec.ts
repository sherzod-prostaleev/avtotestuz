import { expect, test, type Page } from "@playwright/test";

/**
 * The exam variety chooser has to be a *single* phone screen: both cards and
 * both start buttons reachable with a thumb, nothing below the fold. Learners
 * were scrolling past the second mode without noticing it, so this pins the
 * layout to the viewport at the sizes we actually ship to.
 *
 * Heights here are browser *viewport* heights (address bar already subtracted),
 * not device heights — that is what the page really gets.
 */
const PHONES = [
  { name: "android-compact", width: 360, height: 600 },
  { name: "iphone-se", width: 375, height: 553 },
  { name: "iphone-13", width: 390, height: 664 },
  { name: "pixel-7", width: 412, height: 730 },
] as const;

const ME = {
  profile: {
    id: "e2e-user",
    phone: "998901234567",
    name: "E2E",
    region: "Toshkent",
    district: "Chilonzor",
    birth_date: null,
    locale_pref: "uz-Latn",
    theme_pref: "dark",
    referral_code: "E2E123",
    role: "student",
    created_at: "2026-01-01T00:00:00Z",
  },
  vip: { active: true, until: "2027-01-01T00:00:00Z" },
};

/**
 * `/exam` is behind the proxy's session gate, which only checks that the auth
 * cookie exists — the value is never validated here because every API call is
 * stubbed below, so a placeholder gets us onto the page without a backend.
 */
async function seedSession(page: Page, baseURL: string | undefined): Promise<void> {
  await page.context().addCookies([
    {
      name: "at",
      value: "e2e-stub-not-a-real-token",
      url: baseURL ?? "http://localhost:3000",
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
}

async function stubApi(page: Page): Promise<void> {
  await page.route("**/api/proxy/me", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: ME }) }),
  );
  await page.route("**/api/proxy/**", (route) => {
    if (new URL(route.request().url()).pathname === "/api/proxy/me") return route.fallback();
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: null }) });
  });
}

/** Names the elements sticking out below the fold, so a failure points at a culprit. */
async function measureOverflow(page: Page) {
  return page.evaluate(() => {
    const doc = document.documentElement;
    const limit = doc.clientHeight;
    const tallest = Array.from(document.querySelectorAll<HTMLElement>("main *"))
      .map((el) => ({
        tag: el.tagName.toLowerCase(),
        cls: el.className?.toString().slice(0, 70) ?? "",
        bottom: Math.round(el.getBoundingClientRect().bottom),
      }))
      .filter((e) => e.bottom > limit + 1)
      .sort((a, b) => b.bottom - a.bottom)
      .slice(0, 5);
    return { overflow: doc.scrollHeight - doc.clientHeight, tallest };
  });
}

for (const phone of PHONES) {
  test.describe(`exam picker at ${phone.width}x${phone.height} (${phone.name})`, () => {
    test.beforeEach(async ({ page, baseURL }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await seedSession(page, baseURL);
      await stubApi(page);
      await page.goto("/uz-Latn/exam");
      await expect(page.getByTestId("exam-mode-standard")).toBeVisible();
    });

    test("fits one screen with no vertical scroll", async ({ page }) => {
      const { overflow, tallest } = await measureOverflow(page);
      expect(overflow, `page scrolls by ${overflow}px; culprits: ${JSON.stringify(tallest)}`).toBe(0);
    });

    test("both modes and both start buttons sit above the fold", async ({ page }) => {
      // The fixed bottom tab bar owns the last 4.25rem, so "visible" means
      // clear of it, not merely inside the viewport.
      const tabBarTop = phone.height - 68;

      for (const mode of ["standard", "restore"] as const) {
        const card = page.getByTestId(`exam-mode-${mode}`);
        await expect(card).toBeVisible();
        const box = await card.boundingBox();
        expect(box, `${mode} card has no box`).not.toBeNull();
        expect(box!.y, `${mode} card starts above the viewport`).toBeGreaterThanOrEqual(0);
        expect(
          Math.round(box!.y + box!.height),
          `${mode} card runs under the tab bar`,
        ).toBeLessThanOrEqual(tabBarTop);
      }

      // The CTA is the bottom-most thing inside each card; if it clears the tab
      // bar the whole card is genuinely usable.
      const ctas = page.locator("[data-testid^='exam-mode-'] [data-testid='exam-mode-cta']");
      await expect(ctas).toHaveCount(2);
      for (const cta of await ctas.all()) {
        const box = await cta.boundingBox();
        expect(Math.round(box!.y + box!.height)).toBeLessThanOrEqual(tabBarTop);
      }
    });

    // Fitting the screen must not turn into quietly cropping the rules: the
    // rule list clips its overflow, so a card that is too small for its own
    // content shows half a line of text rather than failing loudly.
    test("shows every rule of both exams in full", async ({ page }) => {
      const cropped = await page.evaluate(() =>
        Array.from(document.querySelectorAll<HTMLElement>("main dl")).map((el) => ({
          text: el.textContent?.trim().slice(0, 40) ?? "",
          hidden: el.scrollHeight - el.clientHeight,
        })),
      );
      expect(cropped).toHaveLength(2);
      for (const rules of cropped) {
        expect(rules.hidden, `rules cropped by ${rules.hidden}px: ${rules.text}`).toBeLessThanOrEqual(0);
      }
    });
  });
}

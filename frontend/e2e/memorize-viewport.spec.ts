import { expect, test, type Page } from "@playwright/test";

/**
 * Yodlash (memorize) has to behave like the live test screen on a phone:
 * every control — the numbered chips and the prev/next buttons — reachable
 * with a thumb, nothing below the fold.
 *
 * This exists because it shipped broken once. The page first lived in the
 * (app) route group, whose shell adds a 60px top bar and a 4.25rem bottom tab
 * bar, while the page itself used `.session-shell`, which sizes itself to the
 * full viewport. On a phone the footer landed under the tab bar: the learner
 * could open a topic and then not leave the first question. Component tests
 * render the page with no shell around it, so only a real viewport catches it.
 *
 * Heights here are browser *viewport* heights (address bar already
 * subtracted), not device heights — that is what the page really gets.
 */
const PHONES = [
  { name: "android-compact", width: 360, height: 600 },
  { name: "iphone-se", width: 375, height: 553 },
  { name: "iphone-13", width: 390, height: 664 },
  { name: "pixel-7", width: 412, height: 730 },
] as const;

const QUESTION_COUNT = 12;

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

function memorizePayload() {
  return Array.from({ length: QUESTION_COUNT }, (_, index) => ({
    id: `q-${index + 1}`,
    category_code: "signs",
    text: `Chorrahaga yaqinlashayotgan haydovchi ${index + 1}-vaziyatda qanday yo'l tutishi kerak?`,
    image_url: null,
    answers: [
      { id: `q-${index + 1}-a1`, position: 1, text: "Tezlikni kamaytirib, yo'l belgisiga rioya qiladi", image_url: null },
      { id: `q-${index + 1}-a2`, position: 2, text: "Qarama-qarshi transport o'tguncha kutadi", image_url: null },
      { id: `q-${index + 1}-a3`, position: 3, text: "Piyodalarga yo'l beradi", image_url: null },
    ],
    signs: [],
    explanation: null,
    position: index + 1,
    answered: true,
    correct_answer_id: `q-${index + 1}-a2`,
  }));
}

/**
 * /practice/... is behind the proxy's session gate, which only checks that the
 * auth cookie exists — the value is never validated here because every API
 * call is stubbed below, so a placeholder gets us onto the page.
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
  await page.route("**/api/proxy/categories/*/memorize**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: memorizePayload(), meta: { locale: "uz-Latn", fallback: false } }),
    }),
  );
  await page.route("**/api/proxy/**", (route) => {
    const pathname = new URL(route.request().url()).pathname;
    if (pathname === "/api/proxy/me" || pathname.endsWith("/memorize")) return route.fallback();
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: null }) });
  });
}

async function layoutMetrics(page: Page) {
  return page.evaluate(() => {
    const footer = document.querySelector<HTMLElement>(".session-actions");
    const navigator = document.querySelector<HTMLElement>(".session-navigator");
    const stage = document.querySelector<HTMLElement>("[data-testid='question-stage']");
    if (!footer || !navigator || !stage) throw new Error("memorize layout is incomplete");

    const doc = document.documentElement;
    const footerRect = footer.getBoundingClientRect();
    const navRect = navigator.getBoundingClientRect();

    return {
      pageOverflow: Math.max(doc.scrollHeight, document.body.scrollHeight) - window.innerHeight,
      footerTop: footerRect.top,
      footerBottom: footerRect.bottom,
      navBottom: navRect.bottom,
      chipCount: navigator.querySelectorAll("button").length,
    };
  });
}

test.describe("memorize viewport fit", () => {
  for (const phone of PHONES) {
    test(`${phone.name} keeps every memorize control on screen`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width: phone.width, height: phone.height });
      await seedSession(page, baseURL);
      await stubApi(page);
      await page.goto("/uz-Latn/practice/memorize/signs");

      await expect(page.getByTestId("question-stage")).toBeVisible();

      const metrics = await layoutMetrics(page);
      await test.info().attach("memorize-layout", {
        body: JSON.stringify({ phone, metrics }, null, 2),
        contentType: "application/json",
      });

      // The page itself must not scroll — the session shell owns its height.
      expect(metrics.pageOverflow).toBeLessThanOrEqual(1);
      // The regression that shipped: the footer (prev / next) below the fold.
      expect(metrics.footerBottom).toBeLessThanOrEqual(phone.height + 1);
      expect(metrics.footerTop).toBeGreaterThanOrEqual(0);
      // Every question is reachable from the chips, like the live test screen.
      expect(metrics.chipCount).toBe(QUESTION_COUNT);
      expect(metrics.navBottom).toBeLessThanOrEqual(phone.height + 1);
    });
  }

  test("advances with Keyingisi and jumps from the numbered chips on a phone", async ({ page, baseURL }) => {
    const phone = PHONES[1]; // iphone-se, the shortest viewport we ship to
    await page.setViewportSize({ width: phone.width, height: phone.height });
    await seedSession(page, baseURL);
    await stubApi(page);
    await page.goto("/uz-Latn/practice/memorize/signs");

    await expect(page.getByText("Savol 1 / 12")).toBeVisible();

    await page.getByRole("button", { name: /Keyingisi/ }).click();
    await expect(page.getByText("Savol 2 / 12")).toBeVisible();

    const navigator = page.getByRole("navigation", { name: "Savollar navigatori" });
    await navigator.getByRole("button", { name: /^7-savol/ }).click();
    await expect(page.getByText("Savol 7 / 12")).toBeVisible();

    // The correct option is marked without anyone answering anything.
    await expect(page.getByTestId("answer-correct-icon")).toBeVisible();
  });
});

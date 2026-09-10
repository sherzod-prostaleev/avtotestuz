import { expect, test, type Page } from "@playwright/test";

/**
 * Yodlash (memorize) has to behave like the live test screen on every screen we
 * ship to: the image, the question and every answer visible at once, every
 * control reachable, nothing below the fold, nothing scrolled to.
 *
 * This exists because it shipped broken twice.
 *
 * 1. The page first lived in the (app) route group, whose shell adds a 60px top
 *    bar and a 4.25rem bottom tab bar, while the page itself used
 *    `.session-shell`, which sizes itself to the full viewport. On a phone the
 *    footer landed under the tab bar: the learner could open a topic and then
 *    not leave the first question.
 * 2. Then on a classroom TV: the numbered chips are a wrapping grid borrowed
 *    from the live test screen, which never shows more than 20 of them. A topic
 *    hands back its WHOLE question list (session.CategoryMemorize takes no
 *    limit), so ~120 chips wrapped into five rows, pinned the footer at a third
 *    of the screen and squeezed the answers until the last one was cut in half.
 *
 * Both are invisible to component tests, which render the page with no shell
 * and no real height around it. Only a real viewport catches them — so the
 * counts and viewports below are deliberately realistic, not convenient.
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

/** Classroom PCs and the TVs they are plugged into, plus the office laptop. */
const BIG_SCREENS = [
  { name: "tv-720p", width: 1280, height: 720 },
  { name: "laptop-1366", width: 1366, height: 768 },
  { name: "tv-1080p", width: 1920, height: 1080 },
] as const;

/**
 * A real topic is not a bilet. The largest categories run past a hundred
 * questions, and that is the number the chip strip has to survive.
 */
const QUESTION_COUNT = 120;

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
    text: `Chorrahaga yaqinlashayotgan haydovchi ${index + 1}-vaziyatda qanday yo'l tutishi kerak va qanday qoidaga amal qilishi lozim?`,
    image_url: "/media/questions/sample.png",
    answers: [
      { id: `q-${index + 1}-a1`, position: 1, text: "Tezlikni kamaytirib, yo'l belgisiga rioya qiladi", image_url: null },
      { id: `q-${index + 1}-a2`, position: 2, text: "Qarama-qarshi transport o'tguncha kutadi", image_url: null },
      { id: `q-${index + 1}-a3`, position: 3, text: "Piyodalarga yo'l beradi", image_url: null },
      {
        id: `q-${index + 1}-a4`,
        position: 4,
        text: "To'xtash chizig'i oldida to'liq to'xtaydi va keyin harakatni davom ettiradi",
        image_url: null,
      },
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
  // Question media is not served in e2e. Hand back a real image so the picture
  // box carries its intrinsic size and competes for height like it does live.
  await page.route("**/media/**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600"><rect width="800" height="600" fill="#456"/></svg>',
    }),
  );
}

async function layoutMetrics(page: Page) {
  return page.evaluate(() => {
    const footer = document.querySelector<HTMLElement>(".session-actions");
    const navigator = document.querySelector<HTMLElement>(".session-navigator");
    const stage = document.querySelector<HTMLElement>("[data-testid='question-stage']");
    const answers = document.querySelector<HTMLElement>("[data-testid='answer-list']");
    if (!footer || !navigator || !stage || !answers) throw new Error("memorize layout is incomplete");

    const doc = document.documentElement;
    const footerRect = footer.getBoundingClientRect();
    const navRect = navigator.getBoundingClientRect();

    return {
      pageOverflow: Math.max(doc.scrollHeight, document.body.scrollHeight) - window.innerHeight,
      footerTop: footerRect.top,
      footerBottom: footerRect.bottom,
      footerHeight: footerRect.height,
      navBottom: navRect.bottom,
      navHeight: navRect.height,
      // A wrapped chip grid scrolls vertically; the single strip must not.
      navVerticalOverflow: navigator.scrollHeight - navigator.clientHeight,
      // How much of the answer list is cut off — the half-visible options.
      answersClipped: answers.scrollHeight - answers.clientHeight,
      // The complaint itself: an option the learner can only half see. Measured
      // per option, because a list that scrolls by a hair reads fine while one
      // that hides a whole option does not.
      worstOptionHidden: Array.from(
        answers.querySelectorAll<HTMLElement>("[data-answer-option]"),
      ).reduce((worst, option) => {
        const box = option.getBoundingClientRect();
        const list = answers.getBoundingClientRect();
        const hidden = Math.max(0, box.bottom - list.bottom) + Math.max(0, list.top - box.top);
        return Math.max(worst, hidden);
      }, 0),
      chipCount: navigator.querySelectorAll("button").length,
    };
  });
}

/**
 * A few pixels of the last option's bottom border can be trimmed on a short
 * desktop: a "default density" question whose longest option wraps to two lines
 * sits right at the edge of what the shared session shell fits, and the live
 * test screen trims the same hair on the same question. What must never come
 * back is a whole option hidden — the wrapped chip grid cut 107px, half an
 * option, off a classroom TV.
 */
const CLIP_TOLERANCE = 8;

test.describe("memorize viewport fit", () => {
  for (const screen of [...PHONES, ...BIG_SCREENS]) {
    test(`${screen.name} keeps every memorize control on screen`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width: screen.width, height: screen.height });
      await seedSession(page, baseURL);
      await stubApi(page);
      await page.goto("/uz-Latn/practice/memorize/signs");

      await expect(page.getByTestId("question-stage")).toBeVisible();

      const metrics = await layoutMetrics(page);
      await test.info().attach("memorize-layout", {
        body: JSON.stringify({ screen, metrics }, null, 2),
        contentType: "application/json",
      });

      // The page itself must not scroll — the session shell owns its height.
      expect(metrics.pageOverflow).toBeLessThanOrEqual(1);
      // The regression that shipped first: the footer (prev / next) below the fold.
      expect(metrics.footerBottom).toBeLessThanOrEqual(screen.height + 1);
      expect(metrics.footerTop).toBeGreaterThanOrEqual(0);
      // Every question is reachable from the chips, like the live test screen.
      expect(metrics.chipCount).toBe(QUESTION_COUNT);
      expect(metrics.navBottom).toBeLessThanOrEqual(screen.height + 1);

      // The regression that shipped second: a whole topic's worth of chips
      // wrapping into rows until they owned a third of the screen.
      expect(metrics.navVerticalOverflow).toBeLessThanOrEqual(1);
      expect(metrics.footerHeight).toBeLessThanOrEqual(screen.height * 0.25);
    });
  }

  // The squeeze itself: on a classroom TV the last option was cut in half and
  // the learner had to scroll a box they could not see the edges of.
  for (const screen of BIG_SCREENS) {
    test(`${screen.name} shows every answer without scrolling the list`, async ({ page, baseURL }) => {
      await page.setViewportSize({ width: screen.width, height: screen.height });
      await seedSession(page, baseURL);
      await stubApi(page);
      await page.goto("/uz-Latn/practice/memorize/signs");
      await expect(page.getByTestId("question-stage")).toBeVisible();

      const metrics = await layoutMetrics(page);
      await test.info().attach("memorize-answers", {
        body: JSON.stringify({ screen, metrics }, null, 2),
        contentType: "application/json",
      });
      expect(metrics.answersClipped).toBeLessThanOrEqual(CLIP_TOLERANCE);
      expect(metrics.worstOptionHidden).toBeLessThanOrEqual(CLIP_TOLERANCE);

      // Deep into a long topic the chips have scrolled themselves — the stage
      // above them must be untouched by that.
      await page
        .getByRole("navigation", { name: "Savollar navigatori" })
        .getByRole("button", { name: /^90-savol/ })
        .click();
      await expect(page.getByText(`Savol 90 / ${QUESTION_COUNT}`)).toBeVisible();

      const deep = await layoutMetrics(page);
      expect(deep.answersClipped).toBeLessThanOrEqual(CLIP_TOLERANCE);
      expect(deep.worstOptionHidden).toBeLessThanOrEqual(CLIP_TOLERANCE);
      expect(deep.pageOverflow).toBeLessThanOrEqual(1);
      expect(deep.footerBottom).toBeLessThanOrEqual(screen.height + 1);
    });
  }

  // The kiosk is where the classroom TV actually lives. Its layout shell adds a
  // floating language + theme bar over every station page, which is suppressed
  // over a running session because it would cover the header — Yodlash was left
  // out of that rule and the bar landed on top of its badge.
  for (const screen of BIG_SCREENS) {
    test(`${screen.name} kiosk memorize is not covered by the floating station bar`, async ({ page }) => {
      await page.setViewportSize({ width: screen.width, height: screen.height });
      await stubApi(page);
      await page.goto("/uz-Latn/station/practice/memorize/signs");
      await expect(page.getByTestId("question-stage")).toBeVisible();

      await expect(page.getByTestId("kiosk-chrome")).toHaveCount(0);
      // Nothing on top of the header: the badge and the exit button are the
      // topmost things at their own coordinates.
      const covered = await page.evaluate(() => {
        const header = document.querySelector<HTMLElement>(".session-header");
        if (!header) throw new Error("no memorize header");
        return Array.from(header.querySelectorAll<HTMLElement>("span, button, select"))
          .filter((el) => el.getBoundingClientRect().width > 0)
          .filter((el) => {
            const box = el.getBoundingClientRect();
            const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
            return !hit || !(el.contains(hit) || hit.contains(el));
          })
          .map((el) => `${el.tagName}:${el.textContent?.trim().slice(0, 20)}`);
      });
      expect(covered).toEqual([]);

      // The kiosk still has a language control — it just lives in the header
      // now, the way the live test screen's does.
      await expect(page.getByLabel("Sessiya tili")).toBeVisible();

      const metrics = await layoutMetrics(page);
      expect(metrics.pageOverflow).toBeLessThanOrEqual(1);
      expect(metrics.navVerticalOverflow).toBeLessThanOrEqual(1);
      expect(metrics.answersClipped).toBeLessThanOrEqual(CLIP_TOLERANCE);
      expect(metrics.footerBottom).toBeLessThanOrEqual(screen.height + 1);
    });
  }

  test("advances with Keyingisi and jumps from the numbered chips on a phone", async ({ page, baseURL }) => {
    const phone = PHONES[1]; // iphone-se, the shortest viewport we ship to
    await page.setViewportSize({ width: phone.width, height: phone.height });
    await seedSession(page, baseURL);
    await stubApi(page);
    await page.goto("/uz-Latn/practice/memorize/signs");

    await expect(page.getByText(`Savol 1 / ${QUESTION_COUNT}`)).toBeVisible();

    await page.getByRole("button", { name: /Keyingisi/ }).click();
    await expect(page.getByText(`Savol 2 / ${QUESTION_COUNT}`)).toBeVisible();

    const navigator = page.getByRole("navigation", { name: "Savollar navigatori" });
    await navigator.getByRole("button", { name: /^7-savol/ }).click();
    await expect(page.getByText(`Savol 7 / ${QUESTION_COUNT}`)).toBeVisible();

    // The correct option is marked without anyone answering anything.
    await expect(page.getByTestId("answer-correct-icon")).toBeVisible();
  });

  // A classroom PC is driven from the keyboard. The live test screen walks its
  // questions with the arrow keys; Yodlash used to ignore them entirely, which
  // left a mouse as the only way through a 120-question topic.
  test("walks the topic with the arrow keys on a classroom PC", async ({ page, baseURL }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await seedSession(page, baseURL);
    await stubApi(page);
    await page.goto("/uz-Latn/practice/memorize/signs");

    await expect(page.getByText(`Savol 1 / ${QUESTION_COUNT}`)).toBeVisible();

    // The question text paints before React hydrates, and the keydown listener
    // only exists from hydration onwards — a key pressed in between is lost
    // rather than queued. Retry the first press until the page is live; every
    // press after it lands on a hydrated page.
    await expect(async () => {
      await page.keyboard.press("ArrowRight");
      await expect(page.getByText(`Savol 2 / ${QUESTION_COUNT}`)).toBeVisible({ timeout: 1_000 });
    }).toPass({ timeout: 20_000 });
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowRight");
    await expect(page.getByText(`Savol 4 / ${QUESTION_COUNT}`)).toBeVisible();

    await page.keyboard.press("ArrowLeft");
    await expect(page.getByText(`Savol 3 / ${QUESTION_COUNT}`)).toBeVisible();

    // Walking with the keyboard must keep the active chip in view, exactly as
    // clicking one does — otherwise the strip and the stage disagree.
    await page.keyboard.press("ArrowLeft");
    await page.keyboard.press("ArrowLeft");
    await expect(page.getByText(`Savol 1 / ${QUESTION_COUNT}`)).toBeVisible();
    await expect(
      page.getByRole("navigation", { name: "Savollar navigatori" }).getByRole("button", { name: /^1-savol/ }),
    ).toBeInViewport();
  });
});

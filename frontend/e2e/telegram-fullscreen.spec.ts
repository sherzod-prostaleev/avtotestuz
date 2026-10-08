import { test, expect, type Page } from "@playwright/test";
import { openInFakeTelegram, tapTelegramBack, tgCalls } from "./helpers/fake-telegram";

// A phone in Telegram fullscreen: the device safe area (notch) plus the strip
// Telegram's own close/menu controls cover. Nothing of ours may sit in it.
const SAFE_TOP = 47;
const CONTENT_TOP = 46;
const SAFE_BOTTOM = 34;
const COVERED_TOP = SAFE_TOP + CONTENT_TOP;

const BASE = "http://localhost:" + (process.env.PORT || 3000);
const meOk = { data: { profile: { must_change_password: false, display_name: "Ali" }, vip: { active: false, until: null } } };
const unauthorized = { error: { code: "unauthorized" } };
const needPhone = { data: { need_phone: true, first_name: "Ali" } };

const EXAM_ID = "tg-fullscreen-exam";
const examQuestion = {
  id: "q1",
  category_code: "layout",
  text: "Ushbu belgi haydovchini nima haqida ogohlantiradi?",
  image_url: "/exam/placeholder-driver-go-cars.png",
  answers: ["Yo'l ishlari", "Boshqa xavf", "Tor yo'l"].map((text, i) => ({
    id: `a${i + 1}`,
    position: i + 1,
    text,
    image_url: null,
  })),
  signs: [],
  explanation: null,
  position: 1,
  answered: false,
  user_answer_id: null,
};

async function fullscreenPhone(page: Page) {
  await openInFakeTelegram(page, {
    platform: "ios",
    safeTop: SAFE_TOP,
    safeBottom: SAFE_BOTTOM,
    fullscreenContentTop: CONTENT_TOP,
  });
}

async function signedIn(page: Page) {
  await page.context().addCookies([{ name: "at", value: "x", url: BASE }]);
  await page.route("**/api/proxy/**", (r) =>
    r.fulfill({ json: r.request().url().endsWith("/api/proxy/me") ? meOk : { data: [] } })
  );
}

async function stubExam(page: Page) {
  await page.route(`**/api/proxy/sessions/${EXAM_ID}**`, (r) => {
    const path = new URL(r.request().url()).pathname;
    if (path.endsWith(`/sessions/${EXAM_ID}`)) {
      return r.fulfill({
        json: {
          data: {
            id: EXAM_ID,
            mode: "exam",
            total: 1,
            status: "in_progress",
            stopped_reason: "",
            time_limit_sec: 1500,
            started_at: new Date().toISOString(),
            answers: [{ question_id: "q1", position: 1, answered: false }],
          },
        },
      });
    }
    if (path.endsWith("/questions")) return r.fulfill({ json: { data: [examQuestion] } });
    if (path.endsWith("/questions/q1")) return r.fulfill({ json: { data: examQuestion } });
    return r.fulfill({ json: { data: [] } });
  });
}

/**
 * Every visible piece of content (text, controls, images) that reaches into
 * the strip Telegram covers in fullscreen. Backgrounds and borders may run
 * under it; content may not.
 */
async function contentUnderControls(page: Page): Promise<string[]> {
  return page.evaluate((covered) => {
    const hits: string[] = [];
    const nodes = document.querySelectorAll<HTMLElement>(
      "a, button, input, select, textarea, img, svg, h1, h2, h3, p, label, [role=button], [role=dialog] *"
    );
    for (const el of nodes) {
      const r = el.getBoundingClientRect();
      if (r.width < 1 || r.height < 1 || r.bottom <= 0 || r.top >= covered) continue;
      // A viewport-sized scrim or backdrop is meant to run under them.
      if (r.width * r.height >= 0.8 * window.innerWidth * window.innerHeight) continue;
      const style = getComputedStyle(el);
      if (style.visibility === "hidden" || Number(style.opacity) === 0) continue;
      // A leaf with no text and no box of its own (an empty wrapper) is not content.
      if (!(el.textContent ?? "").trim() && !["IMG", "svg", "INPUT", "BUTTON", "A"].includes(el.tagName)) continue;
      if (el.closest("[aria-hidden=true]")) continue;
      hits.push(`${el.tagName.toLowerCase()}@${Math.round(r.top)} "${(el.textContent ?? "").trim().slice(0, 30)}"`);
    }
    return hits;
  }, COVERED_TOP);
}

async function shoot(page: Page, name: string) {
  const dir = process.env.TG_SHOTS_DIR;
  const path = dir ? `${dir}/${name}.png` : test.info().outputPath(`${name}.png`);
  await page.screenshot({ path });
}

async function expectClearOfControls(page: Page, name: string) {
  // The fullscreen inset arrives with the SDK, after the first paint.
  await expect(page.locator("html.tg-webapp")).toHaveCount(1);
  await expect
    .poll(() => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--tg-content-safe-area-inset-top")))
    .toBe(`${CONTENT_TOP}px`);
  await page.evaluate(() => document.fonts.ready);
  await shoot(page, name);
  expect(await contentUnderControls(page)).toEqual([]);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
}

test.describe("Telegram Mini App fullscreen on a phone", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("launch asks for fullscreen and keeps vertical swipes disabled", async ({ page }) => {
    await fullscreenPhone(page);
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    await expect.poll(() => tgCalls(page)).toContain("requestFullscreen");
    const calls = await tgCalls(page);
    expect(calls).toContain("disableVerticalSwipes");
    expect(calls.indexOf("requestFullscreen")).toBeGreaterThan(calls.indexOf("expand"));
    await expectClearOfControls(page, "tg-welcome");
  });

  test("desktop Telegram stays windowed", async ({ page }) => {
    await openInFakeTelegram(page, { platform: "tdesktop", fullscreenContentTop: CONTENT_TOP });
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    await expect.poll(() => tgCalls(page)).toContain("expand");
    expect(await tgCalls(page)).not.toContain("requestFullscreen");
  });

  test("login", async ({ page }) => {
    await fullscreenPhone(page);
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.goto("/uz-Latn/login");
    await expect(page.locator("form")).toBeVisible();
    await expectClearOfControls(page, "login");
  });

  test("dashboard and its menu drawer", async ({ page }) => {
    await fullscreenPhone(page);
    await signedIn(page);
    await page.goto("/uz-Latn/dashboard");
    await expect(page.locator(".app-top-bar")).toBeVisible();
    await expectClearOfControls(page, "dashboard");
    const pads = await page.evaluate(() => ({
      top: parseFloat(getComputedStyle(document.querySelector(".app-top-bar")!).paddingTop),
      bottom: parseFloat(getComputedStyle(document.querySelector("nav.app-bottom-nav")!).paddingBottom),
    }));
    expect(pads.top).toBeGreaterThanOrEqual(COVERED_TOP);
    expect(pads.bottom).toBeGreaterThanOrEqual(SAFE_BOTTOM);
    await page.locator(".app-top-bar button").last().click();
    // Wait out the 300ms slide-in before measuring.
    await expect.poll(async () => (await page.locator("aside.app-drawer").boundingBox())?.x).toBe(0);
    await expectClearOfControls(page, "dashboard-drawer");
  });

  test("profile", async ({ page }) => {
    await fullscreenPhone(page);
    await signedIn(page);
    await page.goto("/uz-Latn/profile");
    await expect(page.locator(".app-top-bar")).toBeVisible();
    await expectClearOfControls(page, "profile");
  });

  test("exam runner", async ({ page }) => {
    await fullscreenPhone(page);
    await signedIn(page);
    await stubExam(page);
    await page.goto(`/uz-Latn/session/${EXAM_ID}`);
    await expect(page.locator(".exam-top-bar")).toBeVisible();
    await expect(page.getByText("Boshqa xavf")).toBeVisible();
    await expectClearOfControls(page, "exam-runner");
  });

  test("a tall bottom sheet on a short phone stops below the controls", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await fullscreenPhone(page);
    await page.context().addCookies([{ name: "at", value: "x", url: BASE }]);
    const items = Array.from({ length: 12 }, (_, i) => ({
      id: `n${i}`,
      title: `Yangi bilet qo'shildi ${i + 1}`,
      body: "Rasmiy savollar yangilandi — mashq qilib ko'ring.",
      read_at: null,
      created_at: "2026-10-08T09:00:00Z",
    }));
    await page.route("**/api/proxy/**", (r) => {
      const url = r.request().url();
      if (url.endsWith("/api/proxy/me")) return r.fulfill({ json: meOk });
      if (url.includes("/me/notifications/unread-count")) return r.fulfill({ json: { data: { unread: 12 } } });
      if (url.includes("/me/notifications")) return r.fulfill({ json: { data: { items } } });
      return r.fulfill({ json: { data: [] } });
    });
    await page.goto("/uz-Latn/dashboard");
    await expect(page.locator("html.tg-webapp")).toHaveCount(1);
    await page.locator(".app-top-bar button").first().click();
    const sheet = page.getByRole("dialog");
    await expect(sheet.getByText("Yangi bilet qo'shildi 12")).toBeAttached();
    await expect.poll(async () => (await sheet.boundingBox())?.y ?? 0).toBeGreaterThanOrEqual(COVERED_TOP);
    await expectClearOfControls(page, "notifications-short-phone");
  });

  test("BackButton still navigates in fullscreen", async ({ page }) => {
    await fullscreenPhone(page);
    await signedIn(page);
    await page.goto("/uz-Latn/signs");
    await expect.poll(() => tgCalls(page)).toContain("back.show");
    expect(await tgCalls(page)).toContain("requestFullscreen");
    await tapTelegramBack(page);
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/);
  });
});

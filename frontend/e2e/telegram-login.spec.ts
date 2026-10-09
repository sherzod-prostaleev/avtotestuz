import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { openInFakeTelegram } from "./helpers/fake-telegram";

// «Telegram orqali kirish» (website), the passwordless-account message, the
// Mini App one-tap phone sign-in and the Telegram referral link. No backend:
// the BFF routes the browser calls are stubbed with page.route.

const ORIGIN = "http://localhost:" + (process.env.PORT || 3000);
const TOKEN = "Tk".repeat(21) + "x";
const BOT_URL = `https://t.me/DriverGouzBot?start=login_${TOKEN}`;
const meOk = (extra: Record<string, unknown> = {}) => ({
  data: {
    profile: {
      id: "p1",
      phone: "+998901112233",
      name: "Ali",
      region: "",
      district: "",
      birth_date: null,
      locale_pref: "uz-Latn",
      theme_pref: "dark",
      referral_code: "",
      role: "user",
      must_change_password: false,
      has_password: true,
      created_at: "2026-10-01T00:00:00Z",
      ...extra,
    },
    vip: { active: false, until: null },
  },
});

async function stubTelegram(page: Page | BrowserContext) {
  // Never leave the test sandbox for t.me.
  await page.route("https://t.me/**", (r) => r.fulfill({ contentType: "text/html", body: "<title>t.me</title>Telegram" }));
}

async function stubLoginBackend(page: Page, context: BrowserContext, states: string[]) {
  let i = 0;
  let completed = 0;
  await page.route("**/api/auth/telegram-login/start", (r) =>
    r.fulfill({ json: { data: { bot_url: BOT_URL, token: TOKEN, expires_in_sec: 300 } } })
  );
  await page.route("**/api/auth/telegram-login/status**", (r) =>
    r.fulfill({ json: { data: { state: states[Math.min(i++, states.length - 1)] } } })
  );
  await page.route("**/api/auth/telegram-login/complete", async (r) => {
    completed++;
    expect(r.request().postDataJSON()).toEqual({ token: TOKEN });
    // The real route sets the site's at/rt; the middleware only checks presence.
    await context.addCookies([{ name: "at", value: "x", url: ORIGIN }]);
    return r.fulfill({
      json: { data: { ok: true, must_change_password: false, created: false, phone_masked: "+998 90 ••• •• 33" } },
    });
  });
  await page.route("**/api/proxy/**", (r) =>
    r.fulfill({ json: r.request().url().endsWith("/api/proxy/me") ? meOk() : { data: [] } })
  );
  return () => completed;
}

test.describe("Telegram login on the website", () => {
  test("desktop: start opens the bot in a new tab with a QR code, approval lands on the dashboard", async ({ page, context }) => {
    await stubTelegram(context);
    const completed = await stubLoginBackend(page, context, ["pending", "pending", "approved"]);
    await page.goto("/uz-Latn/login");
    const popupPromise = context.waitForEvent("page");
    await page.getByRole("button", { name: "Telegram orqali kirish" }).click();
    const popup = await popupPromise;
    await expect.poll(() => popup.url()).toBe(BOT_URL);
    await expect(page.getByRole("heading", { name: "Telegram'da tasdiqlang" })).toBeVisible();
    await expect(page.getByRole("img", { name: "Telegram orqali kirish havolasining QR kodi" })).toBeVisible();
    // First sign-in is two taps in the bot; the waiting screen says so.
    await expect(
      page.getByText("Birinchi marta bo'lsa — avval «📱 Raqamni yuborish» ni, so'ng «✅ Kirish» ni bosing.")
    ).toBeVisible();
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/, { timeout: 15_000 });
    expect(completed()).toBe(1);
    // Which account this browser ended up in: said once, clearly (a shared
    // or classroom screen must notice a wrong account at once).
    const notice = page.getByRole("status").filter({ hasText: "raqami bilan kirdingiz" });
    // The number is kept on one line with no-break spaces, hence \s.
    await expect(notice).toHaveText(/\+998\s90\s•••\s••\s33 raqami bilan kirdingiz/);
    await expect(notice.getByRole("button", { name: "Bu sizning raqamingiz emasmi? Chiqish" })).toBeVisible();
    await page.reload();
    await expect(page.getByRole("heading").first()).toBeVisible();
    await expect(page.getByText("raqami bilan kirdingiz")).toHaveCount(0);
  });

  test("the telegram_login kill switch hides the button and its divider; the password form stays", async ({ page }) => {
    await page.route("**/api/proxy/flags", (r) => r.fulfill({ json: { data: { telegram_login: false } } }));
    await page.goto("/uz-Latn/login");
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.getByRole("button", { name: "Telegram orqali kirish" })).toHaveCount(0);
    await expect(page.getByText("yoki telefon raqam va parol bilan")).toBeHidden();
    await page.goto("/uz-Latn/register");
    await expect(page.getByRole("button", { name: "Telegram orqali ro'yxatdan o'tish" })).toHaveCount(0);
  });

  test("phone: the page goes to t.me and resumes waiting when the learner comes back", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      userAgent: "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Mobile Safari/537.36",
    });
    const page = await context.newPage();
    await stubTelegram(context);
    const completed = await stubLoginBackend(page, context, ["pending", "approved"]);
    await page.goto("/uz-Latn/login");
    await page.getByRole("button", { name: "Telegram orqali kirish" }).click();
    await expect(page).toHaveURL(BOT_URL);
    await page.goBack();
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/, { timeout: 15_000 });
    expect(completed()).toBe(1);
    await context.close();
  });

  test("a cancel in Telegram ends on a clear notice with a retry", async ({ page, context }) => {
    await stubTelegram(context);
    await stubLoginBackend(page, context, ["cancelled"]);
    await page.goto("/uz-Latn/login");
    await page.getByRole("button", { name: "Telegram orqali kirish" }).click();
    await expect(page.getByRole("heading", { name: "Kirish bekor qilindi" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Qaytadan urinish" })).toBeVisible();
  });

  test("passwordless account: the password form explains and offers Telegram or a password", async ({ page }) => {
    await page.route("**/api/auth/login", (r) =>
      r.fulfill({ status: 409, json: { error: { code: "password_not_set", message: "x" } } })
    );
    await page.goto("/uz-Latn/login");
    await page.locator('input[type="tel"]').fill("901112233");
    await page.locator('input[type="password"]').fill("secret123");
    await page.locator("form button[type=submit]").click();
    const panel = page.getByRole("alert").filter({ hasText: "Parol o'rnatilmagan" });
    await expect(panel).toContainText("Siz Telegram orqali ro'yxatdan o'tgansiz. Telegram orqali kiring yoki parol o'rnating.");
    await expect(panel.getByRole("button", { name: "Telegram orqali kirish" })).toBeVisible();
    await panel.getByRole("link", { name: "Parol o'rnatish" }).click();
    // The number is carried over without ever appearing in a URL.
    await expect(page).toHaveURL(/\/uz-Latn\/forgot-password$/);
    await expect(page.locator('input[type="tel"]')).toHaveValue("90 111 22 33");
    // Read once: a reload starts with an empty field.
    await page.reload();
    await expect(page.locator('input[type="tel"]')).toHaveValue("");
  });
});

test.describe("Mini App one-tap phone sign-in", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("«📱 Raqam bilan davom etish» shares the number and opens the dashboard", async ({ page, context }) => {
    await openInFakeTelegram(page, { startParam: "ref_REF-AB23CD" });
    let signedIn = false;
    let body: Record<string, unknown> | null = null;
    await page.route("**/api/proxy/**", (r) => {
      if (!r.request().url().endsWith("/api/proxy/me")) return r.fulfill({ json: { data: [] } });
      return signedIn ? r.fulfill({ json: meOk() }) : r.fulfill({ status: 401, json: { error: { code: "unauthorized" } } });
    });
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: { data: { need_phone: true, first_name: "Ali" } } }));
    await page.route("**/api/auth/telegram/phone", async (r) => {
      body = r.request().postDataJSON();
      signedIn = true;
      await context.addCookies([{ name: "at", value: "x", url: ORIGIN }]);
      return r.fulfill({ json: { data: { ok: true, must_change_password: false, created: true } } });
    });
    await page.goto("/uz-Latn/tg");
    // The invite rides the password registration link too.
    await expect(page.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register?ref=REF-AB23CD");
    await page.getByRole("button", { name: "📱 Raqam bilan davom etish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/);
    expect(body).toEqual({
      init_data: expect.stringContaining("hash=00"),
      contact: expect.stringContaining("contact="),
    });
  });
});

test.describe("Referral invite link", () => {
  test("the profile shows and shares the t.me startapp link", async ({ page, context }) => {
    await context.addCookies([{ name: "at", value: "x", url: ORIGIN }]);
    const link = "https://t.me/DriverGouzBot?startapp=ref_REF-AB23CD";
    await page.route("**/api/proxy/**", (r) => {
      const url = r.request().url();
      if (url.endsWith("/api/proxy/me")) return r.fulfill({ json: meOk() });
      if (url.endsWith("/api/proxy/me/referral")) {
        return r.fulfill({
          json: {
            data: {
              referral_code: "REF-AB23CD",
              invite_url: link,
              total_invited: 0,
              total_rewarded: 0,
              earned_uzs: 0,
              available_balance_uzs: 0,
              commission_percent: 10,
            },
          },
        });
      }
      return r.fulfill({ json: { data: [] } });
    });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/uz-Latn/profile");
    await expect(page.getByText(link)).toBeVisible();
    const popupPromise = context.waitForEvent("page");
    await stubTelegram(context);
    await page.getByRole("button", { name: "Telegram'da ulashish" }).click();
    const popup = await popupPromise;
    await expect.poll(() => new URL(popup.url()).searchParams.get("url")).toBe(link);
  });

  test("a Telegram-created account gets «Parol o'rnatish» instead of the change form", async ({ page, context }) => {
    await context.addCookies([{ name: "at", value: "x", url: ORIGIN }]);
    await page.route("**/api/proxy/**", (r) =>
      r.fulfill({ json: r.request().url().endsWith("/api/proxy/me") ? meOk({ has_password: false }) : { data: [] } })
    );
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/uz-Latn/profile");
    await expect(page.getByRole("heading", { name: "Parol o'rnatish" })).toBeVisible();
    await expect(page.getByLabel("Joriy parol")).toHaveCount(0);
  });
});

import { test, expect, type Page } from "@playwright/test";

// No backend: every /api/* call is stubbed. The real Telegram SDK is never
// fetched; a fake window.Telegram.WebApp stands in for it.

const SDK_URL = "https://telegram.org/js/telegram-web-app.js";

// Non-zero, so a rule that ignores Telegram's insets actually fails.
const SAFE_TOP = 24;
const SAFE_BOTTOM = 18;

const fakeTelegram = (opts: { autologinOff?: boolean; colorScheme?: "light" | "dark" }) => `
  (() => {
    const store = ${opts.autologinOff ? `{ autologin_off: "1" }` : `{}`};
    window.__tg = { calls: [] };
    // What telegram-web-app.js publishes on <html> for the device safe area.
    // Init scripts can run before <html> exists.
    const setInsets = () => {
      document.documentElement.style.setProperty("--tg-safe-area-inset-top", "${SAFE_TOP}px");
      document.documentElement.style.setProperty("--tg-safe-area-inset-bottom", "${SAFE_BOTTOM}px");
    };
    if (document.documentElement) setInsets();
    else document.addEventListener("DOMContentLoaded", setInsets);
    const rec = (name) => (...args) => window.__tg.calls.push([name, ...args]);
    // The bridge a real Telegram client injects: without a host the app
    // ignores launch data and the SDK object entirely.
    window.TelegramWebviewProxy = { postEvent: rec("proxy") };
    sessionStorage.setItem("tg-webapp", "1");
    window.Telegram = { WebApp: {
      initData: "query_id=x&user=%7B%22id%22%3A1%7D&auth_date=1&hash=00",
      initDataUnsafe: { user: { id: 1, first_name: "Ali", language_code: "uz" } },
      colorScheme: "${opts.colorScheme ?? "dark"}", version: "8.0", platform: "android",
      ready: rec("ready"), expand: rec("expand"), close: rec("close"), isVersionAtLeast: () => true,
      disableVerticalSwipes: rec("disableVerticalSwipes"),
      enableClosingConfirmation: rec("enableClosingConfirmation"),
      disableClosingConfirmation: rec("disableClosingConfirmation"),
      setHeaderColor: rec("setHeaderColor"), setBackgroundColor: rec("setBackgroundColor"), setBottomBarColor: rec("setBottomBarColor"),
      onEvent: () => {}, offEvent: () => {},
      openLink: rec("openLink"), openTelegramLink: rec("openTelegramLink"),
      requestContact: (cb) => cb(true, {
        response: "contact=%7B%22user_id%22%3A1%2C%22phone_number%22%3A%22998901234567%22%7D&auth_date=1&hash=00",
        responseUnsafe: { contact: { phone_number: "998901234567" } },
      }),
      BackButton: { show: rec("back.show"), hide: rec("back.hide"), onClick: () => {}, offClick: () => {} },
      HapticFeedback: { impactOccurred: rec("impact"), notificationOccurred: rec("notify"), selectionChanged: rec("select") },
      CloudStorage: {
        getItem: (k, cb) => cb(null, store[k] || ""),
        setItem: (k, v, cb) => { store[k] = v; cb && cb(null, true); },
        removeItem: (k, cb) => { delete store[k]; cb && cb(null, true); },
      },
    } };
  })();
`;

async function openInFakeTelegram(page: Page, opts: { autologinOff?: boolean; colorScheme?: "light" | "dark" } = {}) {
  await page.route(SDK_URL, (r) => r.fulfill({ contentType: "text/javascript", body: "" }));
  await page.addInitScript(fakeTelegram(opts));
}

const tgCalls = (page: Page) =>
  page.evaluate(() => (window as unknown as { __tg: { calls: unknown[][] } }).__tg.calls.map((c) => c[0]));

const tgCallArgs = (page: Page, name: string) =>
  page.evaluate(
    (n) => (window as unknown as { __tg: { calls: unknown[][] } }).__tg.calls.filter((c) => c[0] === n).map((c) => c[1]),
    name,
  );

const BASE = "http://localhost:" + (process.env.PORT || 3000);

const meOk = { data: { profile: { must_change_password: false, display_name: "Ali" }, vip: { active: false, until: null } } };
const unauthorized = { error: { code: "unauthorized" } };
const needPhone = { data: { need_phone: true, first_name: "Ali" } };

test.describe("Telegram Mini App", () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test("linked user with a live session goes straight to the dashboard without signing in", async ({ page, context }) => {
    await openInFakeTelegram(page);
    // Middleware only checks cookie presence for protected pages.
    await context.addCookies([{ name: "at", value: "x", url: "http://localhost:" + (process.env.PORT || 3000) }]);
    let signIns = 0;
    await page.route("**/api/auth/telegram", (r) => {
      signIns++;
      return r.fulfill({ json: needPhone });
    });
    await page.route("**/api/proxy/**", (r) =>
      r.fulfill({ json: r.request().url().endsWith("/api/proxy/me") ? meOk : { data: [] } })
    );
    await page.goto("/uz-Latn/tg");
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/);
    expect(signIns).toBe(0);
  });

  test("unlinked user: welcome, phone login with Telegram's number, tg_init_data is sent", async ({ page }) => {
    await openInFakeTelegram(page);
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    let loginBody: Record<string, unknown> | null = null;
    await page.route("**/api/auth/login", (r) => {
      loginBody = r.request().postDataJSON();
      return r.fulfill({ status: 401, json: { error: { code: "invalid_credentials" } } });
    });
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    await page.getByRole("link", { name: "Kirish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/login/);
    await page.getByRole("button", { name: "Telegram raqamini olish" }).click();
    await expect(page.locator('input[type="tel"], input[inputmode="tel"]').first()).toHaveValue("90 123 45 67");
    await page.locator('input[type="password"]').fill("secret123");
    await page.locator("form button[type=submit]").click();
    await expect.poll(() => loginBody).not.toBeNull();
    expect(loginBody).toMatchObject({
      tg_init_data: expect.stringContaining("hash=00"),
      tg_contact: expect.stringContaining("contact="),
    });
  });

  test("stranger's session is ended before the welcome, so Kirish reaches /login", async ({ page, context }) => {
    await openInFakeTelegram(page);
    await context.addCookies([{ name: "at", value: "x", url: "http://localhost:" + (process.env.PORT || 3000) }]);
    let loggedOut = 0;
    await page.route("**/api/auth/logout", async (r) => {
      loggedOut++;
      await context.clearCookies();
      return r.fulfill({ json: { data: { ok: true } } });
    });
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    await page.route("**/api/proxy/**", (r) => {
      const url = r.request().url();
      if (url.endsWith("/api/proxy/me")) return r.fulfill({ json: meOk });
      if (url.endsWith("/api/proxy/me/telegram")) {
        return r.fulfill({ json: { data: { linked: true, username: "stranger", tg_user_id: 999 } } });
      }
      return r.fulfill({ json: { data: [] } });
    });
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    expect(loggedOut).toBe(1);
    await page.getByRole("link", { name: "Kirish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/login/);
  });

  test("linked user without a session signs in silently with the launch data", async ({ page, context }) => {
    await openInFakeTelegram(page);
    let meCalls = 0;
    await page.route("**/api/proxy/**", (r) => {
      if (!r.request().url().endsWith("/api/proxy/me")) return r.fulfill({ json: { data: [] } });
      // First probe: no session yet; after sign-in: the cookie works.
      return meCalls++ === 0 ? r.fulfill({ status: 401, json: unauthorized }) : r.fulfill({ json: meOk });
    });
    let signInBody: Record<string, unknown> | null = null;
    await page.route("**/api/auth/telegram", async (r) => {
      signInBody = r.request().postDataJSON();
      await context.addCookies([{ name: "at", value: "x", url: "http://localhost:" + (process.env.PORT || 3000) }]);
      return r.fulfill({ json: { data: { ok: true, must_change_password: false } } });
    });
    await page.goto("/uz-Latn/tg");
    await expect(page).toHaveURL(/\/uz-Latn\/dashboard/);
    expect(signInBody).toEqual({ init_data: expect.stringContaining("hash=00") });
  });

  test("autologin_off: offers 'continue as' and does not sign in until tapped", async ({ page }) => {
    await openInFakeTelegram(page, { autologinOff: true });
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    let signIns = 0;
    let signInBody: Record<string, unknown> | null = null;
    await page.route("**/api/auth/telegram", (r) => {
      signIns++;
      signInBody = r.request().postDataJSON();
      return r.fulfill({ json: needPhone });
    });
    await page.goto("/uz-Latn/tg");
    const button = page.getByRole("button", { name: /Ali sifatida davom etish/ });
    await expect(button).toBeVisible();
    expect(signIns).toBe(0);
    await button.click();
    await expect.poll(() => signIns).toBe(1);
    expect(signInBody).toEqual({ init_data: expect.stringContaining("hash=00") });
  });

  test("invalid_init_data inside Telegram offers 'Botga qaytish' which closes the Mini App", async ({ page }) => {
    await openInFakeTelegram(page);
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.route("**/api/auth/telegram", (r) =>
      r.fulfill({ status: 401, json: { error: { code: "invalid_init_data" } } })
    );
    await page.goto("/uz-Latn/tg");
    await page.getByRole("button", { name: "Botga qaytish" }).click();
    expect(await tgCalls(page)).toContain("close");
  });

  // C1: an attacker sends a link carrying THEIR fresh launch data. Opened in
  // a plain browser it must stay the website: no SDK, no Telegram button,
  // and the victim's phone sign-in carries no Telegram data to link.
  test("planted #tgWebAppData in a plain browser is ignored by login and /tg", async ({ page }) => {
    const sdk: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("telegram-web-app.js")) sdk.push(r.url());
    });
    const planted =
      "#tgWebAppData=" +
      encodeURIComponent("query_id=x&user=%7B%22id%22%3A7001%7D&auth_date=1&hash=00") +
      "&tgWebAppVersion=8.0&tgWebAppPlatform=weba";
    let loginBody: Record<string, unknown> | null = null;
    await page.route("**/api/auth/login", (r) => {
      loginBody = r.request().postDataJSON();
      return r.fulfill({ status: 401, json: { error: { code: "invalid_credentials" } } });
    });
    let telegramSignIns = 0;
    await page.route("**/api/auth/telegram", (r) => {
      telegramSignIns++;
      return r.fulfill({ json: { data: { ok: true, must_change_password: false } } });
    });

    await page.goto("/uz-Latn/login" + planted);
    await page.waitForLoadState("networkidle");
    await expect(page.getByRole("button", { name: "Telegram raqamini olish" })).toHaveCount(0);
    await page.locator('input[type="tel"], input[inputmode="tel"]').first().fill("901234567");
    await page.locator('input[type="password"]').fill("victim-password");
    await page.locator("form button[type=submit]").click();
    await expect.poll(() => loginBody).not.toBeNull();
    expect(loginBody).not.toHaveProperty("tg_init_data");
    expect(loginBody).not.toHaveProperty("tg_contact");

    await page.goto("/uz-Latn/tg" + planted);
    await expect(page.getByRole("heading", { name: "Botdan oching" })).toBeVisible();
    expect(telegramSignIns).toBe(0);
    expect(sdk).toEqual([]);
    expect(await page.evaluate(() => sessionStorage.getItem("tg-webapp"))).toBeNull();
  });

  test("website visit never loads the SDK and shows no Telegram UI", async ({ page }) => {
    const sdk: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("telegram-web-app.js")) sdk.push(r.url());
    });
    await page.goto("/uz-Latn/login");
    await page.waitForLoadState("networkidle");
    expect(sdk).toEqual([]);
    await expect(page.getByRole("button", { name: "Telegram raqamini olish" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /mavzu|theme|tema/i }).first()).toBeVisible();
  });

  test("framing headers: learner pages allow Telegram Web, admin never", async ({ request }) => {
    const learner = await request.get("/uz-Latn/login");
    expect(learner.headers()["content-security-policy"]).toContain("frame-ancestors 'self' https://web.telegram.org");
    expect(learner.headers()["x-frame-options"]).toBeUndefined();
    const admin = await request.get("/uz-Latn/admin/login");
    expect(admin.headers()["x-frame-options"]).toBe("DENY");
    expect(admin.headers()["content-security-policy"]).toContain("frame-ancestors 'none'");
  });

  test("payment return page is public and links back to the bot", async ({ page }) => {
    await page.goto("/uz-Latn/checkout/done/DriverGouzBot");
    await expect(page).toHaveURL(/\/checkout\/done\/DriverGouzBot$/);
    // playwright.config sets TELEGRAM_BOT_USERNAME only for a server it
    // starts; a reused local dev server may run without it. CI never reuses.
    const configured = await page.locator('a[href="https://t.me/DriverGouzBot"]').count();
    test.skip(
      !process.env.CI && configured === 0,
      "reused dev server without TELEGRAM_BOT_USERNAME=DriverGouzBot; restart it or run with CI=true",
    );
    await expect(page.locator('a[href="https://t.me/DriverGouzBot"]')).toBeVisible();
    // Someone else's bot in the path: the page stays text-only.
    await page.goto("/uz-Latn/checkout/done/Evil_payment_bot");
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator('a[href^="https://t.me/"]')).toHaveCount(0);
    await page.goto("/uz-Latn/checkout/done/a/b");
    await expect(page).toHaveURL(/login/);
  });

  test("dashboard inside the Mini App: no horizontal scroll, bottom nav fully visible", async ({ page, context }) => {
    await openInFakeTelegram(page);
    await context.addCookies([{ name: "at", value: "x", url: "http://localhost:" + (process.env.PORT || 3000) }]);
    await page.route("**/api/proxy/**", (r) =>
      r.fulfill({ json: r.request().url().endsWith("/api/proxy/me") ? meOk : { data: [] } })
    );
    await page.goto("/uz-Latn/dashboard");
    const nav = page.locator("nav.app-bottom-nav");
    await expect(nav).toBeVisible();
    const box = await nav.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.y + box!.height).toBeLessThanOrEqual(844 + 0.5);
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390 + 0.5);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth
    );
    expect(overflow).toBeLessThanOrEqual(0);
    // Telegram's safe area is honoured, not just env() (0 in this browser).
    // The tg-webapp class arrives with the SDK, after the first paint.
    await expect(page.locator("html.tg-webapp")).toHaveCount(1);
    const pads = await page.evaluate(() => ({
      top: parseFloat(getComputedStyle(document.querySelector(".app-top-bar")!).paddingTop),
      bottom: parseFloat(getComputedStyle(document.querySelector("nav.app-bottom-nav")!).paddingBottom),
    }));
    expect(pads.top).toBeGreaterThanOrEqual(SAFE_TOP);
    expect(pads.bottom).toBeGreaterThanOrEqual(SAFE_BOTTOM);
  });

  // Audit-2 I2: each Back to /tg used to cost another sign-in call.
  test("Back from login to the welcome does not sign in again", async ({ page }) => {
    await openInFakeTelegram(page);
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    let signIns = 0;
    await page.route("**/api/auth/telegram", (r) => {
      signIns++;
      return r.fulfill({ json: needPhone });
    });
    await page.goto("/uz-Latn/tg");
    for (let i = 0; i < 3; i++) {
      await page.getByRole("link", { name: "Kirish" }).click();
      await expect(page).toHaveURL(/\/uz-Latn\/login/);
      await page.goBack();
      await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    }
    expect(signIns).toBe(1);
  });

  // Audit-2 I1/I3: the form stays inside the app, and a deep link's target
  // survives the phone sign-in.
  test("deep link next survives the welcome and the phone login", async ({ page, context }) => {
    await openInFakeTelegram(page);
    let signedIn = false;
    await page.route("**/api/proxy/**", (r) => {
      const url = r.request().url();
      if (url.endsWith("/api/proxy/me")) {
        return signedIn ? r.fulfill({ json: meOk }) : r.fulfill({ status: 401, json: unauthorized });
      }
      return r.fulfill({ json: { data: [] } });
    });
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    await page.route("**/api/auth/login", async (r) => {
      signedIn = true;
      await context.addCookies([{ name: "at", value: "x", url: BASE }]);
      return r.fulfill({ json: { data: { ok: true, telegram_linked: true } } });
    });
    await page.goto("/uz-Latn/tg?next=%2Fuz-Latn%2Fsigns");
    await page.getByRole("link", { name: "Kirish" }).click();
    await expect(page).toHaveURL(/\/uz-Latn\/login\?next=%2Fuz-Latn%2Fsigns/);
    await expect(page.getByRole("link", { name: /Bosh sahifaga qaytish/ })).toHaveCount(0);
    await expect(page.locator(`header a[href="/uz-Latn"]`)).toHaveCount(0);
    await page.locator('input[type="tel"]').fill("901234567");
    await page.locator('input[type="password"]').fill("secret123");
    await page.locator("form button[type=submit]").click();
    await expect(page).toHaveURL(/\/uz-Latn\/signs$/);
  });

  // Audit-2 I9: a light Telegram must not get a dark first frame or a dark
  // bottom bar while the SDK and React catch up.
  test("light Telegram theme: light from the first paint, never a dark frame colour", async ({ page }) => {
    await openInFakeTelegram(page, { colorScheme: "light" });
    await page.route("**/api/proxy/me", (r) => r.fulfill({ status: 401, json: unauthorized }));
    await page.route("**/api/auth/telegram", (r) => r.fulfill({ json: needPhone }));
    const launch =
      "#tgWebAppData=" +
      encodeURIComponent("query_id=x&user=%7B%22id%22%3A1%7D&auth_date=1&hash=00") +
      "&tgWebAppThemeParams=" +
      encodeURIComponent(JSON.stringify({ bg_color: "#ffffff" }));
    // Record the <html> class as soon as the body starts parsing.
    await page.addInitScript(() => {
      document.addEventListener("DOMContentLoaded", () => {
        (window as unknown as { __firstClass: string }).__firstClass = document.documentElement.className;
      });
    });
    await page.goto("/uz-Latn/tg" + launch);
    await expect(page.getByRole("heading", { name: /Ali/ })).toBeVisible();
    expect(await page.evaluate(() => (window as unknown as { __firstClass: string }).__firstClass)).toContain("light");
    // The chrome is a lazy chunk and paints a frame after it mounts.
    await expect.poll(async () => (await tgCallArgs(page, "setBottomBarColor")).length).toBeGreaterThan(0);
    const bars = (await tgCallArgs(page, "setBottomBarColor")) as string[];
    for (const color of bars) expect(color).toBe("#f3f4f6");
  });

  test("website /tg is not a dead end: the bot and a website login", async ({ page }) => {
    await page.goto("/uz-Latn/tg");
    await expect(page.getByRole("heading", { name: "Botdan oching" })).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole("link", { name: "Saytda kirish" })).toHaveAttribute("href", "/uz-Latn/login");
    const bot = await page.getByRole("link", { name: "Botni ochish" }).count();
    test.skip(
      !process.env.CI && bot === 0,
      "reused dev server without TELEGRAM_BOT_USERNAME=DriverGouzBot; restart it or run with CI=true",
    );
    await expect(page.getByRole("link", { name: "Botni ochish" })).toHaveAttribute("href", "https://t.me/DriverGouzBot");
  });
});

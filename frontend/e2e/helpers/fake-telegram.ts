import type { Page } from "@playwright/test";

// A fake Telegram client for Mini App specs. No backend and no real SDK:
// telegram-web-app.js is stubbed empty and window.Telegram.WebApp stands in.

export const SDK_URL = "https://telegram.org/js/telegram-web-app.js";

export interface FakeTelegramOptions {
  autologinOff?: boolean;
  colorScheme?: "light" | "dark";
  platform?: string;
  /** --tg-safe-area-inset-* as Telegram publishes them on <html>. */
  safeTop?: number;
  safeBottom?: number;
  /**
   * --tg-content-safe-area-inset-top: the strip Telegram's own controls
   * cover. A real client publishes it only once fullscreen is on, so the
   * fake does the same from requestFullscreen().
   */
  fullscreenContentTop?: number;
}

const script = (o: Required<FakeTelegramOptions>) => `
  (() => {
    const store = ${o.autologinOff ? `{ autologin_off: "1" }` : `{}`};
    window.__tg = { calls: [], back: [], events: {} };
    const css = (name, px) => document.documentElement.style.setProperty(name, px + "px");
    // Init scripts can run before <html> exists.
    const setInsets = () => {
      css("--tg-safe-area-inset-top", ${o.safeTop});
      css("--tg-safe-area-inset-bottom", ${o.safeBottom});
    };
    if (document.documentElement) setInsets();
    else document.addEventListener("DOMContentLoaded", setInsets);
    const rec = (name) => (...args) => window.__tg.calls.push([name, ...args]);
    const fire = (name) => (window.__tg.events[name] || []).forEach((cb) => cb());
    // The bridge a real Telegram client injects: without a host the app
    // ignores launch data and the SDK object entirely.
    window.TelegramWebviewProxy = { postEvent: rec("proxy") };
    sessionStorage.setItem("tg-webapp", "1");
    const webApp = {
      initData: "query_id=x&user=%7B%22id%22%3A1%7D&auth_date=1&hash=00",
      initDataUnsafe: { user: { id: 1, first_name: "Ali", language_code: "uz" } },
      colorScheme: "${o.colorScheme}", version: "8.0", platform: "${o.platform}",
      isFullscreen: false,
      ready: rec("ready"), expand: rec("expand"), close: rec("close"), isVersionAtLeast: () => true,
      disableVerticalSwipes: rec("disableVerticalSwipes"),
      requestFullscreen: (...args) => {
        rec("requestFullscreen")(...args);
        ${o.fullscreenContentTop > 0 ? `
        webApp.isFullscreen = true;
        css("--tg-content-safe-area-inset-top", ${o.fullscreenContentTop});
        fire("fullscreenChanged");` : ""}
      },
      enableClosingConfirmation: rec("enableClosingConfirmation"),
      disableClosingConfirmation: rec("disableClosingConfirmation"),
      setHeaderColor: rec("setHeaderColor"), setBackgroundColor: rec("setBackgroundColor"), setBottomBarColor: rec("setBottomBarColor"),
      onEvent: (name, cb) => { (window.__tg.events[name] = window.__tg.events[name] || []).push(cb); },
      offEvent: (name, cb) => { window.__tg.events[name] = (window.__tg.events[name] || []).filter((x) => x !== cb); },
      openLink: rec("openLink"), openTelegramLink: rec("openTelegramLink"),
      requestContact: (cb) => cb(true, {
        response: "contact=%7B%22user_id%22%3A1%2C%22phone_number%22%3A%22998901234567%22%7D&auth_date=1&hash=00",
        responseUnsafe: { contact: { phone_number: "998901234567" } },
      }),
      BackButton: {
        show: rec("back.show"), hide: rec("back.hide"),
        onClick: (cb) => window.__tg.back.push(cb),
        offClick: (cb) => { window.__tg.back = window.__tg.back.filter((x) => x !== cb); },
      },
      HapticFeedback: { impactOccurred: rec("impact"), notificationOccurred: rec("notify"), selectionChanged: rec("select") },
      CloudStorage: {
        getItem: (k, cb) => cb(null, store[k] || ""),
        setItem: (k, v, cb) => { store[k] = v; cb && cb(null, true); },
        removeItem: (k, cb) => { delete store[k]; cb && cb(null, true); },
      },
    };
    window.Telegram = { WebApp: webApp };
  })();
`;

export async function openInFakeTelegram(page: Page, opts: FakeTelegramOptions = {}) {
  await page.route(SDK_URL, (r) => r.fulfill({ contentType: "text/javascript", body: "" }));
  await page.addInitScript(
    script({
      autologinOff: false,
      colorScheme: "dark",
      platform: "android",
      safeTop: 24,
      safeBottom: 18,
      fullscreenContentTop: 0,
      ...opts,
    })
  );
}

type TgWindow = { __tg: { calls: unknown[][]; back: (() => void)[] } };

export const tgCalls = (page: Page) =>
  page.evaluate(() => (window as unknown as TgWindow).__tg.calls.map((c) => c[0]));

export const tgCallArgs = (page: Page, name: string) =>
  page.evaluate((n) => (window as unknown as TgWindow).__tg.calls.filter((c) => c[0] === n).map((c) => c[1]), name);

/** Taps Telegram's BackButton: runs the handlers the app registered. */
export const tapTelegramBack = (page: Page) =>
  page.evaluate(() => (window as unknown as TgWindow).__tg.back.forEach((cb) => cb()));

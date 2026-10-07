import { afterEach, describe, expect, it, vi } from "vitest";
import { markTelegramLogout, postLogoutPath } from "./logout";
import { openExternalUrl, telegramLinkKind } from "./links";
import { AUTOLOGIN_OFF_KEY, markTelegramMiniApp } from "./web-app";

afterEach(() => {
  sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
  vi.restoreAllMocks();
  vi.useRealTimers();
});

function fakeWebApp(extra: Record<string, unknown> = {}) {
  const webApp = { initData: "x", openLink: vi.fn(), openTelegramLink: vi.fn(), ...extra };
  (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
  return webApp;
}

describe("telegram logout helpers", () => {
  it("postLogoutPath keeps the website fallback outside Telegram", () => {
    expect(postLogoutPath("ru", "/ru/login")).toBe("/ru/login");
  });
  it("postLogoutPath lands on /tg inside the Mini App", () => {
    markTelegramMiniApp();
    expect(postLogoutPath("ru", "/ru/login")).toBe("/ru/tg");
  });
  it("markTelegramLogout writes autologin_off", async () => {
    const setItem = vi.fn((_k: string, _v: string, cb?: (err: string | null) => void) => cb?.(null));
    fakeWebApp({ CloudStorage: { setItem, getItem: vi.fn(), removeItem: vi.fn() } });
    await markTelegramLogout();
    expect(setItem).toHaveBeenCalledWith(AUTOLOGIN_OFF_KEY, "1", expect.any(Function));
  });
  it("markTelegramLogout never blocks logout on a silent bridge", async () => {
    vi.useFakeTimers();
    fakeWebApp({ CloudStorage: { setItem: vi.fn(), getItem: vi.fn(), removeItem: vi.fn() } });
    const done = vi.fn();
    void markTelegramLogout().then(done);
    await vi.advanceTimersByTimeAsync(3000);
    expect(done).toHaveBeenCalled();
  });
  it("markTelegramLogout is a no-op on the website", async () => {
    await expect(markTelegramLogout()).resolves.toBeUndefined();
  });
});

describe("telegramLinkKind", () => {
  const here = "https://drivergo.uz/uz-Latn/dashboard";
  it.each([
    ["https://payme.uz/x", "external"],
    ["http://example.com", "external"],
    ["https://t.me/drivergo_bot?start=abc", "telegram"],
    ["https://telegram.me/x", "telegram"],
    ["https://drivergo.uz/uz-Latn/signs", null],
    ["/uz-Latn/signs", null],
    ["mailto:a@b.c", null],
    ["tel:+998901234567", null],
    ["tg://resolve?domain=x", null],
    ["javascript:alert(1)", null],
    ["not a url ::", null],
  ])("%s → %s", (href, kind) => expect(telegramLinkKind(href, here)).toBe(kind));
});

describe("openExternalUrl", () => {
  it("uses window.open on the website", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    openExternalUrl("https://t.me/x");
    expect(open).toHaveBeenCalledWith("https://t.me/x", "_blank", "noopener,noreferrer");
  });
  it("hands t.me to Telegram and everything else to the browser inside the Mini App", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const webApp = fakeWebApp();
    openExternalUrl("https://t.me/x");
    openExternalUrl("https://checkout.paycom.uz/abc");
    expect(webApp.openTelegramLink).toHaveBeenCalledWith("https://t.me/x");
    expect(webApp.openLink).toHaveBeenCalledWith("https://checkout.paycom.uz/abc");
    expect(open).not.toHaveBeenCalled();
  });
});

import { afterEach, describe, expect, it, vi } from "vitest";
import { cloudGet, cloudGetResult, getWebApp, isTelegramMiniApp, markTelegramMiniApp, TG_SESSION_FLAG } from "./web-app";
import { haptics } from "./haptics";

afterEach(() => {
  sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
  window.history.replaceState(null, "", "/");
});

describe("telegram detection", () => {
  it("is false on the plain website", () => {
    expect(isTelegramMiniApp()).toBe(false);
    expect(getWebApp()).toBeNull();
  });
  it("is true after the launch hash or the session flag", () => {
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=x&tgWebAppVersion=8.0");
    expect(isTelegramMiniApp()).toBe(true);
    window.history.replaceState(null, "", "/");
    markTelegramMiniApp();
    expect(sessionStorage.getItem(TG_SESSION_FLAG)).toBe("1");
    expect(isTelegramMiniApp()).toBe(true);
  });
  it("getWebApp ignores an SDK without initData (opened outside Telegram)", () => {
    (window as { Telegram?: unknown }).Telegram = { WebApp: { initData: "" } };
    expect(getWebApp()).toBeNull();
  });
  it("cloudGet resolves null when CloudStorage is missing", async () => {
    expect(await cloudGet("k")).toBeNull();
  });
  it("cloudGet gives up on a CloudStorage that never answers", async () => {
    vi.useFakeTimers();
    try {
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { getItem: () => {} } },
      };
      const pending = cloudGet("k");
      await vi.advanceTimersByTimeAsync(3000);
      expect(await pending).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
  it("cloudGetResult is ok/null without CloudStorage, ok/value when it answers", async () => {
    expect(await cloudGetResult("k")).toEqual({ status: "ok", value: null });
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: { initData: "x", CloudStorage: { getItem: (_k: string, cb: (e: null, v: string) => void) => cb(null, "1") } },
    };
    expect(await cloudGetResult("k")).toEqual({ status: "ok", value: "1" });
  });
  it("cloudGetResult reports unavailable on timeout and on error", async () => {
    vi.useFakeTimers();
    try {
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { getItem: () => {} } },
      };
      const pending = cloudGetResult("k");
      await vi.advanceTimersByTimeAsync(3000);
      expect(await pending).toEqual({ status: "unavailable" });
    } finally {
      vi.useRealTimers();
    }
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: { initData: "x", CloudStorage: { getItem: (_k: string, cb: (e: string) => void) => cb("ERR") } },
    };
    expect(await cloudGetResult("k")).toEqual({ status: "unavailable" });
  });
  it("haptics are silent no-ops outside Telegram", () => {
    expect(() => {
      haptics.select();
      haptics.result(true);
      haptics.impact();
    }).not.toThrow();
  });
  it("haptics swallow a throwing or missing HapticFeedback on old clients", () => {
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: {
        initData: "x",
        HapticFeedback: {
          notificationOccurred: () => {
            throw new Error("WebAppMethodUnsupported");
          },
        },
      },
    };
    expect(() => {
      haptics.result(false);
      haptics.select();
    }).not.toThrow();
  });
});

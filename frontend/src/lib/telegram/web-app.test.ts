import { afterEach, describe, expect, it, vi } from "vitest";
import { cloudGet, cloudGetResult, cloudRemove, cloudSet, getWebApp, hasTelegramHost, isTelegramMiniApp, markTelegramMiniApp, TG_SESSION_FLAG } from "./web-app";
import { haptics } from "./haptics";
import { installTelegramHost, removeTelegramHost } from "@/test/telegram-host";

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
  it("is true after the launch hash or the session flag inside a Telegram host", () => {
    installTelegramHost();
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=x&tgWebAppVersion=8.0");
    expect(isTelegramMiniApp()).toBe(true);
    window.history.replaceState(null, "", "/");
    markTelegramMiniApp();
    expect(sessionStorage.getItem(TG_SESSION_FLAG)).toBe("1");
    expect(isTelegramMiniApp()).toBe(true);
  });
  // C1: anyone can put their own launch data in a link's hash. In a plain
  // browser Telegram's SDK still reads it into initData, so the hash and the
  // SDK object prove nothing without a Telegram client around the page.
  it("ignores injected launch data, the session flag and the SDK object without a host", () => {
    window.history.replaceState(null, "", "/uz-Latn/login#tgWebAppData=attacker&tgWebAppVersion=8.0");
    expect(isTelegramMiniApp()).toBe(false);
    markTelegramMiniApp();
    expect(sessionStorage.getItem(TG_SESSION_FLAG)).toBeNull();
    sessionStorage.setItem(TG_SESSION_FLAG, "1");
    expect(isTelegramMiniApp()).toBe(false);
    (window as { Telegram?: unknown }).Telegram = { WebApp: { initData: "attacker" } };
    expect(getWebApp()).toBeNull();
  });
  it("recognises each host the SDK itself talks to", () => {
    expect(hasTelegramHost()).toBe(false);
    installTelegramHost(); // Android, iOS, new Desktop
    expect(hasTelegramHost()).toBe(true);
    removeTelegramHost();
    const external = Object.getOwnPropertyDescriptor(window, "external");
    Object.defineProperty(window, "external", { value: { notify: () => {} }, configurable: true }); // legacy Desktop
    try {
      expect(hasTelegramHost()).toBe(true);
    } finally {
      if (external) Object.defineProperty(window, "external", external);
      else delete (window as { external?: unknown }).external;
    }
    expect(hasTelegramHost()).toBe(false);
    const parent = Object.getOwnPropertyDescriptor(window, "parent");
    Object.defineProperty(window, "parent", { value: {}, configurable: true }); // web.telegram.org iframe
    try {
      expect(hasTelegramHost()).toBe(true);
    } finally {
      if (parent) Object.defineProperty(window, "parent", parent);
    }
    expect(hasTelegramHost()).toBe(false);
  });
  it("getWebApp ignores an SDK without initData (opened outside Telegram)", () => {
    installTelegramHost();
    (window as { Telegram?: unknown }).Telegram = { WebApp: { initData: "" } };
    expect(getWebApp()).toBeNull();
  });
  it("cloudGet resolves null when CloudStorage is missing", async () => {
    expect(await cloudGet("k")).toBeNull();
  });
  it("cloudGet gives up on a CloudStorage that never answers", async () => {
    vi.useFakeTimers();
    try {
      installTelegramHost();
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
    installTelegramHost();
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: { initData: "x", CloudStorage: { getItem: (_k: string, cb: (e: null, v: string) => void) => cb(null, "1") } },
    };
    expect(await cloudGetResult("k")).toEqual({ status: "ok", value: "1" });
  });
  it("cloudGetResult reports unavailable on timeout and on error", async () => {
    vi.useFakeTimers();
    try {
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { getItem: () => {} } },
      };
      const pending = cloudGetResult("k");
      await vi.advanceTimersByTimeAsync(3000);
      expect(await pending).toEqual({ status: "unavailable" });
    } finally {
      vi.useRealTimers();
    }
    installTelegramHost();
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
    installTelegramHost();
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
  it("cloudSet times out after 3s if the bridge never calls back", async () => {
    vi.useFakeTimers();
    try {
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { setItem: () => {} } },
      };
      const pending = cloudSet("k", "v");
      await vi.advanceTimersByTimeAsync(2999);
      // Still pending
      let settled = false;
      pending.then(() => {
        settled = true;
      });
      await vi.advanceTimersByTimeAsync(1);
      expect(settled).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
  it("cloudSet resolves immediately if the bridge calls back", async () => {
    installTelegramHost();
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: { initData: "x", CloudStorage: { setItem: (_k: string, _v: string, cb?: (e: null) => void) => cb?.(null) } },
    };
    await expect(cloudSet("k", "v")).resolves.toBeUndefined();
  });
  it("cloudRemove times out after 3s if the bridge never calls back", async () => {
    vi.useFakeTimers();
    try {
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { removeItem: () => {} } },
      };
      const pending = cloudRemove("k");
      await vi.advanceTimersByTimeAsync(2999);
      // Still pending
      let settled = false;
      pending.then(() => {
        settled = true;
      });
      await vi.advanceTimersByTimeAsync(1);
      expect(settled).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
  it("cloudRemove resolves immediately if the bridge calls back", async () => {
    installTelegramHost();
    (window as { Telegram?: unknown }).Telegram = {
      WebApp: { initData: "x", CloudStorage: { removeItem: (_k: string, cb?: (e: null) => void) => cb?.(null) } },
    };
    await expect(cloudRemove("k")).resolves.toBeUndefined();
  });
});

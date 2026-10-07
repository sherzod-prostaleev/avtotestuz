import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TelegramProvider, useTelegram, useTelegramColorScheme, useTelegramStatus } from "./telegram-provider";
import { TELEGRAM_SDK_URL, markTelegramMiniApp } from "@/lib/telegram/web-app";
import { installTelegramHost } from "@/test/telegram-host";

// The chrome has its own tests; here only WHEN it mounts matters.
vi.mock("./telegram-chrome", () => ({
  TelegramChrome: () => <span data-testid="tg-chrome" />,
}));

afterEach(() => {
  vi.useRealTimers();
  delete window.Telegram;
  sessionStorage.clear();
  document.head.querySelectorAll("script").forEach((s) => s.remove());
});

describe("TelegramProvider", () => {
  it("adds no script and renders children on the website", () => {
    render(<TelegramProvider><p>site</p></TelegramProvider>);
    expect(screen.getByText("site")).toBeInTheDocument();
    expect(document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)).toBeNull();
  });
  // C1: a planted #tgWebAppData link (or a leftover session flag) in a plain
  // browser must not load Telegram's SDK — it would read the planted data
  // into initData and the forms would send it.
  it("requests no SDK for injected launch data or a stale flag without a Telegram host", () => {
    window.history.replaceState(null, "", "/uz-Latn/login#tgWebAppData=attacker&tgWebAppVersion=8.0");
    sessionStorage.setItem("tg-webapp", "1");
    (window as { Telegram?: unknown }).Telegram = { WebApp: { initData: "attacker" } };
    function Probe() {
      return <span data-testid="probe">{useTelegram() === null ? "null" : "set"}:{useTelegramStatus()}</span>;
    }
    try {
      render(<TelegramProvider><Probe /></TelegramProvider>);
      expect(document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)).toBeNull();
      expect(screen.getByTestId("probe")).toHaveTextContent("null:off");
    } finally {
      window.history.replaceState(null, "", "/");
    }
  });
  it("injects the SDK once inside the Mini App", () => {
    installTelegramHost();
    markTelegramMiniApp();
    const { rerender } = render(<TelegramProvider><p>tg</p></TelegramProvider>);
    rerender(<TelegramProvider><p>tg</p></TelegramProvider>);
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
  });
  it("keeps rendering children when the SDK script fails to load", () => {
    installTelegramHost();
    markTelegramMiniApp();
    function Probe() {
      return <span data-testid="probe">{useTelegram() === null ? "null" : "set"}</span>;
    }
    render(<TelegramProvider><p>still here</p><Probe /></TelegramProvider>);
    const script = document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)!;
    script.dispatchEvent(new Event("error"));
    expect(screen.getByText("still here")).toBeInTheDocument();
    expect(screen.getByTestId("probe")).toHaveTextContent("null");
    expect(document.documentElement.classList.contains("tg-webapp")).toBe(false);
    expect(screen.queryByTestId("tg-chrome")).toBeNull();
  });
  it("mounts no Telegram chrome on the website", () => {
    render(<TelegramProvider><p>site</p></TelegramProvider>);
    expect(screen.queryByTestId("tg-chrome")).toBeNull();
  });

  describe("SDK status", () => {
    function StatusProbe() {
      return <span data-testid="status">{useTelegramStatus()}</span>;
    }
    const mount = () => render(<TelegramProvider><StatusProbe /></TelegramProvider>);
    const sdkScript = () => document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)!;
    const fakeSdk = (initData: string) => {
      installTelegramHost();
      window.Telegram = {
        WebApp: {
          initData,
          ready: vi.fn(),
          expand: vi.fn(),
          isVersionAtLeast: () => false,
          onEvent: vi.fn(),
          offEvent: vi.fn(),
        } as never,
      };
    };

    it("is off on the website", () => {
      mount();
      expect(screen.getByTestId("status")).toHaveTextContent("off");
    });
    it("is loading until the SDK script loads, then ready", () => {
      installTelegramHost();
      markTelegramMiniApp();
      mount();
      expect(screen.getByTestId("status")).toHaveTextContent("loading");
      fakeSdk("signed");
      act(() => {
        sdkScript().dispatchEvent(new Event("load"));
      });
      expect(screen.getByTestId("status")).toHaveTextContent("ready");
    });
    it("is ready at once when the SDK is already present", () => {
      installTelegramHost();
      markTelegramMiniApp();
      fakeSdk("signed");
      mount();
      expect(screen.getByTestId("status")).toHaveTextContent("ready");
      expect(screen.getByTestId("tg-chrome")).toBeInTheDocument();
    });
    it("fails when the script errors", () => {
      installTelegramHost();
      markTelegramMiniApp();
      mount();
      act(() => {
        sdkScript().dispatchEvent(new Event("error"));
      });
      expect(screen.getByTestId("status")).toHaveTextContent("failed");
    });
    it("fails when the SDK loads with empty initData", () => {
      installTelegramHost();
      markTelegramMiniApp();
      mount();
      fakeSdk("");
      act(() => {
        sdkScript().dispatchEvent(new Event("load"));
      });
      expect(screen.getByTestId("status")).toHaveTextContent("failed");
    });
    it("fails after 10 s of silence", () => {
      vi.useFakeTimers();
      installTelegramHost();
      markTelegramMiniApp();
      mount();
      act(() => {
        vi.advanceTimersByTime(9_999);
      });
      expect(screen.getByTestId("status")).toHaveTextContent("loading");
      act(() => {
        vi.advanceTimersByTime(1);
      });
      expect(screen.getByTestId("status")).toHaveTextContent("failed");
    });
    it("does not time out once ready", () => {
      vi.useFakeTimers();
      installTelegramHost();
      markTelegramMiniApp();
      mount();
      fakeSdk("signed");
      act(() => {
        sdkScript().dispatchEvent(new Event("load"));
      });
      act(() => {
        vi.advanceTimersByTime(20_000);
      });
      expect(screen.getByTestId("status")).toHaveTextContent("ready");
    });
  });

  describe("colour scheme", () => {
    type Handler = () => void;
    function sdk(colorScheme: string) {
      const handlers = new Set<Handler>();
      const webApp = {
        initData: "signed",
        colorScheme,
        ready: vi.fn(),
        expand: vi.fn(),
        isVersionAtLeast: () => false,
        onEvent: vi.fn((name: string, cb: Handler) => name === "themeChanged" && handlers.add(cb)),
        offEvent: vi.fn((name: string, cb: Handler) => name === "themeChanged" && handlers.delete(cb)),
      };
      installTelegramHost();
      window.Telegram = { WebApp: webApp as never };
      return { webApp, fire: () => handlers.forEach((cb) => cb()), handlers };
    }
    function SchemeProbe() {
      return <span data-testid="scheme">{useTelegramColorScheme() ?? "none"}</span>;
    }

    it("is null on the website", () => {
      render(<TelegramProvider><SchemeProbe /></TelegramProvider>);
      expect(screen.getByTestId("scheme")).toHaveTextContent("none");
    });

    it("follows Telegram's scheme at launch and on themeChanged, and unsubscribes", () => {
      installTelegramHost();
      markTelegramMiniApp();
      const tg = sdk("light");
      const view = render(<TelegramProvider><SchemeProbe /></TelegramProvider>);
      expect(screen.getByTestId("scheme")).toHaveTextContent("light");
      tg.webApp.colorScheme = "dark";
      act(() => tg.fire());
      expect(screen.getByTestId("scheme")).toHaveTextContent("dark");
      view.unmount();
      expect(tg.handlers.size).toBe(0);
    });

    it("ignores an unknown scheme", () => {
      installTelegramHost();
      markTelegramMiniApp();
      sdk("sepia");
      render(<TelegramProvider><SchemeProbe /></TelegramProvider>);
      expect(screen.getByTestId("scheme")).toHaveTextContent("none");
    });
  });
});

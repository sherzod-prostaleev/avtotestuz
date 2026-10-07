import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TelegramProvider, useTelegram, useTelegramStatus } from "./telegram-provider";
import { TELEGRAM_SDK_URL, markTelegramMiniApp } from "@/lib/telegram/web-app";

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
  it("injects the SDK once inside the Mini App", () => {
    markTelegramMiniApp();
    const { rerender } = render(<TelegramProvider><p>tg</p></TelegramProvider>);
    rerender(<TelegramProvider><p>tg</p></TelegramProvider>);
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
  });
  it("keeps rendering children when the SDK script fails to load", () => {
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
      window.Telegram = {
        WebApp: {
          initData,
          ready: vi.fn(),
          expand: vi.fn(),
          isVersionAtLeast: () => false,
        } as never,
      };
    };

    it("is off on the website", () => {
      mount();
      expect(screen.getByTestId("status")).toHaveTextContent("off");
    });
    it("is loading until the SDK script loads, then ready", () => {
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
      markTelegramMiniApp();
      fakeSdk("signed");
      mount();
      expect(screen.getByTestId("status")).toHaveTextContent("ready");
      expect(screen.getByTestId("tg-chrome")).toBeInTheDocument();
    });
    it("fails when the script errors", () => {
      markTelegramMiniApp();
      mount();
      act(() => {
        sdkScript().dispatchEvent(new Event("error"));
      });
      expect(screen.getByTestId("status")).toHaveTextContent("failed");
    });
    it("fails when the SDK loads with empty initData", () => {
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
});

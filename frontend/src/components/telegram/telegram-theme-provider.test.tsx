import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTheme } from "next-themes";
import { markTelegramMiniApp } from "@/lib/telegram/web-app";
import { TelegramProvider } from "./telegram-provider";
import { TelegramThemeProvider } from "./telegram-theme-provider";
import { installTelegramHost } from "@/test/telegram-host";

vi.mock("./telegram-chrome", () => ({ TelegramChrome: () => null }));

type Handler = () => void;

function sdk(colorScheme: "light" | "dark") {
  const handlers = new Set<Handler>();
  const webApp = {
    initData: "signed",
    colorScheme,
    ready: vi.fn(),
    expand: vi.fn(),
    isVersionAtLeast: () => false,
    onEvent: vi.fn((_: string, cb: Handler) => handlers.add(cb)),
    offEvent: vi.fn((_: string, cb: Handler) => handlers.delete(cb)),
  };
  installTelegramHost();
  window.Telegram = { WebApp: webApp as never };
  return { webApp, fire: () => handlers.forEach((cb) => cb()) };
}

function ThemeProbe() {
  const { forcedTheme } = useTheme();
  return <span data-testid="forced">{forcedTheme ?? "none"}</span>;
}

function mount() {
  return render(
    <TelegramProvider>
      <TelegramThemeProvider>
        <ThemeProbe />
      </TelegramThemeProvider>
    </TelegramProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.className = "";
});
afterEach(() => {
  delete window.Telegram;
  sessionStorage.clear();
  localStorage.clear();
});

describe("TelegramThemeProvider", () => {
  it("forces Telegram's scheme without ever writing the stored theme", () => {
    installTelegramHost();
    markTelegramMiniApp();
    const tg = sdk("light");
    mount();
    expect(screen.getByTestId("forced")).toHaveTextContent("light");
    expect(document.documentElement.classList.contains("light")).toBe(true);
    tg.webApp.colorScheme = "dark";
    act(() => tg.fire());
    expect(screen.getByTestId("forced")).toHaveTextContent("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(localStorage.getItem("theme")).toBeNull();
  });

  it("forces nothing on the website: the learner's saved theme applies", () => {
    localStorage.setItem("theme", "light");
    mount();
    expect(screen.getByTestId("forced")).toHaveTextContent("none");
    expect(document.documentElement.classList.contains("light")).toBe(true);
    expect(localStorage.getItem("theme")).toBe("light");
  });
});

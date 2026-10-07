import { act, render, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { useReportSessionRunning } from "@/lib/session-running";
import { TelegramChrome } from "./telegram-chrome";

const nav = vi.hoisted(() => ({
  pathname: "/uz-Latn/dashboard",
  back: vi.fn(),
  replace: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => nav.pathname,
  useRouter: () => ({ back: nav.back, replace: nav.replace, push: nav.push }),
}));

type Handler = () => void;

function fakeWebApp(overrides: Partial<TelegramWebApp> = {}) {
  const events = new Map<string, Set<Handler>>();
  const backHandlers = new Set<Handler>();
  const webApp = {
    initData: "x",
    colorScheme: "light",
    version: "8.0",
    platform: "android",
    isVersionAtLeast: (v: string) => Number.parseFloat(v) <= 8,
    setHeaderColor: vi.fn(),
    setBackgroundColor: vi.fn(),
    setBottomBarColor: vi.fn(),
    enableClosingConfirmation: vi.fn(),
    disableClosingConfirmation: vi.fn(),
    openLink: vi.fn(),
    openTelegramLink: vi.fn(),
    onEvent: vi.fn((name: string, cb: Handler) => {
      if (!events.has(name)) events.set(name, new Set());
      events.get(name)!.add(cb);
    }),
    offEvent: vi.fn((name: string, cb: Handler) => events.get(name)?.delete(cb)),
    BackButton: {
      show: vi.fn(),
      hide: vi.fn(),
      onClick: vi.fn((cb: Handler) => backHandlers.add(cb)),
      offClick: vi.fn((cb: Handler) => backHandlers.delete(cb)),
    },
    ...overrides,
  } as unknown as TelegramWebApp;
  return {
    webApp,
    fire: (name: string) => events.get(name)?.forEach((cb) => cb()),
    backHandlers,
  };
}

beforeEach(() => {
  nav.pathname = "/uz-Latn/dashboard";
  nav.back.mockReset();
  nav.replace.mockReset();
  window.history.replaceState(null, "", "/uz-Latn/dashboard");
  document.documentElement.style.setProperty("--background", "220 22% 7%");
});
afterEach(() => {
  document.documentElement.style.removeProperty("--background");
  document.body.innerHTML = "";
});

function mount(webApp: TelegramWebApp, colorScheme: "light" | "dark" = "dark") {
  const utils = render(<TelegramChrome webApp={webApp} colorScheme={colorScheme} />);
  return {
    ...utils,
    goTo(path: string) {
      nav.pathname = path;
      utils.rerender(<TelegramChrome webApp={webApp} colorScheme={colorScheme} />);
    },
    scheme(next: "light" | "dark") {
      utils.rerender(<TelegramChrome webApp={webApp} colorScheme={next} />);
    },
  };
}

/** A runner reporting its attempt as running; rerender with false = result screen. */
function runner(initial = true) {
  return renderHook(({ running }) => useReportSessionRunning(running), { initialProps: { running: initial } });
}

describe("TelegramChrome BackButton", () => {
  it("shows Back off the tab roots and goes back within the app", () => {
    const { webApp, backHandlers } = fakeWebApp();
    const view = mount(webApp);
    expect(webApp.BackButton.hide).toHaveBeenCalled();
    // An in-app navigation (through the history wrapper) gives Back somewhere to go.
    act(() => window.history.pushState({ __NA: true }, "", "/uz-Latn/signs"));
    view.goTo("/uz-Latn/signs");
    expect(webApp.BackButton.show).toHaveBeenCalled();
    expect(backHandlers.size).toBe(1);
    [...backHandlers][0]();
    expect(nav.back).toHaveBeenCalledTimes(1);
    expect(nav.replace).not.toHaveBeenCalled();
  });

  it("falls back to the dashboard when the screen was the launch entry", () => {
    nav.pathname = "/ru/signs";
    window.history.replaceState({ __NA: true }, "", "/ru/signs");
    const { webApp, backHandlers } = fakeWebApp();
    mount(webApp);
    [...backHandlers][0]();
    expect(nav.back).not.toHaveBeenCalled();
    expect(nav.replace).toHaveBeenCalledWith("/ru/dashboard");
  });

  it("keeps exactly one click handler across path changes and hides on a tab root", () => {
    const { webApp, backHandlers } = fakeWebApp();
    const view = mount(webApp);
    view.goTo("/uz-Latn/signs");
    view.goTo("/uz-Latn/stats");
    view.goTo("/uz-Latn/premium");
    expect(backHandlers.size).toBe(1);
    vi.mocked(webApp.BackButton.hide).mockClear();
    view.goTo("/uz-Latn/dashboard");
    expect(backHandlers.size).toBe(0);
    expect(webApp.BackButton.hide).toHaveBeenCalled();
    view.unmount();
    expect(backHandlers.size).toBe(0);
  });
});

describe("TelegramChrome BackButton in a running test", () => {
  // The runner's own exit control confirms before abandoning the attempt.
  it("is hidden so it cannot skip the runner's exit confirmation", () => {
    nav.pathname = "/uz-Latn/session/x";
    const attempt = runner();
    const { webApp, backHandlers } = fakeWebApp();
    const view = mount(webApp);
    expect(webApp.BackButton.hide).toHaveBeenCalled();
    expect(webApp.BackButton.show).not.toHaveBeenCalled();
    expect(backHandlers.size).toBe(0);
    view.goTo("/uz-Latn/practice/memorize/7");
    expect(backHandlers.size).toBe(0);
    attempt.unmount();
  });

  it("comes back on the finished session's result screen (same path)", () => {
    nav.pathname = "/uz-Latn/session/x";
    const attempt = runner();
    const { webApp, backHandlers } = fakeWebApp();
    mount(webApp);
    expect(backHandlers.size).toBe(0);
    act(() => attempt.rerender({ running: false }));
    expect(backHandlers.size).toBe(1);
    expect(webApp.BackButton.show).toHaveBeenCalled();
    attempt.unmount();
  });

  it("is shown while the runner is still loading (nothing to lose yet)", () => {
    nav.pathname = "/uz-Latn/session/x";
    const { webApp, backHandlers } = fakeWebApp();
    mount(webApp);
    expect(backHandlers.size).toBe(1);
  });
});

describe("TelegramChrome theme", () => {
  it("paints Telegram's frame with our --background token", async () => {
    const { webApp } = fakeWebApp();
    mount(webApp);
    await act(async () => {
      await new Promise((r) => requestAnimationFrame(() => r(null)));
    });
    expect(webApp.setHeaderColor).toHaveBeenCalledWith("#0e1116");
    expect(webApp.setBackgroundColor).toHaveBeenCalledWith("#0e1116");
    expect(webApp.setBottomBarColor).toHaveBeenCalledWith("#0e1116");
  });

  it("repaints when the resolved theme changes and survives a client that throws", async () => {
    const { webApp } = fakeWebApp({
      setHeaderColor: vi.fn(() => {
        throw new Error("unsupported");
      }),
    });
    const view = mount(webApp);
    await act(async () => {
      await new Promise((r) => requestAnimationFrame(() => r(null)));
    });
    document.documentElement.style.setProperty("--background", "220 16% 96%");
    view.scheme("light");
    await act(async () => {
      await new Promise((r) => requestAnimationFrame(() => r(null)));
    });
    expect(webApp.setBackgroundColor).toHaveBeenLastCalledWith("#f3f4f6");
  });

  it("skips colours the client cannot take as hex", async () => {
    const { webApp } = fakeWebApp({ isVersionAtLeast: () => false });
    mount(webApp);
    await act(async () => {
      await new Promise((r) => requestAnimationFrame(() => r(null)));
    });
    expect(webApp.setHeaderColor).not.toHaveBeenCalled();
    expect(webApp.setBottomBarColor).not.toHaveBeenCalled();
  });
});

describe("TelegramChrome closing confirmation", () => {
  it("is on only inside a running test or memorize session", () => {
    nav.pathname = "/uz-Latn/session/x";
    const attempt = runner();
    const { webApp } = fakeWebApp();
    const view = mount(webApp);
    expect(webApp.enableClosingConfirmation).toHaveBeenCalledTimes(1);
    view.goTo("/uz-Latn/dashboard");
    expect(webApp.disableClosingConfirmation).toHaveBeenCalled();
    view.goTo("/uz-Latn/practice/memorize/7");
    expect(webApp.enableClosingConfirmation).toHaveBeenCalledTimes(2);
    attempt.unmount();
  });

  it("is lifted on the finished session's result screen", () => {
    nav.pathname = "/uz-Latn/session/x";
    const attempt = runner();
    const { webApp } = fakeWebApp();
    mount(webApp);
    expect(webApp.enableClosingConfirmation).toHaveBeenCalledTimes(1);
    vi.mocked(webApp.disableClosingConfirmation).mockClear();
    act(() => attempt.rerender({ running: false }));
    expect(webApp.disableClosingConfirmation).toHaveBeenCalled();
    expect(webApp.enableClosingConfirmation).toHaveBeenCalledTimes(1);
    attempt.unmount();
  });

  it("is off on a runner path while no attempt is running", () => {
    nav.pathname = "/uz-Latn/session/x";
    const { webApp } = fakeWebApp();
    mount(webApp);
    expect(webApp.enableClosingConfirmation).not.toHaveBeenCalled();
  });

  it("is never on outside a runner path, even with a stale running report", () => {
    const attempt = runner();
    const { webApp } = fakeWebApp();
    mount(webApp);
    expect(webApp.enableClosingConfirmation).not.toHaveBeenCalled();
    attempt.unmount();
  });
});

describe("TelegramChrome links", () => {
  function clickAnchor(href: string, init: MouseEventInit = {}, attrs: Record<string, string> = {}) {
    const a = document.createElement("a");
    a.href = href;
    Object.entries(attrs).forEach(([k, v]) => a.setAttribute(k, v));
    const span = document.createElement("span");
    a.appendChild(span);
    document.body.appendChild(a);
    const event = new MouseEvent("click", { bubbles: true, cancelable: true, button: 0, ...init });
    // Record what the chrome decided, then stop jsdom from attempting the
    // navigation it does not implement (window runs after document).
    let prevented = false;
    const settle = (e: Event) => {
      prevented = e.defaultPrevented;
      e.preventDefault();
    };
    window.addEventListener("click", settle);
    span.dispatchEvent(event);
    window.removeEventListener("click", settle);
    return { defaultPrevented: prevented };
  }

  it("opens external links through Telegram", () => {
    const { webApp } = fakeWebApp();
    mount(webApp);
    const event = clickAnchor("https://payme.uz/x");
    expect(webApp.openLink).toHaveBeenCalledWith("https://payme.uz/x");
    expect(event.defaultPrevented).toBe(true);
  });

  it("opens t.me links inside Telegram", () => {
    const { webApp } = fakeWebApp();
    mount(webApp);
    const event = clickAnchor("https://t.me/x");
    expect(webApp.openTelegramLink).toHaveBeenCalledWith("https://t.me/x");
    expect(event.defaultPrevented).toBe(true);
  });

  it.each([
    ["same-origin", `${window.location.origin}/uz-Latn/signs`, {}, {}],
    ["tel:", "tel:+998901234567", {}, {}],
    ["mailto:", "mailto:a@b.c", {}, {}],
    ["ctrl-click", "https://payme.uz/x", { ctrlKey: true }, {}],
    ["meta-click", "https://payme.uz/x", { metaKey: true }, {}],
    ["shift-click", "https://payme.uz/x", { shiftKey: true }, {}],
    ["middle-click", "https://payme.uz/x", { button: 1 }, {}],
    ["download", "https://cdn.example.com/a.pdf", {}, { download: "" }],
  ])("leaves %s alone", (_name, href, init, attrs) => {
    const { webApp } = fakeWebApp();
    mount(webApp);
    const event = clickAnchor(href, init, attrs);
    expect(webApp.openLink).not.toHaveBeenCalled();
    expect(webApp.openTelegramLink).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  // Old clients throw on openLink/openTelegramLink: the click must then go
  // ahead natively instead of being swallowed.
  it.each([
    ["openLink", "https://payme.uz/x"],
    ["openTelegramLink", "https://t.me/x"],
  ] as const)("lets the browser handle the click when %s throws", (method, href) => {
    const { webApp } = fakeWebApp({
      [method]: vi.fn(() => {
        throw new Error("WebAppMethodUnsupported");
      }),
    });
    mount(webApp);
    const event = clickAnchor(href);
    expect(webApp[method]).toHaveBeenCalledWith(href);
    expect(event.defaultPrevented).toBe(false);
  });

  it("stops intercepting once unmounted", () => {
    const { webApp } = fakeWebApp();
    const view = mount(webApp);
    view.unmount();
    clickAnchor("https://payme.uz/x");
    expect(webApp.openLink).not.toHaveBeenCalled();
  });
});

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { TelegramEntry } from "./telegram-entry";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

let currentWebApp: TelegramWebApp | null = null;
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => currentWebApp }));

const markSpy = vi.fn();
vi.mock("@/lib/telegram/web-app", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/telegram/web-app")>();
  return { ...actual, markTelegramMiniApp: () => markSpy() };
});

let cloud: Map<string, string>;

function fakeWebApp(languageCode?: string): TelegramWebApp {
  const cloudStorage: NonNullable<TelegramWebApp["CloudStorage"]> = {
    getItem: (key, cb) => cb(null, cloud.get(key) ?? ""),
    setItem: (key, value, cb) => {
      cloud.set(key, value);
      cb?.(null, true);
    },
    removeItem: (key, cb) => {
      cloud.delete(key);
      cb?.(null, true);
    },
  };
  return {
    initData: "signed",
    initDataUnsafe: { user: { id: 1, first_name: "Ali", language_code: languageCode } },
    CloudStorage: cloudStorage,
  } as unknown as TelegramWebApp;
}

function useWebApp(webApp: TelegramWebApp | null) {
  currentWebApp = webApp;
  // cloud* helpers read the SDK off window, like in the real Mini App.
  window.Telegram = webApp ? { WebApp: webApp } : undefined;
}

type Reply = { status: number; body?: unknown } | "throw";

function json(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const ME_OK = { data: { profile: { must_change_password: false }, vip: null } };
const ME_401 = { status: 401, body: { error: { code: "unauthorized" } } };
const TOKENS_OK = { status: 200, body: { data: { ok: true, must_change_password: false } } };

/** /me answers are consumed in order; the last one repeats. */
function mockFetch(me: Reply[], telegram: Reply[] = []) {
  const fetchMock = vi.fn<typeof fetch>(async (input) => {
    const url = String(input);
    const queue = url === "/api/proxy/me" ? me : url === "/api/auth/telegram" ? telegram : null;
    if (!queue || queue.length === 0) throw new Error(`unexpected fetch ${url}`);
    const reply = queue.length > 1 ? queue.shift()! : queue[0];
    if (reply === "throw") throw new TypeError("Failed to fetch");
    return json(reply.status, reply.body);
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function telegramCalls(fetchMock: ReturnType<typeof mockFetch>) {
  return fetchMock.mock.calls.filter(([url]) => String(url) === "/api/auth/telegram");
}

function renderEntry() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TelegramEntry />
    </NextIntlClientProvider>
  );
}

beforeEach(() => {
  cloud = new Map();
  useWebApp(fakeWebApp());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
  replace.mockClear();
  markSpy.mockClear();
  localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("TelegramEntry", () => {
  it("signs in silently and opens the dashboard once the cookie is proven", async () => {
    cloud.set("other", "x");
    const fetchMock = mockFetch([ME_401, { status: 200, body: ME_OK }], [TOKENS_OK]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    const [, init] = telegramCalls(fetchMock)[0];
    expect(JSON.parse(String(init?.body))).toEqual({ init_data: "signed" });
    expect(markSpy).toHaveBeenCalled();
  });

  it("skips the Telegram sign-in entirely when the session is still alive", async () => {
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }]);
    cloud.set("autologin_off", "1");
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  it("sends a live session that must change its password to change-password", async () => {
    mockFetch([{ status: 200, body: { data: { profile: { must_change_password: true } } } }]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/change-password"));
  });

  it("honours a safe next path from the launch URL", async () => {
    window.history.replaceState(null, "", "/uz-Latn/tg?next=%2Fuz-Latn%2Ftickets");
    mockFetch([{ status: 200, body: ME_OK }]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/tickets"));
  });

  it("greets an unlinked user and offers login and register", async () => {
    mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    renderEntry();
    const heading = await screen.findByRole("heading", { name: /Ali/ });
    expect(heading).toBeInTheDocument();
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register");
    expect(replace).not.toHaveBeenCalled();
  });

  it("asks to reopen the bot on invalid_init_data", async () => {
    mockFetch([ME_401], [{ status: 401, body: { error: { code: "invalid_init_data" } } }]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Botdan oching" })).toBeInTheDocument();
    expect(screen.getByText(/Botni qayta oching/)).toBeInTheDocument();
  });

  it("waits for an explicit tap after a deliberate sign-out", async () => {
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch([ME_401, { status: 200, body: ME_OK }], [TOKENS_OK]);
    renderEntry();
    const button = await screen.findByRole("button", { name: "Ali sifatida davom etish" });
    expect(telegramCalls(fetchMock)).toHaveLength(0);
    fireEvent.click(button);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(1);
    expect(cloud.has("autologin_off")).toBe(false);
  });

  it("explains a refused cookie instead of opening a broken app", async () => {
    mockFetch([ME_401], [TOKENS_OK]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Telefoningizda oching" })).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("shows a rate-limit state with a working retry", async () => {
    const fetchMock = mockFetch([ME_401], [{ status: 429, body: { error: { code: "rate_limited" } } }, TOKENS_OK]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Juda ko'p urinish" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));
    await waitFor(() => expect(telegramCalls(fetchMock)).toHaveLength(2));
  });

  it("tells a blocked account to contact support", async () => {
    mockFetch([ME_401], [{ status: 403, body: { error: { code: "account_blocked" } } }]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Hisob bloklangan" })).toBeInTheDocument();
    expect(screen.getByText(/support bilan bog'laning/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qayta urinish" })).toBeNull();
  });

  it("offers a retry on a network failure", async () => {
    mockFetch([ME_401], ["throw"]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Ulanib bo'lmadi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qayta urinish" })).toBeInTheDocument();
  });

  it("says the service is unavailable when the bot is not configured", async () => {
    mockFetch([ME_401], [{ status: 503, body: { error: { code: "telegram_bot_unconfigured" } } }]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Vaqtincha mavjud emas" })).toBeInTheDocument();
  });

  it("switches a Russian-speaking user to ru on first open without fetching", async () => {
    useWebApp(fakeWebApp("ru"));
    window.history.replaceState(null, "", "/uz-Latn/tg?next=%2Fuz-Latn%2Ftickets");
    const fetchMock = mockFetch([ME_401]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/tg?next=%2Fuz-Latn%2Ftickets"));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("falls back to the open-from-the-bot screen outside Telegram", () => {
    vi.useFakeTimers();
    useWebApp(null);
    const fetchMock = mockFetch([ME_401]);
    renderEntry();
    expect(screen.getByRole("status")).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByRole("heading", { name: "Botdan oching" })).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(markSpy).not.toHaveBeenCalled();
  });
});

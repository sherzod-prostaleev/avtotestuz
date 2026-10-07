import { StrictMode } from "react";
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

type Reply = { status: number; body?: unknown } | "throw" | "hang";

function json(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const ME_OK = { data: { profile: { must_change_password: false }, vip: null } };
const ME_401 = { status: 401, body: { error: { code: "unauthorized" } } };
const TOKENS_OK = { status: 200, body: { data: { ok: true, must_change_password: false } } };

/** /me answers are consumed in order; the last one repeats. */
function mockFetch(me: Reply[], telegram: Reply[] = []) {
  const fetchMock = vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input);
    const queue = url === "/api/proxy/me" ? me : url === "/api/auth/telegram" ? telegram : null;
    if (!queue || queue.length === 0) throw new Error(`unexpected fetch ${url}`);
    const reply = queue.length > 1 ? queue.shift()! : queue[0];
    if (reply === "throw") throw new TypeError("Failed to fetch");
    if (reply === "hang") {
      // A stalled connection: only the caller's abort ends it.
      return new Promise<Response>((_, reject) => {
        init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
      });
    }
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
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/tg?next=%2Fru%2Ftickets"));
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

  it("shows the way back to the bot inside Telegram on invalid_init_data", async () => {
    const close = vi.fn();
    useWebApp({ ...fakeWebApp(), close } as unknown as TelegramWebApp);
    mockFetch([ME_401], [{ status: 401, body: { error: { code: "invalid_init_data" } } }]);
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Botga qaytish" }));
    expect(close).toHaveBeenCalled();
  });

  it("shows the way back to the bot when the bot is unconfigured", async () => {
    const close = vi.fn();
    useWebApp({ ...fakeWebApp(), close } as unknown as TelegramWebApp);
    mockFetch([ME_401], [{ status: 503, body: { error: { code: "telegram_bot_unconfigured" } } }]);
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Botga qaytish" }));
    expect(close).toHaveBeenCalled();
  });

  it("has no bot button on the plain-browser outside screen", () => {
    vi.useFakeTimers();
    useWebApp(null);
    mockFetch([ME_401]);
    renderEntry();
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.queryByRole("button", { name: "Botga qaytish" })).toBeNull();
  });

  it("keeps ?next across the locale redirect", async () => {
    useWebApp(fakeWebApp("ru"));
    window.history.replaceState(null, "", "/uz-Latn/tg?next=%2Fuz-Latn%2Ftickets%3Fa%3D1&x=2");
    mockFetch([ME_401]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/tg?next=%2Fru%2Ftickets%3Fa%3D1&x=2"));
  });

  it("drops a foreign-locale next on the locale redirect", async () => {
    useWebApp(fakeWebApp("ru"));
    window.history.replaceState(null, "", "/uz-Latn/tg?next=%2Fen%2Fx");
    mockFetch([ME_401]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/tg"));
  });

  it("falls through to sign-in after the 8s /me probe timeout", async () => {
    vi.useFakeTimers();
    const fetchMock = mockFetch(["hang", ME_401], [{ status: 200, body: { data: { need_phone: true } } }]);
    renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(7900);
    });
    // Still probing: a shorter probe budget than the 15s sign-in one.
    expect(telegramCalls(fetchMock)).toHaveLength(0);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(telegramCalls(fetchMock)).toHaveLength(1);
    expect(screen.getByRole("link", { name: "Kirish" })).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("shows the welcome with continue-as when CloudStorage times out, never auto sign-in", async () => {
    vi.useFakeTimers();
    const app = fakeWebApp();
    app.CloudStorage = { ...app.CloudStorage!, getItem: () => {} };
    useWebApp(app);
    const fetchMock = mockFetch([ME_401], [TOKENS_OK]);
    renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });
    expect(screen.getByRole("button", { name: "Ali sifatida davom etish" })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  it("shows the welcome with continue-as when CloudStorage errors", async () => {
    const app = fakeWebApp();
    app.CloudStorage = { ...app.CloudStorage!, getItem: (_k, cb) => cb("STORAGE_ERROR") };
    useWebApp(app);
    const fetchMock = mockFetch([ME_401], [TOKENS_OK]);
    renderEntry();
    expect(await screen.findByRole("button", { name: "Ali sifatida davom etish" })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  it("auto signs in when the client has no CloudStorage at all", async () => {
    const app = fakeWebApp();
    delete app.CloudStorage;
    useWebApp(app);
    const fetchMock = mockFetch([ME_401, { status: 200, body: ME_OK }], [TOKENS_OK]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(1);
  });

  it("offers the way back to the bot on the cookie_blocked and blocked screens", async () => {
    const close = vi.fn();
    const app = fakeWebApp();
    app.close = close;
    useWebApp(app);
    mockFetch([ME_401], [TOKENS_OK]);
    const first = renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Botga qaytish" }));
    expect(close).toHaveBeenCalledTimes(1);
    first.unmount();
    mockFetch([ME_401], [{ status: 403, body: { error: { code: "account_blocked" } } }]);
    renderEntry();
    expect(await screen.findByRole("button", { name: "Botga qaytish" })).toBeInTheDocument();
  });

  it("aborts in-flight work on unmount and never navigates afterwards", async () => {
    vi.useFakeTimers();
    const signals: AbortSignal[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: unknown, init?: RequestInit) => {
        signals.push(init!.signal!);
        // Ignores the abort on purpose: a late answer must still be dropped.
        return new Promise<Response>((resolve) => setTimeout(() => resolve(json(200, ME_OK)), 5000));
      })
    );
    const view = renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(signals).toHaveLength(1);
    view.unmount();
    expect(signals[0].aborted).toBe(true);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20000);
    });
    expect(replace).not.toHaveBeenCalled();
    expect(signals).toHaveLength(1);
  });

  it("clears the request timers on unmount", async () => {
    vi.useFakeTimers();
    mockFetch(["hang"]);
    const view = renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(100);
    });
    view.unmount();
    // The aborted fetch rejects on a microtask; its finally then clears the timer.
    await act(async () => {
      await Promise.resolve();
    });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("still signs in under StrictMode's mount, unmount, mount", async () => {
    // The aborted first run consumes one /me reply, so every reply is a live session.
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }]);
    render(
      <StrictMode>
        <NextIntlClientProvider locale="uz-Latn" messages={messages}>
          <TelegramEntry />
        </NextIntlClientProvider>
      </StrictMode>
    );
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(replace).toHaveBeenCalledTimes(1);
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  it("reaches a retry button when the sign-in call stalls", async () => {
    vi.useFakeTimers();
    mockFetch([ME_401], ["hang"]);
    renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(16000);
    });
    expect(screen.getByRole("button", { name: "Qayta urinish" })).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("offers a reload when the SDK never arrives despite launch data", () => {
    vi.useFakeTimers();
    useWebApp(null);
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=x");
    mockFetch([ME_401]);
    renderEntry();
    act(() => {
      vi.advanceTimersByTime(10000);
    });
    expect(screen.getByRole("heading", { name: "Ulanib bo'lmadi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qayta urinish" })).toBeInTheDocument();
  });

  it("stays on sign-in when retrying after continue-as", async () => {
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch(
      [ME_401],
      [{ status: 429, body: { error: { code: "rate_limited" } } }, { status: 200, body: { data: { need_phone: true } } }]
    );
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Ali sifatida davom etish" }));
    fireEvent.click(await screen.findByRole("button", { name: "Qayta urinish" }));
    await waitFor(() => expect(telegramCalls(fetchMock)).toHaveLength(2));
    // Not bounced back to the "continue as" welcome.
    expect(screen.queryByRole("button", { name: "Ali sifatida davom etish" })).toBeNull();
  });

  it("falls through to sign-in when the /me probe fails", async () => {
    const fetchMock = mockFetch([{ status: 500 }, ME_401], [{ status: 200, body: { data: { need_phone: true } } }]);
    renderEntry();
    expect(await screen.findByRole("link", { name: "Kirish" })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(1);
  });
});

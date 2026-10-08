import { StrictMode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { TelegramEntry } from "./telegram-entry";
import { installTelegramHost, removeTelegramHost } from "@/test/telegram-host";

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
  // cloud* helpers read the SDK off window, like in the real Mini App, and
  // only trust it with a Telegram client around the page.
  window.Telegram = webApp ? { WebApp: webApp } : undefined;
  if (webApp) installTelegramHost();
  else removeTelegramHost();
}

type Reply = { status: number; body?: unknown } | "throw" | "hang";

function json(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const ME_OK = { data: { profile: { must_change_password: false }, vip: null } };
const ME_401 = { status: 401, body: { error: { code: "unauthorized" } } };
const TOKENS_OK = { status: 200, body: { data: { ok: true, must_change_password: false } } };

/** /me answers are consumed in order; the last one repeats. */
const LINKED_NONE: Reply = { status: 200, body: { data: { linked: false } } };
const linkedTo = (tgUserId: number): Reply => ({
  status: 200,
  body: { data: { linked: true, username: "someone", tg_user_id: tgUserId } },
});

function mockFetch(me: Reply[], telegram: Reply[] = [], meTelegram: Reply[] = [LINKED_NONE]) {
  const fetchMock = vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input);
    if (url === "/api/auth/logout") return json(200, { data: { ok: true } });
    const queue =
      url === "/api/proxy/me"
        ? me
        : url === "/api/auth/telegram"
          ? telegram
          : url === "/api/proxy/me/telegram"
            ? meTelegram
            : null;
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

function logoutCalls(fetchMock: ReturnType<typeof mockFetch>) {
  return fetchMock.mock.calls.filter(([url]) => String(url) === "/api/auth/logout");
}

function renderEntry(props: { botUsername?: string | null } = {}) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TelegramEntry {...props} />
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
  sessionStorage.clear();
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

  it("keeps a live session already linked to the launching Telegram user", async () => {
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }], [], [linkedTo(1)]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  it("keeps a live session when its link status cannot be read", async () => {
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }], [], [{ status: 500, body: {} }]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  // I4: a shared phone or webview keeps someone else's cookie session. When
  // that profile is linked to ANOTHER Telegram account, the launching user's
  // signed identity wins: sign in with it instead of opening a stranger's app.
  it("signs the launching user in over a session linked to another Telegram account", async () => {
    const fetchMock = mockFetch(
      [{ status: 200, body: ME_OK }, { status: 200, body: ME_OK }],
      [TOKENS_OK],
      [linkedTo(999)],
    );
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    const calls = telegramCalls(fetchMock);
    expect(calls).toHaveLength(1);
    expect(JSON.parse(String(calls[0][1]?.body))).toEqual({ init_data: "signed" });
  });

  it("shows the welcome when the launching user is unlinked and the session is someone else's", async () => {
    const fetchMock = mockFetch(
      [{ status: 200, body: ME_OK }],
      [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }],
      [linkedTo(999)],
    );
    renderEntry();
    expect(await screen.findByRole("heading", { name: /Ali/ })).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
    // The stranger's cookies must be gone before Kirish can be tapped.
    expect(logoutCalls(fetchMock)).toHaveLength(1);
    expect(cloud.has("autologin_off")).toBe(false);
  });

  it("logs the stranger out before the autologin_off welcome", async () => {
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }], [], [linkedTo(999)]);
    renderEntry();
    await screen.findByRole("button", { name: "Ali sifatida davom etish" });
    expect(logoutCalls(fetchMock)).toHaveLength(1);
    expect(telegramCalls(fetchMock)).toHaveLength(0);
    expect(cloud.get("autologin_off")).toBe("1");
  });

  it("shows a retry, not the welcome, when the stranger's logout fails", async () => {
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }], [], [linkedTo(999)]);
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation(async (input, init) =>
      String(input) === "/api/auth/logout" ? json(500) : base(input, init),
    );
    renderEntry();
    await waitFor(() => expect(logoutCalls(fetchMock)).toHaveLength(1));
    expect(screen.queryByRole("button", { name: "Ali sifatida davom etish" })).not.toBeInTheDocument();
  });

  it("does not log out a session linked to the same Telegram user", async () => {
    const same = mockFetch([{ status: 200, body: ME_OK }], [], [linkedTo(1)]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalled());
    expect(logoutCalls(same)).toHaveLength(0);
  });

  it("does not log out a session that is not linked to any Telegram account", async () => {
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }], [], [LINKED_NONE]);
    renderEntry();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(logoutCalls(fetchMock)).toHaveLength(0);
    expect(telegramCalls(fetchMock)).toHaveLength(0);
  });

  // The stranger flag belongs to one enter() attempt. If the session is gone
  // by the retry, the retry must not log out again (and fail again).
  it("re-evaluates the stranger session on retry instead of reusing the old verdict", async () => {
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch([{ status: 200, body: ME_OK }, ME_401], [], [linkedTo(999)]);
    const base = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation(async (input, init) =>
      String(input) === "/api/auth/logout" ? json(500) : base(input, init),
    );
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Qayta urinish" }));
    await screen.findByRole("button", { name: "Ali sifatida davom etish" });
    expect(logoutCalls(fetchMock)).toHaveLength(1);
  });

  it("does not log out when there is no session at all", async () => {
    const fetchMock = mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    renderEntry();
    await screen.findByRole("heading", { name: /Ali/ });
    expect(logoutCalls(fetchMock)).toHaveLength(0);
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
    expect(screen.getByText(messages.TelegramApp.phoneConfirmNote)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register");
    expect(replace).not.toHaveBeenCalled();
  });

  it("asks to reopen the bot on invalid_init_data", async () => {
    mockFetch([ME_401], [{ status: 401, body: { error: { code: "invalid_init_data" } } }]);
    renderEntry();
    expect(await screen.findByRole("heading", { name: "Botdan oching" })).toBeInTheDocument();
    expect(screen.getByText(/Botni oching yoki saytda kiring/)).toBeInTheDocument();
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
    expect(await screen.findByRole("heading", { name: "Telegram ilovasida oching" })).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("shows a rate-limit state whose retry unlocks after a 30 s countdown", async () => {
    vi.useFakeTimers();
    const fetchMock = mockFetch([ME_401], [{ status: 429, body: { error: { code: "rate_limited" } } }, TOKENS_OK]);
    renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(50);
    });
    expect(screen.getByRole("heading", { name: "Juda ko'p urinish" })).toBeInTheDocument();
    const waiting = screen.getByRole("button", { name: "Qayta urinish (30)" });
    expect(waiting).toBeDisabled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(screen.getByRole("button", { name: "Qayta urinish (20)" })).toBeDisabled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });
    const retry = screen.getByRole("button", { name: "Qayta urinish" });
    expect(retry).toBeEnabled();
    fireEvent.click(retry);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(50);
    });
    expect(telegramCalls(fetchMock)).toHaveLength(2);
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

  // C1: a plain browser opening a planted /tg#tgWebAppData=<someone else's>
  // link must not sign in as that account (login CSRF) or flag the tab.
  it("treats injected launch data in a plain browser as outside Telegram", () => {
    vi.useFakeTimers();
    useWebApp(null);
    window.history.replaceState(null, "", "/uz-Latn/tg#tgWebAppData=attacker&tgWebAppVersion=8.0");
    const fetchMock = mockFetch([ME_401]);
    renderEntry();
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByRole("heading", { name: "Botdan oching" })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(0);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(markSpy).not.toHaveBeenCalled();
  });

  it("offers a reload when the SDK never arrives despite launch data", () => {
    vi.useFakeTimers();
    useWebApp(null);
    installTelegramHost();
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
    vi.useFakeTimers();
    cloud.set("autologin_off", "1");
    const fetchMock = mockFetch(
      [ME_401],
      [{ status: 429, body: { error: { code: "rate_limited" } } }, { status: 200, body: { data: { need_phone: true } } }]
    );
    renderEntry();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(50);
    });
    fireEvent.click(screen.getByRole("button", { name: "Ali sifatida davom etish" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(50);
    });
    // The rate-limit cool-down has to run out before the retry is offered.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(50);
    });
    expect(telegramCalls(fetchMock)).toHaveLength(2);
    // Not bounced back to the "continue as" welcome.
    expect(screen.queryByRole("button", { name: "Ali sifatida davom etish" })).toBeNull();
  });

  it("falls through to sign-in when the /me probe fails", async () => {
    const fetchMock = mockFetch([{ status: 500 }, ME_401], [{ status: 200, body: { data: { need_phone: true } } }]);
    renderEntry();
    expect(await screen.findByRole("link", { name: "Kirish" })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(1);
  });

  it("navigates even if cloudRemove never calls back", async () => {
    vi.useFakeTimers();
    try {
      const app = fakeWebApp();
      app.CloudStorage = { ...app.CloudStorage!, removeItem: () => {} };
      useWebApp(app);
      mockFetch([ME_401, { status: 200, body: ME_OK }], [TOKENS_OK]);
      renderEntry();
      // Sign-in completes, probeMe succeeds, cloudRemove is called but never returns.
      // After 3s, cloudRemove times out and navigation should proceed.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3100);
      });
      expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard");
    } finally {
      vi.useRealTimers();
    }
  });
  // Audit-2 I2: Back from /login remounts /tg; the need_phone answer cannot
  // have changed, so it must not cost another rate-limited sign-in call.
  it("reuses the need_phone verdict on remount instead of signing in again", async () => {
    const fetchMock = mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    const first = renderEntry();
    expect(await screen.findByRole("heading", { name: /Ali/ })).toBeInTheDocument();
    first.unmount();
    renderEntry();
    expect(await screen.findByRole("heading", { name: /Ali/ })).toBeInTheDocument();
    expect(telegramCalls(fetchMock)).toHaveLength(1);
  });

  it("does not reuse another Telegram user's need_phone verdict", async () => {
    const fetchMock = mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    const first = renderEntry();
    expect(await screen.findByRole("heading", { name: /Ali/ })).toBeInTheDocument();
    first.unmount();
    const other = fakeWebApp();
    other.initDataUnsafe = { user: { id: 2, first_name: "Vali" } };
    useWebApp(other);
    renderEntry();
    await waitFor(() => expect(telegramCalls(fetchMock)).toHaveLength(2));
  });

  it("forgets the need_phone verdict once a sign-in succeeds", async () => {
    sessionStorage.setItem("tg-need-phone:1", JSON.stringify({ firstName: "Ali" }));
    const fetchMock = mockFetch([ME_401, { status: 200, body: ME_OK }], [TOKENS_OK]);
    cloud.set("autologin_off", "1");
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "Ali sifatida davom etish" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(telegramCalls(fetchMock)).toHaveLength(1);
    expect(sessionStorage.getItem("tg-need-phone:1")).toBeNull();
  });

  // Audit-2 I3: a deep link's next must survive the phone sign-in detour.
  it("passes a safe next through the welcome's Kirish and Ro'yxatdan o'tish", async () => {
    window.history.replaceState(null, "", "/uz-Latn/tg?next=%2Fuz-Latn%2Fpremium%3Fplan%3Dvip");
    mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    renderEntry();
    const login = await screen.findByRole("link", { name: "Kirish" });
    expect(login).toHaveAttribute("href", "/uz-Latn/login?next=%2Fuz-Latn%2Fpremium%3Fplan%3Dvip");
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute(
      "href",
      "/uz-Latn/register?next=%2Fuz-Latn%2Fpremium%3Fplan%3Dvip"
    );
  });

  it("drops an unsafe next from the welcome links", async () => {
    window.history.replaceState(null, "", "/uz-Latn/tg?next=https%3A%2F%2Fevil.com");
    mockFetch([ME_401], [{ status: 200, body: { data: { need_phone: true, first_name: "Ali" } } }]);
    renderEntry();
    expect(await screen.findByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
  });

  it("puts only the first word of a long Telegram name on the continue button", async () => {
    const app = fakeWebApp();
    app.initDataUnsafe = { user: { id: 1, first_name: "Abdurahmonbekjonovich Karimov" } };
    useWebApp(app);
    cloud.set("autologin_off", "1");
    mockFetch([ME_401]);
    renderEntry();
    expect(await screen.findByRole("button", { name: "Abdurahmonbekjon… sifatida davom etish" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /Abdurahmonbekjonovich Karimov/ })).toBeInTheDocument();
  });

  it("says Davom etish when Telegram sent no first name", async () => {
    const app = fakeWebApp();
    app.initDataUnsafe = { user: { id: 1 } };
    useWebApp(app);
    cloud.set("autologin_off", "1");
    mockFetch([ME_401]);
    renderEntry();
    expect(await screen.findByRole("button", { name: "Davom etish" })).toBeInTheDocument();
  });

  it("says it is connecting, not signing in, before any sign-in call", () => {
    mockFetch(["hang"]);
    renderEntry();
    expect(screen.getByRole("status")).toHaveTextContent(messages.TelegramApp.connecting);
  });

  it("says it is signing in while the sign-in call runs", async () => {
    mockFetch([ME_401], ["hang"]);
    renderEntry();
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(messages.TelegramApp.loading));
  });

  // Audit-2 I7: the website's /tg must not be a dead end.
  it("offers the bot and a website login on the plain-browser outside screen", () => {
    vi.useFakeTimers();
    useWebApp(null);
    mockFetch([ME_401]);
    renderEntry({ botUsername: "DriverGouzBot" });
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByRole("link", { name: "Botni ochish" })).toHaveAttribute("href", "https://t.me/DriverGouzBot");
    expect(screen.getByRole("link", { name: "Saytda kirish" })).toHaveAttribute("href", "/uz-Latn/login");
  });

  it("never links to a malformed bot username", () => {
    vi.useFakeTimers();
    useWebApp(null);
    mockFetch([ME_401]);
    renderEntry({ botUsername: "evil.com/x" });
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.queryByRole("link", { name: "Botni ochish" })).toBeNull();
    expect(screen.getByRole("link", { name: "Saytda kirish" })).toBeInTheDocument();
  });

  it("links a blocked account to the support chat", async () => {
    const fetchMock = vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url === "/api/proxy/me") return json(401, { error: { code: "unauthorized" } });
      if (url === "/api/auth/telegram") return json(403, { error: { code: "account_blocked" } });
      if (url === "/api/proxy/site/contacts") return json(200, { data: { telegramUrl: "https://t.me/DriverGoHelp" } });
      throw new Error(`unexpected fetch ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);
    renderEntry();
    expect(await screen.findByRole("link", { name: "Qo'llab-quvvatlashga yozish" })).toHaveAttribute(
      "href",
      "https://t.me/DriverGoHelp"
    );
  });

  it("falls back to our own support account when the contacts cannot be read", async () => {
    const fetchMock = vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      if (url === "/api/proxy/me") return json(401, { error: { code: "unauthorized" } });
      if (url === "/api/auth/telegram") return json(403, { error: { code: "account_blocked" } });
      return json(500, {});
    });
    vi.stubGlobal("fetch", fetchMock);
    renderEntry();
    expect(await screen.findByRole("link", { name: "Qo'llab-quvvatlashga yozish" })).toHaveAttribute(
      "href",
      "https://t.me/DriverGo"
    );
  });

  it("makes drivergo.uz a real link on the unavailable screen", async () => {
    mockFetch([ME_401], [{ status: 503, body: { error: { code: "telegram_bot_unconfigured" } } }]);
    renderEntry();
    expect(await screen.findByRole("link", { name: "drivergo.uz" })).toHaveAttribute("href", "https://drivergo.uz/uz-Latn");
  });

  // A11y: focus moves to the heading; an assertive alert on top of it would
  // make screen readers announce the same screen twice.
  it("focuses a notice heading without also raising an alert", async () => {
    mockFetch([ME_401], [{ status: 401, body: { error: { code: "invalid_init_data" } } }]);
    renderEntry();
    const heading = await screen.findByRole("heading", { name: "Botdan oching" });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

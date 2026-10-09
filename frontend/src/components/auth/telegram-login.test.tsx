import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import ruMessages from "../../../messages/ru.json";
import { TelegramLogin, TELEGRAM_LOGIN_STORAGE_KEY, isTelegramDeepLink } from "./telegram-login";

const TOKEN = "T".repeat(43);
const BOT_URL = `https://t.me/DriverGouzBot?start=login_${TOKEN}`;

afterEach(() => {
  window.sessionStorage.clear();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function json(body: unknown, status = 200) {
  return Promise.resolve(new Response(JSON.stringify(body), { status }));
}

/** fetch stub: start → link, status → the given states in turn, complete → ok. */
function backend(states: string[], completeStatus = 200) {
  let i = 0;
  return vi.fn((url: string) => {
    if (url === "/api/auth/telegram-login/start") {
      return json({ data: { bot_url: BOT_URL, token: TOKEN, expires_in_sec: 300 } });
    }
    if (url.startsWith("/api/auth/telegram-login/status")) {
      const state = states[Math.min(i, states.length - 1)];
      i++;
      return json({ data: { state } });
    }
    if (url === "/api/auth/telegram-login/complete") {
      return completeStatus === 200
        ? json({ data: { ok: true, must_change_password: false, created: true } })
        : json({ error: { code: "invalid_login_request" } }, completeStatus);
    }
    return json({}, 404);
  });
}

function renderLogin(onSuccess = vi.fn(), locale: "uz-Latn" | "ru" = "uz-Latn") {
  render(
    <NextIntlClientProvider locale={locale} messages={locale === "ru" ? ruMessages : messages}>
      <TelegramLogin mode="login" onSuccess={onSuccess} />
    </NextIntlClientProvider>
  );
  return onSuccess;
}

describe("TelegramLogin", () => {
  it("opens the bot in a new tab, shows the QR code, and completes once Telegram approves", async () => {
    const popup = { opener: {}, location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn(() => popup));
    const fetchMock = backend(["pending", "approved"]);
    vi.stubGlobal("fetch", fetchMock);
    const onSuccess = renderLogin();

    fireEvent.click(screen.getByRole("button", { name: "Telegram orqali kirish" }));
    expect(await screen.findByRole("heading", { name: "Telegram'da tasdiqlang" })).toBeInTheDocument();
    // The tab was opened inside the click (popup blockers) without an opener.
    expect(window.open).toHaveBeenCalledWith("", "_blank");
    expect(popup.opener).toBeNull();
    expect(popup.location.href).toBe(BOT_URL);
    expect(screen.getByRole("link", { name: "Telegram'ni ochish" })).toHaveAttribute("href", BOT_URL);
    expect(screen.getByRole("img", { name: "Telegram orqali kirish havolasining QR kodi" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Tasdiq kutilmoqda");

    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith({ mustChangePassword: false, created: true }), {
      timeout: 5000,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/telegram-login/complete",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ token: TOKEN }) })
    );
    expect(window.sessionStorage.getItem(TELEGRAM_LOGIN_STORAGE_KEY)).toBeNull();
  });

  it("says when the learner cancelled in Telegram and offers a retry", async () => {
    vi.stubGlobal("open", vi.fn(() => null));
    vi.stubGlobal("fetch", backend(["cancelled"]));
    const onSuccess = renderLogin();
    fireEvent.click(screen.getByRole("button", { name: "Telegram orqali kirish" }));
    expect(await screen.findByRole("heading", { name: "Kirish bekor qilindi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qaytadan urinish" })).toBeInTheDocument();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it("ends on 'time is up' for an invalid/expired request", async () => {
    vi.stubGlobal("open", vi.fn(() => null));
    vi.stubGlobal("fetch", backend(["invalid"]));
    renderLogin();
    fireEvent.click(screen.getByRole("button", { name: "Telegram orqali kirish" }));
    expect(await screen.findByRole("heading", { name: "Vaqt tugadi" })).toBeInTheDocument();
  });

  it("shows the blocked notice without a retry", async () => {
    vi.stubGlobal("open", vi.fn(() => null));
    vi.stubGlobal("fetch", backend(["blocked"]));
    renderLogin();
    fireEvent.click(screen.getByRole("button", { name: "Telegram orqali kirish" }));
    expect(await screen.findByRole("heading", { name: "Hisob bloklangan" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Qaytadan urinish" })).toBeNull();
  });

  it("closes the pre-opened tab and explains when the bot is not configured", async () => {
    const popup = { opener: {}, location: { href: "" }, close: vi.fn() };
    vi.stubGlobal("open", vi.fn(() => popup));
    vi.stubGlobal("fetch", vi.fn(() => json({ error: { code: "telegram_bot_unconfigured" } }, 503)));
    renderLogin();
    fireEvent.click(screen.getByRole("button", { name: "Telegram orqali kirish" }));
    expect(
      await screen.findByText("Telegram orqali kirish hozircha ishlamayapti. Telefon raqam va parol bilan kiring.")
    ).toBeInTheDocument();
    expect(popup.close).toHaveBeenCalled();
    expect(popup.location.href).toBe("");
  });

  it("resumes a pending request after the page was left for Telegram", async () => {
    window.sessionStorage.setItem(
      TELEGRAM_LOGIN_STORAGE_KEY,
      JSON.stringify({ token: TOKEN, botURL: BOT_URL, expiresAt: Date.now() + 120_000 })
    );
    vi.stubGlobal("fetch", backend(["approved"]));
    const onSuccess = renderLogin();
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
  });

  it("ignores a stored link that is not our t.me login link", () => {
    window.sessionStorage.setItem(
      TELEGRAM_LOGIN_STORAGE_KEY,
      JSON.stringify({ token: TOKEN, botURL: "https://evil.example/?start=login_x", expiresAt: Date.now() + 120_000 })
    );
    vi.stubGlobal("fetch", vi.fn());
    renderLogin();
    expect(screen.getByRole("button", { name: "Telegram orqali kirish" })).toBeInTheDocument();
    expect(isTelegramDeepLink("http://t.me/x?start=login_a")).toBe(false);
    expect(isTelegramDeepLink(BOT_URL)).toBe(true);
  });

  it("speaks Russian on /ru", () => {
    renderLogin(vi.fn(), "ru");
    expect(screen.getByRole("button", { name: "Войти через Telegram" })).toBeInTheDocument();
  });
});

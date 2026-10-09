import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { TelegramEntry, referralFromStartParam } from "./telegram-entry";
import { installTelegramHost, removeTelegramHost } from "@/test/telegram-host";

// The Mini App welcome's one-tap «📱 Raqam bilan davom etish» (Flow B).

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

let currentWebApp: TelegramWebApp | null = null;
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => currentWebApp }));

type Share = { shared: boolean; response?: string; phone?: string };

function webApp(opts: { share?: Share; startParam?: string; noContact?: boolean } = {}): TelegramWebApp {
  const cloud = new Map<string, string>([["autologin_off", ""]]);
  const app = {
    initData: "signed-init",
    initDataUnsafe: { user: { id: 7, first_name: "Ali" }, start_param: opts.startParam },
    CloudStorage: {
      getItem: (key: string, cb: (e: string | null, v?: string) => void) => cb(null, cloud.get(key) ?? ""),
      setItem: (key: string, v: string, cb?: (e: string | null, ok?: boolean) => void) => {
        cloud.set(key, v);
        cb?.(null, true);
      },
      removeItem: (key: string, cb?: (e: string | null, ok?: boolean) => void) => {
        cloud.delete(key);
        cb?.(null, true);
      },
    },
    requestContact: opts.noContact
      ? undefined
      : vi.fn((cb: (shared: boolean, res?: unknown) => void) => {
          const s = opts.share ?? { shared: true, response: "contact=signed&hash=ff", phone: "998901112233" };
          cb(s.shared, s.shared ? { response: s.response, responseUnsafe: { contact: { phone_number: s.phone } } } : undefined);
        }),
  };
  return app as unknown as TelegramWebApp;
}

function use(app: TelegramWebApp) {
  currentWebApp = app;
  window.Telegram = { WebApp: app };
  installTelegramHost();
}

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

/** /me: 401 until the phone sign-in succeeded, then 200. */
function backend(phoneReply: { status: number; body: unknown }) {
  let signedIn = false;
  const fetchMock = vi.fn<typeof fetch>(async (input) => {
    const url = String(input);
    if (url === "/api/proxy/me") {
      return signedIn ? json(200, { data: { profile: { must_change_password: false } } }) : json(401, { error: { code: "unauthorized" } });
    }
    if (url === "/api/auth/telegram") return json(200, { data: { need_phone: true, first_name: "Ali" } });
    if (url === "/api/auth/telegram/phone") {
      if (phoneReply.status === 200) signedIn = true;
      return json(phoneReply.status, phoneReply.body);
    }
    throw new Error(`unexpected fetch ${url}`);
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function renderEntry() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TelegramEntry />
    </NextIntlClientProvider>
  );
}

beforeEach(() => {
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  replace.mockClear();
  currentWebApp = null;
  window.Telegram = undefined;
  removeTelegramHost();
});

describe("TelegramEntry one-tap phone sign-in", () => {
  it("leads the welcome with «📱 Raqam bilan davom etish» and keeps the password paths", async () => {
    use(webApp());
    backend({ status: 200, body: {} });
    renderEntry();
    expect(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register");
  });

  it("shares the signed phone, signs in and opens the dashboard", async () => {
    const app = webApp();
    use(app);
    const fetchMock = backend({ status: 200, body: { data: { ok: true, must_change_password: false, created: true } } });
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    const call = fetchMock.mock.calls.find(([u]) => String(u) === "/api/auth/telegram/phone");
    expect(JSON.parse(String(call?.[1]?.body))).toEqual({ init_data: "signed-init", contact: "contact=signed&hash=ff" });
  });

  it("stays on the welcome with a hint when the share is declined", async () => {
    use(webApp({ share: { shared: false } }));
    const fetchMock = backend({ status: 200, body: {} });
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Raqam yuborilmadi");
    expect(fetchMock.mock.calls.some(([u]) => String(u) === "/api/auth/telegram/phone")).toBe(false);
  });

  it("refuses a non-Uzbek number before calling the server", async () => {
    use(webApp({ share: { shared: true, response: "contact=x", phone: "79161234567" } }));
    const fetchMock = backend({ status: 200, body: {} });
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("O'zbekiston raqami emas");
    expect(fetchMock.mock.calls.some(([u]) => String(u) === "/api/auth/telegram/phone")).toBe(false);
  });

  it("shows the blocked screen for a banned account", async () => {
    use(webApp());
    backend({ status: 403, body: { error: { code: "account_blocked" } } });
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" }));
    expect(await screen.findByRole("heading", { name: "Hisob bloklangan" })).toBeInTheDocument();
  });

  it("drops the one-tap button and points to the password paths when the kill switch is off", async () => {
    use(webApp());
    backend({ status: 503, body: { error: { code: "telegram_login_disabled" } } });
    renderEntry();
    fireEvent.click(await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Telegram orqali kirish hozircha ishlamayapti. Telefon raqam va parol bilan kiring."
    );
    expect(screen.queryByRole("button", { name: "📱 Raqam bilan davom etish" })).toBeNull();
    expect(screen.getByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register");
  });

  it("keeps a startapp=ref_ invite on the password registration link", async () => {
    use(webApp({ startParam: "ref_REF-AB23CD" }));
    backend({ status: 200, body: {} });
    renderEntry();
    await screen.findByRole("button", { name: "📱 Raqam bilan davom etish" });
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register?ref=REF-AB23CD");
    expect(screen.getByRole("link", { name: "Kirish" })).toHaveAttribute("href", "/uz-Latn/login");
  });

  it("falls back to login/register on a client without the phone sheet", async () => {
    use(webApp({ noContact: true }));
    backend({ status: 200, body: {} });
    renderEntry();
    await screen.findByRole("link", { name: "Kirish" });
    expect(screen.queryByRole("button", { name: "📱 Raqam bilan davom etish" })).toBeNull();
  });

  it("reads only well-formed referral start params", () => {
    expect(referralFromStartParam("ref_REF-AB23CD")).toBe("REF-AB23CD");
    expect(referralFromStartParam("ref_")).toBeNull();
    expect(referralFromStartParam("ref_a b")).toBeNull();
    expect(referralFromStartParam("login_x")).toBeNull();
    expect(referralFromStartParam(undefined)).toBeNull();
  });
});

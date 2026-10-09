import { act, render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { takeResetPhone } from "@/lib/reset-phone-handoff";
import { describe, it, expect, vi, afterEach } from "vitest";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { currentTelegramHint } from "@/lib/telegram/hint";
import messages from "../../../../../messages/uz-Latn.json";
import LoginPage from "./page";

// The kill-switch probe is its own request at mount; these tests count and
// order the page's fetches, so it is answered here ("on") without one.
vi.mock("@/lib/telegram-login-flag", () => ({ fetchTelegramLoginEnabled: () => Promise.resolve(true) }));

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/login",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/lib/referral-storage", () => ({
  capturePendingReferralCodeFromUrl: vi.fn(),
  applyPendingReferralCode: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@/lib/demo-progress-storage", () => ({
  migrateDemoProgressOnLogin: vi.fn().mockResolvedValue(undefined),
}));

let currentWebApp: TelegramWebApp | null = null;
let currentStatus: "off" | "loading" | "ready" | "failed" = "off";
vi.mock("@/components/telegram/telegram-provider", () => ({
  useTelegram: () => currentWebApp,
  useTelegramStatus: () => currentStatus,
}));

const cloudRemove = vi.fn().mockResolvedValue(undefined);
vi.mock("@/lib/telegram/web-app", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/telegram/web-app")>();
  return { ...actual, cloudRemove: (key: string) => cloudRemove(key) };
});

afterEach(() => {
  window.sessionStorage.clear();
  currentWebApp = null;
  currentStatus = "off";
  cloudRemove.mockClear();
  vi.unstubAllGlobals();
  pushMock.mockClear();
});

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <LoginPage />
    </NextIntlClientProvider>
  );
}

describe("LoginPage", () => {
  it("uses an accessible icon asset instead of an emoji logo", () => {
    const { container } = renderWithIntl();
    expect(screen.getByRole("heading", { name: "Kirish" })).toBeInTheDocument();
    expect(container.textContent).not.toContain("🚗");
  });

  it("lets the user type all 9 national digits after +998 with grouping spaces", async () => {
    const user = userEvent.setup();
    renderWithIntl();

    const input = screen.getByLabelText("Telefon raqam");
    const max = input.getAttribute("maxLength");
    if (max !== null) {
      expect(Number(max)).toBeGreaterThanOrEqual("90 123 45 67".length);
    }
    await user.type(input, "901234567");
    expect(input).toHaveValue("90 123 45 67");
  });

  it("keeps submit enabled and validates phone on submit", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();
    const button = screen.getByRole("button", { name: "Kirish" });
    expect(button).not.toBeDisabled();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "90111" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.click(button);
    await waitFor(() =>
      expect(screen.getByRole("alert")).toBeInTheDocument()
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("logs in with phone+password and navigates to dashboard", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Kirish" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/login",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123" }),
      })
    );
  });

  it("shows a translated error and does not navigate on invalid credentials", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: { code: "invalid_credentials" } }), { status: 401 })
      )
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Kirish" }));

    await waitFor(() =>
      expect(screen.getByText("Telefon raqam yoki parol noto'g'ri")).toBeInTheDocument()
    );
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("explains a passwordless (Telegram-created) account and offers both ways forward", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url === "/api/auth/login") {
        return Promise.resolve(new Response(JSON.stringify({ error: { code: "password_not_set" } }), { status: 409 }));
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({ data: { bot_url: `https://t.me/DriverGouzBot?start=login_${"a".repeat(43)}`, token: "a".repeat(43), expires_in_sec: 300 } }),
          { status: 200 }
        )
      );
    });
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("open", vi.fn(() => null));
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Kirish" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Siz Telegram orqali ro'yxatdan o'tgansiz. Telegram orqali kiring yoki parol o'rnating."
    );
    // «Parol o'rnatish» is the bot reset flow, with the number carried over —
    // in sessionStorage, never in the URL (access logs, history, Referer).
    const setPassword = screen.getByRole("link", { name: "Parol o'rnatish" });
    expect(setPassword).toHaveAttribute("href", "/uz-Latn/forgot-password");
    setPassword.addEventListener("click", (e) => e.preventDefault());
    fireEvent.click(setPassword);
    expect(takeResetPhone()).toBe("901112233");
    expect(takeResetPhone()).toBeNull();
    // «Telegram orqali kirish» starts the Telegram login right away.
    fireEvent.click(within(alert).getByRole("button", { name: "Telegram orqali kirish" }));
    expect(await screen.findByRole("heading", { name: "Telegram'da tasdiqlang" })).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith("/api/auth/telegram-login/start", { method: "POST" });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("offers Telegram login above the password form on the website only", () => {
    const { unmount } = renderWithIntl();
    expect(screen.getByRole("button", { name: "Telegram orqali kirish" })).toBeInTheDocument();
    expect(screen.getByText("yoki telefon raqam va parol bilan")).toBeInTheDocument();
    unmount();
    currentStatus = "ready";
    currentWebApp = { initData: "signed", isVersionAtLeast: () => true } as unknown as TelegramWebApp;
    renderWithIntl();
    expect(screen.queryByRole("button", { name: "Telegram orqali kirish" })).toBeNull();
  });

  it("shows a full-size register CTA and a forgot-password link", () => {
    renderWithIntl();
    expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute(
      "href",
      "/uz-Latn/register"
    );
    expect(screen.getByRole("link", { name: "Parolni unutdingizmi?" })).toHaveAttribute(
      "href",
      "/uz-Latn/forgot-password"
    );
  });

  describe("inside the Telegram Mini App", () => {
    const webApp = () =>
      ({
        initData: "signed",
        isVersionAtLeast: () => true,
        requestContact: vi.fn((cb: (ok: boolean, r: unknown) => void) =>
          cb(true, { response: "contact=signed", responseUnsafe: { contact: { phone_number: "+998901112233" } } }),
        ),
      }) as unknown as TelegramWebApp;
    // Each call gets a fresh Response: a body can only be read once.
    const replies = (...bodies: unknown[]) => {
      let i = 0;
      return vi.fn(
        async (_url?: string, _init?: RequestInit) =>
          new Response(JSON.stringify(bodies[Math.min(i++, bodies.length - 1)]), { status: 200 }),
      );
    };

    async function submit(fetchMock: ReturnType<typeof vi.fn>) {
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      fireEvent.click(screen.getByRole("button", { name: "Telegram raqamini olish" }));
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
    }

    it("shows no Telegram button on the website", () => {
      renderWithIntl();
      expect(screen.queryByRole("button", { name: "Telegram raqamini olish" })).toBeNull();
    });

    it("pre-fills the phone, sends tg_init_data and re-enables auto-login after a link", async () => {
      currentWebApp = webApp();
      const fetchMock = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ data: { ok: true, telegram_linked: true } }), { status: 200 }),
      );
      await submit(fetchMock);
      const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
      expect(body.phone).toBe("901112233");
      expect(body.tg_init_data).toBe("signed");
      expect(body.tg_contact).toBe("contact=signed");
      expect(cloudRemove).toHaveBeenCalledWith("autologin_off");
      // Linked already: nothing more to ask.
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it("drops the shared contact when the phone is edited afterwards", async () => {
      currentWebApp = webApp();
      const fetchMock = replies({ data: { ok: true, telegram_linked: false } }, { data: { linked: false } });
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      fireEvent.click(screen.getByRole("button", { name: "Telegram raqamini olish" }));
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112244" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
      expect(body.phone).toBe("901112244");
      expect(body.tg_contact).toBeUndefined();
    });

    // Typed the number instead of sharing it: the server cannot link without
    // Telegram's signature, so Telegram's sheet is offered once afterwards.
    it("asks Telegram for the number after a typed-phone sign-in and links with it", async () => {
      const app = webApp();
      currentWebApp = app;
      const fetchMock = replies({ data: { ok: true, telegram_linked: false } }, { data: { linked: true } });
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
      expect(body.tg_init_data).toBe("signed");
      expect(body.tg_contact).toBeUndefined();
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
      expect(app.requestContact).toHaveBeenCalledTimes(1);
      const [url, init] = fetchMock.mock.calls[1] as unknown as [string, RequestInit];
      expect(url).toBe("/api/proxy/me/telegram/link-webapp");
      expect(JSON.parse(init.body as string)).toEqual({ init_data: "signed", contact: "contact=signed" });
      await waitFor(() => expect(cloudRemove).toHaveBeenCalledWith("autologin_off"));
    });

    // A shared number that still did not link (not the profile's phone) is
    // the answer already; asking again would only repeat it.
    it("does not ask again after a shared number that did not link", async () => {
      const app = webApp();
      currentWebApp = app;
      const fetchMock = replies({ data: { ok: true, telegram_linked: false } });
      await submit(fetchMock);
      expect(app.requestContact).toHaveBeenCalledTimes(1);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it("still succeeds quietly when linking was skipped", async () => {
      currentWebApp = webApp();
      const fetchMock = replies({ data: { ok: true } });
      await submit(fetchMock);
      expect(cloudRemove).not.toHaveBeenCalled();
      expect(screen.queryByRole("alert")).toBeNull();
    });

    it("does not wait for CloudStorage before navigating", async () => {
      currentWebApp = webApp();
      cloudRemove.mockReturnValueOnce(new Promise(() => {}));
      const fetchMock = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ data: { telegram_linked: true } }), { status: 200 }),
      );
      await submit(fetchMock);
    });
  });

  describe("while the Telegram SDK is not ready", () => {
    function fill() {
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    }

    it("disables submit, announces the wait and blocks Enter-key submits while loading", () => {
      currentStatus = "loading";
      const fetchMock = vi.fn();
      vi.stubGlobal("fetch", fetchMock);
      const { container } = renderWithIntl();
      fill();
      expect(screen.getByRole("button", { name: "Kirish" })).toBeDisabled();
      expect(screen.getByRole("status")).toHaveTextContent("Telegram bilan ulanmoqda…");
      fireEvent.submit(container.querySelector("form")!);
      expect(fetchMock).not.toHaveBeenCalled();
    });

    it("lets the learner sign in as on the website once the SDK failed", async () => {
      currentStatus = "failed";
      const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      fill();
      expect(screen.queryByRole("status")).toBeNull();
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
      expect(body.tg_init_data).toBeUndefined();
    });
  });

  // Audit-2 I1/I3: inside the Mini App the form is not a page of the website.
  describe("inside the Mini App", () => {
    const miniApp = () =>
      ({
        initData: "signed",
        initDataUnsafe: { user: { id: 1 } },
        isVersionAtLeast: () => true,
        requestContact: vi.fn(),
      }) as unknown as TelegramWebApp;

    afterEach(() => {
      window.history.replaceState(null, "", "/");
      sessionStorage.clear();
    });

    it("has no way back to the landing page", () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      renderWithIntl();
      expect(screen.queryByRole("link", { name: /Bosh sahifaga qaytish/ })).toBeNull();
      expect(screen.queryByRole("link", { name: /Driver Go/ })).toBeNull();
      expect(screen.getByText("Driver Go")).toBeInTheDocument();
    });

    it("hides the landing link while the SDK is still loading, too", () => {
      currentStatus = "loading";
      renderWithIntl();
      expect(screen.queryByRole("link", { name: /Bosh sahifaga qaytish/ })).toBeNull();
    });

    // The post-sign-in redirect honours next only with a live WebApp, so
    // the cross link must not promise it in any other state.
    it.each(["loading", "failed"] as const)("does not carry next while the SDK is %s", async (status) => {
      currentStatus = status;
      currentWebApp = null;
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2Fuz-Latn%2Fsigns");
      renderWithIntl();
      await act(async () => {});
      expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register");
    });

    it("carries next over to the register page", async () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2Fuz-Latn%2Fsigns");
      renderWithIntl();
      await waitFor(() =>
        expect(screen.getByRole("link", { name: "Ro'yxatdan o'tish" })).toHaveAttribute("href", "/uz-Latn/register?next=%2Fuz-Latn%2Fsigns"),
      );
    });

    it("keeps both on the website", () => {
      renderWithIntl();
      expect(screen.getByRole("link", { name: /Bosh sahifaga qaytish/ })).toHaveAttribute("href", "/uz-Latn");
      expect(screen.getByRole("link", { name: /Driver Go/ })).toHaveAttribute("href", "/uz-Latn");
    });

    it("continues to a safe next after signing in", async () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2Fuz-Latn%2Fpremium%3Fplan%3Dvip");
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { telegram_linked: true } }), { status: 200 })));
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/premium?plan=vip"));
    });

    it("never follows a next to another site", async () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2F%2Fevil.com");
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { telegram_linked: true } }), { status: 200 })));
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    });

    it("forgets /tg's need_phone verdict once signed in", async () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      sessionStorage.setItem("tg-need-phone:1", JSON.stringify({ firstName: "Ali" }));
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { telegram_linked: true } }), { status: 200 })));
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      expect(sessionStorage.getItem("tg-need-phone:1")).toBeNull();
    });

    it("still sends a must-change-password sign-in to change-password, whatever next says", async () => {
      currentStatus = "ready";
      currentWebApp = miniApp();
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2Fuz-Latn%2Fpremium");
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { must_change_password: true } }), { status: 200 })),
      );
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/change-password"));
    });

    it("ignores next on the website, as before", async () => {
      window.history.replaceState(null, "", "/uz-Latn/login?next=%2Fuz-Latn%2Fpremium");
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200 })));
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    });

    it("explains Telegram's phone sheet before it opens after a typed-phone sign-in", async () => {
      currentStatus = "ready";
      const app = miniApp();
      let hint: string | null = null;
      (app as unknown as { requestContact: unknown }).requestContact = vi.fn(() => {
        hint = currentTelegramHint()?.text ?? null;
      });
      currentWebApp = app;
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200 })));
      renderWithIntl();
      fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(hint).toBe(messages.TelegramApp.shareAfterLogin));
    });

  });
});

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import messages from "../../../../../messages/uz-Latn.json";
import LoginPage from "./page";

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

  it("does not expose the removed set-password flow", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: { code: "password_not_set" } }), { status: 409 })
      )
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Kirish" }));

    await waitFor(() =>
      expect(screen.getByText("Parol o'rnatilmagan. Pastdagi parolni tiklash orqali yangi parol qo'ying.")).toBeInTheDocument()
    );
    expect(screen.getByRole("heading", { name: "Kirish" })).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(1);
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
        requestContact: (cb: (ok: boolean, r: unknown) => void) =>
          cb(true, { responseUnsafe: { contact: { phone_number: "+998901112233" } } }),
      }) as unknown as TelegramWebApp;

    async function submit(fetchMock: ReturnType<typeof vi.fn>) {
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      fireEvent.click(screen.getByRole("button", { name: "Raqamni Telegram'dan olish" }));
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Kirish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
    }

    it("shows no Telegram button on the website", () => {
      renderWithIntl();
      expect(screen.queryByRole("button", { name: "Raqamni Telegram'dan olish" })).toBeNull();
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
      expect(cloudRemove).toHaveBeenCalledWith("autologin_off");
    });

    it("still succeeds quietly when linking was skipped", async () => {
      currentWebApp = webApp();
      const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
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
});

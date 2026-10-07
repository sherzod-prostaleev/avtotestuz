import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import messages from "../../../../../messages/uz-Latn.json";
import RegisterPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/register",
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
      <RegisterPage />
    </NextIntlClientProvider>
  );
}

describe("RegisterPage", () => {
  it("registers with phone + password and navigates to dashboard", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { ok: true } }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();

    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Ism (ixtiyoriy)"), { target: { value: "Ali" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/register",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali" }),
      })
    );
    expect(screen.queryByText("Tasdiqlash kodini yuborish")).toBeNull();
  });

  it("shows mismatch error without calling the API", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();

    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret999" } });
    fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));

    await waitFor(() => expect(screen.getByText("Parollar mos kelmadi")).toBeInTheDocument());
    expect(fetchMock).not.toHaveBeenCalled();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("lets the user type all 9 national digits after +998 with grouping spaces", async () => {
    const user = userEvent.setup();
    renderWithIntl();

    const input = screen.getByLabelText("Telefon raqam");
    // Formatted value is "90 123 45 67" (12 chars). maxLength=9 would stop at 7 digits.
    expect(Number(input.getAttribute("maxLength"))).toBeGreaterThanOrEqual("90 123 45 67".length);
    await user.type(input, "901234567");

    expect(input).toHaveValue("90 123 45 67");
  });

  it("keeps the submit button clickable and validates phone on submit", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();

    const submit = screen.getByRole("button", { name: "Ro'yxatdan o'tish" });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);
    await waitFor(() =>
      expect(screen.getByText("Telefon raqam noto'g'ri formatda")).toBeInTheDocument(),
    );
    expect(fetchMock).not.toHaveBeenCalled();
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
      fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Raqamni Telegram'dan olish" }));
      fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
      fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));
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
      fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret123" } });
    }

    it("disables submit, announces the wait and blocks Enter-key submits while loading", () => {
      currentStatus = "loading";
      const fetchMock = vi.fn();
      vi.stubGlobal("fetch", fetchMock);
      const { container } = renderWithIntl();
      fill();
      expect(screen.getByRole("button", { name: "Ro'yxatdan o'tish" })).toBeDisabled();
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
      fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));
      await waitFor(() => expect(pushMock).toHaveBeenCalled());
      const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
      expect(body.tg_init_data).toBeUndefined();
    });
  });
});

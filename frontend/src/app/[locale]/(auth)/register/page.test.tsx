import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import RegisterPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/lib/referral-storage", () => ({
  capturePendingReferralCodeFromUrl: vi.fn(),
  applyPendingReferralCode: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("@/lib/demo-progress-storage", () => ({
  migrateDemoProgressOnLogin: vi.fn().mockResolvedValue(undefined),
}));

afterEach(() => {
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
  it("verifies the phone before registration and navigates to dashboard", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { channel: "sandbox" } }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { ok: true } }), { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();

    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Tasdiqlash kodini yuborish" }));
    await waitFor(() => expect(screen.getByLabelText("Tasdiqlash kodi")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("Tasdiqlash kodi"), { target: { value: "123456" } });
    fireEvent.change(screen.getByLabelText("Ism (ixtiyoriy)"), { target: { value: "Ali" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/dashboard"));
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/auth/otp/request",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ phone: "901112233" }) })
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/auth/register",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali", code: "123456" }),
      })
    );
  });

  it("shows mismatch error without calling the API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { channel: "sandbox" } }), { status: 200 })
    );
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();

    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Tasdiqlash kodini yuborish" }));
    await waitFor(() => expect(screen.getByLabelText("Tasdiqlash kodi")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("Tasdiqlash kodi"), { target: { value: "123456" } });
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    fireEvent.change(screen.getByLabelText("Parolni tasdiqlang"), { target: { value: "secret999" } });
    fireEvent.click(screen.getByRole("button", { name: "Ro'yxatdan o'tish" }));

    await waitFor(() => expect(screen.getByText("Parollar mos kelmadi")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(pushMock).not.toHaveBeenCalled();
  });
});

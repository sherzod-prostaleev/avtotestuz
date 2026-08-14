import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ForgotPasswordPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => "/forgot-password",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  pushMock.mockClear();
});

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ForgotPasswordPage />
    </NextIntlClientProvider>
  );
}

describe("ForgotPasswordPage", () => {
  it("starts a telegram reset and navigates with token + bot url", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { bot_url: "https://t.me/AvtoTestBot?start=pwr_tok123", expires_in_sec: 900 },
        }),
        { status: 200 }
      )
    );
    vi.stubGlobal("fetch", fetchMock);
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: /Telegram botni ochish/ }));

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledTimes(1);
      const href = String(pushMock.mock.calls[0][0]);
      expect(href).toContain("/uz-Latn/reset-password?");
      expect(href).toContain("token=tok123");
      expect(href).toContain("exp=900");
      expect(href).toContain("t.me");
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/auth/password-reset/start",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ phone: "901112233" }),
      })
    );
  });
});

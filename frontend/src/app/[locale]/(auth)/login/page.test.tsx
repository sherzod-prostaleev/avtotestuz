import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import LoginPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

afterEach(() => {
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
  it("disables the continue button until 9 digits are entered", () => {
    renderWithIntl();
    const button = screen.getByRole("button", { name: "Davom etish" });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    expect(button).not.toBeDisabled();
  });

  it("requests an OTP and navigates to /login/verify on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { channel: "sandbox" } }), { status: 200 }))
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Davom etish" }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/login/verify?phone=901112233"));
  });

  it("shows a translated error and does not navigate when the backend returns rate_limited", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "rate_limited" } }), { status: 429 }))
    );
    renderWithIntl();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    fireEvent.click(screen.getByRole("button", { name: "Davom etish" }));

    await waitFor(() =>
      expect(screen.getByText("Juda ko'p urinish. Birozdan keyin qayta urinib ko'ring")).toBeInTheDocument()
    );
    expect(pushMock).not.toHaveBeenCalled();
  });
});

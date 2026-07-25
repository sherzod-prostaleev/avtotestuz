import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import LoginPage from "./page";

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

  it("disables submit until phone and password are valid", () => {
    renderWithIntl();
    const button = screen.getByRole("button", { name: "Kirish" });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Telefon raqam"), { target: { value: "901112233" } });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Parol"), { target: { value: "secret123" } });
    expect(button).not.toBeDisabled();
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

  it("offers set-password when backend returns password_not_set", async () => {
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

    await waitFor(() => expect(screen.getByRole("heading", { name: "Parol o'rnatish" })).toBeInTheDocument());
  });
});

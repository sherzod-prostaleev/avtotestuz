import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
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
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ChangePasswordPage from "./page";
import { ApiError, apiGet } from "@/lib/api-client";
import { markTelegramMiniApp } from "@/lib/telegram/web-app";
import { installTelegramHost } from "@/test/telegram-host";

const nav = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: nav.replace, push: nav.push }),
}));
vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return { ...actual, apiGet: vi.fn() };
});
vi.mock("@/components/profile/change-password-form", () => ({
  ChangePasswordForm: () => <form aria-label="change-password-form" />,
}));
vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

function renderPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ChangePasswordPage />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  nav.replace.mockReset();
  vi.mocked(apiGet).mockReset();
});
afterEach(() => sessionStorage.clear());

describe("ChangePasswordPage /me check", () => {
  it("shows the form once /me answers", async () => {
    vi.mocked(apiGet).mockResolvedValue({ profile: { must_change_password: true } });
    renderPage();
    expect(await screen.findByRole("form", { name: "change-password-form" })).toBeInTheDocument();
    expect(nav.replace).not.toHaveBeenCalled();
  });

  it("sends the website to /login on any failure (unchanged)", async () => {
    vi.mocked(apiGet).mockRejectedValue(new ApiError("down", "network_error", 502));
    renderPage();
    await waitFor(() => expect(nav.replace).toHaveBeenCalledWith("/uz-Latn/login"));
  });

  it("sends a Mini App learner to /tg only when the session is gone (401)", async () => {
    installTelegramHost();
    markTelegramMiniApp();
    vi.mocked(apiGet).mockRejectedValue(new ApiError("expired", "unauthorized", 401));
    renderPage();
    await waitFor(() => expect(nav.replace).toHaveBeenCalledWith("/uz-Latn/tg"));
  });

  // /tg would sign the learner straight back in and land here again: a
  // degraded backend must show an error, not loop between the two screens.
  it("shows a retryable error in the Mini App when the backend is degraded", async () => {
    installTelegramHost();
    markTelegramMiniApp();
    vi.mocked(apiGet).mockRejectedValueOnce(new ApiError("down", "network_error", 502));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent(messages.Profile.loadError);
    expect(nav.replace).not.toHaveBeenCalled();
    expect(screen.queryByRole("form", { name: "change-password-form" })).toBeNull();

    vi.mocked(apiGet).mockResolvedValueOnce({ profile: { must_change_password: true } });
    fireEvent.click(screen.getByRole("button", { name: messages.Profile.retry }));
    expect(await screen.findByRole("form", { name: "change-password-form" })).toBeInTheDocument();
    expect(nav.replace).not.toHaveBeenCalled();
  });

  it("shows the error for a plain network failure (not an ApiError) in the Mini App", async () => {
    installTelegramHost();
    markTelegramMiniApp();
    vi.mocked(apiGet).mockRejectedValueOnce(new TypeError("Failed to fetch"));
    renderPage();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(nav.replace).not.toHaveBeenCalled();
  });
});

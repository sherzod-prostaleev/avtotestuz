import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { ReferralCard } from "../referral-card";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ReferralCard />
    </NextIntlClientProvider>
  );
}

describe("ReferralCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders referral code and stats correctly", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      referral_code: "REF123",
      invite_url: "https://avtotest.uz/r/REF123",
      total_invited: 5,
      total_rewarded: 2,
      bonus_days_earned: 14,
    });

    renderWithIntl();

    expect(await screen.findByText("REF123")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("+14")).toBeInTheDocument();
  });

  it("copies the backend-provided invite_url, not a hand-built URL", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      referral_code: "REF123",
      invite_url: "https://avtotest.uz/r/REF123",
      total_invited: 0,
      total_rewarded: 0,
      bonus_days_earned: 0,
    });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });

    renderWithIntl();

    fireEvent.click(await screen.findByRole("button", { name: "Havolani nusxalash" }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("https://avtotest.uz/r/REF123");
    });
  });

  it("handles referral code application", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      referral_code: "REF123",
      invite_url: "https://avtotest.uz/r/REF123",
      total_invited: 0,
      total_rewarded: 0,
      bonus_days_earned: 0,
    });
    const postSpy = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ applied: true });

    renderWithIntl();

    const input = await screen.findByPlaceholderText("Masalan: ABC123");
    fireEvent.change(input, { target: { value: "FRIEND1" } });
    fireEvent.click(screen.getByRole("button", { name: "Biriktirish" }));

    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith("referral/apply", { code: "FRIEND1" });
      expect(screen.getByText("Referal kod muvaffaqiyatli biriktirildi!")).toBeInTheDocument();
    });
  });

  it("shows a localized message (not raw backend English) for a self-referral error", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      referral_code: "REF123",
      invite_url: "https://avtotest.uz/r/REF123",
      total_invited: 0,
      total_rewarded: 0,
      bonus_days_earned: 0,
    });
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("cannot apply your own referral code", "referral_self", 400)
    );

    renderWithIntl();

    const input = await screen.findByPlaceholderText("Masalan: ABC123");
    fireEvent.change(input, { target: { value: "REF123" } });
    fireEvent.click(screen.getByRole("button", { name: "Biriktirish" }));

    await waitFor(() => {
      expect(screen.getByText("O'zingizning referal kodingizni biriktira olmaysiz")).toBeInTheDocument();
      expect(screen.queryByText("cannot apply your own referral code")).not.toBeInTheDocument();
    });
  });
});

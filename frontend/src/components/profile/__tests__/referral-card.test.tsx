import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { ReferralCard, type ReferralActivityResponse, type ReferralResponse } from "../referral-card";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";

const baseStats: ReferralResponse = {
  referral_code: "REF123",
  invite_url: "https://avtotest.uz/r/REF123",
  total_invited: 5,
  total_rewarded: 2,
  earned_uzs: 11980,
  available_balance_uzs: 11980,
  commission_percent: 20,
};

const emptyActivity: ReferralActivityResponse = {
  payout_summary: {
    pending_count: 0,
    pending_uzs: 0,
    paid_count: 0,
    paid_uzs: 0,
    rejected_count: 0,
    rejected_uzs: 0,
  },
  payouts: [],
  earnings: [],
};

function mockReferralApis(stats = baseStats, activity = emptyActivity) {
  return vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    if (path === "me/referral") return stats as never;
    if (path === "me/referral/activity") return activity as never;
    throw new Error(`unexpected apiGet path: ${path}`);
  });
}

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

  it("renders referral code, stats, and monitoring sections", async () => {
    mockReferralApis(baseStats, {
      ...emptyActivity,
      payout_summary: {
        pending_count: 1,
        pending_uzs: 5000,
        paid_count: 1,
        paid_uzs: 6980,
        rejected_count: 0,
        rejected_uzs: 0,
      },
      payouts: [
        {
          id: "p1",
          amount_uzs: 5000,
          card_masked: "**** **** **** 9012",
          card_network: "uzcard",
          status: "pending",
          created_at: "2026-07-20T10:00:00Z",
        },
      ],
      earnings: [
        {
          ledger_id: "e1",
          commission_uzs: 11980,
          payment_amount_uzs: 59900,
          tariff_code: "gentra",
          tariff_days: 30,
          percent_snapshot: 20,
          referee_label: "***1234",
          rewarded_at: "2026-07-19T12:00:00Z",
        },
      ],
    });

    renderWithIntl();

    expect(await screen.findByText("REF123")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("Hisob ochiqligi")).toBeInTheDocument();
    expect(screen.getAllByText("Kutilmoqda").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("To'langan").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("***1234")).toBeInTheDocument();
    expect(screen.getByText(/gentra/)).toBeInTheDocument();
  });

  it("copies the backend-provided invite_url, not a hand-built URL", async () => {
    mockReferralApis({ ...baseStats, total_invited: 0, total_rewarded: 0, earned_uzs: 0, available_balance_uzs: 0 });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });

    renderWithIntl();

    fireEvent.click(await screen.findByRole("button", { name: "Havolani nusxalash" }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("https://avtotest.uz/r/REF123");
    });
  });

  it("handles referral code application", async () => {
    mockReferralApis({ ...baseStats, total_invited: 0, total_rewarded: 0, earned_uzs: 0, available_balance_uzs: 0 });
    const postSpy = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ applied: true });

    renderWithIntl();

    const input = await screen.findByPlaceholderText("Masalan: REF-XXXXXX");
    fireEvent.change(input, { target: { value: "FRIEND1" } });
    fireEvent.click(screen.getByRole("button", { name: "Biriktirish" }));

    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith("referral/apply", { code: "FRIEND1" });
      expect(screen.getByText("Referal kod muvaffaqiyatli biriktirildi!")).toBeInTheDocument();
    });
  });

  it("shows a localized message (not raw backend English) for a self-referral error", async () => {
    mockReferralApis({ ...baseStats, total_invited: 0, total_rewarded: 0, earned_uzs: 0, available_balance_uzs: 0 });
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("cannot apply your own referral code", "referral_self", 400)
    );

    renderWithIntl();

    const input = await screen.findByPlaceholderText("Masalan: REF-XXXXXX");
    fireEvent.change(input, { target: { value: "REF123" } });
    fireEvent.click(screen.getByRole("button", { name: "Biriktirish" }));

    await waitFor(() => {
      expect(screen.getByText("O'zingizning referal kodingizni biriktira olmaysiz")).toBeInTheDocument();
      expect(screen.queryByText("cannot apply your own referral code")).not.toBeInTheDocument();
    });
  });
});

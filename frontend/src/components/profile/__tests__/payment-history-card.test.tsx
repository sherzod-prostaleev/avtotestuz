import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { PaymentHistoryCard } from "../payment-history-card";
import * as apiClient from "@/lib/api-client";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PaymentHistoryCard />
    </NextIntlClientProvider>
  );
}

describe("PaymentHistoryCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders empty state when no payments exist", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([]);

    renderWithIntl();

    expect(await screen.findByText("Hali to'lovlar amalga oshirilmagan.")).toBeInTheDocument();
  });

  it("renders list of payments", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([
      {
        id: "p1",
        amount_uzs: 59900,
        provider: "payme",
        status: "paid",
        created_at: "2026-07-24T12:00:00Z",
        paid_at: "2026-07-24T12:01:00Z",
        tariff_code: "gentra",
        tariff_days: 30,
        tariff_name: "Gentra",
      },
    ]);

    renderWithIntl();

    expect(await screen.findByText(/Gentra/)).toBeInTheDocument();
    expect(screen.getByText(/payme/i)).toBeInTheDocument();
    expect(screen.getByText("To'langan")).toBeInTheDocument();
  });
});

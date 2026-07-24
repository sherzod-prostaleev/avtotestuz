import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../messages/uz-Latn.json";
import { PromoInput } from "../promo-input";
import * as apiClient from "@/lib/api-client";

vi.mock("@/lib/api-client", () => ({
  apiPost: vi.fn(),
}));

describe("PromoInput", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("applies promo code successfully and invokes onApplied", async () => {
    const handleApplied = vi.fn();
    const mockRes = {
      promo_id: "p1",
      code: "TEST20",
      kind: "percent",
      value: 20,
      discount_uzs: 11980,
      original_amount_uzs: 59900,
      final_amount_uzs: 47920,
      bonus_days: 0,
    };
    vi.mocked(apiClient.apiPost).mockResolvedValueOnce(mockRes);

    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <PromoInput tariffCode="gentra" onApplied={handleApplied} />
      </NextIntlClientProvider>
    );

    const input = screen.getByPlaceholderText(/PROMO2026/i);
    const applyBtn = screen.getByRole("button", { name: /Qo'llash/i });

    fireEvent.change(input, { target: { value: "test20" } });
    fireEvent.click(applyBtn);

    await waitFor(() => {
      expect(apiClient.apiPost).toHaveBeenCalledWith("billing/promo/validate", {
        code: "TEST20",
        tariff_code: "gentra",
      });
      expect(handleApplied).toHaveBeenCalledWith(mockRes);
      expect(screen.getByText(/Promo-kod qo'llanildi!/i)).toBeInTheDocument();
    });
  });

  it("handles invalid promo code error", async () => {
    const handleApplied = vi.fn();
    vi.mocked(apiClient.apiPost).mockRejectedValueOnce({ code: "promo_not_found" });

    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <PromoInput tariffCode="gentra" onApplied={handleApplied} />
      </NextIntlClientProvider>
    );

    const input = screen.getByPlaceholderText(/PROMO2026/i);
    const applyBtn = screen.getByRole("button", { name: /Qo'llash/i });

    fireEvent.change(input, { target: { value: "BADCODE" } });
    fireEvent.click(applyBtn);

    await waitFor(() => {
      expect(handleApplied).toHaveBeenCalledWith(null);
      expect(screen.getByRole("alert")).toHaveTextContent(/Promo-kod topilmadi/i);
    });
  });
});

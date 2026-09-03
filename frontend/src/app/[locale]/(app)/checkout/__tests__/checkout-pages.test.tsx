import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../../../messages/uz-Latn.json";
import CheckoutSuccessPage from "../success/page";
import CheckoutFailurePage from "../failure/page";
import CheckoutPendingPage from "../pending/page";
import * as apiClient from "@/lib/api-client";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
    replace: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams("free=true"),
}));

vi.mock("@/lib/api-client", () => ({
  apiGet: vi.fn(),
}));

function renderWithIntl(component: React.ReactNode) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {component}
    </NextIntlClientProvider>
  );
}

describe("Checkout Status Pages", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // The page renders two bodies — the phone one (`md:hidden`) and the wide
  // card (`max-md:hidden`). jsdom applies no CSS, so both are in the DOM.
  it("renders CheckoutSuccessPage with title and practice button", () => {
    renderWithIntl(<CheckoutSuccessPage />);
    expect(screen.getAllByText("To'lov muvaffaqiyatli o'tdi!")).toHaveLength(2);
    expect(screen.getAllByText("Mashqlarni boshlash")).toHaveLength(2);
  });

  // Without a plan in the query string the summary card must not appear at
  // all — an empty "Tarif —" row would be a guess dressed as a fact.
  it("omits the result summary when the redirect carried no plan", () => {
    renderWithIntl(<CheckoutSuccessPage />);
    expect(screen.queryByText("Tarif")).not.toBeInTheDocument();
    expect(screen.queryByText("Amal qiladi")).not.toBeInTheDocument();
  });

  it("renders CheckoutFailurePage with try again button", () => {
    renderWithIntl(<CheckoutFailurePage />);
    expect(screen.getByText("To'lov amalga oshmadi")).toBeInTheDocument();
    expect(screen.getByText("Qayta urinish")).toBeInTheDocument();
  });

  it("renders CheckoutPendingPage and polls entitlement status", async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValueOnce({ active: true, until: "2026-08-24T00:00:00Z" });
    renderWithIntl(<CheckoutPendingPage />);
    expect(screen.getByText("To'lov kutilmoqda...")).toBeInTheDocument();

    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalledWith("me/entitlement");
      expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/success");
    });
  });

  it("forwards proration details to the success page", async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValueOnce({
      active: true,
      until: "2026-08-24T00:00:00Z",
      proration: { applied: true, granted_days: 12, tariff_days: 30, reason: "promo_limit_reached" },
    });
    renderWithIntl(<CheckoutPendingPage />);

    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith(
        "/uz-Latn/checkout/success?prorated=1&granted=12&tariff=30"
      );
    });
  });
});

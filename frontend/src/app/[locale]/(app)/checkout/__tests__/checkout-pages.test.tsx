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

  it("renders CheckoutSuccessPage with title and practice button", () => {
    renderWithIntl(<CheckoutSuccessPage />);
    expect(screen.getByText("To'lov muvaffaqiyatli o'tdi!")).toBeInTheDocument();
    expect(screen.getByText("Mashqlarni boshlash")).toBeInTheDocument();
  });

  it("renders CheckoutFailurePage with try again button", () => {
    renderWithIntl(<CheckoutFailurePage />);
    expect(screen.getByText("To'lov amalga oshmadi")).toBeInTheDocument();
    expect(screen.getByText("Qayta urinish")).toBeInTheDocument();
  });

  it("renders CheckoutPendingPage and polls entitlement status", async () => {
    vi.mocked(apiClient.apiGet).mockResolvedValueOnce({ active: true, until: "2026-08-24T00:00:00Z" });
    renderWithIntl(<CheckoutPendingPage />);
    expect(screen.getByText("To me to'lov kutilmoqda...")).toBeInTheDocument();

    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalledWith("me/entitlement");
      expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/success");
    });
  });
});

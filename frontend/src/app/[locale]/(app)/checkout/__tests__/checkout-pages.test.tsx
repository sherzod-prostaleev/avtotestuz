import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../../../messages/uz-Latn.json";
import CheckoutSuccessPage from "../success/page";
import CheckoutFailurePage from "../failure/page";
import CheckoutPendingPage from "../pending/page";
import * as apiClient from "@/lib/api-client";
import { CHECKOUT_URL_KEY, rememberCheckoutUrl } from "@/lib/telegram/checkout-handoff";
import { installTelegramHost } from "@/test/telegram-host";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

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
    tg.webApp = null;
    delete (window as { Telegram?: unknown }).Telegram;
    sessionStorage.clear();
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

  // The hand-off after the checkout call can be popup-blocked inside
  // Telegram; the pending screen must let the learner open it again from a
  // real tap.
  describe("pending screen inside the Mini App", () => {
    const CHECKOUT = "https://checkout.paycom.uz/abc";
    function enterMiniApp() {
      const webApp = { initData: "x", openLink: vi.fn(), openTelegramLink: vi.fn() };
      tg.webApp = webApp;
      installTelegramHost();
      (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
      return webApp;
    }

    it("reopens the stored checkout page through Telegram from the click", async () => {
      const webApp = enterMiniApp();
      rememberCheckoutUrl(CHECKOUT);
      vi.mocked(apiClient.apiGet).mockResolvedValue({ active: false, until: null });
      renderWithIntl(<CheckoutPendingPage />);

      fireEvent.click(await screen.findByRole("button", { name: "To'lov sahifasini ochish" }));
      expect(webApp.openLink).toHaveBeenCalledWith(CHECKOUT);
    });

    it("offers no reopen button without a stored URL", async () => {
      enterMiniApp();
      vi.mocked(apiClient.apiGet).mockResolvedValue({ active: false, until: null });
      renderWithIntl(<CheckoutPendingPage />);
      await waitFor(() => expect(apiClient.apiGet).toHaveBeenCalled());
      expect(screen.queryByRole("button", { name: "To'lov sahifasini ochish" })).toBeNull();
    });

    it("ignores a stored URL that is not http(s)", async () => {
      enterMiniApp();
      sessionStorage.setItem(CHECKOUT_URL_KEY, JSON.stringify({ url: "javascript:alert(1)", at: Date.now() }));
      vi.mocked(apiClient.apiGet).mockResolvedValue({ active: false, until: null });
      renderWithIntl(<CheckoutPendingPage />);
      await waitFor(() => expect(apiClient.apiGet).toHaveBeenCalled());
      expect(screen.queryByRole("button", { name: "To'lov sahifasini ochish" })).toBeNull();
    });

    it("forgets the URL once the payment is confirmed", async () => {
      enterMiniApp();
      rememberCheckoutUrl(CHECKOUT);
      vi.mocked(apiClient.apiGet).mockResolvedValueOnce({ active: true, until: "2026-08-24T00:00:00Z" });
      renderWithIntl(<CheckoutPendingPage />);
      await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/success"));
      expect(sessionStorage.getItem(CHECKOUT_URL_KEY)).toBeNull();
    });

    it("ignores a stored URL older than 30 minutes", async () => {
      enterMiniApp();
      sessionStorage.setItem(
        CHECKOUT_URL_KEY,
        JSON.stringify({ url: CHECKOUT, at: Date.now() - 31 * 60 * 1000 }),
      );
      vi.mocked(apiClient.apiGet).mockResolvedValue({ active: false, until: null });
      renderWithIntl(<CheckoutPendingPage />);
      await waitFor(() => expect(apiClient.apiGet).toHaveBeenCalled());
      expect(screen.queryByRole("button", { name: "To'lov sahifasini ochish" })).toBeNull();
      expect(sessionStorage.getItem(CHECKOUT_URL_KEY)).toBeNull();
    });

    it("shows no reopen button on the website", async () => {
      rememberCheckoutUrl(CHECKOUT);
      vi.mocked(apiClient.apiGet).mockResolvedValue({ active: false, until: null });
      renderWithIntl(<CheckoutPendingPage />);
      await waitFor(() => expect(apiClient.apiGet).toHaveBeenCalled());
      expect(screen.queryByRole("button", { name: "To'lov sahifasini ochish" })).toBeNull();
    });
  });
});

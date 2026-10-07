import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { CHECKOUT_URL_KEY, readCheckoutUrl, rememberCheckoutUrl } from "@/lib/telegram/checkout-handoff";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import PremiumPage from "./page";
import * as apiClient from "@/lib/api-client";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
    replace: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

const tariffs = [
  { code: "nexia", days: 7, price_uzs: 24900, old_price_uzs: 34900, price_per_day_uzs: 3557, discount_percent: 29, badge: null, name: "Nexia", description: "1 haftalik" },
  { code: "gentra", days: 30, price_uzs: 59900, old_price_uzs: 99900, price_per_day_uzs: 1997, discount_percent: 40, badge: "popular", name: "Gentra", description: "1 oylik" },
];

function mockApiGet(entitlement: { active: boolean; until: string | null }) {
  vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    if (path === "tariffs?locale=uz-Latn") return tariffs as never;
    if (path === "me/entitlement") return entitlement as never;
    throw new Error(`unexpected path ${path}`);
  });
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PremiumPage />
    </NextIntlClientProvider>
  );
}

describe("PremiumPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
    delete (window as { Telegram?: unknown }).Telegram;
    sessionStorage.clear();
  });

  it("renders every API tariff with pricing and badges", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    // Two bodies render — the phone one (`md:hidden`) and the wide grid
    // (`max-md:hidden`). jsdom applies no CSS, so both are in the DOM and each
    // plan name and badge appears twice.
    expect(await screen.findAllByText("Nexia")).toHaveLength(2);
    expect(screen.getAllByText("Gentra").length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText("Ommabop")).toHaveLength(2); // gentra's popular badge, translated
    expect(screen.getByText("−40%")).toBeInTheDocument(); // wide card only
  });

  // The free tier is where a visitor to this page already is, not a fourth
  // thing to weigh up on the screen where they came to pay.
  it("never offers the free Matiz tier among the plans", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    await screen.findAllByText("Nexia");
    expect(screen.queryByText("Matiz")).not.toBeInTheDocument();
    expect(screen.queryByText("Hozirgi tarifingiz")).not.toBeInTheDocument();
  });

  // Checkout asks for nothing but the plan: referral credit rides in on the
  // invite link, and the promo field was one more wall in front of a payment.
  it("asks for no promo or referral code anywhere", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    await screen.findAllByText("Nexia");
    expect(screen.queryByPlaceholderText(/PROMO/i)).not.toBeInTheDocument();
    expect(screen.queryByText("Qo'llash")).not.toBeInTheDocument();
    expect(screen.queryByText(/referal kod/i)).not.toBeInTheDocument();
  });

  it("does not show the VIP banner when entitlement is inactive", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    await screen.findAllByText("Nexia");
    expect(screen.queryByText(/VIP faol/)).not.toBeInTheDocument();
  });

  // Twice: the wide layout keeps its own banner and the phone body renders one
  // inside its scroller, because anything stacked above that box on a phone is
  // height taken from the buy button.
  it("shows the VIP banner in both bodies when entitlement is active", async () => {
    mockApiGet({ active: true, until: "2026-08-24T00:00:00Z" });
    renderWithIntl();
    expect(await screen.findAllByText(/VIP faol/)).toHaveLength(2);
  });

  it("calls POST /me/checkout with the tariff code and redirects on buy", async () => {
    // A hand-off URL left by an earlier checkout must not survive a new one.
    rememberCheckoutUrl("https://checkout.paycom.uz/stale");
    mockApiGet({ active: false, until: null });
    const postSpy = vi.spyOn(apiClient, "apiPost").mockResolvedValue({
      payment_id: "p1",
      manual: {
        payment_id: "p1",
        amount_uzs: 24900,
        pan_full: "9860123456784042",
        pan_last4: "4042",
        holder_name: "TEST",
        network: "humo",
        hold_until: new Date().toISOString(),
        manual_state: "awaiting_transfer",
      },
    } as never);

    renderWithIntl();
    // Default selection is popular tariff (Gentra); checkout lives in the shared panel.
    const buyButtons = await screen.findAllByText("Sotib olish");
    fireEvent.click(buyButtons[0]);

    await waitFor(() =>
      expect(postSpy).toHaveBeenCalledWith("me/checkout?locale=uz-Latn", {
        tariff_code: "gentra",
        provider: "manual",
      })
    );
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/manual?payment_id=p1")
    );
    expect(sessionStorage.getItem(CHECKOUT_URL_KEY)).toBeNull();
  });

  // The phone used to stop at a summary screen with a promo field before the
  // card details. One tap, one checkout call, straight to the card.
  it("takes the phone CTA straight to the card screen", async () => {
    mockApiGet({ active: false, until: null });
    const postSpy = vi.spyOn(apiClient, "apiPost").mockResolvedValue({
      payment_id: "p9",
      manual: {
        payment_id: "p9",
        amount_uzs: 59900,
        pan_full: "9860246603626754",
        pan_last4: "6754",
        holder_name: "TEST",
        network: "humo",
        hold_until: new Date().toISOString(),
        manual_state: "awaiting_transfer",
      },
    } as never);

    renderWithIntl();
    fireEvent.click(await screen.findByText("Sotib olish — Gentra"));

    await waitFor(() =>
      expect(postSpy).toHaveBeenCalledWith("me/checkout?locale=uz-Latn", {
        tariff_code: "gentra",
        provider: "manual",
      })
    );
    await waitFor(() =>
      expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/manual?payment_id=p9")
    );
  });

  it("shows a retry button when the initial load fails", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(new Error("network"));
    renderWithIntl();
    expect(await screen.findByText("Tariflarni yuklab bo'lmadi.")).toBeInTheDocument();
    expect(screen.getByText("Qayta urinish")).toBeInTheDocument();
  });

  it("renders a mobile sticky CTA for the popular tariff", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    expect(await screen.findByText("Sotib olish — Gentra")).toBeInTheDocument();
  });

  // Selecting a tariff used to show two pay buttons at once on phones: the
  // card CTA had no responsive class, so it rendered alongside the sticky
  // bottom bar. They must be mutually exclusive — card on >=sm, sticky
  // below it. jsdom does not evaluate media queries, so the DOM contains
  // both regardless; the responsive classes are the actual mechanism and
  // therefore what this pins.
  it("shows only one pay CTA per viewport: card on desktop, sticky on mobile", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();

    // The boundary moved from `sm` to `md`: the phone body owns everything
    // below 768px now, including its own bottom CTA, so the 640-767px band
    // must show the phone CTA rather than the wide card's.
    const cardCta = await screen.findByText("Sotib olish");
    expect(cardCta).toHaveClass("hidden");
    expect(cardCta).toHaveClass("md:inline-flex");

    const phoneCta = screen.getByText("Sotib olish — Gentra");
    expect(phoneCta.closest("div.md\\:hidden")).not.toBeNull();
  });

  // Payme/Click refuse to be framed on Telegram Web and would replace the Mini
  // App on phones; inside Telegram the hand-off opens in Telegram's browser
  // and the app waits on the pending screen, which polls the entitlement.
  describe("hosted checkout inside the Mini App", () => {
    const CHECKOUT = "https://checkout.paycom.uz/abc";
    function enterMiniApp() {
      const webApp = { initData: "x", openLink: vi.fn(), openTelegramLink: vi.fn() };
      (window as { Telegram?: unknown }).Telegram = { WebApp: webApp };
      return webApp;
    }

    it("opens the provider page through Telegram and waits on the pending screen", async () => {
      const webApp = enterMiniApp();
      mockApiGet({ active: false, until: null });
      vi.spyOn(apiClient, "apiPost").mockResolvedValue({ payment_id: "p1", checkout_url: CHECKOUT } as never);
      const before = window.location.href;

      renderWithIntl();
      fireEvent.click((await screen.findAllByText("Sotib olish"))[0]);

      await waitFor(() => expect(webApp.openLink).toHaveBeenCalledWith(CHECKOUT));
      expect(pushMock).toHaveBeenCalledWith("/uz-Latn/checkout/pending");
      expect(window.location.href).toBe(before);
    });

    // The provider returns the payer in an external browser without our
    // session; return_context makes the backend send them to the public
    // /checkout/done page instead of the session-gated pending screen. The
    // URL is kept so the pending screen can reopen a popup-blocked hand-off.
    it("asks for the Telegram return page and keeps the checkout URL for the pending screen", async () => {
      enterMiniApp();
      mockApiGet({ active: false, until: null });
      const postSpy = vi
        .spyOn(apiClient, "apiPost")
        .mockResolvedValue({ payment_id: "p1", checkout_url: CHECKOUT } as never);

      renderWithIntl();
      fireEvent.click((await screen.findAllByText("Sotib olish"))[0]);

      await waitFor(() =>
        expect(postSpy).toHaveBeenCalledWith(
          "me/checkout?locale=uz-Latn",
          expect.objectContaining({ return_context: "telegram" }),
        ),
      );
      await waitFor(() => expect(readCheckoutUrl()).toBe(CHECKOUT));
    });

    // The pending screen calls an already-active VIP "paid" at once; a
    // renewal that has not been paid yet must not be congratulated.
    it("stays on the plans for a VIP renewing, with the button usable again", async () => {
      const webApp = enterMiniApp();
      mockApiGet({ active: true, until: "2026-12-31T00:00:00Z" });
      vi.spyOn(apiClient, "apiPost").mockResolvedValue({ payment_id: "p1", checkout_url: CHECKOUT } as never);

      renderWithIntl();
      fireEvent.click((await screen.findAllByText("Sotib olish"))[0]);

      await waitFor(() => expect(webApp.openLink).toHaveBeenCalledWith(CHECKOUT));
      expect(pushMock).not.toHaveBeenCalled();
      await waitFor(() => expect(screen.getAllByText("Sotib olish")[0]).toBeInTheDocument());
    });
  });
});

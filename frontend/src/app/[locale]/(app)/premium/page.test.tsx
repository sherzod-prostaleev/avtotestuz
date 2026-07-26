import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  });

  it("renders Matiz + all API tariffs with pricing and badges", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    expect(await screen.findByText("Matiz")).toBeInTheDocument();
    expect(screen.getByText("Nexia")).toBeInTheDocument();
    expect(screen.getAllByText("Gentra").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Ommabop")).toBeInTheDocument(); // gentra's popular badge, translated
    expect(screen.getByText("−40%")).toBeInTheDocument(); // gentra's discount_percent
  });

  it("does not show the VIP banner when entitlement is inactive", async () => {
    mockApiGet({ active: false, until: null });
    renderWithIntl();
    await screen.findByText("Nexia");
    expect(screen.queryByText(/VIP faol/)).not.toBeInTheDocument();
  });

  it("shows the VIP banner when entitlement is active", async () => {
    mockApiGet({ active: true, until: "2026-08-24T00:00:00Z" });
    renderWithIntl();
    expect(await screen.findByText(/VIP faol/)).toBeInTheDocument();
  });

  it("calls POST /me/checkout with the tariff code and redirects on buy", async () => {
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
});

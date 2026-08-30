import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import NarxlarPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

describe("NarxlarPage", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(
        async () =>
          new Response(
          JSON.stringify({
            data: [
              {
                code: "vip_30",
                days: 30,
                price_uzs: 49990,
                old_price_uzs: null,
                price_per_day_uzs: 1666,
                discount_percent: 0,
                badge: null,
                name: "30 kun",
                description: "Test tarif",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        )
      )
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders public pricing shell and fetched tariff", async () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <NarxlarPage />
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("heading", { level: 1, name: "Driver Go tariflari" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 3, name: "30 kun" })).toBeInTheDocument();
    });
    expect(screen.getByText(/49 990/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ofertani o'qish" })).toHaveAttribute(
      "href",
      "/uz-Latn/oferta"
    );
  });
});

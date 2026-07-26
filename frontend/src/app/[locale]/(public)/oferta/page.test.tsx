import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import OfertaPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const apiGet = vi.fn();
vi.mock("@/lib/api-client", () => ({
  apiGet: (...args: unknown[]) => apiGet(...args),
}));

describe("OfertaPage", () => {
  beforeEach(() => {
    apiGet.mockReset();
  });

  it("falls back to i18n sections when CMS oferta is empty", async () => {
    apiGet.mockResolvedValue({ locale: "uz-Latn", oferta: "", privacy: "", refund: "" });
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <OfertaPage />
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("heading", { level: 1, name: "Ommaviy oferta" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText(/mustaqil o'quv|onlayn o'quv xizmati/i)).toBeTruthy();
    });
    expect(screen.getByRole("link", { name: "Bosh sahifaga" })).toHaveAttribute("href", "/uz-Latn");
  });

  it("renders CMS oferta body when present", async () => {
    apiGet.mockResolvedValue({
      locale: "uz-Latn",
      oferta: "## CMS bo‘lim\n\nCMS oferta matni unique-xyz.",
      privacy: "",
      refund: "",
    });
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <OfertaPage />
      </NextIntlClientProvider>
    );
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 2, name: "CMS bo‘lim" })).toBeInTheDocument();
    });
    expect(screen.getByText(/unique-xyz/)).toBeInTheDocument();
    expect(screen.queryByText(/1\. Umumiy qoidalar/)).toBeNull();
  });
});

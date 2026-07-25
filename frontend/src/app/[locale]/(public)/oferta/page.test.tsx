import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import OfertaPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

describe("OfertaPage", () => {
  it("renders oferta title and sections in uz-Latn", () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <OfertaPage />
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("heading", { level: 1, name: "Ommaviy oferta" })).toBeInTheDocument();
    expect(screen.getByText(/mustaqil o'quv|onlayn o'quv xizmati/i)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Bosh sahifaga" })).toHaveAttribute("href", "/uz-Latn");
  });
});

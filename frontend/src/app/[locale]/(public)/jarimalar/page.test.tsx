import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import JarimalarPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));
vi.mock("next-intl/server", () => ({
  getLocale: async () => "uz-Latn",
  getTranslations: async (namespace: "Jarimalar" | "Landing") => {
    const messages = uzLatnMessages[namespace];
    return (key: string) => messages[key as keyof typeof messages];
  },
}));

describe("JarimalarPage", () => {
  it("renders honest SEO shell without invented fine amounts", async () => {
    const page = await JarimalarPage();
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        {page}
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("heading", { level: 1, name: "Yo'l jarimalari" })).toBeInTheDocument();
    expect(screen.getByText(/uydirma summalar/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Bosh sahifaga" })).toHaveAttribute("href", "/uz-Latn");
    expect(screen.getByRole("link", { name: /belgilar/i })).toHaveAttribute("href", "/uz-Latn/signs");
  });
});

import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import PrivacyPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

describe("PrivacyPage", () => {
  it("renders privacy title and contact path", () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <PrivacyPage />
      </NextIntlClientProvider>
    );
    expect(
      screen.getByRole("heading", { level: 1, name: "Maxfiylik siyosati" })
    ).toBeInTheDocument();
    expect(screen.getByText(/support@drivergo\.uz/)).toBeInTheDocument();
  });
});

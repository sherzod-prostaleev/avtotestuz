import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import uzLatnMessages from "../../../../../messages/uz-Latn.json";
import PrivacyPage from "./page";

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

describe("PrivacyPage", () => {
  beforeEach(() => {
    apiGet.mockReset();
  });

  it("falls back to i18n when CMS privacy is empty", async () => {
    apiGet.mockResolvedValue({ locale: "uz-Latn", oferta: "", privacy: "", refund: "" });
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <PrivacyPage />
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("heading", { level: 1, name: "Maxfiylik siyosati" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText(/shaxsiy ma'lumotlarni qayta ishlashini/i)).toBeTruthy();
    });
  });

  it("renders CMS privacy body when present", async () => {
    apiGet.mockResolvedValue({
      locale: "uz-Latn",
      oferta: "",
      privacy: "## Maxfiylik CMS\n\nCMS privacy unique-abc.",
      refund: "",
    });
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={uzLatnMessages}>
        <PrivacyPage />
      </NextIntlClientProvider>
    );
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 2, name: "Maxfiylik CMS" })).toBeInTheDocument();
    });
    expect(screen.getByText(/unique-abc/)).toBeInTheDocument();
  });
});

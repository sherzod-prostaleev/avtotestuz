import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { GrandMockCard } from "./grand-mock-card";
import * as apiClient from "@/lib/api-client";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <GrandMockCard />
    </NextIntlClientProvider>
  );
}

describe("GrandMockCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders locked state with mastery-too-low progress bar", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      eligible: false,
      mastery_percent: 42,
      min_required_percent: 85,
      is_vip: true,
      reason: "mastery_too_low",
    });

    renderWithIntl();

    const progress = await screen.findByRole("progressbar");
    expect(progress).toHaveAttribute("aria-valuenow", "42");
    expect(progress).toHaveAttribute("aria-valuemax", "85");
    expect(screen.getByText("42% / 85%")).toBeInTheDocument();
    expect(
      screen.getByText("Bosh imtihonni ochish uchun bilim darajangiz kamida 85% bo'lishi kerak. Hozircha: 42%.")
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bosh Imtihonni Boshlash" })).not.toBeInTheDocument();
  });

  it("renders locked state with VIP-required copy", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      eligible: false,
      mastery_percent: 90,
      min_required_percent: 85,
      is_vip: false,
      reason: "vip_required",
    });

    renderWithIntl();

    expect(await screen.findByText("Bosh imtihon faqat VIP obunachilar uchun. Obunani faollashtiring.")).toBeInTheDocument();
  });

  it("renders unlocked state with a start link to the grand_mock session-start flow", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      eligible: true,
      mastery_percent: 90,
      min_required_percent: 85,
      is_vip: true,
      reason: null,
    });

    renderWithIntl();

    const button = await screen.findByRole("button", { name: "Bosh Imtihonni Boshlash" });
    const link = button.closest("a");
    expect(link).toHaveAttribute("href", "/uz-Latn/session/start?mode=grand_mock");
  });

  it("shows an error state when the eligibility request fails", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(new Error("network down"));

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bosh imtihon holatini yuklab bo'lmadi. Qayta urinib ko'ring."
    );
  });
});

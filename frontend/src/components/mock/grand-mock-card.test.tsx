import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { GrandMockCard, type MockEligibilityResponse } from "./grand-mock-card";
import { mockEligibilityStore } from "@/lib/dashboard-stores";
import * as apiClient from "@/lib/api-client";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

function mockEligibility(overrides: Partial<MockEligibilityResponse>) {
  const base: MockEligibilityResponse = {
    eligible: false,
    mastery_percent: 90,
    min_required_percent: 85,
    questions_studied: 400,
    min_required_questions: 300,
    is_vip: true,
    reason: null,
  };
  vi.spyOn(apiClient, "apiGet").mockResolvedValue({ ...base, ...overrides });
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <GrandMockCard />
    </NextIntlClientProvider>
  );
}

describe("GrandMockCard", () => {
  beforeEach(() => {
    mockEligibilityStore.reset();
    vi.restoreAllMocks();
  });

  it("renders locked state with mastery-too-low progress bar", async () => {
    mockEligibility({ mastery_percent: 42, reason: "mastery_too_low" });

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

  // The volume floor exists because mastery alone is gameable: answering one
  // question per category correctly reads as 100% mastery. The bar has to track
  // the studied count here, not mastery, or it would sit full while locked.
  it("tracks the studied-question floor when that is the blocking gate", async () => {
    mockEligibility({
      mastery_percent: 100,
      questions_studied: 12,
      min_required_questions: 300,
      reason: "too_few_studied",
    });

    renderWithIntl();

    const progress = await screen.findByRole("progressbar");
    expect(progress).toHaveAttribute("aria-valuenow", "12");
    expect(progress).toHaveAttribute("aria-valuemax", "300");
    expect(screen.getByText("12 / 300 savol")).toBeInTheDocument();
    expect(
      screen.getByText("Bosh imtihonga kirish uchun kamida 300 ta savolni o'rganishingiz kerak. Hozircha: 12 ta.")
    ).toBeInTheDocument();
  });

  it("offers a premium link when VIP is the blocking gate", async () => {
    mockEligibility({ is_vip: false, reason: "vip_required" });

    renderWithIntl();

    expect(
      await screen.findByText("Bosh imtihon faqat VIP obunachilar uchun. Obunani faollashtiring.")
    ).toBeInTheDocument();
    // The CTA renders as <Link><Button as="span">, i.e. a real anchor rather
    // than a <button> nested inside one, so its accessible role is "link".
    const link = screen.getByRole("link", { name: "VIP obunani faollashtirish" });
    expect(link).toHaveAttribute("href", "/uz-Latn/premium");
  });

  it("does not offer the premium link when the block is study progress", async () => {
    mockEligibility({ mastery_percent: 42, reason: "mastery_too_low" });

    renderWithIntl();

    await screen.findByRole("progressbar");
    expect(screen.queryByRole("button", { name: "VIP obunani faollashtirish" })).not.toBeInTheDocument();
  });

  it("renders unlocked state with a start link to the grand_mock session-start flow", async () => {
    mockEligibility({ eligible: true });

    renderWithIntl();

    const link = await screen.findByRole("link", { name: "Bosh Imtihonni Boshlash" });
    expect(link).toHaveAttribute("href", "/uz-Latn/session/start?mode=grand_mock");
  });

  it("shows an error state when the eligibility request fails", async () => {
    vi.spyOn(apiClient, "apiGet").mockRejectedValue(new Error("network down"));

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bosh imtihon holatini yuklab bo'lmadi. Qayta urinib ko'ring."
    );
  });

  // A misconfigured limit_config row (min = 0) must not render a NaN width.
  it("treats a zero requirement as satisfied instead of dividing by zero", async () => {
    mockEligibility({ mastery_percent: 0, min_required_percent: 0, reason: "mastery_too_low" });

    renderWithIntl();

    const progress = await screen.findByRole("progressbar");
    expect(progress.firstElementChild).toHaveStyle({ width: "100%" });
  });
});

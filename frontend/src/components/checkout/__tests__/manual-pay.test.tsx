import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { ManualPayMobile } from "../manual-pay-mobile";
import { ManualPayCard } from "../manual-pay-card";
import { wrongAmountDecoy, type ManualPayInfo } from "../manual-pay-parts";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn(), prefetch: vi.fn() }),
}));

function info(over: Partial<ManualPayInfo> = {}): ManualPayInfo {
  return {
    payment_id: "p1",
    amount_uzs: 24900,
    pan_full: "9860246603626754",
    pan_last4: "6754",
    holder_name: "SADULLOYEV SHERZOD",
    network: "humo",
    hold_until: new Date(Date.now() + 9 * 60_000 + 23_000).toISOString(),
    manual_state: "awaiting_transfer",
    ...over,
  };
}

function renderMobile(over: Partial<ManualPayInfo> = {}) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ManualPayMobile info={info(over)} onClaim={() => {}} />
    </NextIntlClientProvider>
  );
}

describe("wrongAmountDecoy", () => {
  // The sum is the only thing a transfer is matched on, so the number shown
  // crossed out has to be the one this payer would really have sent — and can
  // never equal the right one.
  it("rounds a plain tariff price up to the next thousand", () => {
    expect(wrongAmountDecoy(24900)).toBe(25000);
    expect(wrongAmountDecoy(59900)).toBe(60000);
  });

  // A tail means `uniqueManualAmount` had to step around a sum already open on
  // this card, so the base price belongs to someone else right now: sending it
  // confirms THEIR payment. That, not 25 000, is the mistake to name.
  it("names the base price when the sum carries a tail", () => {
    expect(wrongAmountDecoy(24901)).toBe(24900);
    expect(wrongAmountDecoy(59937)).toBe(59900);
    expect(wrongAmountDecoy(109999)).toBe(109900);
  });

  it("still differs when the sum is already round", () => {
    expect(wrongAmountDecoy(60000)).toBe(61000);
  });
});

describe("ManualPayMobile", () => {
  it("shows the card, the holder and the exact amount", () => {
    renderMobile();
    expect(screen.getByText("9860 2466 0362 6754")).toBeInTheDocument();
    expect(screen.getByText("SADULLOYEV SHERZOD")).toBeInTheDocument();
    expect(screen.getByText("HUMO")).toBeInTheDocument();
    // Once on the card, once ticked in the notice below it.
    expect(screen.getAllByText(/24 900/)).toHaveLength(2);
  });

  it("names the rounded sum that would fail", () => {
    renderMobile();
    expect(screen.getByText(/25 000/)).toBeInTheDocument();
    expect(screen.getByText("tasdiqlanmaydi")).toBeInTheDocument();
    expect(screen.getByText("aynan shunday")).toBeInTheDocument();
    expect(screen.getByText("Summani yaxlitlamang")).toBeInTheDocument();
  });

  // A nudged sum differs from the price on the plan screen by two digits, and
  // that base price is another payer's open sum on this very card — sending it
  // confirms them, not this payer. So the tail is underlined and the base is
  // the number shown crossed out.
  it("defends the two digits that make a nudged sum this payment", () => {
    const { container } = renderMobile({ amount_uzs: 24901 });
    expect(container.textContent).toContain("24 901");
    expect(container.textContent).toContain("24 900");
    expect(container.textContent).not.toContain("25 000");

    const marks = container.querySelectorAll("span.underline");
    expect(marks).toHaveLength(2); // on the card, and on the ticked sum
    for (const mark of marks) {
      expect(mark.textContent).toBe("01");
    }
  });

  // "Karta band" read as "this card cannot take money right now", and people
  // waited instead of paying. The countdown is the window to pay, and says so.
  it("presents the hold as a window to pay, not a busy card", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-20T10:00:00.000Z"));
    try {
      renderMobile({ hold_until: "2026-09-20T10:09:23.000Z" });
      // The clock rides on the card itself, so it cannot be the thing that
      // scrolled away, and a line under it says what it is counting.
      expect(screen.getByText("09:23")).toBeInTheDocument();
      expect(screen.getByText(/shu vaqt ichida o'tkazing/i)).toBeInTheDocument();
      expect(screen.queryByText(/band/i)).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("explains what an expired window means instead of hiding it", () => {
    renderMobile({ hold_until: new Date(Date.now() - 1000).toISOString() });
    expect(screen.getByText("Vaqt tugadi")).toBeInTheDocument();
    expect(screen.getByText(/kuting, tasdiqlaymiz/)).toBeInTheDocument();
  });

  // `manual_state: "review"` is a state the backend never writes — the phone
  // body tested for it and so told a learner who had pressed the button
  // nothing at all.
  it("reports a claim the backend has actually recorded", () => {
    renderMobile({ manual_state: "claimed" });
    expect(screen.getByText(/tekshiryapmiz/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tekshiruvda" })).toBeDisabled();
  });

  // An expired hold rewrites `claimed` to `awaiting_review`, so the state alone
  // cannot say whether this payer reported anything.
  it("still counts a claim made before the hold expired", () => {
    renderMobile({ manual_state: "awaiting_review", claimed_at: "2026-09-20T10:05:00Z" });
    expect(screen.getByRole("button", { name: "Tekshiruvda" })).toBeDisabled();
  });

  // `awaiting_review` on its own only means the window ran out. Someone who
  // transferred late must still be able to say so — the backend accepts it.
  it("lets a late payer report a transfer after the window closed", () => {
    renderMobile({
      manual_state: "awaiting_review",
      hold_until: new Date(Date.now() - 1000).toISOString(),
    });
    expect(screen.getByRole("button", { name: "To‘lov qildim" })).toBeEnabled();
    expect(screen.queryByText(/tekshiryapmiz/i)).not.toBeInTheDocument();
  });
});

describe("ManualPayCard (wide)", () => {
  it("renders the same card, sum and window as the phone body", () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <ManualPayCard info={info()} onClaim={() => {}} />
      </NextIntlClientProvider>
    );
    expect(screen.getByText("9860 2466 0362 6754")).toBeInTheDocument();
    expect(screen.getByText(/25 000/)).toBeInTheDocument();
    expect(screen.getByText(/shu vaqt ichida o'tkazing/i)).toBeInTheDocument();
  });
});

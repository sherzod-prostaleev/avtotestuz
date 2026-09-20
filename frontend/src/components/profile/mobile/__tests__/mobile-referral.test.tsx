import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import { MobileReferral } from "../mobile-referral";
import type { ReferralResponse } from "@/components/profile/referral-card";
import * as apiClient from "@/lib/api-client";

const STATS: ReferralResponse = {
  referral_code: "REF123",
  invite_url: "https://drivergo.uz/r/REF123",
  total_invited: 5,
  total_rewarded: 2,
  earned_uzs: 50000,
  available_balance_uzs: 50000,
  commission_percent: 20,
};

function renderPayout() {
  vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    if (path === "me/referral") return STATS as never;
    throw new Error(`unexpected apiGet path: ${path}`);
  });
  const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({} as never);
  const view = render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MobileReferral onBack={() => {}} />
    </NextIntlClientProvider>
  );
  return { post, view };
}

async function openPayoutForm() {
  fireEvent.click(await screen.findByText("Pulni chiqarish"));
  return {
    amount: screen.getByLabelText("Summa (so'm)"),
    card: screen.getByLabelText("Karta raqami (16 raqam)"),
    submit: screen.getByRole("button", { name: "So'rov yuborish" }),
  };
}

describe("MobileReferral payout", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  // The server takes any 16-digit card and only refuses one whose prefix
  // contradicts the named network (`DetectCardNetwork`: 98… humo, 86… uzcard,
  // anything else unknown and therefore allowed). The phone used to demand
  // exactly 8600 or 9860, so a card the desktop form paid out to without a
  // murmur was refused here before a request was ever made.
  it("accepts an 86xx Uzcard that is not 8600", async () => {
    const { post } = renderPayout();
    const form = await openPayoutForm();

    fireEvent.change(form.amount, { target: { value: "10000" } });
    fireEvent.change(form.card, { target: { value: "8617123456789012" } });
    fireEvent.click(form.submit);

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/referral/payout", {
        amount_uzs: 10000,
        card_number: "8617123456789012",
        card_network: "uzcard",
      })
    );
  });

  it("accepts a 98xx Humo that is not 9860", async () => {
    const { post } = renderPayout();
    const form = await openPayoutForm();

    fireEvent.change(form.amount, { target: { value: "10000" } });
    fireEvent.change(form.card, { target: { value: "9863123456789012" } });
    fireEvent.click(form.submit);

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/referral/payout", {
        amount_uzs: 10000,
        card_number: "9863123456789012",
        card_network: "humo",
      })
    );
  });

  it("reads the number through the spaces people type", async () => {
    const { post } = renderPayout();
    const form = await openPayoutForm();

    fireEvent.change(form.amount, { target: { value: "10 000" } });
    fireEvent.change(form.card, { target: { value: "8600 1234 5678 9012" } });
    fireEvent.click(form.submit);

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/referral/payout", {
        amount_uzs: 10000,
        card_number: "8600123456789012",
        card_network: "uzcard",
      })
    );
  });

  // A prefix the server cannot place is still a card it accepts, so the phone
  // asks which network it is instead of refusing to send anything.
  it("asks which network an unrecognised prefix belongs to", async () => {
    const { post } = renderPayout();
    const form = await openPayoutForm();

    fireEvent.change(form.amount, { target: { value: "10000" } });
    fireEvent.change(form.card, { target: { value: "5614123456789012" } });

    const humo = await screen.findByRole("radio", { name: "Humo" });
    fireEvent.click(humo);
    fireEvent.click(form.submit);

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/referral/payout", {
        amount_uzs: 10000,
        card_number: "5614123456789012",
        card_network: "humo",
      })
    );
  });

  // Sixteen digits is the server's rule too (`^\d{16}$`), and saying so here
  // beats spending a request to be told "invalid card".
  it("names the real problem when the number is too short", async () => {
    const { post } = renderPayout();
    const form = await openPayoutForm();

    fireEvent.change(form.amount, { target: { value: "10000" } });
    fireEvent.change(form.card, { target: { value: "86001234567890" } });
    fireEvent.click(form.submit);

    expect(await screen.findByText(/16 raqam/)).toBeInTheDocument();
    expect(post).not.toHaveBeenCalled();
  });

  // The hint under an empty field used to read "Faqat Uzcard yoki Humo" — the
  // same words the form uses to reject a card, shown before anything is typed.
  it("says nothing about the network until there is a number to judge", async () => {
    renderPayout();
    await openPayoutForm();
    expect(screen.queryByText("Faqat Uzcard yoki Humo")).not.toBeInTheDocument();
  });
});

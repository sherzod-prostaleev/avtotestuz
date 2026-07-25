import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { SupportTicketCard } from "../support-ticket-card";
import * as apiClient from "@/lib/api-client";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SupportTicketCard />
    </NextIntlClientProvider>,
  );
}

describe("SupportTicketCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("submits a short ticket", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ id: "t1" });
    renderWithIntl();
    fireEvent.change(screen.getByPlaceholderText(/Mavzu/i), {
      target: { value: "To‘lov" },
    });
    fireEvent.change(screen.getByPlaceholderText(/Muammo/i), {
      target: { value: "Sandbox fail" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Yuborish/i }));
    await waitFor(() => {
      expect(post).toHaveBeenCalledWith("me/support/tickets", expect.objectContaining({
        subject: "To‘lov",
        body: "Sandbox fail",
      }));
    });
    expect(await screen.findByText(/Ticket yuborildi/i)).toBeInTheDocument();
  });
});

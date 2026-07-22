import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import TicketsPage from "./page";
import * as useTicketsModule from "@/hooks/use-tickets";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/uz-Latn/tickets",
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TicketsPage />
    </NextIntlClientProvider>
  );
}

describe("TicketsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders tickets header and grid", () => {
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false },
      ] as any,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hammasi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tugallangan" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jarayonda" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qulflangan (VIP talab qilinadi)" })).toBeInTheDocument();
    expect(screen.getByText("Bilet 1")).toBeInTheDocument();
    expect(screen.getByText("19/20")).toBeInTheDocument();
    expect(screen.getByText("Bilet 2")).toBeInTheDocument();
  });
});

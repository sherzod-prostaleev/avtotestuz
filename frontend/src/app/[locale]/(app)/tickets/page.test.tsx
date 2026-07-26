import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import TicketsPage from "./page";
import * as useTicketsModule from "@/hooks/use-tickets";

const { pushMock, prefetchMock } = vi.hoisted(() => ({
  pushMock: vi.fn(),
  prefetchMock: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
  usePathname: () => "/uz-Latn/tickets",
}));

vi.mock("@/lib/prefetch-variant", () => ({
  prefetchVariantDetail: prefetchMock,
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
    pushMock.mockReset();
    prefetchMock.mockReset();
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
    expect(
      screen.getByText(/Biletlarni bosqichma-bosqich yoping/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hammasi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tugallangan" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jarayonda" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qulflangan" })).toBeInTheDocument();
    expect(screen.getAllByText("Bilet 1").length).toBeGreaterThan(0);
    expect(screen.getByText("19/20")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "2-biletni ochish" })).toBeInTheDocument();
    expect(screen.getByText(/10 ta to'g'ri → keyingi bilet/i)).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole("button", { name: "1-biletni ochish" }), { key: "Enter" });
    expect(prefetchMock).toHaveBeenCalledWith(1, "uz-Latn");
    expect(pushMock).toHaveBeenCalledWith("/uz-Latn/session/start?mode=variant&variant_id=1");
  });

  it("shows previous-ticket guidance instead of premium for prev_required locks", () => {
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
      tickets: [
        {
          number: 1,
          best_correct: 12,
          attempts: 1,
          unlocked: true,
          completed_at: "2026-07-20T12:00:00Z",
          status: "completed",
        },
        {
          number: 2,
          best_correct: 0,
          attempts: 0,
          unlocked: false,
          lock_reason: "prev_required",
          status: "locked",
        },
      ] as any,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    fireEvent.click(screen.getByRole("button", { name: "2-biletni ochish" }));
    expect(pushMock).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Avval oldingi biletda kamida 10 ta to'g'ri/i)
    ).toBeInTheDocument();
  });
});

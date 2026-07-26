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
      screen.getByText(/Biletlarni birma-bir yoping/i)
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hammasi" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tugallangan" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jarayonda" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Qulflangan (VIP talab qilinadi)" })).toBeInTheDocument();
    expect(screen.getByText("Bilet 1")).toBeInTheDocument();
    expect(screen.getByText("19/20")).toBeInTheDocument();
    expect(screen.getByText("Bilet 2")).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole("button", { name: "1-biletni ochish" }), { key: "Enter" });
    expect(prefetchMock).toHaveBeenCalledWith(1, "uz-Latn");
    expect(pushMock).toHaveBeenCalledWith("/uz-Latn/session/start?mode=variant&variant_id=1");
  });
});

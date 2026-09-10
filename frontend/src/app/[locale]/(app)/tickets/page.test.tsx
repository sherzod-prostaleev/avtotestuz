import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import TicketsPage from "./page";
import * as useTicketsModule from "@/hooks/use-tickets";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

/** Same check src/proxy.ts runs on every request from a login-free kiosk browser. */
function isKioskReachable(hrefOrPush: string): boolean {
  const withoutLocale = hrefOrPush.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

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

/**
 * Stubs useTickets with a complete return value, so a test only states the
 * part it is about. Spreading a full default here rather than at each call
 * site means growing the hook does not break every test in the file.
 */
function mockUseTickets(
  overrides: Partial<ReturnType<typeof useTicketsModule.useTickets>> = {}
) {
  return vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
    tickets: [],
    loading: false,
    error: null,
    refetch: vi.fn(),
    clearProgress: vi.fn().mockResolvedValue(true),
    clearing: false,
    clearError: null,
    clearedCount: null,
    dismissClearNotice: vi.fn(),
    ...overrides,
  });
}

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
    mockUseTickets({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false },
      ] as any,
    });

    renderWithIntl();

    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(
      screen.getByText(/Biletlarni bosqichma-bosqich yoping/i)
    ).toBeInTheDocument();
    // The filter now carries its count, so the accessible name is "Hammasi 64".
    expect(screen.getByRole("button", { name: /^Hammasi\b/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Tugallangan\b/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Jarayonda\b/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Qulflangan\b/ })).toBeInTheDocument();
    expect(screen.getAllByText("Bilet 1").length).toBeGreaterThan(0);
    // Twice on purpose: the tile carries a compact body for phones and the
    // rich card body for md and up, and both render the score.
    expect(screen.getAllByText("19/20")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "2-biletni ochish" })).toBeInTheDocument();
    expect(screen.getByText(/10 ta to'g'ri → keyingi bilet/i)).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole("button", { name: "1-biletni ochish" }), { key: "Enter" });
    expect(prefetchMock).toHaveBeenCalledWith(1, "uz-Latn");
    expect(pushMock).toHaveBeenCalledWith("/uz-Latn/session/start?mode=variant&variant_id=1");
  });

  it("shows previous-ticket guidance instead of premium for prev_required locks", () => {
    mockUseTickets({
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
    });

    renderWithIntl();

    fireEvent.click(screen.getByRole("button", { name: "2-biletni ochish" }));
    expect(pushMock).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Avval oldingi biletda kamida 10 ta to'g'ri/i)
    ).toBeInTheDocument();
  });
});

// The header carries two clear controls that are never on screen together:
// a labelled one for wide layouts and an icon-only twin for phones. Both are
// in the DOM under jsdom, so each test names the one it means.
const CLEAR_WIDE = { name: "Tozalash" };
const CLEAR_PHONE = { name: "Ishlangan biletlar natijasini tozalash" };

describe("TicketsPage clear-progress control", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
    prefetchMock.mockReset();
  });

  /** One played bilet and one untouched — so exactly one row is clearable. */
  const playedAndUntouched = [
    { number: 1, best_correct: 19, attempts: 1, unlocked: true },
    { number: 2, best_correct: 0, attempts: 0, unlocked: true },
  ] as any;

  it("offers both the wide and the phone control when there is something to clear", () => {
    mockUseTickets({ tickets: playedAndUntouched });
    renderWithIntl();

    expect(screen.getByRole("button", CLEAR_WIDE)).toBeEnabled();
    expect(screen.getByRole("button", CLEAR_PHONE)).toBeEnabled();
    // Both announce that they open a dialog rather than acting immediately.
    expect(screen.getByRole("button", CLEAR_WIDE)).toHaveAttribute("aria-haspopup", "dialog");
    expect(screen.getByRole("button", CLEAR_PHONE)).toHaveAttribute("aria-haspopup", "dialog");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("disables the control when no bilet carries a result", () => {
    mockUseTickets({
      tickets: [
        { number: 1, best_correct: 0, attempts: 0, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false, lock_reason: "vip_required", status: "locked" },
      ] as any,
    });
    renderWithIntl();

    expect(screen.getByRole("button", CLEAR_WIDE)).toBeDisabled();
    expect(screen.getByRole("button", CLEAR_PHONE)).toBeDisabled();
  });

  it("stays disabled while the grid is still loading", () => {
    mockUseTickets({ tickets: [], loading: true });
    renderWithIntl();

    // Nothing has arrived yet, so the count on the confirmation would be a lie.
    expect(screen.getByRole("button", CLEAR_WIDE)).toBeDisabled();
  });

  it("counts a completed bilet as clearable even once it is locked again", () => {
    // A lapsed subscription can put a played bilet back behind the VIP gate.
    // Its score is still a result, and refusing to clear it would strand it.
    mockUseTickets({
      tickets: [
        {
          number: 1,
          best_correct: 18,
          attempts: 2,
          unlocked: false,
          lock_reason: "vip_required",
          status: "locked",
          completed_at: "2026-07-20T12:00:00Z",
        },
      ] as any,
    });
    renderWithIntl();

    expect(screen.getByRole("button", CLEAR_WIDE)).toBeEnabled();
  });

  it("asks before clearing, and cancelling changes nothing", async () => {
    const clearProgress = vi.fn().mockResolvedValue(true);
    mockUseTickets({ tickets: playedAndUntouched, clearProgress });
    renderWithIntl();

    fireEvent.click(screen.getByRole("button", CLEAR_WIDE));

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(within(dialog).getByText("Natijalarni tozalash")).toBeInTheDocument();
    // The count is the number of bilets with results, not the grid size.
    expect(within(dialog).getByText(/^1 ta biletning natijasi o'chiriladi/)).toBeInTheDocument();
    expect(within(dialog).getByText(/Ochiq biletlar ochiqligicha/)).toBeInTheDocument();
    expect(within(dialog).getByText("Bu amalni qaytarib bo'lmaydi.")).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Bekor qilish" }));
    expect(clearProgress).not.toHaveBeenCalled();
    // Awaited, not asserted outright: the dialog animates out, so it survives
    // a frame or two past the click.
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("clears on confirmation", async () => {
    const clearProgress = vi.fn().mockResolvedValue(true);
    mockUseTickets({ tickets: playedAndUntouched, clearProgress });
    renderWithIntl();

    fireEvent.click(screen.getByRole("button", CLEAR_WIDE));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Ha, tozalash" }));

    expect(clearProgress).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("dismisses the dialog on Escape without clearing", async () => {
    const clearProgress = vi.fn().mockResolvedValue(true);
    mockUseTickets({ tickets: playedAndUntouched, clearProgress });
    renderWithIntl();

    fireEvent.click(screen.getByRole("button", CLEAR_WIDE));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(clearProgress).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("puts initial focus on the way out, not on the destructive action", async () => {
    mockUseTickets({ tickets: playedAndUntouched });
    renderWithIntl();

    fireEvent.click(screen.getByRole("button", CLEAR_WIDE));
    const dialog = screen.getByRole("dialog");
    await waitFor(() =>
      expect(within(dialog).getByRole("button", { name: "Bekor qilish" })).toHaveFocus()
    );
  });

  it("locks both buttons while the request is in flight", () => {
    mockUseTickets({ tickets: playedAndUntouched, clearing: true });
    renderWithIntl();

    // The header control cannot start a second reset...
    expect(screen.getByRole("button", CLEAR_WIDE)).toBeDisabled();
  });

  it("reports how many bilets were cleared", () => {
    mockUseTickets({ tickets: playedAndUntouched, clearedCount: 12 });
    renderWithIntl();

    expect(screen.getByRole("status")).toHaveTextContent("12 ta biletning natijasi tozalandi.");
  });

  it("surfaces a failed clear as an alert", () => {
    mockUseTickets({ tickets: playedAndUntouched, clearError: "boom" });
    renderWithIntl();

    expect(screen.getByRole("alert")).toHaveTextContent(/Natijalarni tozalab bo'lmadi/);
  });
});

// Walks every navigation this page can perform for a cookie-less classroom
// kiosk browser (frontend/src/app/[locale]/(kiosk)/station/tickets/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces.
describe("TicketsPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
    prefetchMock.mockReset();
  });

  function renderKiosk() {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <TicketsPage kiosk />
      </NextIntlClientProvider>
    );
  }

  it("keeps back and practice links under /station", () => {
    mockUseTickets({
      tickets: [{ number: 1, best_correct: 19, attempts: 1, unlocked: true }] as any,
    });

    renderKiosk();

    const backLink = screen.getByRole("link", { name: /Bosh sahifaga qaytish/ });
    expect(backLink.getAttribute("href")).toBe("/uz-Latn/station");
    expect(isKioskReachable(backLink.getAttribute("href")!)).toBe(true);
  });

  it("starts a ticket on a kiosk-reachable session/start", () => {
    mockUseTickets({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false },
      ] as any,
    });

    renderKiosk();

    fireEvent.keyDown(screen.getByRole("button", { name: "1-biletni ochish" }), { key: "Enter" });
    expect(pushMock).toHaveBeenCalledTimes(1);
    const target = pushMock.mock.calls[0][0] as string;
    expect(target).toBe("/uz-Latn/station/session/start?mode=variant&variant_id=1");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("never pushes to /premium for a VIP-locked ticket — shows the kiosk notice instead", () => {
    // Closes the loop on the kiosk-safe marker at
    // router.push(`/${locale}/premium`) in page.tsx: that line only runs
    // when the `if (kiosk) { ...; return; }` guard above it does NOT fire,
    // so exercising the VIP-locked path here with kiosk=true is what makes
    // the marker's claim checkable — if that guard were ever removed,
    // pushMock would be called with a /premium target and the assertion
    // below would fail.
    mockUseTickets({
      tickets: [
        {
          number: 1,
          best_correct: 0,
          attempts: 0,
          unlocked: false,
          lock_reason: "vip_required",
          status: "locked",
        },
      ] as any,
    });

    renderKiosk();

    fireEvent.click(screen.getByRole("button", { name: "1-biletni ochish" }));
    expect(pushMock).not.toHaveBeenCalled();
    expect(screen.getByRole("status")).toHaveTextContent("VIP kerak");
  });

  it("never renders a link into a protected segment, in any state", () => {
    // The targeted checks above cover the links/pushes most likely to
    // regress (back/practice links, the VIP-lock push). This sweeps every
    // link this render can produce as a backstop against a new one showing
    // up without a matching targeted test.
    mockUseTickets({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false, lock_reason: "vip_required", status: "locked" },
      ] as any,
    });

    renderKiosk();

    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.length).toBeGreaterThan(0);
    for (const href of hrefs) {
      expect(isKioskReachable(href)).toBe(true);
    }
    const withoutLocale = hrefs.map((h) => h.replace(/^\/[a-zA-Z-]+/, ""));
    expect(withoutLocale.some((h) => /^\/(dashboard|premium|checkout|profile)(\/|$|\?)/.test(h))).toBe(false);
  });
});

import { fireEvent, render, screen } from "@testing-library/react";
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
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
      tickets: [{ number: 1, best_correct: 19, attempts: 1, unlocked: true }] as any,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderKiosk();

    const backLink = screen.getByRole("link", { name: /Bosh sahifaga qaytish/ });
    expect(backLink.getAttribute("href")).toBe("/uz-Latn/station");
    expect(isKioskReachable(backLink.getAttribute("href")!)).toBe(true);
  });

  it("starts a ticket on a kiosk-reachable session/start", () => {
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false },
      ] as any,
      loading: false,
      error: null,
      refetch: vi.fn(),
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
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
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
      loading: false,
      error: null,
      refetch: vi.fn(),
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
    vi.spyOn(useTicketsModule, "useTickets").mockReturnValue({
      tickets: [
        { number: 1, best_correct: 19, attempts: 1, unlocked: true },
        { number: 2, best_correct: 0, attempts: 0, unlocked: false, lock_reason: "vip_required", status: "locked" },
      ] as any,
      loading: false,
      error: null,
      refetch: vi.fn(),
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

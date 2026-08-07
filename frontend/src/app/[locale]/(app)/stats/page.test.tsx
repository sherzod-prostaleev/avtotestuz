import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import StatsPage from "./page";
import * as useUserStatsModule from "@/hooks/use-user-stats";
import * as useSessionHistoryModule from "@/hooks/use-session-history";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

/** True once the locale prefix is stripped and every segment is checked
 * against the cookie gate — the same check src/proxy.ts runs on every
 * request from the login-free kiosk browser. */
function isKioskReachable(hrefOrPush: string): boolean {
  const withoutLocale = hrefOrPush.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <StatsPage />
    </NextIntlClientProvider>
  );
}

describe("StatsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders user stats readiness and streak", () => {
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567" },
      entitlement: null,
      streak: { current_streak: 10, max_streak: 20, today_answered: 5, daily_target: 10 },
      stats: { readiness_pct: 88, due_questions_count: 3, total_answered: 100, total_correct: 90, category_mastery: [] },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    vi.spyOn(useSessionHistoryModule, "useSessionHistory").mockReturnValue({
      sessions: [
        {
          id: "session-1",
          mode: "exam",
          status: "passed",
          score: 18,
          total: 20,
          started_at: "2026-07-22T10:00:00Z",
          finished_at: "2026-07-22T10:20:00Z",
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Statistika va Tahlil")).toBeInTheDocument();
    expect(screen.getByText("88%")).toBeInTheDocument();
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("Yaxshi o‘zlashtirish")).toBeInTheDocument();
    expect(screen.getByText("Imtihon")).toBeInTheDocument();
    expect(screen.getByText("Muvaffaqiyatli")).toBeInTheDocument();
    expect(screen.getByText("18/20")).toBeInTheDocument();
    expect(screen.getByText("20 daq")).toBeInTheDocument();

    const dueCta = screen.getByRole("link", { name: "Hozir takrorlash" });
    expect(dueCta).toHaveAttribute("href", "/uz-Latn/session/start?mode=review&count=3");
  });

  it("hides the due sticky CTA when there are no due questions", () => {
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567" },
      entitlement: null,
      streak: { current_streak: 1, max_streak: 1, today_answered: 0, daily_target: 10 },
      stats: { readiness_pct: 40, due_questions_count: 0, total_answered: 10, total_correct: 5, category_mastery: [] },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    vi.spyOn(useSessionHistoryModule, "useSessionHistory").mockReturnValue({
      sessions: [],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();
    expect(screen.queryByRole("link", { name: "Hozir takrorlash" })).not.toBeInTheDocument();
  });
});

// Walks every navigation this page can perform for a cookie-less classroom
// kiosk browser (frontend/src/app/[locale]/(kiosk)/station/stats/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces — a kiosk browser
// carries no auth cookie, so a gated destination is a dead end at /login.
describe("StatsPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function mockStats(dueCount = 3) {
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567" },
      entitlement: null,
      streak: { current_streak: 10, max_streak: 20, today_answered: 5, daily_target: 10 },
      stats: {
        readiness_pct: 88,
        due_questions_count: dueCount,
        total_answered: 100,
        total_correct: 90,
        category_mastery: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
    vi.spyOn(useSessionHistoryModule, "useSessionHistory").mockReturnValue({
      sessions: [],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
  }

  function renderKiosk() {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <StatsPage kiosk />
      </NextIntlClientProvider>
    );
  }

  it("keeps the back link under /station", () => {
    mockStats();
    renderKiosk();

    const backLink = screen.getByRole("link", { name: /Bosh sahifaga qaytish/ });
    expect(backLink.getAttribute("href")).toBe("/uz-Latn/station");
    expect(isKioskReachable(backLink.getAttribute("href")!)).toBe(true);
  });

  it("sends the due-repeat CTA to a kiosk-reachable session/start", () => {
    mockStats(3);
    renderKiosk();

    const dueCta = screen.getByRole("link", { name: "Hozir takrorlash" });
    const href = dueCta.getAttribute("href") ?? "";
    expect(href).toBe("/uz-Latn/station/session/start?mode=review&count=3");
    expect(isKioskReachable(href)).toBe(true);
  });

  it("never renders a link into a protected segment, in any state", () => {
    mockStats(3);
    renderKiosk();

    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.length).toBeGreaterThan(0);
    for (const href of hrefs) {
      expect(isKioskReachable(href)).toBe(true);
    }
    const withoutLocale = hrefs.map((h) => h.replace(/^\/[a-zA-Z-]+/, ""));
    expect(withoutLocale.some((h) => /^\/(dashboard|premium|checkout|profile)(\/|$|\?)/.test(h))).toBe(false);
  });

  it("offers no premium, checkout, profile or dashboard surface", () => {
    mockStats(3);
    renderKiosk();

    // This page has never carried an explicit upsell CTA of its own — the
    // only "dashboard" surface is the generic back link, which the assertion
    // above already redirects to /station. This closes the loop on the
    // brief's requirement by name: none of these render, under any label.
    expect(screen.queryByRole("link", { name: /premium/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /checkout/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /profile/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });
});

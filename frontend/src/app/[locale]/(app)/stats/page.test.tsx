import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import StatsPage from "./page";
import * as useUserStatsModule from "@/hooks/use-user-stats";
import * as useSessionHistoryModule from "@/hooks/use-session-history";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

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
    expect(screen.getByText("Imtihonga tayyor!")).toBeInTheDocument();
    expect(screen.getByText("Imtihon")).toBeInTheDocument();
    expect(screen.getByText("Muvaffaqiyatli")).toBeInTheDocument();
    expect(screen.getByText("18/20")).toBeInTheDocument();
    expect(screen.getByText("20 daq")).toBeInTheDocument();
  });
});

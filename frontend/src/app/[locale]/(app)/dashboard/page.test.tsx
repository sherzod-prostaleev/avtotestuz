import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import DashboardPage from "./page";
import * as useUserStatsModule from "@/hooks/use-user-stats";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/uz-Latn/dashboard",
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <DashboardPage />
    </NextIntlClientProvider>
  );
}

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders user name, streak count and readiness ring from useUserStats", () => {
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: { id: "u1", phone: "+998901234567", name: "Dilshod" },
      entitlement: { is_vip: true } as any,
      streak: { current_streak: 7, max_streak: 15, today_answered: 5, daily_target: 10 },
      stats: {
        readiness_pct: 82,
        due_questions_count: 5,
        total_answered: 200,
        total_correct: 180,
        category_mastery: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText(/Dilshod/)).toBeInTheDocument();
    expect(screen.getByText(/7 kunlik streak/)).toBeInTheDocument();
    expect(screen.getByText("82%")).toBeInTheDocument();
    expect(screen.getByText("VIP Pass")).toBeInTheDocument();
  });

  it("renders all four navigation cards and signs catalog link", () => {
    vi.spyOn(useUserStatsModule, "useUserStats").mockReturnValue({
      user: null,
      entitlement: null,
      streak: null,
      stats: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByText("Imtihon simulyatsiyasi")).toBeInTheDocument();
    expect(screen.getByText("Mashq rejimi")).toBeInTheDocument();
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
  });
});

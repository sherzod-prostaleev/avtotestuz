import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useUserStats } from "./use-user-stats";
import * as apiClient from "@/lib/api-client";

describe("useUserStats", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches dashboard data successfully", async () => {
    const mockUser = { id: "u-1", phone: "+998901234567", name: "Alisher" };
    const mockEnt = { is_vip: true, valid_until: "2026-12-31" };
    const mockStreak = { current_streak: 5, max_streak: 10, today_answered: 3, daily_target: 10 };
    const mockStats = {
      readiness_pct: 75,
      due_questions_count: 12,
      total_answered: 150,
      total_correct: 130,
      category_mastery: [],
    };

    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "me") return mockUser as any;
      if (path === "me/entitlement") return mockEnt as any;
      if (path === "me/streak") return mockStreak as any;
      if (path === "me/stats") return mockStats as any;
      throw new Error("Unknown path");
    });

    const { result } = renderHook(() => useUserStats());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user?.name).toBe("Alisher");
    expect(result.current.entitlement?.is_vip).toBe(true);
    expect(result.current.streak?.current_streak).toBe(5);
    expect(result.current.stats?.readiness_pct).toBe(75);
  });
});

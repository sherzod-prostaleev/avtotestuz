import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useUserStats } from "./use-user-stats";
import * as apiClient from "@/lib/api-client";

vi.mock("next-intl", () => ({
  useLocale: () => "ru",
}));

describe("useUserStats", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("maps real backend DTOs into dashboard data", async () => {
    const mockMe = {
      profile: {
        id: "u-1",
        phone: "+998901234567",
        name: "Alisher",
        region: "Tashkent",
        district: "Yunusobod",
        birth_date: null,
        locale_pref: "ru",
        theme_pref: "system",
        referral_code: "REF-1",
        role: "student",
        created_at: "2026-07-22T10:00:00Z",
      },
      vip: { active: true, until: "2026-12-31T00:00:00Z" },
    };
    const mockStreak = {
      current: 5,
      best: 10,
      today_done: 3,
      daily_goal: 10,
      last_active_date: "2026-07-22",
    };
    const mockStats = {
      categories: [
        { category_code: "signs", mastery: 0.755, seen: 20, correct: 15, studied: 18, total: 100 },
        { category_code: "priority", mastery: 0.5, seen: 10, correct: 5, studied: 10, total: 50 },
      ],
      readiness_pct: 75,
      due_count: 12,
    };
    const mockCategories = [
      { code: "signs", name: "Дорожные знаки", sort_order: 1 },
      { code: "priority", name: "Приоритет", sort_order: 2 },
    ];

    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "me") return mockMe as any;
      if (path === "me/streak") return mockStreak as any;
      if (path === "me/stats") return mockStats as any;
      if (path === "categories?locale=ru") return mockCategories as any;
      throw new Error("Unknown path");
    });

    const { result } = renderHook(() => useUserStats());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.user?.name).toBe("Alisher");
    expect(result.current.entitlement?.is_vip).toBe(true);
    expect(result.current.entitlement?.valid_until).toBe("2026-12-31T00:00:00Z");
    expect(result.current.streak?.current_streak).toBe(5);
    expect(result.current.streak?.max_streak).toBe(10);
    expect(result.current.streak?.today_answered).toBe(3);
    expect(result.current.streak?.last_active_date).toBe("2026-07-22");
    expect(result.current.stats?.readiness_pct).toBe(75);
    expect(result.current.stats?.due_questions_count).toBe(12);
    expect(result.current.stats?.total_answered).toBe(30);
    expect(result.current.stats?.total_correct).toBe(20);
    expect(result.current.stats?.category_mastery).toEqual([
      { code: "signs", name: "Дорожные знаки", answered: 20, correct: 15, studied: 18, total: 100, mastery_pct: 76 },
      { code: "priority", name: "Приоритет", answered: 10, correct: 5, studied: 10, total: 50, mastery_pct: 50 },
    ]);
    expect(apiClient.apiGet).toHaveBeenCalledWith("categories?locale=ru");
    expect(apiClient.apiGet).not.toHaveBeenCalledWith("me/entitlement");
  });

  it("reports an API failure instead of returning partial dashboard data", async () => {
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "me/stats") {
        throw new apiClient.ApiError("stats unavailable", "internal", 500);
      }
      return [] as any;
    });

    const { result } = renderHook(() => useUserStats());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe("stats unavailable");
    expect(result.current.user).toBeNull();
    expect(result.current.stats).toBeNull();
  });
});

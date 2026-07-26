import { useState, useEffect, useCallback } from "react";
import { useLocale } from "next-intl";
import { apiGet, ApiError } from "@/lib/api-client";

export interface UserProfile {
  id: string;
  phone: string;
  name?: string;
  region?: string;
  district?: string;
  birth_date?: string | null;
  locale_pref?: string;
  theme_pref?: string;
  referral_code?: string;
  role?: string;
  created_at?: string;
}

export interface UserEntitlement {
  is_vip: boolean;
  valid_until?: string | null;
}

export interface UserStreak {
  current_streak: number;
  max_streak: number;
  today_answered: number;
  daily_target: number;
  last_active_date?: string | null;
}

export interface CategoryMastery {
  code: string;
  name: string;
  answered: number;
  correct: number;
  studied: number;
  total: number;
  mastery_pct: number;
}

export interface UserStats {
  readiness_pct: number;
  due_questions_count: number;
  total_answered: number;
  total_correct: number;
  category_mastery: CategoryMastery[];
}

export interface DashboardData {
  user: UserProfile | null;
  entitlement: UserEntitlement | null;
  streak: UserStreak | null;
  stats: UserStats | null;
}

interface MeResponseDTO {
  profile: {
    id: string;
    phone: string;
    name: string;
    region: string;
    district: string;
    birth_date: string | null;
    locale_pref: string;
    theme_pref: string;
    referral_code: string;
    role: string;
    created_at: string;
  };
  vip: {
    active: boolean;
    until: string | null;
  };
}

interface StreakDTO {
  current: number;
  best: number;
  today_done: number;
  daily_goal: number;
  last_active_date: string | null;
}

interface CategoryStatDTO {
  category_code: string;
  mastery: number;
  seen: number;
  correct: number;
  studied: number;
  total: number;
}

interface StatsResponseDTO {
  categories: CategoryStatDTO[];
  readiness_pct: number;
  due_count: number;
}

interface CategoryDTO {
  code: string;
  name: string;
  sort_order: number;
}

export function useUserStats() {
  const locale = useLocale();
  const [data, setData] = useState<DashboardData>({
    user: null,
    entitlement: null,
    streak: null,
    stats: null,
  });
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [me, streakDTO, statsDTO, categories] = await Promise.all([
        apiGet<MeResponseDTO>("me"),
        apiGet<StreakDTO>("me/streak"),
        apiGet<StatsResponseDTO>("me/stats"),
        apiGet<CategoryDTO[]>(`categories?locale=${encodeURIComponent(locale)}`),
      ]);

      const namesByCode = new Map(categories.map((category) => [category.code, category.name]));
      const categoryMastery = statsDTO.categories.map((category) => {
        const name = namesByCode.get(category.category_code);
        if (!name) {
          throw new Error(`Missing localized category: ${category.category_code}`);
        }
        return {
          code: category.category_code,
          name,
          answered: category.seen,
          correct: category.correct,
          studied: category.studied ?? 0,
          total: category.total ?? 0,
          mastery_pct: Math.round(category.mastery * 100),
        };
      });

      const entitlement: UserEntitlement = {
        is_vip: me.vip.active,
        valid_until: me.vip.until,
      };
      const streak: UserStreak = {
        current_streak: streakDTO.current,
        max_streak: streakDTO.best,
        today_answered: streakDTO.today_done,
        daily_target: streakDTO.daily_goal,
        last_active_date: streakDTO.last_active_date,
      };
      const stats: UserStats = {
        readiness_pct: statsDTO.readiness_pct,
        due_questions_count: statsDTO.due_count,
        total_answered: statsDTO.categories.reduce((sum, category) => sum + category.seen, 0),
        total_correct: statsDTO.categories.reduce((sum, category) => sum + category.correct, 0),
        category_mastery: categoryMastery,
      };

      setData({ user: me.profile, entitlement, streak, stats });
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to load user data");
      }
    } finally {
      setLoading(false);
    }
  }, [locale]);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  return {
    ...data,
    loading,
    error,
    refetch: fetchAll,
  };
}

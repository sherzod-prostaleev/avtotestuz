import { useState, useEffect, useCallback } from "react";
import { apiGet, ApiError } from "@/lib/api-client";

export interface UserProfile {
  id: string;
  phone: string;
  name?: string;
  region?: string;
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
}

export interface CategoryMastery {
  category_id: string;
  code: string;
  name: string;
  total: number;
  answered: number;
  correct: number;
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

export function useUserStats() {
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
      const [userRes, entRes, streakRes, statsRes] = await Promise.allSettled([
        apiGet<UserProfile>("me"),
        apiGet<UserEntitlement>("me/entitlement"),
        apiGet<UserStreak>("me/streak"),
        apiGet<UserStats>("me/stats"),
      ]);

      const user = userRes.status === "fulfilled" ? userRes.value : null;
      const entitlement = entRes.status === "fulfilled" ? entRes.value : null;
      const streak = streakRes.status === "fulfilled" ? streakRes.value : null;
      const stats = statsRes.status === "fulfilled" ? statsRes.value : null;

      setData({ user, entitlement, streak, stats });
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to load user data");
      }
    } finally {
      setLoading(false);
    }
  }, []);

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

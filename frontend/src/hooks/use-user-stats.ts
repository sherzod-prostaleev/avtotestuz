import { useQuery } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { apiGet, ApiError } from "@/lib/api-client";
import { useMeQuery, type MeResponseDTO } from "@/hooks/use-me";

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

export interface PassEstimate {
  estimated_pass_pct: number;
  source: "empirical" | "model" | string;
  sample_size: number;
  bucket_lo: number;
}

export interface UserStats {
  readiness_pct: number;
  due_questions_count: number;
  total_answered: number;
  total_correct: number;
  category_mastery: CategoryMastery[];
  pass_estimate?: PassEstimate | null;
}

export interface DashboardData {
  user: UserProfile | null;
  entitlement: UserEntitlement | null;
  streak: UserStreak | null;
  stats: UserStats | null;
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
  pass_estimate?: PassEstimate;
}

interface CategoryDTO {
  code: string;
  name: string;
  sort_order: number;
}

type UserStatsRest = {
  streak: UserStreak;
  stats: UserStats;
};

function queryErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Failed to load user data";
}

function mapMe(me: MeResponseDTO): Pick<DashboardData, "user" | "entitlement"> {
  return {
    user: me.profile,
    entitlement: {
      is_vip: me.vip.active,
      valid_until: me.vip.until,
    },
  };
}

/**
 * No module-level cache in here, on purpose.
 *
 * React Query already caches this exact call under ["user-stats-rest", locale]
 * with the same 30s staleTime, and a second cache underneath it is not free.
 * It sits below `refetch()` — which exists precisely to bypass staleTime — so
 * the retry button on /stats would go on serving the copy it was pressed to
 * replace. And `me/streak` / `me/stats` are per-learner, while module scope
 * outlives a logout: logout is a router.push (profile/page.tsx), not a reload,
 * so on a shared classroom PC the next learner would inherit the previous
 * one's streak for half a minute.
 *
 * If the three requests ever need to survive a locale switch, split them into
 * their own queries — `me/streak` and `me/stats` do not depend on the locale
 * and do not belong under a locale-keyed key. Do not reach for a module cache.
 */
async function fetchUserStatsRest(locale: string): Promise<UserStatsRest> {
  const [streakDTO, statsDTO, categories] = await Promise.all([
    apiGet<StreakDTO>("me/streak"),
    apiGet<StatsResponseDTO>("me/stats"),
    apiGet<CategoryDTO[]>(`categories?locale=${encodeURIComponent(locale)}`),
  ]);

  const namesByCode = new Map(categories.map((category) => [category.code, category.name]));
  const categoryMastery = statsDTO.categories.map((category) => {
    const name = namesByCode.get(category.category_code) ?? category.category_code;
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

  return {
    streak: {
      current_streak: streakDTO.current,
      max_streak: streakDTO.best,
      today_answered: streakDTO.today_done,
      daily_target: streakDTO.daily_goal,
      last_active_date: streakDTO.last_active_date,
    },
    stats: {
      readiness_pct: statsDTO.readiness_pct,
      due_questions_count: statsDTO.due_count,
      total_answered: statsDTO.categories.reduce((sum, category) => sum + category.seen, 0),
      total_correct: statsDTO.categories.reduce((sum, category) => sum + category.correct, 0),
      category_mastery: categoryMastery,
      pass_estimate: statsDTO.pass_estimate ?? null,
    },
  };
}

export function useUserStats() {
  const locale = useLocale();
  const meQuery = useMeQuery();
  const restQuery = useQuery({
    queryKey: ["user-stats-rest", locale],
    queryFn: () => fetchUserStatsRest(locale),
    staleTime: 30_000,
    placeholderData: (previousData) => previousData,
    retry: false,
  });

  const loading = meQuery.isPending || restQuery.isPending;
  const error = meQuery.error
    ? queryErrorMessage(meQuery.error)
    : restQuery.error
      ? queryErrorMessage(restQuery.error)
      : null;

  const mappedMe = meQuery.data ? mapMe(meQuery.data) : { user: null, entitlement: null };
  const rest = restQuery.data;

  return {
    user: error ? null : mappedMe.user,
    entitlement: error ? null : mappedMe.entitlement,
    streak: error || !rest ? null : rest.streak,
    stats: error || !rest ? null : rest.stats,
    loading,
    error,
    refetch: async () => {
      await Promise.all([meQuery.refetch(), restQuery.refetch()]);
    },
  };
}

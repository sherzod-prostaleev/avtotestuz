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

const categoriesCache: Record<string, CategoryDTO[]> = {};
let streakCache: { data: StreakDTO; time: number } | null = null;
let statsCache: { data: StatsResponseDTO; time: number } | null = null;

export function clearUserStatsModuleCacheForTests(): void {
  for (const k in categoriesCache) delete categoriesCache[k];
  streakCache = null;
  statsCache = null;
}

async function fetchUserStatsRest(locale: string): Promise<UserStatsRest> {
  const now = Date.now();
  const getStreak = async (): Promise<StreakDTO> => {
    if (streakCache && now - streakCache.time < 30_000) return streakCache.data;
    const data = await apiGet<StreakDTO>("me/streak");
    streakCache = { data, time: Date.now() };
    return data;
  };
  const getStats = async (): Promise<StatsResponseDTO> => {
    if (statsCache && now - statsCache.time < 30_000) return statsCache.data;
    const data = await apiGet<StatsResponseDTO>("me/stats");
    statsCache = { data, time: Date.now() };
    return data;
  };
  const getCategories = async (): Promise<CategoryDTO[]> => {
    if (categoriesCache[locale]) return categoriesCache[locale];
    const data = await apiGet<CategoryDTO[]>(`categories?locale=${encodeURIComponent(locale)}`);
    categoriesCache[locale] = data;
    return data;
  };

  const [streakDTO, statsDTO, categories] = await Promise.all([
    getStreak(),
    getStats(),
    getCategories(),
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

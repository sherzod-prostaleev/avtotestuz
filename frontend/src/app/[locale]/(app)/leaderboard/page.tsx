"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { RefreshCw, Trophy } from "lucide-react";
import { BackLink } from "@/components/layout/back-link";
import { motion } from "motion/react";



export type LeaderboardPeriod = "daily" | "weekly" | "monthly" | "alltime";

export interface LeaderboardEntry {
  rank: number;
  name: string;
  score: number;
}

export interface LeaderboardYou {
  rank: number | null;
  score: number;
  name: string;
}

export interface LeaderboardResponse {
  period: LeaderboardPeriod;
  you: LeaderboardYou;
  top: LeaderboardEntry[];
  around_you: LeaderboardEntry[];
}

const PERIODS: { value: LeaderboardPeriod; labelKey: string }[] = [
  { value: "daily", labelKey: "periodDaily" },
  { value: "weekly", labelKey: "periodWeekly" },
  { value: "monthly", labelKey: "periodMonthly" },
  { value: "alltime", labelKey: "periodAlltime" },
];

/** Medal styling for the top 3 ranks; every other rank falls back to a plain badge. */
const RANK_BADGE_STYLES: Record<number, string> = {
  1: "bg-gold text-slate-950 border-amber-600",
  2: "bg-slate-300 text-slate-900 border-slate-400",
  3: "bg-amber-700/80 text-white border-amber-800",
};

function RankBadge({ rank }: { rank: number }) {
  const style = RANK_BADGE_STYLES[rank] ?? "bg-background text-muted-foreground border-border";
  return (
    <span
      className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full border text-xs font-extrabold ${style}`}
    >
      {rank}
    </span>
  );
}

function EntryRow({ entry, highlighted, youBadge }: { entry: LeaderboardEntry; highlighted: boolean; youBadge: string }) {
  return (
    <li
      className={`flex items-center gap-3 rounded-xl px-3 py-2.5 transition-colors ${
        highlighted ? "border border-accent bg-accent/10" : "border border-transparent"
      }`}
    >
      <RankBadge rank={entry.rank} />
      <span className="min-w-0 flex-1 truncate text-sm font-bold text-foreground">{entry.name}</span>
      {highlighted && (
        <span className="shrink-0 rounded-full bg-accent/20 px-2 py-0.5 text-[10px] font-extrabold text-accent">
          {youBadge}
        </span>
      )}
      <span className="shrink-0 font-display text-sm font-extrabold text-gold">{entry.score}</span>
    </li>
  );
}

export interface LeaderboardPageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/leaderboard/page.tsx):
  // a station never appears in the rankings (leaderboard.Service.RecordPoint
  // returns early for a kind='station' profile), so this page has no
  // dashboard, premium, checkout or profile surface of its own to worry
  // about — the only navigation is the back link, which must stay inside
  // /station instead of the learner app's gated /dashboard.
  kiosk?: boolean;
}

export default function LeaderboardPage({ kiosk = false }: LeaderboardPageProps = {}) {
  const t = useTranslations("Leaderboard");
  const locale = useLocale();
  const backHref = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;

  const [period, setPeriod] = useState<LeaderboardPeriod>("weekly");
  const [data, setData] = useState<LeaderboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async (p: LeaderboardPeriod) => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<LeaderboardResponse>(`leaderboard?period=${p}`);
      setData(res);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(period);
  }, [load, period]);

  return (
    <main className="page-shell-narrow">
      <header className="mb-6">
        <BackLink href={backHref} kiosk={kiosk}>{t("backHome")}</BackLink>
        <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-tight">
          <Trophy aria-hidden="true" className="h-6 w-6 text-gold" />
          {t("title")}
        </h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      <div
        role="radiogroup"
        aria-label={t("periodGroupLabel")}
        className="mb-6 grid grid-cols-2 gap-2 sm:grid-cols-4"
      >
        {PERIODS.map((item) => {
          const isSelected = period === item.value;
          return (
            <button
              key={item.value}
              type="button"
              role="radio"
              aria-checked={isSelected}
              onClick={() => setPeriod(item.value)}
              className={`filter-chip relative w-full transition-colors ${
                isSelected ? "font-extrabold text-accent-foreground" : "filter-chip-idle font-bold"
              }`}
            >
              {isSelected && (
                <motion.span
                  layoutId="leaderboard-period-pill"
                  transition={{ type: "spring", stiffness: 350, damping: 30 }}
                  className="absolute inset-0 rounded-xl bg-accent shadow-3d"
                />
              )}
              <span className="relative z-10">{t(item.labelKey)}</span>
            </button>
          );
        })}
      </div>


      {loading ? (
        <p role="status" className="py-12 text-center text-sm text-muted-foreground">
          {t("loading")}
        </p>
      ) : error ? (
        <div role="alert" className="rounded-xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          <p>{t("loadError")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void load(period)}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
          </Button>
        </div>
      ) : data ? (
        <div className="space-y-6">
          {/* Your rank card */}
          <Card className="overflow-hidden border-gold/40 bg-card p-5 sm:p-6">
            <p className="mb-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("yourPositionLabel")}
            </p>
            <div className="flex items-center gap-4">
              <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border border-gold/40 bg-gold/10 font-display text-xl font-extrabold text-gold">
                {data.you.rank === null ? t("rankNone") : t("rankValue", { rank: data.you.rank })}
              </span>
              <span className="min-w-0 flex-1 truncate font-display text-lg font-bold text-foreground">
                {data.you.name}
              </span>
              <span className="shrink-0 text-right">
                <span className="font-display text-2xl font-extrabold text-gold">{data.you.score}</span>
              </span>
            </div>
            {data.you.rank === null && (
              <p className="mt-4 text-xs text-muted-foreground">{t("notRankedYet")}</p>
            )}
          </Card>

          {/* Top 10 */}
          <Card className="p-6">
            <CardHeader className="p-0 mb-4">
              <CardTitle className="text-lg font-bold">{t("topTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {data.top.length === 0 ? (
                <p className="py-6 text-center text-sm text-muted-foreground">{t("topEmpty")}</p>
              ) : (
                <ol className="space-y-1">
                  {data.top.map((entry) => (
                    <EntryRow
                      key={entry.rank}
                      entry={entry}
                      highlighted={data.you.rank === entry.rank}
                      youBadge={t("youBadge")}
                    />
                  ))}
                </ol>
              )}
            </CardContent>
          </Card>

          {/* Around you — only when the caller falls outside the top 10 */}
          {data.around_you.length > 0 && (
            <Card className="p-6">
              <CardHeader className="p-0 mb-4">
                <CardTitle className="text-lg font-bold">{t("aroundYouTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <ol className="space-y-1">
                  {data.around_you.map((entry) => (
                    <EntryRow
                      key={entry.rank}
                      entry={entry}
                      highlighted={data.you.rank === entry.rank}
                      youBadge={t("youBadge")}
                    />
                  ))}
                </ol>
              </CardContent>
            </Card>
          )}
        </div>
      ) : null}
    </main>
  );
}

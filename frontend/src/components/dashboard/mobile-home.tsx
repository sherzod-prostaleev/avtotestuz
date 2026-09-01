"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { AlertTriangle, Flame, RotateCcw, Target } from "lucide-react";
import type { SessionSummary } from "@/hooks/use-session-history";
import { DailyPlanCard } from "./daily-plan-card";

export interface MobileHomeProps {
  userName: string;
  readinessPct: number;
  currentStreak: number;
  todayAnswered: number;
  dailyTarget: number;
  dueCount: number;
  totalAnswered: number;
  weakest: { code: string; name: string; mastery_pct: number } | null;
  sessions: SessionSummary[];
  resumeSession: SessionSummary | null;
  /** Stats and history are still in flight — show shapes, never guessed numbers. */
  loading: boolean;
  className?: string;
}

/**
 * The phone home screen, in the order the approved design has it: who you are
 * and where you stand, today's plan, an unfinished session if there is one,
 * then three plain numbers.
 *
 * A component of its own rather than a reflow of the wide dashboard, because
 * that one keeps its eleven blocks and must stay pixel-identical — it is
 * simply `max-md:hidden` at the call site while this renders `md:hidden`.
 */
export function MobileHome({
  userName,
  readinessPct,
  currentStreak,
  todayAnswered,
  dailyTarget,
  dueCount,
  totalAnswered,
  weakest,
  sessions,
  resumeSession,
  loading,
  className = "",
}: MobileHomeProps) {
  const t = useTranslations("Dashboard");
  const locale = useLocale();

  const tiles = [
    { key: "streak", Icon: Flame, tone: "text-streak", value: currentStreak, caption: t("homeTileStreak") },
    { key: "answered", Icon: Target, tone: "text-success", value: totalAnswered, caption: t("homeTileAnswered") },
    { key: "due", Icon: AlertTriangle, tone: "text-danger", value: dueCount, caption: t("homeTileDue") },
  ];

  return (
    <div className={`flex flex-col gap-2.5 ${className}`}>
      <header>
        <h1 suppressHydrationWarning className="font-display text-2xl font-extrabold leading-tight tracking-tight">
          {loading ? (
            <span aria-hidden="true" className="inline-block h-7 w-44 max-w-full animate-pulse rounded-lg bg-border/60 align-middle" />
          ) : (
            t("homeGreeting", { name: userName })
          )}
        </h1>
        {/* Readiness and streak are guesses until `me/stats` and the session
            history both land; showing "0%" first and correcting it is worse
            than showing nothing. */}
        {loading ? (
          <span aria-hidden="true" className="mt-1 block h-4 w-60 max-w-full animate-pulse rounded bg-border/50" />
        ) : (
          <p suppressHydrationWarning className="text-sm leading-snug text-muted-foreground">
            {currentStreak > 0
              ? t.rich("homeStatus", {
                  percent: readinessPct,
                  days: currentStreak,
                  b: (chunks) => <strong className="font-bold text-accent">{chunks}</strong>,
                })
              : t.rich("homeStatusNoStreak", {
                  percent: readinessPct,
                  b: (chunks) => <strong className="font-bold text-accent">{chunks}</strong>,
                })}
          </p>
        )}
      </header>

      {loading ? (
        <div
          aria-hidden="true"
          className="surface-raised h-[298px] animate-pulse rounded-2xl border border-border bg-card"
        />
      ) : (
        <DailyPlanCard
          todayAnswered={todayAnswered}
          dailyTarget={dailyTarget}
          dueCount={dueCount}
          weakest={weakest}
          sessions={sessions}
        />
      )}

      {/* Only while a session is genuinely open — a dashed row, deliberately
          quieter than the plan's button. */}
      {!loading && resumeSession && (
        <Link
          href={`/${locale}/session/${encodeURIComponent(resumeSession.id)}`}
          className="flex min-h-12 items-center gap-2.5 rounded-xl border border-dashed border-border bg-card/60 px-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <RotateCcw aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
          <span className="min-w-0 flex-1 truncate text-sm font-semibold text-muted-foreground">
            {t("homeResume")}
          </span>
          <span className="shrink-0 text-sm font-bold text-accent">{t("homeResumeCta")}</span>
        </Link>
      )}

      <div suppressHydrationWarning className="grid grid-cols-3 gap-1.5">
        {tiles.map(({ key, Icon, tone, value, caption }) => (
          <div key={key} className="surface-raised-sm rounded-xl border border-border bg-card p-2">
            <Icon aria-hidden="true" className={`h-4 w-4 ${tone}`} />
            {loading ? (
              <span aria-hidden="true" className="mt-0.5 block h-5 w-8 animate-pulse rounded bg-border/60" />
            ) : (
              <p className="font-display text-xl font-extrabold leading-none tabular-nums">{value}</p>
            )}
            <p className="mt-1 text-xs leading-[1.15] text-muted-foreground">{caption}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

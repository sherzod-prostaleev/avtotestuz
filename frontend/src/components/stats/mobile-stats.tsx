"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { CheckCircle2, Flame, RotateCcw } from "lucide-react";
import type { SessionSummary } from "@/hooks/use-session-history";

export interface MobileStatsWeakTopic {
  code: string;
  name: string;
  mastery_pct: number;
}

export interface MobileStatsProps {
  readinessPct: number;
  passPct: number | null;
  currentStreak: number;
  maxStreak: number;
  accuracyPct: number;
  totalAnswered: number;
  dueCount: number;
  repeatHref: string;
  /** Already sorted weakest-first; only the first three are drawn. */
  weakest: MobileStatsWeakTopic[];
  sessions: SessionSummary[];
  modeLabels: Record<SessionSummary["mode"], string>;
  historyLoading: boolean;
  historyError: boolean;
  className?: string;
}

type Tone = "success" | "gold" | "accent" | "danger";

// Written out in full: Tailwind reads class names as literal text, so a tone
// assembled at runtime never reaches the stylesheet.
const TONE_TEXT: Record<Tone, string> = {
  success: "text-success",
  gold: "text-gold",
  accent: "text-accent",
  danger: "text-danger",
};

const TONE_BG: Record<Tone, string> = {
  success: "bg-success",
  gold: "bg-gold",
  accent: "bg-accent",
  danger: "bg-danger",
};

/** Same buckets `MasteryBar` uses, so a topic keeps its colour across screens. */
function masteryTone(pct: number): Tone {
  if (pct >= 80) return "success";
  if (pct >= 40) return "gold";
  return "danger";
}

function sessionTone(session: SessionSummary): Tone {
  if (session.status === "failed") return "danger";
  if (session.score === undefined || session.total <= 0) return "accent";
  return (session.score / session.total) * 100 >= 80 ? "success" : "accent";
}

function clampPct(value: number): number {
  return Math.min(100, Math.max(0, value));
}

export function MobileStats({
  readinessPct,
  passPct,
  currentStreak,
  maxStreak,
  accuracyPct,
  totalAnswered,
  dueCount,
  repeatHref,
  weakest,
  sessions,
  modeLabels,
  historyLoading,
  historyError,
  className = "",
}: MobileStatsProps) {
  const t = useTranslations("Stats");
  const recentSessions = sessions.slice(0, 3);

  /** "bugun" / "kecha" / "N kun" — compared by local calendar day, not by hours. */
  function relativeDay(iso: string): string {
    const startOfDay = (date: Date) =>
      new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
    const then = new Date(iso);
    if (Number.isNaN(then.getTime())) return "";
    const days = Math.round((startOfDay(new Date()) - startOfDay(then)) / 86_400_000);
    if (days <= 0) return t("recentToday");
    if (days === 1) return t("recentYesterday");
    return t("recentDaysAgo", { count: days });
  }

  return (
    <div className={`flex flex-col gap-2.5 ${className}`}>
      {/* Readiness, with the streak record and the accuracy folded in — on a
          phone these were three separate cards costing a screen of their own. */}
      <div className="surface-raised flex flex-col gap-2 rounded-2xl border border-border bg-card p-3">
        <div className="flex items-baseline gap-2.5">
          <span className="font-display text-[38px] font-extrabold leading-none text-accent">
            {readinessPct}%
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-bold leading-snug">{t("readiness")}</p>
            <p className="truncate text-xs leading-snug text-muted-foreground">
              {passPct === null
                ? t("passEstimateTitle")
                : t("passEstimateCompact", { pct: passPct })}
            </p>
          </div>
        </div>
        <span aria-hidden="true" className="block h-2 overflow-hidden rounded-full bg-border">
          <span
            className="block h-full rounded-full bg-accent"
            style={{ width: `${clampPct(readinessPct)}%` }}
          />
        </span>
        <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1">
          <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
            <Flame aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-streak" />
            {t.rich("streakCompact", {
              days: currentStreak,
              max: maxStreak,
              b: (chunks) => <strong className="font-bold text-foreground">{chunks}</strong>,
            })}
          </span>
          <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
            <CheckCircle2 aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-success" />
            {t.rich("accuracyCompact", {
              pct: accuracyPct,
              count: totalAnswered,
              b: (chunks) => <strong className="font-bold text-foreground">{chunks}</strong>,
            })}
          </span>
        </div>
      </div>

      {dueCount > 0 && (
        // Padding and type size are the design's own values rather than the
        // scale's nearest step: the app's root font is 6% larger than the
        // artboard's absolute px, and at `text-sm` this sentence loses its
        // last word to the ellipsis.
        <div className="flex items-center gap-2.5 rounded-2xl border border-danger/35 bg-danger/[0.06] py-2 pl-3.5 pr-2">
          <RotateCcw aria-hidden="true" className="h-5 w-5 shrink-0 text-danger" />
          {/* Clamped rather than truncated: in uz-Latn this is one line with a
              fraction of a pixel to spare, and Russian is half a line longer —
              two lines keep the count readable and the row inside the height
              the design draws. */}
          <span className="line-clamp-2 min-w-0 flex-1 text-[15px] font-semibold leading-snug">
            {t("dueQuestions", { count: dueCount })}
          </span>
          <Link
            href={repeatHref}
            className="btn-3d-primary inline-flex min-h-11 shrink-0 items-center rounded-xl px-3 text-[15px] font-extrabold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("startRepeatShort")}
          </Link>
        </div>
      )}

      {weakest.length > 0 && (
        <div>
          <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
            {t("weakestTopicsTitle")}
          </p>
          <div className="overflow-hidden rounded-2xl border border-border bg-card">
            {weakest.slice(0, 3).map((topic, index) => {
              const tone = masteryTone(topic.mastery_pct);
              return (
                <div
                  key={topic.code}
                  className={`px-3 py-2 ${index > 0 ? "border-t border-border/60" : ""}`}
                >
                  <div className="flex items-baseline gap-2">
                    <span className="min-w-0 flex-1 truncate text-sm font-semibold leading-snug">
                      {topic.name}
                    </span>
                    <span className={`text-xs font-bold tabular-nums ${TONE_TEXT[tone]}`}>
                      {topic.mastery_pct}%
                    </span>
                  </div>
                  <span
                    aria-hidden="true"
                    className="mt-1 block h-[5px] overflow-hidden rounded-full bg-border"
                  >
                    <span
                      className={`block h-full rounded-full ${TONE_BG[tone]}`}
                      style={{ width: `${clampPct(topic.mastery_pct)}%` }}
                    />
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div>
        <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
          {t("recentSessionsTitle")}
        </p>
        <div className="overflow-hidden rounded-2xl border border-border bg-card">
          {historyLoading ? (
            <div aria-hidden="true" className="flex flex-col gap-2 p-3">
              {[0, 1, 2].map((row) => (
                <span key={row} className="block h-5 w-full animate-pulse rounded bg-border/60" />
              ))}
            </div>
          ) : historyError ? (
            <p className="px-3 py-3 text-xs text-danger">{t("historyLoadError")}</p>
          ) : recentSessions.length === 0 ? (
            <p className="px-3 py-3 text-xs text-muted-foreground">{t("historyEmpty")}</p>
          ) : (
            recentSessions.map((session, index) => {
              const tone = sessionTone(session);
              return (
                <div
                  key={session.id}
                  className={`flex min-h-10 items-center gap-2.5 px-3 ${
                    index > 0 ? "border-t border-border/60" : ""
                  }`}
                >
                  <span
                    aria-hidden="true"
                    className={`h-[7px] w-[7px] shrink-0 rounded-full ${TONE_BG[tone]}`}
                  />
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold">
                    {modeLabels[session.mode]}
                  </span>
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {relativeDay(session.started_at)}
                  </span>
                  <span
                    className={`w-[52px] shrink-0 text-right text-sm font-bold tabular-nums ${TONE_TEXT[tone]}`}
                  >
                    {session.score === undefined ? "—" : `${session.score}/${session.total}`}
                  </span>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

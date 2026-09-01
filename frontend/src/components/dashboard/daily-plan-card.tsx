"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Check, ChevronRight } from "lucide-react";
import type { SessionSummary } from "@/hooks/use-session-history";
import { buildDailyPlan, type DailyPlanStepKey } from "./daily-plan";

export interface DailyPlanCardProps {
  todayAnswered: number;
  dailyTarget: number;
  dueCount: number;
  /** Weakest category, when stats have arrived — names the daily step's work. */
  weakest: { code: string; name: string; mastery_pct: number } | null;
  sessions: SessionSummary[];
  className?: string;
}

/**
 * The phone dashboard's whole job: three steps for today, exactly one button.
 *
 * Kept out of the desktop layout (`md:hidden` at the call site) — the wide
 * dashboard has room for the full set of cards and is not what learners
 * complained about.
 *
 * The exam meta line is a display copy of backend/internal/session/rules.go,
 * same as the exam picker's, and must be changed together with it.
 */
export function DailyPlanCard({
  todayAnswered,
  dailyTarget,
  dueCount,
  weakest,
  sessions,
  className = "",
}: DailyPlanCardProps) {
  const t = useTranslations("Dashboard");
  const locale = useLocale();
  const plan = buildDailyPlan({ todayAnswered, dailyTarget, dueCount, sessions });

  const href: Record<DailyPlanStepKey, string> = {
    daily: weakest
      ? `/${locale}/session/start?mode=practice&category_id=${encodeURIComponent(weakest.code)}&count=20`
      : `/${locale}/practice`,
    review: `/${locale}/session/start?mode=review&count=${Math.min(Math.max(dueCount, 1), 20)}`,
    exam: `/${locale}/exam`,
  };

  const title: Record<DailyPlanStepKey, string> = {
    daily: t("planStepDaily", { target: dailyTarget }),
    review: t("planStepReview"),
    exam: t("planStepExam"),
  };

  const caption: Record<DailyPlanStepKey, string> = {
    daily: weakest
      ? t("planStepDailyWeak", { category: weakest.name, percent: weakest.mastery_pct })
      : t("planStepDailyAny"),
    review: dueCount > 0 ? t("planStepReviewReady", { count: dueCount }) : t("planStepReviewEmpty"),
    exam: t("planStepExamMeta"),
  };

  const doneCaption: Record<DailyPlanStepKey, string> = {
    daily: t("planStepDailyProgress", { done: todayAnswered, target: dailyTarget }),
    review: t("planStepReviewEmpty"),
    exam: t("planStepExamDone"),
  };

  return (
    <section
      aria-label={t("planTitle")}
      className={`surface-raised overflow-hidden rounded-2xl border border-border bg-card ${className}`}
    >
      <div className="flex items-center justify-between gap-2 px-3.5 pb-2.5 pt-3">
        <h2 className="text-xs font-extrabold uppercase tracking-[0.08em] text-muted-foreground">
          {t("planTitle")}
        </h2>
        <span className="text-xs font-bold tabular-nums text-accent">
          {t("planProgress", { done: plan.doneCount, total: plan.steps.length })}
        </span>
      </div>

      {plan.steps.map((step, index) => {
        const isLast = index === plan.steps.length - 1;

        if (step.active) {
          return (
            // The one primary action on the screen. Everything else on this
            // card is a status line.
            <div key={step.key} className="flex flex-col gap-2.5 bg-accent/5 p-3.5">
              <div className="flex items-start gap-2.5">
                <span
                  aria-hidden="true"
                  className="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-full border-2 border-accent text-xs font-extrabold leading-none text-accent"
                >
                  {index + 1}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-base font-extrabold leading-snug">{title[step.key]}</p>
                  <p className="text-sm leading-snug text-muted-foreground">{caption[step.key]}</p>
                </div>
              </div>
              <Link
                href={href[step.key]}
                className="btn-3d-primary inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl px-4 font-display text-lg font-extrabold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card"
              >
                {t("planStart")}
                <ChevronRight aria-hidden="true" className="h-5 w-5" />
              </Link>
            </div>
          );
        }

        return (
          <Link
            key={step.key}
            href={href[step.key]}
            className={`flex min-h-12 items-center gap-2.5 px-3.5 py-2.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
              isLast ? "" : "border-b border-border"
            }`}
          >
            {step.done ? (
              <span
                aria-hidden="true"
                className="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-full bg-success text-success-foreground"
              >
                <Check className="h-4 w-4" strokeWidth={3} />
              </span>
            ) : (
              <span
                aria-hidden="true"
                className="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-full border-2 border-border text-xs font-extrabold leading-none text-muted-foreground"
              >
                {index + 1}
              </span>
            )}
            <span className="min-w-0 flex-1">
              <span
                className={`block text-sm font-bold ${
                  step.done ? "text-muted-foreground line-through" : "text-muted-foreground"
                }`}
              >
                {title[step.key]}
              </span>
              <span className="block text-xs leading-snug text-muted-foreground">
                {step.done ? doneCaption[step.key] : caption[step.key]}
              </span>
            </span>
            <ChevronRight aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
          </Link>
        );
      })}

      {plan.activeKey === null && (
        <p className="border-t border-border px-3.5 py-2.5 text-sm font-bold text-success">
          {t("planAllDone")}
        </p>
      )}
    </section>
  );
}

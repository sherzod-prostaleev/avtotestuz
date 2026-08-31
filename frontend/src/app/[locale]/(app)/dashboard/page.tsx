"use client";

import { useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useVariantCount } from "@/hooks/use-variant-count";
import { useUserStats } from "@/hooks/use-user-stats";
import { useSessionHistory, type SessionSummary } from "@/hooks/use-session-history";
import { apiGet } from "@/lib/api-client";
import { mistakesCountStore } from "@/lib/dashboard-stores";
import { formatDateWithTime } from "@/lib/date-format";
import { MasteryBar } from "@/components/shared/mastery-bar";
import { GrandMockCard } from "@/components/mock/grand-mock-card";
import { Card, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  BookOpen,
  Award,
  Sparkles,
  Flame,
  Crown,
  Target,
  AlertTriangle,
  Signpost,
  ChevronRight,
  Bookmark,
  CheckCircle2,
  Hand,
  PlayCircle,
  Clock3,
  ListChecks,
  BrainCircuit,
  RefreshCw,
  Route,
  X,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

const ONBOARDING_DISMISS_KEY = "dg-onboarding-v1-dismissed";

const resumableModes: SessionSummary["mode"][] = [
  "variant",
  "exam",
  "practice",
  "mistakes",
  "grand_mock",
  "review",
  "placement",
];
const resumeModeKeys: Record<SessionSummary["mode"], string> = {
  variant: "resumeModeVariant",
  exam: "resumeModeExam",
  practice: "resumeModePractice",
  mistakes: "resumeModeMistakes",
  grand_mock: "resumeModeGrandMock",
  review: "resumeModeReview",
  placement: "resumeModePlacement",
};

function newestInProgressSession(sessions: SessionSummary[]): SessionSummary | null {
  return sessions.reduce<SessionSummary | null>((newest, session) => {
    const startedAt = Date.parse(session.started_at);
    const isValid =
      session.status === "in_progress" &&
      typeof session.id === "string" &&
      session.id.trim().length > 0 &&
      resumableModes.includes(session.mode) &&
      Number.isInteger(session.total) &&
      session.total > 0 &&
      Number.isFinite(startedAt);

    if (!isValid) return newest;
    if (!newest || startedAt > Date.parse(newest.started_at)) return session;
    return newest;
  }, null);
}

type CategoryMasteryItem = NonNullable<NonNullable<ReturnType<typeof useUserStats>["stats"]>["category_mastery"]>[number];

function weakestCategories(categories: CategoryMasteryItem[]) {
  return [...categories].sort((left, right) => {
    if (left.mastery_pct !== right.mastery_pct) {
      return left.mastery_pct - right.mastery_pct;
    }
    return left.name.localeCompare(right.name);
  });
}

function weakestCategory(categories: CategoryMasteryItem[]) {
  return weakestCategories(categories)[0] ?? null;
}

function isFinishedSession(session: SessionSummary) {
  return session.status === "passed" || session.status === "failed";
}

// Reserves the same footprint as a filled-in hero stat card (label + big
// number + caption) so those cards don't resize once real numbers replace
// the "0" defaults that `useUserStats` starts with.
function StatCardSkeleton() {
  return (
    <div aria-hidden="true" className="animate-pulse space-y-2">
      <div className="h-3.5 w-24 rounded bg-border/60" />
      <div className="h-7 w-14 rounded bg-border/60" />
      <div className="h-3 w-32 rounded bg-border/50" />
    </div>
  );
}

export default function DashboardPage() {
  const t = useTranslations("Dashboard");
  const ticketCount = useVariantCount();
  const savedT = useTranslations("Saved");
  const locale = useLocale();
  const { user, entitlement, streak, stats, loading, error } = useUserStats();
  const { sessions, loading: historyLoading, error: historyError } = useSessionHistory(20);
  const [onboardingVisible, setOnboardingVisible] = useState(false);
  const [mistakesDueCount, setMistakesDueCount] = useState(() => mistakesCountStore.get());

  // Hero and stats sections settle based on user profile & stats; session history
  // loads seamlessly in the background with in-memory caching.
  const isPersonalizationLoading = loading;

  const userName = user?.name || t("guestUser");
  const isVip = entitlement?.is_vip ?? false;
  const readinessPct = stats?.readiness_pct ?? 0;
  const passEstimate = stats?.pass_estimate;
  const currentStreak = streak?.current_streak ?? 0;
  const todayAnswered = streak?.today_answered ?? 0;
  const dailyTarget = streak?.daily_target ?? 30;
  const dueQuestionsCount = stats?.due_questions_count ?? 0;
  const totalCategories = stats?.category_mastery?.length ?? 0;
  const masteryCategories = stats?.category_mastery ? weakestCategories(stats.category_mastery).slice(0, 4) : [];
  const weakest = stats?.category_mastery ? weakestCategory(stats.category_mastery) : null;
  const resumeSession = newestInProgressSession(sessions);
  const resumeStartedAt = resumeSession
    ? formatDateWithTime(resumeSession.started_at)
    : null;
  const hasAnyProgress =
    sessions.some(isFinishedSession) || (stats?.total_answered ?? 0) > 0;
  // The onboarding list used to open with a placement test. A learner's level
  // now emerges from the practice they do, so the first step is the practice
  // picker itself.
  const onboardingHref = `/${locale}/practice`;

  useEffect(() => {
    try {
      if (window.localStorage.getItem(ONBOARDING_DISMISS_KEY) === "1") return;
      setOnboardingVisible(true);
    } catch {
      setOnboardingVisible(true);
    }
  }, []);

  // Mistakes card must show me/mistakes.due_count (lapses>0), not FSRS me/stats.due_count.
  useEffect(() => {
    let cancelled = false;
    void mistakesCountStore
      .load()
      .then((count) => {
        if (!cancelled) {
          setMistakesDueCount(count);
        }
      })
      .catch(() => {
        if (!cancelled) setMistakesDueCount(0);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const dismissOnboarding = () => {
    try {
      window.localStorage.setItem(ONBOARDING_DISMISS_KEY, "1");
    } catch {
      // Ignore quota / private-mode failures; still hide for this visit.
    }
    setOnboardingVisible(false);
  };

  const showOnboarding = onboardingVisible && !isPersonalizationLoading && !hasAnyProgress;

  const nextAction = resumeSession
    ? {
        icon: PlayCircle,
        title: t("nextActionResumeTitle"),
        description: t("nextActionResumeDesc"),
        cta: t("nextActionResumeCta"),
        href: `/${locale}/session/${encodeURIComponent(resumeSession.id)}`,
        tone: "accent" as const,
      }
    : dueQuestionsCount > 0
      ? {
          icon: RefreshCw,
          title: t("nextActionReviewTitle"),
          description: t("nextActionReviewDesc", { count: dueQuestionsCount }),
          cta: t("nextActionReviewCta"),
          href: `/${locale}/session/start?mode=review&count=${Math.min(dueQuestionsCount, 20)}`,
          tone: "danger" as const,
        }
      : readinessPct >= 80
        ? {
            icon: CheckCircle2,
            title: t("nextActionExamTitle"),
            description: t("nextActionExamDesc"),
            cta: t("nextActionExamCta"),
            href: `/${locale}/exam`,
            tone: "success" as const,
          }
        : weakest
          ? {
              icon: Route,
              title: t("nextActionPracticeTitle", { category: weakest.name }),
              description: t("nextActionPracticeDesc"),
              cta: t("nextActionPracticeCta"),
              href: `/${locale}/session/start?mode=practice&category_id=${encodeURIComponent(weakest.code)}&count=20`,
              tone: "accent" as const,
            }
          : {
              icon: BookOpen,
              title: t("nextActionTicketsTitle"),
              description: t("nextActionTicketsDesc"),
              cta: t("nextActionTicketsCta"),
              href: `/${locale}/tickets`,
              tone: "accent" as const,
            };
  const NextActionIcon = nextAction.icon;
  const nextActionToneClass =
    nextAction.tone === "success"
      ? "bg-success/15 text-success"
      : nextAction.tone === "danger"
        ? "bg-destructive/10 text-destructive"
        : "bg-accent/15 text-accent";

  return (
    <main suppressHydrationWarning className="page-shell space-y-3 sm:space-y-8">
      {showOnboarding && (
        <section
          aria-labelledby="onboarding-title"
          className="surface-raised relative overflow-hidden rounded-2xl border border-accent/35 bg-card p-3.5 sm:rounded-3xl sm:p-5 md:p-6"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="space-y-3">
              <h2 id="onboarding-title" className="font-display text-lg font-extrabold tracking-tight">
                {t("onboardingTitle")}
              </h2>
              <ul className="space-y-2 text-base leading-relaxed text-muted-foreground">
                <li className="flex items-start gap-2">
                  <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />
                  {t("onboardingBullet1")}
                </li>
                <li className="flex items-start gap-2">
                  <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />
                  {t("onboardingBullet2")}
                </li>
                <li className="flex items-start gap-2">
                  <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />
                  {t("onboardingBullet3")}
                </li>
              </ul>
              <Link
                href={onboardingHref}
                className="inline-flex min-h-12 items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-5 text-base font-extrabold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:border-b-0 active:shadow-none"
              >
                {t("onboardingCta")}
                <ChevronRight aria-hidden="true" className="ml-2 h-4 w-4" />
              </Link>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="shrink-0 px-2"
              onClick={dismissOnboarding}
              aria-label={t("onboardingDismiss")}
            >
              <X aria-hidden="true" className="h-4 w-4" />
            </Button>
          </div>
        </section>
      )}

      <section className="grid gap-2.5 sm:gap-4 lg:grid-cols-[1.35fr_0.9fr]">
        <section className="surface-raised relative min-w-0 overflow-hidden rounded-2xl border border-border bg-card p-3 sm:rounded-3xl sm:p-6 md:p-8">
          <div className="relative z-10 flex h-full flex-col justify-between gap-3 sm:gap-6">
            <div className="space-y-2 sm:space-y-4">
              <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
                <span className="inline-flex items-center gap-1 rounded-md border border-border bg-background px-2 py-1 text-[10px] font-bold text-muted-foreground sm:gap-1.5 sm:px-3 sm:py-1.5 sm:text-xs">
                  <BrainCircuit aria-hidden="true" className="h-3.5 w-3.5 text-accent sm:h-4 sm:w-4" />
                  {t("todayTitle")}
                </span>
                {loading ? (
                  <span
                    aria-hidden="true"
                    className="inline-flex h-7 w-24 animate-pulse items-center rounded-md border border-border bg-background px-3 py-1 sm:h-8 sm:w-28"
                  />
                ) : isVip ? (
                  <span className="inline-flex items-center gap-1 rounded-md border border-gold/40 bg-gold/15 px-2 py-1 text-[11px] font-bold text-gold sm:gap-1.5 sm:px-3 sm:py-1.5 sm:text-sm">
                    <Crown aria-hidden="true" className="h-3.5 w-3.5 sm:h-4 sm:w-4" /> {t("vipBadge")}
                  </span>
                ) : (
                  <Link href={`/${locale}/premium`}>
                    <span className="inline-flex items-center gap-1 rounded-md border border-border bg-accent/15 px-2 py-1 text-[11px] font-bold text-foreground hover:border-accent sm:gap-1.5 sm:px-3 sm:py-1.5 sm:text-sm">
                      <Sparkles aria-hidden="true" className="h-3.5 w-3.5 text-accent sm:h-4 sm:w-4" /> {t("upgradeVip")}
                    </span>
                  </Link>
                )}
              </div>

              <div className="flex min-w-0 items-start gap-2 sm:gap-3">
                <Hand aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-accent sm:mt-1 sm:h-7 sm:w-7" />
                <div className="min-w-0">
                  <h1 suppressHydrationWarning className="truncate font-display text-xl font-extrabold tracking-tight sm:text-3xl md:text-4xl">
                    {loading ? (
                      <span
                        aria-hidden="true"
                        className="inline-block h-7 w-40 max-w-full animate-pulse rounded-lg bg-border/60 align-middle sm:h-9 sm:w-56"
                      />
                    ) : (
                      t("welcomeUser", { name: userName })
                    )}
                  </h1>
                  <p className="mt-1 max-w-2xl text-xs leading-snug text-muted-foreground sm:mt-2 sm:text-base sm:leading-relaxed">
                    {t("welcomeSubtitle")}
                  </p>
                  {!loading ? (
                    <div className="mt-2 max-w-xs space-y-1.5">
                      <div
                        className="h-2 overflow-hidden rounded-full bg-border"
                        role="progressbar"
                        aria-valuemin={0}
                        aria-valuemax={dailyTarget}
                        aria-valuenow={Math.min(todayAnswered, dailyTarget)}
                        aria-label={t("streakToday", { done: todayAnswered, goal: dailyTarget })}
                      >
                        <div
                          className={`h-full rounded-full ${todayAnswered >= dailyTarget ? "bg-success" : "bg-accent"}`}
                          style={{
                            width: `${Math.min(100, dailyTarget > 0 ? (todayAnswered / dailyTarget) * 100 : 0)}%`,
                          }}
                        />
                      </div>
                      {todayAnswered < dailyTarget ? (
                        <>
                          <p className="text-xs font-bold text-accent sm:text-sm">
                            {t("habitKeepStreak", { remaining: Math.max(0, dailyTarget - todayAnswered) })}
                          </p>
                          <Link
                            href={nextAction.href}
                            // Can point at /session/start, which starts a session
                            // on mount — a prefetch of it renders server-side for
                            // nothing, and re-renders on every remount.
                            prefetch={false}
                            className="inline-flex min-h-11 items-center text-sm font-extrabold text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                          >
                            {t("habitContinueCta")}
                          </Link>
                        </>
                      ) : (
                        <p className="text-xs font-bold text-success sm:text-sm">{t("habitGoalMet")}</p>
                      )}
                    </div>
                  ) : null}
                </div>
              </div>
            </div>

            <div suppressHydrationWarning className="grid grid-cols-3 gap-1.5 sm:gap-3">
              <div className="surface-raised-sm min-w-0 rounded-xl border border-border bg-background p-2 sm:rounded-2xl sm:p-5">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-1 text-[10px] font-bold text-muted-foreground sm:gap-2 sm:text-sm">
                      <Flame aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-streak sm:h-5 sm:w-5" />
                      <span className="truncate">{t("streakCount", { count: currentStreak })}</span>
                    </div>
                    <p className="mt-1 font-display text-xl font-extrabold sm:mt-2 sm:text-3xl">{todayAnswered}</p>
                    <p className="mt-0.5 line-clamp-2 text-[10px] leading-snug text-muted-foreground sm:mt-1 sm:text-sm">
                      {t("streakToday", { done: todayAnswered, goal: dailyTarget })}
                    </p>
                  </>
                )}
              </div>

              <div className="surface-raised-sm min-w-0 rounded-xl border border-border bg-background p-2 sm:rounded-2xl sm:p-5">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-1 text-[10px] font-bold text-muted-foreground sm:gap-2 sm:text-sm">
                      <CheckCircle2 aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-success sm:h-5 sm:w-5" />
                      <span className="truncate">{t("readinessLabel")}</span>
                    </div>
                    <p className="mt-1 font-display text-xl font-extrabold sm:mt-2 sm:text-3xl">{readinessPct}%</p>
                    <p className="mt-0.5 line-clamp-2 text-[10px] leading-snug text-muted-foreground sm:mt-1 sm:text-sm">
                      {readinessPct >= 80 ? t("readyBadge") : t("notReadyBadge")}
                      {passEstimate != null && (
                        <>
                          {" · "}
                          {t("passEstimateLabel", { pct: passEstimate.estimated_pass_pct })}
                        </>
                      )}
                    </p>
                  </>
                )}
              </div>

              <div className="surface-raised-sm min-w-0 rounded-xl border border-border bg-background p-2 sm:rounded-2xl sm:p-5">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-1 text-[10px] font-bold text-muted-foreground sm:gap-2 sm:text-sm">
                      <AlertTriangle aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-danger sm:h-5 sm:w-5" />
                      <span className="truncate">{t("dueQuestionsLabel")}</span>
                    </div>
                    <p className="mt-1 font-display text-xl font-extrabold sm:mt-2 sm:text-3xl">{dueQuestionsCount}</p>
                    <p className="mt-0.5 line-clamp-2 text-[10px] leading-snug text-muted-foreground sm:mt-1 sm:text-sm">
                      {weakest ? t("weakestCategory", { category: weakest.name, percent: weakest.mastery_pct }) : t("weakestCategoryEmpty")}
                    </p>
                  </>
                )}
              </div>
            </div>
          </div>
        </section>

        <aside className="surface-raised min-w-0 rounded-2xl border border-border bg-card p-3 sm:rounded-3xl sm:p-6 md:p-7">
          {isPersonalizationLoading ? (
            // The recommendation below depends on stats + session history
            // together (readiness, weakest category, resumable session).
            // Rendering a guess from partial defaults would pick the wrong
            // action and then swap to the real one once both requests
            // settle, so this placeholder waits for both instead.
            <div aria-hidden="true" className="animate-pulse">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-2">
                  <div className="h-3 w-28 rounded-full bg-border/60" />
                  <div className="h-6 w-44 rounded bg-border/60" />
                </div>
                <div className="h-10 w-10 rounded-xl bg-border/60 sm:h-12 sm:w-12 sm:rounded-2xl" />
              </div>
              <div className="mt-3 space-y-2 sm:mt-4">
                <div className="h-3 w-full rounded bg-border/50" />
                <div className="h-3 w-3/4 rounded bg-border/50" />
              </div>
              <div className="mt-4 h-11 w-full rounded-xl bg-border/60 sm:mt-6 sm:h-12 sm:rounded-2xl" />
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-2 sm:gap-3">
                <div className="min-w-0">
                  <p className="text-[10px] font-bold uppercase tracking-[0.14em] text-muted-foreground sm:text-xs sm:tracking-[0.16em]">
                    {t("nextActionLabel")}
                  </p>
                  <h2 className="mt-1 truncate font-display text-lg font-extrabold tracking-tight sm:mt-2 sm:text-2xl">
                    {nextAction.title}
                  </h2>
                </div>
                <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl sm:h-14 sm:w-14 sm:rounded-2xl ${nextActionToneClass}`}>
                  <NextActionIcon aria-hidden="true" className="h-5 w-5 sm:h-7 sm:w-7" />
                </div>
              </div>

              <p className="mt-2 text-xs leading-snug text-muted-foreground sm:mt-4 sm:text-base sm:leading-relaxed">
                {nextAction.description}
              </p>

              <Link
                href={nextAction.href}
                prefetch={false}
                className="mt-3 inline-flex min-h-11 w-full items-center justify-center rounded-xl border-b-4 border-accent-shadow bg-accent px-4 text-sm font-extrabold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:border-b-0 active:shadow-none sm:mt-6 sm:min-h-13 sm:rounded-2xl sm:px-6 sm:text-base"
              >
                {nextAction.cta}
                <ChevronRight aria-hidden="true" className="ml-1.5 h-4 w-4 sm:ml-2 sm:h-5 sm:w-5" />
              </Link>
            </>
          )}

          <div className="surface-raised-sm mt-3 rounded-xl border border-border bg-background p-2.5 sm:mt-6 sm:rounded-2xl sm:p-4">
            <p className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground sm:text-sm">{t("studyLoopTitle")}</p>
            <div className="mt-2 flex flex-wrap gap-1.5 sm:mt-3 sm:gap-2">
              <span className="rounded-md bg-accent/15 px-2 py-1 text-[11px] font-semibold text-foreground sm:px-3 sm:py-1.5 sm:text-sm">{t("studyLoopRecall")}</span>
              <span className="rounded-md bg-gold/15 px-2 py-1 text-[11px] font-semibold text-gold sm:px-3 sm:py-1.5 sm:text-sm">{t("studyLoopSpacing")}</span>
              <span className="rounded-md bg-success/15 px-2 py-1 text-[11px] font-semibold text-success sm:px-3 sm:py-1.5 sm:text-sm">{t("studyLoopFeedback")}</span>
            </div>
          </div>
        </aside>
      </section>

      {error && (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          {t("loadError")}
        </div>
      )}

      {historyError && (
        <div
          role="alert"
          className="rounded-2xl border border-destructive/40 bg-destructive/5 px-5 py-4 text-sm font-medium text-destructive"
        >
          {t("resumeLoadError")}
        </div>
      )}

      <AnimatePresence>
        {resumeSession && resumeStartedAt && (
          <motion.section
            key={resumeSession.id}
            initial={{ opacity: 0, y: 10, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -10, scale: 0.98 }}
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            aria-labelledby="resume-session-title"
            className="surface-raised relative overflow-hidden rounded-3xl border border-accent/40 bg-card p-6 md:p-8"
          >
            <div className="relative flex flex-col items-start justify-between gap-6 md:flex-row md:items-center">
              <div className="flex items-start gap-4">
                <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-accent text-accent-foreground shadow-3d">
                  <PlayCircle aria-hidden="true" className="h-7 w-7" />
                </div>
                <div className="space-y-2">
                  <h2 id="resume-session-title" className="font-display text-xl font-extrabold tracking-tight md:text-2xl">
                    {t("resumeTitle")}
                  </h2>
                  <p className="max-w-xl text-base leading-relaxed text-muted-foreground">{t("resumeDescription")}</p>
                  <div className="flex flex-wrap items-center gap-2 text-sm font-bold">
                    <span className="rounded-full border border-accent/30 bg-accent/10 px-3 py-1.5 text-accent">
                      {t(resumeModeKeys[resumeSession.mode])}
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-background/80 px-3 py-1.5 text-muted-foreground">
                      <Clock3 aria-hidden="true" className="h-4 w-4" />
                      {t("resumeStartedAt", { date: resumeStartedAt })}
                    </span>
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-background/80 px-3 py-1.5 text-muted-foreground">
                      <ListChecks aria-hidden="true" className="h-4 w-4" />
                      {t("resumeTotal", { total: resumeSession.total })}
                    </span>
                  </div>
                </div>
              </div>
              <Link
                href={`/${locale}/session/${encodeURIComponent(resumeSession.id)}`}
                // The session page loads its own state on mount; the prefetched
                // shell is thrown away and only costs a server render.
                prefetch={false}
                className="inline-flex min-h-13 w-full shrink-0 items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-7 text-base font-extrabold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:border-b-0 active:shadow-none md:w-auto"
              >
                {t("resumeCta")}
                <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
              </Link>
            </div>
          </motion.section>
        )}
      </AnimatePresence>

      {/* 4 Main Mode Cards — dense 2×2 on phones (competitor-style compact tiles) */}
      <section className="space-y-2 sm:space-y-4">
        <h2 className="font-display text-lg font-bold tracking-tight sm:text-2xl">{t("modesTitle")}</h2>
        <div className="grid grid-cols-2 gap-2 sm:gap-4 lg:grid-cols-4">
          {/* 1. Biletlar Grid */}
          <Link href={`/${locale}/tickets`} className="block min-w-0">
            <Card className="glass-card group flex h-full flex-col justify-between p-3 sm:p-5 md:p-6">
              <div>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-xl bg-accent/15 text-accent transition-transform group-hover:scale-105 sm:mb-3 sm:h-12 sm:w-12 sm:rounded-2xl">
                  <BookOpen aria-hidden="true" className="h-4 w-4 sm:h-6 sm:w-6" />
                </div>
                <CardTitle className="text-sm sm:text-lg">{t("navVariantsTitle")}</CardTitle>
              </div>
              <div className="mt-2 flex items-center justify-between text-[11px] font-bold text-accent sm:mt-4 sm:text-sm">
                <span className="truncate">{t("variantsMeta", { count: ticketCount })}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-1 sm:h-5 sm:w-5" />
              </div>
            </Card>
          </Link>

          {/* 2. Imtihon Simulation */}
          <Link href={`/${locale}/exam`} className="block min-w-0">
            <Card className="glass-card group flex h-full flex-col justify-between border-accent/30 p-3 sm:p-5 md:p-6">
              <div>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-xl bg-accent/15 text-accent transition-transform group-hover:scale-105 sm:mb-3 sm:h-12 sm:w-12 sm:rounded-2xl">
                  <Award aria-hidden="true" className="h-4 w-4 sm:h-6 sm:w-6" />
                </div>
                <CardTitle className="text-sm sm:text-lg">{t("navExamTitle")}</CardTitle>
                <CardDescription className="mt-1 line-clamp-2 text-[11px] sm:mt-1.5 sm:text-sm">{t("navExamDesc")}</CardDescription>
              </div>
              <div className="mt-2 flex items-center justify-between text-[11px] font-bold text-accent sm:mt-4 sm:text-sm">
                <span className="truncate">{t("examMeta")}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-1 sm:h-5 sm:w-5" />
              </div>
            </Card>
          </Link>

          {/* 3. Mavzulashtirilgan testlar */}
          <Link href={`/${locale}/practice`} className="block min-w-0">
            <Card className="glass-card group flex h-full flex-col justify-between p-3 sm:p-5 md:p-6">
              <div>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-xl bg-success/15 text-success transition-transform group-hover:scale-105 sm:mb-3 sm:h-12 sm:w-12 sm:rounded-2xl">
                  <Target aria-hidden="true" className="h-4 w-4 sm:h-6 sm:w-6" />
                </div>
                <CardTitle className="text-sm sm:text-lg">{t("navPracticeTitle")}</CardTitle>
                <CardDescription className="mt-1 line-clamp-2 text-[11px] sm:mt-1.5 sm:text-sm">{t("navPracticeDesc")}</CardDescription>
              </div>
              <div className="mt-2 flex items-center justify-between text-[11px] font-bold text-success sm:mt-4 sm:text-sm">
                <span className="truncate">{t("practiceMeta")}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-1 sm:h-5 sm:w-5" />
              </div>
            </Card>
          </Link>

          {/* 4. Xatolar Banki */}
          <Link href={`/${locale}/mistakes`} className="block min-w-0">
            <Card className="glass-card group flex h-full flex-col justify-between p-3 sm:p-5 md:p-6">
              <div>
                <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-xl bg-danger/15 text-danger transition-transform group-hover:scale-105 sm:mb-3 sm:h-12 sm:w-12 sm:rounded-2xl">
                  <AlertTriangle aria-hidden="true" className="h-4 w-4 sm:h-6 sm:w-6" />
                </div>
                <CardTitle className="text-sm sm:text-lg">{t("navMistakesTitle")}</CardTitle>
                <CardDescription className="mt-1 line-clamp-2 text-[11px] sm:mt-1.5 sm:text-sm">{t("navMistakesDesc")}</CardDescription>
              </div>
              <div className="mt-2 flex items-center justify-between text-[11px] font-bold text-danger sm:mt-4 sm:text-sm">
                <span className="truncate">{t("mistakesMeta", { count: mistakesDueCount })}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-1 sm:h-5 sm:w-5" />
              </div>
            </Card>
          </Link>
        </div>
      </section>

      {/* Grand Mock — gated full exam simulation */}
      <section className="min-w-0">
        <GrandMockCard />
      </section>

      {/* Road Signs Banner */}
      <section className="min-w-0">
        <Link href={`/${locale}/signs`}>
          <Card className="glass-card border-border p-3 hover:border-accent sm:p-6">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2.5 sm:gap-4">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-accent text-accent-foreground shadow-3d sm:h-14 sm:w-14 sm:rounded-2xl">
                  <Signpost aria-hidden="true" className="h-5 w-5 sm:h-7 sm:w-7" />
                </div>
                <div className="min-w-0">
                  <h3 className="font-display text-sm font-bold sm:text-xl">{t("signsTitle")}</h3>
                  <p className="mt-0.5 line-clamp-2 text-[11px] leading-snug text-muted-foreground sm:text-sm sm:leading-relaxed">
                    {t("signsDescription")}
                  </p>
                </div>
              </div>
              <Button as="span" variant="game" size="sm" className="shrink-0 px-2.5 text-xs sm:px-3 sm:text-sm">
                {t("signsOpen")} <ChevronRight aria-hidden="true" className="ml-0.5 h-3.5 w-3.5 sm:ml-1 sm:h-4 sm:w-4" />
              </Button>
            </div>
          </Card>
        </Link>
      </section>

      {/* Saved Questions Entry */}
      <section className="min-w-0">
        <Link href={`/${locale}/saved`}>
          <Card className="glass-card border-gold/40 bg-card p-3 hover:border-gold sm:p-6">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2.5 sm:gap-4">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gold/20 text-gold sm:h-14 sm:w-14 sm:rounded-2xl">
                  <Bookmark aria-hidden="true" className="h-5 w-5 sm:h-7 sm:w-7" />
                </div>
                <div className="min-w-0">
                  <h3 className="font-display text-sm font-bold sm:text-xl">{savedT("title")}</h3>
                  <p className="mt-0.5 line-clamp-2 text-[11px] leading-snug text-muted-foreground sm:text-sm sm:leading-relaxed">
                    {savedT("subtitle")}
                  </p>
                </div>
              </div>
              <Button as="span" variant="outline" size="sm" className="shrink-0 px-2.5 text-xs sm:px-3 sm:text-sm">
                {savedT("open")} <ChevronRight aria-hidden="true" className="ml-0.5 h-3.5 w-3.5 sm:ml-1 sm:h-4 sm:w-4" />
              </Button>
            </div>
          </Card>
        </Link>
      </section>

      {/* Weakest Category Mastery — while stats are loading, `stats` is
          null and this section would otherwise be fully absent, then pop in
          below the signs/saved banners once the fetch resolves. A skeleton
          of the same shape keeps that spot reserved instead. */}
      {loading ? (
        <section className="min-w-0 space-y-2 sm:space-y-4" aria-hidden="true">
          <div className="h-5 w-48 max-w-full animate-pulse rounded bg-border/60 sm:h-6 sm:w-64" />
          <Card className="overflow-hidden p-3 sm:p-6">
            <div className="grid min-w-0 animate-pulse gap-3 sm:grid-cols-2 sm:gap-4">
              {[0, 1, 2, 3].map((i) => (
                <div key={i} className="min-w-0 space-y-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <div className="h-3 w-24 rounded bg-border/60 sm:h-3.5 sm:w-28" />
                    <div className="h-3 w-10 shrink-0 rounded bg-border/60 sm:h-3.5 sm:w-8" />
                  </div>
                  <div className="h-1.5 w-full rounded-full bg-border/50 sm:h-2" />
                </div>
              ))}
            </div>
          </Card>
        </section>
      ) : (
        masteryCategories.length > 0 && (
          <section className="min-w-0 space-y-2 sm:space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
              <h2 className="font-display text-lg font-bold tracking-tight sm:text-2xl">
                {t("weakestCategoriesTitle", { count: masteryCategories.length })}
              </h2>
              {totalCategories > masteryCategories.length && (
                <Link
                  href={`/${locale}/stats`}
                  className="inline-flex items-center gap-1 text-xs font-semibold text-accent hover:underline sm:text-base"
                >
                  {t("viewAllCategories", { count: totalCategories })}
                  <ChevronRight aria-hidden="true" className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
                </Link>
              )}
            </div>
            <p className="text-[11px] text-muted-foreground sm:text-sm">{t("masteryCoverageHint")}</p>
            <Card className="overflow-hidden p-3 sm:p-6">
              <div className="grid min-w-0 gap-3 sm:grid-cols-2 sm:gap-4">
                {masteryCategories.map((cat) => (
                  <MasteryBar
                    key={cat.code}
                    categoryName={cat.name}
                    masteryPercent={cat.mastery_pct}
                    studied={cat.studied}
                    total={cat.total}
                  />
                ))}
              </div>
            </Card>
          </section>
        )
      )}
    </main>
  );
}

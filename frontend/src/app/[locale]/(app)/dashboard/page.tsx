"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useUserStats } from "@/hooks/use-user-stats";
import { useSessionHistory, type SessionSummary } from "@/hooks/use-session-history";
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
} from "lucide-react";

const resumableModes: SessionSummary["mode"][] = ["variant", "exam", "practice", "mistakes", "grand_mock"];
const resumeModeKeys: Record<SessionSummary["mode"], string> = {
  variant: "resumeModeVariant",
  exam: "resumeModeExam",
  practice: "resumeModePractice",
  mistakes: "resumeModeMistakes",
  grand_mock: "resumeModeGrandMock",
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
  const savedT = useTranslations("Saved");
  const locale = useLocale();
  const { user, entitlement, streak, stats, loading, error } = useUserStats();
  const { sessions, loading: historyLoading, error: historyError } = useSessionHistory(20);

  // Both hooks feed the hero/next-action UI below; treat them as loaded
  // together so those sections settle once instead of updating piecemeal.
  const isPersonalizationLoading = loading || historyLoading;

  const userName = user?.name || t("guestUser");
  const isVip = entitlement?.is_vip ?? false;
  const readinessPct = stats?.readiness_pct ?? 0;
  const currentStreak = streak?.current_streak ?? 0;
  const todayAnswered = streak?.today_answered ?? 0;
  const dailyTarget = streak?.daily_target ?? 20;
  const dueQuestionsCount = stats?.due_questions_count ?? 0;
  const totalCategories = stats?.category_mastery?.length ?? 0;
  const masteryCategories = stats?.category_mastery ? weakestCategories(stats.category_mastery).slice(0, 4) : [];
  const weakest = stats?.category_mastery ? weakestCategory(stats.category_mastery) : null;
  const resumeSession = newestInProgressSession(sessions);
  const resumeStartedAt = resumeSession
    ? formatDateWithTime(resumeSession.started_at)
    : null;
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
          href: `/${locale}/mistakes`,
          tone: "danger" as const,
        }
      : readinessPct >= 80
        ? {
            icon: CheckCircle2,
            title: t("nextActionExamTitle"),
            description: t("nextActionExamDesc"),
            cta: t("nextActionExamCta"),
            href: `/${locale}/session/start?mode=exam`,
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
    <main className="mx-auto max-w-6xl px-4 py-8 space-y-8">
      <section className="grid gap-4 lg:grid-cols-[1.35fr_0.9fr]">
        <section className="relative overflow-hidden rounded-3xl border border-border bg-card p-6 md:p-8">
          <div className="relative z-10 flex h-full flex-col justify-between gap-6">
            <div className="space-y-4">
              <div className="flex flex-wrap items-center gap-2">
                <span className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background px-3 py-1 text-[11px] font-bold text-muted-foreground">
                  <BrainCircuit aria-hidden="true" className="h-3.5 w-3.5 text-accent" />
                  {t("todayTitle")}
                </span>
                {loading ? (
                  <span
                    aria-hidden="true"
                    className="inline-flex h-[26px] w-24 animate-pulse items-center rounded-md border border-border bg-background px-3 py-1"
                  />
                ) : isVip ? (
                  <span className="inline-flex items-center gap-1 rounded-md border border-gold/40 bg-gold/15 px-3 py-1 text-xs font-bold text-gold">
                    <Crown aria-hidden="true" className="h-3.5 w-3.5" /> {t("vipBadge")}
                  </span>
                ) : (
                  <Link href={`/${locale}/premium`}>
                    <span className="inline-flex items-center gap-1 rounded-md border border-border bg-accent/15 px-3 py-1 text-xs font-bold text-foreground hover:border-accent">
                      <Sparkles aria-hidden="true" className="h-3.5 w-3.5 text-accent" /> {t("upgradeVip")}
                    </span>
                  </Link>
                )}
              </div>

              <div className="flex items-start gap-3">
                <Hand aria-hidden="true" className="mt-1 h-6 w-6 shrink-0 text-accent" />
                <div>
                  <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
                    {loading ? (
                      <span
                        aria-hidden="true"
                        className="inline-block h-8 w-56 max-w-full animate-pulse rounded-lg bg-border/60 align-middle"
                      />
                    ) : (
                      t("welcomeUser", { name: userName })
                    )}
                  </h1>
                  <p className="mt-2 max-w-2xl text-sm text-muted-foreground">{t("welcomeSubtitle")}</p>
                </div>
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-border bg-background p-4">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-2 text-xs font-bold text-muted-foreground">
                      <Flame aria-hidden="true" className="h-4 w-4 text-streak" />
                      {t("streakCount", { count: currentStreak })}
                    </div>
                    <p className="mt-2 font-display text-2xl font-extrabold">{todayAnswered}</p>
                    <p className="text-xs text-muted-foreground">{t("streakToday", { done: todayAnswered, goal: dailyTarget })}</p>
                  </>
                )}
              </div>

              <div className="rounded-2xl border border-border bg-background p-4">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-2 text-xs font-bold text-muted-foreground">
                      <CheckCircle2 aria-hidden="true" className="h-4 w-4 text-success" />
                      {t("readinessLabel")}
                    </div>
                    <p className="mt-2 font-display text-2xl font-extrabold">{readinessPct}%</p>
                    <p className="text-xs text-muted-foreground">{readinessPct >= 80 ? t("readyBadge") : t("notReadyBadge")}</p>
                  </>
                )}
              </div>

              <div className="rounded-2xl border border-border bg-background p-4">
                {loading ? (
                  <StatCardSkeleton />
                ) : (
                  <>
                    <div className="flex items-center gap-2 text-xs font-bold text-muted-foreground">
                      <AlertTriangle aria-hidden="true" className="h-4 w-4 text-danger" />
                      {t("dueQuestionsLabel")}
                    </div>
                    <p className="mt-2 font-display text-2xl font-extrabold">{dueQuestionsCount}</p>
                    <p className="text-xs text-muted-foreground">
                      {weakest ? t("weakestCategory", { category: weakest.name, percent: weakest.mastery_pct }) : t("weakestCategoryEmpty")}
                    </p>
                  </>
                )}
              </div>
            </div>
          </div>
        </section>

        <aside className="rounded-3xl border border-border bg-card p-6 md:p-7">
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
                <div className="h-12 w-12 rounded-2xl bg-border/60" />
              </div>
              <div className="mt-4 space-y-2">
                <div className="h-3.5 w-full rounded bg-border/50" />
                <div className="h-3.5 w-3/4 rounded bg-border/50" />
              </div>
              <div className="mt-6 h-12 w-full rounded-2xl bg-border/60" />
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">{t("nextActionLabel")}</p>
                  <h2 className="mt-2 font-display text-xl font-extrabold tracking-tight">{nextAction.title}</h2>
                </div>
                <div className={`flex h-12 w-12 items-center justify-center rounded-2xl ${nextActionToneClass}`}>
                  <NextActionIcon aria-hidden="true" className="h-6 w-6" />
                </div>
              </div>

              <p className="mt-4 text-sm leading-6 text-muted-foreground">{nextAction.description}</p>

              <Link
                href={nextAction.href}
                className="mt-6 inline-flex min-h-12 w-full items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-6 text-sm font-extrabold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:border-b-0 active:shadow-none"
              >
                {nextAction.cta}
                <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
              </Link>
            </>
          )}

          <div className="mt-6 rounded-2xl border border-border bg-background p-4">
            <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{t("studyLoopTitle")}</p>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className="rounded-md bg-accent/15 px-3 py-1 text-xs font-semibold text-foreground">{t("studyLoopRecall")}</span>
              <span className="rounded-md bg-gold/15 px-3 py-1 text-xs font-semibold text-gold">{t("studyLoopSpacing")}</span>
              <span className="rounded-md bg-success/15 px-3 py-1 text-xs font-semibold text-success">{t("studyLoopFeedback")}</span>
            </div>
          </div>
        </aside>
      </section>

      {error && (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          {t("loadError")}
        </div>
      )}

      {historyLoading ? (
        <div
          role="status"
          aria-live="polite"
          className="rounded-2xl border border-accent/20 bg-accent/5 px-5 py-4 text-sm font-medium text-muted-foreground"
        >
          {t("resumeLoading")}
        </div>
      ) : historyError ? (
        <div
          role="alert"
          className="rounded-2xl border border-destructive/40 bg-destructive/5 px-5 py-4 text-sm font-medium text-destructive"
        >
          {t("resumeLoadError")}
        </div>
      ) : resumeSession && resumeStartedAt ? (
        <section
          aria-labelledby="resume-session-title"
          className="relative overflow-hidden rounded-3xl border border-accent/40 bg-card p-6 md:p-8"
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
                <p className="max-w-xl text-sm text-muted-foreground">{t("resumeDescription")}</p>
                <div className="flex flex-wrap items-center gap-2 text-xs font-bold">
                  <span className="rounded-full border border-accent/30 bg-accent/10 px-3 py-1 text-accent">
                    {t(resumeModeKeys[resumeSession.mode])}
                  </span>
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-background/80 px-3 py-1 text-muted-foreground">
                    <Clock3 aria-hidden="true" className="h-3.5 w-3.5" />
                    {t("resumeStartedAt", { date: resumeStartedAt })}
                  </span>
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-background/80 px-3 py-1 text-muted-foreground">
                    <ListChecks aria-hidden="true" className="h-3.5 w-3.5" />
                    {t("resumeTotal", { total: resumeSession.total })}
                  </span>
                </div>
              </div>
            </div>
            <Link
              href={`/${locale}/session/${encodeURIComponent(resumeSession.id)}`}
              className="inline-flex min-h-12 shrink-0 items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-7 text-sm font-extrabold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:border-b-0 active:shadow-none"
            >
              {t("resumeCta")}
              <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
            </Link>
          </div>
        </section>
      ) : null}

      {/* 4 Main Mode Cards */}
      <section className="space-y-4">
        <h2 className="font-display text-xl font-bold tracking-tight">{t("modesTitle")}</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {/* 1. Biletlar Grid */}
          <Link href={`/${locale}/tickets`}>
            <Card className="glass-card h-full p-5 flex flex-col justify-between group">
              <div>
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-accent/15 text-accent group-hover:scale-110 transition-transform">
                  <BookOpen aria-hidden="true" className="h-6 w-6" />
                </div>
                <CardTitle className="text-base">{t("navVariantsTitle")}</CardTitle>
                <CardDescription className="mt-1">{t("navVariantsDesc")}</CardDescription>
              </div>
              <div className="mt-4 flex items-center justify-between text-xs font-bold text-accent">
                <span>{t("variantsMeta")}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 transition-transform group-hover:translate-x-1" />
              </div>
            </Card>
          </Link>

          {/* 2. Imtihon Simulation */}
          <Link href={`/${locale}/session/start?mode=exam`}>
            <Card className="glass-card h-full p-5 flex flex-col justify-between group border-amber-500/30">
              <div>
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-500/15 text-amber-500 group-hover:scale-110 transition-transform">
                  <Award aria-hidden="true" className="h-6 w-6" />
                </div>
                <CardTitle className="text-base">{t("navExamTitle")}</CardTitle>
                <CardDescription className="mt-1">{t("navExamDesc")}</CardDescription>
              </div>
              <div className="mt-4 flex items-center justify-between text-xs font-bold text-amber-500">
                <span>{t("examMeta")}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 transition-transform group-hover:translate-x-1" />
              </div>
            </Card>
          </Link>

          {/* 3. Mashq Rejimi */}
          <Link href={`/${locale}/practice`}>
            <Card className="glass-card h-full p-5 flex flex-col justify-between group">
              <div>
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/15 text-emerald-500 group-hover:scale-110 transition-transform">
                  <Target aria-hidden="true" className="h-6 w-6" />
                </div>
                <CardTitle className="text-base">{t("navPracticeTitle")}</CardTitle>
                <CardDescription className="mt-1">{t("navPracticeDesc")}</CardDescription>
              </div>
              <div className="mt-4 flex items-center justify-between text-xs font-bold text-emerald-500">
                <span>{t("practiceMeta")}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 transition-transform group-hover:translate-x-1" />
              </div>
            </Card>
          </Link>

          {/* 4. Xatolar Banki */}
          <Link href={`/${locale}/mistakes`}>
            <Card className="glass-card h-full p-5 flex flex-col justify-between group">
              <div>
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-rose-500/15 text-rose-500 group-hover:scale-110 transition-transform">
                  <AlertTriangle aria-hidden="true" className="h-6 w-6" />
                </div>
                <CardTitle className="text-base">{t("navMistakesTitle")}</CardTitle>
                <CardDescription className="mt-1">{t("navMistakesDesc")}</CardDescription>
              </div>
              <div className="mt-4 flex items-center justify-between text-xs font-bold text-rose-500">
                <span>{t("mistakesMeta", { count: stats?.due_questions_count ?? 0 })}</span>
                <ChevronRight aria-hidden="true" className="h-4 w-4 transition-transform group-hover:translate-x-1" />
              </div>
            </Card>
          </Link>
        </div>
      </section>

      {/* Grand Mock — gated full exam simulation */}
      <section>
        <GrandMockCard />
      </section>

      {/* Road Signs Banner */}
      <section>
        <Link href={`/${locale}/signs`}>
          <Card className="glass-card border-border p-6 hover:border-accent">
            <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
              <div className="flex items-center gap-4">
                <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-accent text-accent-foreground shadow-3d">
                  <Signpost aria-hidden="true" className="h-7 w-7" />
                </div>
                <div>
                  <h3 className="font-display text-lg font-bold">{t("signsTitle")}</h3>
                  <p className="text-xs text-muted-foreground">
                    {t("signsDescription")}
                  </p>
                </div>
              </div>
              <Button variant="game" size="sm" className="shrink-0">
                {t("signsOpen")} <ChevronRight aria-hidden="true" className="ml-1 h-4 w-4" />
              </Button>
            </div>
          </Card>
        </Link>
      </section>

      {/* Saved Questions Entry */}
      <section>
        <Link href={`/${locale}/saved`}>
          <Card className="glass-card p-6 border-gold/40 bg-gradient-to-r from-gold/10 via-card to-card hover:border-gold">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div className="flex items-center gap-4">
                <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gold/20 text-gold">
                  <Bookmark aria-hidden="true" className="h-7 w-7" />
                </div>
                <div>
                  <h3 className="font-display text-lg font-bold">{savedT("title")}</h3>
                  <p className="text-xs text-muted-foreground">{savedT("subtitle")}</p>
                </div>
              </div>
              <Button variant="outline" size="sm" className="shrink-0">
                {savedT("open")} <ChevronRight aria-hidden="true" className="ml-1 h-4 w-4" />
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
        <section className="space-y-4" aria-hidden="true">
          <div className="h-6 w-64 max-w-full animate-pulse rounded bg-border/60" />
          <Card className="p-6">
            <div className="grid animate-pulse gap-4 sm:grid-cols-2">
              {[0, 1, 2, 3].map((i) => (
                <div key={i} className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <div className="h-3.5 w-28 rounded bg-border/60" />
                    <div className="h-3.5 w-8 rounded bg-border/60" />
                  </div>
                  <div className="h-2 w-full rounded-full bg-border/50" />
                </div>
              ))}
            </div>
          </Card>
        </section>
      ) : (
        masteryCategories.length > 0 && (
          <section className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 className="font-display text-xl font-bold tracking-tight">
                {t("weakestCategoriesTitle", { count: masteryCategories.length })}
              </h2>
              {totalCategories > masteryCategories.length && (
                <Link
                  href={`/${locale}/stats`}
                  className="inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline"
                >
                  {t("viewAllCategories", { count: totalCategories })}
                  <ChevronRight aria-hidden="true" className="h-4 w-4" />
                </Link>
              )}
            </div>
            <Card className="p-6">
              <div className="grid gap-4 sm:grid-cols-2">
                {masteryCategories.map((cat) => (
                  <MasteryBar key={cat.code} categoryName={cat.name} masteryPercent={cat.mastery_pct} />
                ))}
              </div>
            </Card>
          </section>
        )
      )}
    </main>
  );
}

"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useUserStats } from "@/hooks/use-user-stats";
import { useSessionHistory, type SessionSummary } from "@/hooks/use-session-history";
import { ResultRing } from "@/components/shared/result-ring";
import { MasteryBar } from "@/components/shared/mastery-bar";
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
} from "lucide-react";

const resumableModes: SessionSummary["mode"][] = ["variant", "exam", "practice", "mistakes"];
const resumeModeKeys: Record<SessionSummary["mode"], string> = {
  variant: "resumeModeVariant",
  exam: "resumeModeExam",
  practice: "resumeModePractice",
  mistakes: "resumeModeMistakes",
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

export default function DashboardPage() {
  const t = useTranslations("Dashboard");
  const savedT = useTranslations("Saved");
  const locale = useLocale();
  const { user, entitlement, streak, stats, error } = useUserStats();
  const { sessions, loading: historyLoading, error: historyError } = useSessionHistory(20);

  const userName = user?.name || t("guestUser");
  const isVip = entitlement?.is_vip ?? false;
  const readinessPct = stats?.readiness_pct ?? 0;
  const currentStreak = streak?.current_streak ?? 0;
  const todayAnswered = streak?.today_answered ?? 0;
  const dailyTarget = streak?.daily_target ?? 20;
  const resumeSession = newestInProgressSession(sessions);
  const resumeStartedAt = resumeSession
    ? new Intl.DateTimeFormat(locale, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(resumeSession.started_at))
    : null;

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 space-y-8">
      {/* Top Welcome Hero Banner */}
      <section className="relative overflow-hidden rounded-3xl border border-accent/30 bg-gradient-to-br from-card via-card to-accent/10 p-6 md:p-8 shadow-xl">
        <div className="relative z-10 flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <Hand aria-hidden="true" className="h-7 w-7 text-accent" />
              <h1 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">
                {t("welcomeUser", { name: userName })}
              </h1>
              {isVip ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-gold/20 px-3 py-1 text-xs font-bold text-gold border border-gold/40">
                  <Crown aria-hidden="true" className="h-3.5 w-3.5" /> {t("vipBadge")}
                </span>
              ) : (
                <Link href={`/${locale}/premium`}>
                  <span className="inline-flex items-center gap-1 rounded-full bg-accent/20 px-3 py-1 text-xs font-bold text-accent border border-accent/40 hover:scale-105 transition-transform">
                    <Sparkles aria-hidden="true" className="h-3.5 w-3.5" /> {t("upgradeVip")}
                  </span>
                </Link>
              )}
            </div>
            <p className="text-sm text-muted-foreground max-w-xl">{t("welcomeSubtitle")}</p>

            {/* Daily Target Stepper */}
            <div className="pt-3 flex items-center gap-3">
              <div className="flex items-center gap-1.5 text-xs font-bold text-streak">
                <Flame aria-hidden="true" className="h-4 w-4 animate-flame" />
                <span>{t("streakCount", { count: currentStreak })}</span>
              </div>
              <div className="h-2 w-36 rounded-full bg-background overflow-hidden border border-border">
                <div
                  className="h-full bg-gradient-to-r from-amber-500 to-streak transition-all duration-500"
                  style={{ width: `${Math.min(100, Math.round((todayAnswered / dailyTarget) * 100))}%` }}
                />
              </div>
              <span className="text-xs text-muted-foreground font-semibold">
                {t("streakToday", { done: todayAnswered, goal: dailyTarget })}
              </span>
            </div>
          </div>

          {/* Readiness Gauge Ring */}
          <div className="flex flex-col items-center justify-center rounded-2xl border border-border/80 bg-background/80 p-4 shadow-sm backdrop-blur-md">
            <ResultRing percent={readinessPct} label={t("readinessLabel")} />
            <span
              className={`mt-2 rounded-full px-3 py-0.5 text-[11px] font-extrabold ${
                readinessPct >= 80 ? "bg-success/20 text-success" : "bg-accent/20 text-accent"
              }`}
            >
              {readinessPct >= 80 && <CheckCircle2 aria-hidden="true" className="mr-1 inline h-3.5 w-3.5" />}
              {readinessPct >= 80 ? t("readyBadge") : t("notReadyBadge")}
            </span>
          </div>
        </div>
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
          className="relative overflow-hidden rounded-3xl border-2 border-accent/50 bg-gradient-to-r from-accent/20 via-card to-indigo-500/10 p-6 shadow-2xl md:p-8"
        >
          <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-accent/15 blur-3xl" />
          <div className="relative flex flex-col items-start justify-between gap-6 md:flex-row md:items-center">
            <div className="flex items-start gap-4">
              <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-accent text-white shadow-3d">
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

      {/* Road Signs Banner */}
      <section>
        <Link href={`/${locale}/signs`}>
          <Card className="glass-card p-6 border-accent/40 bg-gradient-to-r from-accent/10 via-card to-card hover:border-accent">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <div className="flex items-center gap-4">
                <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-accent text-white shadow-3d">
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

      {/* Category Mastery Progress */}
      {stats?.category_mastery && stats.category_mastery.length > 0 && (
        <section className="space-y-4">
          <h2 className="font-display text-xl font-bold tracking-tight">{t("categoryMasteryTitle")}</h2>
          <Card className="p-6">
            <div className="grid gap-4 sm:grid-cols-2">
              {stats.category_mastery.map((cat) => (
                <MasteryBar key={cat.code} categoryName={cat.name} masteryPercent={cat.mastery_pct} />
              ))}
            </div>
          </Card>
        </section>
      )}
    </main>
  );
}

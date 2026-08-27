"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useUserStats } from "@/hooks/use-user-stats";
import { useSessionHistory, type SessionSummary } from "@/hooks/use-session-history";
import { formatDateShort } from "@/lib/date-format";
import { ResultRing } from "@/components/shared/result-ring";
import { MasteryBar } from "@/components/shared/mastery-bar";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Flame, RefreshCw } from "lucide-react";

export interface StatsPageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/stats/page.tsx):
  // this page has no premium, checkout or profile surface of its own — the
  // only navigation is the back link and the due-repeat CTA, both of which
  // must stay inside /station instead of the learner app's gated /dashboard
  // and /session/start routes.
  kiosk?: boolean;
}

export default function StatsPage({ kiosk = false }: StatsPageProps = {}) {
  const t = useTranslations("Stats");
  const locale = useLocale();
  const backHref = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;
  // /station/session/start (not /session/start): keeps the proxy.ts kiosk
  // exemption scoped to the /station/... namespace — see practice/page.tsx
  // for the same pattern.
  const sessionStartBase = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;
  const { streak, stats, loading, error, refetch } = useUserStats();
  const {
    sessions,
    loading: historyLoading,
    error: historyError,
    refetch: refetchHistory,
  } = useSessionHistory(20);

  const readinessPct = stats?.readiness_pct ?? 0;
  const isReady = readinessPct >= 80;
  const dueCount = stats?.due_questions_count ?? 0;
  const passEstimate = stats?.pass_estimate;
  const passPct = passEstimate?.estimated_pass_pct ?? null;

  const modeLabels: Record<SessionSummary["mode"], string> = {
    variant: t("modeVariant"),
    exam: t("modeExam"),
    practice: t("modePractice"),
    mistakes: t("modeMistakes"),
    grand_mock: t("modeGrandMock"),
    review: t("modeReview"),
    placement: t("modePlacement"),
  };
  const statusLabels: Record<SessionSummary["status"], string> = {
    in_progress: t("statusInProgress"),
    passed: t("statusPassed"),
    failed: t("statusFailed"),
    abandoned: t("statusAbandoned"),
  };

  return (
    <main className="page-shell-narrow">
      <header className="mb-6">
        <Link href={backHref} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-1 text-xs text-muted-foreground/90">{t("disclaimer")}</p>
      </header>

      {loading ? (
        <p role="status" className="py-12 text-center text-sm text-muted-foreground">
          {t("loading")}
        </p>
      ) : error ? (
        <div role="alert" className="rounded-xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          <p>{t("loadError")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void refetch()}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
          </Button>
        </div>
      ) : (
      <div className="space-y-6">
        {/* Top Summary: Readiness, pass estimate & Streak */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Card className="flex flex-col items-center justify-center p-6 text-center">
            <ResultRing percent={readinessPct} label={t("readiness")} />
            <span
              className={`mt-4 rounded-full px-3 py-1 text-xs font-bold ${
                isReady ? "bg-success/20 text-success" : "bg-accent/20 text-accent"
              }`}
            >
              {isReady ? t("readyBadge") : t("notReadyBadge")}
            </span>
          </Card>

          <Card className="flex flex-col justify-center p-6">
            <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("passEstimateTitle")}
            </p>
            <p className="mt-2 font-display text-4xl font-extrabold text-accent">
              {passPct === null ? "—" : t("passEstimateValue", { pct: passPct })}
            </p>
          </Card>

          <Card className="flex flex-col justify-between p-6 sm:col-span-2 lg:col-span-1">
            <div>
              <div className="flex items-center gap-2">
                <Flame aria-hidden="true" className="h-8 w-8 text-streak" />
                <span className="font-display text-4xl font-extrabold">{streak?.current_streak ?? 0}</span>
                <span className="text-sm text-muted-foreground">{t("streakLabel")}</span>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("streakRecord", { max: streak?.max_streak ?? streak?.current_streak ?? 0 })}
              </p>
            </div>

            <div className="mt-6 rounded-md border border-border bg-background p-4">
              <p className="text-xs text-muted-foreground">{t("dueLabel")}</p>
              <p className="font-display text-xl font-bold text-accent">{t("questionCount", { count: dueCount })}</p>
            </div>
          </Card>
        </div>

        {/* Category Mastery List */}
        {stats?.category_mastery && stats.category_mastery.length > 0 && (
          <Card className="p-6">
            <CardHeader className="p-0 mb-4">
              <CardTitle className="text-lg font-bold">{t("categoryMastery")}</CardTitle>
              <p className="mt-1 text-xs text-muted-foreground">{t("categoryMasteryHint")}</p>
            </CardHeader>
            <CardContent className="p-0">
              <div className="grid min-w-0 gap-3 overflow-hidden sm:grid-cols-2 sm:gap-4">
                {stats.category_mastery.map((cat) => (
                  <MasteryBar
                    key={cat.code}
                    categoryName={cat.name}
                    masteryPercent={cat.mastery_pct}
                    studied={cat.studied}
                    total={cat.total}
                  />
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        <Card className="p-6">
          <CardHeader className="p-0 mb-4">
            <CardTitle className="text-lg font-bold">{t("history")}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {historyLoading ? (
              <p role="status" className="py-6 text-center text-sm text-muted-foreground">{t("historyLoading")}</p>
            ) : historyError ? (
              <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                <p>{t("historyLoadError")}</p>
                <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void refetchHistory()}>
                  <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
                </Button>
              </div>
            ) : sessions.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">{t("historyEmpty")}</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <caption className="sr-only">{t("historyTableCaption")}</caption>
                  <thead className="border-b border-border text-muted-foreground">
                    <tr>
                      <th scope="col" className="pb-3 font-semibold">{t("dateColumn")}</th>
                      <th scope="col" className="pb-3 font-semibold">{t("modeColumn")}</th>
                      <th scope="col" className="pb-3 font-semibold">{t("statusColumn")}</th>
                      <th scope="col" className="pb-3 text-right font-semibold">{t("resultColumn")}</th>
                      <th scope="col" className="pb-3 text-right font-semibold">{t("durationColumn")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sessions.map((item) => {
                      const durationMinutes = item.finished_at
                        ? Math.max(
                            0,
                            Math.round(
                              (new Date(item.finished_at).getTime() - new Date(item.started_at).getTime()) /
                                60_000
                            )
                          )
                        : null;
                      return (
                        <tr key={item.id} className="border-b border-border/60 last:border-0">
                          <td className="py-3">
                            {formatDateShort(item.started_at)}
                          </td>
                          <td className="py-3 font-medium">{modeLabels[item.mode]}</td>
                          <td className="py-3">{statusLabels[item.status]}</td>
                          <td className="py-3 text-right font-bold">
                            {item.score === undefined ? "—" : `${item.score}/${item.total}`}
                          </td>
                          <td className="py-3 text-right">
                            {durationMinutes === null ? "—" : t("minutes", { count: durationMinutes })}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        {dueCount > 0 && (
          <div className="mt-6">
            <Link
              href={`${sessionStartBase}?mode=review&count=${Math.min(dueCount, 20)}`}
              className="inline-flex h-12 min-h-12 w-full items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-7 text-base font-bold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {t("startRepeat")}
            </Link>
          </div>
        )}
      </div>
      )}
    </main>
  );
}

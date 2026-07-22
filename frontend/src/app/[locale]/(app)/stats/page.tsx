"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useUserStats } from "@/hooks/use-user-stats";
import { useSessionHistory } from "@/hooks/use-session-history";
import { ResultRing } from "@/components/shared/result-ring";
import { MasteryBar } from "@/components/shared/mastery-bar";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Flame, Award, RefreshCw } from "lucide-react";

export default function StatsPage() {
  const t = useTranslations("Stats");
  const locale = useLocale();
  const { streak, stats, loading, error } = useUserStats();
  const { sessions, loading: historyLoading, error: historyError } = useSessionHistory(20);

  const readinessPct = stats?.readiness_pct ?? 0;
  const isReady = readinessPct >= 80;
  const dueCount = stats?.due_questions_count ?? 0;

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
        </Link>
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      {error && (
        <div className="mb-6 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-6">
        {/* Top Summary: Readiness & Streak */}
        <div className="grid gap-4 sm:grid-cols-2">
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

          <Card className="flex flex-col justify-between p-6">
            <div>
              <div className="flex items-center gap-2">
                <Flame className="h-8 w-8 text-streak" />
                <span className="font-display text-4xl font-extrabold">{streak?.current_streak ?? 0}</span>
                <span className="text-sm text-muted-foreground">kunlik streak</span>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("streakRecord", { max: streak?.max_streak ?? streak?.current_streak ?? 0 })}
              </p>
            </div>

            <div className="mt-6 rounded-md border border-border bg-background p-4 flex items-center justify-between">
              <div>
                <p className="text-xs text-muted-foreground">Takrorlash kerak bo'lgan savollar</p>
                <p className="font-display text-xl font-bold text-accent">{dueCount} ta savol</p>
              </div>
              <Link href={`/${locale}/mistakes`}>
                <Button variant="game" size="sm">
                  {t("startRepeat")}
                </Button>
              </Link>
            </div>
          </Card>
        </div>

        {/* Category Mastery List */}
        {stats?.category_mastery && stats.category_mastery.length > 0 && (
          <Card className="p-6">
            <CardHeader className="p-0 mb-4">
              <CardTitle className="text-lg font-bold">{t("categoryMastery")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <div className="grid gap-4 sm:grid-cols-2">
                {stats.category_mastery.map((cat) => (
                  <MasteryBar key={cat.code} categoryName={cat.name} masteryPercent={cat.mastery_pct} />
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
              <p className="py-6 text-center text-sm text-muted-foreground">Sessiyalar yuklanmoqda...</p>
            ) : historyError ? (
              <p className="rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                {historyError}
              </p>
            ) : sessions.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">Hali yakunlangan sessiyalar yo'q.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-xs">
                  <thead className="border-b border-border text-muted-foreground">
                    <tr>
                      <th className="pb-3 font-semibold">Sana</th>
                      <th className="pb-3 font-semibold">Rejim</th>
                      <th className="pb-3 font-semibold">Holat</th>
                      <th className="pb-3 text-right font-semibold">Natija</th>
                      <th className="pb-3 text-right font-semibold">Vaqt</th>
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
                            {new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(
                              new Date(item.started_at)
                            )}
                          </td>
                          <td className="py-3 font-medium">{item.mode}</td>
                          <td className="py-3">{item.status}</td>
                          <td className="py-3 text-right font-bold">
                            {item.score === undefined ? "—" : `${item.score}/${item.total}`}
                          </td>
                          <td className="py-3 text-right">
                            {durationMinutes === null ? "—" : `${durationMinutes} daq`}
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
      </div>
    </main>
  );
}

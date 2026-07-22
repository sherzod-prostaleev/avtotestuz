import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { ResultRing } from "@/components/shared/result-ring";
import { mockProfile } from "@/lib/mock-data";
import { Flame } from "lucide-react";

export default function DashboardPage() {
  const t = useTranslations("Dashboard");
  const { name, isVip, streak, readinessPercent } = mockProfile;

  const navCards = [
    { title: t("navVariantsTitle"), desc: t("navVariantsDesc") },
    { title: t("navExamTitle"), desc: t("navExamDesc") },
    { title: t("navPracticeTitle"), desc: t("navPracticeDesc") },
    { title: t("navMistakesTitle"), desc: t("navMistakesDesc") },
  ];

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-muted-foreground">{t("welcome")}</p>
          <h1 className="font-display text-2xl font-bold">{name}</h1>
        </div>
        <span
          className={
            isVip
              ? "rounded-full bg-gold px-4 py-1 text-sm font-bold text-background"
              : "rounded-full border border-border px-4 py-1 text-sm text-muted-foreground"
          }
        >
          {isVip ? t("planPremium") : t("planFree")}
        </span>
      </header>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-2">
            <Flame className="h-6 w-6 text-streak" />
            <span className="font-display text-2xl font-extrabold">{streak.current}</span>
            <span className="text-muted-foreground">{t("streakSuffix")}</span>
          </div>
          <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-streak"
              style={{ width: `${Math.min(100, (streak.todayDone / streak.dailyGoal) * 100)}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("streakToday", { done: streak.todayDone, goal: streak.dailyGoal })}
          </p>
        </div>

        <div className="flex items-center justify-center rounded-lg border border-border bg-card p-5">
          <ResultRing percent={readinessPercent} label={t("readinessLabel")} />
        </div>
      </section>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        {navCards.map((c) => (
          <Button
            key={c.title}
            variant="game"
            className="h-auto flex-col items-start gap-1 whitespace-normal px-5 py-4 text-left"
          >
            <span className="font-display text-base font-bold">{c.title}</span>
            <span className="text-xs font-normal opacity-80">{c.desc}</span>
          </Button>
        ))}
      </section>
    </main>
  );
}

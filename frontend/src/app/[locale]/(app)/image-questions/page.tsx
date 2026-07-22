"use client";

import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, AlignLeft, CheckCircle2, Image as ImageIcon, Play } from "lucide-react";

const QUESTION_COUNT_OPTIONS = [10, 20, 30] as const;

type QuestionKind = "with-image" | "without-image";

export default function ImageQuestionsPage() {
  const t = useTranslations("ImageQuestions");
  const locale = useLocale();
  const router = useRouter();

  const [kind, setKind] = useState<QuestionKind>("with-image");
  const [questionCount, setQuestionCount] = useState<number>(20);

  const handleStart = () => {
    const hasImage = kind === "with-image";
    router.push(
      `/${locale}/session/start?mode=practice&has_image=${hasImage}&count=${questionCount}`
    );
  };

  const modes = [
    {
      value: "with-image" as const,
      icon: ImageIcon,
      label: t("withImage"),
      description: t("withImageDescription"),
      accent: "bg-accent/15 text-accent",
    },
    {
      value: "without-image" as const,
      icon: AlignLeft,
      label: t("withoutImage"),
      description: t("withoutImageDescription"),
      accent: "bg-emerald-500/15 text-emerald-500",
    },
  ];

  return (
    <main className="mx-auto max-w-4xl space-y-8 px-4 py-8">
      <div>
        <Link
          href={`/${locale}/dashboard`}
          className="mb-2 inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline"
        >
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-3xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 max-w-xl text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <section aria-label={t("modeGroupLabel")} className="grid gap-4 sm:grid-cols-2">
        {modes.map((item) => {
          const Icon = item.icon;
          const isSelected = kind === item.value;
          return (
            <button
              key={item.value}
              type="button"
              onClick={() => setKind(item.value)}
              aria-pressed={isSelected}
              className={`flex items-start gap-4 rounded-2xl border p-5 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                isSelected
                  ? "border-accent bg-accent/10 shadow-md"
                  : "border-border bg-card hover:border-accent/50 hover:-translate-y-0.5"
              }`}
            >
              <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl ${item.accent}`}>
                <Icon aria-hidden="true" className="h-6 w-6" />
              </div>
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex items-center gap-2">
                  <h2 className="font-display text-base font-bold">{item.label}</h2>
                  {isSelected && (
                    <CheckCircle2 aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                  )}
                </div>
                <p className="text-xs leading-relaxed text-muted-foreground">{item.description}</p>
              </div>
            </button>
          );
        })}
      </section>

      <Card className="space-y-6 p-6">
        <div className="space-y-3">
          <label className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("questionCount")}
          </label>
          <div role="group" aria-label={t("questionCount")} className="flex gap-3">
            {QUESTION_COUNT_OPTIONS.map((count) => (
              <Button
                key={count}
                variant={questionCount === count ? "game" : "outline"}
                size="sm"
                onClick={() => setQuestionCount(count)}
                aria-pressed={questionCount === count}
                className="flex-1 py-2 text-xs font-extrabold"
              >
                {t("questionCountOption", { count })}
              </Button>
            ))}
          </div>
        </div>

        <Button variant="game" size="lg" className="w-full py-3 text-base" onClick={handleStart}>
          <Play aria-hidden="true" className="mr-2 h-5 w-5 fill-current" /> {t("start")}
        </Button>
      </Card>
    </main>
  );
}

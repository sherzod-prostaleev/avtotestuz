"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, BookOpen, Signpost, Play, Sparkles, CheckCircle2, ChevronRight } from "lucide-react";

const QUESTION_COUNT_OPTIONS = [10, 20, 30] as const;

interface CategoryItem {
  code: string;
  name: string;
  sort_order: number;
}

export default function PracticePage() {
  const t = useTranslations("Practice");
  const locale = useLocale();
  const router = useRouter();

  const [mode, setMode] = useState<"category" | "sign">("category");
  const [categories, setCategories] = useState<CategoryItem[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<string>("");
  const [questionCount, setQuestionCount] = useState<number>(10);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<boolean>(false);

  const loadCategories = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const data = await apiGet<CategoryItem[]>(`categories?locale=${encodeURIComponent(locale)}`);
      const ordered = [...data].sort((left, right) => left.sort_order - right.sort_order);
      setCategories(ordered);
      setSelectedCategory(ordered[0]?.code ?? "");
    } catch {
      setCategories([]);
      setSelectedCategory("");
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [locale]);

  useEffect(() => {
    void loadCategories();
  }, [loadCategories]);

  const handleStart = () => {
    if (mode === "sign") {
      router.push(`/${locale}/signs`);
      return;
    }
    if (!selectedCategory) return;
    router.push(`/${locale}/session/start?mode=practice&category_id=${selectedCategory}&count=${questionCount}`);
  };

  return (
    <main className="mx-auto max-w-5xl px-4 py-8 space-y-8">
      <div>
        <Link href={`/${locale}/dashboard`} className="mb-2 inline-flex items-center gap-1 text-sm font-semibold text-accent hover:underline">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-3xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground max-w-xl">{t("subtitle")}</p>
      </div>

      <section className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
        <Card className="glass-card overflow-hidden border-accent/20 bg-gradient-to-br from-card via-card to-accent/10 p-6 md:p-8">
          <div className="flex flex-col gap-5">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full bg-accent/15 px-3 py-1 text-[11px] font-extrabold text-accent">
                {t("modeGroupLabel")}
              </span>
              {categories.length > 0 && (
                <span className="rounded-full bg-emerald-500/15 px-3 py-1 text-[11px] font-extrabold text-emerald-600">
                  {t("byCategoryDescription", { count: categories.length })}
                </span>
              )}
              <span className="rounded-full bg-gold/15 px-3 py-1 text-[11px] font-extrabold text-gold">
                {t("bySignDescription")}
              </span>
            </div>

            <div className="space-y-3">
              <h2 className="font-display text-xl font-extrabold tracking-tight md:text-2xl">
                {t("heroModeTitle")}
              </h2>
              <p className="max-w-2xl text-sm text-muted-foreground">
                {mode === "category" ? t("heroCategoryBody") : t("heroSignBody")}
              </p>
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-border/70 bg-background/80 p-4">
                <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                  {t("selectCategory")}
                </p>
                <p className="mt-2 text-sm font-semibold text-foreground">
                  {loading
                    ? t("categoryCountLoading")
                    : categories.length > 0
                      ? t("categoryCountSummary", { count: categories.length })
                      : t("categoryCountUnavailable")}
                </p>
              </div>
              <div className="rounded-2xl border border-border/70 bg-background/80 p-4">
                <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                  {t("questionCount")}
                </p>
                <p className="mt-2 text-sm font-semibold text-foreground">{t("selectedQuestionCount", { count: questionCount })}</p>
              </div>
              <div className="rounded-2xl border border-border/70 bg-background/80 p-4">
                <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                  {t("bySign")}
                </p>
                <p className="mt-2 text-sm font-semibold text-foreground">{t("signCatalogSummary")}</p>
              </div>
            </div>
          </div>
        </Card>

        <Card className="glass-card flex flex-col justify-between border-border/70 bg-background/80 p-6 md:p-8">
          <div className="space-y-4">
            <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
              {t("modeGroupLabel")}
            </p>
            <div className="rounded-2xl border border-border bg-card/80 p-4">
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent/15 text-accent">
                  <BookOpen aria-hidden="true" className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-sm font-semibold">{t("selectCategory")}</p>
                  <p className="text-xs text-muted-foreground">{t("selectCategoryDescription")}</p>
                </div>
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-card/80 p-4">
              <div className="flex items-center gap-3">
                <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-emerald-500/15 text-emerald-500">
                  <Signpost aria-hidden="true" className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-sm font-semibold">{t("bySign")}</p>
                  <p className="text-xs text-muted-foreground">{t("signCatalogHint")}</p>
                </div>
              </div>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap gap-2">
            {QUESTION_COUNT_OPTIONS.map((count) => (
              <span key={count} className="rounded-full bg-accent/10 px-3 py-1 text-xs font-semibold text-accent">
                {t("questionCountOption", { count })}
              </span>
            ))}
          </div>
        </Card>
      </section>

      {/* 2 Primary Practice Modes */}
      <section aria-label={t("modeGroupLabel")} className="grid gap-4 sm:grid-cols-2">
        <button
          type="button"
          onClick={() => setMode("category")}
          aria-pressed={mode === "category"}
          className={`glass-card rounded-2xl border p-6 text-left transition-all duration-200 hover:-translate-y-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            mode === "category"
              ? "border-accent bg-accent/15 ring-2 ring-accent/40 shadow-xl"
              : "hover:border-accent/60"
          }`}
        >
          <div className="flex items-center gap-4">
            <div className={`flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl ${
              mode === "category" ? "bg-accent text-white shadow-3d" : "bg-accent/10 text-accent"
            }`}>
              <BookOpen aria-hidden="true" className="h-7 w-7" />
            </div>
            <div>
              <h3 className="font-display text-lg font-bold">{t("byCategory")}</h3>
              <p className="text-xs text-muted-foreground">
                {categories.length > 0 ? t("byCategoryDescription", { count: categories.length }) : t("selectCategoryDescription")}
              </p>
            </div>
          </div>
        </button>

        <button
          type="button"
          onClick={() => setMode("sign")}
          aria-pressed={mode === "sign"}
          className={`glass-card rounded-2xl border p-6 text-left transition-all duration-200 hover:-translate-y-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
            mode === "sign"
              ? "border-emerald-500 bg-emerald-500/15 ring-2 ring-emerald-500/40 shadow-xl"
              : "hover:border-emerald-500/60"
          }`}
        >
          <div className="flex items-center gap-4">
            <div className={`flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl ${
              mode === "sign" ? "bg-emerald-500 text-white shadow-3d" : "bg-emerald-500/10 text-emerald-500"
            }`}>
              <Signpost aria-hidden="true" className="h-7 w-7" />
            </div>
            <div>
              <h3 className="font-display text-lg font-bold">{t("bySign")}</h3>
              <p className="text-xs text-muted-foreground">{t("bySignDescription")}</p>
            </div>
          </div>
        </button>
      </section>

      {/* Category Selection Panel */}
      {mode === "category" && (
        <Card className="glass-card p-6 md:p-8 space-y-6">
          <div className="flex items-center justify-between border-b border-border pb-4">
            <div>
              <h2 className="font-display text-xl font-bold">{t("selectCategory")}</h2>
              <p className="text-xs text-muted-foreground">{t("selectCategoryDescription")}</p>
            </div>
            <Sparkles aria-hidden="true" className="h-5 w-5 text-gold animate-pulse" />
          </div>

          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground animate-pulse">
              {t("categoriesLoading")}
            </div>
          ) : loadError ? (
            <div role="alert" className="space-y-3 py-8 text-center">
              <p className="text-sm font-semibold text-destructive">{t("categoriesLoadError")}</p>
              <Button variant="outline" size="sm" onClick={() => void loadCategories()}>
                {t("retry")}
              </Button>
            </div>
          ) : categories.length === 0 ? (
            <div className="py-8 text-center text-sm text-muted-foreground">{t("categoriesEmpty")}</div>
          ) : (
            <div role="group" aria-label={t("categoryGroupLabel")} className="grid gap-3 sm:grid-cols-2">
              {categories.map((cat) => {
                const isSelected = selectedCategory === cat.code;

                return (
                  <button
                    type="button"
                    key={cat.code}
                    onClick={() => setSelectedCategory(cat.code)}
                    aria-pressed={isSelected}
                    className={`flex items-center justify-between rounded-2xl border p-4 text-left transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                      isSelected
                        ? "border-accent bg-accent/15 text-accent font-bold ring-2 ring-accent/40 shadow-md translate-x-0.5"
                        : "border-border bg-card/80 text-foreground hover:border-accent/60 hover:bg-card"
                    }`}
                  >
                    <span className="text-xs font-bold leading-relaxed">{cat.name}</span>
                    {isSelected && <CheckCircle2 aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />}
                  </button>
                );
              })}
            </div>
          )}

          {/* Question Count Stepper */}
          <div className="pt-4 space-y-3 border-t border-border">
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

          {/* Start CTA Button */}
          <div className="pt-4">
            <Button
              variant="game"
              size="lg"
              className="w-full text-base py-3"
              onClick={handleStart}
              disabled={loading || loadError || !selectedCategory}
            >
              <Play aria-hidden="true" className="mr-2 h-5 w-5 fill-current" /> {t("start")}
            </Button>
          </div>
        </Card>
      )}

      {/* Sign Mode Container */}
      {mode === "sign" && (
        <Card className="glass-card p-8 text-center space-y-4">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-emerald-500/15 text-emerald-500 shadow-sm">
            <Signpost aria-hidden="true" className="h-8 w-8" />
          </div>
          <h2 className="font-display text-xl font-bold">{t("signPracticeTitle")}</h2>
          <p className="mx-auto max-w-md text-xs leading-relaxed text-muted-foreground">
            {t("signPracticeDescription", { count: 7 })}
          </p>
          <Button variant="success" size="lg" onClick={handleStart} className="px-8">
            {t("openSigns")} <ChevronRight aria-hidden="true" className="ml-1 h-5 w-5" />
          </Button>
        </Card>
      )}
    </main>
  );
}

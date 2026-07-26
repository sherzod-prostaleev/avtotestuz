"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet } from "@/lib/api-client";
import { usePracticeAllowance } from "@/hooks/use-practice-allowance";
import { useSigns } from "@/hooks/use-signs";
import { SignPracticeGrid } from "@/components/practice/sign-practice-grid";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { formatDateWithTime } from "@/lib/date-format";
import {
  AlignLeft,
  ArrowLeft,
  BookOpen,
  CalendarClock,
  CheckCircle2,
  Crown,
  Image as ImageIcon,
  Layers,
  Play,
  RefreshCw,
  Signpost,
  BrainCircuit,
} from "lucide-react";
import { OFFICIAL_QUESTION_COUNT } from "@/lib/content-counts";

const COUNT_PRESETS = [20, 50, 100] as const;
const MAX_CUSTOM_COUNT = OFFICIAL_QUESTION_COUNT;

type Source = "due" | "category" | "variant" | "image" | "sign";

interface CategoryItem {
  code: string;
  name: string;
  sort_order: number;
  question_count: number;
}

interface VariantItem {
  number: number;
}

interface StatsDTO {
  due_count: number;
}

interface MistakesDTO {
  due_count: number;
  total_bank_count: number;
  next_due_at: string | null;
}

export default function PracticePage() {
  const t = useTranslations("Practice");
  const locale = useLocale();
  const router = useRouter();

  const [source, setSource] = useState<Source>("category");
  const [categories, setCategories] = useState<CategoryItem[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<string>("");
  const [variantCount, setVariantCount] = useState<number>(0);
  const [variantFrom, setVariantFrom] = useState<number>(1);
  const [variantTo, setVariantTo] = useState<number>(10);
  const [withImage, setWithImage] = useState<boolean>(true);
  const [count, setCount] = useState<number>(20);
  const [customCount, setCustomCount] = useState<string>("");
  const [allQuestions, setAllQuestions] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<boolean>(false);
  const [dueCount, setDueCount] = useState<number>(0);
  const [bankCount, setBankCount] = useState<number>(0);
  const [nextDueAt, setNextDueAt] = useState<string | null>(null);

  const { allowance } = usePracticeAllowance();
  const { signs } = useSigns(locale);
  const signsAvailable = signs.length > 0;

  const loadContent = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const [cats, variants, stats, mistakes] = await Promise.all([
        apiGet<CategoryItem[]>(`categories?locale=${encodeURIComponent(locale)}`),
        apiGet<VariantItem[]>("variants"),
        apiGet<StatsDTO>("me/stats").catch(() => ({ due_count: 0 })),
        apiGet<MistakesDTO>("me/mistakes").catch(() => ({
          due_count: 0,
          total_bank_count: 0,
          next_due_at: null,
        })),
      ]);
      const ordered = [...cats].sort((left, right) => left.sort_order - right.sort_order);
      setCategories(ordered);
      setSelectedCategory(ordered[0]?.code ?? "");
      const dueFromMistakes = Number.isInteger(mistakes.due_count) ? mistakes.due_count : 0;
      const dueFromStats = Number.isInteger(stats.due_count) ? stats.due_count : 0;
      setDueCount(Math.max(dueFromMistakes, dueFromStats));
      setBankCount(Number.isInteger(mistakes.total_bank_count) ? mistakes.total_bank_count : 0);
      setNextDueAt(typeof mistakes.next_due_at === "string" ? mistakes.next_due_at : null);

      const highest = variants.reduce((max, item) => Math.max(max, item.number), 0);
      setVariantCount(highest);
      if (highest > 0) {
        setVariantFrom(1);
        setVariantTo(Math.min(10, highest));
      }
    } catch {
      setCategories([]);
      setSelectedCategory("");
      setVariantCount(0);
      setDueCount(0);
      setBankCount(0);
      setNextDueAt(null);
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [locale]);

  useEffect(() => {
    void loadContent();
  }, [loadContent]);

  const imageCounts = useMemo(() => {
    // Derived from the category totals rather than a second endpoint; the
    // exact split is shown by the session itself once it starts.
    const total = categories.reduce((sum, item) => sum + item.question_count, 0);
    return total;
  }, [categories]);

  const rangeValid = variantFrom >= 1 && variantTo >= variantFrom && variantTo <= variantCount;

  const effectiveCount = useMemo(() => {
    if (!allowance || allowance.unlimited) return count;
    return Math.min(count, allowance.remaining);
  }, [allowance, count]);

  const exhausted = Boolean(allowance && !allowance.unlimited && allowance.remaining <= 0);
  const clamped = Boolean(allowance && !allowance.unlimited && !exhausted && effectiveCount < count);

  const canStart = (() => {
    if (loading) return false;
    if (source === "due") return dueCount > 0;
    if (exhausted) return false;
    if (source === "category") return Boolean(selectedCategory);
    if (source === "variant") return rangeValid;
    if (source === "sign") return false;
    return true;
  })();

  const handleStart = () => {
    if (source === "sign") return;
    if (source === "due") {
      const reviewCount = Math.min(20, Math.max(dueCount, 1));
      router.push(`/${locale}/session/start?mode=review&count=${reviewCount}`);
      return;
    }
    const base = `/${locale}/session/start?mode=practice&count=${count}`;
    if (source === "category") {
      router.push(`${base}&category_id=${encodeURIComponent(selectedCategory)}`);
      return;
    }
    if (source === "variant") {
      router.push(`${base}&variant_from=${variantFrom}&variant_to=${variantTo}`);
      return;
    }
    router.push(`${base}&has_image=${withImage}`);
  };

  const applyCustomCount = (raw: string) => {
    setCustomCount(raw);
    setAllQuestions(false);
    const parsed = Number(raw);
    if (Number.isInteger(parsed) && parsed > 0) {
      setCount(Math.min(parsed, MAX_CUSTOM_COUNT));
    }
  };

  const nextDueLabel = nextDueAt ? formatDateWithTime(nextDueAt) : null;

  const sources: { value: Source; icon: typeof BookOpen; label: string; hint: string; disabled?: boolean }[] = [
    {
      value: "due",
      icon: BrainCircuit,
      label: t("sourceDue"),
      // Always selectable — empty queue shows a waiting notice instead of a dead button.
      hint: dueCount > 0 ? t("sourceDueHint", { count: dueCount }) : t("sourceDueEmpty"),
    },
    { value: "category", icon: BookOpen, label: t("sourceCategory"), hint: t("sourceCategoryHint") },
    { value: "variant", icon: Layers, label: t("sourceVariant"), hint: t("sourceVariantHint") },
    { value: "image", icon: ImageIcon, label: t("sourceImage"), hint: t("sourceImageHint") },
    {
      value: "sign",
      icon: Signpost,
      label: t("sourceSign"),
      hint: signsAvailable ? t("sourceSignHint") : t("sourceSignUnavailable"),
      disabled: !signsAvailable,
    },
  ];

  return (
    <main className="page-shell-tight space-y-5 sm:space-y-6">
      <div>
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">{t("title")}</h1>
        <p className="mt-2 max-w-2xl text-base leading-relaxed text-muted-foreground">{t("subtitle")}</p>
      </div>

      {loadError && (
        <Card
          role="alert"
          className="flex flex-col items-start gap-3 border-destructive/40 bg-destructive/10 p-4 text-base sm:flex-row sm:items-center sm:justify-between"
        >
          <span>{t("categoriesLoadError")}</span>
          <Button variant="outline" size="sm" onClick={() => void loadContent()}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" />
            {t("retry")}
          </Button>
        </Card>
      )}

      {/* Source picker */}
      <section aria-label={t("sourceLabel")} className="space-y-3">
        <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
          {t("sourceLabel")}
        </h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
          {sources.map((item) => {
            const Icon = item.icon;
            const isSelected = source === item.value;
            return (
              <button
                key={item.value}
                type="button"
                disabled={item.disabled}
                aria-pressed={isSelected}
                onClick={() => setSource(item.value)}
                className={`surface-raised-sm surface-interactive min-h-touch rounded-2xl border p-4 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-55 disabled:hover:translate-y-0 ${
                  isSelected
                    ? "border-accent bg-accent/10 shadow-3d"
                    : "border-border bg-card hover:border-accent/50"
                }`}
              >
                <div className="flex items-center gap-2">
                  <Icon aria-hidden="true" className="h-5 w-5 shrink-0 text-accent" />
                  <span className="font-display text-base font-bold">{item.label}</span>
                  {isSelected && <CheckCircle2 aria-hidden="true" className="h-5 w-5 shrink-0 text-accent" />}
                </div>
                <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{item.hint}</p>
              </button>
            );
          })}
        </div>
      </section>

      {/* Source-specific selection */}
      {source === "due" && (
        <Card
          className={`space-y-3 p-5 sm:p-6 ${
            dueCount > 0
              ? "border-accent/35 bg-accent/5"
              : "border-gold/35 bg-gold/5"
          }`}
        >
          <div className="flex items-start gap-3">
            <div
              className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${
                dueCount > 0 ? "bg-accent/15 text-accent" : "bg-gold/15 text-gold"
              }`}
            >
              {dueCount > 0 ? (
                <BrainCircuit aria-hidden="true" className="h-5 w-5" />
              ) : (
                <CalendarClock aria-hidden="true" className="h-5 w-5" />
              )}
            </div>
            <div className="min-w-0 space-y-1.5">
              <h2 className="font-display text-lg font-extrabold tracking-tight">
                {dueCount > 0 ? t("sourceDue") : t("sourceDueWaitingTitle")}
              </h2>
              {dueCount > 0 ? (
                <p className="text-base leading-relaxed text-muted-foreground">
                  {t("sourceDueReady", { count: dueCount })}
                </p>
              ) : (
                <>
                  {nextDueLabel && (
                    <p className="text-base font-semibold text-gold">
                      {t("sourceDueWaitingNext", { when: nextDueLabel })}
                    </p>
                  )}
                  <p className="text-base leading-relaxed text-muted-foreground">
                    {bankCount > 0
                      ? t("sourceDueWaitingBank", { count: bankCount })
                      : t("sourceDueWaitingEmpty")}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="mt-2"
                    onClick={() => setSource("category")}
                  >
                    {t("sourceDueWaitingCta")}
                  </Button>
                </>
              )}
            </div>
          </div>
        </Card>
      )}

      {source === "category" && (
        <Card className="space-y-3 p-5 sm:p-6">
          <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("selectCategory")}
          </h2>
          <div className="grid gap-2.5 sm:grid-cols-2">
            {categories.map((cat) => {
              const isSelected = selectedCategory === cat.code;
              return (
                <button
                  key={cat.code}
                  type="button"
                  aria-pressed={isSelected}
                  onClick={() => setSelectedCategory(cat.code)}
                  className={`flex min-h-12 items-center justify-between gap-3 rounded-xl border p-3.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    isSelected ? "border-accent bg-accent/10" : "border-border bg-background hover:border-accent/50"
                  }`}
                >
                  <span className="text-base font-bold leading-snug">{cat.name}</span>
                  <span className="shrink-0 rounded-full bg-muted px-2.5 py-1 text-sm font-bold text-muted-foreground">
                    {t("categoryQuestionCount", { count: cat.question_count })}
                  </span>
                </button>
              );
            })}
          </div>
        </Card>
      )}

      {source === "variant" && (
        <Card className="space-y-4 p-5 sm:p-6">
          <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("variantRangeLabel")}
          </h2>
          <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1.5 text-sm font-bold text-muted-foreground">
              {t("variantFrom")}
              <input
                type="number"
                min={1}
                max={variantCount || 1}
                value={variantFrom}
                onChange={(event) => setVariantFrom(Number(event.target.value))}
                className="h-12 w-28 rounded-xl border border-border bg-background px-3 text-base font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
            <label className="flex flex-col gap-1.5 text-sm font-bold text-muted-foreground">
              {t("variantTo")}
              <input
                type="number"
                min={1}
                max={variantCount || 1}
                value={variantTo}
                onChange={(event) => setVariantTo(Number(event.target.value))}
                className="h-12 w-28 rounded-xl border border-border bg-background px-3 text-base font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
            {rangeValid && variantCount > 0 && (
              <p className="pb-2 text-sm font-semibold text-muted-foreground">
                {t("variantRangeSummary", {
                  from: variantFrom,
                  to: variantTo,
                  count: (variantTo - variantFrom + 1) * 20,
                })}
              </p>
            )}
          </div>
          {!rangeValid && variantCount > 0 && (
            <p role="alert" className="text-sm font-semibold text-destructive">
              {t("variantRangeInvalid")}
            </p>
          )}
        </Card>
      )}

      {source === "image" && (
        <Card className="grid gap-3 p-5 sm:grid-cols-2 sm:p-6">
          {[
            { value: true, icon: ImageIcon, label: t("imageWith") },
            { value: false, icon: AlignLeft, label: t("imageWithout") },
          ].map((item) => {
            const Icon = item.icon;
            const isSelected = withImage === item.value;
            return (
              <button
                key={String(item.value)}
                type="button"
                aria-pressed={isSelected}
                onClick={() => setWithImage(item.value)}
                className={`flex min-h-14 items-center gap-3 rounded-xl border p-4 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                  isSelected ? "border-accent bg-accent/10" : "border-border bg-background hover:border-accent/50"
                }`}
              >
                <Icon aria-hidden="true" className="h-6 w-6 shrink-0 text-accent" />
                <span className="text-base font-bold">{item.label}</span>
                {isSelected && <CheckCircle2 aria-hidden="true" className="ml-auto h-5 w-5 text-accent" />}
              </button>
            );
          })}
        </Card>
      )}

      {/* Question count + allowance */}
      {source !== "sign" && source !== "due" && (
        <Card className="space-y-4 p-5 sm:p-6">
          <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("countLabel")}
          </h2>
          <div className="flex flex-wrap gap-3">
            {COUNT_PRESETS.map((preset) => (
              <Button
                key={preset}
                variant={!allQuestions && count === preset && customCount === "" ? "game" : "outline"}
                size="sm"
                aria-pressed={!allQuestions && count === preset && customCount === ""}
                onClick={() => {
                  setAllQuestions(false);
                  setCustomCount("");
                  setCount(preset);
                }}
                className="min-w-28 py-2.5 text-sm font-extrabold"
              >
                {t("questionCountOption", { count: preset })}
              </Button>
            ))}
            <Button
              variant={allQuestions ? "game" : "outline"}
              size="sm"
              aria-pressed={allQuestions}
              onClick={() => {
                setAllQuestions(true);
                setCustomCount("");
                setCount(MAX_CUSTOM_COUNT);
              }}
              className="min-w-28 py-2.5 text-sm font-extrabold"
            >
              {t("countAll")}
            </Button>
            <label className="flex items-center gap-2 text-sm font-bold text-muted-foreground">
              {t("countCustom")}
              <input
                type="number"
                min={1}
                max={MAX_CUSTOM_COUNT}
                value={customCount}
                placeholder={t("countCustomPlaceholder")}
                onChange={(event) => applyCustomCount(event.target.value)}
                className="h-12 w-28 rounded-xl border border-border bg-background px-3 text-base font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
          </div>

          {allowance && (
            <div
              className={`rounded-xl border p-3.5 text-sm font-semibold ${
                exhausted
                  ? "border-gold/40 bg-gold/10 text-gold"
                  : "border-border bg-background text-muted-foreground"
              }`}
            >
              {allowance.unlimited ? (
                <span className="flex items-center gap-1.5 text-gold">
                  <Crown aria-hidden="true" className="h-4 w-4" />
                  {t("allowanceUnlimited")}
                </span>
              ) : exhausted ? (
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span>{t("allowanceExhausted")}</span>
                  <Link
                    href={`/${locale}/premium`}
                    className="inline-flex min-h-11 items-center gap-1.5 rounded-xl bg-gold px-3 text-sm font-extrabold text-slate-950 hover:brightness-105"
                  >
                    <Crown aria-hidden="true" className="h-4 w-4" />
                    {t("allowanceUpgrade")}
                  </Link>
                </div>
              ) : (
                <div className="space-y-1">
                  <p>{t("allowanceRemaining", { count: allowance.remaining })}</p>
                  {clamped && <p className="text-gold">{t("allowanceClamped", { count: effectiveCount })}</p>}
                </div>
              )}
            </div>
          )}
        </Card>
      )}

      {source === "sign" && signsAvailable && (
        <section className="space-y-3" aria-label={t("sourceSignPick")}>
          <div className="flex items-end justify-between gap-3">
            <div>
              <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
                {t("sourceSignPick")}
              </h2>
              <p className="text-base text-muted-foreground">{t("sourceSignHint")}</p>
            </div>
            <Link
              href={`/${locale}/signs`}
              className="shrink-0 text-sm font-bold text-accent hover:underline"
            >
              {t("openSigns")}
            </Link>
          </div>
          <SignPracticeGrid signs={signs} emptyHint={t("sourceSignEmptyLinks")} />
        </section>
      )}

      {imageCounts === 0 && !loading && !loadError && (
        <p className="text-center text-sm text-muted-foreground">{t("categoryCountUnavailable")}</p>
      )}

      {/* Page-level so `sticky bottom-0` can pin to the viewport for the whole
          scroll (inside a Card it could never leave the card's box). */}
      {source !== "sign" && (
        <div className="sticky-cta-bar">
          <Button
            variant="game"
            size="lg"
            className="w-full text-base"
            onClick={handleStart}
            disabled={!canStart}
          >
            <Play aria-hidden="true" className="mr-2 h-5 w-5 fill-current" />
            {t("startPractice")}
          </Button>
        </div>
      )}
    </main>
  );
}

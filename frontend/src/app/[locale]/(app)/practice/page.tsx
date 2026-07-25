"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet } from "@/lib/api-client";
import { usePracticeAllowance } from "@/hooks/use-practice-allowance";
import { useSigns } from "@/hooks/use-signs";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  AlignLeft,
  ArrowLeft,
  BookOpen,
  CheckCircle2,
  Crown,
  Image as ImageIcon,
  Layers,
  Play,
  RefreshCw,
  Signpost,
} from "lucide-react";

const COUNT_PRESETS = [20, 50, 100] as const;
const MAX_CUSTOM_COUNT = 200;

type Source = "category" | "variant" | "image" | "sign";

interface CategoryItem {
  code: string;
  name: string;
  sort_order: number;
  question_count: number;
}

interface VariantItem {
  number: number;
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
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<boolean>(false);

  const { allowance } = usePracticeAllowance();
  const { signs } = useSigns(locale);
  const signsAvailable = signs.length > 0;

  const loadContent = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const [cats, variants] = await Promise.all([
        apiGet<CategoryItem[]>(`categories?locale=${encodeURIComponent(locale)}`),
        apiGet<VariantItem[]>("variants"),
      ]);
      const ordered = [...cats].sort((left, right) => left.sort_order - right.sort_order);
      setCategories(ordered);
      setSelectedCategory(ordered[0]?.code ?? "");

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
    if (loading || exhausted) return false;
    if (source === "category") return Boolean(selectedCategory);
    if (source === "variant") return rangeValid;
    if (source === "sign") return signsAvailable;
    return true;
  })();

  const handleStart = () => {
    const base = `/${locale}/session/start?mode=practice&count=${count}`;
    if (source === "category") {
      router.push(`${base}&category_id=${encodeURIComponent(selectedCategory)}`);
      return;
    }
    if (source === "variant") {
      router.push(`${base}&variant_from=${variantFrom}&variant_to=${variantTo}`);
      return;
    }
    if (source === "sign") {
      router.push(`/${locale}/signs`);
      return;
    }
    router.push(`${base}&has_image=${withImage}`);
  };

  const applyCustomCount = (raw: string) => {
    setCustomCount(raw);
    const parsed = Number(raw);
    if (Number.isInteger(parsed) && parsed > 0) {
      setCount(Math.min(parsed, MAX_CUSTOM_COUNT));
    }
  };

  const sources: { value: Source; icon: typeof BookOpen; label: string; hint: string; disabled?: boolean }[] = [
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
        <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">{t("title")}</h1>
        <p className="mt-1 max-w-xl text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {loadError && (
        <Card
          role="alert"
          className="flex flex-col items-start gap-3 border-destructive/40 bg-destructive/10 p-4 text-sm sm:flex-row sm:items-center sm:justify-between"
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
        <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
          {t("sourceLabel")}
        </h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
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
                className={`min-h-touch rounded-2xl border p-4 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-55 ${
                  isSelected
                    ? "border-accent bg-accent/10"
                    : "border-border bg-card hover:border-accent/50"
                }`}
              >
                <div className="flex items-center gap-2">
                  <Icon aria-hidden="true" className="h-5 w-5 text-accent" />
                  <span className="font-display text-sm font-bold">{item.label}</span>
                  {isSelected && <CheckCircle2 aria-hidden="true" className="h-4 w-4 text-accent" />}
                </div>
                <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{item.hint}</p>
              </button>
            );
          })}
        </div>
      </section>

      {/* Source-specific selection */}
      {source === "category" && (
        <Card className="space-y-3 p-5">
          <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("selectCategory")}
          </h2>
          <div className="grid gap-2 sm:grid-cols-2">
            {categories.map((cat) => {
              const isSelected = selectedCategory === cat.code;
              return (
                <button
                  key={cat.code}
                  type="button"
                  aria-pressed={isSelected}
                  onClick={() => setSelectedCategory(cat.code)}
                  className={`flex min-h-11 items-center justify-between gap-3 rounded-xl border p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                    isSelected ? "border-accent bg-accent/10" : "border-border bg-background hover:border-accent/50"
                  }`}
                >
                  <span className="text-xs font-bold leading-relaxed">{cat.name}</span>
                  <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-[11px] font-bold text-muted-foreground">
                    {t("categoryQuestionCount", { count: cat.question_count })}
                  </span>
                </button>
              );
            })}
          </div>
        </Card>
      )}

      {source === "variant" && (
        <Card className="space-y-4 p-5">
          <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("variantRangeLabel")}
          </h2>
          <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1 text-xs font-bold text-muted-foreground">
              {t("variantFrom")}
              <input
                type="number"
                min={1}
                max={variantCount || 1}
                value={variantFrom}
                onChange={(event) => setVariantFrom(Number(event.target.value))}
                className="h-11 w-24 rounded-xl border border-border bg-background px-3 text-sm font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
            <label className="flex flex-col gap-1 text-xs font-bold text-muted-foreground">
              {t("variantTo")}
              <input
                type="number"
                min={1}
                max={variantCount || 1}
                value={variantTo}
                onChange={(event) => setVariantTo(Number(event.target.value))}
                className="h-11 w-24 rounded-xl border border-border bg-background px-3 text-sm font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
            {rangeValid && variantCount > 0 && (
              <p className="pb-2 text-xs font-semibold text-muted-foreground">
                {t("variantRangeSummary", {
                  from: variantFrom,
                  to: variantTo,
                  count: (variantTo - variantFrom + 1) * 20,
                })}
              </p>
            )}
          </div>
          {!rangeValid && variantCount > 0 && (
            <p role="alert" className="text-xs font-semibold text-destructive">
              {t("variantRangeInvalid")}
            </p>
          )}
        </Card>
      )}

      {source === "image" && (
        <Card className="grid gap-3 p-5 sm:grid-cols-2">
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
                className={`flex items-center gap-3 rounded-xl border p-4 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                  isSelected ? "border-accent bg-accent/10" : "border-border bg-background hover:border-accent/50"
                }`}
              >
                <Icon aria-hidden="true" className="h-5 w-5 shrink-0 text-accent" />
                <span className="text-sm font-bold">{item.label}</span>
                {isSelected && <CheckCircle2 aria-hidden="true" className="ml-auto h-4 w-4 text-accent" />}
              </button>
            );
          })}
        </Card>
      )}

      {/* Question count + allowance */}
      {source !== "sign" && (
        <Card className="space-y-4 p-5">
          <h2 className="text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("countLabel")}
          </h2>
          <div className="flex flex-wrap gap-3">
            {COUNT_PRESETS.map((preset) => (
              <Button
                key={preset}
                variant={count === preset && customCount === "" ? "game" : "outline"}
                size="sm"
                aria-pressed={count === preset && customCount === ""}
                onClick={() => {
                  setCustomCount("");
                  setCount(preset);
                }}
                className="min-w-24 py-2 text-xs font-extrabold"
              >
                {t("questionCountOption", { count: preset })}
              </Button>
            ))}
            <label className="flex items-center gap-2 text-xs font-bold text-muted-foreground">
              {t("countCustom")}
              <input
                type="number"
                min={1}
                max={MAX_CUSTOM_COUNT}
                value={customCount}
                placeholder={t("countCustomPlaceholder")}
                onChange={(event) => applyCustomCount(event.target.value)}
                className="h-11 w-28 rounded-xl border border-border bg-background px-3 text-sm font-bold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
          </div>

          {allowance && (
            <div
              className={`rounded-xl border p-3 text-xs font-semibold ${
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
                    className="inline-flex min-h-11 items-center gap-1.5 rounded-xl bg-gold px-3 text-[11px] font-extrabold text-slate-950 hover:brightness-105"
                  >
                    <Crown aria-hidden="true" className="h-3.5 w-3.5" />
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
        <Card className="p-5 text-center">
          <p className="text-sm text-muted-foreground">{t("sourceSignHint")}</p>
        </Card>
      )}

      {imageCounts === 0 && !loading && !loadError && (
        <p className="text-center text-xs text-muted-foreground">{t("categoryCountUnavailable")}</p>
      )}

      {/* Page-level so `sticky bottom-0` can pin to the viewport for the whole
          scroll (inside a Card it could never leave the card's box). */}
      <div className="sticky-cta-bar">
        <Button
          variant="game"
          size="lg"
          className="w-full text-base"
          onClick={handleStart}
          disabled={!canStart}
        >
          {source === "sign" ? (
            <Signpost aria-hidden="true" className="mr-2 h-5 w-5" />
          ) : (
            <Play aria-hidden="true" className="mr-2 h-5 w-5 fill-current" />
          )}
          {t("startPractice")}
        </Button>
      </div>
    </main>
  );
}

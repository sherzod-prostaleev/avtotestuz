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
  TriangleAlert,
  Route,
  Siren,
  Car,
  Navigation,
  MapPin,
  Truck,
  HeartPulse,
} from "lucide-react";
import { BackLink } from "@/components/layout/back-link";
import { useCatalogCounts } from "@/hooks/use-variant-count";


// Every preset is a one-tap start, so the list runs past a single sitting on
// purpose: a learner picking 800 is choosing a marathon, and the server clamps
// to whatever the bank and the daily allowance actually hold.
const COUNT_PRESETS = [20, 50, 100, 200, 400, 500, 800, 1000] as const;

type Source = "due" | "category" | "variant" | "image" | "sign";

interface TopicSectionDef {
  key: string;
  icon: typeof BookOpen;
  codes: readonly string[];
}

// The 42 topics are shown as one continuous list — the YHQ chapter order is
// the order a learner is taught in, and nine collapsed headers hid 38 of them
// behind a tap. The section a topic belongs to still rides on its card as a
// label, so "the sign topics" stays findable without a band to open.
const TOPIC_SECTIONS: readonly TopicSectionDef[] = [
  {
    key: "sectionGeneralRulesDuties",
    icon: BookOpen,
    codes: ["general_rules", "driver_duties", "pedestrian_duties", "special_vehicle_priority"],
  },
  {
    key: "sectionRoadSigns",
    icon: TriangleAlert,
    codes: [
      "signs_warning",
      "signs_priority",
      "signs_prohibitory",
      "signs_mandatory",
      "signs_information",
      "signs_service",
      "signs_additional",
    ],
  },
  {
    key: "sectionRoadMarkings",
    icon: Route,
    codes: ["markings_horizontal", "markings_vertical"],
  },
  {
    key: "sectionSignals",
    icon: Siren,
    codes: ["traffic_lights", "traffic_controller", "warning_hazard_signals"],
  },
  {
    key: "sectionDrivingOrder",
    icon: Car,
    codes: ["starting_manoeuvring", "lane_position", "speed_limits", "overtaking", "stopping_and_parking"],
  },
  {
    key: "sectionIntersections",
    icon: Navigation,
    codes: [
      "intersections_general",
      "intersections_regulated",
      "intersections_main_straight",
      "intersections_equal",
      "intersections_main_turns",
    ],
  },
  {
    key: "sectionSpecialSectionsPriority",
    icon: MapPin,
    codes: [
      "pedestrian_crossings_stops",
      "railway_crossings",
      "motorways",
      "residential_zones",
      "slopes",
      "public_transport_priority",
    ],
  },
  {
    key: "sectionVehicleCarriage",
    icon: Truck,
    codes: [
      "lighting_devices",
      "towing",
      "driver_training",
      "passenger_carriage",
      "cargo_carriage",
      "cyclists_mopeds_animals",
      "officials_duties",
      "vehicle_defects",
    ],
  },
  {
    key: "sectionSafetyFirstAid",
    icon: HeartPulse,
    codes: ["safety_basics", "first_aid"],
  },
] as const;

/** code -> the section it belongs to, so a card can label itself. */
const SECTION_BY_CODE: ReadonlyMap<string, TopicSectionDef> = new Map(
  TOPIC_SECTIONS.flatMap((section) => section.codes.map((code) => [code, section] as const))
);

interface CategoryItem {
  code: string;
  name: string;
  sort_order: number;
  question_count: number;
}

interface VariantItem {
  number: number;
  question_count?: number;
}

interface StatsDTO {
  due_count: number;
}

interface MistakesDTO {
  due_count: number;
  total_bank_count: number;
  next_due_at: string | null;
}

const categoriesModuleCache: Record<string, CategoryItem[]> = {};
let variantsModuleCache: number | null = null;
let statsModuleCache: { dueCount: number; bankCount: number; nextDueAt: string | null } | null = null;
let practiceFetchedAt = 0;
const practiceInFlight = new Map<string, Promise<void>>();

export function clearPracticeCacheForTests(): void {
  for (const k in categoriesModuleCache) delete categoriesModuleCache[k];
  variantsModuleCache = null;
  statsModuleCache = null;
  practiceFetchedAt = 0;
  practiceInFlight.clear();
}

export interface PracticePageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/practice/page.tsx):
  // a walk-up student has no dashboard or VIP checkout to go back to, so
  // those two surfaces must not appear when kiosk is true.
  kiosk?: boolean;
}

export default function PracticePage({ kiosk = false }: PracticePageProps = {}) {
  const t = useTranslations("Practice");
  // "Hammasi" means the bank as it actually is, so a freshly imported
  // question is included without touching this file.
  const { questions: maxCustomCount } = useCatalogCounts();
  const locale = useLocale();
  const router = useRouter();
  const backHref = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;
  // /station/session/start (not /session/start): keeps the proxy.ts kiosk
  // exemption scoped to the /station/... namespace instead of unauthenticating
  // /session/start for every learner — see the (kiosk)/station/session/start
  // route, which reuses this same session/start page with kiosk=true.
  const sessionStartBase = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;

  const [source, setSource] = useState<Source>("category");
  const [categories, setCategories] = useState<CategoryItem[]>(() => categoriesModuleCache[locale] ?? []);
  // Highest bilet number the bank holds. The ticket source draws across the
  // whole span, so this is the only part of the old from/to pair still needed.
  const [variantCount, setVariantCount] = useState<number>(() => variantsModuleCache ?? 0);
  const [withImage, setWithImage] = useState<boolean>(true);
  const [loading, setLoading] = useState<boolean>(() => !categoriesModuleCache[locale]);
  const [loadError, setLoadError] = useState<boolean>(false);
  const [dueCount, setDueCount] = useState<number>(() => statsModuleCache?.dueCount ?? 0);
  const [bankCount, setBankCount] = useState<number>(() => statsModuleCache?.bankCount ?? 0);
  const [nextDueAt, setNextDueAt] = useState<string | null>(() => statsModuleCache?.nextDueAt ?? null);

  const { allowance } = usePracticeAllowance();
  const { signs } = useSigns(locale);
  const signsAvailable = signs.length > 0;

  const loadContent = useCallback(async () => {
    if (categoriesModuleCache[locale] && Date.now() - practiceFetchedAt < 30_000) {
      setCategories(categoriesModuleCache[locale]);
      if (variantsModuleCache !== null) setVariantCount(variantsModuleCache);
      if (statsModuleCache) {
        setDueCount(statsModuleCache.dueCount);
        setBankCount(statsModuleCache.bankCount);
        setNextDueAt(statsModuleCache.nextDueAt);
      }
      setLoading(false);
      return;
    }

    const existingTask = practiceInFlight.get(locale);
    if (existingTask) {
      await existingTask;
      if (categoriesModuleCache[locale]) {
        setCategories(categoriesModuleCache[locale]);
        if (variantsModuleCache !== null) setVariantCount(variantsModuleCache);
        if (statsModuleCache) {
          setDueCount(statsModuleCache.dueCount);
          setBankCount(statsModuleCache.bankCount);
          setNextDueAt(statsModuleCache.nextDueAt);
        }
      }
      setLoading(false);
      return;
    }

    if (!categoriesModuleCache[locale]) {
      setLoading(true);
    }
    setLoadError(false);

    const task = (async () => {
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
        categoriesModuleCache[locale] = ordered;
        practiceFetchedAt = Date.now();
        setCategories(ordered);

        const dueFromMistakes = Number.isInteger(mistakes.due_count) ? mistakes.due_count : 0;
        const dueFromStats = Number.isInteger(stats.due_count) ? stats.due_count : 0;
        const calculatedDue = Math.max(dueFromMistakes, dueFromStats);
        const calculatedBank = Number.isInteger(mistakes.total_bank_count) ? mistakes.total_bank_count : 0;
        const calculatedNextDue = typeof mistakes.next_due_at === "string" ? mistakes.next_due_at : null;

        statsModuleCache = { dueCount: calculatedDue, bankCount: calculatedBank, nextDueAt: calculatedNextDue };
        setDueCount(calculatedDue);
        setBankCount(calculatedBank);
        setNextDueAt(calculatedNextDue);

        const maxVar = variants.reduce((max, item) => Math.max(max, item.number), 0);
        variantsModuleCache = maxVar;
        setVariantCount(maxVar);
      } catch {
        if (!categoriesModuleCache[locale]) {
          setCategories([]);
          setVariantCount(0);
          setDueCount(0);
          setBankCount(0);
          setNextDueAt(null);
          setLoadError(true);
        }
      } finally {
        practiceInFlight.delete(locale);
        setLoading(false);
      }
    })();

    practiceInFlight.set(locale, task);
    await task;
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

  const exhausted = Boolean(allowance && !allowance.unlimited && allowance.remaining <= 0);

  // Every source except Takrorlash now starts from the thing you pick — a
  // topic, or a size — so Start survives only for the review queue, which has
  // nothing to pick.
  const canStart = !loading && source === "due" && dueCount > 0;

  // The ticket source draws across every bilet, so it needs the bank loaded
  // before a size button can mean anything.
  const countStartReady = !loading && !exhausted && (source !== "variant" || variantCount > 0);

  const handleCategoryClick = (catCode: string) => {
    router.push(
      `${sessionStartBase}?mode=practice&count=${maxCustomCount}&category_id=${encodeURIComponent(catCode)}&ordered=true`
    );
  };

  // Ticket and image practice are a single tap: the size *is* the start button.
  // Both stay random draws — only "all questions of one topic" is ordered.
  const handleCountClick = (size: number) => {
    if (!countStartReady) return;
    const base = `${sessionStartBase}?mode=practice&count=${size}`;
    if (source === "variant") {
      router.push(`${base}&variant_from=1&variant_to=${variantCount}`);
      return;
    }
    router.push(`${base}&has_image=${withImage}`);
  };

  const handleStart = () => {
    if (source !== "due") return;
    const reviewCount = Math.min(20, Math.max(dueCount, 1));
    router.push(`${sessionStartBase}?mode=review&count=${reviewCount}`);
  };

  const nextDueLabel = nextDueAt ? formatDateWithTime(nextDueAt) : null;

  const allSources: { value: Source; icon: typeof BookOpen; label: string; hint: string; disabled?: boolean }[] = [
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
  // The sign source is a browse-and-deep-link picker (SignPracticeGrid links
  // to /signs and /session/start directly, bypassing sessionStartBase) with
  // no station-scoped equivalent, and it never supports Start anyway
  // (canStart is false for it below). Drop it entirely on the kiosk instead
  // of leaving a tile that dead-ends at a login-gated route.
  const sources = kiosk ? allSources.filter((item) => item.value !== "sign") : allSources;

  return (
    <main
      className={`space-y-5 sm:space-y-6 ${
        // A classroom monitor is 1920px wide and the learner shell caps at
        // 1024, so half of it sat empty. Wider shell, same four columns:
        // the cards grow instead of multiplying.
        kiosk ? "page-shell" : "page-shell-tight"
      }`}
    >
      <div>
        <BackLink href={backHref} kiosk={kiosk}>{t("backHome")}</BackLink>
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
        <Card className="space-y-4 p-5 sm:p-6">
          <h2 className="max-w-full text-[11px] font-extrabold uppercase leading-snug tracking-wide text-muted-foreground sm:text-sm sm:tracking-wider">
            {t("selectCategory")}
          </h2>
          {/* One continuous 1..42 grid in YHQ chapter order. Two columns on a
              phone, four from `sm` up — the kiosk gets the same four in a wider
              shell, so a classroom screen scales the cards rather than cramming
              a fifth column in. */}
          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3 lg:grid-cols-4">
            {categories.length === 0 && loading
              ? Array.from({ length: 42 }).map((_, i) => (
                  <div
                    key={i}
                    aria-hidden="true"
                    className={`surface-raised-sm flex flex-col justify-between gap-2 rounded-2xl border border-border bg-background p-3 animate-pulse ${
                      kiosk ? "min-h-[8.25rem] p-4" : "min-h-[7rem]"
                    }`}
                  >
                    <div className="flex items-center gap-1.5">
                      <div className="h-3.5 w-3.5 rounded bg-border/60" />
                      <div className="h-2.5 w-20 rounded bg-border/60" />
                    </div>
                    <div className="space-y-1">
                      <div className="h-4 w-full rounded bg-border/60" />
                      <div className="h-3 w-2/3 rounded bg-border/50" />
                    </div>
                    <div className="flex items-center justify-between">
                      <div className="h-3 w-12 rounded bg-border/50" />
                      <div className="h-6 w-6 rounded-lg bg-border/60" />
                    </div>
                  </div>
                ))
              : categories.map((cat) => {
                  const section = SECTION_BY_CODE.get(cat.code);
                  const SectionIcon = section?.icon;
                  return (
                    <button
                      key={cat.code}
                      type="button"
                      onClick={() => handleCategoryClick(cat.code)}
                      className={`surface-raised-sm surface-interactive flex flex-col justify-between gap-2 rounded-2xl border border-border bg-background p-3 text-left hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                        kiosk ? "min-h-[8.25rem] p-4" : "min-h-[7rem]"
                      }`}
                    >
                      {section && SectionIcon && (
                        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground max-md:hidden">
                          <SectionIcon aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
                          <span
                            className={`truncate font-bold uppercase tracking-wider ${
                              kiosk ? "text-[11px]" : "text-[10px]"
                            }`}
                          >
                            {t(section.key as never)}
                          </span>
                        </span>
                      )}
                      <span
                        className={`line-clamp-2 font-bold leading-snug max-md:line-clamp-3 ${
                          kiosk ? "text-base" : "text-sm"
                        }`}
                      >
                        {cat.name}
                      </span>
                      <span className="flex items-center justify-between gap-2">
                        <span
                          className={`font-semibold tabular-nums text-muted-foreground ${
                            kiosk ? "text-sm" : "text-xs"
                          }`}
                        >
                          {t("categoryQuestionCount", { count: cat.question_count })}
                        </span>
                        <span
                          className={`inline-flex shrink-0 items-center justify-center rounded-lg bg-accent/15 font-bold tabular-nums text-accent ${
                            kiosk ? "h-7 min-w-7 text-xs" : "h-6 min-w-6 text-[11px]"
                          }`}
                        >
                          {cat.sort_order}
                        </span>
                      </span>
                    </button>
                  );
                })}
          </div>
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

      {/* Ticket and image practice: the size is the start button. Presented as
          a grid rather than a row because eight sizes wrap badly on a phone,
          and the kiosk is touched, not clicked. */}
      {(source === "variant" || source === "image") && (
        <Card className="space-y-4 p-5 sm:p-6">
          <h2 className="text-sm font-extrabold uppercase tracking-wider text-muted-foreground">
            {t("countLabel")}
          </h2>
          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
            {COUNT_PRESETS.map((preset) => (
              <Button
                key={preset}
                variant="outline"
                onClick={() => handleCountClick(preset)}
                disabled={!countStartReady}
                className="min-h-12 py-2.5 text-sm font-extrabold"
              >
                {t("questionCountOption", { count: preset })}
              </Button>
            ))}
            <Button
              variant="game"
              onClick={() => handleCountClick(maxCustomCount)}
              disabled={!countStartReady}
              className="min-h-12 py-2.5 text-sm font-extrabold"
            >
              {t("countAll")}
            </Button>
          </div>
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
            {/* Unreachable today — sources filters out "sign" on the kiosk, so
                source can never be "sign" there — but guarded so it stays
                correct if source ever becomes settable independently. */}
            {!kiosk && (
              <Link
                // kiosk-safe: wrapped in the !kiosk check two lines up, and sources already filters "sign" out of the kiosk's source list, so this block is structurally unreachable there
                href={`/${locale}/signs`}
                className="shrink-0 text-sm font-bold text-accent hover:underline"
              >
                {t("openSigns")}
              </Link>
            )}
          </div>
          <SignPracticeGrid signs={signs} emptyHint={t("sourceSignEmptyLinks")} />
        </section>
      )}

      {/* Only the review queue still needs a Start: topics, tickets and image
          practice all start from the card you picked in. */}
      {source === "due" && (
        <div className="mt-6 sm:mt-8">
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


      {/* Outside the size card on purpose: every source except the review queue
          now starts from its own card, and the daily budget has to stay visible
          in all of them or a learner meets the limit only as a failed start. */}
      {allowance && source !== "sign" && (
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
              {/* No checkout entry point on the kiosk: a walk-up student
                  must never be one tap from VIP purchase. */}
              {!kiosk && (
                <Link
                  // kiosk-safe: wrapped in the !kiosk check two lines up — no checkout entry point on the kiosk, a walk-up student must never be one tap from VIP purchase
                  href={`/${locale}/premium`}
                  className="inline-flex min-h-11 items-center gap-1.5 rounded-xl bg-gold px-3 text-sm font-extrabold text-slate-950 hover:brightness-105"
                >
                  <Crown aria-hidden="true" className="h-4 w-4" />
                  {t("allowanceUpgrade")}
                </Link>
              )}
            </div>
          ) : (
            <p>{t("allowanceRemaining", { count: allowance.remaining })}</p>
          )}
        </div>
      )}

      {imageCounts === 0 && !loading && !loadError && (
        <p className="text-center text-sm text-muted-foreground">{t("categoryCountUnavailable")}</p>
      )}

    </main>
  );
}

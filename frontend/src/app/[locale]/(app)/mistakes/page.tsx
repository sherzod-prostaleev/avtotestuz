"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  ArrowLeft,
  BookOpen,
  BarChart3,
  CalendarClock,
  CheckCircle2,
  ChevronRight,
  Clock,
  LockKeyhole,
  Play,
  RefreshCw,
} from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface MistakesData {
  due_count: number;
  total_bank_count: number;
  next_due_at: string | null;
}

interface EntitlementData {
  active: boolean;
  until: string | null;
}

const dateLocales: Record<string, string> = {
  "uz-Latn": "uz-Latn-UZ",
  "uz-Cyrl": "uz-Cyrl-UZ",
  ru: "ru-RU",
};

function isMistakesData(value: unknown): value is MistakesData {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<MistakesData>;
  return (
    Number.isInteger(candidate.due_count) &&
    Number.isInteger(candidate.total_bank_count) &&
    (candidate.due_count ?? -1) >= 0 &&
    (candidate.total_bank_count ?? -1) >= 0 &&
    (candidate.next_due_at === null || typeof candidate.next_due_at === "string")
  );
}

function isEntitlementData(value: unknown): value is EntitlementData {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<EntitlementData>;
  return (
    typeof candidate.active === "boolean" &&
    (candidate.until === null || typeof candidate.until === "string")
  );
}

import { daysUntil, formatDateWithTime } from "@/lib/date-format";

function formatNextDue(value: string | null): string | null {
  if (!value) return null;
  const formatted = formatDateWithTime(value);
  return formatted || null;
}

const linkButtonClass =
  "inline-flex min-h-11 items-center justify-center rounded-xl bg-accent px-5 text-sm font-bold tracking-wide text-accent-foreground shadow-3d transition-all hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export default function MistakesPage() {
  const t = useTranslations("Mistakes");
  const locale = useLocale();
  const router = useRouter();
  const [data, setData] = useState<MistakesData | null>(null);
  const [entitlement, setEntitlement] = useState<EntitlementData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [starting, setStarting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    setData(null);
    setEntitlement(null);

    try {
      const [mistakesResult, entitlementResult] = await Promise.all([
        apiGet<unknown>("me/mistakes"),
        apiGet<unknown>("me/entitlement"),
      ]);
      if (!isMistakesData(mistakesResult) || !isEntitlementData(entitlementResult)) {
        throw new Error("Invalid mistakes response");
      }
      setData(mistakesResult);
      setEntitlement(entitlementResult);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleStart = () => {
    if (!data || data.due_count < 1 || starting) return;
    setStarting(true);
    router.push(`/${locale}/session/start?mode=mistakes&count=${data.due_count}`);
  };

  const nextDue = formatNextDue(data?.next_due_at ?? null);
  // The one state the design draws for the phone: a bank with something due
  // and an entitlement to review it. The other three keep today's cards.
  const isReviewable = Boolean(
    data && entitlement?.active && data.total_bank_count > 0 && data.due_count > 0
  );
  const dueInDays = daysUntil(data?.next_due_at ?? null);
  const nextDueRelative =
    dueInDays === null
      ? t("nextDueNone")
      : dueInDays <= 0
        ? t("nextDueToday")
        : dueInDays === 1
          ? t("nextDueTomorrow")
          : t("nextDueInDays", { days: dueInDays });

  return (
    <main className="page-shell-narrow space-y-6">
      {/* The phone drops the back link the tab bar already provides, and names
          the screen the way the menu does — "Xatolar banki" rather than the
          desktop's longer "Xatolar ustida ishlash". */}
      <header>
        <Link href={`/${locale}/dashboard`} className="back-link max-md:hidden">
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("back")}
        </Link>
        <h1 className="font-display text-2xl font-bold tracking-tight max-md:hidden">{t("title")}</h1>
        <p className="text-sm text-muted-foreground max-md:hidden">{t("subtitle")}</p>
        <h1 className="font-display text-2xl font-bold tracking-tight md:hidden">{t("titleMobile")}</h1>
        <p className="text-sm text-muted-foreground md:hidden">{t("subtitleMobile")}</p>
      </header>

      {loading && (
        <div className="flex min-h-40 items-center justify-center" role="status" aria-live="polite">
          <RefreshCw className="mr-2 h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          <span className="text-sm text-muted-foreground">{t("loading")}</span>
        </div>
      )}

      {!loading && error && (
        <Card className="border-destructive/40 bg-destructive/5 p-6 text-center" role="alert">
          <AlertTriangle className="mx-auto mb-3 h-10 w-10 text-destructive" aria-hidden="true" />
          <p className="font-semibold">{t("loadError")}</p>
          <Button className="mt-5" variant="outline" onClick={() => void load()}>
            <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
            {t("retry")}
          </Button>
        </Card>
      )}

      {!loading && !error && data && entitlement && (
        <div className="space-y-6">
          {/* Hidden on the phone only in the review-ready state, where the red
              card below carries both numbers; the other three states still
              need this grid at every width. */}
          <div
            className={`grid gap-4 sm:grid-cols-2 ${isReviewable ? "max-md:hidden" : ""}`}
            aria-label={t("title")}
          >
            <Card className="border-accent/40 bg-accent/5 p-5">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">
                  {t("dueNow")}
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <p className="font-display text-4xl font-extrabold text-accent">{data.due_count}</p>
              </CardContent>
            </Card>

            <Card className="p-5">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm font-semibold text-muted-foreground">
                  {t("totalBank")}
                </CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <p className="font-display text-4xl font-extrabold">{data.total_bank_count}</p>
              </CardContent>
            </Card>
          </div>

          {!entitlement.active ? (
            <Card className="border-gold/40 bg-gold/5 p-6 text-center sm:p-8">
              <LockKeyhole className="mx-auto mb-3 h-12 w-12 text-gold" aria-hidden="true" />
              <h2 className="font-display text-xl font-bold">{t("vipRequired")}</h2>
              <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">{t("vipBody")}</p>
              <div className="mt-6">
                <Link className={`${linkButtonClass} w-full sm:w-auto`} href={`/${locale}/premium`}>
                  {t("upgrade")}
                </Link>
              </div>
            </Card>
          ) : data.total_bank_count === 0 ? (
            <Card className="p-6 text-center sm:p-8">
              <BookOpen className="mx-auto mb-3 h-12 w-12 text-accent" aria-hidden="true" />
              <h2 className="font-display text-lg font-bold">{t("emptyBankTitle")}</h2>
              <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">
                {t("emptyBankBody")}
              </p>
              <div className="mt-6">
                <Link className={`${linkButtonClass} w-full sm:w-auto`} href={`/${locale}/tickets`}>
                  {t("browseTickets")}
                </Link>
              </div>
            </Card>
          ) : data.due_count === 0 ? (
            <Card className="p-6 text-center sm:p-8">
              {nextDue ? (
                <CalendarClock className="mx-auto mb-3 h-12 w-12 text-success" aria-hidden="true" />
              ) : (
                <CheckCircle2 className="mx-auto mb-3 h-12 w-12 text-success" aria-hidden="true" />
              )}
              <h2 className="font-display text-lg font-bold">{t("emptyDueTitle")}</h2>
              <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">
                {nextDue ? t("emptyDueWithDate", { date: nextDue }) : t("emptyDueWithoutDate")}
              </p>
            </Card>
          ) : (
            <>
              {/* `max-md:[&+*]:!mt-0`: on a phone this card and the grid above
                  are both `display:none`, yet still count as siblings for the
                  wrapper's `space-y`, so the block below would open with a
                  margin and nothing above it to justify one. */}
              <Card className="p-5 max-md:hidden max-md:[&+*]:!mt-0 sm:p-6">
                <div className="flex flex-col items-stretch justify-between gap-5 sm:flex-row sm:items-center">
                  <div>
                    <h2 className="font-display text-lg font-bold">{t("reviewTitle")}</h2>
                    <p className="mt-1 text-sm text-muted-foreground">{t("fsrsNote")}</p>
                  </div>
                  <div className="w-full sm:w-auto">
                    <Button variant="game" className="w-full sm:w-auto" onClick={handleStart} disabled={starting}>
                      {starting ? (
                        <RefreshCw className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
                      ) : (
                        <Play className="mr-2 h-4 w-4" aria-hidden="true" />
                      )}
                      {starting ? t("starting") : t("start")}
                    </Button>
                  </div>
                </div>
              </Card>

              {/* Phone: the count, what it means and the button in one red
                  card, with the bank total and the next return underneath. */}
              <div className="space-y-3 md:hidden">
                <div className="surface-raised space-y-3 rounded-2xl border border-danger/40 bg-danger/[0.06] p-3.5">
                  <div className="flex items-center gap-3">
                    <AlertTriangle className="h-[26px] w-[26px] shrink-0 text-danger" aria-hidden="true" />
                    <div className="min-w-0 flex-1">
                      <p className="font-display text-[34px] font-extrabold leading-none">{data.due_count}</p>
                      <p className="mt-[3px] text-sm leading-snug text-muted-foreground">{t("dueReadyBody")}</p>
                    </div>
                  </div>
                  <Button
                    variant="game"
                    className="min-h-[50px] w-full gap-2 font-display text-lg"
                    onClick={handleStart}
                    disabled={starting}
                  >
                    {starting && <RefreshCw className="h-4 w-4 animate-spin" aria-hidden="true" />}
                    {starting ? t("starting") : t("start")}
                    {!starting && <ChevronRight className="h-5 w-5" strokeWidth={2.5} aria-hidden="true" />}
                  </Button>
                </div>

                <div className="overflow-hidden rounded-2xl border border-border bg-card">
                  <div className="flex min-h-12 items-center gap-3 px-3.5">
                    <BarChart3 className="h-5 w-5 shrink-0 text-muted-foreground" aria-hidden="true" />
                    <span className="min-w-0 flex-1 text-sm text-muted-foreground">{t("totalBank")}</span>
                    <span className="font-display text-lg font-extrabold">{data.total_bank_count}</span>
                  </div>
                  <div aria-hidden="true" className="ml-[46px] h-px bg-border" />
                  <div className="flex min-h-12 items-center gap-3 px-3.5">
                    <Clock className="h-5 w-5 shrink-0 text-muted-foreground" aria-hidden="true" />
                    <span className="min-w-0 flex-1 text-sm text-muted-foreground">{t("nextDueLabel")}</span>
                    <span className="text-sm font-bold">{nextDueRelative}</span>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      )}
    </main>
  );
}

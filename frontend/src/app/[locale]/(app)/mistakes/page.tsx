"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  ArrowLeft,
  BookOpen,
  CalendarClock,
  CheckCircle2,
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

function formatNextDue(value: string | null, locale: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;

  return new Intl.DateTimeFormat(dateLocales[locale] ?? locale, {
    dateStyle: "long",
    timeStyle: "short",
    timeZone: "Asia/Tashkent",
  }).format(date);
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

  const nextDue = formatNextDue(data?.next_due_at ?? null, locale);

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="mb-6">
        <Link
          href={`/${locale}/dashboard`}
          className="mb-2 inline-flex min-h-11 items-center gap-1 text-sm text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("back")}
        </Link>
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
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
          <div className="grid gap-4 sm:grid-cols-2" aria-label={t("title")}>
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
            <Card className="border-amber-500/30 bg-amber-500/5 p-8 text-center">
              <LockKeyhole className="mx-auto mb-3 h-12 w-12 text-gold" aria-hidden="true" />
              <h2 className="font-display text-xl font-bold">{t("vipRequired")}</h2>
              <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">{t("vipBody")}</p>
              <Link className={`${linkButtonClass} mt-6`} href={`/${locale}/premium`}>
                {t("upgrade")}
              </Link>
            </Card>
          ) : data.total_bank_count === 0 ? (
            <Card className="p-8 text-center">
              <BookOpen className="mx-auto mb-3 h-12 w-12 text-accent" aria-hidden="true" />
              <h2 className="font-display text-lg font-bold">{t("emptyBankTitle")}</h2>
              <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">
                {t("emptyBankBody")}
              </p>
              <Link className={`${linkButtonClass} mt-6`} href={`/${locale}/tickets`}>
                {t("browseTickets")}
              </Link>
            </Card>
          ) : data.due_count === 0 ? (
            <Card className="p-8 text-center">
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
            <Card className="p-6">
              <div className="flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-center">
                <div>
                  <h2 className="font-display text-lg font-bold">{t("reviewTitle")}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">{t("fsrsNote")}</p>
                </div>
                <Button variant="game" onClick={handleStart} disabled={starting}>
                  {starting ? (
                    <RefreshCw className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
                  ) : (
                    <Play className="mr-2 h-4 w-4" aria-hidden="true" />
                  )}
                  {starting ? t("starting") : t("start")}
                </Button>
              </div>
            </Card>
          )}
        </div>
      )}
    </main>
  );
}

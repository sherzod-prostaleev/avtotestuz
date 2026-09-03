"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Crown, Check, CheckCircle2, ArrowRight, Info } from "lucide-react";

export default function CheckoutSuccessPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const searchParams = useSearchParams();
  const isFree = searchParams.get("free") === "true";
  const prorated = searchParams.get("prorated") === "1";
  const granted = Number(searchParams.get("granted") || "0");
  const tariff = Number(searchParams.get("tariff") || "0");
  const showProration = prorated && granted > 0 && tariff > 0 && granted < tariff;
  // The design draws a "Tarif / Amal qiladi" summary. Neither value is
  // something this page can derive, so it renders only when the redirect
  // carried them — never a guessed plan or a made-up expiry date.
  const tariffName = searchParams.get("tariff_name");
  const untilParam = searchParams.get("until");
  // Formatted by hand as DD.MM.YYYY, the way the design writes it. Through
  // `toLocaleDateString` the server and the browser disagree — different
  // timezone, different ICU — and React tears the tree down on hydration.
  const untilText = (() => {
    if (!untilParam) return null;
    const date = new Date(untilParam);
    if (Number.isNaN(date.getTime())) return null;
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${pad(date.getUTCDate())}.${pad(date.getUTCMonth() + 1)}.${date.getUTCFullYear()}`;
  })();
  const summaryPlan =
    tariffName && granted > 0
      ? `${tariffName} · ${granted} ${t("daysLabel")}`
      : tariffName;

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center max-md:!pb-0 max-md:p-3">
      {/* Phone: the design's full-height result — an 88px success mark, the
          summary, and the two actions pinned to the bottom. */}
      <div className="mobile-fit-screen flex w-full flex-col gap-3 md:hidden">
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4">
          <span className="flex h-[88px] w-[88px] items-center justify-center rounded-full bg-success/[0.14] text-success">
            <Check aria-hidden="true" className="h-11 w-11" strokeWidth={2.5} />
          </span>
          <div>
            <p className="font-display text-2xl font-extrabold text-success">
              {t("checkoutSuccessTitle")}
            </p>
            <p className="mt-1.5 text-sm leading-snug text-muted-foreground">
              {isFree ? t("checkoutSuccessFreeSubtitle") : t("checkoutSuccessSubtitle")}
            </p>
          </div>

          {(summaryPlan || untilText) && (
            <div className="w-full overflow-hidden rounded-2xl border border-border bg-card">
              {summaryPlan && (
                <div className="flex min-h-11 items-center gap-2.5 px-3.5 text-left">
                  <span className="min-w-0 flex-1 text-sm text-muted-foreground">
                    {t("successTariffLabel")}
                  </span>
                  <span className="shrink-0 font-display text-base font-extrabold">{summaryPlan}</span>
                </div>
              )}
              {summaryPlan && untilText && (
                <div aria-hidden="true" className="ml-3.5 h-px bg-border" />
              )}
              {untilText && (
                <div className="flex min-h-11 items-center gap-2.5 px-3.5 text-left">
                  <span className="min-w-0 flex-1 text-sm text-muted-foreground">
                    {t("successUntilLabel")}
                  </span>
                  <span className="shrink-0 font-display text-base font-extrabold text-gold">
                    {untilText}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <Link
            href={`/${locale}/practice`}
            className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center gap-2 rounded-xl px-4 font-display text-lg font-extrabold"
          >
            {t("checkoutStartPractice")}
          </Link>
          {/* The artboard labels this "To'lovlar tarixi"; the history itself
              lives on the profile page, which is where this goes. */}
          <Link
            href={`/${locale}/profile`}
            className="inline-flex min-h-[46px] w-full items-center justify-center rounded-xl border border-border text-sm font-bold text-muted-foreground"
          >
            {t("paymentsHistory")}
          </Link>
        </div>
      </div>

      <Card className="flex w-full flex-col items-center p-6 max-md:hidden sm:p-8">
        <div className="relative mb-4 flex h-16 w-16 items-center justify-center rounded-full border-2 border-gold bg-gold/10 text-gold">
          <Crown className="h-8 w-8" />
          <CheckCircle2 className="absolute -bottom-1 -right-1 h-6 w-6 rounded-full bg-background text-success" />
        </div>

        <h1 className="font-display text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
          {t("checkoutSuccessTitle")}
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          {isFree ? t("checkoutSuccessFreeSubtitle") : t("checkoutSuccessSubtitle")}
        </p>

        {showProration && (
          <div
            role="status"
            className="mt-4 w-full rounded-xl border border-gold/40 bg-gold/10 p-3 text-left text-sm text-foreground"
          >
            <div className="flex items-start gap-2">
              <Info aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-gold" />
              <div className="space-y-1">
                <p className="font-bold">{t("checkoutProratedTitle")}</p>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t("checkoutProratedBody", { granted, tariff })}
                </p>
              </div>
            </div>
          </div>
        )}

        <div className="mt-6 flex w-full flex-col gap-2">
          <Link href={`/${locale}/practice`} className="w-full">
            <Button as="span" variant="gold" size="lg" className="w-full gap-2">
              {t("checkoutStartPractice")} <ArrowRight className="h-4 w-4" />
            </Button>
          </Link>
          <Link href={`/${locale}/dashboard`} className="w-full">
            <Button as="span" variant="outline" size="lg" className="w-full">
              {t("backHome")}
            </Button>
          </Link>
        </div>
      </Card>
    </main>
  );
}

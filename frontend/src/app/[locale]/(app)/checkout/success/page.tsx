"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Crown, CheckCircle2, ArrowRight, Info } from "lucide-react";

export default function CheckoutSuccessPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const searchParams = useSearchParams();
  const isFree = searchParams.get("free") === "true";
  const prorated = searchParams.get("prorated") === "1";
  const granted = Number(searchParams.get("granted") || "0");
  const tariff = Number(searchParams.get("tariff") || "0");
  const showProration = prorated && granted > 0 && tariff > 0 && granted < tariff;

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="flex w-full flex-col items-center p-6 sm:p-8">
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

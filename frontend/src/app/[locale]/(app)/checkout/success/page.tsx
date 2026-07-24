"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Crown, CheckCircle2, ArrowRight } from "lucide-react";

export default function CheckoutSuccessPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const searchParams = useSearchParams();
  const isFree = searchParams.get("free") === "true";

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="w-full p-6 sm:p-8 flex flex-col items-center">
        <div className="relative mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gold/10 border-2 border-gold text-gold">
          <Crown className="h-8 w-8" />
          <CheckCircle2 className="absolute -bottom-1 -right-1 h-6 w-6 text-success bg-background rounded-full" />
        </div>

        <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl text-foreground">
          {t("checkoutSuccessTitle")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
          {isFree ? t("checkoutSuccessFreeSubtitle") : t("checkoutSuccessSubtitle")}
        </p>

        <div className="mt-6 flex w-full flex-col gap-2">
          <Link href={`/${locale}/practice`} className="w-full">
            <Button type="button" variant="gold" size="lg" className="w-full gap-2">
              {t("checkoutStartPractice")} <ArrowRight className="h-4 w-4" />
            </Button>
          </Link>
          <Link href={`/${locale}/dashboard`} className="w-full">
            <Button type="button" variant="outline" size="lg" className="w-full">
              {t("backHome")}
            </Button>
          </Link>
        </div>
      </Card>
    </main>
  );
}

"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Crown, CheckCircle2, ArrowLeft, Info } from "lucide-react";

export default function PremiumPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();

  const features = [
    t("feature1"),
    t("feature2"),
    t("feature3"),
    t("feature4"),
  ];

  return (
    <main className="mx-auto max-w-3xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
        </Link>
      </header>

      <Card className="p-8 text-center border-gold/40 bg-gradient-to-b from-card to-gold/5 shadow-xl">
        <div className="mx-auto mb-4 flex h-20 w-20 items-center justify-center rounded-full bg-gold/20">
          <Crown className="h-10 w-10 text-gold animate-pulse" />
        </div>

        <h1 className="font-display text-3xl font-extrabold">{t("title")}</h1>
        <p className="mx-auto mt-2 max-w-lg text-sm text-muted-foreground">{t("subtitle")}</p>

        {/* Features Checklist */}
        <div className="mx-auto my-8 max-w-md space-y-3 text-left">
          {features.map((feat, idx) => (
            <div key={idx} className="flex items-start gap-3 rounded-lg border border-border/60 bg-background/50 p-3">
              <CheckCircle2 className="h-5 w-5 shrink-0 text-gold mt-0.5" />
              <span className="text-sm font-medium text-foreground">{feat}</span>
            </div>
          ))}
        </div>

        {/* Honest M1 Notice */}
        <div className="mx-auto mb-8 max-w-md rounded-md border border-amber-500/30 bg-amber-500/10 p-4 text-xs text-amber-500 flex items-start gap-2 text-left">
          <Info className="h-4 w-4 shrink-0 mt-0.5" />
          <span>{t("m1Notice")}</span>
        </div>

        <Link href={`/${locale}/dashboard`}>
          <Button variant="game" size="lg" className="px-8">
            {t("backHome")}
          </Button>
        </Link>
      </Card>
    </main>
  );
}

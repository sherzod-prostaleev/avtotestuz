"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { Card } from "@/components/ui/card";
import { Crown, CheckCircle2, ArrowLeft, Info, Sparkles, ShieldCheck, BookOpenText, TimerReset } from "lucide-react";

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
    <main className="mx-auto max-w-5xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
      </header>

      <section className="relative overflow-hidden rounded-[2rem] border border-gold/30 bg-gradient-to-br from-gold/10 via-card to-card p-6 md:p-8 shadow-2xl">
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-gold/15 blur-3xl" />
        <div className="pointer-events-none absolute -left-20 bottom-0 h-44 w-44 rounded-full bg-accent/10 blur-3xl" />

        <div className="relative grid gap-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-center">
          <div className="space-y-5">
            <div className="inline-flex items-center gap-2 rounded-full border border-gold/40 bg-gold/10 px-3 py-1 text-[11px] font-extrabold text-gold">
              <Crown aria-hidden="true" className="h-3.5 w-3.5" />
              {t("badge")}
            </div>

            <h1 className="font-display text-3xl font-extrabold tracking-tight md:text-5xl">{t("title")}</h1>
            <p className="max-w-xl text-sm leading-6 text-muted-foreground md:text-base">{t("subtitle")}</p>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="flex items-start gap-3 rounded-2xl border border-border/60 bg-background/70 p-4">
                <ShieldCheck aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-gold" />
                <span className="text-sm font-medium text-foreground">{t("trust1")}</span>
              </div>
              <div className="flex items-start gap-3 rounded-2xl border border-border/60 bg-background/70 p-4">
                <Sparkles aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-accent" />
                <span className="text-sm font-medium text-foreground">{t("trust2")}</span>
              </div>
            </div>

            <Link
              href={`/${locale}/dashboard`}
              className="inline-flex h-13 items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent px-8 text-base font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none"
            >
              {t("backHome")}
            </Link>
          </div>

          <Card className="border-gold/30 bg-background/85 p-5 md:p-6">
            <div className="grid gap-3 sm:grid-cols-2">
              {features.map((feat, idx) => (
                <div key={idx} className="flex items-start gap-3 rounded-2xl border border-border/60 bg-card p-4">
                  <CheckCircle2 aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-gold" />
                  <span className="text-sm font-medium text-foreground">{feat}</span>
                </div>
              ))}
            </div>

            <div className="mt-4 flex items-start gap-3 rounded-2xl border border-amber-500/30 bg-amber-500/10 p-4 text-left text-xs text-amber-500">
              <TimerReset aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0" />
              <span>{t("m1Notice")}</span>
            </div>

            <div className="mt-4 rounded-2xl border border-border/60 bg-gradient-to-r from-card to-accent/5 p-4 text-sm text-muted-foreground">
              <div className="mb-2 flex items-center gap-2 font-semibold text-foreground">
                <BookOpenText aria-hidden="true" className="h-4 w-4 text-accent" />
                {t("panelTitle")}
              </div>
              <p>{t("panelBody")}</p>
            </div>
          </Card>
        </div>
      </section>
    </main>
  );
}

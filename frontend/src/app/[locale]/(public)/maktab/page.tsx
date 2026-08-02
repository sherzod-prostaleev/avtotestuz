"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";

const PLANS = [
  { seats: 20, months: [6, 12] },
  { seats: 30, months: [6, 12] },
  { seats: 50, months: [6, 12] },
] as const;

export default function SchoolB2BPage() {
  const t = useTranslations("SchoolB2B");
  const tLanding = useTranslations("Landing");
  const locale = useLocale();

  return (
    <main className="mx-auto max-w-3xl space-y-8 px-4 py-12">
      <header className="space-y-3">
        <p className="text-sm font-semibold text-accent-ink">{tLanding("brandName")}</p>
        <h1 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">{t("title")}</h1>
        <p className="max-w-2xl text-sm text-muted-foreground sm:text-base">{t("subtitle")}</p>
      </header>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{t("howTitle")}</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>{t("how1")}</li>
          <li>{t("how2")}</li>
          <li>{t("how3")}</li>
          <li>{t("how4")}</li>
        </ol>
      </section>

      <section className="grid gap-3 sm:grid-cols-3">
        {PLANS.map((p) => (
          <article key={p.seats} className="rounded-2xl border border-border bg-card p-4">
            <h3 className="font-display text-xl font-extrabold">
              {t("planTitle", { seats: p.seats })}
            </h3>
            <p className="mt-1 text-xs text-muted-foreground">{t("planBody")}</p>
            <p className="mt-3 text-sm font-semibold">{t("planTerms")}</p>
          </article>
        ))}
      </section>

      <section className="space-y-2 rounded-2xl border border-border bg-card p-5">
        <h2 className="text-lg font-bold">{t("rulesTitle")}</h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
          <li>{t("rule1")}</li>
          <li>{t("rule2")}</li>
          <li>{t("rule3")}</li>
        </ul>
      </section>

      <div className="flex flex-wrap gap-3">
        <a href="mailto:support@drivergo.uz?subject=B2B%20School%20Classroom">
          <Button type="button">{t("ctaContact")}</Button>
        </a>
        <Link href={`/${locale}/oferta`}>
          <Button type="button" variant="outline" as="span">
            {t("ctaOferta")}
          </Button>
        </Link>
        <Link href={`/${locale}`}>
          <Button type="button" variant="outline" as="span">
            {t("ctaHome")}
          </Button>
        </Link>
      </div>
    </main>
  );
}

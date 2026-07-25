"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Crown } from "lucide-react";
import { apiGet } from "@/lib/api-client";
import { ThemeToggle } from "@/components/theme-toggle";
import { BrandLogo } from "@/components/brand/brand-logo";
import { Button } from "@/components/ui/button";

type TariffDTO = {
  code: string;
  days: number;
  price_uzs: number;
  old_price_uzs: number | null;
  price_per_day_uzs: number;
  discount_percent: number;
  badge: string | null;
  name: string;
  description: string;
};

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export default function NarxlarPage() {
  const t = useTranslations("Narxlar");
  const tLanding = useTranslations("Landing");
  const tPremium = useTranslations("Premium");
  const locale = useLocale();
  const home = `/${locale}`;

  const [tariffs, setTariffs] = useState<TariffDTO[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const data = await apiGet<TariffDTO[]>(`tariffs?locale=${encodeURIComponent(locale)}`);
      setTariffs(data);
    } catch {
      setTariffs(null);
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [locale]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="sticky top-0 z-40 w-full border-b border-border/70 bg-background/90 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-4xl items-center justify-between px-4">
          <Link
            href={home}
            className="flex items-center gap-2.5 font-display text-xl font-black tracking-tight"
          >
            <BrandLogo size={36} className="h-9 w-9 rounded-full object-cover" />
            <span>{tLanding("brandName")}</span>
          </Link>
          <div className="flex items-center gap-3">
            <ThemeToggle />
            <Link
              href={`/${locale}/login`}
              className="inline-flex h-9 items-center rounded-xl bg-accent px-4 text-xs font-bold text-accent-foreground shadow-3d"
            >
              {tLanding("login")}
            </Link>
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-4xl flex-1 px-4 py-10">
        <Link
          href={home}
          className="mb-6 inline-flex min-h-11 items-center gap-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft aria-hidden="true" className="h-4 w-4" />
          {t("backHome")}
        </Link>

        <div className="space-y-3">
          <p className="inline-flex items-center gap-2 text-xs font-extrabold uppercase tracking-wider text-accent">
            <Crown aria-hidden="true" className="h-4 w-4" />
            {t("badge")}
          </p>
          <h1 className="font-display text-3xl font-black tracking-tight sm:text-4xl">{t("title")}</h1>
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground sm:text-base">
            {t("subtitle")}
          </p>
        </div>

        <ul className="mt-8 grid gap-3 sm:grid-cols-2">
          <li className="rounded-2xl border border-border/80 bg-card/40 px-4 py-3 text-sm text-muted-foreground">
            {t("perk1")}
          </li>
          <li className="rounded-2xl border border-border/80 bg-card/40 px-4 py-3 text-sm text-muted-foreground">
            {t("perk2")}
          </li>
          <li className="rounded-2xl border border-border/80 bg-card/40 px-4 py-3 text-sm text-muted-foreground">
            {t("perk3")}
          </li>
          <li className="rounded-2xl border border-border/80 bg-card/40 px-4 py-3 text-sm text-muted-foreground">
            {t("perk4")}
          </li>
        </ul>

        <div className="mt-10 space-y-4">
          <h2 className="font-display text-xl font-bold">{t("plansTitle")}</h2>
          {loading ? (
            <p className="text-sm text-muted-foreground">{t("loading")}</p>
          ) : error || !tariffs?.length ? (
            <div className="space-y-3 rounded-2xl border border-border bg-card/30 p-5">
              <p className="text-sm text-muted-foreground">{t("loadHint")}</p>
              <Button type="button" variant="outline" onClick={() => void load()}>
                {tPremium("retry")}
              </Button>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {tariffs.map((tariff) => (
                <article
                  key={tariff.code}
                  className="flex flex-col rounded-2xl border border-border bg-card/40 p-5"
                >
                  <h3 className="font-display text-lg font-bold">{tariff.name}</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {tariff.days} {tPremium("daysLabel")}
                  </p>
                  <p className="mt-4 font-display text-2xl font-black">
                    {formatSom(tariff.price_uzs)}{" "}
                    <span className="text-sm font-semibold text-muted-foreground">
                      {tPremium("somSuffix")}
                    </span>
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {tPremium("perDay")} ~{formatSom(tariff.price_per_day_uzs)} {tPremium("somSuffix")}
                  </p>
                  {tariff.description ? (
                    <p className="mt-3 text-sm text-muted-foreground">{tariff.description}</p>
                  ) : null}
                </article>
              ))}
            </div>
          )}
        </div>

        <div className="mt-10 flex flex-wrap gap-3">
          <Link
            href={`/${locale}/login`}
            className="inline-flex min-h-11 items-center rounded-xl bg-accent px-5 text-sm font-bold text-accent-foreground shadow-3d"
          >
            {t("ctaLogin")}
          </Link>
          <Link
            href={`/${locale}/oferta`}
            className="inline-flex min-h-11 items-center rounded-xl border border-border px-5 text-sm font-semibold text-foreground hover:bg-muted/40"
          >
            {t("ctaOferta")}
          </Link>
        </div>
      </main>
    </div>
  );
}

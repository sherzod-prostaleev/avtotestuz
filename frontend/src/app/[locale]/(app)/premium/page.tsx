"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet, apiPost } from "@/lib/api-client";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Crown, CheckCircle2, Sparkles, ShieldCheck } from "lucide-react";
import { ProviderPicker, PaymentProvider } from "@/components/checkout/provider-picker";
import { PromoInput, ValidatePromoResult } from "@/components/checkout/promo-input";

interface TariffDTO {
  code: string;
  days: number;
  price_uzs: number;
  old_price_uzs: number | null;
  price_per_day_uzs: number;
  discount_percent: number;
  badge: string | null;
  name: string;
  description: string;
}

interface EntitlementDTO {
  active: boolean;
  until: string | null;
}

interface CheckoutResult {
  payment_id: string;
  checkout_url: string;
  free?: boolean;
}

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export default function PremiumPage() {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const router = useRouter();

  const [tariffs, setTariffs] = useState<TariffDTO[] | null>(null);
  const [entitlement, setEntitlement] = useState<EntitlementDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [buyingCode, setBuyingCode] = useState<string | null>(null);
  const [buyError, setBuyError] = useState<string | null>(null);

  const [provider, setProvider] = useState<PaymentProvider>("payme");
  const [promoMap, setPromoMap] = useState<Record<string, ValidatePromoResult | null>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const [tariffData, entitlementData] = await Promise.all([
        apiGet<TariffDTO[]>("tariffs"),
        apiGet<EntitlementDTO>("me/entitlement"),
      ]);
      setTariffs(tariffData);
      setEntitlement(entitlementData);
    } catch {
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleBuy = async (code: string) => {
    setBuyError(null);
    setBuyingCode(code);
    const promo = promoMap[code];
    try {
      const result = await apiPost<CheckoutResult>("me/checkout", {
        tariff_code: code,
        provider: provider,
        promo_code: promo?.code || undefined,
      });
      if (result.free) {
        router.push(`/${locale}/checkout/success?free=true`);
      } else if (result.checkout_url) {
        window.location.href = result.checkout_url;
      }
    } catch {
      setBuyError(t("buyError"));
      setBuyingCode(null);
    }
  };

  const badgeLabel = (badge: string | null): string | null => {
    if (badge === "popular") return t("badgePopular");
    if (badge === "best_value") return t("badgeBestValue");
    return null;
  };

  const features = [t("feature1"), t("feature2"), t("feature3"), t("feature4")];

  return (
    <main className="mx-auto max-w-5xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <div className="inline-flex items-center gap-2 rounded-full border border-gold/40 bg-gold/10 px-3 py-1 text-[11px] font-extrabold text-gold">
          <Crown aria-hidden="true" className="h-3.5 w-3.5" />
          {t("badge")}
        </div>
        <h1 className="mt-3 font-display text-3xl font-extrabold tracking-tight md:text-4xl">{t("title")}</h1>
        <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground md:text-base">{t("subtitle")}</p>
      </header>

      {entitlement?.active && entitlement.until && (
        <div role="status" className="mb-6 flex items-center gap-3 rounded-2xl border border-success/40 bg-success/10 p-4 text-sm font-medium text-success">
          <ShieldCheck aria-hidden="true" className="h-5 w-5 shrink-0" />
          {t("vipActiveBanner", { date: new Date(entitlement.until).toLocaleDateString(locale) })}
        </div>
      )}

      {loading && (
        <div role="status" className="py-10 text-center text-sm text-muted-foreground">
          {t("buyLoading")}
        </div>
      )}

      {loadError && (
        <div role="alert" className="mb-6 flex items-center justify-between rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          <span>{t("loadError")}</span>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
            {t("retry")}
          </Button>
        </div>
      )}

      {!loading && !loadError && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Card className="flex flex-col p-5">
            <div className="mb-2 inline-flex w-fit items-center rounded-full border border-border/60 bg-background/70 px-2.5 py-0.5 text-[10px] font-bold text-muted-foreground">
              {t("matizFree")}
            </div>
            <h2 className="font-display text-xl font-bold">{t("matizTitle")}</h2>
            <p className="mt-2 flex-1 text-sm text-muted-foreground">{t("matizDescription")}</p>
            <Button type="button" variant="outline" size="sm" className="mt-4" disabled>
              {t("matizCurrentPlan")}
            </Button>
          </Card>

          {tariffs?.map((tariff) => {
            const label = badgeLabel(tariff.badge);
            const promo = promoMap[tariff.code];
            const finalPrice = promo ? promo.final_amount_uzs : tariff.price_uzs;
            const isFree = promo && finalPrice === 0;

            return (
              <Card
                key={tariff.code}
                className={`flex flex-col p-5 ${tariff.badge === "popular" ? "border-2 border-gold" : ""}`}
              >
                {label && (
                  <div className="mb-2 inline-flex w-fit items-center gap-1 rounded-full border border-gold/40 bg-gold/10 px-2.5 py-0.5 text-[10px] font-extrabold text-gold">
                    <Sparkles aria-hidden="true" className="h-3 w-3" />
                    {label}
                  </div>
                )}
                <h2 className="font-display text-xl font-bold">{tariff.name}</h2>
                <p className="mt-1 text-xs text-muted-foreground">{tariff.description}</p>

                <div className="mt-3">
                  <div className="flex items-baseline gap-1">
                    <span className="font-display text-2xl font-extrabold">{formatSom(isFree ? 0 : Math.round(finalPrice / tariff.days))}</span>
                    <span className="text-xs font-medium text-muted-foreground">
                      {t("somSuffix")} / {t("perDay")}
                    </span>
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                    <span>
                      {formatSom(finalPrice)} {t("somSuffix")} / {tariff.days + (promo?.bonus_days || 0)} {t("daysLabel")}
                    </span>
                    {promo ? (
                      <span className="font-bold text-success">-{formatSom(promo.discount_uzs)} {t("somSuffix")}</span>
                    ) : (
                      tariff.old_price_uzs !== null && (
                        <>
                          <span className="line-through">{formatSom(tariff.old_price_uzs)}</span>
                          <span className="font-bold text-success">-{tariff.discount_percent}%</span>
                        </>
                      )
                    )}
                  </div>
                </div>

                <div className="mt-4 flex-1 space-y-3">
                  <div className="space-y-2">
                    {features.map((feat) => (
                      <div key={feat} className="flex items-start gap-2 text-xs text-foreground">
                        <CheckCircle2 aria-hidden="true" className="mt-0.5 h-3.5 w-3.5 shrink-0 text-gold" />
                        {feat}
                      </div>
                    ))}
                  </div>

                  <div className="border-t border-border/40 pt-3 space-y-3">
                    <PromoInput
                      tariffCode={tariff.code}
                      onApplied={(res) => setPromoMap((prev) => ({ ...prev, [tariff.code]: res }))}
                    />
                    {!isFree && (
                      <ProviderPicker
                        selected={provider}
                        onChange={(p) => setProvider(p)}
                      />
                    )}
                  </div>
                </div>

                {buyError && buyingCode === tariff.code && (
                  <p role="alert" className="mt-2 text-xs text-destructive">
                    {buyError}
                  </p>
                )}

                <Button
                  type="button"
                  variant={isFree ? "success" : "gold"}
                  size="sm"
                  className="mt-4"
                  disabled={buyingCode === tariff.code}
                  onClick={() => void handleBuy(tariff.code)}
                >
                  {buyingCode === tariff.code
                    ? t("buyLoading")
                    : isFree
                    ? t("freeCheckoutButton")
                    : t("buyButton")}
                </Button>
              </Card>
            );
          })}
        </div>
      )}
    </main>
  );
}

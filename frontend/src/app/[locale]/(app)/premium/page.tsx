"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
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
  manual?: {
    payment_id: string;
    amount_uzs: number;
    pan_full: string;
    pan_last4: string;
    holder_name: string;
    network: string;
    hold_until: string;
    manual_state: string;
  };
}

interface ProviderStatusDTO {
  provider: string;
  enabled: boolean;
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

  const [provider, setProvider] = useState<PaymentProvider>("manual");
  const [promoMap, setPromoMap] = useState<Record<string, ValidatePromoResult | null>>({});
  const [referralMap, setReferralMap] = useState<Record<string, string | null>>({});
  const [providerEnabled, setProviderEnabled] = useState<Partial<Record<PaymentProvider, boolean>>>({
    manual: true,
    payme: true,
    click: true,
  });

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const [tariffData, entitlementData, providers] = await Promise.all([
        apiGet<TariffDTO[]>(`tariffs?locale=${encodeURIComponent(locale)}`),
        apiGet<EntitlementDTO>("me/entitlement"),
        apiGet<ProviderStatusDTO[]>("billing/providers").catch(() => [
          { provider: "manual", enabled: true },
          { provider: "payme", enabled: true },
          { provider: "click", enabled: true },
        ]),
      ]);
      setTariffs(tariffData);
      setEntitlement(entitlementData);
      const enabled: Partial<Record<PaymentProvider, boolean>> = {
        manual: true,
        payme: true,
        click: true,
      };
      for (const row of providers) {
        if (row.provider === "payme" || row.provider === "click" || row.provider === "manual") {
          enabled[row.provider] = row.enabled;
        }
      }
      setProviderEnabled(enabled);
      setProvider((prev) => {
        if (enabled[prev] !== false) return prev;
        if (enabled.manual !== false) return "manual";
        if (enabled.payme !== false) return "payme";
        if (enabled.click !== false) return "click";
        return prev;
      });
    } catch {
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [locale]);

  useEffect(() => {
    void load();
  }, [load]);

  const paymentsOffline =
    providerEnabled.payme === false &&
    providerEnabled.click === false &&
    providerEnabled.manual === false;

  const handleBuy = async (code: string) => {
    setBuyError(null);
    setBuyingCode(code);
    const promo = promoMap[code];
    const isFree = Boolean(promo && promo.final_amount_uzs === 0);
    if (!isFree && providerEnabled[provider] === false) {
      setBuyError(t("providerUnavailable"));
      setBuyingCode(null);
      return;
    }
    try {
      const result = await apiPost<CheckoutResult>(
        `me/checkout?locale=${encodeURIComponent(locale)}`,
        {
          tariff_code: code,
          provider: provider,
          promo_code: promo?.code || referralMap[code] || undefined,
        }
      );
      if (result.free) {
        router.push(`/${locale}/checkout/success?free=true`);
      } else if (result.manual?.payment_id) {
        router.push(`/${locale}/checkout/manual?payment_id=${encodeURIComponent(result.manual.payment_id)}`);
      } else if (result.checkout_url) {
        window.location.href = result.checkout_url;
      } else {
        setBuyError(t("buyError"));
        setBuyingCode(null);
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === "provider_unavailable") {
        setBuyError(t("providerUnavailable"));
      } else if (err instanceof ApiError && err.code === "manual_busy") {
        setBuyError(t("manualBusy"));
      } else {
        setBuyError(t("buyError"));
      }
      setBuyingCode(null);
    }
  };

  const badgeLabel = (badge: string | null): string | null => {
    if (badge === "popular") return t("badgePopular");
    if (badge === "best_value") return t("badgeBestValue");
    return null;
  };

  const features = [t("feature1"), t("feature2"), t("feature3"), t("feature4")];

  // Mobile sticky CTA: popular plan first, else first paid tariff (v2 chrome matrix).
  const featuredTariff =
    tariffs?.find((row) => row.badge === "popular") ?? tariffs?.[0] ?? null;
  const featuredPromo = featuredTariff ? promoMap[featuredTariff.code] : null;
  const featuredIsFree = Boolean(featuredPromo && featuredPromo.final_amount_uzs === 0);
  const stickyBuyDisabled =
    !featuredTariff ||
    buyingCode === featuredTariff.code ||
    (!featuredIsFree && providerEnabled[provider] === false);

  return (
    <main className="page-shell-tight">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <div className="inline-flex items-center gap-2 rounded-md border border-gold/40 bg-gold/10 px-3 py-1 text-[11px] font-extrabold text-gold">
          <Crown aria-hidden="true" className="h-3.5 w-3.5" />
          {t("badge")}
        </div>
        <h1 className="mt-3 font-display text-2xl font-extrabold tracking-tight sm:text-3xl md:text-4xl">{t("title")}</h1>
        <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground md:text-base">{t("subtitle")}</p>
      </header>

      {entitlement?.active && entitlement.until && (
        <div role="status" className="mb-6 flex items-center gap-3 rounded-2xl border border-success/40 bg-success/10 p-4 text-sm font-medium text-success">
          <ShieldCheck aria-hidden="true" className="h-5 w-5 shrink-0" />
          {t("vipActiveBanner", { date: new Date(entitlement.until).toLocaleDateString(locale) })}
        </div>
      )}

      {paymentsOffline && !loading && !loadError && (
        <div
          role="status"
          className="mb-6 rounded-2xl border border-border bg-muted/40 p-4 text-sm leading-6 text-foreground"
        >
          <p className="font-display text-base font-bold">{t("paymentsAllOfflineTitle")}</p>
          <p className="mt-1 text-muted-foreground">{t("paymentsAllOfflineBody")}</p>
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
            <div className="mb-2 inline-flex w-fit items-center rounded-md border border-border bg-background px-2.5 py-0.5 text-[10px] font-bold text-muted-foreground">
              {t("matizFree")}
            </div>
            <h2 className="font-display text-xl font-bold">{t("matizTitle")}</h2>
            <p className="mt-2 flex-1 text-sm text-muted-foreground">{t("matizDescription")}</p>
            <Button type="button" variant="outline" size="sm" className="mt-4 w-full" disabled>
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
                  <div className="mb-2 inline-flex w-fit items-center gap-1 rounded-md border border-gold/40 bg-gold/10 px-2.5 py-0.5 text-[10px] font-extrabold text-gold">
                    <Sparkles aria-hidden="true" className="h-3 w-3" />
                    {label}
                  </div>
                )}
                <h2 className="font-display text-xl font-bold">{tariff.name}</h2>
                <p className="mt-1 text-xs text-muted-foreground">{tariff.description}</p>

                <div className="mt-3">
                  <div className="flex items-baseline gap-1">
                    <span className="font-display text-2xl font-extrabold tabular-nums">{formatSom(isFree ? 0 : Math.round(finalPrice / tariff.days))}</span>
                    <span className="text-xs font-medium text-muted-foreground">
                      {t("somSuffix")} / {t("perDay")}
                    </span>
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                    <span className="tabular-nums">
                      {formatSom(finalPrice)} {t("somSuffix")} / {tariff.days + (promo?.bonus_days || 0)} {t("daysLabel")}
                    </span>
                    {promo ? (
                      <span className="font-bold text-success">-{formatSom(promo.discount_uzs)} {t("somSuffix")}</span>
                    ) : (
                      tariff.old_price_uzs !== null && (
                        <>
                          <span className="line-through tabular-nums">{formatSom(tariff.old_price_uzs)}</span>
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

                  <div className="space-y-3 border-t border-border pt-3">
                    <PromoInput
                      tariffCode={tariff.code}
                      onApplied={(res) => setPromoMap((prev) => ({ ...prev, [tariff.code]: res }))}
                      onReferralCode={(ref) =>
                        setReferralMap((prev) => ({ ...prev, [tariff.code]: ref }))
                      }
                    />
                    {!isFree && (
                      <ProviderPicker
                        selected={provider}
                        onChange={(p) => setProvider(p)}
                        enabled={providerEnabled}
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
                  className="mt-4 w-full"
                  disabled={
                    buyingCode === tariff.code ||
                    (!isFree && providerEnabled[provider] === false)
                  }
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

      {!loading && !loadError && featuredTariff && (
        <div className="sticky-cta-bar sm:hidden">
          <Button
            type="button"
            variant={featuredIsFree ? "success" : "gold"}
            size="lg"
            className="w-full"
            disabled={stickyBuyDisabled}
            onClick={() => void handleBuy(featuredTariff.code)}
          >
            {buyingCode === featuredTariff.code
              ? t("buyLoading")
              : featuredIsFree
                ? t("freeCheckoutButton")
                : t("stickyBuy", { name: featuredTariff.name })}
          </Button>
        </div>
      )}
    </main>
  );
}

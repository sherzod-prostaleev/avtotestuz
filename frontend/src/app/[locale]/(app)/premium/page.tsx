"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useVariantCount } from "@/hooks/use-variant-count";
import { useRouter } from "next/navigation";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Crown, CheckCircle2, Sparkles, ShieldCheck } from "lucide-react";
import { ProviderPicker, PaymentProvider } from "@/components/checkout/provider-picker";
import { PremiumMobile } from "@/components/premium/premium-mobile";
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
  const ticketCount = useVariantCount();
  const locale = useLocale();
  const router = useRouter();

  const [tariffs, setTariffs] = useState<TariffDTO[] | null>(null);
  const [entitlement, setEntitlement] = useState<EntitlementDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [buyingCode, setBuyingCode] = useState<string | null>(null);
  const [buyError, setBuyError] = useState<string | null>(null);

  const [provider, setProvider] = useState<PaymentProvider>("manual");
  const [selectedCode, setSelectedCode] = useState<string | null>(null);
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
          { provider: "payme", enabled: false },
          { provider: "click", enabled: false },
        ]),
      ]);
      setTariffs(tariffData);
      setEntitlement(entitlementData);
      const enabled: Partial<Record<PaymentProvider, boolean>> = {
        manual: false,
        payme: false,
        click: false,
      };
      for (const row of providers) {
        if (row.provider === "payme" || row.provider === "click" || row.provider === "manual") {
          enabled[row.provider] = row.enabled;
        }
      }
      setProviderEnabled(enabled);
      setProvider((prev) => {
        if (enabled[prev]) return prev;
        if (enabled.manual) return "manual";
        if (enabled.payme) return "payme";
        if (enabled.click) return "click";
        return prev;
      });
      setSelectedCode((prev) => {
        if (prev && tariffData.some((row) => row.code === prev)) return prev;
        return tariffData.find((row) => row.badge === "popular")?.code ?? tariffData[0]?.code ?? null;
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
    !providerEnabled.payme && !providerEnabled.click && !providerEnabled.manual;

  const selectedTariff = useMemo(
    () => tariffs?.find((row) => row.code === selectedCode) ?? null,
    [tariffs, selectedCode],
  );
  const selectedPromo = selectedTariff ? promoMap[selectedTariff.code] : null;
  const selectedFinal = selectedPromo ? selectedPromo.final_amount_uzs : selectedTariff?.price_uzs ?? 0;
  const selectedIsFree = Boolean(selectedPromo && selectedFinal === 0);
  const selectedDays =
    (selectedTariff?.days ?? 0) + (selectedPromo?.bonus_days ?? 0);

  const handleBuy = async (code: string) => {
    setBuyError(null);
    setBuyingCode(code);
    const promo = promoMap[code];
    const isFree = Boolean(promo && promo.final_amount_uzs === 0);
    if (!isFree && !providerEnabled[provider]) {
      setBuyError(t("providerUnavailable"));
      setBuyingCode(null);
      return;
    }
    try {
      const result = await apiPost<CheckoutResult>(
        `me/checkout?locale=${encodeURIComponent(locale)}`,
        {
          tariff_code: code,
          provider,
          promo_code: promo?.code || referralMap[code] || undefined,
        },
      );
      if (result.free) {
        router.push(`/${locale}/checkout/success?free=true`);
      } else if (result.manual?.payment_id) {
        router.push(
          `/${locale}/checkout/manual?payment_id=${encodeURIComponent(result.manual.payment_id)}`,
        );
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

  const features = [t("feature1", { count: ticketCount }), t("feature2"), t("feature3"), t("feature4")];


  return (
    // `max-md:!pb-0`: the shell's bottom padding stacks on the tab bar's own
    // 4.25rem reserve, and the phone screen needs those pixels to fit.
    <main className="page-shell-tight max-md:!pb-0">
      {/* The phone rebuilds this header inside PremiumMobile — a back
          button and the bare title, without the crown pill or the paragraph. */}
      <header className="mb-6 max-md:hidden">
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <div className="inline-flex items-center gap-2 rounded-md border border-gold/40 bg-gold/10 px-3 py-1 text-[11px] font-extrabold text-gold">
          <Crown aria-hidden="true" className="h-3.5 w-3.5" />
          {t("badge")}
        </div>
        <h1 className="mt-3 font-display text-2xl font-extrabold tracking-tight sm:text-3xl md:text-4xl">
          {t("title")}
        </h1>
        <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground md:text-base">
          {t("subtitle", { count: ticketCount })}
        </p>
      </header>

      {entitlement?.active && entitlement.until && (
        <div
          role="status"
          className="mb-6 flex items-center gap-3 rounded-2xl border border-success/40 bg-success/10 p-4 text-sm font-medium text-success"
        >
          <ShieldCheck aria-hidden="true" className="h-5 w-5 shrink-0" />
          {t("vipActiveBanner", {
            date: new Date(entitlement.until).toLocaleDateString(locale),
          })}
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
        <div
          role="alert"
          className="mb-6 flex items-center justify-between rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive"
        >
          <span>{t("loadError")}</span>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
            {t("retry")}
          </Button>
        </div>
      )}

      {!loading && !loadError && (
        <div className="space-y-6">
          {/* `md:[&+*]:!mt-0`: a `md:hidden` element still counts as a child for
              `space-y-6`, and would otherwise hand the list below it a margin
              the wide layout never had. */}
          <PremiumMobile
            className="md:hidden md:[&+*]:!mt-0"
            tariffs={tariffs ?? []}
            selectedCode={selectedCode}
            onSelect={setSelectedCode}
            promo={selectedPromo}
            onPromoApplied={(res) => {
              if (selectedTariff) {
                setPromoMap((prev) => ({ ...prev, [selectedTariff.code]: res }));
              }
            }}
            onReferralCode={(code) => {
              if (selectedTariff) {
                setReferralMap((prev) => ({ ...prev, [selectedTariff.code]: code }));
              }
            }}
            provider={provider}
            onProviderChange={setProvider}
            providerEnabled={providerEnabled}
            features={features}
            badgeLabel={badgeLabel}
            formatSom={formatSom}
            buying={buyingCode === selectedCode}
            buyError={buyError}
            onBuy={(code) => void handleBuy(code)}
          />
          <ul className="grid gap-2 text-sm text-muted-foreground max-md:hidden sm:grid-cols-2">
            {features.map((feat) => (
              <li key={feat} className="flex items-start gap-2">
                <CheckCircle2
                  aria-hidden="true"
                  className="mt-0.5 h-4 w-4 shrink-0 text-gold"
                />
                <span className="text-foreground/90">{feat}</span>
              </li>
            ))}
          </ul>

          <div className="grid gap-3 max-md:hidden sm:grid-cols-2 xl:grid-cols-4">
            <article className="flex flex-col rounded-2xl border border-border/70 bg-card/40 p-4">
              <span className="w-fit rounded-md bg-muted px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-muted-foreground">
                {t("matizFree")}
              </span>
              <h2 className="mt-3 font-display text-xl font-bold">{t("matizTitle")}</h2>
              <p className="mt-1 flex-1 text-sm text-muted-foreground">{t("matizDescription")}</p>
              <p className="mt-4 font-display text-2xl font-extrabold tabular-nums">0</p>
              <p className="text-xs text-muted-foreground">{t("somSuffix")}</p>
              <Button type="button" variant="outline" size="sm" className="mt-4 w-full" disabled>
                {t("matizCurrentPlan")}
              </Button>
            </article>

            {tariffs?.map((tariff) => {
              const label = badgeLabel(tariff.badge);
              const promo = promoMap[tariff.code];
              const finalPrice = promo ? promo.final_amount_uzs : tariff.price_uzs;
              const days = tariff.days + (promo?.bonus_days || 0);
              const perDay = Math.round(finalPrice / Math.max(days, 1));
              const selected = selectedCode === tariff.code;
              const popular = tariff.badge === "popular";

              return (
                <button
                  key={tariff.code}
                  type="button"
                  onClick={() => setSelectedCode(tariff.code)}
                  className={`flex flex-col rounded-2xl border p-4 text-left transition-all ${
                    selected
                      ? "border-gold bg-gold/5 shadow-[0_0_0_1px_rgba(234,179,8,0.35)]"
                      : popular
                        ? "border-gold/50 bg-card/50 hover:border-gold"
                        : "border-border/70 bg-card/40 hover:border-border"
                  }`}
                >
                  <div className="flex min-h-[1.25rem] items-center justify-between gap-2">
                    {label ? (
                      <span className="inline-flex items-center gap-1 rounded-md border border-gold/40 bg-gold/10 px-2 py-0.5 text-[10px] font-extrabold text-gold">
                        <Sparkles aria-hidden="true" className="h-3 w-3" />
                        {label}
                      </span>
                    ) : (
                      <span />
                    )}
                    {selected ? (
                      <span className="text-[10px] font-bold uppercase tracking-wide text-gold">
                        {t("planSelected")}
                      </span>
                    ) : null}
                  </div>

                  <h2 className="mt-3 font-display text-xl font-bold tracking-tight">
                    {tariff.name}
                  </h2>
                  <p className="mt-0.5 text-xs text-muted-foreground">{tariff.description}</p>

                  <div className="mt-4">
                    <p className="font-display text-3xl font-extrabold tabular-nums tracking-tight">
                      {formatSom(finalPrice)}
                      <span className="ml-1 text-sm font-semibold text-muted-foreground">
                        {t("somSuffix")}
                      </span>
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {days} {t("daysLabel")} · {formatSom(perDay)} {t("somSuffix")}/{t("perDay")}
                    </p>
                    {promo ? (
                      <p className="mt-1 text-xs font-semibold text-success">
                        −{formatSom(promo.discount_uzs)} {t("somSuffix")}
                      </p>
                    ) : tariff.old_price_uzs !== null ? (
                      <p className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                        <span className="line-through tabular-nums">
                          {formatSom(tariff.old_price_uzs)}
                        </span>
                        <span className="font-bold text-success">−{tariff.discount_percent}%</span>
                      </p>
                    ) : null}
                  </div>
                </button>
              );
            })}
          </div>

          {selectedTariff && !paymentsOffline && (
            <section className="rounded-2xl border border-border/80 bg-gradient-to-b from-card/80 to-background p-4 max-md:hidden sm:p-5">
              <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {t("checkoutTitle")}
                  </p>
                  <p className="mt-1 font-display text-lg font-bold">
                    {selectedTariff.name}
                    <span className="ml-2 text-base font-semibold text-muted-foreground">
                      {formatSom(selectedFinal)} {t("somSuffix")}
                      {selectedDays > 0 ? ` · ${selectedDays} ${t("daysLabel")}` : ""}
                    </span>
                  </p>
                </div>
              </div>

              <div className="mt-4 grid gap-4 lg:grid-cols-[1fr_auto] lg:items-end">
                <div className="space-y-3">
                  <PromoInput
                    key={selectedTariff.code}
                    tariffCode={selectedTariff.code}
                    onApplied={(res) =>
                      setPromoMap((prev) => ({ ...prev, [selectedTariff.code]: res }))
                    }
                    onReferralCode={(ref) =>
                      setReferralMap((prev) => ({ ...prev, [selectedTariff.code]: ref }))
                    }
                  />
                  {!selectedIsFree && (
                    <ProviderPicker
                      selected={provider}
                      onChange={setProvider}
                      enabled={providerEnabled}
                    />
                  )}
                </div>

                <div className="space-y-2 lg:w-56">
                  {buyError && (
                    <p role="alert" className="text-xs text-destructive">
                      {buyError}
                    </p>
                  )}
                  {/* Hidden below sm: on phones the sticky bottom bar is the
                      CTA. Without this the card button and the sticky bar
                      both rendered, showing two pay buttons at once. */}
                  <Button
                    type="button"
                    variant={selectedIsFree ? "success" : "gold"}
                    size="lg"
                    className="hidden w-full md:inline-flex"
                    disabled={
                      buyingCode === selectedTariff.code ||
                      (!selectedIsFree && !providerEnabled[provider])
                    }
                    onClick={() => void handleBuy(selectedTariff.code)}
                  >
                    {buyingCode === selectedTariff.code
                      ? t("buyLoading")
                      : selectedIsFree
                        ? t("freeCheckoutButton")
                        : t("buyButton")}
                  </Button>
                </div>
              </div>
            </section>
          )}
        </div>
      )}

    </main>
  );
}

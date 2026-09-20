"use client";

import { useRouter } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { ArrowLeft, Check, Circle, CircleDot, ShieldCheck } from "lucide-react";
import type { PaymentProvider } from "@/components/checkout/provider-picker";

export interface PremiumMobileTariff {
  code: string;
  days: number;
  price_uzs: number;
  price_per_day_uzs: number;
  badge: string | null;
  name: string;
}

export interface PremiumMobileProps {
  tariffs: PremiumMobileTariff[];
  selectedCode: string | null;
  onSelect: (code: string) => void;
  provider: PaymentProvider;
  onProviderChange: (provider: PaymentProvider) => void;
  providerEnabled: Partial<Record<PaymentProvider, boolean>>;
  features: string[];
  badgeLabel: (badge: string | null) => string | null;
  formatSom: (value: number) => string;
  buying: boolean;
  buyError: string | null;
  onBuy: (code: string) => void;
  /** ISO date an active subscription runs to, or null. */
  vipUntil?: string | null;
  /** Every payment method is switched off in the admin panel. */
  paymentsOffline?: boolean;
  className?: string;
}

/**
 * The order and the filtering `provider-picker.tsx` uses, so the phone and the
 * wide layout offer exactly the same methods. A provider switched off in the
 * admin panel disappears from the list rather than sitting there greyed out,
 * and with only one left the choice is not worth a control at all — the parent
 * has already selected it.
 */
const PROVIDER_ORDER: PaymentProvider[] = ["manual", "payme", "click"];

/** Tinted pill per badge, written out in full so Tailwind emits both. */
const BADGE_TONE: Record<string, string> = {
  popular: "bg-success/15 text-success",
  best_value: "bg-gold/15 text-gold",
};

/** The same keys `provider-picker.tsx` uses, so both spell the names alike. */
const PROVIDER_LABEL_KEY: Record<PaymentProvider, string> = {
  manual: "manualName",
  payme: "paymeName",
  click: "clickName",
};

/**
 * Premium on a phone: pick a plan and pay, on one screen.
 *
 * A body of its own rather than a reflow of the wide page, which keeps its
 * card grid and its inline checkout section — that layout is what the kiosk
 * and the desktop render and must not move.
 *
 * Two rules hold this screen together, and both were bought with real money:
 *
 *  1. The buy button is pinned to the bottom of the viewport box and everything
 *     above it scrolls. The column used to just grow, so a short phone — or a
 *     learner whose VIP banner sat above the list — pushed the CTA under the
 *     tab bar on a page that cannot scroll.
 *  2. Buying goes straight to payment. The promo field in between asked for a
 *     code almost nobody has (referrals attach themselves through the invite
 *     link) and cost every buyer an extra screen.
 */
export function PremiumMobile({
  tariffs,
  selectedCode,
  onSelect,
  provider,
  onProviderChange,
  providerEnabled,
  features,
  badgeLabel,
  formatSom,
  buying,
  buyError,
  onBuy,
  vipUntil = null,
  paymentsOffline = false,
  className = "",
}: PremiumMobileProps) {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const router = useRouter();

  const selected = tariffs.find((row) => row.code === selectedCode) ?? null;
  const buyDisabled = !selected || buying || !providerEnabled[provider];
  const availableProviders = PROVIDER_ORDER.filter((id) => providerEnabled[id] !== false);

  const planSubtitle = (tariff: PremiumMobileTariff) =>
    `${tariff.days} ${t("daysLabel")} · ${formatSom(tariff.price_per_day_uzs)} ${t("somSuffix")} ${t("perDay")}`;

  return (
    <div className={`mobile-fit-screen flex flex-col gap-2.5 ${className}`}>
      <div className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          onClick={() => router.back()}
          aria-label={t("backHome")}
          className="-ml-2 flex min-h-touch min-w-11 items-center justify-center text-muted-foreground"
        >
          <ArrowLeft aria-hidden="true" className="h-[22px] w-[22px]" />
        </button>
        <h1 className="min-w-0 flex-1 truncate font-display text-xl font-extrabold">{t("title")}</h1>
      </div>

      {/* The banners live inside the scroller, not above it: anything stacked
          on top of this box on a phone steals height the CTA needs.
          `-mx-1 px-1` keeps the plan cards' shadows from being shaved off. */}
      <div
        data-testid="premium-scroll"
        className="-mx-1 flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto overscroll-contain px-1"
      >
        {vipUntil && (
          <div
            role="status"
            className="flex items-center gap-2.5 rounded-2xl border border-success/40 bg-success/10 px-3 py-2 text-[13px] font-semibold leading-snug text-success-ink"
          >
            <ShieldCheck aria-hidden="true" className="h-[18px] w-[18px] shrink-0" />
            <span className="min-w-0 flex-1">
              {t("vipActiveBanner", { date: new Date(vipUntil).toLocaleDateString(locale) })}
            </span>
          </div>
        )}

        {paymentsOffline && (
          <div role="status" className="rounded-2xl border border-border bg-muted/40 px-3 py-2">
            <p className="font-display text-sm font-bold">{t("paymentsAllOfflineTitle")}</p>
            <p className="mt-0.5 text-xs leading-snug text-muted-foreground">
              {t("paymentsAllOfflineBody")}
            </p>
          </div>
        )}

        {tariffs.length === 0 ? (
          <div aria-hidden="true" className="flex flex-col gap-1.5">
            {[0, 1, 2].map((row) => (
              <span key={row} className="block h-[52px] animate-pulse rounded-xl bg-border/50" />
            ))}
          </div>
        ) : (
          <div role="radiogroup" aria-label={t("planListLabel")} className="flex flex-col gap-1.5">
            {tariffs.map((tariff) => {
              const isSelected = tariff.code === selectedCode;
              const badge = badgeLabel(tariff.badge);
              return (
                <button
                  key={tariff.code}
                  type="button"
                  role="radio"
                  aria-checked={isSelected}
                  onClick={() => onSelect(tariff.code)}
                  className={`flex min-h-touch items-center gap-2.5 rounded-xl border px-3 py-2 text-left ${
                    isSelected ? "border-accent bg-accent/10" : "border-border bg-card"
                  }`}
                >
                  {isSelected ? (
                    <CircleDot aria-hidden="true" className="h-5 w-5 shrink-0 text-accent" />
                  ) : (
                    <Circle aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-[7px]">
                      <span className="truncate font-display text-lg font-extrabold">{tariff.name}</span>
                      {badge && (
                        <span
                          className={`inline-flex h-[22px] shrink-0 items-center rounded-full px-2 text-xs font-extrabold ${
                            BADGE_TONE[tariff.badge ?? ""] ?? "bg-accent/15 text-accent"
                          }`}
                        >
                          {badge}
                        </span>
                      )}
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {planSubtitle(tariff)}
                    </span>
                  </span>
                  <span className="shrink-0 font-display text-lg font-extrabold tabular-nums">
                    {formatSom(tariff.price_uzs)}
                  </span>
                </button>
              );
            })}
          </div>
        )}

        <div className="surface-raised-sm rounded-2xl border border-border bg-card p-3">
          <p className="mb-1.5 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
            {t("panelTitle")}
          </p>
          <ul className="flex flex-col gap-1">
            {features.map((feature) => (
              <li key={feature} className="flex items-start gap-2 text-sm leading-snug">
                <Check aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-success" />
                <span className="min-w-0 flex-1">{feature}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      {/* Only worth a control when there is something to choose: with one
          method left the parent has already selected it. */}
      {availableProviders.length > 1 && (
        <div className="shrink-0">
          <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
            {t("selectProvider")}
          </p>
          <div
            role="radiogroup"
            aria-label={t("selectProvider")}
            className="grid auto-cols-fr grid-flow-col gap-1.5"
          >
            {availableProviders.map((id) => {
              const isSelected = provider === id;
              return (
                <button
                  key={id}
                  type="button"
                  role="radio"
                  aria-checked={isSelected}
                  onClick={() => onProviderChange(id)}
                  className={`flex min-h-touch items-center justify-center gap-1.5 rounded-xl border px-2 text-center ${
                    isSelected ? "border-accent bg-accent/10" : "border-border bg-card"
                  }`}
                >
                  {isSelected && (
                    <CircleDot aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                  )}
                  <span className="min-w-0 truncate text-[13px] font-bold">
                    {t(PROVIDER_LABEL_KEY[id])}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {buyError && (
        <p role="alert" className="shrink-0 text-sm leading-snug text-destructive">
          {buyError}
        </p>
      )}

      {selected && (
        <button
          type="button"
          data-testid="premium-buy"
          disabled={buyDisabled}
          onClick={() => onBuy(selected.code)}
          className="btn-3d-primary inline-flex min-h-[50px] w-full shrink-0 items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {buying ? t("buyLoading") : t("stickyBuy", { name: selected.name })}
        </button>
      )}
    </div>
  );
}

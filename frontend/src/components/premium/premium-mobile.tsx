"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft, Check, Circle, CircleDot } from "lucide-react";
import { PromoInput, type ValidatePromoResult } from "@/components/checkout/promo-input";
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
  promo: ValidatePromoResult | null;
  onPromoApplied: (result: ValidatePromoResult | null) => void;
  onReferralCode: (code: string | null) => void;
  provider: PaymentProvider;
  onProviderChange: (provider: PaymentProvider) => void;
  providerEnabled: Partial<Record<PaymentProvider, boolean>>;
  features: string[];
  badgeLabel: (badge: string | null) => string | null;
  formatSom: (value: number) => string;
  buying: boolean;
  buyError: string | null;
  onBuy: (code: string) => void;
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
 * Premium on a phone: choose a plan on one screen, pay on the next.
 *
 * A body of its own rather than a reflow of the wide page, which keeps its
 * four-up card grid and its inline checkout section — that layout is what the
 * kiosk and the desktop render and must not move.
 *
 * Two screens rather than one long scroll because the design says so, and
 * because the promo field, the totals and the provider tiles are only
 * meaningful once a plan is chosen.
 */
export function PremiumMobile({
  tariffs,
  selectedCode,
  onSelect,
  promo,
  onPromoApplied,
  onReferralCode,
  provider,
  onProviderChange,
  providerEnabled,
  features,
  badgeLabel,
  formatSom,
  buying,
  buyError,
  onBuy,
  className = "",
}: PremiumMobileProps) {
  const t = useTranslations("Premium");
  const router = useRouter();
  const [step, setStep] = useState<"plans" | "buy">("plans");

  const selected = tariffs.find((row) => row.code === selectedCode) ?? null;
  const finalPrice = promo ? promo.final_amount_uzs : (selected?.price_uzs ?? 0);
  const isFree = Boolean(promo && finalPrice === 0);
  const buyDisabled = !selected || buying || (!isFree && !providerEnabled[provider]);
  const availableProviders = PROVIDER_ORDER.filter((id) => providerEnabled[id] !== false);

  const planSubtitle = (tariff: PremiumMobileTariff) =>
    `${tariff.days} ${t("daysLabel")} · ${formatSom(tariff.price_per_day_uzs)} ${t("somSuffix")} ${t("perDay")}`;

  const header = (title: string, onBack: () => void) => (
    <div className="flex items-center gap-1">
      <button
        type="button"
        onClick={onBack}
        aria-label={t("backHome")}
        className="-ml-2 flex min-h-touch min-w-11 items-center justify-center text-muted-foreground"
      >
        <ArrowLeft aria-hidden="true" className="h-[22px] w-[22px]" />
      </button>
      <h1 className="min-w-0 flex-1 truncate font-display text-xl font-extrabold">{title}</h1>
    </div>
  );

  if (step === "buy" && selected) {
    return (
      <div className={`mobile-fit-screen flex flex-col gap-2.5 ${className}`}>
        {header(t("buyButton"), () => setStep("plans"))}

        <div className="surface-raised flex items-center gap-3 rounded-2xl border border-border bg-card px-3 py-2.5">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-bold">{selected.name}</p>
            <p className="truncate text-xs text-muted-foreground">{planSubtitle(selected)}</p>
          </div>
          <span className="shrink-0 font-display text-lg font-extrabold text-gold">
            {formatSom(selected.price_uzs)}
          </span>
        </div>

        <PromoInput
          tariffCode={selected.code}
          onApplied={onPromoApplied}
          onReferralCode={onReferralCode}
        />

        {/* Only once a code has actually been applied — a "0 chegirma" row
            before that would be an invented number. */}
        {promo && (
          <div className="rounded-2xl border border-border bg-card px-3 py-2 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">{t("promoDiscountLabel")}</span>
              <span className="font-bold text-success">−{formatSom(promo.discount_uzs)}</span>
            </div>
            <div className="mt-1 flex items-center justify-between border-t border-border pt-1">
              <span className="text-muted-foreground">{t("promoFinalPrice")}</span>
              <span className="font-display text-base font-extrabold text-gold">
                {formatSom(finalPrice)}
              </span>
            </div>
          </div>
        )}

        {!isFree && availableProviders.length > 1 && (
          <div>
            <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
              {t("selectProvider")}
            </p>
            <div role="radiogroup" aria-label={t("selectProvider")} className="flex flex-col gap-1.5">
              {availableProviders.map((id) => {
                const isSelected = provider === id;
                return (
                  <button
                    key={id}
                    type="button"
                    role="radio"
                    aria-checked={isSelected}
                    onClick={() => onProviderChange(id)}
                    className={`flex min-h-[52px] items-center gap-2.5 rounded-xl border px-3 text-left ${
                      isSelected ? "border-accent bg-accent/10" : "border-border bg-card"
                    }`}
                  >
                    {isSelected ? (
                      <CircleDot aria-hidden="true" className="h-5 w-5 shrink-0 text-accent" />
                    ) : (
                      <Circle aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
                    )}
                    <span className="min-w-0 flex-1 truncate text-sm font-bold">
                      {t(PROVIDER_LABEL_KEY[id])}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>
        )}

        <div className="flex-1" />

        {buyError && (
          <p role="alert" className="text-sm text-destructive">
            {buyError}
          </p>
        )}

        <button
          type="button"
          disabled={buyDisabled}
          onClick={() => onBuy(selected.code)}
          className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {buying
            ? t("buyLoading")
            : isFree
              ? t("freeCheckoutButton")
              : t("stickyBuyAmount", { amount: formatSom(finalPrice) })}
        </button>
      </div>
    );
  }

  return (
    <div className={`mobile-fit-screen flex flex-col gap-2.5 ${className}`}>
      {header(t("title"), () => router.back())}

      {/* Free tier: drawn unfilled and dashed, because it is where you already
          are rather than something to choose. */}
      <div className="flex items-center gap-2.5 rounded-2xl border border-dashed border-border px-3 py-2.5">
        <Circle aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-display text-lg font-extrabold text-muted-foreground">
            {t("matizTitle")}
          </p>
          <p className="truncate text-xs text-muted-foreground">{t("matizDescription")}</p>
        </div>
        <span className="shrink-0 text-xs font-bold text-muted-foreground">
          {t("matizCurrentShort")}
        </span>
      </div>

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

      <div className="flex-1" />

      {selected && (
        <button
          type="button"
          onClick={() => setStep("buy")}
          className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {t("stickyBuy", { name: selected.name })}
        </button>
      )}
    </div>
  );
}

"use client";

import { useTranslations } from "next-intl";

export type PaymentProvider = "payme" | "click" | "manual";

interface ProviderPickerProps {
  selected: PaymentProvider;
  onChange: (provider: PaymentProvider) => void;
  /** Providers that are currently accepting new checkouts. */
  enabled?: Partial<Record<PaymentProvider, boolean>>;
}

export function ProviderPicker({ selected, onChange, enabled }: ProviderPickerProps) {
  const t = useTranslations("Premium");
  const paymeOn = enabled?.payme !== false;
  const clickOn = enabled?.click !== false;
  const manualOn = enabled?.manual !== false;

  const options: { id: PaymentProvider; label: string; on: boolean; accent: string }[] = [
    { id: "manual", label: t("manualName"), on: manualOn, accent: "bg-emerald-600" },
    { id: "payme", label: t("paymeName"), on: paymeOn, accent: "bg-accent" },
    { id: "click", label: t("clickName"), on: clickOn, accent: "bg-gold" },
  ];

  return (
    <div className="space-y-1.5">
      <label className="text-xs font-semibold text-muted-foreground">
        {t("selectProvider")}
      </label>
      <div
        className="grid grid-cols-1 gap-2 sm:grid-cols-3"
        role="radiogroup"
        aria-label={t("selectProvider")}
      >
        {options.map((opt) => (
          <button
            key={opt.id}
            type="button"
            role="radio"
            aria-checked={selected === opt.id}
            aria-disabled={!opt.on}
            disabled={!opt.on}
            onClick={() => opt.on && onChange(opt.id)}
            className={`flex flex-col items-center justify-center gap-1 rounded-xl border p-2.5 text-xs font-bold transition-all ${
              !opt.on
                ? "cursor-not-allowed border-border/40 bg-muted/40 text-muted-foreground opacity-70"
                : selected === opt.id
                  ? "border-accent bg-accent/10 text-accent shadow-sm"
                  : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
            }`}
          >
            <span className="flex items-center gap-2">
              <span className={`h-2 w-2 rounded-full ${opt.accent}`} aria-hidden="true" />
              {opt.label}
            </span>
            {!opt.on && (
              <span className="text-[10px] font-semibold leading-tight text-muted-foreground">
                {t("providerTemporarilyOff")}
              </span>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

"use client";

import { useTranslations } from "next-intl";

export type PaymentProvider = "payme" | "click" | "manual";

interface ProviderPickerProps {
  selected: PaymentProvider;
  onChange: (provider: PaymentProvider) => void;
  /** Providers that are currently accepting new checkouts. Disabled ones are hidden. */
  enabled?: Partial<Record<PaymentProvider, boolean>>;
}

const ORDER: PaymentProvider[] = ["manual", "payme", "click"];

export function ProviderPicker({ selected, onChange, enabled }: ProviderPickerProps) {
  const t = useTranslations("Premium");

  const options = ORDER.filter((id) => enabled?.[id] !== false).map((id) => ({
    id,
    label: id === "manual" ? t("manualName") : id === "payme" ? t("paymeName") : t("clickName"),
    accent: id === "manual" ? "bg-emerald-600" : id === "payme" ? "bg-accent" : "bg-gold",
  }));

  // Single enabled provider — no UI noise; parent already selected it.
  if (options.length <= 1) {
    return null;
  }

  const cols =
    options.length === 2 ? "grid-cols-2" : "grid-cols-1 sm:grid-cols-3";

  return (
    <div className="space-y-1.5">
      <label className="text-xs font-semibold text-muted-foreground">
        {t("selectProvider")}
      </label>
      <div className={`grid gap-2 ${cols}`} role="radiogroup" aria-label={t("selectProvider")}>
        {options.map((opt) => {
          const active = selected === opt.id;
          return (
            <button
              key={opt.id}
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => onChange(opt.id)}
              className={`flex items-center justify-center gap-2 rounded-xl border px-3 py-2.5 text-xs font-bold transition-all ${
                active
                  ? "border-gold bg-gold/10 text-foreground shadow-sm"
                  : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
              }`}
            >
              <span className={`h-2 w-2 rounded-full ${opt.accent}`} aria-hidden="true" />
              {opt.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

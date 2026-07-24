"use client";

import { useTranslations } from "next-intl";

export type PaymentProvider = "payme" | "click";

interface ProviderPickerProps {
  selected: PaymentProvider;
  onChange: (provider: PaymentProvider) => void;
}

export function ProviderPicker({ selected, onChange }: ProviderPickerProps) {
  const t = useTranslations("Premium");

  return (
    <div className="space-y-1.5">
      <label className="text-xs font-semibold text-muted-foreground">
        {t("selectProvider")}
      </label>
      <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label={t("selectProvider")}>
        <button
          type="button"
          role="radio"
          aria-checked={selected === "payme"}
          onClick={() => onChange("payme")}
          className={`flex items-center justify-center gap-2 rounded-xl border p-2.5 text-xs font-bold transition-all ${
            selected === "payme"
              ? "border-accent bg-accent/10 text-accent shadow-sm"
              : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
          }`}
        >
          <span className="h-2 w-2 rounded-full bg-cyan-400" aria-hidden="true" />
          {t("paymeName")}
        </button>

        <button
          type="button"
          role="radio"
          aria-checked={selected === "click"}
          onClick={() => onChange("click")}
          className={`flex items-center justify-center gap-2 rounded-xl border p-2.5 text-xs font-bold transition-all ${
            selected === "click"
              ? "border-accent bg-accent/10 text-accent shadow-sm"
              : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
          }`}
        >
          <span className="h-2 w-2 rounded-full bg-blue-500" aria-hidden="true" />
          {t("clickName")}
        </button>
      </div>
    </div>
  );
}

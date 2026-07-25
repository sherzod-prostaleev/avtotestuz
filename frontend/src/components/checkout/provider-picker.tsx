"use client";

import { useTranslations } from "next-intl";

export type PaymentProvider = "payme" | "click";

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
          aria-disabled={!paymeOn}
          disabled={!paymeOn}
          onClick={() => paymeOn && onChange("payme")}
          className={`flex flex-col items-center justify-center gap-1 rounded-xl border p-2.5 text-xs font-bold transition-all ${
            !paymeOn
              ? "cursor-not-allowed border-border/40 bg-muted/40 text-muted-foreground opacity-70"
              : selected === "payme"
                ? "border-accent bg-accent/10 text-accent shadow-sm"
                : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
          }`}
        >
          <span className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-cyan-400" aria-hidden="true" />
            {t("paymeName")}
          </span>
          {!paymeOn && (
            <span className="text-[10px] font-semibold leading-tight text-muted-foreground">
              {t("providerTemporarilyOff")}
            </span>
          )}
        </button>

        <button
          type="button"
          role="radio"
          aria-checked={selected === "click"}
          aria-disabled={!clickOn}
          disabled={!clickOn}
          onClick={() => clickOn && onChange("click")}
          className={`flex flex-col items-center justify-center gap-1 rounded-xl border p-2.5 text-xs font-bold transition-all ${
            !clickOn
              ? "cursor-not-allowed border-border/40 bg-muted/40 text-muted-foreground opacity-70"
              : selected === "click"
                ? "border-accent bg-accent/10 text-accent shadow-sm"
                : "border-border/60 bg-background/50 text-muted-foreground hover:border-border hover:bg-background"
          }`}
        >
          <span className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-blue-500" aria-hidden="true" />
            {t("clickName")}
          </span>
          {!clickOn && (
            <span className="text-[10px] font-semibold leading-tight text-muted-foreground">
              {t("providerTemporarilyOff")}
            </span>
          )}
        </button>
      </div>
    </div>
  );
}

"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Tag, CheckCircle2, AlertCircle } from "lucide-react";

export interface ValidatePromoResult {
  promo_id: string;
  code: string;
  kind: "percent" | "fixed" | "days";
  value: number;
  discount_uzs: number;
  original_amount_uzs: number;
  final_amount_uzs: number;
  bonus_days: number;
}

interface PromoInputProps {
  tariffCode: string;
  onApplied: (res: ValidatePromoResult | null) => void;
}

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export function PromoInput({ tariffCode, onApplied }: PromoInputProps) {
  const t = useTranslations("Premium");
  const locale = useLocale();
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ValidatePromoResult | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleApply = async () => {
    const trimmed = code.trim();
    if (!trimmed) return;
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await apiPost<ValidatePromoResult>(
        `billing/promo/validate?locale=${encodeURIComponent(locale)}`,
        {
          code: trimmed,
          tariff_code: tariffCode,
        }
      );
      setResult(res);
      onApplied(res);
    } catch (err: unknown) {
      setResult(null);
      onApplied(null);
      const apiErr = err as { code?: string };
      if (apiErr?.code === "promo_expired") {
        setErrorMsg(t("promoErrorExpired"));
      } else if (apiErr?.code === "promo_not_started") {
        setErrorMsg(t("promoErrorNotStarted"));
      } else if (apiErr?.code === "promo_limit_reached") {
        setErrorMsg(t("promoErrorLimitReached"));
      } else if (apiErr?.code === "promo_user_limit_reached") {
        setErrorMsg(t("promoErrorUserLimit"));
      } else {
        setErrorMsg(t("promoErrorNotFound"));
      }
    } finally {
      setLoading(false);
    }
  };

  const handleClear = () => {
    setCode("");
    setResult(null);
    setErrorMsg(null);
    onApplied(null);
  };

  return (
    <div className="space-y-2">
      <label className="text-xs font-semibold text-muted-foreground flex items-center gap-1.5">
        <Tag className="h-3.5 w-3.5" />
        {t("promoTitle")}
      </label>

      {!result ? (
        <div className="flex gap-2">
          <input
            type="text"
            placeholder={t("promoPlaceholder")}
            value={code}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCode(e.target.value.toUpperCase())}
            className="flex h-9 w-full rounded-xl border border-border/60 bg-background/50 px-3 py-1 text-xs font-mono uppercase text-foreground placeholder:text-muted-foreground focus:border-accent focus:outline-none"
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void handleApply()}
            disabled={loading || !code.trim()}
          >
            {loading ? "..." : t("promoApply")}
          </Button>
        </div>
      ) : (
        <div className="flex items-center justify-between rounded-xl border border-success/40 bg-success/10 p-2.5 text-xs">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-success shrink-0" />
            <div>
              <p className="font-bold text-success">{t("promoApplied")} ({result.code})</p>
              <p className="text-[11px] text-muted-foreground">
                {t("promoDiscountLabel")} -{formatSom(result.discount_uzs)} {t("somSuffix")}
                {result.bonus_days > 0 && ` (+${result.bonus_days} ${t("daysLabel")})`}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={handleClear}
            className="text-[11px] font-bold text-muted-foreground hover:text-foreground"
          >
            ✕
          </button>
        </div>
      )}

      {errorMsg && (
        <p role="alert" className="flex items-center gap-1 text-[11px] font-medium text-destructive">
          <AlertCircle className="h-3 w-3 shrink-0" />
          {errorMsg}
        </p>
      )}
    </div>
  );
}

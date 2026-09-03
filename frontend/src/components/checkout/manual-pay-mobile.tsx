"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { AlertTriangle, ArrowLeft, Check, Copy, Timer } from "lucide-react";
import {
  formatPan,
  formatSom,
  useCountdown,
  type ManualPayInfo,
} from "@/components/checkout/manual-pay-card";

export interface ManualPayMobileProps {
  info: ManualPayInfo;
  onClaim: () => void;
  claiming?: boolean;
  claimed?: boolean;
  className?: string;
}

/**
 * The card-transfer screen on a phone: what to send, where to send it, how long
 * the card is held, and one button.
 *
 * A body of its own beside the wide `ManualPayCard`, which keeps its gradient
 * card and stays as it is. The two share `formatSom`, `formatPan` and
 * `useCountdown` rather than each keeping a copy — this is money on screen and
 * a second countdown that drifts from the first would be a real bug.
 */
export function ManualPayMobile({
  info,
  onClaim,
  claiming,
  claimed,
  className = "",
}: ManualPayMobileProps) {
  const t = useTranslations("ManualPay");
  const router = useRouter();
  const left = useCountdown(info.hold_until);
  const mm = String(Math.floor(left / 60)).padStart(2, "0");
  const ss = String(left % 60).padStart(2, "0");
  const [copied, setCopied] = useState<"pan" | "amount" | null>(null);

  async function copy(kind: "pan" | "amount", value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(kind);
      window.setTimeout(() => setCopied(null), 1500);
    } catch {
      // A denied clipboard permission is not worth an error state — the number
      // is on screen and can be typed.
    }
  }

  const copyButton = (kind: "pan" | "amount", value: string, label: string) => (
    <button
      type="button"
      aria-label={label}
      onClick={() => void copy(kind, value)}
      className="flex min-h-touch min-w-11 shrink-0 items-center justify-center rounded-xl border border-border text-muted-foreground"
    >
      {copied === kind ? (
        <Check aria-hidden="true" className="h-5 w-5 text-success" />
      ) : (
        <Copy aria-hidden="true" className="h-5 w-5" />
      )}
    </button>
  );

  return (
    <div className={`mobile-fit-screen flex flex-col gap-[11px] ${className}`}>
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => router.back()}
          aria-label={t("title")}
          className="-ml-2 flex min-h-touch min-w-11 items-center justify-center text-muted-foreground"
        >
          <ArrowLeft aria-hidden="true" className="h-[22px] w-[22px]" />
        </button>
        <h1 className="min-w-0 flex-1 truncate font-display text-xl font-extrabold">{t("title")}</h1>
      </div>

      <div className="flex items-start gap-2.5 rounded-2xl border border-danger/40 bg-danger/[0.06] p-3">
        <AlertTriangle aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-danger" />
        <p className="min-w-0 flex-1 text-[13px] leading-snug">{t("strictAmountNote")}</p>
      </div>

      <div className="surface-raised rounded-2xl border border-border bg-card p-3">
        <p className="text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
          {t("exactAmount")}
        </p>
        <div className="mt-1 flex items-center gap-2">
          <span className="min-w-0 flex-1 font-display text-[30px] font-extrabold leading-none tabular-nums">
            {formatSom(info.amount_uzs)}
          </span>
          {copyButton("amount", String(info.amount_uzs), t("copyAmountLabel"))}
        </div>

        <div className="mt-3 border-t border-border pt-3">
          <p className="text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
            {t("cardLabel")}
          </p>
          <div className="mt-1 flex items-center gap-2">
            <span className="min-w-0 flex-1 font-display text-xl font-extrabold tabular-nums">
              {formatPan(info.pan_full)}
            </span>
            {copyButton("pan", info.pan_full, t("copyPanLabel"))}
          </div>
          <p className="mt-0.5 truncate text-[13px] text-muted-foreground">{info.holder_name}</p>
        </div>
      </div>

      <div
        className={`flex min-h-12 items-center gap-2.5 rounded-2xl border px-3 ${
          left > 0 ? "border-danger/35 bg-danger/[0.06]" : "border-border bg-card"
        }`}
      >
        <Timer
          aria-hidden="true"
          className={`h-5 w-5 shrink-0 ${left > 0 ? "text-danger" : "text-muted-foreground"}`}
        />
        <span className="min-w-0 flex-1 text-sm text-muted-foreground">{t("holdLabelShort")}</span>
        <span className="shrink-0 text-sm font-bold tabular-nums">
          {left > 0 ? `${mm}:${ss}` : t("holdEnded")}
        </span>
      </div>

      <div className="flex-1" />

      {/* The artboard draws the happy path only, but these two are the sole
          signal that a claim was received — dropping them would leave a learner
          pressing the button again. */}
      {info.payment_status === "paid" && (
        <p role="status" className="text-sm font-bold text-success">
          {t("statusPaid")}
        </p>
      )}
      {info.manual_state === "review" && (
        <p role="status" className="text-sm font-bold text-accent">
          {t("statusReview")}
        </p>
      )}

      <button
        type="button"
        onClick={onClaim}
        disabled={claiming || claimed}
        className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {claimed ? t("claimed") : claiming ? t("claiming") : t("claim")}
      </button>

      <p className="text-center text-xs leading-snug text-muted-foreground">
        {t("autoActivateNote")}
      </p>
    </div>
  );
}

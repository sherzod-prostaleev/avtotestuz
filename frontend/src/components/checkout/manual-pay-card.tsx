"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Check, Copy } from "lucide-react";

export type ManualPayInfo = {
  payment_id: string;
  amount_uzs: number;
  pan_full: string;
  pan_last4: string;
  holder_name: string;
  network: string;
  hold_until: string;
  manual_state: string;
  payment_status?: string;
};

function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

function formatPan(pan: string): string {
  const d = pan.replace(/\D/g, "");
  return d.replace(/(.{4})/g, "$1 ").trim();
}

function useCountdown(untilIso: string): number {
  const until = useMemo(() => new Date(untilIso).getTime(), [untilIso]);
  const [left, setLeft] = useState(() => Math.max(0, Math.floor((until - Date.now()) / 1000)));
  useEffect(() => {
    const id = window.setInterval(() => {
      setLeft(Math.max(0, Math.floor((until - Date.now()) / 1000)));
    }, 1000);
    return () => window.clearInterval(id);
  }, [until]);
  return left;
}

export function ManualPayCard({
  info,
  onClaim,
  claiming,
  claimed,
}: {
  info: ManualPayInfo;
  onClaim: () => void;
  claiming?: boolean;
  claimed?: boolean;
}) {
  const t = useTranslations("ManualPay");
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
      /* ignore */
    }
  }

  const paid = info.payment_status === "paid" || info.manual_state === "consumed";
  const review = info.manual_state === "awaiting_review" || info.manual_state === "claimed";

  return (
    <div className="mx-auto w-full max-w-md space-y-4">
      <div
        className="relative overflow-hidden rounded-2xl p-5 text-white shadow-lg"
        style={{
          background:
            "linear-gradient(145deg, #0b3d2e 0%, #146c4f 45%, #1f8f68 100%)",
        }}
      >
        <div className="mb-6 flex items-start justify-between">
          <div>
            <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-white/70">
              HUMO
            </p>
            <p className="mt-1 text-sm font-medium text-white/90">{info.holder_name}</p>
          </div>
          <p className="rounded-md bg-white/15 px-2 py-1 text-[10px] font-bold uppercase tracking-wide">
            {left > 0 ? t("holdActive", { time: `${mm}:${ss}` }) : t("holdEnded")}
          </p>
        </div>
        <p className="font-mono text-xl tracking-widest">{formatPan(info.pan_full)}</p>
        <div className="mt-6 flex items-end justify-between gap-3">
          <div>
            <p className="text-[10px] uppercase tracking-wide text-white/60">{t("exactAmount")}</p>
            <p className="text-2xl font-bold tabular-nums">{formatSom(info.amount_uzs)} so&apos;m</p>
          </div>
          <div className="flex gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="border-white/30 bg-white/15 text-white hover:bg-white/25 hover:text-white"
              onClick={() => void copy("pan", info.pan_full)}
            >
              {copied === "pan" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="border-white/30 bg-white/15 text-white hover:bg-white/25 hover:text-white"
              onClick={() => void copy("amount", String(info.amount_uzs))}
            >
              {copied === "amount" ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              <span className="ml-1 text-[10px]">{t("copyAmount")}</span>
            </Button>
          </div>
        </div>
      </div>

      <p className="text-sm leading-relaxed text-muted-foreground">{t("strictAmountNote")}</p>

      {paid ? (
        <p className="rounded-xl border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm font-semibold text-emerald-700 dark:text-emerald-300">
          {t("statusPaid")}
        </p>
      ) : review ? (
        <p className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-200">
          {t("statusReview")}
        </p>
      ) : null}

      {!paid && (
        <Button type="button" className="w-full" disabled={claiming || claimed} onClick={onClaim}>
          {claiming ? t("claiming") : claimed || review ? t("claimed") : t("claim")}
        </Button>
      )}
    </div>
  );
}

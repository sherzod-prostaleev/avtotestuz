"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { type PaymentItem } from "@/components/profile/payment-history-card";
import { MobileScreen } from "./mobile-screen";

/** "149000" → "149 000", the way every price on the phone is written. */
function groupDigits(value: number): string {
  return String(value).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

/** DD.MM.YYYY, by hand — the server and the browser disagree through Intl. */
function shortDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(date.getUTCDate())}.${pad(date.getUTCMonth() + 1)}.${date.getUTCFullYear()}`;
}

const PROVIDER_LABEL: Record<string, string> = {
  payme: "Payme",
  click: "Click",
  manual: "Humo",
};

/**
 * Payment history on a phone: one row per payment — what it bought on the
 * left, what it cost and how it ended on the right.
 *
 * Fetches the same `me/payments` the wide card does, with its own plain
 * `apiGet`, so opening the panel shows the current list.
 */
export function MobilePayments({ onBack }: { onBack: () => void }) {
  const t = useTranslations("PaymentHistory");
  const [payments, setPayments] = useState<PaymentItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      setPayments(await apiGet<PaymentItem[]>("me/payments?limit=20"));
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const statusTone = (status: string) =>
    status === "paid" ? "text-success" : status === "pending" ? "text-accent" : "text-muted-foreground";

  const statusLabel = (status: string) => {
    if (status === "paid") return t("statusPaid");
    if (status === "pending") return t("statusPending");
    if (status === "canceled") return t("statusCanceled");
    if (status === "refunded") return t("statusRefunded");
    return t("statusFailed");
  };

  return (
    <MobileScreen title={t("title")} onBack={onBack}>
      {loading ? (
        <div aria-hidden="true" className="flex flex-col gap-2">
          {[0, 1, 2].map((row) => (
            <span key={row} className="block h-14 animate-pulse rounded-2xl bg-border/50" />
          ))}
        </div>
      ) : error ? (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          <p>{t("loadError")}</p>
          <button
            type="button"
            onClick={() => void load()}
            className="mt-2 min-h-11 text-sm font-bold underline"
          >
            {t("retry")}
          </button>
        </div>
      ) : payments.length === 0 ? (
        <p className="rounded-2xl border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          {t("empty")}
        </p>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-border bg-card">
          {payments.map((payment, index) => (
            <div
              key={payment.id}
              className={`flex items-center gap-3 px-3.5 py-2.5 ${
                index > 0 ? "border-t border-border" : ""
              }`}
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-bold">
                  {t("planLine", { name: payment.tariff_name, days: payment.tariff_days })}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {shortDate(payment.paid_at ?? payment.created_at)} ·{" "}
                  {PROVIDER_LABEL[payment.provider] ?? payment.provider}
                </p>
              </div>
              <div className="shrink-0 text-right">
                <p className="font-display text-base font-extrabold tabular-nums">
                  {groupDigits(payment.amount_uzs)}
                </p>
                <p className={`text-xs font-bold ${statusTone(payment.status)}`}>
                  {statusLabel(payment.status)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </MobileScreen>
  );
}

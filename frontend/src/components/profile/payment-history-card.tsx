"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { CreditCard, CheckCircle2, Clock, XCircle, AlertCircle, RotateCcw, type LucideIcon } from "lucide-react";
import { formatDateWithTime } from "@/lib/date-format";

const NUMBER_FORMAT_LOCALES: Record<string, string> = {
  "uz-Latn": "uz-Latn-UZ",
  "uz-Cyrl": "uz-Cyrl-UZ",
  ru: "ru-RU",
};

/**
 * Presentation for each value of payment.status (see the CHECK constraint in
 * 0003_billing.up.sql). "refunded" is deliberately not lumped in with the
 * failure styling: a refund means the charge went through and was later
 * reversed, and showing it in red as "payment failed" reads as an error the
 * user should retry.
 */
const STATUS_STYLES: Record<string, { icon: LucideIcon; labelKey: string; className: string }> = {
  paid: {
    icon: CheckCircle2,
    labelKey: "statusPaid",
    className: "bg-success/10 text-success border-success/20",
  },
  created: {
    icon: Clock,
    labelKey: "statusPending",
    className: "bg-accent/10 text-accent border-accent/20",
  },
  pending: {
    icon: Clock,
    labelKey: "statusPending",
    className: "bg-accent/10 text-accent border-accent/20",
  },
  canceled: {
    icon: XCircle,
    labelKey: "statusCanceled",
    className: "bg-muted text-muted-foreground border-border",
  },
  cancelled: {
    icon: XCircle,
    labelKey: "statusCanceled",
    className: "bg-muted text-muted-foreground border-border",
  },
  refunded: {
    icon: RotateCcw,
    labelKey: "statusRefunded",
    className: "bg-muted text-muted-foreground border-border",
  },
};

const FAILED_STATUS_STYLE = {
  icon: AlertCircle,
  labelKey: "statusFailed",
  className: "bg-destructive/10 text-destructive border-destructive/20",
};

export interface PaymentItem {
  id: string;
  amount_uzs: number;
  provider: string;
  status: string;
  created_at: string;
  paid_at: string | null;
  tariff_code: string;
  tariff_days: number;
  tariff_name: string;
}

export function PaymentHistoryCard() {
  const t = useTranslations("PaymentHistory");
  const locale = useLocale();

  const [payments, setPayments] = useState<PaymentItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<boolean>(false);

  const loadPayments = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<PaymentItem[]>("me/payments?limit=20");
      setPayments(res || []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadPayments();
  }, [loadPayments]);

  const formatAmount = (amount: number) => {
    const intlLocale = NUMBER_FORMAT_LOCALES[locale] ?? "uz-UZ";
    return t("amountWithCurrency", { amount: new Intl.NumberFormat(intlLocale).format(amount) });
  };

  const formatDate = (dateStr: string) => formatDateWithTime(dateStr);

  const getStatusBadge = (status: string) => {
    const { icon: Icon, labelKey, className } =
      STATUS_STYLES[status.toLowerCase()] ?? FAILED_STATUS_STYLE;
    return (
      <span
        className={`inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-bold ${className}`}
      >
        <Icon aria-hidden="true" className="h-3 w-3" /> {t(labelKey)}
      </span>
    );
  };

  return (
    <Card className="p-6">
      <CardHeader className="p-0 mb-4 flex flex-row items-center gap-2">
        <CreditCard aria-hidden="true" className="h-5 w-5 text-accent" />
        <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
      </CardHeader>

      <p className="text-xs text-muted-foreground mb-4">{t("subtitle")}</p>

      {loading && (
        <div role="status" className="text-sm text-muted-foreground">
          {t("loading")}
        </div>
      )}

      {error && (
        <div role="alert" className="text-sm text-destructive">
          {t("loadError")}
        </div>
      )}

      {!loading && !error && payments.length === 0 && (
        <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          {t("empty")}
        </div>
      )}

      {!loading && !error && payments.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-border text-muted-foreground font-bold uppercase tracking-wider">
                <th className="py-2.5 px-3">{t("date")}</th>
                <th className="py-2.5 px-3">{t("tariff")}</th>
                <th className="py-2.5 px-3">{t("provider")}</th>
                <th className="py-2.5 px-3 text-right">{t("amount")}</th>
                <th className="py-2.5 px-3 text-center">{t("status")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/50 font-medium">
              {payments.map((item) => (
                <tr key={item.id} className="hover:bg-card/50 transition-colors">
                  <td className="py-3 px-3 text-muted-foreground whitespace-nowrap">
                    {formatDate(item.created_at)}
                  </td>
                  <td className="py-3 px-3 font-bold text-foreground">
                    {item.tariff_name || item.tariff_code} ({t("daysCount", { days: item.tariff_days })})
                  </td>
                  <td className="py-3 px-3 uppercase font-bold text-muted-foreground">
                    {item.provider}
                  </td>
                  <td className="py-3 px-3 text-right font-mono font-bold text-foreground whitespace-nowrap">
                    {formatAmount(item.amount_uzs)}
                  </td>
                  <td className="py-3 px-3 text-center whitespace-nowrap">
                    {getStatusBadge(item.status)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

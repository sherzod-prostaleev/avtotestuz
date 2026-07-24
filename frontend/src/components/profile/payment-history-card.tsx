"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { CreditCard, CheckCircle2, Clock, XCircle, AlertCircle } from "lucide-react";

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

  const [payments, setPayments] = useState<PaymentItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<boolean>(false);

  const loadPayments = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await apiGet<{ items: PaymentItem[] }>("me/payments?limit=20");
      setPayments(res.items || []);
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
    return new Intl.NumberFormat("uz-UZ").format(amount) + " so'm";
  };

  const formatDate = (dateStr: string) => {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString("uz-UZ", {
        day: "numeric",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case "paid":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-success/10 px-2.5 py-0.5 text-xs font-bold text-success border border-success/20">
            <CheckCircle2 aria-hidden="true" className="h-3 w-3" /> {t("statusPaid")}
          </span>
        );
      case "pending":
      case "created":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2.5 py-0.5 text-xs font-bold text-amber-500 border border-amber-500/20">
            <Clock aria-hidden="true" className="h-3 w-3" /> {t("statusPending")}
          </span>
        );
      case "cancelled":
      case "canceled":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2.5 py-0.5 text-xs font-bold text-muted-foreground border border-border">
            <XCircle aria-hidden="true" className="h-3 w-3" /> {t("statusCanceled")}
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-destructive/10 px-2.5 py-0.5 text-xs font-bold text-destructive border border-destructive/20">
            <AlertCircle aria-hidden="true" className="h-3 w-3" /> {t("statusFailed")}
          </span>
        );
    }
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
          To'lovlar tarixi yuklanmoqda...
        </div>
      )}

      {error && (
        <div role="alert" className="text-sm text-destructive">
          To'lovlar tarixini yuklab bo'lmadi.
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
                    {item.tariff_name || item.tariff_code} ({item.tariff_days} kun)
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

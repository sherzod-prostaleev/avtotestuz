"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { PermissionGate } from "@/components/admin/permission-gate";

type PaymentRow = {
  id: string;
  profile_id: string;
  phone_masked: string;
  provider: string;
  status: string;
  amount_uzs: number;
  tariff_code: string;
  created_at: string;
  paid_at?: string;
};

type ListPayload = {
  items: PaymentRow[];
  page: number;
  limit: number;
  total: number;
};

const STATUSES = ["", "created", "pending", "paid", "failed", "canceled", "refunded"];
const PROVIDERS = ["", "payme", "click"];

export default function AdminPaymentsTransactionsPage() {
  const t = useTranslations("AdminPayments");
  const tNav = useTranslations("AdminNav");
  const locale = useLocale();
  const searchParams = useSearchParams();
  const [status, setStatus] = useState(() => searchParams.get("status") ?? "");
  const [provider, setProvider] = useState(() => searchParams.get("provider") ?? "");
  const [from, setFrom] = useState(() => searchParams.get("from") ?? "");
  const [to, setTo] = useState(() => searchParams.get("to") ?? "");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (status) params.set("status", status);
        if (provider) params.set("provider", provider);
        if (from) params.set("from", from);
        if (to) params.set("to", to);
        const res = await fetch(`/api/admin/payments/transactions?${params}`, { cache: "no-store" });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
          setData(null);
          return;
        }
        setData(json.data as ListPayload);
      } catch {
        setError(t("errorLoad"));
        setData(null);
      } finally {
        setLoading(false);
      }
    },
    [status, provider, from, to, t],
  );

  useEffect(() => {
    void load(page);
  }, [page, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  return (
    <PermissionGate permission="payments.read">
      <main className="mx-auto max-w-6xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupPayments")}
          title={t("transactionsTitle")}
          description={t("transactionsSubtitle")}
        />
        <p className="-mt-2 text-xs text-muted-foreground">{t("refundHonest")}</p>

        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            setPage(1);
            void load(1);
          }}
        >
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("filterStatus")}
          >
            {STATUSES.map((s) => (
              <option key={s || "all"} value={s}>
                {s || t("filterAll")}
              </option>
            ))}
          </select>
          <select
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("filterProvider")}
          >
            {PROVIDERS.map((p) => (
              <option key={p || "all"} value={p}>
                {p || t("filterAll")}
              </option>
            ))}
          </select>
          <input
            type="date"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("filterFrom")}
          />
          <input
            type="date"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("filterTo")}
          />
          <Button type="submit" size="sm" disabled={loading}>
            {t("search")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={loading}
            onClick={() => void load(page)}
          >
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {t("refresh")}
          </Button>
        </form>

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load(page)} />
        ) : !data ? (
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        ) : data.items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <>
            <div className="overflow-x-auto rounded-xl border border-border">
              <table className="w-full min-w-[720px] text-left text-sm">
                <thead className="border-b border-border bg-card text-xs uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2 font-bold">{t("colPhone")}</th>
                    <th className="px-3 py-2 font-bold">{t("colProvider")}</th>
                    <th className="px-3 py-2 font-bold">{t("colStatus")}</th>
                    <th className="px-3 py-2 font-bold">{t("colAmount")}</th>
                    <th className="px-3 py-2 font-bold">{t("colTariff")}</th>
                    <th className="px-3 py-2 font-bold">{t("colCreated")}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((row) => (
                    <tr key={row.id} className="border-b border-border/60 last:border-0 hover:bg-card/60">
                      <td className="px-3 py-2 font-mono text-xs">
                        <Link
                          href={`/${locale}/admin/payments/transactions/${row.id}`}
                          className="font-semibold text-accent hover:underline"
                        >
                          {row.phone_masked}
                        </Link>
                      </td>
                      <td className="px-3 py-2">{row.provider}</td>
                      <td className="px-3 py-2">
                        <span className="text-xs font-bold uppercase text-muted-foreground">{row.status}</span>
                      </td>
                      <td className="px-3 py-2 tabular-nums">{row.amount_uzs.toLocaleString(locale)}</td>
                      <td className="px-3 py-2">{row.tariff_code || "—"}</td>
                      <td className="px-3 py-2 text-xs text-muted-foreground">
                        {new Date(row.created_at).toLocaleString(locale)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <p>
                {t("total", { count: data.total })} · {t("pageOf", { page: data.page, pages: totalPages })}
              </p>
              <div className="flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={page <= 1 || loading}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t("prev")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={page >= totalPages || loading}
                  onClick={() => setPage((p) => p + 1)}
                >
                  {t("next")}
                </Button>
              </div>
            </div>
          </>
        )}
      </main>
    </PermissionGate>
  );
}

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { type ColumnDef } from "@tanstack/react-table";
import { CircleSlash2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { PermissionGate } from "@/components/admin/permission-gate";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import { hasPermission } from "@/lib/admin-permissions";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";

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

const STATUSES = ["", "created", "pending", "paid", "failed", "canceled", "refunded", "voided"];
const PROVIDERS = ["", "payme", "click", "manual"];

export default function AdminPaymentsTransactionsPage() {
  const t = useTranslations("AdminPayments");
  const tNav = useTranslations("AdminNav");
  const locale = useLocale();
  const me = useAdminMeOptional();
  const canVoid = hasPermission(me?.permissions, "payments.void");
  const searchParams = useSearchParams();
  const [status, setStatus] = useState(() => searchParams.get("status") ?? "");
  const [provider, setProvider] = useState(() => searchParams.get("provider") ?? "");
  const [from, setFrom] = useState(() => searchParams.get("from") ?? "");
  const [to, setTo] = useState(() => searchParams.get("to") ?? "");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [voidingId, setVoidingId] = useState<string | null>(null);

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

  const voidPayment = useCallback(
    async (row: PaymentRow) => {
      const reason = window.prompt(t("voidReasonPrompt", { phone: row.phone_masked, status: row.status }));
      if (reason === null) {
        return;
      }
      if (reason.trim().length < 10 || reason.trim().length > 500) {
        setError(t("voidReasonRequired"));
        return;
      }
      setVoidingId(row.id);
      setError(null);
      try {
        const res = await fetch(`/api/admin/payments/transactions/${row.id}/void`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ reason: reason.trim() }),
        });
        const json = await res.json().catch(() => ({}));
        if (!res.ok) {
          const code = json?.error?.code as string | undefined;
          setError(
            code === "forbidden"
              ? t("errorForbiddenVoid")
              : code === "not_found"
                ? t("errorNotFound")
                : code === "payment_settled"
                  ? t("errorSettled")
                  : t("errorVoid"),
          );
          return;
        }
        await load(page);
      } catch {
        setError(t("errorVoid"));
      } finally {
        setVoidingId(null);
      }
    },
    [load, page, t],
  );

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const columns = useMemo<ColumnDef<PaymentRow>[]>(
    () => [
      {
        accessorKey: "phone_masked",
        header: t("colPhone"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <Link
            href={`/${locale}/admin/payments/transactions/${row.original.id}`}
            className="inline-flex min-h-[44px] items-center md:min-h-0 font-mono text-xs font-semibold text-accent-ink hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {row.original.phone_masked}
          </Link>
        ),
      },
      {
        accessorKey: "provider",
        header: t("colProvider"),
        cell: ({ row }) => row.original.provider,
      },
      {
        accessorKey: "status",
        header: t("colStatus"),
        cell: ({ row }) => (
          <span className="text-xs font-bold uppercase text-muted-foreground">
            {row.original.status}
          </span>
        ),
      },
      {
        accessorKey: "amount_uzs",
        header: t("colAmount"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => row.original.amount_uzs.toLocaleString(locale),
      },
      {
        accessorKey: "tariff_code",
        header: t("colTariff"),
        cell: ({ row }) => row.original.tariff_code || "—",
      },
      {
        accessorKey: "created_at",
        header: t("colCreated"),
        cell: ({ row }) => (
          <span className="text-xs text-muted-foreground">
            {new Date(row.original.created_at).toLocaleString(locale)}
          </span>
        ),
      },
      ...(canVoid
        ? ([
            {
              id: "actions",
              header: t("colActions"),
              cell: ({ row }) => row.original.status === "failed" ||
                row.original.status === "canceled" ||
                (row.original.provider === "manual" &&
                  (row.original.status === "created" || row.original.status === "pending")) ? (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={voidingId === row.original.id || loading}
                  onClick={() => void voidPayment(row.original)}
                  aria-label={t("void")}
                >
                  <CircleSlash2 aria-hidden className="mr-1 h-3.5 w-3.5" />
                  {voidingId === row.original.id ? t("voiding") : t("void")}
                </Button>
              ) : null,
            },
          ] as ColumnDef<PaymentRow>[])
        : []),
    ],
    [t, locale, canVoid, voidingId, loading, voidPayment],
  );

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
        ) : (
          <>
            <AdminDataTable
              data={data.items}
              columns={columns}
              emptyTitle={t("empty")}
              getRowId={(row) => row.id}
            />
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

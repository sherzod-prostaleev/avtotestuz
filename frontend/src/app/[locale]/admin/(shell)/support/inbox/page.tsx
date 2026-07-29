"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { type ColumnDef } from "@tanstack/react-table";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { PermissionGate } from "@/components/admin/permission-gate";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";

type Ticket = {
  id: string;
  profile_id?: string;
  contact_email: string;
  contact_phone: string;
  subject: string;
  body: string;
  status: string;
  locale: string;
  source: string;
  admin_note: string;
  created_at: string;
  updated_at: string;
};

type ListPayload = {
  items: Ticket[];
  page: number;
  limit: number;
  total: number;
};

const STATUSES = ["", "open", "in_progress", "resolved", "closed"] as const;

export default function AdminSupportInboxPage() {
  const t = useTranslations("AdminInbox");
  const tNav = useTranslations("AdminNav");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (query: string, statusFilter: string, pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        if (statusFilter.trim()) params.set("status", statusFilter.trim());
        const res = await fetch(`/api/admin/support/tickets?${params}`, { cache: "no-store" });
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
    [t],
  );

  useEffect(() => {
    void load(q, status, page);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- search on submit / page change
  }, [page, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const columns = useMemo<ColumnDef<Ticket>[]>(
    () => [
      {
        accessorKey: "subject",
        header: t("colSubject"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <>
            <Link
              href={`/${locale}/admin/support/inbox/${row.original.id}`}
              className="font-semibold text-accent-ink hover:underline"
            >
              {row.original.subject}
            </Link>
            <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">{row.original.body}</p>
          </>
        ),
      },
      {
        accessorKey: "status",
        header: t("colStatus"),
        cell: ({ row }) => (
          <span className="text-xs font-semibold">
            {t(`status_${row.original.status}` as "status_open")}
          </span>
        ),
      },
      {
        accessorKey: "source",
        header: t("colSource"),
        cell: ({ row }) => <span className="text-xs">{row.original.source}</span>,
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
    ],
    [t, locale],
  );

  return (
    <PermissionGate permission="support.inbox">
      <main className="mx-auto max-w-5xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupSupport")}
          title={t("title")}
          description={`${t("subtitle")} ${t("note")}`}
          actions={
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={loading}
              onClick={() => void load(q, status, page)}
            >
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load(q, status, page)} />
        ) : null}

        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            setPage(1);
            void load(q, status, 1);
          }}
        >
          <div className="relative min-w-[12rem] flex-1">
            <Search
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={t("search")}
              className="h-10 w-full rounded-xl border border-border bg-card pl-9 pr-3 text-sm"
            />
          </div>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("status")}
          >
            {STATUSES.map((s) => (
              <option key={s || "all"} value={s}>
                {s ? t(`status_${s}` as "status_open") : t("statusAll")}
              </option>
            ))}
          </select>
          <Button type="submit" size="sm" disabled={loading}>
            {t("searchSubmit")}
          </Button>
        </form>

        {loading && !data ? <AdminSkeleton rows={6} /> : null}

        {data ? (
          <>
            <AdminDataTable
              data={data.items}
              columns={columns}
              emptyTitle={t("empty")}
              getRowId={(row) => row.id}
            />

            {data.total > data.limit ? (
              <div className="flex items-center justify-between gap-2 text-sm">
                <p className="text-muted-foreground">
                  {t("pageInfo", { page: data.page, total: data.total })}
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
            ) : null}
          </>
        ) : null}
      </main>
    </PermissionGate>
  );
}

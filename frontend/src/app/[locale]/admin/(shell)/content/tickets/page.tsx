"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { type ColumnDef } from "@tanstack/react-table";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";

type TicketRow = {
  number: number;
  question_count: number;
  sort_order: number;
};

type ListPayload = {
  items: TicketRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminTicketsPage() {
  const t = useTranslations("AdminContent");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (query: string, pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        const res = await fetch(`/api/admin/content/tickets?${params}`, { cache: "no-store" });
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
    void load(q, page);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- search on submit / page change
  }, [page, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const columns = useMemo<ColumnDef<TicketRow>[]>(
    () => [
      {
        accessorKey: "number",
        header: t("colNumber"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <span className="font-mono text-xs font-semibold tabular-nums">{row.original.number}</span>
        ),
      },
      {
        accessorKey: "question_count",
        header: t("colQuestionCount"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => row.original.question_count,
      },
      {
        accessorKey: "sort_order",
        header: t("colSortOrder"),
        meta: { numeric: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <span className="text-muted-foreground">{row.original.sort_order}</span>
        ),
      },
    ],
    [t],
  );

  return (
    <PermissionGate permission="content.questions.read">
      <main className="mx-auto max-w-6xl space-y-4">
        <AdminPageHeader
          title={t("ticketsTitle")}
          description={t("ticketsSubtitle")}
          actions={
            <Link
              href={`/${locale}/admin/content/questions`}
              className="text-sm font-semibold text-accent hover:underline"
            >
              {t("questionsLink")}
            </Link>
          }
        />

        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            setPage(1);
            void load(q, 1);
          }}
        >
          <div className="relative min-w-0 flex-1 basis-48">
            <Search
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={t("ticketsSearchPlaceholder")}
              className="h-10 w-full rounded-xl border border-border bg-card pl-9 pr-3 text-sm"
            />
          </div>
          <Button type="submit" size="sm" disabled={loading}>
            {t("search")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={loading}
            onClick={() => void load(q, page)}
          >
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {t("refresh")}
          </Button>
        </form>

        {error ? (
          <AdminErrorState message={error} />
        ) : !data ? (
          <AdminSkeleton rows={8} />
        ) : (
          <>
            <AdminDataTable
              data={data.items}
              columns={columns}
              emptyTitle={t("empty")}
              getRowId={(row) => String(row.number)}
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

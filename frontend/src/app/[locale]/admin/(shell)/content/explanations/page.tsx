"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { type ColumnDef } from "@tanstack/react-table";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import {
  AdminDataTable,
  type AdminColumnMeta,
} from "@/components/admin/admin-data-table";

type ExplRow = {
  question_id: string;
  source_ext_id: string;
  text_preview: string;
  locale: string;
  status: string;
  source: string;
  category_code: string;
  explanation_id: string;
  verified_at?: string;
};

type ListPayload = {
  items: ExplRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminExplanationsPage() {
  const t = useTranslations("AdminContent");
  const locale = useLocale();
  const [status, setStatus] = useState("draft");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [busyID, setBusyID] = useState<string | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [bulkMsg, setBulkMsg] = useState<string | null>(null);

  const load = useCallback(
    async (st: string, pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          status: st,
          page: String(pageNum),
          limit: "50",
        });
        const res = await fetch(`/api/admin/content/explanations?${params}`, { cache: "no-store" });
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
    void load(status, page);
  }, [status, page, load]);

  async function bulkVerify() {
    const ids = Object.entries(selected)
      .filter(([, v]) => v)
      .map(([k]) => k);
    if (!ids.length) return;
    setBusyID("bulk");
    setError(null);
    setBulkMsg(null);
    try {
      const res = await fetch("/api/admin/content/explanations/bulk-verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question_ids: ids }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      setBulkMsg(
        t("bulkVerifyResult", {
          verified: json.data.verified,
          skipped: json.data.skipped,
        }),
      );
      setSelected({});
      await load(status, page);
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusyID(null);
    }
  }

  const verify = useCallback(
    async (questionID: string) => {
      setBusyID(questionID);
      setError(null);
      try {
        const res = await fetch(`/api/admin/content/questions/${questionID}/explanation/verify`, {
          method: "POST",
        });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.message ?? t("errorAction"));
          return;
        }
        await load(status, page);
      } catch {
        setError(t("errorAction"));
      } finally {
        setBusyID(null);
      }
    },
    [load, page, status, t],
  );

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const selectedCount = Object.values(selected).filter(Boolean).length;

  const columns = useMemo<ColumnDef<ExplRow>[]>(
    () => [
      {
        id: "select",
        header: " ",
        cell: ({ row }) =>
          row.original.status !== "verified" ? (
            // 44px hit area on a phone, where this is the only way to reach
            // bulk verify; the box itself stays visually small.
            <label className="-my-2 inline-flex min-h-[44px] min-w-[44px] cursor-pointer items-center justify-center md:my-0 md:min-h-0 md:min-w-0">
              <input
                type="checkbox"
                className="h-4 w-4 accent-[hsl(var(--accent))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                checked={Boolean(selected[row.original.question_id])}
                onChange={(e) =>
                  setSelected((s) => ({ ...s, [row.original.question_id]: e.target.checked }))
                }
                aria-label={row.original.source_ext_id}
              />
            </label>
          ) : null,
      },
      {
        accessorKey: "source_ext_id",
        header: t("colExtId"),
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        cell: ({ row }) => (
          <Link
            href={`/${locale}/admin/content/questions/${row.original.question_id}`}
            className="inline-flex min-h-[44px] items-center md:min-h-0 font-mono text-xs font-semibold text-accent-ink hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {row.original.source_ext_id}
          </Link>
        ),
      },
      {
        accessorKey: "text_preview",
        header: t("colText"),
        cell: ({ row }) => (
          <span className="block md:max-w-sm md:truncate">{row.original.text_preview || "—"}</span>
        ),
      },
      {
        accessorKey: "category_code",
        header: t("colCategory"),
        cell: ({ row }) => <span className="text-xs">{row.original.category_code}</span>,
      },
      {
        accessorKey: "status",
        header: t("colExplanation"),
        cell: ({ row }) => (
          <span className="text-xs font-semibold uppercase">{row.original.status}</span>
        ),
      },
      {
        // NOT hideOnCard. Verifying an explanation is ordinary queue work and
        // the operator doing it is often holding a phone; hiding it here left
        // a 13px checkbox as the only way through.
        id: "actions",
        header: t("colActions"),
        cell: ({ row }) =>
          row.original.status !== "verified" ? (
            <Button
              type="button"
              size="sm"
              disabled={busyID === row.original.question_id}
              onClick={() => void verify(row.original.question_id)}
            >
              {t("verifyExplanation")}
            </Button>
          ) : (
            <span className="text-xs text-muted-foreground">—</span>
          ),
      },
    ],
    [t, locale, selected, busyID, verify],
  );

  return (
    <PermissionGate permission="content.questions.read">
    <main className="mx-auto max-w-5xl space-y-4">
      <AdminPageHeader
        title={t("explanationsTitle")}
        description={t("explanationsSubtitle")}
        actions={
          <Link
            href={`/${locale}/admin/content/questions`}
            className="back-link"
          >
            {t("questionsLink")}
          </Link>
        }
      />

      <div className="flex flex-wrap gap-2">
        <select
          value={status}
          onChange={(e) => {
            setPage(1);
            setStatus(e.target.value);
          }}
          className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
          aria-label={t("filterExplanation")}
        >
          <option value="draft">draft</option>
          <option value="pending">pending</option>
          <option value="verified">verified</option>
        </select>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={loading}
          onClick={() => void load(status, page)}
        >
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
        <Button
          type="button"
          size="sm"
          disabled={!selectedCount || busyID === "bulk"}
          onClick={() => void bulkVerify()}
        >
          {t("bulkVerify", { count: selectedCount })}
        </Button>
      </div>

      {bulkMsg ? <p className="text-sm text-success">{bulkMsg}</p> : null}
      {error ? <AdminErrorState message={error} /> : null}
      {!data && !error ? <p className="text-sm text-muted-foreground">{t("loading")}</p> : null}

      {data ? (
        <>
          <AdminDataTable
            data={data.items}
            columns={columns}
            emptyTitle={t("empty")}
            getRowId={(row) => row.explanation_id}
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
      ) : null}
    </main>
    </PermissionGate>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";

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

  async function verify(questionID: string) {
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
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const selectedCount = Object.values(selected).filter(Boolean).length;

  return (
    <PermissionGate permission="content.questions.read">
    <main className="mx-auto max-w-5xl space-y-4">
      <AdminPageHeader
        title={t("explanationsTitle")}
        description={t("explanationsSubtitle")}
        actions={
          <Link
            href={`/${locale}/admin/content/questions`}
            className="text-sm font-semibold text-accent hover:underline"
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

      {bulkMsg ? <p className="text-sm text-emerald-300">{bulkMsg}</p> : null}
      {error ? <AdminErrorState message={error} /> : null}
      {!data && !error ? <p className="text-sm text-muted-foreground">{t("loading")}</p> : null}

      {data && data.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : null}

      {data && data.items.length > 0 ? (
        <>
          <div className="overflow-x-auto rounded-xl border border-border">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-border bg-card text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 font-bold"> </th>
                  <th className="px-3 py-2 font-bold">{t("colExtId")}</th>
                  <th className="px-3 py-2 font-bold">{t("colText")}</th>
                  <th className="px-3 py-2 font-bold">{t("colCategory")}</th>
                  <th className="px-3 py-2 font-bold">{t("colExplanation")}</th>
                  <th className="px-3 py-2 font-bold">{t("colActions")}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((row) => (
                  <tr key={row.explanation_id} className="border-b border-border/60 last:border-0">
                    <td className="px-3 py-2">
                      {row.status !== "verified" ? (
                        <input
                          type="checkbox"
                          checked={Boolean(selected[row.question_id])}
                          onChange={(e) =>
                            setSelected((s) => ({ ...s, [row.question_id]: e.target.checked }))
                          }
                          aria-label={row.source_ext_id}
                        />
                      ) : null}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">
                      <Link
                        href={`/${locale}/admin/content/questions/${row.question_id}`}
                        className="font-semibold text-accent hover:underline"
                      >
                        {row.source_ext_id}
                      </Link>
                    </td>
                    <td className="max-w-sm truncate px-3 py-2">{row.text_preview || "—"}</td>
                    <td className="px-3 py-2 text-xs">{row.category_code}</td>
                    <td className="px-3 py-2 text-xs font-semibold uppercase">{row.status}</td>
                    <td className="px-3 py-2">
                      {row.status !== "verified" ? (
                        <Button
                          type="button"
                          size="sm"
                          disabled={busyID === row.question_id}
                          onClick={() => void verify(row.question_id)}
                        >
                          {t("verifyExplanation")}
                        </Button>
                      ) : (
                        <span className="text-xs text-muted-foreground">—</span>
                      )}
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
      ) : null}
    </main>
    </PermissionGate>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";

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

  return (
    <main className="mx-auto max-w-5xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

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
            className="h-10 w-full rounded-xl border border-border bg-background pl-9 pr-3 text-sm"
          />
        </div>
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-10 rounded-xl border border-border bg-background px-3 text-sm"
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
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={loading}
          onClick={() => void load(q, status, page)}
        >
          <RefreshCw aria-hidden className="mr-1.5 h-3.5 w-3.5" />
          {t("refresh")}
        </Button>
      </form>

      <div className="overflow-x-auto rounded-xl border border-border">
        <table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="border-b border-border bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-bold">{t("colSubject")}</th>
              <th className="px-3 py-2 font-bold">{t("colStatus")}</th>
              <th className="px-3 py-2 font-bold">{t("colSource")}</th>
              <th className="px-3 py-2 font-bold">{t("colCreated")}</th>
            </tr>
          </thead>
          <tbody>
            {loading && !data ? (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-muted-foreground">
                  {t("loading")}
                </td>
              </tr>
            ) : null}
            {data?.items.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-muted-foreground">
                  {t("empty")}
                </td>
              </tr>
            ) : null}
            {data?.items.map((row) => (
              <tr key={row.id} className="border-b border-border last:border-0 hover:bg-muted/30">
                <td className="px-3 py-2">
                  <Link
                    href={`/${locale}/admin/support/inbox/${row.id}`}
                    className="font-semibold text-accent hover:underline"
                  >
                    {row.subject}
                  </Link>
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">{row.body}</p>
                </td>
                <td className="px-3 py-2 text-xs font-semibold">
                  {t(`status_${row.status}` as "status_open")}
                </td>
                <td className="px-3 py-2 text-xs">{row.source}</td>
                <td className="px-3 py-2 text-xs text-muted-foreground">
                  {new Date(row.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {data && data.total > data.limit ? (
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
    </main>
  );
}

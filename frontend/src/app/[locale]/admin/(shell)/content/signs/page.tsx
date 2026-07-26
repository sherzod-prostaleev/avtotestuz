"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";

type SignRow = {
  code: string;
  group_code: string;
  name: string;
  question_count: number;
  has_image: boolean;
};

type ListPayload = {
  items: SignRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminSignsPage() {
  const t = useTranslations("AdminContent");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [group, setGroup] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (query: string, groupCode: string, pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        if (groupCode.trim()) params.set("group", groupCode.trim());
        const res = await fetch(`/api/admin/content/signs?${params}`, { cache: "no-store" });
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
    void load(q, group, page);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- search on submit / page/filter change
  }, [page, group, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  return (
    <PermissionGate permission="content.questions.read">
      <main className="mx-auto max-w-6xl space-y-4">
        <AdminPageHeader
          title={t("signsTitle")}
          description={t("signsSubtitle")}
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
            void load(q, group, 1);
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
              placeholder={t("signsSearchPlaceholder")}
              className="h-10 w-full rounded-xl border border-border bg-card pl-9 pr-3 text-sm"
            />
          </div>
          <input
            value={group}
            onChange={(e) => {
              setPage(1);
              setGroup(e.target.value);
            }}
            placeholder={t("filterGroupPlaceholder")}
            className="h-10 w-36 rounded-xl border border-border bg-card px-3 text-sm"
            aria-label={t("filterGroup")}
          />
          <Button type="submit" size="sm" disabled={loading}>
            {t("search")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={loading}
            onClick={() => void load(q, group, page)}
          >
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {t("refresh")}
          </Button>
        </form>

        {error ? (
          <AdminErrorState message={error} />
        ) : !data ? (
          <AdminSkeleton rows={8} />
        ) : data.items.length === 0 ? (
          <AdminEmptyState title={t("empty")} />
        ) : (
          <>
            <div className="overflow-x-auto rounded-xl border border-border">
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead className="border-b border-border bg-card text-xs uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2 font-bold">{t("colCode")}</th>
                    <th className="px-3 py-2 font-bold">{t("colGroup")}</th>
                    <th className="px-3 py-2 font-bold">{t("colName")}</th>
                    <th className="px-3 py-2 font-bold">{t("colQuestionCount")}</th>
                    <th className="px-3 py-2 font-bold">{t("colImage")}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((row) => (
                    <tr key={row.code} className="border-b border-border/60 last:border-0 hover:bg-card/60">
                      <td className="px-3 py-2 font-mono text-xs font-semibold">{row.code}</td>
                      <td className="px-3 py-2 text-xs">{row.group_code}</td>
                      <td className="max-w-xs truncate px-3 py-2">{row.name || "—"}</td>
                      <td className="px-3 py-2 tabular-nums">{row.question_count}</td>
                      <td className="px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
                        {row.has_image ? t("yes") : t("no")}
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

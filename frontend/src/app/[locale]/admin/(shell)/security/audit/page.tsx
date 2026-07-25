"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";

type AuditRow = {
  id: string;
  admin_user_id?: string;
  admin_email?: string;
  action: string;
  entity_type: string;
  entity_id?: string | null;
  before_json?: unknown;
  after_json?: unknown;
  ip?: string | null;
  ua?: string;
  request_id?: string;
  created_at: string;
};

type ListPayload = {
  items: AuditRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminSecurityAuditPage() {
  const t = useTranslations("AdminAudit");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [action, setAction] = useState("");
  const [entityType, setEntityType] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(
    async (query: string, actionFilter: string, entityFilter: string, pageNum: number) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        if (actionFilter.trim()) params.set("action", actionFilter.trim());
        if (entityFilter.trim()) params.set("entity_type", entityFilter.trim());
        const res = await fetch(`/api/admin/security/audit?${params}`, { cache: "no-store" });
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
    void load(q, action, entityType, page);
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

      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          void load(q, action, entityType, 1);
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
            placeholder={t("searchPlaceholder")}
            className="h-10 w-full rounded-xl border border-border bg-card pl-9 pr-3 text-sm"
          />
        </div>
        <input
          value={action}
          onChange={(e) => setAction(e.target.value)}
          placeholder={t("actionPlaceholder")}
          className="h-10 w-40 rounded-xl border border-border bg-card px-3 text-sm"
        />
        <input
          value={entityType}
          onChange={(e) => setEntityType(e.target.value)}
          placeholder={t("entityPlaceholder")}
          className="h-10 w-36 rounded-xl border border-border bg-card px-3 text-sm"
        />
        <Button type="submit" size="sm" disabled={loading}>
          {t("search")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={loading}
          onClick={() => void load(q, action, entityType, page)}
        >
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
      </form>

      {error ? (
        <p className="text-sm text-destructive">{error}</p>
      ) : !data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : data.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <>
          <p className="text-xs text-muted-foreground">
            {t("total", { count: data.total, page: data.page, pages: totalPages })}
          </p>
          <ul className="space-y-2">
            {data.items.map((row) => {
              const open = expanded === row.id;
              return (
                <li key={row.id} className="rounded-xl border border-border bg-card px-4 py-3">
                  <button
                    type="button"
                    className="w-full text-left"
                    onClick={() => setExpanded(open ? null : row.id)}
                    aria-expanded={open}
                  >
                    <p className="text-sm font-semibold">
                      <span className="font-mono">{row.action}</span>
                      <span className="text-muted-foreground"> · {row.entity_type}</span>
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {new Date(row.created_at).toLocaleString(locale)}
                      {row.admin_email ? ` · ${row.admin_email}` : ""}
                      {row.entity_id ? (
                        <>
                          {" "}
                          · <span className="font-mono">{row.entity_id.slice(0, 12)}</span>
                        </>
                      ) : null}
                      {row.ip ? ` · ${row.ip}` : ""}
                    </p>
                  </button>
                  {open ? (
                    <div className="mt-3 space-y-2 border-t border-border pt-3">
                      {row.request_id ? (
                        <p className="text-[11px] text-muted-foreground">
                          request_id: <span className="font-mono">{row.request_id}</span>
                        </p>
                      ) : null}
                      {row.ua ? (
                        <p className="truncate text-[11px] text-muted-foreground">ua: {row.ua}</p>
                      ) : null}
                      <div className="grid gap-2 sm:grid-cols-2">
                        <div>
                          <p className="mb-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            {t("before")}
                          </p>
                          <pre className="max-h-48 overflow-auto rounded-lg bg-background p-2 font-mono text-[11px]">
                            {row.before_json != null
                              ? JSON.stringify(row.before_json, null, 2)
                              : t("none")}
                          </pre>
                        </div>
                        <div>
                          <p className="mb-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                            {t("after")}
                          </p>
                          <pre className="max-h-48 overflow-auto rounded-lg bg-background p-2 font-mono text-[11px]">
                            {row.after_json != null
                              ? JSON.stringify(row.after_json, null, 2)
                              : t("none")}
                          </pre>
                        </div>
                      </div>
                    </div>
                  ) : null}
                </li>
              );
            })}
          </ul>
          <div className="flex items-center gap-2">
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
        </>
      )}
    </main>
  );
}

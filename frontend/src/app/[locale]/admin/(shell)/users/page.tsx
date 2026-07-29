"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { type ColumnDef } from "@tanstack/react-table";
import { KeyRound, RefreshCw, Search, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminDataTable, type AdminColumnMeta } from "@/components/admin/admin-data-table";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { PermissionGate } from "@/components/admin/permission-gate";

type UserRow = {
  id: string;
  phone: string;
  phone_masked: string;
  name: string;
  locale_pref: string;
  status: string;
  vip_active: boolean;
  has_password: boolean;
  streak: number;
  created_at: string;
  last_seen_at?: string;
  referral_code?: string;
};

type ListPayload = {
  items: UserRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminUsersPage() {
  const t = useTranslations("AdminUsers");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [vip, setVip] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (query: string, pageNum: number, vipF: string, statusF: string) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        if (vipF) params.set("vip", vipF);
        if (statusF) params.set("status", statusF);
        const res = await fetch(`/api/admin/users?${params}`, { cache: "no-store" });
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
    void load(q, page, vip, status);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, vip, status, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  const columns = useMemo<ColumnDef<UserRow>[]>(
    () => [
      {
        accessorKey: "phone",
        meta: { cardTitle: true } satisfies AdminColumnMeta,
        header: t("colPhone"),
        // Unmasked first, `phone_masked` only as a fallback. Support answers
        // users by calling them back, and the phone is also how an operator
        // tells two accounts with the same name apart, so a masked directory
        // would make the page unusable for the job it exists to do.
        cell: ({ row }) => (
          <Link
            href={`/${locale}/admin/users/${row.original.id}`}
            className="inline-flex min-h-[44px] items-center md:min-h-0 font-mono text-[13px] font-semibold tracking-tight text-foreground hover:text-accent-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {row.original.phone || row.original.phone_masked}
          </Link>
        ),
      },
      {
        accessorKey: "name",
        header: t("colName"),
        cell: ({ getValue }) => (getValue<string>() || "—"),
      },
      {
        accessorKey: "status",
        header: t("colStatus"),
        cell: ({ getValue }) => {
          const s = getValue<string>();
          return (
            <span
              className={`inline-flex rounded-md px-1.5 py-0.5 text-[11px] font-extrabold uppercase ${
                s === "blocked" ? "bg-destructive/15 text-danger-ink" : "bg-success/15 text-success-ink"
              }`}
            >
              {s}
            </span>
          );
        },
      },
      {
        accessorKey: "vip_active",
        header: t("colVip"),
        cell: ({ getValue }) =>
          getValue<boolean>() ? (
            <span className="inline-flex items-center gap-1 text-xs font-bold text-accent-ink">
              <Shield aria-hidden className="h-3.5 w-3.5" />
              {t("vipYes")}
            </span>
          ) : (
            <span className="text-xs text-muted-foreground">{t("vipNo")}</span>
          ),
      },
      {
        accessorKey: "has_password",
        header: t("colPassword"),
        cell: ({ getValue }) => (
          <span
            className={`inline-flex items-center gap-1 text-xs ${
              getValue<boolean>() ? "text-foreground" : "text-muted-foreground"
            }`}
            title={t("passwordNote")}
          >
            <KeyRound aria-hidden className="h-3.5 w-3.5" />
            {getValue<boolean>() ? t("passwordSet") : t("passwordUnset")}
          </span>
        ),
      },
      {
        accessorKey: "streak",
        header: t("colStreak"),
        cell: ({ getValue }) => <span className="tabular-nums">{getValue<number>()}</span>,
      },
      {
        accessorKey: "created_at",
        header: t("colCreated"),
        cell: ({ getValue }) => (
          <span className="text-xs text-muted-foreground">
            {new Date(getValue<string>()).toLocaleString(locale)}
          </span>
        ),
      },
    ],
    [locale, t],
  );

  return (
    <PermissionGate permission="users.read">
      <main className="mx-auto max-w-6xl space-y-5">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("title")}
          description={t("subtitle")}
          actions={
            <>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={!data?.items.length}
                onClick={() => {
                  if (!data?.items.length) return;
                  const header = ["id", "phone", "name", "status", "vip", "has_password", "streak", "created_at"];
                  const lines = data.items.map((r) =>
                    [
                      r.id,
                      r.phone,
                      JSON.stringify(r.name || ""),
                      r.status,
                      r.vip_active ? "1" : "0",
                      r.has_password ? "1" : "0",
                      r.streak,
                      r.created_at,
                    ].join(","),
                  );
                  const blob = new Blob([[header.join(","), ...lines].join("\n")], {
                    type: "text/csv;charset=utf-8",
                  });
                  const a = document.createElement("a");
                  a.href = URL.createObjectURL(blob);
                  a.download = `users-page-${data.page}.csv`;
                  a.click();
                  URL.revokeObjectURL(a.href);
                }}
              >
                {t("exportCsv")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading}
                onClick={() => void load(q, page, vip, status)}
              >
                <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
                {t("refresh")}
              </Button>
            </>
          }
        />

        <form
          className="flex flex-wrap items-end gap-2 rounded-2xl border border-border/80 bg-card/80 p-3"
          onSubmit={(e) => {
            e.preventDefault();
            setPage(1);
            void load(q, 1, vip, status);
          }}
        >
          <div className="relative min-w-[220px] flex-1">
            <Search
              aria-hidden
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            />
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={t("searchPlaceholder")}
              className="h-10 w-full rounded-xl border border-border bg-background pl-9 pr-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>
          <label className="space-y-1 text-xs font-semibold text-muted-foreground">
            <span>{t("colVip")}</span>
            <select
              value={vip}
              onChange={(e) => {
                setVip(e.target.value);
                setPage(1);
              }}
              className="block h-10 min-w-[120px] rounded-xl border border-border bg-background px-2 text-sm text-foreground"
            >
              <option value="">{t("filterVipAll")}</option>
              <option value="1">{t("filterVipYes")}</option>
              <option value="0">{t("filterVipNo")}</option>
            </select>
          </label>
          <label className="space-y-1 text-xs font-semibold text-muted-foreground">
            <span>{t("colStatus")}</span>
            <select
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setPage(1);
              }}
              className="block h-10 min-w-[130px] rounded-xl border border-border bg-background px-2 text-sm text-foreground"
            >
              <option value="">{t("filterStatusAll")}</option>
              <option value="active">{t("filterStatusActive")}</option>
              <option value="blocked">{t("filterStatusBlocked")}</option>
            </select>
          </label>
          <Button type="submit" size="sm" disabled={loading}>
            {t("search")}
          </Button>
        </form>

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load(q, page, vip, status)} />
        ) : !data ? (
          <AdminSkeleton rows={8} />
        ) : (
          <>
            <AdminDataTable
              data={data.items}
              columns={columns}
              emptyTitle={t("empty")}
              getRowId={(r) => r.id}
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

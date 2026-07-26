"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowUpRight, RefreshCw } from "lucide-react";
import { ComingSoon } from "@/components/admin/coming-soon";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { MetricTile } from "@/components/admin/metric-tile";
import { AdminBarChart } from "@/components/admin/admin-bar-chart";
import { AdminSkeletonTiles } from "@/components/admin/admin-skeleton";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { PermissionGate } from "@/components/admin/permission-gate";
import { Button } from "@/components/ui/button";

type DayCount = { day: string; count: number };

type Overview = {
  generated_at: string;
  profiles_total: number;
  profiles_created_24h: number;
  entitlements_active: number;
  revenue_paid_uzs_7d: number;
  payments_paid_7d: number;
  events_last_7d: number;
  signups_by_day_14d?: DayCount[];
  revenue_by_day_14d?: DayCount[];
};

function fmtUzs(n: number): string {
  return new Intl.NumberFormat("uz-UZ").format(n) + " UZS";
}

export default function AdminStubOrOverviewPage({
  params,
}: {
  params: { stub?: string[] };
}) {
  const t = useTranslations("AdminOverview");
  const ta = useTranslations("AdminAnalytics");
  const locale = useLocale();
  const parts = params.stub ?? [];
  const [data, setData] = useState<Overview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/analytics/overview", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? ta("errorForbidden") : ta("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as Overview);
    } catch {
      setError(ta("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [ta]);

  useEffect(() => {
    if (parts.length === 0) void load();
  }, [load, parts.length]);

  if (parts.length > 0) {
    return <ComingSoon path={parts.join("/")} phase="M3" />;
  }

  const links = [
    { href: `/${locale}/admin/users`, label: t("linkUsers") },
    { href: `/${locale}/admin/analytics/overview`, label: t("linkAnalytics") },
    { href: `/${locale}/admin/payments/transactions`, label: t("linkPayments") },
    { href: `/${locale}/admin/monitoring/health`, label: t("linkHealth") },
    { href: `/${locale}/admin/support/inbox`, label: t("linkSupport") },
    { href: `/${locale}/admin/security/audit`, label: t("linkAudit") },
  ];

  return (
    <main className="mx-auto max-w-6xl space-y-6">
      <AdminPageHeader
        badge={t("controlBadge")}
        title={t("title")}
        description={t("body")}
        actions={
          <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
            <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
            {t("refresh")}
          </Button>
        }
      />

      <PermissionGate permission="analytics.read" mode="hide">
        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : null}
        {!data && !error ? <AdminSkeletonTiles /> : null}
        {data ? (
          <>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <MetricTile
                label={t("kpiUsers")}
                value={String(data.profiles_total)}
                href={`/${locale}/admin/users`}
                sparkline={(data.signups_by_day_14d ?? []).map((p) => p.count)}
              />
              <MetricTile label={t("kpiNew24h")} value={String(data.profiles_created_24h)} />
              <MetricTile label={t("kpiVip")} value={String(data.entitlements_active)} />
              <MetricTile
                label={t("kpiRevenue7d")}
                value={fmtUzs(data.revenue_paid_uzs_7d)}
                href={`/${locale}/admin/payments/transactions`}
              />
              <MetricTile label={t("kpiPaid7d")} value={String(data.payments_paid_7d)} />
              <MetricTile label={t("kpiEvents7d")} value={String(data.events_last_7d)} />
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
                <h2 className="text-sm font-bold">{t("chartSignups")}</h2>
                <div className="mt-3">
                  <AdminBarChart
                    points={(data.signups_by_day_14d ?? []).map((p) => ({
                      label: p.day,
                      value: p.count,
                    }))}
                    emptyLabel={t("chartEmpty")}
                  />
                </div>
              </section>
              <section className="rounded-2xl border border-border/80 bg-card/70 p-4">
                <h2 className="text-sm font-bold">{t("chartRevenue")}</h2>
                <div className="mt-3">
                  <AdminBarChart
                    points={(data.revenue_by_day_14d ?? []).map((p) => ({
                      label: p.day,
                      value: p.count,
                    }))}
                    emptyLabel={t("chartEmpty")}
                    valueFormatter={fmtUzs}
                  />
                </div>
              </section>
            </div>
          </>
        ) : null}
      </PermissionGate>

      <section className="rounded-2xl border border-border/80 bg-card/50 p-4">
        <h2 className="text-sm font-bold">{t("quickLinks")}</h2>
        <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {links.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className="flex items-center justify-between rounded-xl border border-border/70 px-3 py-2.5 text-sm font-semibold transition hover:border-accent/40 hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {l.label}
              <ArrowUpRight className="h-4 w-4 opacity-60" aria-hidden />
            </Link>
          ))}
        </div>
      </section>
    </main>
  );
}

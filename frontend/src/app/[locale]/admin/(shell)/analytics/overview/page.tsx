"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { AdminBarChart } from "@/components/admin/admin-bar-chart";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminSkeletonTiles } from "@/components/admin/admin-skeleton";
import { MetricTile } from "@/components/admin/metric-tile";
import { PermissionGate } from "@/components/admin/permission-gate";
import { Button } from "@/components/ui/button";

type NameCount = { name: string; count: number };
type DayCount = { day: string; count: number };

type Overview = {
  generated_at: string;
  profiles_total: number;
  profiles_created_24h: number;
  profiles_created_7d: number;
  payments_paid_total: number;
  payments_paid_7d: number;
  revenue_paid_uzs_total: number;
  revenue_paid_uzs_7d: number;
  entitlements_active: number;
  events_last_7d: number;
  top_event_names_7d: NameCount[];
  signups_by_day_14d?: DayCount[];
  revenue_by_day_14d?: DayCount[];
  note: string;
};

function fmtUzs(n: number): string {
  return new Intl.NumberFormat("uz-UZ").format(n) + " UZS";
}

export default function AdminAnalyticsOverviewPage() {
  const t = useTranslations("AdminAnalytics");
  const locale = useLocale();
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
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as Overview);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const refreshActions = (
    <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
      <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
      {t("refresh")}
    </Button>
  );

  return (
    <PermissionGate permission="analytics.read">
      <main className="mx-auto max-w-6xl space-y-5">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("title")}
          description={`${t("subtitle")} ${t("note")}`}
          actions={refreshActions}
        />

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : null}

        {!data && !error ? <AdminSkeletonTiles count={9} /> : null}

        {data ? (
          <>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <MetricTile label={t("tileProfiles")} value={String(data.profiles_total)} />
              <MetricTile label={t("tileProfiles24h")} value={String(data.profiles_created_24h)} />
              <MetricTile label={t("tileProfiles7d")} value={String(data.profiles_created_7d)} />
              <MetricTile label={t("tilePaid")} value={String(data.payments_paid_total)} />
              <MetricTile label={t("tilePaid7d")} value={String(data.payments_paid_7d)} />
              <MetricTile label={t("tileRevenue")} value={fmtUzs(data.revenue_paid_uzs_total)} />
              <MetricTile label={t("tileRevenue7d")} value={fmtUzs(data.revenue_paid_uzs_7d)} />
              <MetricTile label={t("tileEntitlements")} value={String(data.entitlements_active)} />
              <MetricTile label={t("tileEvents7d")} value={String(data.events_last_7d)} />
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

            <section className="space-y-2">
              <h2 className="text-sm font-bold">{t("eventsTitle")}</h2>
              {data.top_event_names_7d.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("eventsEmpty")}</p>
              ) : (
                <ul className="space-y-2">
                  {data.top_event_names_7d.map((row) => {
                    const max = Math.max(1, ...data.top_event_names_7d.map((r) => r.count));
                    const pct = Math.round((row.count / max) * 100);
                    return (
                      <li
                        key={row.name}
                        className="rounded-xl border border-border/70 bg-card/60 px-3 py-2 text-sm"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="font-mono text-xs sm:text-sm">{row.name}</span>
                          <span className="font-bold tabular-nums">{row.count}</span>
                        </div>
                        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-border/60">
                          <div
                            className="h-full rounded-full bg-accent"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </li>
                    );
                  })}
                </ul>
              )}
            </section>

            {data.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
            <p className="text-xs text-muted-foreground">
              {t("generatedAt", { time: new Date(data.generated_at).toLocaleString(locale) })}
            </p>
          </>
        ) : null}
      </main>
    </PermissionGate>
  );
}

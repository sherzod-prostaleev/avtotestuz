"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeletonTiles } from "@/components/admin/admin-skeleton";

type HostMetrics = {
  available?: boolean;
  status?: string;
  cpu_pct?: number | null;
  ram_pct?: number | null;
  disk_pct?: number | null;
  note?: string;
};

type MetricsData = {
  uptime_seconds?: number;
  requests_total?: number;
  requests_by_status_class?: Record<string, number>;
  host?: HostMetrics;
  note?: string;
};

function formatHostPct(value: number | null | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value)) return "—";
  return value.toFixed(2);
}

export default function AdminMonitoringPerfPage() {
  const t = useTranslations("AdminMonitoring");
  const [data, setData] = useState<MetricsData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/monitoring/metrics", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as MetricsData);
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

  const host = data?.host;
  const hostUnavailable = !host?.available;

  return (
    <PermissionGate permission="monitoring.read">
      <main className="mx-auto max-w-lg space-y-4">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("perfTitle")}
          description={`${t("perfSubtitle")} ${t("perfNote")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? <AdminErrorState message={error} retryLabel={t("retry")} onRetry={() => void load()} /> : null}

        {!data && loading ? <AdminSkeletonTiles count={4} /> : null}

        {data ? (
          <>
            <section className="rounded-xl border border-border bg-card px-4 py-3">
              <p className="font-bold">{t("hostMetricsTitle")}</p>
              {hostUnavailable ? (
                <>
                  <p className="mt-2 text-sm font-semibold text-destructive">{t("hostMetricsUnavailable")}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{t("hostMetricsNote")}</p>
                  <dl className="mt-3 grid grid-cols-3 gap-2 text-xs">
                    {[t("hostMetricsCpu"), t("hostMetricsRam"), t("hostMetricsDisk")].map((label) => (
                      <div
                        key={label}
                        className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2"
                      >
                        <dt className="font-semibold text-muted-foreground">{label}</dt>
                        <dd className="mt-0.5 font-mono font-bold text-destructive">—</dd>
                      </div>
                    ))}
                  </dl>
                </>
              ) : (
                <dl className="mt-3 grid grid-cols-3 gap-2 text-xs">
                  <div className="rounded-lg border border-border/70 bg-background px-3 py-2">
                    <dt className="font-semibold text-muted-foreground">{t("hostMetricsCpu")}</dt>
                    <dd className="mt-0.5 font-mono font-bold">{formatHostPct(host?.cpu_pct)}%</dd>
                  </div>
                  <div className="rounded-lg border border-border/70 bg-background px-3 py-2">
                    <dt className="font-semibold text-muted-foreground">{t("hostMetricsRam")}</dt>
                    <dd className="mt-0.5 font-mono font-bold">{formatHostPct(host?.ram_pct)}%</dd>
                  </div>
                  <div className="rounded-lg border border-border/70 bg-background px-3 py-2">
                    <dt className="font-semibold text-muted-foreground">{t("hostMetricsDisk")}</dt>
                    <dd className="mt-0.5 font-mono font-bold">{formatHostPct(host?.disk_pct)}%</dd>
                  </div>
                </dl>
              )}
            </section>

            <dl className="grid grid-cols-2 gap-2 text-xs">
              <div className="rounded-xl border border-border bg-card px-4 py-3">
                <dt className="font-semibold text-muted-foreground">{t("metricsUptime")}</dt>
                <dd className="mt-1 font-mono text-lg font-bold">{data.uptime_seconds ?? "—"}s</dd>
              </div>
              <div className="rounded-xl border border-border bg-card px-4 py-3">
                <dt className="font-semibold text-muted-foreground">{t("metricsTotal")}</dt>
                <dd className="mt-1 font-mono text-lg font-bold">{data.requests_total ?? "—"}</dd>
              </div>
              {data.requests_by_status_class &&
                Object.entries(data.requests_by_status_class).map(([name, value]) => (
                  <div key={name} className="rounded-xl border border-border bg-card px-4 py-3">
                    <dt className="font-semibold text-muted-foreground">{name}</dt>
                    <dd className="mt-1 font-mono text-lg font-bold">{value}</dd>
                  </div>
                ))}
            </dl>
          </>
        ) : null}

        {data?.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
      </main>
    </PermissionGate>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type MetricsData = {
  uptime_seconds?: number;
  requests_total?: number;
  requests_by_status_class?: Record<string, number>;
  note?: string;
};

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

  return (
    <main className="mx-auto max-w-lg space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("perfTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("perfSubtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("perfNote")}</p>
      </header>

      <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
        <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        {t("refresh")}
      </Button>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
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
      )}
      {data?.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
    </main>
  );
}

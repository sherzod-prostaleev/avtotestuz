"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";

type HealthData = {
  status: string;
  live: string;
  checks: Record<string, string>;
  checked_at: string;
  alerts?: {
    id: string;
    name: string;
    kind: string;
    enabled: boolean;
    description: string;
    status: string;
    detail?: string;
  }[];
};

export default function AdminMonitoringHealthPage() {
  const t = useTranslations("AdminMonitoring");
  const [data, setData] = useState<HealthData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);
  const [streamMode, setStreamMode] = useState<"sse" | "poll" | null>(null);

  const applyHealth = useCallback(
    (snap: HealthData) => {
      setData(snap);
      setError(snap.status === "ok" ? null : t("errorDegraded"));
    },
    [t],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/monitoring/health", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok && !json?.data) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      applyHealth(json.data as HealthData);
    } catch {
      setError(t("errorLoad"));
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [applyHealth, t]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!auto) {
      setStreamMode(null);
      return;
    }

    const es = new EventSource("/api/admin/monitoring/stream");
    es.addEventListener("health", (ev) => {
      try {
        applyHealth(JSON.parse(ev.data) as HealthData);
        setStreamMode("sse");
        setLoading(false);
      } catch {
        /* ignore malformed event */
      }
    });
    es.onerror = () => {
      es.close();
      setStreamMode("poll");
    };
    return () => es.close();
  }, [auto, applyHealth]);

  useEffect(() => {
    if (!auto || streamMode !== "poll") return;
    const id = window.setInterval(() => void load(), 10_000);
    return () => window.clearInterval(id);
  }, [auto, streamMode, load]);

  const refreshActions = (
    <>
      <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
        <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        {t("refresh")}
      </Button>
      <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
        <input
          type="checkbox"
          checked={auto}
          onChange={(e) => setAuto(e.target.checked)}
          className="rounded border-border"
        />
        {t("autoRefresh")}
      </label>
    </>
  );

  return (
    <PermissionGate permission="monitoring.read">
      <main className="mx-auto max-w-lg space-y-4">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("healthTitle")}
          description={t("healthSubtitle")}
          actions={refreshActions}
        />

        {auto && streamMode ? (
          <p className="text-xs text-muted-foreground">
            {streamMode === "sse" ? t("streamLive") : t("streamPolling")}
          </p>
        ) : null}

        {error ? <AdminErrorState message={error} retryLabel={t("retry")} onRetry={() => void load()} /> : null}

        {!data && loading ? <AdminSkeleton rows={3} /> : null}

        {data ? (
          <ul className="space-y-3">
            <li className="rounded-xl border border-border bg-card px-4 py-3">
              <div className="flex items-center justify-between">
                <p className="font-bold">{t("liveTitle")}</p>
                <span className="text-xs font-bold text-accent">{data.live}</span>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">{t("liveHint")}</p>
            </li>
            <li className="rounded-xl border border-border bg-card px-4 py-3">
              <div className="flex items-center justify-between">
                <p className="font-bold">{t("readyTitle")}</p>
                <span
                  className={`text-xs font-bold ${
                    data.status === "ok" ? "text-accent" : "text-destructive"
                  }`}
                >
                  {data.status}
                </span>
              </div>
              <dl className="mt-3 grid grid-cols-2 gap-2 text-xs">
                {Object.entries(data.checks ?? {}).map(([name, value]) => (
                  <div key={name} className="rounded-lg border border-border/70 bg-background px-3 py-2">
                    <dt className="font-semibold text-muted-foreground">{name}</dt>
                    <dd
                      className={`mt-0.5 font-mono font-bold ${
                        value === "ok" || value === "skipped" ? "text-foreground" : "text-destructive"
                      }`}
                    >
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
              <p className="mt-2 text-xs text-muted-foreground">
                {t("lastChecked", { time: new Date(data.checked_at).toLocaleTimeString() })}
              </p>
            </li>
            {(data.alerts ?? []).length > 0 ? (
              <li className="rounded-xl border border-border bg-card px-4 py-3">
                <p className="font-bold">{t("alertsOnHealth")}</p>
                <ul className="mt-2 space-y-2">
                  {data.alerts!.map((a) => (
                    <li
                      key={a.id}
                      className="flex items-start justify-between gap-2 rounded-lg border border-border/70 bg-background px-3 py-2 text-xs"
                    >
                      <div>
                        <p className="font-semibold">{a.name}</p>
                        <p className="text-muted-foreground">{a.detail ?? a.description}</p>
                      </div>
                      <span
                        className={`shrink-0 font-bold ${
                          a.status === "ok" || a.status === "skipped"
                            ? "text-accent"
                            : a.status === "warn"
                              ? "text-foreground"
                              : "text-destructive"
                        }`}
                      >
                        {a.status}
                      </span>
                    </li>
                  ))}
                </ul>
              </li>
            ) : null}
          </ul>
        ) : !loading && !error ? (
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        ) : null}
      </main>
    </PermissionGate>
  );
}

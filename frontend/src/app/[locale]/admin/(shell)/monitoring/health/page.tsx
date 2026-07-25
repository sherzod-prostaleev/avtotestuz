"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type HealthData = {
  status: string;
  live: string;
  checks: Record<string, string>;
  checked_at: string;
};

export default function AdminMonitoringHealthPage() {
  const t = useTranslations("AdminMonitoring");
  const [data, setData] = useState<HealthData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);

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
      setData(json.data as HealthData);
      if (!res.ok) setError(t("errorDegraded"));
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

  useEffect(() => {
    if (!auto) return;
    const id = window.setInterval(() => void load(), 10_000);
    return () => window.clearInterval(id);
  }, [auto, load]);

  return (
    <main className="mx-auto max-w-lg space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("healthTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("healthSubtitle")}</p>
      </header>

      <div className="flex flex-wrap items-center gap-2">
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
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

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
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      )}
    </main>
  );
}

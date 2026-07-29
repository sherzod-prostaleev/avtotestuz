"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";

type AlertItem = {
  id: string;
  name: string;
  kind: string;
  enabled: boolean;
  description: string;
  status: string;
  detail?: string;
};

type AlertsPayload = {
  items: AlertItem[];
  checked_at: string;
  note: string;
};

export default function AdminMonitoringAlertsPage() {
  const t = useTranslations("AdminMonitoring");
  const [data, setData] = useState<AlertsPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/monitoring/alerts", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as AlertsPayload);
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
    <PermissionGate permission="monitoring.read">
      <main className="mx-auto max-w-lg space-y-4">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("alertsTitle")}
          description={`${t("alertsSubtitle")} ${t("alertsNote")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? <AdminErrorState message={error} retryLabel={t("retry")} onRetry={() => void load()} /> : null}
        {data?.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}
        {data?.checked_at ? (
          <p className="text-xs text-muted-foreground">
            {t("lastChecked", { time: new Date(data.checked_at).toLocaleTimeString() })}
          </p>
        ) : null}

        {loading && !data ? <AdminSkeleton rows={3} /> : null}

        <ul className="space-y-3">
          {(data?.items ?? []).map((a) => (
            <li key={a.id} className="rounded-xl border border-border bg-card px-4 py-3">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <p className="font-bold">{a.name}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{a.description}</p>
                  {a.detail ? <p className="mt-1 text-xs">{a.detail}</p> : null}
                </div>
                <span
                  className={`text-xs font-bold ${
                    a.status === "ok" || a.status === "skipped"
                      ? "text-accent-ink"
                      : a.status === "warn"
                        ? "text-foreground"
                        : "text-destructive"
                  }`}
                >
                  {a.status}
                </span>
              </div>
            </li>
          ))}
        </ul>
      </main>
    </PermissionGate>
  );
}

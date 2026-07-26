"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";

type FeedItem = {
  kind: string;
  id: string;
  title: string;
  detail?: string;
  created_at: string;
};

type FeedPayload = {
  items: FeedItem[];
  note: string;
};

export default function AdminMonitoringLogsPage() {
  const t = useTranslations("AdminMonitoring");
  const [data, setData] = useState<FeedPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/monitoring/feed", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setData(null);
        return;
      }
      setData(json.data as FeedPayload);
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
      <main className="mx-auto max-w-3xl space-y-4">
        <AdminPageHeader
          badge={t("controlBadge")}
          title={t("logsTitle")}
          description={`${t("logsSubtitle")} ${t("logsNote")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? <AdminErrorState message={error} retryLabel={t("retry")} onRetry={() => void load()} /> : null}
        {data?.note ? <p className="text-xs text-muted-foreground">{data.note}</p> : null}

        {loading && !data ? <AdminSkeleton rows={5} /> : null}

        {!loading && data && data.items.length === 0 ? (
          <AdminEmptyState title={t("logsEmpty")} />
        ) : null}

        <ul className="space-y-2">
          {(data?.items ?? []).map((item) => (
            <li
              key={`${item.kind}-${item.id}`}
              className="rounded-xl border border-border bg-card px-4 py-3 text-sm"
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="font-semibold">{item.title}</p>
                <span className="text-[10px] font-bold uppercase text-muted-foreground">{item.kind}</span>
              </div>
              {item.detail ? <p className="mt-1 text-xs text-muted-foreground">{item.detail}</p> : null}
              <p className="mt-1 text-[11px] text-muted-foreground">
                {new Date(item.created_at).toLocaleString()}
              </p>
            </li>
          ))}
        </ul>
      </main>
    </PermissionGate>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type JobRow = {
  id: string;
  name: string;
  kind: string;
  status: string;
  description: string;
  invoke: string;
};

export default function AdminMonitoringJobsPage() {
  const t = useTranslations("AdminMonitoring");
  const [items, setItems] = useState<JobRow[] | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/monitoring/jobs", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setItems(null);
        return;
      }
      setItems((json.data?.items as JobRow[]) ?? []);
      setNote(typeof json.data?.note === "string" ? json.data.note : null);
    } catch {
      setError(t("errorLoad"));
      setItems(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("jobsTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("jobsSubtitle")}</p>
      </header>

      <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
        <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        {t("refresh")}
      </Button>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {note ? <p className="text-xs text-muted-foreground">{note}</p> : null}

      {!items ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <ul className="space-y-3">
          {items.map((job) => (
            <li key={job.id} className="rounded-xl border border-border bg-card px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="font-bold">{job.name}</p>
                <span className="rounded-md bg-background px-2 py-0.5 text-[10px] font-bold uppercase text-muted-foreground">
                  {job.status} · {job.kind}
                </span>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{job.description}</p>
              <p className="mt-2 font-mono text-xs text-foreground">{job.invoke}</p>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}

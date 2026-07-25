"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type FlagRow = {
  key: string;
  type: string;
  value: unknown;
  description: string;
  updated_by: string;
  updated_at: string;
};

export default function AdminSettingsFlagsPage() {
  const t = useTranslations("AdminFlags");
  const [rows, setRows] = useState<FlagRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/settings/flags", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setRows(null);
        return;
      }
      setRows(json.data as FlagRow[]);
    } catch {
      setError(t("errorLoad"));
      setRows(null);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function toggleBoolean(key: string, next: boolean) {
    setBusy(key);
    setError(null);
    try {
      const res = await fetch(`/api/admin/settings/flags/${encodeURIComponent(key)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: next }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorToggle"));
        return;
      }
      setRows((prev) =>
        (prev ?? []).map((row) => (row.key === key ? { ...(json.data as FlagRow) } : row)),
      );
    } catch {
      setError(t("errorToggle"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
        <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
        {t("refresh")}
      </Button>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!rows ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <ul className="space-y-3">
          {rows.map((row) => {
            const isBool = row.type === "boolean";
            const enabled = isBool && row.value === true;
            return (
              <li key={row.key} className="rounded-xl border border-border bg-card px-4 py-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="font-mono text-sm font-bold">{row.key}</p>
                    <p className="mt-1 text-xs text-muted-foreground">{row.description || t("noDescription")}</p>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      {row.type}
                      {row.updated_by ? ` · ${row.updated_by}` : ""}
                    </p>
                  </div>
                  {isBool ? (
                    <Button
                      type="button"
                      size="sm"
                      variant={enabled ? "outline" : "default"}
                      disabled={busy === row.key}
                      onClick={() => void toggleBoolean(row.key, !enabled)}
                    >
                      {enabled ? t("disable") : t("enable")}
                    </Button>
                  ) : (
                    <pre className="max-w-xs overflow-x-auto rounded-lg bg-background px-2 py-1 font-mono text-xs">
                      {JSON.stringify(row.value)}
                    </pre>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </main>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type LimitRow = {
  key: string;
  free_value: number;
  vip_value: number;
  updated_at: string;
};

export default function AdminSettingsLimitsPage() {
  const t = useTranslations("AdminLimits");
  const [rows, setRows] = useState<LimitRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, { free: string; vip: string }>>({});

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await fetch("/api/admin/settings/limits", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
        setRows(null);
        return;
      }
      const data = json.data as LimitRow[];
      setRows(data);
      const next: Record<string, { free: string; vip: string }> = {};
      for (const row of data) {
        next[row.key] = { free: String(row.free_value), vip: String(row.vip_value) };
      }
      setDrafts(next);
    } catch {
      setError(t("errorLoad"));
      setRows(null);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function save(key: string) {
    const draft = drafts[key];
    if (!draft) return;
    setBusy(key);
    setError(null);
    try {
      const res = await fetch(`/api/admin/settings/limits/${encodeURIComponent(key)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          free_value: Number(draft.free),
          vip_value: Number(draft.vip),
        }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorSave"));
        return;
      }
      const updated = json.data as LimitRow;
      setRows((prev) => (prev ?? []).map((r) => (r.key === key ? updated : r)));
      setDrafts((prev) => ({
        ...prev,
        [key]: { free: String(updated.free_value), vip: String(updated.vip_value) },
      }));
    } catch {
      setError(t("errorSave"));
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

      <Button type="button" size="sm" variant="outline" onClick={() => void load()}>
        <RefreshCw aria-hidden className="mr-1.5 h-3.5 w-3.5" />
        {t("refresh")}
      </Button>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!rows ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <ul className="space-y-3">
          {rows.map((row) => {
            const draft = drafts[row.key] ?? { free: String(row.free_value), vip: String(row.vip_value) };
            return (
              <li key={row.key} className="rounded-xl border border-border bg-card px-4 py-3">
                <p className="font-mono text-sm font-bold">{row.key}</p>
                <div className="mt-2 flex flex-wrap items-end gap-2">
                  <label className="text-xs text-muted-foreground">
                    {t("free")}
                    <input
                      value={draft.free}
                      onChange={(e) =>
                        setDrafts((prev) => ({
                          ...prev,
                          [row.key]: { ...draft, free: e.target.value },
                        }))
                      }
                      className="mt-1 block h-9 w-24 rounded-lg border border-border bg-background px-2 font-mono text-sm"
                    />
                  </label>
                  <label className="text-xs text-muted-foreground">
                    {t("vip")}
                    <input
                      value={draft.vip}
                      onChange={(e) =>
                        setDrafts((prev) => ({
                          ...prev,
                          [row.key]: { ...draft, vip: e.target.value },
                        }))
                      }
                      className="mt-1 block h-9 w-24 rounded-lg border border-border bg-background px-2 font-mono text-sm"
                    />
                  </label>
                  <Button
                    type="button"
                    size="sm"
                    disabled={busy === row.key}
                    onClick={() => void save(row.key)}
                  >
                    {t("save")}
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </main>
  );
}

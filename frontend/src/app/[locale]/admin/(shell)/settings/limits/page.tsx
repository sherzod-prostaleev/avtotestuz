"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { AdminSkeleton } from "@/components/admin/admin-skeleton";
import { AdminTooltip, type AdminTooltipContent } from "@/components/admin/admin-tooltip";
import { PermissionGate } from "@/components/admin/permission-gate";

type LimitRow = {
  key: string;
  free_value: number;
  vip_value: number;
  updated_at: string;
};

const DANGEROUS_LIMITS = new Set([
  "daily_practice_questions",
  "grand_mock_min_studied_pct",
  "leaderboard_daily_points",
]);

export default function AdminSettingsLimitsPage() {
  const t = useTranslations("AdminLimits");
  const tNav = useTranslations("AdminNav");
  const [rows, setRows] = useState<LimitRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, { free: string; vip: string }>>({});

  const load = useCallback(async () => {
    setLoading(true);
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
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  function limitTooltip(key: string): AdminTooltipContent | null {
    if (!DANGEROUS_LIMITS.has(key)) return null;
    const base = `tip_${key}` as "tip_daily_practice_questions";
    return {
      what: t(`${base}_what`),
      why: t(`${base}_why`),
      risks: t(`${base}_risks`),
      recommend: t(`${base}_recommend`),
    };
  }

  async function save(key: string) {
    const draft = drafts[key];
    if (!draft) return;
    if (DANGEROUS_LIMITS.has(key)) {
      const ok = window.confirm(t("confirmSave", { key }));
      if (!ok) return;
    }
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
    <PermissionGate permission="settings.config">
      <main className="mx-auto max-w-2xl space-y-5">
        <AdminPageHeader
          badge={tNav("groupSettings")}
          title={t("title")}
          description={`${t("subtitle")} ${t("note")}`}
          actions={
            <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
              <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              {t("refresh")}
            </Button>
          }
        />

        {error ? (
          <AdminErrorState message={error} retryLabel={t("refresh")} onRetry={() => void load()} />
        ) : null}

        {!rows && !error ? <AdminSkeleton rows={5} /> : null}

        {rows ? (
          <ul className="space-y-3">
            {rows.map((row) => {
              const draft = drafts[row.key] ?? { free: String(row.free_value), vip: String(row.vip_value) };
              const tooltip = limitTooltip(row.key);
              const dangerous = DANGEROUS_LIMITS.has(row.key);
              return (
                <li
                  key={row.key}
                  className={`rounded-2xl border bg-card/70 px-4 py-3 ${
                    dangerous ? "border-amber-500/30" : "border-border/80"
                  }`}
                >
                  <div className="flex items-center gap-1">
                    <p className="font-mono text-sm font-bold">{row.key}</p>
                    {tooltip ? <AdminTooltip content={tooltip} label={t("tipLabel")} /> : null}
                    {dangerous ? (
                      <span className="rounded-md bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-bold uppercase text-amber-200">
                        {t("dangerBadge")}
                      </span>
                    ) : null}
                  </div>
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
        ) : null}
      </main>
    </PermissionGate>
  );
}

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

type FlagRow = {
  key: string;
  type: string;
  value: unknown;
  description: string;
  updated_by: string;
  updated_at: string;
};

const DANGEROUS_FLAGS = new Set([
  "maintenance_mode",
  "checkout_payme",
  "checkout_click",
  "web_push_digest",
  "arena_enabled",
]);

export default function AdminSettingsFlagsPage() {
  const t = useTranslations("AdminFlags");
  const tNav = useTranslations("AdminNav");
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

  function flagTooltip(key: string): AdminTooltipContent | null {
    if (!DANGEROUS_FLAGS.has(key)) return null;
    const base = `tip_${key}` as "tip_maintenance_mode";
    return {
      what: t(`${base}_what`),
      why: t(`${base}_why`),
      when: t(`${base}_when`),
      risks: t(`${base}_risks`),
      recommend: t(`${base}_recommend`),
    };
  }

  async function toggleBoolean(key: string, next: boolean) {
    if (DANGEROUS_FLAGS.has(key) && next) {
      const ok = window.confirm(t("confirmEnable", { key }));
      if (!ok) return;
    }
    if (DANGEROUS_FLAGS.has(key) && !next) {
      const ok = window.confirm(t("confirmDisable", { key }));
      if (!ok) return;
    }
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
    <PermissionGate permission="settings.flags">
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
              const isBool = row.type === "boolean";
              const enabled = isBool && row.value === true;
              const tooltip = flagTooltip(row.key);
              const dangerous = DANGEROUS_FLAGS.has(row.key);
              return (
                <li
                  key={row.key}
                  className={`rounded-2xl border bg-card/70 px-4 py-3 ${
                    dangerous ? "border-amber-500/30" : "border-border/80"
                  }`}
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1">
                        <p className="font-mono text-sm font-bold">{row.key}</p>
                        {tooltip ? <AdminTooltip content={tooltip} label={t("tipLabel")} /> : null}
                        {dangerous ? (
                          <span className="rounded-md bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-bold uppercase text-amber-200">
                            {t("dangerBadge")}
                          </span>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">{row.description || t("noDescription")}</p>
                      <p className="mt-1 text-[11px] text-muted-foreground">
                        {row.type}
                        {row.updated_by ? ` · ${row.updated_by}` : ""}
                        {row.updated_at ? ` · ${new Date(row.updated_at).toLocaleString()}` : ""}
                      </p>
                    </div>
                    {isBool ? (
                      <Button
                        type="button"
                        size="sm"
                        variant={enabled ? "outline" : "game"}
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
        ) : null}
      </main>
    </PermissionGate>
  );
}

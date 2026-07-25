"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

type ProviderRow = { provider: string; enabled: boolean };

export default function AdminPaymentsProvidersPage() {
  const t = useTranslations("AdminPayments");
  const [rows, setRows] = useState<ProviderRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch("/api/admin/payments/providers", { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "forbidden" ? t("errorForbiddenProviders") : t("errorLoad"));
        setRows(null);
        return;
      }
      setRows(json.data as ProviderRow[]);
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

  async function toggle(provider: string, enabled: boolean) {
    if (!enabled) {
      const ok = window.confirm(t("confirmDisable", { provider }));
      if (!ok) return;
    }
    setBusy(provider);
    setError(null);
    try {
      const res = await fetch(`/api/admin/payments/providers/${provider}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(
          json?.error?.code === "forbidden" ? t("errorForbiddenKeys") : t("errorToggle"),
        );
        return;
      }
      setRows((prev) =>
        (prev ?? []).map((row) =>
          row.provider === provider
            ? { ...row, enabled: Boolean(json.data?.enabled ?? enabled) }
            : row,
        ),
      );
    } catch {
      setError(t("errorToggle"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <main className="mx-auto max-w-lg space-y-4">
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("providersTitle")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("providersSubtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("providersNote")}</p>
      </header>

      <div className="flex gap-2">
        <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void load()}>
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!rows ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : (
        <ul className="space-y-3">
          {rows.map((row) => (
            <li
              key={row.provider}
              className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
            >
              <div>
                <p className="font-bold capitalize">{row.provider}</p>
                <p className="text-xs text-muted-foreground">
                  {row.enabled ? t("providerOn") : t("providerOff")}
                </p>
              </div>
              <Button
                type="button"
                size="sm"
                variant={row.enabled ? "outline" : "game"}
                disabled={busy === row.provider}
                onClick={() => void toggle(row.provider, !row.enabled)}
              >
                {row.enabled ? t("disable") : t("enable")}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
